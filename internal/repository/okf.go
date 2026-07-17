package repository

import (
	"errors"

	"github.com/agentdisk/agent-disk/internal/model"
	"gorm.io/gorm"
)

// OkfBundleRepo provides data access for OKF bundle records.
type OkfBundleRepo struct {
	db *gorm.DB
}

// NewOkfBundleRepo creates a new OkfBundleRepo.
func NewOkfBundleRepo(db *gorm.DB) *OkfBundleRepo {
	return &OkfBundleRepo{db: db}
}

// Create inserts a new bundle record.
func (r *OkfBundleRepo) Create(b *model.OkfBundle) error {
	return r.db.Create(b).Error
}

// GetByID returns a bundle by its primary key.
func (r *OkfBundleRepo) GetByID(id uint64) (*model.OkfBundle, error) {
	var b model.OkfBundle
	if err := r.db.Where("id = ?", id).First(&b).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

// GetByPublicDirectoryID returns the bundle registered against the given
// public directory, or gorm.ErrRecordNotFound if none exists.
func (r *OkfBundleRepo) GetByPublicDirectoryID(pdID uint64) (*model.OkfBundle, error) {
	var b model.OkfBundle
	if err := r.db.Where("public_directory_id = ?", pdID).First(&b).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

// List returns bundles filtered by status. An empty status returns all rows.
// Batches are paged by limit/offset; callers pass 0/0 for "all".
func (r *OkfBundleRepo) List(status string, limit, offset int) ([]model.OkfBundle, error) {
	q := r.db.Model(&model.OkfBundle{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
	}
	var bundles []model.OkfBundle
	err := q.Order("id ASC").Find(&bundles).Error
	return bundles, err
}

// Update saves all fields of the bundle (full save, not partial update).
func (r *OkfBundleRepo) Update(b *model.OkfBundle) error {
	return r.db.Save(b).Error
}

// Delete removes a bundle by ID. This is hard-delete: bundles are meant to be
// unregistered, not soft-deleted, so the underlying public directory can be
// re-registered later.
func (r *OkfBundleRepo) Delete(id uint64) error {
	return r.db.Where("id = ?", id).Delete(&model.OkfBundle{}).Error
}

// OkfNodeRepo provides data access for the OKF materialized node index.
type OkfNodeRepo struct {
	db *gorm.DB
}

// NewOkfNodeRepo creates a new OkfNodeRepo.
func NewOkfNodeRepo(db *gorm.DB) *OkfNodeRepo {
	return &OkfNodeRepo{db: db}
}

// Upsert inserts or updates the node identified by (bundle_id, rel_path). It
// runs within tx when non-nil so callers can batch-refresh a bundle and roll
// back atomically on the first error.
func (r *OkfNodeRepo) Upsert(tx *gorm.DB, n *model.OkfNode) error {
	exec := tx
	if exec == nil {
		exec = r.db
	}
	// Look up by the natural key first so we can preserve the surrogate ID
	// across updates. A raw INSERT ... ON DUPLICATE KEY UPDATE would also
	// work on MySQL but is not portable to the SQLite test driver.
	var existing model.OkfNode
	err := exec.Where("bundle_id = ? AND rel_path = ?", n.BundleID, n.RelPath).First(&existing).Error
	if err == nil {
		n.ID = existing.ID
		n.CreatedAt = existing.CreatedAt
		return exec.Save(n).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return exec.Create(n).Error
}

// GetByBundleAndRelPath returns the node for a bundle + relative path.
func (r *OkfNodeRepo) GetByBundleAndRelPath(bundleID uint64, relPath string) (*model.OkfNode, error) {
	var n model.OkfNode
	if err := r.db.Where("bundle_id = ? AND rel_path = ?", bundleID, relPath).First(&n).Error; err != nil {
		return nil, err
	}
	return &n, nil
}

// ListByBundleAndRelPaths returns the nodes for a bundle whose rel_path is in
// relPaths, in a single query (WHERE bundle_id = ? AND rel_path IN (?)). The
// edge materializer uses it to resolve all of a node's bundle-relative link
// targets at once instead of one GetByBundleAndRelPath per link (an N+1: K links
// used to cost K round trips). relPaths with no match are absent from the result.
func (r *OkfNodeRepo) ListByBundleAndRelPaths(bundleID uint64, relPaths []string) ([]model.OkfNode, error) {
	if len(relPaths) == 0 {
		return nil, nil
	}
	var out []model.OkfNode
	if err := r.db.Where("bundle_id = ? AND rel_path IN ?", bundleID, relPaths).Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// GetByID returns the node for a primary key. P3b graph queries take a nodeID
// from the URL path and need to resolve it back to its bundle for the ACL
// check, so this is the entry point for every reader.
func (r *OkfNodeRepo) GetByID(id uint64) (*model.OkfNode, error) {
	var n model.OkfNode
	if err := r.db.First(&n, id).Error; err != nil {
		return nil, err
	}
	return &n, nil
}

// ListByIDs returns nodes for a set of primary keys, in the order the caller
// passes. P3b graph queries resolve BFS-reachable sets back to node rows for
// the response payload; a batched lookup avoids one query per node.
//
// The returned slice preserves the input order. Missing IDs are silently
// dropped — the BFS layer has already established the IDs exist via the edge
// table, so a miss here would indicate a racing delete and the node simply
// disappears from the response rather than crashing the query.
func (r *OkfNodeRepo) ListByIDs(ids []uint64) ([]model.OkfNode, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var out []model.OkfNode
	if err := r.db.Where("id IN ?", ids).Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// NodeListFilter narrows ListByBundle results by type and/or tag (JSON
// contains). Empty fields mean "no filter on this dimension".
type NodeListFilter struct {
	Type string
	Tag  string
}

// ListByBundle returns nodes for a bundle, optionally narrowed by filter.
// Results are ordered by rel_path for stable output.
func (r *OkfNodeRepo) ListByBundle(bundleID uint64, filter NodeListFilter) ([]model.OkfNode, error) {
	q := r.db.Model(&model.OkfNode{}).Where("bundle_id = ?", bundleID)
	if filter.Type != "" {
		q = q.Where("type = ?", filter.Type)
	}
	if filter.Tag != "" {
		// JSON_CONTAINS works on MySQL; on SQLite the JSON column is stored as
		// TEXT and a LIKE fallback is used. The service layer handles both by
		// switching on the driver — see okf.go ListNodes for the SQLite path.
		q = q.Where("JSON_CONTAINS(tags_json, JSON_QUOTE(?))", filter.Tag)
	}
	var nodes []model.OkfNode
	err := q.Order("rel_path ASC").Find(&nodes).Error
	return nodes, err
}

// ListByBundleSQLite is the SQLite-friendly equivalent of ListByBundle. It
// applies type as a plain equality and tag as a LIKE substring over the JSON
// text, since SQLite lacks JSON_CONTAINS in older versions.
func (r *OkfNodeRepo) ListByBundleSQLite(bundleID uint64, filter NodeListFilter) ([]model.OkfNode, error) {
	q := r.db.Model(&model.OkfNode{}).Where("bundle_id = ?", bundleID)
	if filter.Type != "" {
		q = q.Where("type = ?", filter.Type)
	}
	if filter.Tag != "" {
		// tags_json is stored as TEXT on SQLite; matching the quoted tag value
		// is sufficient for an equality substring scan.
		q = q.Where("tags_json LIKE ?", "%\""+filter.Tag+"\"%")
	}
	var nodes []model.OkfNode
	err := q.Order("rel_path ASC").Find(&nodes).Error
	return nodes, err
}

// TypeCount is one row of the by-type aggregation.
type TypeCount struct {
	Type  string `json:"type"`
	Count uint32 `json:"count"`
}

// AggregateByType returns the per-type node count for a bundle.
func (r *OkfNodeRepo) AggregateByType(bundleID uint64) ([]TypeCount, error) {
	var out []TypeCount
	err := r.db.Model(&model.OkfNode{}).
		Select("type, COUNT(*) as count").
		Where("bundle_id = ?", bundleID).
		Group("type").
		Order("count DESC, type ASC").
		Scan(&out).Error
	return out, err
}

// AggregateTypesByBundles returns the type→count rollup across multiple bundles
// in a single grouped query (WHERE bundle_id IN (?)). It collapses the N+1
// per-bundle queries the global AggregateTypes handler used to issue — a tenant
// with 100 visible bundles now pays one round trip, not 100.
func (r *OkfNodeRepo) AggregateTypesByBundles(bundleIDs []uint64) ([]TypeCount, error) {
	var out []TypeCount
	if len(bundleIDs) == 0 {
		return out, nil
	}
	err := r.db.Model(&model.OkfNode{}).
		Select("type, COUNT(*) as count").
		Where("bundle_id IN ?", bundleIDs).
		Group("type").
		Order("count DESC, type ASC").
		Scan(&out).Error
	return out, err
}

// DeleteByBundle removes all nodes for a bundle. Intended for use during a
// refresh within the same transaction as the new upserts.
func (r *OkfNodeRepo) DeleteByBundle(tx *gorm.DB, bundleID uint64) error {
	exec := tx
	if exec == nil {
		exec = r.db
	}
	return exec.Where("bundle_id = ?", bundleID).Delete(&model.OkfNode{}).Error
}

// CountByBundle returns the total node count for a bundle. tx is the
// in-progress transaction (or nil to use the repo's default handle); passing
// the caller's tx keeps the count inside the same transaction as the
// surrounding upserts so the bundle's node_count is consistent on commit.
func (r *OkfNodeRepo) CountByBundle(tx *gorm.DB, bundleID uint64) (uint32, error) {
	exec := tx
	if exec == nil {
		exec = r.db
	}
	var count int64
	err := exec.Model(&model.OkfNode{}).Where("bundle_id = ?", bundleID).Count(&count).Error
	if err != nil {
		return 0, err
	}
	if count < 0 {
		return 0, nil
	}
	if count > int64(^uint32(0)) {
		return ^uint32(0), nil
	}
	return uint32(count), nil
}

// SearchFilter narrows a Search call by bundle set and optional type. An empty
// BundleIDs slice means "no bundle restriction" — the caller is responsible
// for ACL-filtering the set before passing it in.
type SearchFilter struct {
	BundleIDs []uint64
	Type      string
}

// Search runs a FULLTEXT query against (title, description) on MySQL. The
// ngram parser (set up via migrateOkfFullTextIndex) gives CJK-token-aware
// matching. Pagination is cursor-based on the primary key: pass the last
// node's ID as cursor to fetch the next page; the returned nextCursor is 0
// when the page is the last.
//
// Empty query returns no rows — the caller should branch on that rather than
// issuing a "match everything" search, since FULLTEXT against an empty string
// is undefined behavior across MySQL versions.
func (r *OkfNodeRepo) Search(query string, filter SearchFilter, limit int, cursor uint64) ([]model.OkfNode, uint64, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if query == "" {
		return nil, 0, nil
	}
	q := r.db.Model(&model.OkfNode{}).
		Where("MATCH(title, description) AGAINST(? IN NATURAL LANGUAGE MODE)", query)
	if len(filter.BundleIDs) > 0 {
		q = q.Where("bundle_id IN ?", filter.BundleIDs)
	}
	if filter.Type != "" {
		q = q.Where("type = ?", filter.Type)
	}
	if cursor > 0 {
		q = q.Where("id > ?", cursor)
	}
	var out []model.OkfNode
	if err := q.Order("id ASC").Limit(limit + 1).Find(&out).Error; err != nil {
		return nil, 0, err
	}
	next := uint64(0)
	if len(out) > limit {
		next = out[limit-1].ID
		out = out[:limit]
	}
	return out, next, nil
}

// SearchSQLite is the SQLite equivalent of Search. It runs a MATCH query
// against the disk_okf_node_fts FTS5 virtual table (set up by
// migrateOkfFts5Index) and joins back to the base table for the full row.
// FTS5's unicode61 tokenizer handles ASCII word boundaries and CJK code
// points; ranking is by FTS5's default bm25.
//
// Cursor pagination runs on the joined base table's primary key so the
// caller can walk pages without re-running the ranking. The first page
// comes back ranked; subsequent pages lose the ranking but stay cheap.
//
// On the SQLite test path this is what's exercised; on the MySQL path
// the FULLTEXT index covers the same shape.
func (r *OkfNodeRepo) SearchSQLite(query string, filter SearchFilter, limit int, cursor uint64) ([]model.OkfNode, uint64, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if query == "" {
		return nil, 0, nil
	}
	q := r.db.Model(&model.OkfNode{}).
		Joins("JOIN disk_okf_node_fts ON disk_okf_node_fts.rowid = disk_okf_node.id").
		Where("disk_okf_node_fts MATCH ?", query)
	if len(filter.BundleIDs) > 0 {
		q = q.Where("disk_okf_node.bundle_id IN ?", filter.BundleIDs)
	}
	if filter.Type != "" {
		q = q.Where("disk_okf_node.type = ?", filter.Type)
	}
	if cursor > 0 {
		q = q.Where("disk_okf_node.id > ?", cursor)
	}
	var out []model.OkfNode
	if err := q.Order("disk_okf_node.id ASC").Limit(limit + 1).Find(&out).Error; err != nil {
		return nil, 0, err
	}
	next := uint64(0)
	if len(out) > limit {
		next = out[limit-1].ID
		out = out[:limit]
	}
	return out, next, nil
}
