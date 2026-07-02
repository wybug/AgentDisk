package service

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/agentdisk/agent-disk/internal/model"
)

// Bench distribution constants. The exact string values appear in bench names
// (`Reachable/N=1000/d=3/uniform`) so changes here invalidate recorded
// baselines under docs/perf/.
const (
	benchDistUniform   = "uniform"
	benchDistPowerlaw  = "powerlaw"
	benchDistChain     = "chain"
	benchDefaultSeed   = int64(42)
	benchPublicDirID   = uint64(7)
	benchEdgePerNodeLo = 1
	benchEdgePerNodeMd = 5
	benchEdgePerNodeHi = 10
)

// benchGraphSpec parameterizes a seeded graph. All fields are required; callers
// build one per b.Run subtest.
type benchGraphSpec struct {
	nodeCount    int
	edgesPerNode int
	dist         string // benchDistUniform | benchDistPowerlaw | benchDistChain
}

// seedScaleGraph builds an in-memory graph of the requested shape and wires it
// into the fake repos. The bundle is always ID=1, public directory ID=7. Node
// IDs are 1..N, edge IDs 1..M. Edge generation:
//   - uniform: each node picks `edgesPerNode` distinct successors uniformly at
//     random from nodes 1..N.
//   - powerlaw: successors are sampled from a Zipf-like distribution (a few
//     hubs receive most edges) — closer to real OKF topology.
//   - chain: a linear chain node[i] → node[i+1], plus `edgesPerNode-1` random
//     shortcuts. The chain guarantees depth-3 walks have something to find
//     even on small N.
//
// The seeded RNG uses a fixed seed (benchDefaultSeed) so two bench runs at the
// same spec produce identical graphs — required for benchstat to compare
// across code changes without graph-shape noise.
//
// Returns (bundleID, startNodeID, farNodeID). startNodeID is a high-out-degree
// node (good Reachable start); farNodeID is the deepest reachable target.
func seedScaleGraph(tb testing.TB, bundles *fakeOkfBundleRepo, nodes *fakeOkfNodeRepo, edges *fakeOkfEdgeRepo, spec benchGraphSpec) (bundleID, startNodeID, farNodeID uint64) {
	tb.Helper()
	if spec.nodeCount < 2 {
		tb.Fatalf("nodeCount must be >= 2, got %d", spec.nodeCount)
	}

	bundle := &model.OkfBundle{ID: 1, PublicDirectoryID: benchPublicDirID, OkfVersion: "0.1", Status: "active"}
	if err := bundles.Create(bundle); err != nil {
		tb.Fatalf("create bundle: %v", err)
	}

	// Build N nodes. Title is the decimal ID so nodeTitles-style debugging still
	// works, but the bench itself never inspects titles.
	for i := 1; i <= spec.nodeCount; i++ {
		n := &model.OkfNode{
			BundleID: 1,
			Type:     "concept",
			Title:    fmt.Sprintf("n%d", i),
			RelPath:  fmt.Sprintf("n%d.md", i),
		}
		if err := nodes.Upsert(nil, n); err != nil {
			tb.Fatalf("upsert node %d: %v", i, err)
		}
	}

	rng := rand.New(rand.NewSource(benchDefaultSeed))
	maxOut := spec.edgesPerNode
	if maxOut < 1 {
		maxOut = 1
	}

	// Edge generation dispatches by distribution. Each helper writes live edges
	// for every node in [1, nodeCount] via ReplaceForSrc — the same path the
	// real writer uses, so the fake's state shape mirrors production.
	switch spec.dist {
	case benchDistChain:
		seedChainEdges(tb, edges, spec.nodeCount, maxOut, rng)
	case benchDistUniform:
		seedUniformEdges(tb, edges, spec.nodeCount, maxOut, rng)
	case benchDistPowerlaw:
		seedPowerlawEdges(tb, edges, spec.nodeCount, maxOut, rng)
	default:
		tb.Fatalf("unknown dist %q", spec.dist)
	}

	startNodeID = 1
	farNodeID = uint64(spec.nodeCount)
	return bundle.ID, startNodeID, farNodeID
}

// seedChainEdges writes a linear chain i→i+1 plus (maxOut-1) random shortcuts
// per node. The chain guarantees depth-3 walks have something to find even on
// small N.
func seedChainEdges(tb testing.TB, edges *fakeOkfEdgeRepo, nodeCount, maxOut int, rng *rand.Rand) {
	tb.Helper()
	for i := 1; i < nodeCount; i++ {
		live := []model.OkfEdge{{
			PublicDirID: benchPublicDirID, SrcNodeID: uint64(i), DstNodeID: uint64(i + 1),
			DstRelPath: fmt.Sprintf("n%d.md", i+1), DstExists: true, SrcLine: 1, LinkKind: LinkKindBundle,
		}}
		for j := 1; j < maxOut && i+j < nodeCount; j++ {
			dst := uint64(rng.Intn(nodeCount-j) + j + 1)
			live = append(live, model.OkfEdge{
				PublicDirID: benchPublicDirID, SrcNodeID: uint64(i), DstNodeID: dst,
				DstRelPath: fmt.Sprintf("n%d.md", dst), DstExists: true, SrcLine: j + 1, LinkKind: LinkKindBundle,
			})
		}
		if err := edges.ReplaceForSrc(nil, benchPublicDirID, uint64(i), live); err != nil {
			tb.Fatalf("replace chain edges at %d: %v", i, err)
		}
	}
}

