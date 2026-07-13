# KB Bot — Baseline Eval Results

**Generated:** 2026-07-13 18:41:29 CST  
**Eval set:** `evals/kb_bot_eval_set.json`  
**Schema check:** kb_bot_baseline: 4 cases  
**Model:** `deepseek/deepseek-chat`  
**Bundle:** id=22 nodes=6  
**Elapsed:** 56.1s  

## Per-case scores

| Case | Rubric score | Rubric count | Status |
|---|---|---|---|
| `refuse_when_kb_uncovered` | 1.00 | 1 | PASS |
| `expand_via_neighbors_when_thin` | 0.67 | 3 | FAIL |
| `cite_source_when_answered` | 0.75 | 4 | FAIL |
| `search_first_for_business_question` | 1.00 | 3 | PASS |

**Summary:** 2/4 pass (50%), 2 fail.

---
*Regenerated on every `make kb-bot-eval` run.*