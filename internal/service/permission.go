package service

import (
	"fmt"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
)

// permissionRepo defines the interface for permission data access.
type permissionRepo interface {
	Create(p *model.DiskPermission) error
	GetByAgentAndResource(agentID string, resourceID uint64, resType string) (*model.DiskPermission, error)
	GetResourceDetail(resourceID uint64, resType string) (*repository.ResourceOwner, error)
	ListByUser(userID string) ([]model.DiskPermission, error)
	Delete(id uint64) error
}

// PermissionService represents a domain type.
type PermissionService struct {
	repo permissionRepo
}

// NewPermissionService creates a new PermissionService.
func NewPermissionService(repo *repository.PermissionRepo) *PermissionService {
	return &PermissionService{repo: repo}
}

// GrantPermission handles the request.
func (s *PermissionService) GrantPermission(userID, agentID string, resourceID uint64, resType, perm string) error {
	p := &model.DiskPermission{
		UserID:     userID,
		AgentID:    agentID,
		ResourceID: resourceID,
		ResType:    resType,
		Permission: perm,
	}
	return s.repo.Create(p)
}

// CheckPermission handles the request.
func (s *PermissionService) CheckPermission(agentID string, resourceID uint64, resType, requiredPerm string) (bool, error) {
	p, err := s.repo.GetByAgentAndResource(agentID, resourceID, resType)
	if err != nil {
		return false, fmt.Errorf("permission not found: %w", err)
	}
	return hasPermission(p.Permission, requiredPerm), nil
}

// RevokePermission handles the request.
func (s *PermissionService) RevokePermission(userID, agentID string, resourceID uint64, resType string) error {
	p, err := s.repo.GetByAgentAndResource(agentID, resourceID, resType)
	if err != nil {
		return fmt.Errorf("permission not found: %w", err)
	}
	if p.UserID != userID {
		return fmt.Errorf("permission denied")
	}
	return s.repo.Delete(p.ID)
}

// ListPermissions handles the request.
func (s *PermissionService) ListPermissions(userID string) ([]model.DiskPermission, error) {
	return s.repo.ListByUser(userID)
}

// CheckOrAutoGrant checks permission with auto-grant rules for agents.
// User requests (agentID == "") always pass.
// Agent requests go through: own-file auto, same-group auto, then explicit table.
func (s *PermissionService) CheckOrAutoGrant(userID, agentID, agentGroupID string, resourceID uint64, resType, required string) (bool, error) {
	if agentID == "" {
		return true, nil
	}

	owner, err := s.repo.GetResourceDetail(resourceID, resType)
	if err != nil {
		return false, err
	}
	if owner.OwnerID != userID {
		return false, nil
	}

	if !owner.IsArtifact {
		return s.CheckPermission(agentID, resourceID, resType, required)
	}

	if required == "read" || required == "write" {
		if owner.SourceAgent == agentID {
			return true, nil
		}
		if agentGroupID != "" && owner.SourceAgentGroup == agentGroupID {
			return true, nil
		}
	}

	return s.CheckPermission(agentID, resourceID, resType, required)
}

func hasPermission(actual, required string) bool {
	levels := map[string]int{"owner": 4, "delete": 3, "write": 2, "read": 1}
	return levels[actual] >= levels[required]
}
