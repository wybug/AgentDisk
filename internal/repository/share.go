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
