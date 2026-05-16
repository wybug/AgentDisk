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
