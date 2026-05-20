package service

import (
	"fmt"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
	"github.com/agentdisk/agent-disk/pkg/oss"
)

// FolderService represents a domain type.
type FolderService struct {
	folderRepo *repository.FolderRepo
	ossClient  *oss.Client
}

// NewFolderService creates a new FolderService.
func NewFolderService(folderRepo *repository.FolderRepo, ossClient *oss.Client) *FolderService {
	return &FolderService{folderRepo: folderRepo, ossClient: ossClient}
}

// CreateFolder handles the request.
func (s *FolderService) CreateFolder(userID string, parentID uint64, name string) (*model.DiskFolder, error) {
	exists, err := s.folderRepo.ExistsByName(userID, parentID, name)
	if err != nil {
		return nil, fmt.Errorf("check duplicate: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("同名文件夹已存在")
	}

	var fullPath string
	if parentID == 0 {
		fullPath = name
	} else {
		parent, err := s.folderRepo.GetByID(parentID)
		if err != nil {
			return nil, fmt.Errorf("parent folder not found: %w", err)
		}
		fullPath = parent.FullPath + "/" + name
	}

	folder := &model.DiskFolder{
		UserID:     userID,
		ParentID:   parentID,
		FolderName: name,
		FullPath:   fullPath,
	}
	if err := s.folderRepo.Create(folder); err != nil {
		return nil, fmt.Errorf("create folder: %w", err)
	}
	return folder, nil
}

// AncestorItem represents a breadcrumb item.
type AncestorItem struct {
	ID         uint64 `json:"id"`
	FolderName string `json:"folderName"`
}

// GetFolder returns a single folder by ID.
func (s *FolderService) GetFolder(userID string, folderID uint64) (*model.DiskFolder, error) {
	folder, err := s.folderRepo.GetByID(folderID)
	if err != nil {
		return nil, fmt.Errorf("folder not found: %w", err)
	}
	if folder.UserID != userID {
		return nil, fmt.Errorf("permission denied")
	}
	return folder, nil
}

// GetAncestors returns the path from root to the given folder.
func (s *FolderService) GetAncestors(userID string, folderID uint64) ([]AncestorItem, error) {
	var chain []AncestorItem
	currentID := folderID
	for currentID != 0 {
		folder, err := s.folderRepo.GetByID(currentID)
		if err != nil {
			return nil, fmt.Errorf("folder not found: %w", err)
		}
		if folder.UserID != userID {
			return nil, fmt.Errorf("permission denied")
		}
		chain = append([]AncestorItem{{ID: folder.ID, FolderName: folder.FolderName}}, chain...)
		currentID = folder.ParentID
	}
	return chain, nil
}

// RenameFolder renames a folder and updates its fullPath.
func (s *FolderService) RenameFolder(userID string, folderID uint64, newName string) (*model.DiskFolder, error) {
	folder, err := s.folderRepo.GetByID(folderID)
	if err != nil {
		return nil, fmt.Errorf("folder not found: %w", err)
	}
	if folder.UserID != userID {
		return nil, fmt.Errorf("permission denied")
	}
	exists, err := s.folderRepo.ExistsByName(userID, folder.ParentID, newName)
	if err != nil {
		return nil, fmt.Errorf("check duplicate: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("同名文件夹已存在")
	}

	oldPath := folder.FullPath
	var newPath string
	if folder.ParentID == 0 {
		newPath = newName
	} else {
		parent, err := s.folderRepo.GetByID(folder.ParentID)
		if err != nil {
			return nil, fmt.Errorf("parent folder not found: %w", err)
		}
		newPath = parent.FullPath + "/" + newName
	}
	folder.FolderName = newName
	folder.FullPath = newPath
	if err := s.folderRepo.UpdateNameAndPath(folder.ID, newName, newPath); err != nil {
		return nil, fmt.Errorf("update folder: %w", err)
	}
	// Update child paths recursively
	if err := s.updateChildPaths(folder.ID, newPath); err != nil {
		return nil, fmt.Errorf("update child paths: %w", err)
	}
	// Also rename in OSS if needed
	_ = oldPath
	return folder, nil
}

func (s *FolderService) updateChildPaths(parentID uint64, parentPath string) error {
	subs, err := s.folderRepo.ListSubFolders(parentID)
	if err != nil {
		return err
	}
	for _, sub := range subs {
		if sub.IsDeleted {
			continue
		}
		newPath := parentPath + "/" + sub.FolderName
		if err := s.folderRepo.UpdateFullPath(sub.ID, newPath); err != nil {
			return err
		}
		if err := s.updateChildPaths(sub.ID, newPath); err != nil {
			return err
		}
	}
	return nil
}

// ListFolders handles the request.
func (s *FolderService) ListFolders(userID string, parentID uint64) ([]model.DiskFolder, error) {
	return s.folderRepo.ListByParent(userID, parentID)
}

// DeleteFolder handles the request.
func (s *FolderService) DeleteFolder(userID string, folderID uint64) error {
	folder, err := s.folderRepo.GetByID(folderID)
	if err != nil {
		return fmt.Errorf("folder not found: %w", err)
	}
	if folder.UserID != userID {
		return fmt.Errorf("permission denied")
	}
	if err := s.deleteRecursive(folderID); err != nil {
		return fmt.Errorf("delete recursive: %w", err)
	}
	return s.folderRepo.SoftDelete(folderID)
}

func (s *FolderService) deleteRecursive(parentID uint64) error {
	subs, err := s.folderRepo.ListSubFolders(parentID)
	if err != nil {
		return err
	}
	for _, sub := range subs {
		if err := s.deleteRecursive(sub.ID); err != nil {
			return err
		}
		if err := s.folderRepo.SoftDelete(sub.ID); err != nil {
			return err
		}
	}
	return nil
}
