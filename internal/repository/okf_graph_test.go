package repository

import (
	"fmt"
	"strings"
	"testing"

	"github.com/agentdisk/agent-disk/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newGraphDB opens an in-memory SQLite database with the OKF tables migrated.
// Closing the underlying sql.DB is unnecessary in tests — the process exits
// and the memory DB is reclaimed.
//
// The DSN's DSN-unique name (per test) avoids cache=shared collisions across
// tests; otherwise every test reuses one shared in-memory database and the
// unique constraint on disk_okf_bundle.public_directory_id trips on the second
// test that seeds pdID=7.
func newGraphDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:okf_%s?mode=memory&cache=shared", sanitizeDSN(t.Name()))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.OkfBundle{}, &model.OkfNode{}, &model.OkfEdge{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	return db
}

// sanitizeDSN strips SQLite DSN-illegal characters from the test name so
// "TestOkfEdgeRepo_ReplaceForSrc_Inserts" becomes a valid filename token.
func sanitizeDSN(s string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "'", "_")
	return r.Replace(s)
}

// seedBundle creates one bundle row for a public directory. Each test seeds
// exactly one bundle and then attaches nodes to it, mirroring the production
// shape where a bundle owns many nodes.
func seedBundle(t *testing.T, db *gorm.DB, pdID uint64) uint64 {
	t.Helper()
	bundle := &model.OkfBundle{PublicDirectoryID: pdID, OkfVersion: "0.1", Status: "active"}
	if err := db.Create(bundle).Error; err != nil {
		t.Fatalf("create bundle: %v", err)
	}
	return bundle.ID
}

// addNode inserts a node under an existing bundle. Returns the node id.
func addNode(t *testing.T, db *gorm.DB, bundleID uint64, relPath string) uint64 {
	t.Helper()
	node := &model.OkfNode{BundleID: bundleID, FileID: 1, RelPath: relPath, Type: "concept"}
	if err := db.Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	return node.ID
}

