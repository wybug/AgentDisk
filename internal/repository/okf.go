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
