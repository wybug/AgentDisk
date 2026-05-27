package service

import (
	"fmt"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
)

const (
	// SystemUserID is the owner of all public directory folders.
	SystemUserID = "__system_public__"

	// ScopeGlobal represents a globally visible public directory.
	ScopeGlobal = "global"

	// ScopeDepartment represents a department-scoped public directory.
	ScopeDepartment = "department"
)

// publicDirRepo defines the interface for public directory data access.
type publicDirRepo interface {
	Create(pd *model.DiskPublicDirectory) error
	GetByID(id uint64) (*model.DiskPublicDirectory, error)
	GetByFolderID(folderID uint64) (*model.DiskPublicDirectory, error)
	ListActive() ([]model.DiskPublicDirectory, error)
	ListByScope(scope string) ([]model.DiskPublicDirectory, error)
	Update(pd *model.DiskPublicDirectory) error
	Delete(id uint64) error
}

// folderCreator defines the interface needed from the folder repo for creating system folders.
type folderCreator interface {
	Create(folder *model.DiskFolder) error
	GetByID(id uint64) (*model.DiskFolder, error)
	ListByParent(userID string, parentID uint64) ([]model.DiskFolder, error)
	SoftDelete(id uint64) error
}

// PublicDirectoryService handles public directory operations.
type PublicDirectoryService struct {
	pdRepo     publicDirRepo
	folderRepo folderCreator
}

// NewPublicDirectoryService creates a new PublicDirectoryService.
func NewPublicDirectoryService(pdRepo *repository.PublicDirectoryRepo, folderRepo *repository.FolderRepo) *PublicDirectoryService {
	return &PublicDirectoryService{pdRepo: pdRepo, folderRepo: folderRepo}
}

// NewPublicDirectoryServiceFromRepo creates a PublicDirectoryService from interfaces (for testing).
func NewPublicDirectoryServiceFromRepo(pdRepo publicDirRepo, folderRepo folderCreator) *PublicDirectoryService {
	return &PublicDirectoryService{pdRepo: pdRepo, folderRepo: folderRepo}
}

// CreatePublicDirectory creates a new public directory with a system folder.
func (s *PublicDirectoryService) CreatePublicDirectory(displayName, scope, department, createdBy string) (*model.DiskPublicDirectory, error) {
	if scope != ScopeGlobal && scope != ScopeDepartment {
		return nil, fmt.Errorf("invalid scope: %s", scope)
	}
	if scope == ScopeDepartment && department == "" {
		return nil, fmt.Errorf("department is required for department scope")
	}

	var fixedPath string
	if scope == ScopeGlobal {
		fixedPath = "/public/" + displayName
	} else {
		fixedPath = "/department/" + department + "/" + displayName
	}

	folder := &model.DiskFolder{
		UserID:     SystemUserID,
		ParentID:   0,
		FolderName: displayName,
		FullPath:   fixedPath,
	}
	if err := s.folderRepo.Create(folder); err != nil {
		return nil, fmt.Errorf("create system folder: %w", err)
	}

	pd := &model.DiskPublicDirectory{
		FolderID:    folder.ID,
		Scope:       scope,
		Department:  department,
		DisplayName: displayName,
		FixedPath:   fixedPath,
		CreatedBy:   createdBy,
		IsActive:    true,
	}
	if err := s.pdRepo.Create(pd); err != nil {
		return nil, fmt.Errorf("create public directory: %w", err)
	}

	return pd, nil
}

// ListVisible returns public directories visible to the given department.
// Global directories are always visible. Department directories match by department.
func (s *PublicDirectoryService) ListVisible(department string) ([]model.DiskPublicDirectory, error) {
	all, err := s.pdRepo.ListActive()
	if err != nil {
		return nil, err
	}
	var visible []model.DiskPublicDirectory
	for _, d := range all {
		if d.Scope == ScopeGlobal {
			visible = append(visible, d)
		} else if d.Scope == ScopeDepartment && (d.Department == department || department == "") {
			visible = append(visible, d)
		}
	}
	return visible, nil
}

// GetPublicDirectory returns a public directory by ID.
func (s *PublicDirectoryService) GetPublicDirectory(id uint64) (*model.DiskPublicDirectory, error) {
	return s.pdRepo.GetByID(id)
}

// UpdatePublicDirectory updates a public directory's display name and active status.
func (s *PublicDirectoryService) UpdatePublicDirectory(id uint64, displayName string, isActive bool) (*model.DiskPublicDirectory, error) {
	pd, err := s.pdRepo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("public directory not found: %w", err)
	}
	if displayName != "" {
		pd.DisplayName = displayName
	}
	pd.IsActive = isActive
	if err := s.pdRepo.Update(pd); err != nil {
		return nil, err
	}
	return pd, nil
}

// DeletePublicDirectory removes a public directory and its system folder.
func (s *PublicDirectoryService) DeletePublicDirectory(id uint64) error {
	pd, err := s.pdRepo.GetByID(id)
	if err != nil {
		return fmt.Errorf("public directory not found: %w", err)
	}
	if err := s.pdRepo.Delete(id); err != nil {
		return err
	}
	return s.folderRepo.SoftDelete(pd.FolderID)
}

// ListSubFolders returns sub-folders of a public directory's system folder.
func (s *PublicDirectoryService) ListSubFolders(publicDirID uint64) ([]model.DiskFolder, error) {
	pd, err := s.pdRepo.GetByID(publicDirID)
	if err != nil {
		return nil, fmt.Errorf("public directory not found: %w", err)
	}
	return s.folderRepo.ListByParent(SystemUserID, pd.FolderID)
}

// ListAdminAll returns all public directories (active and inactive) for admin management.
func (s *PublicDirectoryService) ListAdminAll() ([]model.DiskPublicDirectory, error) {
	return s.pdRepo.ListActive()
}

// IsReservedPath checks if a folder name conflicts with reserved prefixes.
func IsReservedPath(name string) bool {
	return name == "public" || name == "department"
}