// TestOkfEdgeRepo_ReplaceForSrc_Inserts verifies the happy-path replace: a
// first call writes 3 edges, the second call writes 1 edge, the table must
// reflect only the latest set.
func TestOkfEdgeRepo_ReplaceForSrc_Inserts(t *testing.T) {
	db := newGraphDB(t)
	repo := NewOkfEdgeRepo(db)
	bundle := seedBundle(t, db, 7)
	srcID := addNode(t, db, bundle, "a.md")

	first := []model.OkfEdge{
		{PublicDirID: 7, SrcNodeID: srcID, DstRelPath: "b.md", SrcLine: 1, LinkKind: "bundle"},
		{PublicDirID: 7, SrcNodeID: srcID, DstRelPath: "c.md", SrcLine: 2, LinkKind: "bundle"},
		{PublicDirID: 7, SrcNodeID: srcID, DstRelPath: "d.md", SrcLine: 3, LinkKind: "bundle"},
	}
	if err := repo.ReplaceForSrc(nil, 7, srcID, first); err != nil {
		t.Fatalf("replace#1: %v", err)
	}
	got, err := repo.ListBySrc(7, srcID, 0)
	if err != nil {
		t.Fatalf("list#1: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("after first replace: %d edges, want 3", len(got))
	}

	// Second replace with a single edge must wipe the other two.
	second := []model.OkfEdge{
		{PublicDirID: 7, SrcNodeID: srcID, DstRelPath: "z.md", SrcLine: 9, LinkKind: "bundle"},
	}
	if err := repo.ReplaceForSrc(nil, 7, srcID, second); err != nil {
		t.Fatalf("replace#2: %v", err)
	}
	got, err = repo.ListBySrc(7, srcID, 0)
	if err != nil {
		t.Fatalf("list#2: %v", err)
	}
	if len(got) != 1 || got[0].DstRelPath != "z.md" {
		t.Errorf("after second replace: %+v, want [z.md]", got)
	}
}

// TestOkfEdgeRepo_ReplaceForSrc_EmptyClearsExisting confirms that passing an
// empty slice is the legitimate "no outgoing edges" case and clears prior
// rows rather than leaving them.
func TestOkfEdgeRepo_ReplaceForSrc_EmptyClearsExisting(t *testing.T) {
	db := newGraphDB(t)
	repo := NewOkfEdgeRepo(db)
	bundle := seedBundle(t, db, 7)
	srcID := addNode(t, db, bundle, "a.md")

	if err := repo.ReplaceForSrc(nil, 7, srcID, []model.OkfEdge{
		{PublicDirID: 7, SrcNodeID: srcID, DstRelPath: "b.md", SrcLine: 1, LinkKind: "bundle"},
	}); err != nil {
		t.Fatalf("replace#1: %v", err)
	}
	if err := repo.ReplaceForSrc(nil, 7, srcID, nil); err != nil {
		t.Fatalf("replace#2: %v", err)
	}
	got, _ := repo.ListBySrc(7, srcID, 0)
	if len(got) != 0 {
		t.Errorf("after empty replace: %d edges, want 0", len(got))
	}
}

// TestOkfEdgeRepo_ListByDst_FiltersLive verifies ListByDst returns only live
// edges (dst_exists=true), which is what backlink traversal depends on.
func TestOkfEdgeRepo_ListByDst_FiltersLive(t *testing.T) {
	db := newGraphDB(t)
	repo := NewOkfEdgeRepo(db)
	bundle := seedBundle(t, db, 7)
	srcID := addNode(t, db, bundle, "a.md")
	dstID := addNode(t, db, bundle, "b.md")

	if err := repo.ReplaceForSrc(nil, 7, srcID, []model.OkfEdge{
		{PublicDirID: 7, SrcNodeID: srcID, DstNodeID: dstID, DstRelPath: "b.md", DstExists: true, SrcLine: 1, LinkKind: "bundle"},
		{PublicDirID: 7, SrcNodeID: srcID, DstNodeID: 0, DstRelPath: "missing.md", DstExists: false, SrcLine: 2, LinkKind: "bundle"},
	}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	got, err := repo.ListByDst(7, dstID, 0)
	if err != nil {
		t.Fatalf("ListByDst: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d incoming, want 1 (live only)", len(got))
	}
	if got[0].DstNodeID != dstID {
		t.Errorf("DstNodeID = %d, want %d", got[0].DstNodeID, dstID)
	}
}

// TestOkfEdgeRepo_ListBrokenByBundle_Pagination verifies the cursor paging
// shape: a bundle with 3 dead links paged at limit=2 yields 2 then 1, with
// nextCursor carrying across the page boundary.
func TestOkfEdgeRepo_ListBrokenByBundle_Pagination(t *testing.T) {
	db := newGraphDB(t)
	repo := NewOkfEdgeRepo(db)
	bundle := seedBundle(t, db, 7)
	srcID := addNode(t, db, bundle, "a.md")

	edges := []model.OkfEdge{
		{PublicDirID: 7, SrcNodeID: srcID, DstRelPath: "m1.md", DstExists: false, SrcLine: 1, LinkKind: "bundle"},
		{PublicDirID: 7, SrcNodeID: srcID, DstRelPath: "m2.md", DstExists: false, SrcLine: 2, LinkKind: "bundle"},
		{PublicDirID: 7, SrcNodeID: srcID, DstRelPath: "m3.md", DstExists: false, SrcLine: 3, LinkKind: "bundle"},
	}
	if err := repo.ReplaceForSrc(nil, 7, srcID, edges); err != nil {
		t.Fatalf("replace: %v", err)
	}

	page1, cur, err := repo.ListBrokenByBundle(7, 0, 2)
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("page1 len = %d, want 2", len(page1))
	}
	if cur == 0 {
		t.Fatalf("page1 nextCursor = 0, want non-zero (more pages)")
	}
	page2, cur2, err := repo.ListBrokenByBundle(7, cur, 2)
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if len(page2) != 1 {
		t.Errorf("page2 len = %d, want 1", len(page2))
	}
	if cur2 != 0 {
		t.Errorf("page2 nextCursor = %d, want 0 (last page)", cur2)
	}
}

// TestOkfEdgeRepo_CountByBundle asserts the count is scoped to the bundle's
// public_dir_id, not affected by edges in another bundle.
func TestOkfEdgeRepo_CountByBundle(t *testing.T) {
	db := newGraphDB(t)
	repo := NewOkfEdgeRepo(db)
	bundleA := seedBundle(t, db, 7)
	bundleB := seedBundle(t, db, 8)
	srcA := addNode(t, db, bundleA, "a.md")
	srcB := addNode(t, db, bundleB, "x.md")

	if err := repo.ReplaceForSrc(nil, 7, srcA, []model.OkfEdge{
		{PublicDirID: 7, SrcNodeID: srcA, DstRelPath: "b.md", SrcLine: 1, LinkKind: "bundle"},
	}); err != nil {
		t.Fatalf("replace A: %v", err)
	}
	if err := repo.ReplaceForSrc(nil, 8, srcB, []model.OkfEdge{
		{PublicDirID: 8, SrcNodeID: srcB, DstRelPath: "y.md", SrcLine: 1, LinkKind: "bundle"},
		{PublicDirID: 8, SrcNodeID: srcB, DstRelPath: "z.md", SrcLine: 2, LinkKind: "bundle"},
	}); err != nil {
		t.Fatalf("replace B: %v", err)
	}
	if got, _ := repo.CountByBundle(nil, 0, 7); got != 1 {
		t.Errorf("bundle 7 count = %d, want 1", got)
	}
	if got, _ := repo.CountByBundle(nil, 0, 8); got != 2 {
		t.Errorf("bundle 8 count = %d, want 2", got)
	}
}

// TestOkfEdgeRepo_DeleteByBundle removes one bundle's edges and leaves the
// other untouched.
func TestOkfEdgeRepo_DeleteByBundle(t *testing.T) {
	db := newGraphDB(t)
	repo := NewOkfEdgeRepo(db)
	bundleA := seedBundle(t, db, 7)
	bundleB := seedBundle(t, db, 8)
	srcA := addNode(t, db, bundleA, "a.md")
	srcB := addNode(t, db, bundleB, "x.md")

	_ = repo.ReplaceForSrc(nil, 7, srcA, []model.OkfEdge{
		{PublicDirID: 7, SrcNodeID: srcA, DstRelPath: "b.md", SrcLine: 1, LinkKind: "bundle"},
	})
	_ = repo.ReplaceForSrc(nil, 8, srcB, []model.OkfEdge{
		{PublicDirID: 8, SrcNodeID: srcB, DstRelPath: "y.md", SrcLine: 1, LinkKind: "bundle"},
	})

	if err := repo.DeleteByBundle(nil, 7); err != nil {
		t.Fatalf("DeleteByBundle: %v", err)
	}
	if got, _ := repo.CountByBundle(nil, 0, 7); got != 0 {
		t.Errorf("bundle 7 count after delete = %d, want 0", got)
	}
	if got, _ := repo.CountByBundle(nil, 0, 8); got != 1 {
		t.Errorf("bundle 8 count after delete = %d, want 1 (untouched)", got)
	}
}

// TestOkfEdgeRepo_AdjustBacklinks_BumpsClamped exercises the +1/-1 update and
// the clamp-at-zero guard so a buggy diff cannot produce a negative count.
func TestOkfEdgeRepo_AdjustBacklinks_BumpsClamped(t *testing.T) {
	db := newGraphDB(t)
	repo := NewOkfEdgeRepo(db)
	bundle := seedBundle(t, db, 7)
	n1 := addNode(t, db, bundle, "a.md")
	n2 := addNode(t, db, bundle, "b.md")

	// Bump both to 1.
	if err := repo.AdjustBacklinks(nil, []uint64{n1, n2}, nil); err != nil {
		t.Fatalf("increment: %v", err)
	}
	var node1, node2 model.OkfNode
	db.First(&node1, n1)
	db.First(&node2, n2)
	if node1.BacklinkCount != 1 || node2.BacklinkCount != 1 {
		t.Errorf("after +1: n1=%d n2=%d, want 1/1", node1.BacklinkCount, node2.BacklinkCount)
	}
	// Decrement n1 twice — second decrement clamps at 0.
	if err := repo.AdjustBacklinks(nil, nil, []uint64{n1, n1}); err != nil {
		t.Fatalf("decrement: %v", err)
	}
	db.First(&node1, n1)
	if node1.BacklinkCount != 0 {
		t.Errorf("after 2x decrement: n1=%d, want 0 (clamped)", node1.BacklinkCount)
	}
}
