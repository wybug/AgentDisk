package repository

import (
	"strconv"
	"strings"
	"testing"

	"github.com/agentdisk/agent-disk/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newSearchDB opens an in-memory SQLite DB with the OKF tables + the FTS5
// virtual table + sync triggers. This mirrors what production SQLite
// deployments get via AutoMigrate → migrateOkfFts5Index; the graph test
// helper (newGraphDB) skips FTS5 because graph tests do not exercise MATCH.
//
// The DSN-unique name keeps each test's in-memory DB isolated; without it,
// cache=shared would hand every test the same memory DB and the bundle
// uniqueness constraint would trip on the second test that seeds pdID=7.
//
// FTS5 is gated on a build tag (go test -tags fts5); without it,
// mattn/go-sqlite3 returns "no such module: fts5" on the CREATE VIRTUAL
// TABLE. We skip the test in that case rather than failing — production
// builds set the tag, but the lint/default-test sweep should still pass on
// a stock toolchain.
func newSearchDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:okf_search_" + sanitizeFTSName(t.Name()) + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.OkfBundle{}, &model.OkfNode{}, &model.OkfEdge{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	if err := migrateOkfFts5Index(db); err != nil {
		if strings.Contains(err.Error(), "no such module: fts5") {
			t.Skip("fts5 module not compiled in; rebuild with -tags fts5 to exercise this test")
		}
		t.Fatalf("fts5 migrate: %v", err)
	}
	return db
}

// sanitizeFTSName mirrors sanitizeDSN but inlined here so the search test
// file stays self-contained. Reusing sanitizeDSN directly would work, but
// the helper is test-fixture glue and keeping the search tests decoupled
// from the graph test file makes it easier to reason about in isolation.
func sanitizeFTSName(s string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "'", "_")
	return r.Replace(s)
}

// seedSearchBundle inserts a bundle row + several nodes. Returns the bundle
// id so tests can scope queries. The nodes' titles are written verbatim so
// tests can reason about what FTS5 should match.
func seedSearchBundle(t *testing.T, db *gorm.DB, pdID uint64, nodes []*model.OkfNode) uint64 {
	t.Helper()
	bundle := &model.OkfBundle{PublicDirectoryID: pdID, OkfVersion: "0.1", Status: "active"}
	if err := db.Create(bundle).Error; err != nil {
		t.Fatalf("create bundle: %v", err)
	}
	for i, n := range nodes {
		n.BundleID = bundle.ID
		if n.RelPath == "" {
			// RelPath is unique per bundle; derive from the title slug + index
			// so two nodes that share a leading word ("Gemma card" /
			// "Gemma tuning") don't collide on "gemma.md".
			slug := "node"
			if fields := strings.Fields(n.Title); len(fields) > 0 {
				slug = strings.ToLower(fields[0])
			}
			n.RelPath = slug + "-" + strconv.Itoa(i) + ".md"
		}
		if n.Type == "" {
			n.Type = "concept"
		}
		if err := db.Create(n).Error; err != nil {
			t.Fatalf("create node[%d]: %v", i, err)
		}
	}
	return bundle.ID
}

