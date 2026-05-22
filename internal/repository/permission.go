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

// ListPathPermissionsByAgent returns all path-based permissions for an agent.
func (r *PermissionRepo) ListPathPermissionsByAgent(agentID string) ([]model.DiskPermission, error) {
	var perms []model.DiskPermission
	err := r.db.Where("agent_id = ? AND resource_path != ''", agentID).Find(&perms).Error
	return perms, err
}

// ListGroupPermissions returns all permissions (including path-based) for an agent group.
func (r *PermissionRepo) ListGroupPermissions(agentGroupID string) ([]model.DiskPermission, error) {
	var perms []model.DiskPermission
	err := r.db.Where("agent_group_id = ?", agentGroupID).Find(&perms).Error
	return perms, err
}

// GetByAgentAndResourcePath returns the permission for an agent on a specific path.
func (r *PermissionRepo) GetByAgentAndResourcePath(agentID, resourcePath string) (*model.DiskPermission, error) {
	var p model.DiskPermission
	if err := r.db.Where("agent_id = ? AND resource_path = ?", agentID, resourcePath).
		First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// GetByGroupAndResourcePath returns the permission for a group on a specific path.
func (r *PermissionRepo) GetByGroupAndResourcePath(agentGroupID, resourcePath string) (*model.DiskPermission, error) {
	var p model.DiskPermission
	if err := r.db.Where("agent_group_id = ? AND resource_path = ?", agentGroupID, resourcePath).
		First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// GetByGroupAndResource returns the permission for a group on a specific resource.
func (r *PermissionRepo) GetByGroupAndResource(agentGroupID string, resourceID uint64, resType string) (*model.DiskPermission, error) {
	var p model.DiskPermission
	if err := r.db.Where("agent_group_id = ? AND resource_id = ? AND res_type = ?", agentGroupID, resourceID, resType).
		First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

// GetResourcePath returns the full path for a resource (file or folder).
func (r *PermissionRepo) GetResourcePath(resourceID uint64, resType string) (string, error) {
	if resType == "file" {
		var f model.DiskFile
		if err := r.db.Select("file_name, folder_id").Where("id = ?", resourceID).First(&f).Error; err != nil {
			return "", err
		}
		if f.FolderID == 0 {
			return "/" + f.FileName, nil
		}
		var fo model.DiskFolder
		if err := r.db.Select("full_path").Where("id = ?", f.FolderID).First(&fo).Error; err != nil {
			return "", err
		}
		if fo.FullPath == "" {
			return "/" + f.FileName, nil
		}
		return "/" + fo.FullPath + "/" + f.FileName, nil
	}
	var fo model.DiskFolder
	if err := r.db.Select("full_path").Where("id = ?", resourceID).First(&fo).Error; err != nil {
		return "", err
	}
	return "/" + fo.FullPath, nil
}
