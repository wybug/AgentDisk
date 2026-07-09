# KB Bot — Evaluation Suite

This directory holds the **ADK eval** suite for `adk_kb_bot`, the OKF knowledge-base
Q&A consumer agent. The suite is **read-only** — unlike the writer's eval, no state
gets wiped between cases because kb_bot consumes a pre-built bundle.

## What gets evaluated

`kb_bot_eval_set.json` — 4 cases covering the three behaviors that define a
trustworthy grounded bot:

| Case | Asserts that the agent... |
|---|---|
| `search_first_for_business_question` | Calls `search_knowledge` before any other OKF read tool (the INSTRUCTION mandates search-first) |
| `cite_source_when_answered` | Final response contains at least one `[nodeId=N, relPath=...]` citation; doesn't invent article numbers |
| `refuse_when_kb_uncovered` | For an out-of-scope question, explicitly says "知识库未覆盖" and doesn't invent a statute |
| `expand_via_neighbors_when_thin` | Calls `neighbors` / `reachable` after search to expand multi-hop questions |

Rubric-based scoring — `response_match_score` is informational because LLM
phrasing varies. Pass/fail comes from whether the rubric criteria are met
(citation present / explicit refusal / etc.).

## Prerequisites

1. **Backend running**: `make dev-start` from repo root.
2. **Bundle exists with sample data**: easiest path is
   `cd ../adk_writer_agent && python -m scenarios.bootstrap_bundle` (writes
   `index.md` + `concepts/gemma.md` and registers a bundle). For fuller
   coverage (反洗钱 / 客户身份识别), run the writer's full bulk-import:
   `python scripts/import_raw_phase1.py && python scripts/import_raw_phase2.py`.
3. **`.env` configured**: see `.env.example`. Note `KB_BOT_BUNDLE_ID` must
   match the bundle id from step 2.

## Run

```bash
make kb-bot-eval                 # via Makefile
# or directly:
pip install -e ".[eval]"
python evals/run_evals.py
```

Subset / verbose:

```bash
python evals/run_evals.py cite_source_when_answered
python evals/run_evals.py --print-detailed-results
python evals/run_evals.py --dry-run       # validate schema + env, no LLM
```

## Output

`baseline_results.md` is regenerated on every run. Commit it to track
baseline drift as the prompt / tools / model change.

## Extending

Add cases by appending to `kb_bot_eval_set.json`. Each case has:

- `eval_id`: stable identifier (used by `--print-detailed-results` and for
  running a subset).
- `conversation`: one user turn (multi-turn flows are possible but the
  bot is single-shot today).
- `rubrics`: list of structural criteria. Prefer rubric over trajectory
  match — LLM responses are inherently non-deterministic, and the goal
  is to validate the *contract* (cite sources / refuse when uncovered),
  not exact phrasing.

See `adk_writer_agent/evals/README.md` for a fuller treatment of the ADK
eval framework's quirks (some apply, some don't — the writer's suite is
trajectory-heavy because writes are deterministic; ours is rubric-heavy
because reads aren't).
