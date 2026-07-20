# PR title

feat(okf): OKF v0.1 integration — bundle API + knowledge graph + share/preview + SDK + UI

# PR body

## Summary

Full OKF (Open Knowledge Format) v0.1 integration across backend, SDK, frontend, docs, and tests. 37 commits organized as 13 sub-PRs landing on `feature/okf-integration` since the last main (`bb666d9`), plus a raw → OKF initialization + batch-import + eval-suite top-up after PR-okf-eval landed. A subsequent product-dimension review drove a **hardening & enhancement pass (Tier 1–3, +21 commits)** — see the section at the end of this body.

### What's in scope

- **Backend (Go)**
  - `pkg/okf` — v0.1 spec parser/writer utilities
  - Bundle registration + reader/writer APIs (PR-B)
  - Edge materialization on WriteMarkdown (PR-P3a)
  - Graph queries: reachable, shortest-path, subgraph, neighbors + Redis adjacency cache + rebuild-graph (PR-P3b)
  - FTS5 (SQLite) + ngram FULLTEXT (MySQL) full-text search (PR-P3c)
  - Link scanning + auto index/log + bundle lock + ACL fixes (PR-P2, PR-P3a)
  - Bundle sharing via existing share resType + read-only recipient view (PR-P4a)
  - Field-level feature flags: `internal/feature` (atomic.Pointer + fsnotify) + admin API + middleware gate (PR-P5c.2)
- **SDK (Python)**
  - Wiki module: reader + writer (PR-P5a)
  - Async client parity
- **Frontend (Web)**
  - Bundle list, node browser, Cytoscape graph, drawer (PR-P4b+c)
  - OKF share view
- **Docs**
  - VitePress site under `docs/site/architecture/okf.md` + OpenAPI coverage (PR-P5b)
- **Tests & benchmarks**
  - BFS benchmarks: 1K/10K/100K node sweep, cold vs warm cache, uniform/powerlaw topology (PR-P5c.3)
  - Baseline: 100K Reachable d=3 ≈ 5.8ms (target was 500ms)
  - Protocol-level e2e: Python `sdk/tests/e2e/` + browser `t24-e2e-protocol.js` (PR-P5c.4)
  - Browser t22 (bundle UI), t23 (bundle share)
  - Demo: Google ADK writer agent (`examples/adk_writer_agent/`)

### New since the prior draft (2026-07-07)

- `5234649 feat(okf): adk_writer_agent raw → OKF initialization + graph retrieval` — two new agent tools (`list_raw_files`, `read_raw_file`) + ~70 lines of INSTRUCTION covering two-phase write discipline, filename → type inference, and bundle registration order.
- `44061df feat(okf): batch-import script for raw → bundle 21` — `examples/adk_writer_agent/scripts/import_raw_phase1.py` walks `raw/` (96 files), strips frontmatter / dedupes same-line links / quotes YAML titles, writes phase-1 markdown (no bundle-internal links yet). End state: bundle 21 = 97 nodes / 865 edges.
- `c67e103 test(okf): extend eval suite to cover raw-import workflow (8→14 cases)` — six new ADK eval cases targeting the raw workflow decision points. Baseline: 12/14 pass on `deepseek/deepseek-v4-flash`; the two known failures (`bundle_registration_order`, `infer_type_from_filename`) are eval-set strictness limits, not agent bugs — their rubrics still validate the real constraints. See `examples/adk_writer_agent/evals/baseline_results.md`.
- `826e118 feat(okf): phase-2 citation links + broken-link strip for bundle 21` — `scripts/import_raw_phase2.py` (scan + apply modes) injects verified `[《XX》](./target.md)` citation links (142 candidates above the 0.78 SequenceMatcher threshold) and strips residual broken markdown links (461 instances across 41 files). End state: bundle 21 = 101 nodes / 251 live edges / 0 broken bundle-relative links per `/broken-links` endpoint. The `edgeBroken=740` stat counts external/anchor link rows, which aren't bundle-broken.

### Hardening & enhancements (2026-07-15 → 07-20, +21 commits)

