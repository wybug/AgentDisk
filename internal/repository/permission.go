package repository

import (
	"github.com/agentdisk/agent-disk/internal/model"
	"gorm.io/gorm"
)

// PermissionRepo provides data access for disk permission operations.
type PermissionRepo struct {
	db *gorm.DB
}

// NewPermissionRepo creates a new PermissionRepo.
func NewPermissionRepo(db *gorm.DB) *PermissionRepo {
	return &PermissionRepo{db: db}
}

// Create inserts a new permission record.
func (r *PermissionRepo) Create(p *model.DiskPermission) error {
	return r.db.Create(p).Error
}

// GetByAgentAndResource returns the permission for an agent on a specific resource.
func (r *PermissionRepo) GetByAgentAndResource(agentID string, resourceID uint64, resType string) (*model.DiskPermission, error) {
	var p model.DiskPermission
	if err := r.db.Where("agent_id = ? AND resource_id = ? AND res_type = ?", agentID, resourceID, resType).
		First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// ListByUser returns all permissions belonging to a specific user.
func (r *PermissionRepo) ListByUser(userID string) ([]model.DiskPermission, error) {
	var perms []model.DiskPermission
	err := r.db.Where("user_id = ?", userID).Find(&perms).Error
	return perms, err
}

// Delete removes a permission record by its primary key.
func (r *PermissionRepo) Delete(id uint64) error {
	return r.db.Where("id = ?", id).Delete(&model.DiskPermission{}).Error
}

// ResourceOwner holds resource ownership detail.
type ResourceOwner struct {
	OwnerID          string
	SourceAgent      string
	SourceAgentGroup string
	IsArtifact       bool
}

// GetResourceDetail returns the owner, source agent and artifact flag for a resource.
func (r *PermissionRepo) GetResourceDetail(resourceID uint64, resType string) (*ResourceOwner, error) {
	if resType == "file" {
		var f model.DiskFile
		if err := r.db.Select("user_id, source_agent, source_agent_group, is_artifact").Where("id = ? AND is_deleted = ?", resourceID, false).First(&f).Error; err != nil {
			return nil, err
		}
		return &ResourceOwner{OwnerID: f.UserID, SourceAgent: f.SourceAgent, SourceAgentGroup: f.SourceAgentGroup, IsArtifact: f.IsArtifact}, nil
	}
	var fo model.DiskFolder
	if err := r.db.Select("user_id").Where("id = ? AND is_deleted = ?", resourceID, false).First(&fo).Error; err != nil {
		return nil, err
	}
	return &ResourceOwner{OwnerID: fo.UserID, SourceAgent: "", SourceAgentGroup: "", IsArtifact: false}, nil
}
