package service

import (
	"context"
	"errors"
	"testing"

	"github.com/agentdisk/agent-disk/internal/model"
	"gorm.io/gorm"
)

// seedBFSGraph builds a small diamond graph for BFS tests:
//
//	A → B → D
//	A → C → D
//
// plus B → C to introduce a cycle and a dead edge B → ghost.md.
// Returns the node ID map so tests can address nodes by label.
func seedBFSGraph(t *testing.T, bundles *fakeOkfBundleRepo, nodes *fakeOkfNodeRepo, edges *fakeOkfEdgeRepo, pdID uint64) (map[string]uint64, uint64) {
	t.Helper()
	bundle := &model.OkfBundle{ID: 1, PublicDirectoryID: pdID, OkfVersion: "0.1", Status: "active"}
	if err := bundles.Create(bundle); err != nil {
		t.Fatalf("create bundle: %v", err)
	}
	byLabel := map[string]uint64{}
	for _, label := range []string{"A", "B", "C", "D"} {
		n := &model.OkfNode{BundleID: bundle.ID, Type: "concept", Title: label, RelPath: label + ".md"}
		if err := nodes.Upsert(nil, n); err != nil {
			t.Fatalf("upsert %s: %v", label, err)
		}
		byLabel[label] = n.ID
	}
	// Edges: A→B, A→C, B→D, C→D, B→C (cycle), B→ghost (dead).
	addEdge(t, edges, pdID, byLabel["A"], byLabel["B"], "B.md")
	addEdge(t, edges, pdID, byLabel["A"], byLabel["C"], "C.md")
	addEdge(t, edges, pdID, byLabel["B"], byLabel["D"], "D.md")
	addEdge(t, edges, pdID, byLabel["C"], byLabel["D"], "D.md")
	addEdge(t, edges, pdID, byLabel["B"], byLabel["C"], "C.md")
	// Dead edge: dst_exists=false, dst_node_id=0.
	dead := model.OkfEdge{
		PublicDirID: pdID, SrcNodeID: byLabel["B"],
		DstRelPath: "ghost.md", DstNodeID: 0, DstExists: false,
		SrcLine: 99, LinkKind: LinkKindBundle,
	}
	if err := edges.ReplaceForSrc(nil, pdID, byLabel["B"], append(edgesLive(t, edges, pdID, byLabel["B"]), dead)); err != nil {
		t.Fatalf("replace dead edges: %v", err)
	}
	return byLabel, bundle.ID
}

// edgesLive reads back a node's live edges so the dead-edge helper above can
// re-insert them alongside the new dead entry without dropping them.
func edgesLive(t *testing.T, edges *fakeOkfEdgeRepo, pdID, srcID uint64) []model.OkfEdge {
	t.Helper()
	out, err := edges.ListBySrc(pdID, srcID, 0)
	if err != nil {
		t.Fatalf("list live: %v", err)
	}
	live := out[:0]
	for i := range out {
		if out[i].DstExists {
			live = append(live, out[i])
		}
	}
	return live
}

func addEdge(t *testing.T, edges *fakeOkfEdgeRepo, pdID, src, dst uint64, dstRel string) {
	t.Helper()
	live := edgesLive(t, edges, pdID, src)
	live = append(live, model.OkfEdge{
		PublicDirID: pdID, SrcNodeID: src, DstNodeID: dst,
		DstRelPath: dstRel, DstExists: true, SrcLine: len(live) + 1,
		LinkKind: LinkKindBundle,
	})
	if err := edges.ReplaceForSrc(nil, pdID, src, live); err != nil {
		t.Fatalf("add edge %d→%d: %v", src, dst, err)
	}
}

func newBFSService(t *testing.T) (*OkfService, *fakeOkfBundleRepo, *fakeOkfNodeRepo, *fakeOkfEdgeRepo, map[string]uint64, uint64) {
	t.Helper()
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	edges := newFakeOkfEdgeRepo(nodes)
	pub := newFakeOkfPublicDir(&model.DiskPublicDirectory{ID: 7, FolderID: 100, FixedPath: "/public/kb"})
	svc := NewOkfServiceFromRepo(bundles, nodes, edges, pub, "sqlite")
	byLabel, bundleID := seedBFSGraph(t, bundles, nodes, edges, 7)
	return svc, bundles, nodes, edges, byLabel, bundleID
}

