package repository

import (
	"errors"

	"github.com/agentdisk/agent-disk/internal/model"
	"gorm.io/gorm"
)

// TagRepo provides data access for tag and tag-relation operations.
type TagRepo struct {
	db *gorm.DB
}

// NewTagRepo creates a new TagRepo.
func NewTagRepo(db *gorm.DB) *TagRepo {
	return &TagRepo{db: db}
}

// CreateTag inserts a new tag record.
func (r *TagRepo) CreateTag(tag *model.DiskTag) error {
	return r.db.Create(tag).Error
}

// FindOrCreate returns an existing tag for the user with the given name, or creates one.
func (r *TagRepo) FindOrCreate(userID, name string) (*model.DiskTag, error) {
	var tag model.DiskTag
	err := r.db.Where("user_id = ? AND tag_name = ?", userID, name).First(&tag).Error
	if err == nil {
		return &tag, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	tag = model.DiskTag{
		UserID:  userID,
		TagName: name,
	}
	if err := r.db.Create(&tag).Error; err != nil {
		return nil, err
	}
	return &tag, nil
}

// BindFile creates a tag-file association record (idempotent).
func (r *TagRepo) BindFile(tagID, fileID uint64) error {
	relation := model.DiskTagRelation{
		TagID:  tagID,
		FileID: fileID,
	}
	result := r.db.Where("tag_id = ? AND file_id = ?", tagID, fileID).FirstOrCreate(&relation)
	return result.Error
}

// UnbindFile removes the tag-file association.
func (r *TagRepo) UnbindFile(tagID, fileID uint64) error {
	return r.db.Where("tag_id = ? AND file_id = ?", tagID, fileID).
		Delete(&model.DiskTagRelation{}).Error
}

// ListByFile returns all tags associated with a given file.
func (r *TagRepo) ListByFile(fileID uint64) ([]model.DiskTag, error) {
	var tags []model.DiskTag
	err := r.db.Joins("JOIN disk_tag_relation ON disk_tag_relation.tag_id = disk_tag.id").
		Where("disk_tag_relation.file_id = ?", fileID).
		Find(&tags).Error
	return tags, err
}

// ListByUser returns all tags belonging to a specific user.
func (r *TagRepo) ListByUser(userID string) ([]model.DiskTag, error) {
	var tags []model.DiskTag
	err := r.db.Where("user_id = ?", userID).
		Order("created_at ASC").
		Find(&tags).Error
	return tags, err
}

// SearchFiles returns all non-deleted files that are associated with all of the given tag IDs
// and belong to the specified user.
func (r *TagRepo) SearchFiles(userID string, tagIDs []uint64) ([]model.DiskFile, error) {
	var files []model.DiskFile
	err := r.db.Joins("JOIN disk_tag_relation ON disk_tag_relation.file_id = disk_file.id").
		Where("disk_file.user_id = ? AND disk_file.is_deleted = ? AND disk_tag_relation.tag_id IN ?", userID, false, tagIDs).
		Group("disk_file.id").
		Having("COUNT(DISTINCT disk_tag_relation.tag_id) = ?", len(tagIDs)).
		Find(&files).Error
	return files, err
}
