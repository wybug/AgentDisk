# OKF Writer Agent — Evaluation Suite

This directory holds the **ADK eval** suite for the OKF v0.1 maintenance agent.
The point is to give us regression protection when the prompt, tools, or model
change — not to chase a 100% pass rate. Failures surface concrete
improvement opportunities.

## What gets evaluated

The suite is `okf_writer_eval_set.json` — 8 cases covering the agent's main
decision points:

| Case | Asserts that the agent... |
|---|---|
| `register_when_missing` | Calls `register_bundle` before `list_nodes` |
| `write_concept_with_frontmatter` | Emits YAML frontmatter with at least a `type` field |
| `refresh_after_writes` | Calls `refresh_index` when told writes happened out-of-band |
| `multi_folder_organization` | Creates `concepts/` and `playbooks/` folders before writing into them |
| `internal_link_format` | Uses bundle-relative links (`./gemma.md`), not absolute URLs |
| `noop_clarification` | Doesn't call any tool for an ambiguous "你好" |
| `list_nodes_with_type_filter` | Uses the `type=` filter rather than unfiltered listing |
| `aggregate_for_overview` | Calls `aggregate_types()` for a type-distribution overview |

## How to run

```bash
# 1. Start the backend + make sure you have a public directory
make dev-start

# 2. Configure env (one-time)
cd examples/adk_writer_agent
cp .env.example .env
# fill in AGENTDISK_API_KEY, AGENTDISK_PUBLIC_DIRECTORY_ID, and one of the LLM keys

# 3. Run the whole suite
make okf-eval             # from repo root
# or: python evals/run_evals.py

# 4. Run a single case (faster iteration)
python evals/run_evals.py register_when_missing noop_clarification

# 5. Validate the suite without spending LLM tokens
make okf-eval-dry-run
```

`make okf-eval` does, in order:

1. `pip install -e ".[eval]"` (idempotent — only reinstalls when source changes).
2. Validates `okf_writer_eval_set.json` (the **template**) against the ADK
   pydantic schema.
3. Checks that all required env vars are present.
4. Pre-cleans OKF state via `clean_okf_state()` so a fresh bundle exists.
   Its `bundle_id` is persisted to `${TMPDIR}/okf_eval_bundle_state.json`.
5. Resolves the PD id from env (direct `AGENTDISK_PUBLIC_DIRECTORY_ID` or
   `AGENTDISK_PUBLIC_DIRECTORY_PATH` lookup).
6. Renders the parameterized template into `okf_writer_eval_set.concrete.json`
   by substituting `{{PD_ID}}` and `{{BUNDLE_ID}}` placeholders.
7. Invokes `adk eval adk_writer_agent evals/okf_writer_eval_set.concrete.json`.
8. Locates the latest `*.evalset_result.json` written by ADK.
9. Parses the result, categorizes failures, and writes `baseline_results.md`.

## Reading the report

`baseline_results.md` (generated next to this README) has three sections:

- **Per-case scores** — a table with `tool_trajectory_avg_score` (1.0 = perfect
  match against expected tool calls) and `response_match_score` (0.8 default
  threshold for text similarity). The response score is **informational only**
  by default — the eval set has no `final_response` to match against (LLM
  output is non-deterministic), so it's typically 0 for every case. Pass/fail
  is decided by trajectory score unless at least one case has a non-zero
  response score.
