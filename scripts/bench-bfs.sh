#!/usr/bin/env bash
# Run the OKF BFS benchmark suite and capture results.
#
# Usage:
#   bash scripts/bench-bfs.sh                # default: 10x × 3 runs
#   BENCHTIME=100x COUNT=5 bash scripts/bench-bfs.sh
#
# Output:
#   docs/perf/bfs-latest.txt — raw go test output (benchstat-compatible)
#
# Stats tooling: `go install golang.org/x/perf/cmd/benchstat@latest`

set -euo pipefail

BENCHTIME="${BENCHTIME:-10x}"
COUNT="${COUNT:-3}"
OUT="docs/perf/bfs-latest.txt"

cd "$(dirname "$0")/.."

mkdir -p docs/perf

echo "# BFS bench — $(date -u +%Y-%m-%dT%H:%M:%SZ)" > "$OUT"
echo "# benchtime=$BENCHTIME count=$COUNT" >> "$OUT"
echo "# go version: $(go version)" >> "$OUT"
echo "" >> "$OUT"

go test -bench='BenchmarkReachable|BenchmarkSubgraph|BenchmarkShortestPath|BenchmarkNeighbors' \
  -run='^$' \
  -benchtime="$BENCHTIME" \
  -count="$COUNT" \
  ./internal/service/ 2>&1 | tee -a "$OUT"

echo ""
echo "Wrote: $OUT"
echo ""
echo "Compare with baseline:"
echo "  benchstat docs/perf/bfs-baseline-raw.txt $OUT"
