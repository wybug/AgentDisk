"""Run the OKF writer agent eval suite + emit a baseline report.

This is the entry point invoked by ``make okf-eval``. It:

1. Validates the eval set JSON against the ADK pydantic schema (catches
   typos before paying for LLM calls).
2. Checks that all required env vars are set (base URL, API key, public
   directory id, LLM key).
3. Invokes ``adk eval`` as a subprocess — same command users run by hand.
4. Locates the most recent ``*.evalset_result.json`` written by ADK under
   ``.adk/eval_history/``.
5. Parses the result, computes per-case scores, and emits
   ``baseline_results.md`` next to the eval set.

Usage::

    python evals/run_evals.py                  # run all cases
    python evals/run_evals.py register_when_missing noop_clarification  # subset
    python evals/run_evals.py --dry-run        # validate without calling adk

"""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
import time
from pathlib import Path
from typing import Any

# Make this script runnable both as `python evals/run_evals.py` from the
# package root and as `python -m evals.run_evals` — same sys.path trick as
# scenarios/bootstrap_bundle.py.
_PKG_ROOT = Path(__file__).resolve().parent.parent
if str(_PKG_ROOT) not in sys.path:
    sys.path.insert(0, str(_PKG_ROOT))

# Resolve paths relative to the package root, not the CWD the user ran from.
AGENT_MODULE_DIR = _PKG_ROOT / "adk_writer_agent"
EVAL_SET_TEMPLATE_PATH = _PKG_ROOT / "evals" / "okf_writer_eval_set.json"
EVAL_SET_CONCRETE_PATH = _PKG_ROOT / "evals" / "okf_writer_eval_set.concrete.json"
REPORT_PATH = _PKG_ROOT / "evals" / "baseline_results.md"
EVAL_HISTORY_DIR = AGENT_MODULE_DIR / ".adk" / "eval_history"

# The template uses {{PD_ID}} and {{BUNDLE_ID}} placeholders that we
# substitute before invoking `adk eval`. Schema validation runs against
# the template — placeholders are JSON values (strings/ints) that the
# ADK schema accepts at validation time even though final values get
# rewritten before the actual eval run.
EVAL_SET_PATH = EVAL_SET_TEMPLATE_PATH

# Metrics emitted by ADK's default config — see
# google.adk.cli.cli_eval.DEFAULT_CRITERIA.
TRAJECTORY_KEY = "tool_trajectory_avg_score"
RESPONSE_KEY = "response_match_score"

# Scores below this go into the "improvement opportunities" section of the
# report. 1.0 = perfect trajectory (default threshold); RESPONSE 0.8 is the
# ADK default. We use 0.8 for both so the report flags anything that isn't
# clearly passing.
PASS_THRESHOLD = 0.8


def _err(msg: str) -> None:
    sys.stderr.write(f"error: {msg}\n")


def _substitute_placeholders(obj: Any, mapping: dict[str, Any]) -> Any:
    """Recursively replace ``{{KEY}}`` placeholders in a JSON-like tree.

    Two cases:
    * If a string is EXACTLY ``"{{KEY}}"``, returns ``mapping["{{KEY}}"]``
      directly — preserving its type (so int substitutions stay ints).
    * Otherwise, replaces ``{{KEY}}`` inside the string with ``str(value)``.
    """
    if isinstance(obj, str):
        if obj in mapping:
            return mapping[obj]
        result = obj
        for placeholder, value in mapping.items():
            result = result.replace(placeholder, str(value))
        return result
    if isinstance(obj, dict):
        return {k: _substitute_placeholders(v, mapping) for k, v in obj.items()}
    if isinstance(obj, list):
        return [_substitute_placeholders(x, mapping) for x in obj]
    return obj


