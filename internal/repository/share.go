package repository

import (
	"github.com/agentdisk/agent-disk/internal/model"
	"gorm.io/gorm"
)

// ShareRepo provides data access for share and share-access-log operations.
type ShareRepo struct {
	db *gorm.DB
}

// NewShareRepo creates a new ShareRepo.
func NewShareRepo(db *gorm.DB) *ShareRepo {
	return &ShareRepo{db: db}
}

// Create inserts a new share record.
func (r *ShareRepo) Create(s *model.DiskShare) error {
	return r.db.Create(s).Error
}

// GetByCode returns a share record by its share code.
func (r *ShareRepo) GetByCode(code string) (*model.DiskShare, error) {
	var s model.DiskShare
	if err := r.db.Where("share_code = ?", code).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

// GetByID returns a share record by its primary key.
func (r *ShareRepo) GetByID(id uint64) (*model.DiskShare, error) {
	var s model.DiskShare
	if err := r.db.Where("id = ?", id).First(&s).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

// IncrementVisitCount atomically increments visit_count for a share.
func (r *ShareRepo) IncrementVisitCount(id uint64) error {
	return r.db.Model(&model.DiskShare{}).
		Where("id = ?", id).
		Update("visit_count", gorm.Expr("visit_count + 1")).Error
}

// RevokeByID sets is_active = false for a share by ID.
func (r *ShareRepo) RevokeByID(id uint64) error {
	return r.db.Model(&model.DiskShare{}).Where("id = ?", id).Update("is_active", false).Error
}

// ListByUser returns all share records for a specific user.
func (r *ShareRepo) ListByUser(userID string) ([]model.DiskShare, error) {
	var shares []model.DiskShare
	err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&shares).Error
	return shares, err
}

// LogAccess inserts a share access log record.
func (r *ShareRepo) LogAccess(log *model.ShareAccessLog) error {
	return r.db.Create(log).Error
}

// ListAccessLogsByShare returns the most-recent access-log rows for a share,
// newest first, capped at limit. Used by the owner-facing share stats view.
// A non-positive limit returns all rows for the share.
func (r *ShareRepo) ListAccessLogsByShare(shareID uint64, limit int) ([]model.ShareAccessLog, error) {
	var logs []model.ShareAccessLog
	q := r.db.Where("share_id = ?", shareID).Order("created_at DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

// ShareAccessStatsRow holds the distinct-visitor count for a share. The
// last-access time is derived by the service from the most-recent log row
// rather than scanned from a MAX(created_at) aggregate — SQLite returns that
// aggregate as a string that database/sql cannot scan into time.Time.
type ShareAccessStatsRow struct {
	UniqueIPs uint64 `gorm:"column:unique_ips"`
}

// ShareAccessStats returns the distinct-visitor-IP count for a share (zero
// when never accessed). Uses SQL aggregation so the count stays accurate
// regardless of how many raw log rows exist.
func (r *ShareRepo) ShareAccessStats(shareID uint64) (ShareAccessStatsRow, error) {
	var row ShareAccessStatsRow
	err := r.db.Model(&model.ShareAccessLog{}).
		Select("COUNT(DISTINCT visitor_ip) AS unique_ips").
		Where("share_id = ?", shareID).
		Scan(&row).Error
	if err != nil {
		return ShareAccessStatsRow{}, err
	}
	return row, nil
}
