package repository

import (
	"github.com/agentdisk/agent-disk/internal/model"
	"gorm.io/gorm"
)

// PublicDirectoryRepo provides data access for public directory operations.
type PublicDirectoryRepo struct {
	db *gorm.DB
}

// NewPublicDirectoryRepo creates a new PublicDirectoryRepo.
func NewPublicDirectoryRepo(db *gorm.DB) *PublicDirectoryRepo {
	return &PublicDirectoryRepo{db: db}
}

// Create inserts a new public directory record.
func (r *PublicDirectoryRepo) Create(pd *model.DiskPublicDirectory) error {
	return r.db.Create(pd).Error
}

// GetByID returns a public directory by its primary key.
func (r *PublicDirectoryRepo) GetByID(id uint64) (*model.DiskPublicDirectory, error) {
	var pd model.DiskPublicDirectory
	if err := r.db.Where("id = ?", id).First(&pd).Error; err != nil {
		return nil, err
	}
	return &pd, nil
}

// GetByFolderID returns a public directory by its associated folder ID.
func (r *PublicDirectoryRepo) GetByFolderID(folderID uint64) (*model.DiskPublicDirectory, error) {
	var pd model.DiskPublicDirectory
	if err := r.db.Where("folder_id = ?", folderID).First(&pd).Error; err != nil {
		return nil, err
	}
	return &pd, nil
}

// ListActive returns all active public directories.
func (r *PublicDirectoryRepo) ListActive() ([]model.DiskPublicDirectory, error) {
	var dirs []model.DiskPublicDirectory
	err := r.db.Where("is_active = ?", true).Order("id ASC").Find(&dirs).Error
	return dirs, err
}

// ListByScope returns active public directories matching the given scope.
func (r *PublicDirectoryRepo) ListByScope(scope string) ([]model.DiskPublicDirectory, error) {
	var dirs []model.DiskPublicDirectory
	err := r.db.Where("is_active = ? AND scope = ?", true, scope).Order("id ASC").Find(&dirs).Error
	return dirs, err
}

// Update updates a public directory record.
func (r *PublicDirectoryRepo) Update(pd *model.DiskPublicDirectory) error {
	return r.db.Save(pd).Error
}

// Delete removes a public directory by ID.
func (r *PublicDirectoryRepo) Delete(id uint64) error {
	return r.db.Where("id = ?", id).Delete(&model.DiskPublicDirectory{}).Error
}