def _render_concrete_eval_set(pd_id: int, bundle_id: int | None) -> Path:
    """Render the parameterized eval set into a concrete JSON file.

    Replaces ``{{PD_ID}}`` and ``{{BUNDLE_ID}}`` placeholders with actual
    integer values. Writes to ``EVAL_SET_CONCRETE_PATH`` and returns it.
    """
    raw = EVAL_SET_TEMPLATE_PATH.read_text(encoding="utf-8")
    template = json.loads(raw)
    mapping = {"{{PD_ID}}": pd_id}
    if bundle_id is not None:
        mapping["{{BUNDLE_ID}}"] = bundle_id
    concrete = _substitute_placeholders(template, mapping)
    EVAL_SET_CONCRETE_PATH.write_text(
        json.dumps(concrete, indent=2, ensure_ascii=False),
        encoding="utf-8",
    )
    return EVAL_SET_CONCRETE_PATH


def _resolve_pd_id() -> int | None:
    """Resolve the PD id from env (ID directly or PATH via API lookup)."""
    pd_id_raw = os.environ.get("AGENTDISK_PUBLIC_DIRECTORY_ID")
    if pd_id_raw and pd_id_raw.strip():
        try:
            return int(pd_id_raw)
        except ValueError:
            _err(f"AGENTDISK_PUBLIC_DIRECTORY_ID not an int: {pd_id_raw!r}")
            return None

    pd_path = os.environ.get("AGENTDISK_PUBLIC_DIRECTORY_PATH", "").strip()
    if not pd_path:
        return None

    # Use the same helper as eval_setup.py so PATH→ID resolution is
    # consistent between pre-clean and eval set rendering.
    import httpx  # local import — only needed when PATH form is used

    from evals.eval_setup import _resolve_pd_id_by_path  # type: ignore[import-not-found]

    base_url = os.environ.get("AGENTDISK_BASE_URL", "").rstrip("/")
    api_key = os.environ.get("AGENTDISK_API_KEY", "")
    with httpx.Client(
        base_url=base_url,
        headers={"X-API-Key": api_key, "Content-Type": "application/json"},
        timeout=15.0,
    ) as client:
        return _resolve_pd_id_by_path(client, pd_path)


def _read_bundle_id_from_state() -> int | None:
    """Read the bundle id persisted by the most recent reset_data call."""
    from evals.eval_setup import OKF_EVAL_BUNDLE_STATE_FILE  # type: ignore[import-not-found]

    try:
        with open(OKF_EVAL_BUNDLE_STATE_FILE) as f:
            data = json.load(f)
    except (OSError, json.JSONDecodeError):
        return None
    bid = data.get("bundle_id") if isinstance(data, dict) else None
    return int(bid) if isinstance(bid, int) and bid > 0 else None


def _validate_eval_set(eval_set_path: Path) -> tuple[bool, str]:
    """Parse the eval set JSON against the ADK pydantic schema.

    Returns (ok, summary) where summary is a one-line description for the
    report header. Failing this check means the JSON doesn't even load —
    fix it before spending LLM tokens.
    """
    try:
        from google.adk.evaluation.eval_set import EvalSet
    except ImportError as exc:
        return False, f"google-adk not installed: {exc}"
    try:
        eval_set = EvalSet.model_validate_json(eval_set_path.read_text(encoding="utf-8"))
    except Exception as exc:  # noqa: BLE001 — surface the validation error verbatim
        return False, f"eval set schema validation failed: {exc}"
    return True, f"{eval_set.eval_set_id}: {len(eval_set.eval_cases)} cases"


