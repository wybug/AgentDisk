package repository

import (
	"github.com/agentdisk/agent-disk/internal/model"
	"gorm.io/gorm"
)

// PublicDirectoryGrantRepo provides data access for public directory user grants.
type PublicDirectoryGrantRepo struct {
	db *gorm.DB
}

// NewPublicDirectoryGrantRepo creates a new PublicDirectoryGrantRepo.
func NewPublicDirectoryGrantRepo(db *gorm.DB) *PublicDirectoryGrantRepo {
	return &PublicDirectoryGrantRepo{db: db}
}

// Create inserts a new grant record.
func (r *PublicDirectoryGrantRepo) Create(grant *model.DiskPublicDirectoryGrant) error {
	return r.db.Create(grant).Error
}

// Delete removes a grant by public directory ID and user ID.
func (r *PublicDirectoryGrantRepo) Delete(publicDirID uint64, userID string) error {
	return r.db.Where("public_directory_id = ? AND user_id = ?", publicDirID, userID).
		Delete(&model.DiskPublicDirectoryGrant{}).Error
}

// ListByPublicDirID returns all user IDs granted access to a public directory.
func (r *PublicDirectoryGrantRepo) ListByPublicDirID(publicDirID uint64) ([]model.DiskPublicDirectoryGrant, error) {
	var grants []model.DiskPublicDirectoryGrant
	err := r.db.Where("public_directory_id = ?", publicDirID).Order("id ASC").Find(&grants).Error
	return grants, err
}

// Exists checks if a user has been granted access to a public directory.
func (r *PublicDirectoryGrantRepo) Exists(publicDirID uint64, userID string) (bool, error) {
	var count int64
	err := r.db.Model(&model.DiskPublicDirectoryGrant{}).
		Where("public_directory_id = ? AND user_id = ?", publicDirID, userID).
		Count(&count).Error
	return count > 0, err
}

// ListByUserID returns all public directory IDs granted to a user.
func (r *PublicDirectoryGrantRepo) ListByUserID(userID string) ([]uint64, error) {
	var ids []uint64
	err := r.db.Model(&model.DiskPublicDirectoryGrant{}).
		Where("user_id = ?", userID).
		Pluck("public_directory_id", &ids).Error
	return ids, err
}
