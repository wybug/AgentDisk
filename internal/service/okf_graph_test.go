package service

import (
	"context"
	"testing"

	"github.com/agentdisk/agent-disk/internal/model"
)

// writeTestMarkdown is a thin wrapper around WriteMarkdown used by the edge
// materialization tests. It writes a body with the given type and content
// under concepts/<name>.md so tests stay readable.
func writeTestMarkdown(t *testing.T, svc *OkfService, name, body string) *model.OkfNode {
	t.Helper()
	node, err := svc.WriteMarkdown(context.Background(), WriteMarkdownRequest{
		PublicDirectoryID: 7,
		RelPath:           "concepts/" + name + ".md",
		Content:           []byte(body),
	})
	if err != nil {
		t.Fatalf("WriteMarkdown %s: %v", name, err)
	}
	return node
}

// TestMaterializeEdges_NewNode_PersistsEdges writes a single .md with two
// bundle-relative links (one existing, one missing) and verifies the edge
// table picked up both, plus that has_broken_link flips on.
func TestMaterializeEdges_NewNode_PersistsEdges(t *testing.T) {
	_, nodes, edges, pub, svc := okfTestHarness(t)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", ""))
	if _, err := svc.RegisterBundle(context.Background(), 7); err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}

	writeTestMarkdown(t, svc, "target", "---\ntype: concept\n---\nbody")
	srcBody := "---\ntype: concept\n---\n[t](./target.md) [miss](./missing.md)\n"
	src := writeTestMarkdown(t, svc, "source", srcBody)

	got, err := edges.ListBySrc(7, src.ID, 0)
	if err != nil {
		t.Fatalf("ListBySrc: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d edges, want 2", len(got))
	}
	live := 0
	broken := 0
	for _, e := range got {
		if e.DstExists {
			live++
		} else {
			broken++
		}
	}
	if live != 1 || broken != 1 {
		t.Errorf("edges live/broken = %d/%d, want 1/1", live, broken)
	}
	// The source node's link_count counts every edge regardless of liveness,
	// so it is 2 here even though only one resolved.
	refreshed, _ := nodes.GetByBundleAndRelPath(1, "concepts/source.md")
	if refreshed.LinkCount != 2 {
		t.Errorf("LinkCount = %d, want 2", refreshed.LinkCount)
	}
	if !refreshed.HasBrokenLink {
		t.Errorf("HasBrokenLink = false, want true")
	}
	// bundle.edge_count is maintained on every write, so it tracks the
	// materialized graph rather than the OSS file set.
	bundle, _ := svc.bundles.GetByID(1)
	if bundle.EdgeCount != 2 {
		t.Errorf("bundle.EdgeCount = %d, want 2", bundle.EdgeCount)
	}
}

// TestMaterializeEdges_BacklinkIncremental writes a → b, then c → b, and
// verifies b's backlink_count lands at 2.
func TestMaterializeEdges_BacklinkIncremental(t *testing.T) {
	_, nodes, _, pub, svc := okfTestHarness(t)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", ""))
	if _, err := svc.RegisterBundle(context.Background(), 7); err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}

	writeTestMarkdown(t, svc, "b", "---\ntype: concept\n---\nb body")
	writeTestMarkdown(t, svc, "a", "---\ntype: concept\n---\n[b](./b.md)\n")
	writeTestMarkdown(t, svc, "c", "---\ntype: concept\n---\n[b](./b.md)\n")

	b, _ := nodes.GetByBundleAndRelPath(1, "concepts/b.md")
	if b.BacklinkCount != 2 {
		t.Errorf("b.BacklinkCount = %d, want 2", b.BacklinkCount)
	}
}