def _check_env() -> list[str]:
    """Return a list of missing env vars (empty list = OK)."""
    missing: list[str] = []
    for name in ("AGENTDISK_BASE_URL", "AGENTDISK_API_KEY"):
        if not os.environ.get(name):
            missing.append(name)

    # Public directory can be specified by ID (preferred) or by PATH
    # (what the UI surfaces). Exactly one is required.
    if not (
        os.environ.get("AGENTDISK_PUBLIC_DIRECTORY_ID")
        or os.environ.get("AGENTDISK_PUBLIC_DIRECTORY_PATH")
    ):
        missing.append("AGENTDISK_PUBLIC_DIRECTORY_ID|AGENTDISK_PUBLIC_DIRECTORY_PATH")

    # LLM key — depends on WRITER_AGENT_MODEL. Default is DeepSeek's official
    # API (deepseek/deepseek-chat), so DEEPSEEK_API_KEY is the canonical key.
    model = os.environ.get("WRITER_AGENT_MODEL", "deepseek/deepseek-chat").lower()
    if "deepseek" in model:
        # DeepSeek's official endpoint (https://api.deepseek.com/beta) is
        # baked into LiteLLM — no URL config needed unless using a proxy.
        if not os.environ.get("DEEPSEEK_API_KEY"):
            missing.append("DEEPSEEK_API_KEY")
    elif "gemini" in model:
        # LiteLLM (which ADK uses) supports either GEMINI_API_KEY or
        # GOOGLE_API_KEY — accept either.
        if not (os.environ.get("GEMINI_API_KEY") or os.environ.get("GOOGLE_API_KEY")):
            missing.append("GEMINI_API_KEY|GOOGLE_API_KEY")
    elif "claude" in model or "anthropic" in model:
        if not os.environ.get("ANTHROPIC_API_KEY"):
            missing.append("ANTHROPIC_API_KEY")
    return missing


def _build_adk_cmd(
    case_ids: list[str] | None,
    print_detailed: bool,
    eval_set_path: Path | None = None,
) -> list[str]:
    """Construct the `adk eval` argv list.

    The path-then-evalset-or-id form lets us append ``:case1,case2`` to
    run only specific cases (see ``parse_and_get_evals_to_run``).
    """
    target_path = eval_set_path or EVAL_SET_CONCRETE_PATH
    target = str(target_path)
    if case_ids:
        target = f"{target}:{','.join(case_ids)}"
    cmd = [
        sys.executable,
        "-m",
        "google.adk.cli",
        "eval",
        str(AGENT_MODULE_DIR),
        target,
    ]
    if print_detailed:
        cmd.append("--print_detailed_results")
    return cmd


def _find_latest_result() -> Path | None:
    """Newest ``*.evalset_result.json`` under ``.adk/eval_history/``.

    Each run writes one file with a ``time.time()`` suffix, so lexical
    sort + last = most recent. Returns ``None`` if no results exist.
    """
    if not EVAL_HISTORY_DIR.exists():
        return None
    files = sorted(EVAL_HISTORY_DIR.glob("*.evalset_result.json"))
    return files[-1] if files else None


def _short_tool_summary(tool_calls: list[Any], limit: int = 6) -> str:
    """Render a list of FunctionCall-shaped dicts as ``name(arg=...)`` strings.

    Used in the trajectory diff column. Long arg values (like the
    ``content`` field of write_markdown) are truncated so the table stays
    readable.
    """
    out: list[str] = []
    for call in tool_calls[:limit]:
        name = call.get("name") or call.get("functionCall", {}).get("name") or "?"
        args = call.get("args") or call.get("functionCall", {}).get("args") or {}
        arg_pairs: list[str] = []
        for k, v in args.items():
            text = json.dumps(v, ensure_ascii=False)
            if len(text) > 30:
                text = text[:27] + "..."
            arg_pairs.append(f"{k}={text}")
        out.append(f"{name}({', '.join(arg_pairs)})" if arg_pairs else f"{name}()")
    if len(tool_calls) > limit:
        out.append(f"... +{len(tool_calls) - limit} more")
    return " → ".join(out) if out else "(none)"


