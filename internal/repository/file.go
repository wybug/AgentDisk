package repository

import (
	"github.com/agentdisk/agent-disk/internal/model"
	"gorm.io/gorm"
)

// FileRepo provides data access for disk file operations.
type FileRepo struct {
	db *gorm.DB
}

// NewFileRepo creates a new FileRepo.
func NewFileRepo(db *gorm.DB) *FileRepo {
	return &FileRepo{db: db}
}

// Create inserts a new file record.
func (r *FileRepo) Create(file *model.DiskFile) error {
	return r.db.Create(file).Error
}

// GetByID returns a file by its primary key.
func (r *FileRepo) GetByID(id uint64) (*model.DiskFile, error) {
	var file model.DiskFile
	if err := r.db.Where("id = ?", id).First(&file).Error; err != nil {
		return nil, err
	}
	return &file, nil
}

// ListByFolder returns all non-deleted files in a given folder for a specific user.
func (r *FileRepo) ListByFolder(userID string, folderID uint64) ([]model.DiskFile, error) {
	var files []model.DiskFile
	err := r.db.Where("user_id = ? AND folder_id = ? AND is_deleted = ?", userID, folderID, false).
		Order("created_at DESC").
		Find(&files).Error
	return files, err
}

// Update saves changes to an existing file record.
func (r *FileRepo) Update(file *model.DiskFile) error {
	return r.db.Save(file).Error
}

// SoftDelete marks a file as deleted by setting is_deleted = true.
func (r *FileRepo) SoftDelete(id uint64) error {
	return r.db.Model(&model.DiskFile{}).
		Where("id = ?", id).
		Update("is_deleted", true).Error
}

// UnDelete marks a file as not deleted by setting is_deleted = false.
func (r *FileRepo) UnDelete(id uint64) error {
	return r.db.Model(&model.DiskFile{}).
		Where("id = ?", id).
		Update("is_deleted", false).Error
}

// GetByMD5 returns all non-deleted files matching the given MD5 hash for a specific user.
func (r *FileRepo) GetByMD5(userID, md5 string) ([]model.DiskFile, error) {
	var files []model.DiskFile
	err := r.db.Where("user_id = ? AND md5 = ? AND is_deleted = ?", userID, md5, false).
		Find(&files).Error
	return files, err
}
