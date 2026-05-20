package repository

import (
	"github.com/agentdisk/agent-disk/internal/model"
	"gorm.io/gorm"
)

// FolderRepo provides data access for disk folder operations.
type FolderRepo struct {
	db *gorm.DB
}

// NewFolderRepo creates a new FolderRepo.
func NewFolderRepo(db *gorm.DB) *FolderRepo {
	return &FolderRepo{db: db}
}

// Create inserts a new folder record.
func (r *FolderRepo) Create(folder *model.DiskFolder) error {
	return r.db.Create(folder).Error
}

// GetByID returns a folder by its primary key.
func (r *FolderRepo) GetByID(id uint64) (*model.DiskFolder, error) {
	var folder model.DiskFolder
	if err := r.db.Where("id = ?", id).First(&folder).Error; err != nil {
		return nil, err
	}
	return &folder, nil
}

// ListByParent returns all non-deleted folders under a given parent for a specific user.
func (r *FolderRepo) ListByParent(userID string, parentID uint64) ([]model.DiskFolder, error) {
	var folders []model.DiskFolder
	err := r.db.Where("user_id = ? AND parent_id = ? AND is_deleted = ?", userID, parentID, false).
		Order("sort_order ASC").
		Find(&folders).Error
	return folders, err
}

// UpdateNameAndPath sets folder_name and full_path for a folder.
func (r *FolderRepo) UpdateNameAndPath(id uint64, folderName, fullPath string) error {
	return r.db.Model(&model.DiskFolder{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"folder_name": folderName,
			"full_path":   fullPath,
		}).Error
}

// UpdateFullPath sets only the full_path for a folder.
func (r *FolderRepo) UpdateFullPath(id uint64, fullPath string) error {
	return r.db.Model(&model.DiskFolder{}).
		Where("id = ?", id).
		Update("full_path", fullPath).Error
}

// SoftDelete marks a folder as deleted by setting is_deleted = true.
func (r *FolderRepo) SoftDelete(id uint64) error {
	return r.db.Model(&model.DiskFolder{}).
		Where("id = ?", id).
		Update("is_deleted", true).Error
}

// UnDelete marks a folder as not deleted by setting is_deleted = false.
func (r *FolderRepo) UnDelete(id uint64) error {
	return r.db.Model(&model.DiskFolder{}).
		Where("id = ?", id).
		Update("is_deleted", false).Error
}

// ExistsByName checks if a non-deleted folder with the same name exists under the same parent.
func (r *FolderRepo) ExistsByName(userID string, parentID uint64, name string) (bool, error) {
	var count int64
	err := r.db.Model(&model.DiskFolder{}).
		Where("user_id = ? AND parent_id = ? AND folder_name = ? AND is_deleted = ?", userID, parentID, name, false).
		Count(&count).Error
	return count > 0, err
}

// ListSubFolders returns all direct sub-folders of the given parent (including deleted).
func (r *FolderRepo) ListSubFolders(parentID uint64) ([]model.DiskFolder, error) {
	var folders []model.DiskFolder
	err := r.db.Where("parent_id = ?", parentID).
		Order("sort_order ASC").
		Find(&folders).Error
	return folders, err
}