// TestMaterializeEdges_UpdatePreservesUnchanged verifies that re-writing a
// node's body in a way that keeps one edge and drops another does not double
// count the unchanged edge.
func TestMaterializeEdges_UpdatePreservesUnchanged(t *testing.T) {
	_, nodes, _, pub, svc := okfTestHarness(t)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", ""))
	if _, err := svc.RegisterBundle(context.Background(), 7); err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}

	writeTestMarkdown(t, svc, "b", "---\ntype: concept\n---\nb body")
	writeTestMarkdown(t, svc, "c", "---\ntype: concept\n---\nc body")
	// a links to both b and c.
	writeTestMarkdown(t, svc, "a", "---\ntype: concept\n---\n[b](./b.md) [c](./c.md)\n")

	b, _ := nodes.GetByBundleAndRelPath(1, "concepts/b.md")
	c, _ := nodes.GetByBundleAndRelPath(1, "concepts/c.md")
	if b.BacklinkCount != 1 || c.BacklinkCount != 1 {
		t.Fatalf("after first write: b=%d c=%d, want 1/1", b.BacklinkCount, c.BacklinkCount)
	}
	// Re-write a keeping the link to b but dropping the link to c. b's count
	// stays at 1, c's drops to 0.
	writeTestMarkdown(t, svc, "a", "---\ntype: concept\n---\n[b](./b.md)\n")

	b, _ = nodes.GetByBundleAndRelPath(1, "concepts/b.md")
	c, _ = nodes.GetByBundleAndRelPath(1, "concepts/c.md")
	if b.BacklinkCount != 1 {
		t.Errorf("b.BacklinkCount = %d, want 1 (unchanged)", b.BacklinkCount)
	}
	if c.BacklinkCount != 0 {
		t.Errorf("c.BacklinkCount = %d, want 0 (link removed)", c.BacklinkCount)
	}
	a, _ := nodes.GetByBundleAndRelPath(1, "concepts/a.md")
	if a.LinkCount != 1 {
		t.Errorf("a.LinkCount = %d, want 1", a.LinkCount)
	}
	if a.HasBrokenLink {
		t.Errorf("a.HasBrokenLink = true, want false")
	}
}

// TestMaterializeEdges_EmptyBodyClearsEdges writes a node that links to a
// target, then re-writes it with no links. The old edge must be gone and
// link_count/has_broken_link reset.
func TestMaterializeEdges_EmptyBodyClearsEdges(t *testing.T) {
	_, nodes, edges, pub, svc := okfTestHarness(t)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", ""))
	if _, err := svc.RegisterBundle(context.Background(), 7); err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}

	writeTestMarkdown(t, svc, "b", "---\ntype: concept\n---\nb body")
	src := writeTestMarkdown(t, svc, "a", "---\ntype: concept\n---\n[b](./b.md)\n")

	if got, _ := edges.ListBySrc(7, src.ID, 0); len(got) != 1 {
		t.Fatalf("after first write: %d edges, want 1", len(got))
	}
	// Re-write a with no links.
	src = writeTestMarkdown(t, svc, "a", "---\ntype: concept\n---\nno links here\n")

	if got, _ := edges.ListBySrc(7, src.ID, 0); len(got) != 0 {
		t.Errorf("after rewrite: %d edges, want 0", len(got))
	}
	a, _ := nodes.GetByBundleAndRelPath(1, "concepts/a.md")
	if a.LinkCount != 0 {
		t.Errorf("a.LinkCount = %d, want 0", a.LinkCount)
	}
	if a.HasBrokenLink {
		t.Errorf("a.HasBrokenLink = true, want false")
	}
	b, _ := nodes.GetByBundleAndRelPath(1, "concepts/b.md")
	if b.BacklinkCount != 0 {
		t.Errorf("b.BacklinkCount = %d, want 0 after edge removed", b.BacklinkCount)
	}
}

// TestMaterializeEdges_NewTargetResolvesBrokenLink writes a → b when b does
// not exist (broken), then writes b, then re-writes a. The re-write must
// resolve the edge and bump b's backlink_count from 0 to 1.
func TestMaterializeEdges_NewTargetResolvesBrokenLink(t *testing.T) {
	_, nodes, _, pub, svc := okfTestHarness(t)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", ""))
	if _, err := svc.RegisterBundle(context.Background(), 7); err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}

	// a links to ./b.md which does not exist yet.
	writeTestMarkdown(t, svc, "a", "---\ntype: concept\n---\n[b](./b.md)\n")
	a, _ := nodes.GetByBundleAndRelPath(1, "concepts/a.md")
	if !a.HasBrokenLink {
		t.Errorf("a.HasBrokenLink = false, want true (b missing)")
	}

	// b appears, then a is re-written. The diff sees the same (dst_rel_path,
	// src_line) edge but with dst_exists flipping false → true, so b gains
	// an incoming edge.
	writeTestMarkdown(t, svc, "b", "---\ntype: concept\n---\nb body")
	writeTestMarkdown(t, svc, "a", "---\ntype: concept\n---\n[b](./b.md)\n")

	a, _ = nodes.GetByBundleAndRelPath(1, "concepts/a.md")
	if a.HasBrokenLink {
		t.Errorf("a.HasBrokenLink = true, want false after b appears")
	}
	b, _ := nodes.GetByBundleAndRelPath(1, "concepts/b.md")
	if b.BacklinkCount != 1 {
		t.Errorf("b.BacklinkCount = %d, want 1", b.BacklinkCount)
	}
}

