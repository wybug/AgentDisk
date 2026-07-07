# OKF Writer Agent — Baseline Eval Results

**Generated:** 2026-07-07 13:37:26 CST  
**Eval set:** `evals/okf_writer_eval_set.json`  
**Schema check:** okf_writer_baseline: 14 cases  
**Model:** `deepseek/deepseek-v4-flash`  
**Elapsed:** 79.4s  

## Per-case scores

> Note: ``response_match_score`` is informational only — the eval set has no ``final_response`` to match against (LLM output is non-deterministic). Pass/fail is decided by trajectory score.

| Case | Trajectory | Response | Status | Failure category |
|---|---|---|---|---|
| `read_named_raw_file` | 1.00 | 0.00 | PASS | ok |
| `bundle_id_not_swapped_to_pd` | 1.00 | 0.00 | PASS | ok |
| `aggregate_for_overview` | 1.00 | 0.00 | PASS | ok |
| `multi_folder_organization` | 1.00 | 0.00 | PASS | ok |
| `bundle_registration_order` | 0.00 | 0.00 | FAIL | arg_mismatch(write_markdown.content) |
| `chunk_large_raw_file` | 1.00 | 0.00 | PASS | ok |
| `scan_raw_root` | 1.00 | 0.00 | PASS | ok |
| `write_concept_with_frontmatter` | 1.00 | 0.00 | PASS | ok |
| `infer_type_from_filename` | 0.00 | 0.00 | FAIL | extra_tool(create_folder,read_raw_file) |
| `noop_clarification` | 1.00 | 0.00 | PASS | no_data |
| `register_when_missing` | 1.00 | 0.00 | PASS | ok |
| `refresh_after_writes` | 1.00 | 0.00 | PASS | ok |
| `list_nodes_with_type_filter` | 1.00 | 0.00 | PASS | ok |
| `internal_link_format` | 1.00 | 0.00 | PASS | ok |

**Summary:** 12/14 pass (85%), 2 fail.

## Improvement opportunities

Failed cases — sorted by category so you can pick a fix strategy.

### arg_mismatch(write_markdown.content)

#### `bundle_registration_order`

- **Expected trajectory:** write_markdown(rel_path="index.md", public_directory_id=61, content="---\nokf_version: \"0.1\"\...) → register_bundle(public_directory_id=61)
- **Actual trajectory:** write_markdown(rel_path="index.md", content="---\nokf_version: \"0.1\"\..., public_directory_id=61) → register_bundle(public_directory_id=61)
- **Trajectory score:** 0.00
- **Response score:** 0.00

### extra_tool(create_folder,read_raw_file)

#### `infer_type_from_filename`

- **Expected trajectory:** write_markdown(rel_path="机构监管/金融机构客户尽职调查和客户身份资料及交易记..., public_directory_id=61, content="---\ntype: regulation\n---\n")
- **Actual trajectory:** read_raw_file(path="机构监管/金融机构客户尽职调查和客户身份资料及交易记...) → create_folder(folder_name="机构监管", public_directory_id=61) → write_markdown(rel_path="机构监管/金融机构客户尽职调查和客户身份资料及交易记..., content="---\ntype: regulation\ntit..., public_directory_id=61)
- **Trajectory score:** 0.00
- **Response score:** 0.00

## Suggested next steps

- **Extra tool calls** — agent is being overly cautious. Tighten the prompt to say 'do not call X unless Y' (e.g. 'do not register_bundle if the prompt mentions an existing bundle_id').
- **Argument mismatch (content verbosity)** — the agent's ``write_markdown`` content is longer than the eval set's minimal expected content. This is by design (LLMs naturally expand prompts). Two fix paths: (a) relax the eval set to use ``IN_ORDER`` match + ``rubrics`` that check for required frontmatter keys instead of exact content; (b) tighten the prompt to say '回复尽量短，只写最小内容'.

---
*This report is regenerated on every `make okf-eval` run. Commit it
to track baseline drift over time.*