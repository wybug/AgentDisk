package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/agentdisk/agent-disk/internal/model"
)

// addSearchNode is a thin wrapper around the fake node repo's Upsert that
// builds a node from a title + type so search tests stay readable. The fake
// keys nodes by (bundleID, relPath), so each call must mint a unique relPath
// or every node would clobber the prior one.
func addSearchNode(t *testing.T, nodes *fakeOkfNodeRepo, bundleID uint64, typ, title string) *model.OkfNode {
	t.Helper()
	relPath := title
	n := &model.OkfNode{BundleID: bundleID, Type: typ, Title: title, RelPath: relPath}
	if err := nodes.Upsert(nil, n); err != nil {
		t.Fatalf("upsert node: %v", err)
	}
	return n
}

// TestSearch_ScopedToBundle exercises the bundle-scoped path: req.BundleID is
// set, the bundle is visible, and only matches inside that bundle come back.
// Nodes in another bundle with the same title must not leak through.
func TestSearch_ScopedToBundle(t *testing.T) {
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	edges := newFakeOkfEdgeRepo(nodes)
	pub := newFakeOkfPublicDir(&model.DiskPublicDirectory{ID: 7, FolderID: 100, FixedPath: "/public/kb"})
	svc := NewOkfServiceFromRepo(bundles, nodes, edges, pub, "sqlite")

	bundle := &model.OkfBundle{ID: 1, PublicDirectoryID: 7, OkfVersion: "0.1", Status: "active"}
	_ = bundles.Create(bundle)
	other := &model.OkfBundle{ID: 2, PublicDirectoryID: 8, OkfVersion: "0.1", Status: "active"}
	_ = bundles.Create(other)

	addSearchNode(t, nodes, 1, "concept", "Gemma model card")
	addSearchNode(t, nodes, 1, "concept", "Transformer overview")
	addSearchNode(t, nodes, 2, "concept", "Gemma tuning guide") // other bundle, must be excluded

	out, err := svc.Search(context.Background(), SearchRequest{
		Query:    "gemma",
		BundleID: 1,
		Limit:    50,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(out.Nodes) != 1 {
		t.Fatalf("got %d nodes, want 1 (bundle-scoped)", len(out.Nodes))
	}
	if !strings.Contains(out.Nodes[0].Title, "Gemma") {
		t.Errorf("title = %q, want a Gemma match", out.Nodes[0].Title)
	}
	if out.NextCursor != 0 {
		t.Errorf("nextCursor = %d, want 0 (single page)", out.NextCursor)
	}
}

// TestSearch_ForbiddenWhenBundleInvisible asserts the ACL: when the caller's
// visibility set excludes the requested bundle, Search returns
// ErrOkfForbidden rather than leaking a result.
func TestSearch_ForbiddenWhenBundleInvisible(t *testing.T) {
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	edges := newFakeOkfEdgeRepo(nodes)
	pub := newFakeOkfPublicDir(&model.DiskPublicDirectory{ID: 7, FolderID: 100, FixedPath: "/public/kb"})
	svc := NewOkfServiceFromRepo(bundles, nodes, edges, pub, "sqlite")
	svc.SetVisibility(stubVisibility{visible: map[uint64]bool{}}) // deny everything

	_ = bundles.Create(&model.OkfBundle{ID: 1, PublicDirectoryID: 7, OkfVersion: "0.1", Status: "active"})
	addSearchNode(t, nodes, 1, "concept", "Gemma")

	_, err := svc.Search(context.Background(), SearchRequest{
		Query:    "gemma",
		BundleID: 1,
	})
	if !errors.Is(err, ErrOkfForbidden) {
		t.Errorf("expected ErrOkfForbidden, got %v", err)
	}
}

// TestSearch_AcrossVisibleBundles walks every bundle, applies the visibility
// filter, and matches across the visible set. The denied bundle's title
// matches the query but must not appear in the response.
func TestSearch_AcrossVisibleBundles(t *testing.T) {
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	edges := newFakeOkfEdgeRepo(nodes)
	pub := newFakeOkfPublicDir(&model.DiskPublicDirectory{ID: 7, FolderID: 100, FixedPath: "/public/kb"})
	svc := NewOkfServiceFromRepo(bundles, nodes, edges, pub, "sqlite")

	_ = bundles.Create(&model.OkfBundle{ID: 1, PublicDirectoryID: 7, OkfVersion: "0.1", Status: "active"})
	_ = bundles.Create(&model.OkfBundle{ID: 2, PublicDirectoryID: 8, OkfVersion: "0.1", Status: "active"})
	addSearchNode(t, nodes, 1, "concept", "Gemma card")
	addSearchNode(t, nodes, 2, "concept", "Gemma tuning") // bundle 2 will be invisible

	svc.SetVisibility(stubVisibility{visible: map[uint64]bool{7: true}})

	out, err := svc.Search(context.Background(), SearchRequest{Query: "gemma", Limit: 50})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(out.Nodes) != 1 {
		t.Fatalf("got %d nodes, want 1 (visible only)", len(out.Nodes))
	}
	if out.Nodes[0].BundleID != 1 {
		t.Errorf("node.BundleID = %d, want 1", out.Nodes[0].BundleID)
	}
}

// TestSearch_TypeFilter layers a type restriction on top of the query. A
// matching title under the wrong type must not appear.
func TestSearch_TypeFilter(t *testing.T) {
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	edges := newFakeOkfEdgeRepo(nodes)
	pub := newFakeOkfPublicDir(&model.DiskPublicDirectory{ID: 7, FolderID: 100, FixedPath: "/public/kb"})
	svc := NewOkfServiceFromRepo(bundles, nodes, edges, pub, "sqlite")
	_ = bundles.Create(&model.OkfBundle{ID: 1, PublicDirectoryID: 7, OkfVersion: "0.1", Status: "active"})

	addSearchNode(t, nodes, 1, "concept", "Gemma model")
	addSearchNode(t, nodes, 1, "guide", "Gemma quickstart") // wrong type

	out, err := svc.Search(context.Background(), SearchRequest{
		Query:    "gemma",
		BundleID: 1,
		Type:     "concept",
		Limit:    50,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(out.Nodes) != 1 {
		t.Fatalf("got %d nodes, want 1 (type-filtered)", len(out.Nodes))
	}
	if out.Nodes[0].Type != "concept" {
		t.Errorf("type = %q, want concept", out.Nodes[0].Type)
	}
}

// TestSearch_Pagination walks two pages of a wide query. The fake repo
// returns ordered-by-id, so the cursor is the last seen id; the second page
// picks up the remaining matches.
func TestSearch_Pagination(t *testing.T) {
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	edges := newFakeOkfEdgeRepo(nodes)
	pub := newFakeOkfPublicDir(&model.DiskPublicDirectory{ID: 7, FolderID: 100, FixedPath: "/public/kb"})
	svc := NewOkfServiceFromRepo(bundles, nodes, edges, pub, "sqlite")
	_ = bundles.Create(&model.OkfBundle{ID: 1, PublicDirectoryID: 7, OkfVersion: "0.1", Status: "active"})

	// Seed three matches so a page size of 2 yields 2 + 1.
	addSearchNode(t, nodes, 1, "concept", "Gemma card")
	addSearchNode(t, nodes, 1, "concept", "Gemma tuning")
	addSearchNode(t, nodes, 1, "concept", "Gemma advanced")

	page1, err := svc.Search(context.Background(), SearchRequest{
		Query:    "gemma",
		BundleID: 1,
		Limit:    2,
	})
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if len(page1.Nodes) != 2 {
		t.Fatalf("page1 len = %d, want 2", len(page1.Nodes))
	}
	if page1.NextCursor == 0 {
		t.Fatalf("page1 nextCursor = 0, want non-zero (more pages)")
	}

	page2, err := svc.Search(context.Background(), SearchRequest{
		Query:    "gemma",
		BundleID: 1,
		Limit:    2,
		Cursor:   page1.NextCursor,
	})
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if len(page2.Nodes) != 1 {
		t.Errorf("page2 len = %d, want 1", len(page2.Nodes))
	}
	if page2.NextCursor != 0 {
		t.Errorf("page2 nextCursor = %d, want 0 (last page)", page2.NextCursor)
	}
}

// TestSearch_BundleNotFound ensures the missing-bundle sentinel surfaces for
// a search scoped to a bundle that does not exist.
func TestSearch_BundleNotFound(t *testing.T) {
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	edges := newFakeOkfEdgeRepo(nodes)
	pub := newFakeOkfPublicDir(&model.DiskPublicDirectory{ID: 7, FolderID: 100, FixedPath: "/public/kb"})
	svc := NewOkfServiceFromRepo(bundles, nodes, edges, pub, "sqlite")

	_, err := svc.Search(context.Background(), SearchRequest{
		Query:    "anything",
		BundleID: 999,
	})
	if !errors.Is(err, ErrOkfBundleNotFound) {
		t.Errorf("expected ErrOkfBundleNotFound, got %v", err)
	}
}

// TestSearch_EmptyQueryReturnsEmpty verifies the service tolerates an empty
// query by returning an empty page rather than erroring. The HTTP layer is
// expected to reject empty bodies with a 400, but a direct service caller
// should not crash.
func TestSearch_EmptyQueryReturnsEmpty(t *testing.T) {
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	edges := newFakeOkfEdgeRepo(nodes)
	pub := newFakeOkfPublicDir(&model.DiskPublicDirectory{ID: 7, FolderID: 100, FixedPath: "/public/kb"})
	svc := NewOkfServiceFromRepo(bundles, nodes, edges, pub, "sqlite")
	_ = bundles.Create(&model.OkfBundle{ID: 1, PublicDirectoryID: 7, OkfVersion: "0.1", Status: "active"})
	addSearchNode(t, nodes, 1, "concept", "anything")

	out, err := svc.Search(context.Background(), SearchRequest{Query: "", BundleID: 1})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(out.Nodes) != 0 {
		t.Errorf("got %d nodes, want 0 for empty query", len(out.Nodes))
	}
}

// stubVisibility is shared with okf_test.go (defined there). The search
// tests reuse it via SetVisibility so they can exercise the ACL branches
// without standing up a full public directory service.