def _extract_metric(result: dict[str, Any], key: str) -> tuple[float | None, str]:
    """Pull a metric score out of ``overall_eval_metric_results``.

    ADK's pydantic schema uses camelCase aliases. The result JSON may
    contain either the camelCase key (when serialized) or the snake_case
    key (if dumped via model_dump). We try both.

    Returns (score, status_string). Score is None if the metric is
    absent — e.g. noop cases have no expected response so
    response_match_score is omitted.
    """
    metrics = (
        result.get("overallEvalMetricResults") or result.get("overall_eval_metric_results") or []
    )
    for entry in metrics:
        name = entry.get("metricName") or entry.get("metric_name") or ""
        if name == key:
            score = entry.get("score")
            status = entry.get("evalStatus") or entry.get("eval_status") or "?"
            return score, str(status)
    return None, "N/A"


def _extract_trajectories(result: dict[str, Any]) -> tuple[list[Any], list[Any]]:
    """Pull the actual + expected tool call lists out of a case result.

    Actual tool calls live inside ``invocation_events[].content.parts[].function_call``
    — ADK stores events (not just tool calls) so each part can also hold
    thoughts, text, or function_responses. We filter to function_call only.

    Expected tool calls live inside the eval set's
    ``intermediate_data.tool_uses`` (defined statically when the eval set
    was authored) — the result JSON echoes them back via
    ``expected_invocation.intermediate_data.tool_uses``.

    Older serializations used camelCase aliases; we handle both.
    """
    actual_calls: list[Any] = []
    expected_calls: list[Any] = []
    per_inv = (
        result.get("evalMetricResultPerInvocation")
        or result.get("eval_metric_result_per_invocation")
        or []
    )
    for entry in per_inv:
        for side in ("actualInvocation", "actual_invocation"):
            inv = entry.get(side)
            if not inv:
                continue
            inter = inv.get("intermediateData") or inv.get("intermediate_data") or {}
            # Newer ADK: tool calls are nested in invocation_events.
            events = inter.get("invocationEvents") or inter.get("invocation_events") or []
            for event in events:
                parts = (event.get("content") or {}).get("parts") or []
                for part in parts:
                    fc = part.get("functionCall") or part.get("function_call")
                    if fc:
                        actual_calls.append(fc)
            # Older ADK / direct serialization: tool calls are at top level.
            calls = inter.get("toolUses") or inter.get("tool_uses") or []
            for call in calls:
                # Avoid double-counting if both shapes are present.
                if call not in actual_calls:
                    actual_calls.append(call)
        for side in ("expectedInvocation", "expected_invocation"):
            inv = entry.get(side)
            if not inv:
                continue
            inter = inv.get("intermediateData") or inv.get("intermediate_data") or {}
            calls = inter.get("toolUses") or inter.get("tool_uses") or []
            expected_calls.extend(calls)
    return actual_calls, expected_calls


def _categorize_failure(actual: list[Any], expected: list[Any], traj_score: float | None) -> str:
    """Best-effort root-cause categorization for failed trajectory cases.

    The point of the baseline is to surface IMPROVEMENT OPPORTUNITIES, so
    we distinguish:

    * missing_tool — expected a tool that wasn't called (prompt issue)
    * extra_tool — agent called a tool not in expected (over-cautious agent)
    * duplicate_tool — agent called the same tool more than once (real bug)
    * arg_mismatch — same names in same order, args differ (likely content
      verbosity — LLM wrote a fuller response than the eval set's expected
      minimal content)
    * no_data — neither side has tool calls (response check failed)
    """
    actual_names = [c.get("name") for c in actual]
    expected_names = [c.get("name") for c in expected]
    if not expected and not actual:
        return "no_data"
    if not expected:
        return "spurious_tool_call"

    missing = set(expected_names) - set(actual_names)
    if missing and traj_score is not None and traj_score < PASS_THRESHOLD:
        return f"missing_tool({','.join(sorted(missing))})"
    extra = set(actual_names) - set(expected_names)
    if extra:
        return f"extra_tool({','.join(sorted(extra))})"

    # Detect duplicates: same tool name called more than once in actual
    # when expected has it only once. This is a real agent bug (e.g.
    # calling aggregate_types twice for no reason).
    if len(actual_names) > len(expected_names):
        actual_counts: dict[str, int] = {}
        for name in actual_names:
            actual_counts[name] = actual_counts.get(name, 0) + 1
        expected_counts: dict[str, int] = {}
        for name in expected_names:
            expected_counts[name] = expected_counts.get(name, 0) + 1
        duplicates = [n for n, c in actual_counts.items() if c > expected_counts.get(n, 0)]
        if duplicates:
            return f"duplicate_tool({','.join(sorted(duplicates))})"

    if expected_names == actual_names and traj_score is not None and traj_score < PASS_THRESHOLD:
        # Args differ. Almost always the ``content`` field of write_markdown
        # — LLM writes a fuller response than the eval set's minimal seed.
        differing_args: list[str] = []
        for a, e in zip(actual, expected, strict=False):
            if a.get("name") != e.get("name"):
                continue
            a_args = a.get("args") or {}
            e_args = e.get("args") or {}
            for k in set(a_args) | set(e_args):
                if a_args.get(k) != e_args.get(k):
                    differing_args.append(f"{e.get('name')}.{k}")
        if differing_args:
            return f"arg_mismatch({','.join(differing_args[:3])})"
        return "arg_mismatch"
    return "ok"


