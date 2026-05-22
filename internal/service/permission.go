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
	GetByAgentAndResourcePath(agentID, resourcePath string) (*model.DiskPermission, error)
	GetByGroupAndResource(agentGroupID string, resourceID uint64, resType string) (*model.DiskPermission, error)
	GetByGroupAndResourcePath(agentGroupID, resourcePath string) (*model.DiskPermission, error)
	GetResourceDetail(resourceID uint64, resType string) (*repository.ResourceOwner, error)
	GetResourcePath(resourceID uint64, resType string) (string, error)
	ListByUser(userID string) ([]model.DiskPermission, error)
	ListPathPermissionsByAgent(agentID string) ([]model.DiskPermission, error)
	ListGroupPermissions(agentGroupID string) ([]model.DiskPermission, error)
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

// NewPermissionServiceWithRepo creates a new PermissionService with a custom repo implementation.
func NewPermissionServiceWithRepo(repo permissionRepo) *PermissionService {
	return &PermissionService{repo: repo}
}

// GrantPermission handles the request.
func (s *PermissionService) GrantPermission(userID, agentID, agentGroupID string, resourceID uint64, resType, resourcePath, perm string) error {
	p := &model.DiskPermission{
		UserID:       userID,
		AgentID:      agentID,
		AgentGroupID: agentGroupID,
		ResourceID:   resourceID,
		ResType:      resType,
		ResourcePath: resourcePath,
		Permission:   perm,
	}
	return s.repo.Create(p)
}

// CheckPermission checks agent permission on a resource, including path-based rules.
func (s *PermissionService) CheckPermission(agentID string, resourceID uint64, resType, requiredPerm string) (bool, error) {
	// 1. Exact resource ID match
	p, err := s.repo.GetByAgentAndResource(agentID, resourceID, resType)
	if err == nil && hasPermission(p.Permission, requiredPerm) {
		return true, nil
	}

	// 2. Path-based rules for this agent
	return s.checkPathPermission(agentID, "", resourceID, resType, requiredPerm)
}

// CheckGroupPermission checks group-level permission on a resource.
func (s *PermissionService) CheckGroupPermission(agentGroupID string, resourceID uint64, resType, requiredPerm string) (bool, error) {
	// 1. Exact resource ID match for group
	p, err := s.repo.GetByGroupAndResource(agentGroupID, resourceID, resType)
	if err == nil && hasPermission(p.Permission, requiredPerm) {
		return true, nil
	}

	// 2. Path-based rules for this group
	return s.checkPathPermission("", agentGroupID, resourceID, resType, requiredPerm)
}

func (s *PermissionService) checkPathPermission(agentID, agentGroupID string, resourceID uint64, resType, requiredPerm string) (bool, error) {
	// Get the full path of the resource
	resourcePath, err := s.repo.GetResourcePath(resourceID, resType)
	if err != nil {
		return false, err
	}

	// Collect all path-based permissions to check
	var pathPerms []model.DiskPermission

	if agentID != "" {
		agentPathPerms, _ := s.repo.ListPathPermissionsByAgent(agentID)
		pathPerms = append(pathPerms, agentPathPerms...)
	}
	if agentGroupID != "" {
		groupPerms, _ := s.repo.ListGroupPermissions(agentGroupID)
		pathPerms = append(pathPerms, groupPerms...)
	}

	for _, pp := range pathPerms {
		if pp.ResourcePath != "" && MatchGlob(pp.ResourcePath, resourcePath) && hasPermission(pp.Permission, requiredPerm) {
			return true, nil
		}
	}

	return false, nil
}

// RevokePermission handles the request.
func (s *PermissionService) RevokePermission(userID, agentID, agentGroupID string, resourceID uint64, resType, resourcePath string) error {
	var p *model.DiskPermission
	var err error

	switch {
	case resourcePath != "" && agentID != "":
		p, err = s.repo.GetByAgentAndResourcePath(agentID, resourcePath)
	case resourcePath != "" && agentGroupID != "":
		p, err = s.repo.GetByGroupAndResourcePath(agentGroupID, resourcePath)
	case agentID != "":
		p, err = s.repo.GetByAgentAndResource(agentID, resourceID, resType)
	default:
		p, err = s.repo.GetByGroupAndResource(agentGroupID, resourceID, resType)
	}
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
		return s.CheckFullPermission(agentID, agentGroupID, resourceID, resType, required)
	}

	if required == "read" || required == "write" {
		if owner.SourceAgent == agentID {
			return true, nil
		}
		if agentGroupID != "" && owner.SourceAgentGroup == agentGroupID {
			return true, nil
		}
	}

	return s.CheckFullPermission(agentID, agentGroupID, resourceID, resType, required)
}

// CheckFullPermission checks all permission sources: resource ID + path + group.
func (s *PermissionService) CheckFullPermission(agentID, agentGroupID string, resourceID uint64, resType, required string) (bool, error) {
	// 1. Agent exact resource ID
	if ok, _ := s.CheckPermission(agentID, resourceID, resType, required); ok {
		return true, nil
	}

	// 2. Agent group exact resource ID + path
	if agentGroupID != "" {
		if ok, _ := s.CheckGroupPermission(agentGroupID, resourceID, resType, required); ok {
			return true, nil
		}
	}

	return false, nil
}

func hasPermission(actual, required string) bool {
	levels := map[string]int{"owner": 4, "delete": 3, "write": 2, "read": 1}
	return levels[actual] >= levels[required]
}