- **Improvement opportunities** — failed cases grouped by failure category:
  - `missing_tool(X)` — agent skipped tool X. Prompt needs to make the step
    explicit.
  - `extra_tool(X)` — agent called tool X when it shouldn't have. Prompt needs
    a "do not call X unless Y" clause.
  - `duplicate_tool(X)` — agent called the same tool twice with no new
    information. Real agent control-flow bug.
  - `arg_mismatch(X.k)` — same tool names in same order, but the value of
    argument `k` differs. Usually the `content` field of `write_markdown`
    (LLM wrote more than the eval set's minimal seed). Either relax the eval
    set to use `IN_ORDER` + rubrics, or tighten the prompt.
  - `spurious_tool_call` — agent produced a tool call when the prompt expected
    zero. Often a "be helpful" reflex; tighten the prompt.
  - `no_data` — neither side produced tool calls. Could be a PASS for
    `noop_clarification`; check the per-case detail.
- **Suggested next steps** — one-line action items based on which categories
  are present.

## How state isolation works

ADK's `cli_eval.try_get_reset_func` looks up `agent.reset_data` on the agent
submodule. We define it in `adk_writer_agent/agent.py`, which delegates to
`evals/eval_setup.py:clean_okf_state`. Before every case, `clean_okf_state`:

1. Reads the persisted bundle id from `${TMPDIR}/okf_eval_bundle_state.json`.
   If it exists AND the server still has that bundle, **reuses it** so the
   `bundle_id` stays stable across cases in the same eval run (critical
   because `{{BUNDLE_ID}}` is substituted once at eval set build time).
2. Otherwise: deletes every existing bundle, writes a minimal `index.md`
   (required so the PD qualifies as an OKF bundle root), and registers ONE
   fresh bundle.
3. Always wipes PD files so cases don't see each other's writes, then
   re-seeds `index.md` so the bundle root stays valid.

Failures during cleanup are logged + swallowed so the eval run completes; a
dirty state will surface as unexpected tool trajectories (false failures with
obvious diagnostics, which is preferable to aborting).

The `AGENTDISK_PUBLIC_DIRECTORY_ID` (or `AGENTDISK_PUBLIC_DIRECTORY_PATH`) is
NOT recreated between cases — point it at a dedicated test directory, not a
production one. PATH form is convenient when you only have the UI path; it's
resolved once on first use and cached.

## Adding a new case

1. **Decide what you're testing.** Pick a single decision point — cases that
   bundle multiple decisions are hard to triage when they fail.
2. **Add a stanza to `okf_writer_eval_set.json`.** Each case looks like:

   ```json
   {
     "eval_id": "your_case_id",
     "creation_timestamp": 0.0,
     "session_input": {
       "app_name": "agentdisk_writer",
       "user_id": "okf_eval_user",
       "state": {}
     },
     "conversation": [
       {
         "invocation_id": "your_case_id_1",
         "creation_timestamp": 0.0,
         "user_content": {
           "parts": [{ "text": "the user prompt goes here" }]
         },
         "intermediate_data": {
           "tool_uses": [
             {
               "name": "tool_function_name",
               "args": {"param": "value"}
             }
           ],
           "tool_responses": [],
           "intermediate_responses": []
         },
         "final_response": null,
         "rubrics": null,
         "app_details": null
       }
     ],
     "rubrics": [
       {
         "rubric_id": "short_id",
         "rubric_content": {
           "text_property": "What a correct agent response should look like."
         },
         "type": "TOOL_USE_QUALITY"
       }
     ],
     "final_session_state": null
   }
   ```

3. **Validate without spending tokens:** `python evals/run_evals.py --dry-run`.
   The schema check will catch typos in field names.
4. **Run just the new case:** `python evals/run_evals.py your_case_id`.
5. **Commit the updated `baseline_results.md`** so future PRs can compare.

### Gotchas

- **Dynamic IDs.** Use the `{{PD_ID}}` and `{{BUNDLE_ID}}` placeholders in
  prompts and expected args — `run_evals.py` substitutes them at render time.
  Hardcoding a specific PD or bundle id makes the case non-portable across
  machines / fresh test directories. The PD id is resolved from
  `AGENTDISK_PUBLIC_DIRECTORY_ID` or `AGENTDISK_PUBLIC_DIRECTORY_PATH`; the
  bundle id comes from `clean_okf_state()` pre-registering one fresh bundle
  and persisting it to `${TMPDIR}/okf_eval_bundle_state.json`.
- **Content verbosity.** ADK's default `tool_trajectory_avg_score` uses
  `EXACT` match on args. The agent's `write_markdown` content will rarely
  match the eval set's minimal seed verbatim. Either accept this as a known
  failure (the baseline report flags it as `arg_mismatch(write_markdown.content)`),
  or relax the eval set to use `IN_ORDER` match + rubrics that check for
  required frontmatter keys instead of exact content.
- **LLM determinism.** Same case may score differently across runs. Re-run a
  flaky case 2-3 times before treating it as a real regression.
- **Cost.** Each case ≈ 1 LLM call. The full suite is ~8 calls; budget
  accordingly. Use `deepseek/deepseek-v4-flash` (or `gemini-2.5-flash`) for
  development iterations, `deepseek/deepseek-v4-pro` (or `gemini-2.5-pro`)
  for the committed baseline.

## Files in this directory

| File | Purpose |
|---|---|
| `okf_writer_eval_set.json` | The eval cases themselves (ADK EvalSet schema, **template** with `{{PD_ID}}` / `{{BUNDLE_ID}}` placeholders) |
| `okf_writer_eval_set.concrete.json` | Generated by `run_evals.py` at runtime — never edit by hand, do not commit |
| `eval_setup.py` | `clean_okf_state()` — invoked by `agent.reset_data` before each case; also writes bundle_id to `${TMPDIR}/okf_eval_bundle_state.json` |
| `run_evals.py` | Wrapper that resolves placeholders, renders the concrete eval set, invokes `adk eval`, and emits `baseline_results.md` |
| `baseline_results.md` | Generated — latest run's scores + improvement suggestions |
| `README.md` | This file |
| `__init__.py` | Marks the dir as a Python package (so `from evals.eval_setup import ...` works from the agent) |

## What this suite does NOT do

- It does not evaluate the OKF backend itself (that's covered by Go tests).
- It does not test the AgentDisk Python SDK (`sdk/`).
- It does not run in CI — evals need a live backend + real LLM key, so they're
  a manual / on-demand check.
- It does not yet support multi-turn conversations — each case is a single
  user message + agent response. Add a second `invocation` stanza if you need
  multi-turn coverage.
