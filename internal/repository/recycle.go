package repository

import (
	"time"

	"github.com/agentdisk/agent-disk/internal/model"
	"gorm.io/gorm"
)

// RecycleRepo provides data access for recycle bin operations.
type RecycleRepo struct {
	db *gorm.DB
}

// NewRecycleRepo creates a new RecycleRepo.
func NewRecycleRepo(db *gorm.DB) *RecycleRepo {
	return &RecycleRepo{db: db}
}

// Create inserts a new recycle bin record.
func (r *RecycleRepo) Create(recycle *model.DiskRecycleBin) error {
	return r.db.Create(recycle).Error
}

// ListByUser returns all recycle bin records for a specific user.
func (r *RecycleRepo) ListByUser(userID string) ([]model.DiskRecycleBin, error) {
	var items []model.DiskRecycleBin
	err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&items).Error
	return items, err
}

// GetByID returns a recycle bin record by its primary key.
func (r *RecycleRepo) GetByID(id uint64) (*model.DiskRecycleBin, error) {
	var item model.DiskRecycleBin
	if err := r.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

// Delete removes a recycle bin record by its primary key.
func (r *RecycleRepo) Delete(id uint64) error {
	return r.db.Where("id = ?", id).Delete(&model.DiskRecycleBin{}).Error
}

// DeleteExpired removes all recycle bin records whose expire_at is before now.
func (r *RecycleRepo) DeleteExpired() error {
	return r.db.Where("expire_at < ?", time.Now()).
		Delete(&model.DiskRecycleBin{}).Error
}