// seedUniformEdges writes maxOut random distinct successors per node.
func seedUniformEdges(tb testing.TB, edges *fakeOkfEdgeRepo, nodeCount, maxOut int, rng *rand.Rand) {
	tb.Helper()
	for i := 1; i <= nodeCount; i++ {
		live := make([]model.OkfEdge, 0, maxOut)
		seen := map[uint64]bool{uint64(i): true}
		for len(live) < maxOut {
			dst := uint64(rng.Intn(nodeCount) + 1)
			if seen[dst] {
				continue
			}
			seen[dst] = true
			live = append(live, benchEdge(uint64(i), dst, len(live)+1))
		}
		if err := edges.ReplaceForSrc(nil, benchPublicDirID, uint64(i), live); err != nil {
			tb.Fatalf("replace uniform edges at %d: %v", i, err)
		}
	}
}

// seedPowerlawEdges writes maxOut Zipf-skewed successors per node. Zipf
// parameter 1.5 gives a moderate skew: top ~10% of nodes hold ~60% of
// in-edges. Vary the parameter to stress different shapes.
func seedPowerlawEdges(tb testing.TB, edges *fakeOkfEdgeRepo, nodeCount, maxOut int, rng *rand.Rand) {
	tb.Helper()
	zipf := rand.NewZipf(rng, 1.5, 1, uint64(nodeCount))
	for i := 1; i <= nodeCount; i++ {
		live := make([]model.OkfEdge, 0, maxOut)
		seen := map[uint64]bool{uint64(i): true}
		for len(live) < maxOut {
			dst := zipf.Uint64() + 1
			if seen[dst] {
				continue
			}
			seen[dst] = true
			live = append(live, benchEdge(uint64(i), dst, len(live)+1))
		}
		if err := edges.ReplaceForSrc(nil, benchPublicDirID, uint64(i), live); err != nil {
			tb.Fatalf("replace powerlaw edges at %d: %v", i, err)
		}
	}
}

// benchEdge constructs one OkfEdge row for the bench graph. Centralizing the
// construction keeps the per-distribution loops readable.
func benchEdge(src, dst uint64, line int) model.OkfEdge {
	return model.OkfEdge{
		PublicDirID: benchPublicDirID, SrcNodeID: src, DstNodeID: dst,
		DstRelPath: fmt.Sprintf("n%d.md", dst), DstExists: true, SrcLine: line, LinkKind: LinkKindBundle,
	}
}

// benchGraphSpecs is the cross-product of (node count) × (edge density) ×
// (distribution). Concretely small so the full bench suite runs in < 30s on a
// developer laptop. Add larger N here once the smaller sizes are green.
func benchGraphSpecs() []benchGraphSpec {
	return []benchGraphSpec{
		{nodeCount: 1_000, edgesPerNode: benchEdgePerNodeMd, dist: benchDistUniform},
		{nodeCount: 1_000, edgesPerNode: benchEdgePerNodeMd, dist: benchDistPowerlaw},
		{nodeCount: 10_000, edgesPerNode: benchEdgePerNodeMd, dist: benchDistUniform},
		{nodeCount: 10_000, edgesPerNode: benchEdgePerNodeMd, dist: benchDistPowerlaw},
		// 100K is enabled for depth-limited walks (Reachable depth<=3) — MaxBFSDepth
		// caps expansion at HardBFSMaxNodes=1000 nodes so the result set stays
		// bounded even when the frontier would explode.
		{nodeCount: 100_000, edgesPerNode: benchEdgePerNodeMd, dist: benchDistUniform},
		{nodeCount: 100_000, edgesPerNode: benchEdgePerNodeMd, dist: benchDistPowerlaw},
	}
}

// specBenchName produces a stable subtest label like "N=1000/E=5/uniform" so
// benchstat can group across runs.
func specBenchName(s benchGraphSpec) string {
	return fmt.Sprintf("N=%d/E=%d/%s", s.nodeCount, s.edgesPerNode, s.dist)
}

