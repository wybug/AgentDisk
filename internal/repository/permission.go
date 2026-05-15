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
