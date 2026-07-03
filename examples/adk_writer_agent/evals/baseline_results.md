# OKF Writer Agent — Baseline Eval Results

**Generated:** 2026-07-03 16:24:59 CST  
**Eval set:** `evals/okf_writer_eval_set.json`  
**Schema check:** okf_writer_baseline: 8 cases  
**Model:** `deepseek/deepseek-v4-flash`  
**Elapsed:** 24.3s  

## Per-case scores

> Note: ``response_match_score`` is informational only — the eval set has no ``final_response`` to match against (LLM output is non-deterministic). Pass/fail is decided by trajectory score.

| Case | Trajectory | Response | Status | Failure category |
|---|---|---|---|---|
| `aggregate_for_overview` | 1.00 | 0.00 | PASS | ok |
| `noop_clarification` | 1.00 | 0.00 | PASS | no_data |
| `write_concept_with_frontmatter` | 1.00 | 0.00 | PASS | ok |
| `list_nodes_with_type_filter` | 1.00 | 0.00 | PASS | ok |
| `multi_folder_organization` | 1.00 | 0.00 | PASS | ok |
| `refresh_after_writes` | 1.00 | 0.00 | PASS | ok |
| `register_when_missing` | 1.00 | 0.00 | PASS | ok |
| `internal_link_format` | 1.00 | 0.00 | PASS | ok |

**Summary:** 8/8 pass (100%), 0 fail.

## No improvement opportunities

All cases passed the 0.8 threshold. Re-run after changing the prompt
or tools to catch regressions.