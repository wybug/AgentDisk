package repository

import (
	"github.com/agentdisk/agent-disk/internal/model"
	"gorm.io/gorm"
)

// SpaceRepo provides data access for user disk quota operations.
type SpaceRepo struct {
	db *gorm.DB
}

// NewSpaceRepo creates a new SpaceRepo.
func NewSpaceRepo(db *gorm.DB) *SpaceRepo {
	return &SpaceRepo{db: db}
}

// GetByUserID returns the UserDisk record for the given user.
func (r *SpaceRepo) GetByUserID(userID string) (*model.UserDisk, error) {
	var ud model.UserDisk
	if err := r.db.Where("user_id = ?", userID).First(&ud).Error; err != nil {
		return nil, err
	}
	return &ud, nil
}

// CreateQuota initializes a new UserDisk quota entry for the given user.
func (r *SpaceRepo) CreateQuota(userID string, quota int64) error {
	ud := model.UserDisk{
		UserID:     userID,
		TotalQuota: quota,
	}
	return r.db.Create(&ud).Error
}

// UpdateUsedQuota adjusts the used quota for a user by delta (positive or negative).
func (r *SpaceRepo) UpdateUsedQuota(userID string, delta int64) error {
	return r.db.Model(&model.UserDisk{}).
		Where("user_id = ?", userID).
		Update("used_quota", gorm.Expr("used_quota + ?", delta)).Error
}