def _render_report(
    ok: bool,
    schema_summary: str,
    cases: list[dict[str, Any]],
    elapsed_s: float,
    dry_run: bool,
) -> str:
    """Build the baseline_results.md content."""
    lines: list[str] = []
    lines.append("# OKF Writer Agent — Baseline Eval Results")
    lines.append("")
    lines.append(f"**Generated:** {time.strftime('%Y-%m-%d %H:%M:%S %Z')}  ")
    lines.append(f"**Eval set:** `{EVAL_SET_PATH.relative_to(_PKG_ROOT)}`  ")
    lines.append(f"**Schema check:** {schema_summary}  ")
    lines.append(f"**Model:** `{os.environ.get('WRITER_AGENT_MODEL', 'gemini-2.5-flash')}`  ")
    if dry_run:
        lines.append("**Mode:** dry-run (no LLM calls)  ")
    else:
        lines.append(f"**Elapsed:** {elapsed_s:.1f}s  ")
    lines.append("")

    if not cases:
        lines.append("_No case results parsed — see schema check + run output above._")
        return "\n".join(lines)

    # Determine whether response_match_score should gate "pass" status. The
    # eval set has ``final_response: null`` for every case (LLM output is
    # non-deterministic; locking in a fixed expected string would make
    # every run fail for stylistic reasons). When ALL cases report
    # response_match_score = 0, treat the metric as informational — base
    # pass/fail on trajectory only. If even one case has a non-null/non-zero
    # response score, the metric is real and we gate on it.
    has_real_response_scores = any((c.get("response_score") or 0) > 0 for c in cases)

    lines.append("## Per-case scores")
    lines.append("")
    if not has_real_response_scores:
        lines.append(
            "> Note: ``response_match_score`` is informational only — the eval"
            " set has no ``final_response`` to match against (LLM output is"
            " non-deterministic). Pass/fail is decided by trajectory score."
        )
        lines.append("")
    lines.append("| Case | Trajectory | Response | Status | Failure category |")
    lines.append("|---|---|---|---|---|")
    summary_rows: list[dict[str, Any]] = []
    for c in cases:
        traj = c["trajectory_score"]
        resp = c["response_score"]
        traj_str = f"{traj:.2f}" if traj is not None else "—"
        resp_str = f"{resp:.2f}" if resp is not None else "—"
        if has_real_response_scores:
            passed = (traj is None or traj >= PASS_THRESHOLD) and (
                resp is None or resp >= PASS_THRESHOLD
            )
        else:
            passed = traj is None or traj >= PASS_THRESHOLD
        c["passed"] = passed
        status = "PASS" if passed else "FAIL"
        lines.append(f"| `{c['eval_id']}` | {traj_str} | {resp_str} | {status} | {c['category']} |")
        summary_rows.append(c)
    lines.append("")

    n_pass = sum(1 for r in summary_rows if r["passed"])
    n_fail = len(summary_rows) - n_pass
    pct = n_pass * 100 // max(1, len(summary_rows))
    lines.append(f"**Summary:** {n_pass}/{len(summary_rows)} pass ({pct}%), {n_fail} fail.")
    lines.append("")

    if n_fail == 0:
        lines.append("## No improvement opportunities")
        lines.append("")
        lines.append("All cases passed the 0.8 threshold. Re-run after changing the prompt")
        lines.append("or tools to catch regressions.")
        return "\n".join(lines)

    lines.append("## Improvement opportunities")
    lines.append("")
    lines.append("Failed cases — sorted by category so you can pick a fix strategy.")
    lines.append("")

    by_cat: dict[str, list[dict[str, Any]]] = {}
    for r in summary_rows:
        if r["passed"]:
            continue
        by_cat.setdefault(r["category"], []).append(r)
    for cat in sorted(by_cat):
        lines.append(f"### {cat}")
        lines.append("")
        for r in by_cat[cat]:
            lines.append(f"#### `{r['eval_id']}`")
            lines.append("")
            lines.append(f"- **Expected trajectory:** {_short_tool_summary(r['expected_calls'])}")
            lines.append(f"- **Actual trajectory:** {_short_tool_summary(r['actual_calls'])}")
            if r["trajectory_score"] is not None:
                lines.append(f"- **Trajectory score:** {r['trajectory_score']:.2f}")
            if r["response_score"] is not None:
                lines.append(f"- **Response score:** {r['response_score']:.2f}")
            lines.append("")

    lines.append("## Suggested next steps")
    lines.append("")
    cat_set = set(r["category"] for r in summary_rows if not r["passed"])
    suggestions: list[str] = []
    if any(c.startswith("missing_tool") for c in cat_set):
        suggestions.append(
            "- **Missing tool calls** — review `agent.py` INSTRUCTION. The agent "
            "isn't aware a step is required. Add explicit numbered steps to the "
            "prompt (e.g. '1. Always register_bundle before list_nodes')."
        )
    if any(c.startswith("extra_tool") for c in cat_set):
        suggestions.append(
            "- **Extra tool calls** — agent is being overly cautious. Tighten the "
            "prompt to say 'do not call X unless Y' (e.g. 'do not register_bundle "
            "if the prompt mentions an existing bundle_id')."
        )
    if any(c.startswith("duplicate_tool") for c in cat_set):
        suggestions.append(
            "- **Duplicate tool calls** — agent called the same tool twice in a "
            "row with no new information. Likely a control-flow bug in the agent's "
            "reasoning. Reproduce with `adk eval . evals/okf_writer_eval_set."
            "concrete.json:<case_id> --print_detailed_results` and inspect the "
            "thought chain to find what triggered the second call."
        )
    if any(c.startswith("arg_mismatch") for c in cat_set):
        suggestions.append(
            "- **Argument mismatch (content verbosity)** — the agent's "
            "``write_markdown`` content is longer than the eval set's minimal "
            "expected content. This is by design (LLMs naturally expand prompts). "
            "Two fix paths: (a) relax the eval set to use ``IN_ORDER`` match + "
            "``rubrics`` that check for required frontmatter keys instead of "
            "exact content; (b) tighten the prompt to say '回复尽量短，只写最小内容'."
        )
    if any(c == "spurious_tool_call" for c in cat_set):
        suggestions.append(
            "- **Spurious tool calls** — agent called a tool when none were "
            "needed (e.g. greeted user with a register_bundle call). Add a "
            "'do not call tools without an explicit instruction' clause."
        )
    if any(c == "no_data" for c in cat_set):
        suggestions.append(
            "- **No trajectory data** — agent produced no tool calls but the "
            "case expected some. Verify the prompt is unambiguous; if the case "
            "is intentionally a no-op (e.g. `noop_clarification`), this is a "
            "PASS, not a fail."
        )
    if not suggestions:
        suggestions.append("- (No specific suggestions — review the per-case details above.)")
    lines.extend(suggestions)
    lines.append("")
    lines.append("---")
    lines.append("*This report is regenerated on every `make okf-eval` run. Commit it")
    lines.append("to track baseline drift over time.*")
    return "\n".join(lines)


