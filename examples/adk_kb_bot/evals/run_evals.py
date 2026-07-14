"""Run the kb_bot eval suite + emit a baseline report.

Read-only counterpart to ``adk_writer_agent/evals/run_evals.py``. The
flow is simpler because kb_bot consumes a pre-built bundle:

1. Schema-validate the eval set JSON.
2. Check env + bundle reachability via ``eval_setup.check_preconditions``.
3. Invoke ``adk eval`` as a subprocess.
4. Locate the latest ``*.evalset_result.json``.
5. Parse rubric scores + emit ``baseline_results.md``.

Usage::

    python evals/run_evals.py                  # run all cases
    python evals/run_evals.py cite_source_when_answered  # subset
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

_PKG_ROOT = Path(__file__).resolve().parent.parent
if str(_PKG_ROOT) not in sys.path:
    sys.path.insert(0, str(_PKG_ROOT))

# Auto-load .env so values with spaces (e.g. KB_BOT_PUBLIC_DIRECTORY_NAME) and
# secrets don't have to be sourced in the shell. python-dotenv is a core dep.
try:
    from dotenv import load_dotenv

    load_dotenv(_PKG_ROOT / ".env")
except ImportError:  # pragma: no cover
    pass

AGENT_MODULE_DIR = _PKG_ROOT / "adk_kb_bot"
EVAL_SET_PATH = _PKG_ROOT / "evals" / "kb_bot_eval_set.json"
EVAL_CONFIG_PATH = _PKG_ROOT / "evals" / "eval_config.json"
REPORT_PATH = _PKG_ROOT / "evals" / "baseline_results.md"
EVAL_HISTORY_DIR = AGENT_MODULE_DIR / ".adk" / "eval_history"

PASS_THRESHOLD = 0.8


def _err(msg: str) -> None:
    sys.stderr.write(f"error: {msg}\n")


def _validate_eval_set() -> tuple[bool, str]:
    try:
        from google.adk.evaluation.eval_set import EvalSet
    except ImportError as exc:
        return False, f"google-adk not installed: {exc}"
    try:
        eval_set = EvalSet.model_validate_json(EVAL_SET_PATH.read_text(encoding="utf-8"))
    except Exception as exc:  # noqa: BLE001
        return False, f"eval set schema validation failed: {exc}"
    return True, f"{eval_set.eval_set_id}: {len(eval_set.eval_cases)} cases"


def _check_env() -> list[str]:
    missing: list[str] = []
    for name in ("AGENTDISK_BASE_URL", "AGENTDISK_API_KEY", "KB_BOT_BUNDLE_ID"):
        if not os.environ.get(name):
            missing.append(name)
    model = os.environ.get("KB_BOT_MODEL", "deepseek/deepseek-chat").lower()
    if "deepseek" in model and not os.environ.get("DEEPSEEK_API_KEY"):
        missing.append("DEEPSEEK_API_KEY")
    elif "gemini" in model and not (
        os.environ.get("GEMINI_API_KEY") or os.environ.get("GOOGLE_API_KEY")
    ):
        missing.append("GEMINI_API_KEY|GOOGLE_API_KEY")
    elif ("claude" in model or "anthropic" in model) and not os.environ.get(
        "ANTHROPIC_API_KEY"
    ):
        missing.append("ANTHROPIC_API_KEY")
    return missing


def _build_adk_cmd(case_ids: list[str] | None, detailed: bool) -> list[str]:
    target = str(EVAL_SET_PATH)
    if case_ids:
        target = f"{target}:{','.join(case_ids)}"
    cmd = [
        sys.executable,
        "-m",
        "google.adk.cli",
        "eval",
        str(AGENT_MODULE_DIR),
        target,
        "--config_file_path",
        str(EVAL_CONFIG_PATH),
    ]
    if detailed:
        cmd.append("--print_detailed_results")
    return cmd


def _find_latest_result() -> Path | None:
    if not EVAL_HISTORY_DIR.exists():
        return None
    files = sorted(EVAL_HISTORY_DIR.glob("*.evalset_result.json"))
    return files[-1] if files else None


def _parse_result_file(result_path: Path) -> list[dict[str, Any]]:
    """Extract per-case rubric pass/fail from the EvalSetResult JSON.

    ADK 2.x stores rubric results under
    ``overall_eval_metric_results[].details.rubric_scores`` (a list of
    ``{rubric_id, score, rationale}``). Each rubric metric emits one entry
    whose ``metric_name`` starts with ``rubric_based_``. We aggregate across
    all such metrics; non-rubric metrics (tool_trajectory_avg_score etc.)
    are ignored because they're strict trajectory matchers, not what this
    suite validates.
    """
    raw = result_path.read_text(encoding="utf-8")
    try:
        data = json.loads(raw)
    except json.JSONDecodeError:
        data = json.loads(json.loads(raw))

    case_results = data.get("evalCaseResults") or data.get("eval_case_results") or []
    rows: list[dict[str, Any]] = []
    for case in case_results:
        eval_id = case.get("evalId") or case.get("eval_id") or "?"
        rubric_scores: list[dict[str, Any]] = []
        for metric in (
            case.get("overallEvalMetricResults")
            or case.get("overall_eval_metric_results")
            or []
        ):
            name = metric.get("metric_name", "")
            if not name.startswith("rubric_based_"):
                continue
            details = metric.get("details") or {}
            scores = details.get("rubricScores") or details.get("rubric_scores") or []
            rubric_scores.extend(scores)

        if rubric_scores:
            # score is None when the auto-rater declined to score (e.g.
            # rubric wasn't applicable to the trajectory). Count those as
            # not-passed.
            n_pass = sum(
                1
                for r in rubric_scores
                if isinstance(r.get("score"), (int, float)) and float(r["score"]) >= 1.0
            )
            n_total = len(rubric_scores)
            score = n_pass / n_total if n_total else 0.0
            passed = score >= PASS_THRESHOLD
        else:
            score = None
            passed = False

        rows.append(
            {
                "eval_id": eval_id,
                "score": score,
                "rubric_count": len(rubric_scores),
                "passed": passed,
            }
        )
    return rows


def _render_report(
    schema_summary: str,
    precondition: dict[str, Any],
    cases: list[dict[str, Any]],
    elapsed_s: float,
    dry_run: bool,
) -> str:
    lines: list[str] = []
    lines.append("# KB Bot — Baseline Eval Results")
    lines.append("")
    lines.append(f"**Generated:** {time.strftime('%Y-%m-%d %H:%M:%S %Z')}  ")
    lines.append(f"**Eval set:** `{EVAL_SET_PATH.relative_to(_PKG_ROOT)}`  ")
    lines.append(f"**Schema check:** {schema_summary}  ")
    lines.append(f"**Model:** `{os.environ.get('KB_BOT_MODEL', 'deepseek/deepseek-chat')}`  ")
    lines.append(f"**Bundle:** id={precondition.get('bundle_id')} "
                 f"nodes={precondition.get('node_count', 0)}  ")
    if dry_run:
        lines.append("**Mode:** dry-run (no LLM calls)  ")
    else:
        lines.append(f"**Elapsed:** {elapsed_s:.1f}s  ")
    lines.append("")

    if not precondition.get("ok"):
        lines.append("## ⚠ Precondition check failed")
        lines.append("")
        lines.append(f"```\n{precondition.get('error')}\n```")
        lines.append("")
        lines.append("Eval results below may be misleading — fix this first.")
        lines.append("")

    if not cases:
        lines.append("_No case results parsed — see schema check + run output above._")
        return "\n".join(lines)

    lines.append("## Per-case scores")
    lines.append("")
    lines.append("| Case | Rubric score | Rubric count | Status |")
    lines.append("|---|---|---|---|")
    for c in cases:
        score_str = f"{c['score']:.2f}" if c["score"] is not None else "—"
        status = "PASS" if c["passed"] else "FAIL"
        lines.append(
            f"| `{c['eval_id']}` | {score_str} | {c['rubric_count']} | {status} |"
        )
    lines.append("")

    n_pass = sum(1 for c in cases if c["passed"])
    n_fail = len(cases) - n_pass
    pct = n_pass * 100 // max(1, len(cases))
    lines.append(f"**Summary:** {n_pass}/{len(cases)} pass ({pct}%), {n_fail} fail.")
    lines.append("")
    lines.append("---")
    lines.append("*Regenerated on every `make kb-bot-eval` run.*")
    return "\n".join(lines)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("case_ids", nargs="*", help="Optional subset of eval_id values.")
    parser.add_argument("--dry-run", action="store_true")
    parser.add_argument("--print-detailed-results", action="store_true")
    parser.add_argument("--no-report", action="store_true")
    args = parser.parse_args()

    ok, schema_summary = _validate_eval_set()
    if not ok:
        _err(schema_summary)
        return 2
    print(f"[ok] {schema_summary}")

    missing = _check_env()
    if missing:
        _err("missing required env vars: " + ", ".join(missing))
        if not args.dry_run:
            return 3
    else:
        print("[ok] env vars present")

    from evals.eval_setup import check_preconditions  # type: ignore[import-not-found]

    precondition = check_preconditions()
    if precondition["ok"]:
        print(
            f"[ok] bundle_id={precondition['bundle_id']} "
            f"node_count={precondition['node_count']}"
        )
    elif not args.dry_run:
        _err(f"precondition check failed: {precondition['error']}")
        return 7

    if args.dry_run:
        print("[dry-run] skipping adk eval invocation")
        report = _render_report(
            schema_summary=schema_summary,
            precondition=precondition,
            cases=[],
            elapsed_s=0.0,
            dry_run=True,
        )
        if not args.no_report:
            REPORT_PATH.write_text(report, encoding="utf-8")
            print(f"[ok] dry-run stub report → {REPORT_PATH}")
        return 0

    cmd = _build_adk_cmd(args.case_ids, args.print_detailed_results)
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

    result_path = _find_latest_result()
    if result_path is None:
        _err(f"no eval result file found under {EVAL_HISTORY_DIR}")
        return 5
    print(f"[ok] result → {result_path.relative_to(_PKG_ROOT)}")

    try:
        rows = _parse_result_file(result_path)
    except Exception as exc:  # noqa: BLE001
        _err(f"failed to parse result file: {exc}")
        return 6

    report = _render_report(
        schema_summary=schema_summary,
        precondition=precondition,
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
