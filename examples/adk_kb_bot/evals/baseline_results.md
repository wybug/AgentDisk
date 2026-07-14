# KB Bot — Baseline Eval Results

**Generated:** 2026-07-13 20:50:11 CST  
**Eval set:** `evals/kb_bot_eval_set.json`  
**Schema check:** kb_bot_baseline: 4 cases  
**Model:** `deepseek/deepseek-chat`  
**Bundle:** id=24 nodes=87  
**Elapsed:** 100.6s  

## Per-case scores

| Case | Rubric score | Rubric count | Status |
|---|---|---|---|
| `refuse_when_kb_uncovered` | 1.00 | 4 | PASS |
| `search_first_for_business_question` | 1.00 | 3 | PASS |
| `cite_source_when_answered` | 0.75 | 4 | FAIL |
| `expand_via_neighbors_when_thin` | 1.00 | 3 | PASS |

**Summary:** 3/4 pass (75%), 1 fail.

---
*Regenerated on every `make kb-bot-eval` run.*