// TestOkfNodeRepo_SearchSQLite_Match exercises the happy path: write three
// nodes with distinct titles, two of which share a token with the query, and
// confirm MATCH returns exactly the matching rows.
func TestOkfNodeRepo_SearchSQLite_Match(t *testing.T) {
	db := newSearchDB(t)
	repo := NewOkfNodeRepo(db)
	bundleID := seedSearchBundle(t, db, 7, []*model.OkfNode{
		{Title: "Gemma model card", Description: "overview of the Gemma LLM"},
		{Title: "Transformer primer", Description: "attention is all you need"},
		{Title: "Gemma tuning guide", Description: "fine-tuning recipes"},
	})

	out, next, err := repo.SearchSQLite("gemma", SearchFilter{BundleIDs: []uint64{bundleID}}, 50, 0)
	if err != nil {
		t.Fatalf("SearchSQLite: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("got %d rows, want 2 (both Gemma-titled nodes)", len(out))
	}
	if next != 0 {
		t.Errorf("next = %d, want 0 (single page)", next)
	}
	for _, n := range out {
		if !strings.Contains(n.Title, "Gemma") {
			t.Errorf("row title = %q, want a Gemma match", n.Title)
		}
	}
}

// TestOkfNodeRepo_SearchSQLite_NoMatch verifies a query that hits zero rows
// returns an empty slice + zero cursor rather than erroring.
func TestOkfNodeRepo_SearchSQLite_NoMatch(t *testing.T) {
	db := newSearchDB(t)
	repo := NewOkfNodeRepo(db)
	bundleID := seedSearchBundle(t, db, 7, []*model.OkfNode{
		{Title: "Gemma model card"},
	})

	out, _, err := repo.SearchSQLite("nonexistentterm", SearchFilter{BundleIDs: []uint64{bundleID}}, 50, 0)
	if err != nil {
		t.Fatalf("SearchSQLite: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("got %d rows, want 0", len(out))
	}
}

// TestOkfNodeRepo_SearchSQLite_EmptyQuery confirms the repo short-circuits
// an empty query to (nil, 0, nil). MATCH on an empty string is undefined
// across SQLite builds; the service layer relies on this guard.
func TestOkfNodeRepo_SearchSQLite_EmptyQuery(t *testing.T) {
	db := newSearchDB(t)
	repo := NewOkfNodeRepo(db)
	seedSearchBundle(t, db, 7, []*model.OkfNode{{Title: "anything"}})

	out, next, err := repo.SearchSQLite("", SearchFilter{}, 50, 0)
	if err != nil {
		t.Fatalf("SearchSQLite: %v", err)
	}
	if out != nil {
		t.Errorf("got %d rows, want nil for empty query", len(out))
	}
	if next != 0 {
		t.Errorf("next = %d, want 0 for empty query", next)
	}
}

// TestOkfNodeRepo_SearchSQLite_BundleFilter ensures the bundle-set filter
// excludes nodes from bundles the caller did not ask for. Two bundles each
// have a matching node; only the requested bundle's node comes back.
func TestOkfNodeRepo_SearchSQLite_BundleFilter(t *testing.T) {
	db := newSearchDB(t)
	repo := NewOkfNodeRepo(db)
	a := seedSearchBundle(t, db, 7, []*model.OkfNode{{Title: "Gemma card"}})
	b := seedSearchBundle(t, db, 8, []*model.OkfNode{{Title: "Gemma tuning"}})

	out, _, err := repo.SearchSQLite("gemma", SearchFilter{BundleIDs: []uint64{a}}, 50, 0)
	if err != nil {
		t.Fatalf("SearchSQLite: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("got %d rows, want 1 (only bundle a)", len(out))
	}
	if out[0].BundleID != a {
		t.Errorf("row.BundleID = %d, want %d", out[0].BundleID, a)
	}
	// Sanity: the other bundle's node exists and is also a match.
	if a == b {
		t.Fatalf("bundle ids collided: %d", a)
	}
}

// TestOkfNodeRepo_SearchSQLite_TypeFilter layers a type restriction on top
// of the MATCH. A matching title under the wrong type must not surface.
func TestOkfNodeRepo_SearchSQLite_TypeFilter(t *testing.T) {
	db := newSearchDB(t)
	repo := NewOkfNodeRepo(db)
	bundleID := seedSearchBundle(t, db, 7, []*model.OkfNode{
		{Title: "Gemma model", Type: "concept"},
		{Title: "Gemma quickstart", Type: "guide"},
	})

	out, _, err := repo.SearchSQLite("gemma", SearchFilter{BundleIDs: []uint64{bundleID}, Type: "concept"}, 50, 0)
	if err != nil {
		t.Fatalf("SearchSQLite: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("got %d rows, want 1 (type-filtered)", len(out))
	}
	if out[0].Type != "concept" {
		t.Errorf("row.Type = %q, want concept", out[0].Type)
	}
}

// TestOkfNodeRepo_SearchSQLite_Pagination walks two pages of a wide query.
// Cursor is the last seen id; the second page picks up the remainder.
func TestOkfNodeRepo_SearchSQLite_Pagination(t *testing.T) {
	db := newSearchDB(t)
	repo := NewOkfNodeRepo(db)
	bundleID := seedSearchBundle(t, db, 7, []*model.OkfNode{
		{Title: "Gemma card"},
		{Title: "Gemma tuning"},
		{Title: "Gemma advanced"},
	})

	page1, next, err := repo.SearchSQLite("gemma", SearchFilter{BundleIDs: []uint64{bundleID}}, 2, 0)
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("page1 len = %d, want 2", len(page1))
	}
	if next == 0 {
		t.Fatalf("page1 next = 0, want non-zero (more pages)")
	}

	page2, next2, err := repo.SearchSQLite("gemma", SearchFilter{BundleIDs: []uint64{bundleID}}, 2, next)
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if len(page2) != 1 {
		t.Errorf("page2 len = %d, want 1", len(page2))
	}
	if next2 != 0 {
		t.Errorf("page2 next = %d, want 0 (last page)", next2)
	}
}

// TestOkfNodeRepo_SearchSQLite_TriggerSyncOnUpdate verifies the AFTER UPDATE
// trigger keeps the FTS index honest: a node whose title did not match the
// query before an UPDATE must match after the title changes.
//
// This is the regression test for the trigger wiring. If the trigger is ever
// dropped or its rowid mapping broken, this test will fail because the new
// title will never land in the index.
func TestOkfNodeRepo_SearchSQLite_TriggerSyncOnUpdate(t *testing.T) {
	db := newSearchDB(t)
	repo := NewOkfNodeRepo(db)
	bundleID := seedSearchBundle(t, db, 7, []*model.OkfNode{
		{Title: "Original title"},
	})

	// Before update: a "rewritten" query hits zero rows.
	out, _, err := repo.SearchSQLite("rewritten", SearchFilter{BundleIDs: []uint64{bundleID}}, 50, 0)
	if err != nil {
		t.Fatalf("search before update: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("pre-update got %d rows, want 0", len(out))
	}

	// Rewrite the title in place. The trigger should delete the old FTS row
	// and insert the new one.
	var node model.OkfNode
	if err := db.First(&node, "bundle_id = ?", bundleID).Error; err != nil {
		t.Fatalf("find node: %v", err)
	}
	node.Title = "Rewritten title"
	if err := db.Save(&node).Error; err != nil {
		t.Fatalf("save node: %v", err)
	}

	out, _, err = repo.SearchSQLite("rewritten", SearchFilter{BundleIDs: []uint64{bundleID}}, 50, 0)
	if err != nil {
		t.Fatalf("search after update: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("post-update got %d rows, want 1 (trigger must index new title)", len(out))
	}
}

// TestOkfNodeRepo_SearchSQLite_TriggerSyncOnDelete verifies the AFTER DELETE
// trigger removes the row from the FTS index. If the trigger is missing,
// MATCH would keep surfacing the deleted node's title.
func TestOkfNodeRepo_SearchSQLite_TriggerSyncOnDelete(t *testing.T) {
	db := newSearchDB(t)
	repo := NewOkfNodeRepo(db)
	bundleID := seedSearchBundle(t, db, 7, []*model.OkfNode{
		{Title: "Doomed node"},
	})

	out, _, err := repo.SearchSQLite("doomed", SearchFilter{BundleIDs: []uint64{bundleID}}, 50, 0)
	if err != nil {
		t.Fatalf("search before delete: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("pre-delete got %d rows, want 1", len(out))
	}

	if err := db.Where("bundle_id = ?", bundleID).Delete(&model.OkfNode{}).Error; err != nil {
		t.Fatalf("delete node: %v", err)
	}

	out, _, err = repo.SearchSQLite("doomed", SearchFilter{BundleIDs: []uint64{bundleID}}, 50, 0)
	if err != nil {
		t.Fatalf("search after delete: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("post-delete got %d rows, want 0 (trigger must evict from index)", len(out))
	}
}

// TestOkfNodeRepo_SearchSQLite_MatchesBody verifies the FTS index includes the
// body column: a query term that appears only in the body (not title or
// description) still matches. Regression guard for the body-indexing change.
func TestOkfNodeRepo_SearchSQLite_MatchesBody(t *testing.T) {
	db := newSearchDB(t)
	repo := NewOkfNodeRepo(db)
	bundleID := seedSearchBundle(t, db, 7, []*model.OkfNode{
		{Title: "Quantum primer", Description: "introduction", Body: "the quick brown fox jumps over the lazy dog"},
		{Title: "Unrelated node", Description: "nothing here"},
	})

	out, _, err := repo.SearchSQLite("fox", SearchFilter{BundleIDs: []uint64{bundleID}}, 50, 0)
	if err != nil {
		t.Fatalf("SearchSQLite: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("got %d rows, want 1 (body-only match)", len(out))
	}
	if !strings.Contains(strings.ToLower(out[0].Title), "quantum") {
		t.Errorf("matched title = %q, want the Quantum primer (body contains 'fox')", out[0].Title)
	}
}

// TestOkfNodeRepo_SearchSQLite_RanksByRelevance verifies results come back in
// bm25 relevance order: a node where the query term is prominent (title +
// description + repeated in body) outranks one where it appears once.
func TestOkfNodeRepo_SearchSQLite_RanksByRelevance(t *testing.T) {
	db := newSearchDB(t)
	repo := NewOkfNodeRepo(db)
	bundleID := seedSearchBundle(t, db, 7, []*model.OkfNode{
		{Title: "Glossary", Description: "miscellaneous terms", Body: "the word gemma appears here only once"},
		{Title: "Gemma deep dive", Description: "all about the Gemma model", Body: "gemma gemma gemma architecture details"},
	})

	out, _, err := repo.SearchSQLite("gemma", SearchFilter{BundleIDs: []uint64{bundleID}}, 50, 0)
	if err != nil {
		t.Fatalf("SearchSQLite: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("got %d rows, want 2", len(out))
	}
	if !strings.Contains(strings.ToLower(out[0].Title), "deep dive") {
		t.Errorf("top result = %q, want the Gemma deep dive (more relevant by bm25)", out[0].Title)
	}
}