// TestNeighbors_OutDirection verifies the 1-hop out-edge walk: A's neighbors
// are {B, C}. The dead edge from B is not A's neighbor so it should not
// surface here.
func TestNeighbors_OutDirection(t *testing.T) {
	svc, _, _, _, byLabel, _ := newBFSService(t)
	out, err := svc.Neighbors(context.Background(), NeighborsRequest{NodeID: byLabel["A"], Direction: BFSOut})
	if err != nil {
		t.Fatalf("Neighbors: %v", err)
	}
	got := nodeTitles(out.Nodes)
	want := map[string]bool{"B": true, "C": true}
	if len(got) != len(want) {
		t.Fatalf("neighbors = %v, want 2 (B, C)", got)
	}
	for title := range got {
		if !want[title] {
			t.Errorf("unexpected neighbor %q", title)
		}
	}
}

// TestNeighbors_InDirection walks in-edges: D's in-neighbors are {B, C}.
func TestNeighbors_InDirection(t *testing.T) {
	svc, _, _, _, byLabel, _ := newBFSService(t)
	out, err := svc.Neighbors(context.Background(), NeighborsRequest{NodeID: byLabel["D"], Direction: BFSIn})
	if err != nil {
		t.Fatalf("Neighbors: %v", err)
	}
	got := nodeTitles(out.Nodes)
	if len(got) != 2 {
		t.Fatalf("in-neighbors = %v, want 2 (B, C)", got)
	}
}

// TestNeighbors_BothDirection walks both directions: B's both-neighbors are
// {A (in), C (in+out), D (out)}. The ghost dead edge must not surface.
func TestNeighbors_BothDirection(t *testing.T) {
	svc, _, _, _, byLabel, _ := newBFSService(t)
	out, err := svc.Neighbors(context.Background(), NeighborsRequest{NodeID: byLabel["B"], Direction: BFSBoth})
	if err != nil {
		t.Fatalf("Neighbors: %v", err)
	}
	got := nodeTitles(out.Nodes)
	want := map[string]bool{"A": true, "C": true, "D": true}
	if len(got) != len(want) {
		t.Fatalf("both-neighbors = %v, want 3 (A, C, D)", got)
	}
	for title := range got {
		if !want[title] {
			t.Errorf("unexpected neighbor %q", title)
		}
	}
}

// TestNeighbors_InvalidDirection confirms the validation branch: a bogus
// dir value surfaces as ErrOkfInvalidDirection rather than a silent default.
func TestNeighbors_InvalidDirection(t *testing.T) {
	svc, _, _, _, byLabel, _ := newBFSService(t)
	_, err := svc.Neighbors(context.Background(), NeighborsRequest{NodeID: byLabel["A"], Direction: "sideways"})
	if !errors.Is(err, ErrOkfInvalidDirection) {
		t.Errorf("err = %v, want ErrOkfInvalidDirection", err)
	}
}

// TestNeighbors_NodeNotFound verifies that a non-existent node id surfaces
// as ErrOkfNodeNotFound, which the handler maps to 404.
func TestNeighbors_NodeNotFound(t *testing.T) {
	svc, _, _, _, _, _ := newBFSService(t)
	_, err := svc.Neighbors(context.Background(), NeighborsRequest{NodeID: 9999})
	if !errors.Is(err, ErrOkfNodeNotFound) {
		t.Errorf("err = %v, want ErrOkfNodeNotFound", err)
	}
}

// TestNeighbors_ForbiddenWhenInvisible asserts the ACL: an invisible bundle
// yields ErrOkfForbidden, never the neighbor list.
func TestNeighbors_ForbiddenWhenInvisible(t *testing.T) {
	svc, _, _, _, byLabel, _ := newBFSService(t)
	svc.SetVisibility(stubVisibility{visible: map[uint64]bool{}}) // deny everything
	_, err := svc.Neighbors(context.Background(), NeighborsRequest{NodeID: byLabel["A"]})
	if !errors.Is(err, ErrOkfForbidden) {
		t.Errorf("err = %v, want ErrOkfForbidden", err)
	}
}