def _parse_result_file(result_path: Path) -> list[dict[str, Any]]:
    """Extract one row per eval case from the EvalSetResult JSON."""
    raw = result_path.read_text(encoding="utf-8")
    try:
        data = json.loads(raw)
    except json.JSONDecodeError:
        # Some legacy serializations wrap the JSON in a string — unwrap.
        data = json.loads(json.loads(raw))

    case_results = data.get("evalCaseResults") or data.get("eval_case_results") or []
    rows: list[dict[str, Any]] = []
    for case in case_results:
        traj, traj_status = _extract_metric(case, TRAJECTORY_KEY)
        resp, _ = _extract_metric(case, RESPONSE_KEY)
        actual, expected = _extract_trajectories(case)
        category = _categorize_failure(actual, expected, traj)
        rows.append(
            {
                "eval_id": case.get("evalId") or case.get("eval_id") or "?",
                "trajectory_score": traj,
                "trajectory_status": traj_status,
                "response_score": resp,
                "actual_calls": actual,
                "expected_calls": expected,
                "category": category,
            }
        )
    return rows


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "case_ids",
        nargs="*",
        help="Optional subset of eval_id values to run (default: all).",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Validate eval set + env, then exit without invoking adk eval.",
    )
    parser.add_argument(
        "--print-detailed-results",
        action="store_true",
        help="Pass --print_detailed_results to `adk eval` (verbose console output).",
    )
    parser.add_argument(
        "--no-report",
        action="store_true",
        help="Skip emitting baseline_results.md (use with --print-detailed-results).",
    )
    parser.add_argument(
        "--reparse",
        action="store_true",
        help=(
            "Re-parse the most recent result file and emit a fresh report "
            "without invoking adk eval. Useful after parser fixes."
        ),
    )
    args = parser.parse_args()

    if args.reparse:
        result_path = _find_latest_result()
        if result_path is None:
            _err(f"no eval result file found under {EVAL_HISTORY_DIR}")
            return 5
        try:
            rows = _parse_result_file(result_path)
        except Exception as exc:  # noqa: BLE001
            _err(f"failed to parse result file: {exc}")
            return 6
        report = _render_report(
            ok=True,
            schema_summary="(reparse — skipped)",
            cases=rows,
            elapsed_s=0.0,
            dry_run=False,
        )
        REPORT_PATH.write_text(report, encoding="utf-8")
        print(f"[ok] re-parsed report → {REPORT_PATH.relative_to(_PKG_ROOT)}")
        n_pass = sum(1 for r in rows if r.get("passed", False))
        print(f"[summary] {n_pass}/{len(rows)} cases pass at threshold {PASS_THRESHOLD}")
        return 0

    # 1. Schema check first — cheapest, no network. Validates the TEMPLATE
    # (placeholders are JSON values, so the schema accepts them).
    ok, schema_summary = _validate_eval_set(EVAL_SET_TEMPLATE_PATH)
    if not ok:
        _err(schema_summary)
        return 2
    print(f"[ok] {schema_summary}")

    # 2. Env check. Missing env → instruct the user, don't try to run.
    missing = _check_env()
    if missing:
        _err("missing required env vars: " + ", ".join(missing))
        _err("copy .env.example to .env and fill in the values.")
        if not args.dry_run:
            return 3
    else:
        print("[ok] env vars present")

    if args.dry_run:
        print("[dry-run] skipping adk eval invocation")
        report = _render_report(
            ok=True,
            schema_summary=schema_summary,
            cases=[],
            elapsed_s=0.0,
            dry_run=True,
        )
        if not args.no_report:
            REPORT_PATH.write_text(report, encoding="utf-8")
            print(f"[ok] dry-run stub report → {REPORT_PATH}")
        return 0

    # 3. Pre-clean OKF state so a fresh bundle exists. reset_data also
    # runs before each case via ADK's hook, but the FIRST pre-clean here
    # is what populates the bundle_id we'll substitute into the eval set.
    pd_id = _resolve_pd_id()
    if pd_id is None:
        _err("could not resolve PD id from AGENTDISK_PUBLIC_DIRECTORY_ID/PATH")
        return 7
    print(f"[ok] PD id = {pd_id}")

    try:
        from evals.eval_setup import clean_okf_state  # type: ignore[import-not-found]
    except ImportError as exc:
        _err(f"could not import evals.eval_setup.clean_okf_state: {exc}")
        return 7
    stats = clean_okf_state()
    bundle_id = stats.get("bundle_id") if isinstance(stats, dict) else None
    if not isinstance(bundle_id, int):
        # Fall back to the state file in case clean_okf_state ran in a
        # different process (it shouldn't, but be defensive).
        bundle_id = _read_bundle_id_from_state()
    if bundle_id is None:
        _err("clean_okf_state did not register a bundle; cannot substitute {{BUNDLE_ID}}")
        _err("check backend connectivity + AGENTDISK_API_KEY permissions.")
        return 7
    print(f"[ok] bundle id = {bundle_id}")

    # 4. Render the concrete eval set by substituting placeholders. ADK
    # reads this file; the template (with {{PD_ID}}, {{BUNDLE_ID}}) is
    # never passed to adk eval directly.
    concrete_path = _render_concrete_eval_set(pd_id, bundle_id)
    print(f"[ok] concrete eval set → {concrete_path.relative_to(_PKG_ROOT)}")

    # 5. Invoke `adk eval` as a subprocess — same command users would run
    # by hand, so debugging from the wrapper output maps 1:1 to manual runs.
    cmd = _build_adk_cmd(args.case_ids, args.print_detailed_results, concrete_path)
    print(f"[run] {' '.join(cmd)}")
    start = time.perf_counter()
    try:
        proc = subprocess.run(cmd, check=False, cwd=str(_PKG_ROOT))
    except FileNotFoundError:
        _err("`adk` / google-adk not on PATH. Run `pip install -e .[eval]`.")
        return 4
    elapsed = time.perf_counter() - start
    if proc.returncode != 0:
        _err(f"`adk eval` exited with code {proc.returncode}")
        return proc.returncode

    # 4. Locate the most recent result file. ADK writes one per run.
    result_path = _find_latest_result()
    if result_path is None:
        _err(f"no eval result file found under {EVAL_HISTORY_DIR}")
        _err("did ADK complete its run? check the output above.")
        return 5
    print(f"[ok] result → {result_path.relative_to(_PKG_ROOT)}")

    # 5. Parse + render.
    try:
        rows = _parse_result_file(result_path)
    except Exception as exc:  # noqa: BLE001 — surface ADK schema drift clearly
        _err(f"failed to parse result file: {exc}")
        return 6
    if not rows:
        _err("result file contained no case results")
        return 6

    report = _render_report(
        ok=True,
        schema_summary=schema_summary,
        cases=rows,
        elapsed_s=elapsed,
        dry_run=False,
    )
    if args.no_report:
        print(report)
    else:
        REPORT_PATH.write_text(report, encoding="utf-8")
        print(f"[ok] report → {REPORT_PATH.relative_to(_PKG_ROOT)}")
        n_pass = sum(1 for r in rows if r.get("passed", False))
        print(f"[summary] {n_pass}/{len(rows)} cases pass at threshold {PASS_THRESHOLD}")

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
