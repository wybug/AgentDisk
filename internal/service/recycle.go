package service

import (
	"context"
	"fmt"
	"time"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
	"github.com/agentdisk/agent-disk/pkg/oss"
)

// RecycleService represents a domain type.
type RecycleService struct {
	recycleRepo *repository.RecycleRepo
	fileRepo    *repository.FileRepo
	folderRepo  *repository.FolderRepo
	ossClient   *oss.Client
}

// NewRecycleService creates a new RecycleService.
func NewRecycleService(recycleRepo *repository.RecycleRepo, fileRepo *repository.FileRepo, folderRepo *repository.FolderRepo, ossClient *oss.Client) *RecycleService {
	return &RecycleService{recycleRepo: recycleRepo, fileRepo: fileRepo, folderRepo: folderRepo, ossClient: ossClient}
}

// MoveToRecycle handles the request.
func (s *RecycleService) MoveToRecycle(userID string, resourceID uint64, resType, operator string) error {
	var name, origPath string
	switch resType {
	case "file":
		f, err := s.fileRepo.GetByID(resourceID)
		if err != nil {
			return fmt.Errorf("file not found: %w", err)
		}
		name = f.FileName
	case "folder":
		f, err := s.folderRepo.GetByID(resourceID)
		if err != nil {
			return fmt.Errorf("folder not found: %w", err)
		}
		name = f.FolderName
		origPath = f.FullPath
	}

	record := &model.DiskRecycleBin{
		UserID:       userID,
		ResourceID:   resourceID,
		ResType:      resType,
		ResName:      name,
		OriginalPath: origPath,
		DeletedBy:    operator,
		ExpireAt:     time.Now().Add(30 * 24 * time.Hour), // 30 days
	}
	return s.recycleRepo.Create(record)
}

// ListRecycle handles the request.
func (s *RecycleService) ListRecycle(userID string) ([]model.DiskRecycleBin, error) {
	return s.recycleRepo.ListByUser(userID)
}

// Restore handles the request.
func (s *RecycleService) Restore(userID string, recycleID uint64) error {
	item, err := s.recycleRepo.GetByID(recycleID)
	if err != nil {
		return fmt.Errorf("recycle item not found: %w", err)
	}
	if item.UserID != userID {
		return fmt.Errorf("permission denied")
	}
	switch item.ResType {
	case "file":
		_ = s.fileRepo.UnDelete(item.ResourceID)
	case "folder":
		_ = s.folderRepo.UnDelete(item.ResourceID)
	}
	return s.recycleRepo.Delete(recycleID)
}

// PermanentlyDelete handles the request.
func (s *RecycleService) PermanentlyDelete(ctx context.Context, userID string, recycleID uint64) error {
	item, err := s.recycleRepo.GetByID(recycleID)
	if err != nil {
		return fmt.Errorf("recycle item not found: %w", err)
	}
	if item.UserID != userID {
		return fmt.Errorf("permission denied")
	}
	// Physical delete from OSS
	if item.ResType == "file" {
		f, err := s.fileRepo.GetByID(item.ResourceID)
		if err == nil {
			_ = s.ossClient.Delete(ctx, f.OSSKey) // best-effort physical delete
		}
	}
	return s.recycleRepo.Delete(recycleID)
}