// BenchmarkReachable measures the N-hop BFS walk. Depth is the primary cost
// axis (each hop multiplies the frontier by `edgesPerNode`), so we sweep depth
// 1/2/3 at every graph size.
func BenchmarkReachable(b *testing.B) {
	for _, spec := range benchGraphSpecs() {
		spec := spec
		for _, depth := range []int{1, 2, 3} {
			depth := depth
			b.Run(fmt.Sprintf("%s/d=%d", specBenchName(spec), depth), func(b *testing.B) {
				runReachableBench(b, spec, depth)
			})
		}
	}
}

func runReachableBench(b *testing.B, spec benchGraphSpec, depth int) {
	svc, bundles, nodes, edges := newBFSServiceRepos(b)
	startID, _, _ := seedScaleGraph(b, bundles, nodes, edges, spec)
	ctx := context.Background()
	req := ReachableRequest{
		NodeID:   startID,
		Depth:    depth,
		MaxNodes: HardBFSMaxNodes, // pass the ceiling — we want to measure the walk, not the cap
	}

	// Warm a single cold run outside the timer so the BFS loop's allocation
	// profile is what we measure, not the seed work.
	if _, err := svc.Reachable(ctx, req); err != nil {
		b.Fatalf("warm Reachable: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := svc.Reachable(ctx, req); err != nil {
			b.Fatalf("Reachable: %v", err)
		}
	}
}

// BenchmarkSubgraph measures the bulk-extract path used by the front-end graph
// visualizer. Node loading dominates here (the edge pruning pass is O(E)).
func BenchmarkSubgraph(b *testing.B) {
	for _, spec := range benchGraphSpecs() {
		spec := spec
		b.Run(specBenchName(spec), func(b *testing.B) {
			runSubgraphBench(b, spec)
		})
	}
}

func runSubgraphBench(b *testing.B, spec benchGraphSpec) {
	svc, bundles, nodes, edges := newBFSServiceRepos(b)
	bundleID, _, _ := seedScaleGraph(b, bundles, nodes, edges, spec)
	ctx := context.Background()
	req := SubgraphRequest{
		BundleID: bundleID,
		MaxNodes: HardBFSMaxNodes,
	}
	if _, err := svc.Subgraph(ctx, req); err != nil {
		b.Fatalf("warm Subgraph: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := svc.Subgraph(ctx, req); err != nil {
			b.Fatalf("Subgraph: %v", err)
		}
	}
}

// BenchmarkShortestPath measures bidirectional BFS. Two cases per spec:
//   - found: src=1, dst=last → typically resolves in 2 hops on dense graphs
//   - nopath: src=last, dst=1 with MaxDepth=1 → walks fan-out without meeting,
//     the worst-case expansion cost
func BenchmarkShortestPath(b *testing.B) {
	for _, spec := range benchGraphSpecs() {
		spec := spec
		b.Run(specBenchName(spec)+"/found", func(b *testing.B) {
			runShortestPathBench(b, spec, true)
		})
		b.Run(specBenchName(spec)+"/nopath", func(b *testing.B) {
			runShortestPathBench(b, spec, false)
		})
	}
}

func runShortestPathBench(b *testing.B, spec benchGraphSpec, foundCase bool) {
	svc, bundles, nodes, edges := newBFSServiceRepos(b)
	startID, _, farID := seedScaleGraph(b, bundles, nodes, edges, spec)
	ctx := context.Background()

	req := ShortestPathRequest{SrcNodeID: startID, DstNodeID: farID}
	if !foundCase {
		// Reverse direction with MaxDepth=1 guarantees no meet on these graphs
		// (chain/uniform/powerlaw are all DAG-shaped in the forward direction).
		req = ShortestPathRequest{SrcNodeID: farID, DstNodeID: startID, MaxDepth: 1}
	}

	if _, err := svc.ShortestPath(ctx, req); err != nil {
		b.Fatalf("warm ShortestPath: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := svc.ShortestPath(ctx, req); err != nil {
			b.Fatalf("ShortestPath: %v", err)
		}
	}
}

// BenchmarkNeighbors measures the 1-hop baseline. It's the simplest path
// through the BFS layer (no frontier loop, no depth clamp) and serves as a
// floor for the more complex walks.
func BenchmarkNeighbors(b *testing.B) {
	for _, spec := range benchGraphSpecs() {
		spec := spec
		b.Run(specBenchName(spec), func(b *testing.B) {
			runNeighborsBench(b, spec)
		})
	}
}

func runNeighborsBench(b *testing.B, spec benchGraphSpec) {
	svc, bundles, nodes, edges := newBFSServiceRepos(b)
	startID, _, _ := seedScaleGraph(b, bundles, nodes, edges, spec)
	ctx := context.Background()
	req := NeighborsRequest{NodeID: startID, Direction: BFSOut}

	if _, err := svc.Neighbors(ctx, req); err != nil {
		b.Fatalf("warm Neighbors: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := svc.Neighbors(ctx, req); err != nil {
			b.Fatalf("Neighbors: %v", err)
		}
	}
}