// TestUnregisterBundle_DeletesEdges verifies that unregistering a bundle
// wipes its edge rows. Otherwise a future re-registration of the same public
// directory would inherit orphan edges from the previous bundle.
func TestUnregisterBundle_DeletesEdges(t *testing.T) {
	_, _, edges, pub, svc := okfTestHarness(t)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", ""))
	if _, err := svc.RegisterBundle(context.Background(), 7); err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}

	writeTestMarkdown(t, svc, "a", "---\ntype: concept\n---\n[b](./b.md)\n")
	writeTestMarkdown(t, svc, "b", "---\ntype: concept\n---\nb body")
	if count, _ := edges.CountByBundle(nil, 1, 7); count != 1 {
		t.Fatalf("before unregister: %d edges, want 1", count)
	}

	if err := svc.UnregisterBundle(1); err != nil {
		t.Fatalf("UnregisterBundle: %v", err)
	}
	if count, _ := edges.CountByBundle(nil, 1, 7); count != 0 {
		t.Errorf("after unregister: %d edges, want 0", count)
	}
}

// TestMaterializeEdges_NoEdgesRepo exercises the unit-test path where the
// service is built without an edge repo. WriteMarkdown must still succeed and
// the node must be materialized; the edge path simply no-ops.
func TestMaterializeEdges_NoEdgesRepo(t *testing.T) {
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	pub := newFakeOkfPublicDir(&model.DiskPublicDirectory{ID: 7, FolderID: 100, FixedPath: "/public/kb"})
	pub.seedContent("index.md", mustIndexMD(t, "0.1", ""))
	svc := NewOkfServiceFromRepo(bundles, nodes, nil, pub, "sqlite")
	if _, err := svc.RegisterBundle(context.Background(), 7); err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}
	if _, err := svc.WriteMarkdown(context.Background(), WriteMarkdownRequest{
		PublicDirectoryID: 7,
		RelPath:           "concepts/a.md",
		Content:           []byte("---\ntype: concept\n---\nbody"),
	}); err != nil {
		t.Fatalf("WriteMarkdown: %v", err)
	}
	if _, err := nodes.GetByBundleAndRelPath(1, "concepts/a.md"); err != nil {
		t.Errorf("node missing after write with nil edge repo: %v", err)
	}
}

// TestDiffEdges_Classify covers the diff helper directly. Identical edges
// produce no inserts/deletes; a removed edge yields a decrement on its dst;
// an added live edge yields an increment.
func TestDiffEdges_Classify(t *testing.T) {
	old := []model.OkfEdge{
		{DstRelPath: "a.md", SrcLine: 1, DstExists: true, DstNodeID: 10},
		{DstRelPath: "b.md", SrcLine: 2, DstExists: true, DstNodeID: 20},
	}
	new := []model.OkfEdge{
		{DstRelPath: "a.md", SrcLine: 1, DstExists: true, DstNodeID: 10}, // unchanged
		{DstRelPath: "c.md", SrcLine: 3, DstExists: true, DstNodeID: 30}, // added
	}
	toIns, toDel, inc, dec := diffEdges(old, new)
	if len(toIns) != 1 || toIns[0].DstRelPath != "c.md" {
		t.Errorf("toInsert = %+v, want [c.md]", toIns)
	}
	if len(toDel) != 1 || toDel[0].DstRelPath != "b.md" {
		t.Errorf("toDelete = %+v, want [b.md]", toDel)
	}
	if len(inc) != 1 || inc[0] != 30 {
		t.Errorf("increment = %v, want [30]", inc)
	}
	if len(dec) != 1 || dec[0] != 20 {
		t.Errorf("decrement = %v, want [20]", dec)
	}
}

// TestDiffEdges_LivenessFlip covers the case where the same (dst_rel_path,
// src_line) edge exists before and after, but the target appeared between
// writes. The dst node should land in increment, not be treated as unchanged.
func TestDiffEdges_LivenessFlip(t *testing.T) {
	old := []model.OkfEdge{
		{DstRelPath: "a.md", SrcLine: 1, DstExists: false, DstNodeID: 0},
	}
	new := []model.OkfEdge{
		{DstRelPath: "a.md", SrcLine: 1, DstExists: true, DstNodeID: 99},
	}
	toIns, toDel, inc, dec := diffEdges(old, new)
	if len(toIns) != 0 || len(toDel) != 0 {
		t.Errorf("liveness flip should not insert/delete, got ins=%d del=%d", len(toIns), len(toDel))
	}
	if len(inc) != 1 || inc[0] != 99 {
		t.Errorf("increment = %v, want [99]", inc)
	}
	if len(dec) != 0 {
		t.Errorf("decrement = %v, want []", dec)
	}
}