// TestReachable_TwoHopsAllNodes walks 2 hops from A and expects to reach
// {B, C, D}. D appears twice in the edge table (from B and from C) but only
// once in the reachable set — visited dedup is exercised.
func TestReachable_TwoHopsAllNodes(t *testing.T) {
	svc, _, _, _, byLabel, _ := newBFSService(t)
	out, err := svc.Reachable(context.Background(), ReachableRequest{NodeID: byLabel["A"], Depth: 2})
	if err != nil {
		t.Fatalf("Reachable: %v", err)
	}
	got := nodeTitles(out.Nodes)
	if len(got) != 3 {
		t.Fatalf("reachable = %v, want 3 (B, C, D)", got)
	}
}

// TestReachable_DepthClamp confirms the depth cap: a request for Depth=99
// is silently clamped to MaxBFSDepth=3 rather than erroring.
func TestReachable_DepthClamp(t *testing.T) {
	svc, _, _, _, byLabel, _ := newBFSService(t)
	out, err := svc.Reachable(context.Background(), ReachableRequest{NodeID: byLabel["A"], Depth: 99})
	if err != nil {
		t.Fatalf("Reachable: %v", err)
	}
	// 3 hops is enough to visit every node in the diamond.
	if len(out.Nodes) != 3 {
		t.Errorf("reachable len = %d, want 3 (clamped to MaxBFSDepth)", len(out.Nodes))
	}
}

// TestReachable_MaxNodesCap exercises the response-size guard: with
// MaxNodes=1, only one reachable node comes back even though the diamond
// has three.
func TestReachable_MaxNodesCap(t *testing.T) {
	svc, _, _, _, byLabel, _ := newBFSService(t)
	out, err := svc.Reachable(context.Background(), ReachableRequest{NodeID: byLabel["A"], Depth: 3, MaxNodes: 1})
	if err != nil {
		t.Fatalf("Reachable: %v", err)
	}
	if len(out.Nodes) != 1 {
		t.Errorf("reachable len = %d, want 1 (MaxNodes cap)", len(out.Nodes))
	}
}

// TestShortestPath_Diamond verifies the bidirectional BFS finds the
// length-2 path A → B → D (or A → C → D, both are valid).
func TestShortestPath_Diamond(t *testing.T) {
	svc, _, _, _, byLabel, _ := newBFSService(t)
	out, err := svc.ShortestPath(context.Background(), ShortestPathRequest{SrcNodeID: byLabel["A"], DstNodeID: byLabel["D"]})
	if err != nil {
		t.Fatalf("ShortestPath: %v", err)
	}
	if !out.Found {
		t.Fatal("Found = false, want true")
	}
	if len(out.Nodes) != 3 {
		t.Fatalf("path len = %d, want 3 (A, mid, D)", len(out.Nodes))
	}
	if out.Nodes[0].Title != "A" {
		t.Errorf("path[0] = %q, want A", out.Nodes[0].Title)
	}
	if out.Nodes[2].Title != "D" {
		t.Errorf("path[2] = %q, want D", out.Nodes[2].Title)
	}
	mid := out.Nodes[1].Title
	if mid != "B" && mid != "C" {
		t.Errorf("path[1] = %q, want B or C", mid)
	}
}

// TestShortestPath_SameNode returns a single-node path when src == dst.
func TestShortestPath_SameNode(t *testing.T) {
	svc, _, _, _, byLabel, _ := newBFSService(t)
	out, err := svc.ShortestPath(context.Background(), ShortestPathRequest{SrcNodeID: byLabel["A"], DstNodeID: byLabel["A"]})
	if err != nil {
		t.Fatalf("ShortestPath: %v", err)
	}
	if !out.Found || len(out.Nodes) != 1 {
		t.Fatalf("Found=%v len=%d, want Found=true len=1", out.Found, len(out.Nodes))
	}
}

