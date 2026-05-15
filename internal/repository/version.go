package repository

import (
	"github.com/agentdisk/agent-disk/internal/model"
	"gorm.io/gorm"
)

// VersionRepo provides data access for file version operations.
type VersionRepo struct {
	db *gorm.DB
}

// NewVersionRepo creates a new VersionRepo.
func NewVersionRepo(db *gorm.DB) *VersionRepo {
	return &VersionRepo{db: db}
}

// Create inserts a new file version record.
func (r *VersionRepo) Create(v *model.DiskFileVersion) error {
	return r.db.Create(v).Error
}

// ListByFile returns all version records for a given file, ordered by version number.
func (r *VersionRepo) ListByFile(fileID uint64) ([]model.DiskFileVersion, error) {
	var versions []model.DiskFileVersion
	err := r.db.Where("file_id = ?", fileID).
		Order("version DESC").
		Find(&versions).Error
	return versions, err
}

// GetByVersion returns a specific version record for a file.
func (r *VersionRepo) GetByVersion(fileID uint64, version int) (*model.DiskFileVersion, error) {
	var v model.DiskFileVersion
	if err := r.db.Where("file_id = ? AND version = ?", fileID, version).
		First(&v).Error; err != nil {
		return nil, err
	}
	return &v, nil
}