A three-way product-dimension review (backend / frontend / SDK-docs-quality, ~60 findings) drove a follow-up pass on top of v0.1. Full backlog + completion status in `OKF v0.1 改进清单（产品维度评估 · 团队排期）.md`; all gates green throughout.

- **Tier 1 hardening (5 modules)**: wired the dead Redis graph-adjacency cache (`SetGraphCache` was never called — BFS hit the DB every hop); propagated SDK credentials to the `_wiki` sub-client (a JWT→API-Key switch silently kept the old token → 401); mapped lock-held to 409 + `Retry-After` (was falling through to 500); reset the browser-test driver between cases (fixed the cascading `spawnSync ETIMEDOUT` that hung t19/t22/t23 for ~1000s each on full-suite runs); corrected non-existent SDK methods in `sdk-python.md` + enabled VitePress dead-link checking; styled + sanitized the share markdown (`.markdown-body` + URL allowlist); real broken-links `total`; fixed the vacuous Cytoscape-mount assertion; etc.
- **Tier 2 product gaps**: search end-to-end — the search API had **zero frontend call sites** (wired a search panel + indexed node body for full-text match + bm25 relevance ranking + offset pagination); perf trio (WriteMarkdown edge-materialization N+1 → batched lookup, RefreshBundle double OSS read → single pass, ListBrokenLinks per-node body scan → materialized edge table); node-list server-side pagination ("load more"); bundle-list search/sort; OKF end-user guide + SDK pagination iterators (`iter_search`/`iter_broken_links`); graph edge labels + type→color legend.
- **Tier 3 (started)**: observability foundation (`InternalError` now logs the real detail + a request id; silently-swallowed best-effort failures — `AppendLogEntry`, `scheduleIndexRegen`, etc. — are logged) + a defensive cross-bundle guard on BFS results.

### Wire format / ops

- Config: new `features:` block in `config.yaml` (defaults to true), `cfg.Okf.Enabled` still gates boot-time registration
- Admin API: `GET/PATCH /v1/disk/admin/features` flips runtime flags
- fsnotify watches `config.yaml` for external edits (200ms debounce)
- No DB migrations required — schema additive only

## Test plan

- [x] `go test -race -count=1 ./...` — all green
- [x] `make lint` — 0 issues
- [x] `make sdk-check` (ruff + format + mypy) — clean
- [x] `cd test/browser && node runner.js t22 t23 t24 t11` — all pass
- [x] Full browser suite `node runner.js` (28 auto cases) + t25 (search UI) — 29/29, no hangs (runner resets the driver between cases)
- [x] `cd sdk && python -m pytest tests/` — 91/91
- [x] `make okf-eval` — 12/14 cases pass (2 known eval-strictness fails; documented in `examples/adk_writer_agent/evals/README.md`)
- [x] Smoke: `bash scripts/dev.sh start` then `curl -H "Authorization: Bearer $ADMIN_JWT" http://localhost:9100/v1/disk/admin/features`
- [x] Manual share preview flow via web UI

### Reviewer notes

- The PR is intentionally large because the OKF milestones were developed as a sequence on `feature/okf-integration`. The 13 sub-PR merges are preserved as merge commits (`Merge PR-X: ...`) — review commit-by-commit if the diff is too big to grok as one.
- `config.Load` was refactored to use `viper.New()` instead of package-level viper calls — concurrent `Load` (e.g. fsnotify reload from `internal/feature`) was racing on viper's global state. The fix is in commit `2f71022`.
- The `cloudDisk` field originally planned for P5c.1 was dropped — the entire system is already AgentDisk-scoped, the field was redundant.
- Phase-2 citation-link pass landed in `826e118`. Bundle 21 now has 251 live edges (was 113 after phase 1). The `edgeBroken=740` stat is misleading — it counts every edge with `dst_exists=false`, including external/anchor link rows that aren't bundle-relative. The authoritative broken-link count comes from `GET /v1/disk/okf/bundles/21/broken-links`, which is 0.
- The 142 auto-apply citations were picked by `SequenceMatcher` fuzzy match (≥0.78) on normalized titles; the 180 review-tier candidates (0.55–0.78) are documented in `phase2_scan.md` but not applied.