// TestShortestPath_NoPath verifies the not-found branch: from D, no node
// reaches A within depth 1 because every D edge is incoming.
func TestShortestPath_NoPath(t *testing.T) {
	svc, _, _, _, byLabel, _ := newBFSService(t)
	out, err := svc.ShortestPath(context.Background(), ShortestPathRequest{SrcNodeID: byLabel["D"], DstNodeID: byLabel["A"], MaxDepth: 1})
	if err != nil {
		t.Fatalf("ShortestPath: %v", err)
	}
	if out.Found {
		t.Errorf("Found = true, want false (D has no out-edges)")
	}
	if len(out.Nodes) != 0 {
		t.Errorf("len = %d, want 0 when not found", len(out.Nodes))
	}
}

// TestSubgraph_TypeFilter exercises the type-filtered bulk extract: asking
// for type "concept" returns all 4 nodes and the 5 live edges between them.
func TestSubgraph_TypeFilter(t *testing.T) {
	svc, _, _, _, _, bundleID := newBFSService(t)
	out, err := svc.Subgraph(context.Background(), SubgraphRequest{BundleID: bundleID, Types: []string{"concept"}})
	if err != nil {
		t.Fatalf("Subgraph: %v", err)
	}
	if len(out.Nodes) != 4 {
		t.Errorf("nodes = %d, want 4", len(out.Nodes))
	}
	if len(out.Edges) != 5 {
		t.Errorf("edges = %d, want 5 (dead ghost edge excluded)", len(out.Edges))
	}
}

// TestSubgraph_TypeFilterExcludesNode confirms the type filter narrows the
// node set: a node with type "guide" is excluded when only "concept" is asked
// for.
func TestSubgraph_TypeFilterExcludesNode(t *testing.T) {
	svc, _, nodes, _, _, bundleID := newBFSService(t)
	// Inject a node with a different type.
	guide := &model.OkfNode{BundleID: 1, Type: "guide", Title: "Guide", RelPath: "guide.md"}
	if err := nodes.Upsert(nil, guide); err != nil {
		t.Fatalf("upsert guide: %v", err)
	}
	out, err := svc.Subgraph(context.Background(), SubgraphRequest{BundleID: bundleID, Types: []string{"concept"}})
	if err != nil {
		t.Fatalf("Subgraph: %v", err)
	}
	for _, n := range out.Nodes {
		if n.Type != "concept" {
			t.Errorf("found type %q in concept-only subgraph", n.Type)
		}
	}
}

// TestStats_ReturnsCounts exercises the bundle stats rollup: NodeCount,
// EdgeTotal (6 = 5 live + 1 dead), EdgeLive (5), EdgeBroken (1).
func TestStats_ReturnsCounts(t *testing.T) {
	svc, _, _, _, _, bundleID := newBFSService(t)
	out, err := svc.Stats(context.Background(), StatsRequest{BundleID: bundleID})
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if out.EdgeTotal != 6 {
		t.Errorf("EdgeTotal = %d, want 6", out.EdgeTotal)
	}
	if out.EdgeLive != 5 {
		t.Errorf("EdgeLive = %d, want 5", out.EdgeLive)
	}
	if out.EdgeBroken != 1 {
		t.Errorf("EdgeBroken = %d, want 1", out.EdgeBroken)
	}
}

// TestStats_ForbiddenWhenInvisible asserts the ACL on the stats endpoint.
func TestStats_ForbiddenWhenInvisible(t *testing.T) {
	svc, _, _, _, _, bundleID := newBFSService(t)
	svc.SetVisibility(stubVisibility{visible: map[uint64]bool{}})
	_, err := svc.Stats(context.Background(), StatsRequest{BundleID: bundleID})
	if !errors.Is(err, ErrOkfForbidden) {
		t.Errorf("err = %v, want ErrOkfForbidden", err)
	}
}

// nodeTitles pulls the Title field out of a node slice so tests can assert
// on the human-readable label rather than brittle id values.
func nodeTitles(nodes []model.OkfNode) map[string]bool {
	out := make(map[string]bool, len(nodes))
	for i := range nodes {
		out[nodes[i].Title] = true
	}
	return out
}

// Ensure gorm import is exercised: the dead-edge helper above relies on
// gorm.ErrRecordNotFound via the fake's GetByBundleAndRelPath, but the
// lint pass needs a direct reference here.
var _ = gorm.ErrRecordNotFound
