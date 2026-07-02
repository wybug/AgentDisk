# BFS Performance Baseline

This document records the **algorithm-only** performance baseline for the OKF
graph walks (`Reachable`, `Subgraph`, `ShortestPath`, `Neighbors`). Numbers
are taken from `internal/service/okf_bfs_bench_test.go` running against the
in-memory fake repos, so they measure pure BFS cost without DB I/O or Redis
roundtrips. Use them as a regression guard and a sanity check against the
end-to-end target.

The **end-to-end** target recorded in `docs/site/architecture/okf.md` is
P95 < 500 ms for `Reachable depth=3` at 100 K nodes / 500 K edges. The
algorithm portion measured here is < 10 ms at that scale, so the remaining
budget is consumed by DB + Redis + JSON encode + network. If the algorithm
portion rises above 50 ms at 100 K, investigate before chasing the DB layer.

## How to reproduce

```bash
# Quick single-iter smoke (≈ 2s)
go test -bench=. -run=^$ -benchtime=1x ./internal/service/

# Stable baseline (≈ 30s) — what the numbers below were taken with
go test -bench=. -run=^$ -benchtime=10x -count=3 ./internal/service/ \
  | tee docs/perf/bfs-baseline-raw.txt

# Cross-branch comparison
go test -bench=. -count=5 -run=^$ ./internal/service/ > old.txt
# (change code)
go test -bench=. -count=5 -run=^$ ./internal/service/ > new.txt
benchstat old.txt new.txt
```

A helper script is provided: `bash scripts/bench-bfs.sh` runs the stable
configuration and writes to `docs/perf/bfs-latest.txt`.

## Hardware reference

- Apple M4 (8 cores), 24 GB RAM
- Go 1.25.0
- macOS Darwin 25.5.0
- Single-iteration benches are noisy; rely on the `benchtime=10x -count=3`
  medians below for trend analysis.

## Algorithm-only baselines (fake repos, no DB/Redis)

All numbers are ns/op, median of 3 runs at `benchtime=10x`. Raw output:
`docs/perf/bfs-baseline-raw.txt`.

### Reachable (BFS, depth 1/2/3)

| Nodes | Dist      | d=1     | d=2     | d=3     |
|-------|-----------|---------|---------|---------|
| 1 K   | uniform   | 21 µs   | 52 µs   | 130 µs  |
| 1 K   | powerlaw  | 58 µs   | 216 µs  | 336 µs  |
| 10 K  | uniform   | 140 µs  | 318 µs  | 770 µs  |
| 10 K  | powerlaw  | 143 µs  | 307 µs  | 501 µs  |
| 100 K | uniform   | 1.27 ms | 2.75 ms | 5.8 ms  |
| 100 K | powerlaw  | 1.26 ms | 2.85 ms | 5.1 ms  |

**Observation**: Cost is dominated by `MaxNodes` (frontier cap at 1000) once
depth ≥ 2 — the depth=2 → depth=3 jump is only ~2x because the visited set
saturates. The depth=1 → depth=2 jump is ~5-10x, reflecting the first
frontier expansion (`edgesPerNode` × N reached nodes).

Powerlaw is **cheaper** at depth=2/3 because the Zipf skew sends most edges
to a small hub set that the visited set already covers — the walk dies
early.

### Subgraph (bulk type-filtered extract)

| Nodes | Dist     | Time    | Bytes/op   |
|-------|----------|---------|------------|
| 1 K   | uniform  | 1.23 ms | 4.5 MB     |
| 1 K   | powerlaw | 1.20 ms | 4.5 MB     |
| 10 K  | uniform  | 5.0 ms  | 14.2 MB    |
| 10 K  | powerlaw | 4.9 ms  | 14.2 MB    |
| 100 K | uniform  | 62 ms   | 123 MB     |
| 100 K | powerlaw | 60 ms   | 123 MB     |

**Observation**: `Subgraph` scales O(N) — it loads every node of the
requested type before applying `MaxNodes`. The 100 K numbers reflect 1000
nodes worth of edge data + node rows being copied into the response. If
real-world Subgraph calls surface as slow, push the `MaxNodes` cap into the
SQL query rather than slicing in Go.

### ShortestPath (bidirectional BFS)

| Nodes | Dist     | Case   | Time    |
|-------|----------|--------|---------|
| 1 K   | uniform  | found  | 146 µs  |
| 1 K   | uniform  | nopath | 53 µs   |
| 10 K  | uniform  | found  | 990 µs  |
| 10 K  | uniform  | nopath | 142 µs  |
| 100 K | uniform  | found  | 25 ms   |
| 100 K | uniform  | nopath | 1.3 ms  |

**Observation**: The `found` case is more expensive because both frontiers
expand fully until they meet — for a 100 K-node graph, finding the path can
visit ~500 nodes per side. The `nopath` case (MaxDepth=1) is bounded by the
1-hop cap.

### Neighbors (1-hop baseline)

| Nodes | Dist     | Time    |
|-------|----------|---------|
| 1 K   | uniform  | 58 µs   |
| 10 K  | uniform  | 142 µs  |
| 100 K | uniform  | 1.3 ms  |

**Observation**: Cost is dominated by `loadNodeAndBundle` (2 sequential
queries per call). Even at 100 K nodes the work is bounded by the source's
out-degree (capped by `edgesPerNode` in the bench), not the graph size.

## End-to-end target

The numbers above are **algorithm-only**. The user-facing target is
documented in `docs/site/architecture/okf.md`:

> At 100 K nodes / 500 K edges per bundle:
> - `Neighbors`: P50 < 10 ms
> - `Reachable depth=3`: P50 < 100 ms, P95 < 500 ms
> - `ShortestPath`: P50 < 80 ms

The algorithm portion (this bench) is < 10 ms at 100 K. The remaining
budget is consumed by DB I/O, Redis cache misses, and response encoding.
To validate end-to-end:

```bash
bash scripts/dev.sh start
python3 scripts/seed_okf_large.py --nodes 100000 --edges-per-node 5 --dist uniform
# Then drive the API with a load tester (k6 / hey) against /v1/disk/okf/bundles/:id/reachable
```

## Regressions

If a code change slows these benches by > 20 % at the 100 K scale, treat it
as a regression. The likely culprits, in order of historical impact:

1. **Lost batched adjacency** — if `batchAdjacency` regresses to per-node
   queries, depth=3 walks become O(N) DB roundtrips instead of O(depth).
2. **Cache miss path** — if `RedisGraphCache` GetAdj returns errors as
   misses, every hop hits the DB. Check the cache hit rate via Redis stats.
3. **`loadNodeAndBundle` N+1** — if the node-loading path stops using
   `ListByIDs`, every BFS entry point adds a sequential pair of queries.
