package service

import (
	"context"
	"fmt"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
	"github.com/agentdisk/agent-disk/pkg/oss"
)

// VersionService represents a domain type.
type VersionService struct {
	versionRepo *repository.VersionRepo
	fileRepo    *repository.FileRepo
	ossClient   *oss.Client
}

// NewVersionService creates a new VersionService.
func NewVersionService(versionRepo *repository.VersionRepo, fileRepo *repository.FileRepo, ossClient *oss.Client) *VersionService {
	return &VersionService{versionRepo: versionRepo, fileRepo: fileRepo, ossClient: ossClient}
}

// ListVersions handles the request.
func (s *VersionService) ListVersions(userID string, fileID uint64) ([]model.DiskFileVersion, error) {
	file, err := s.fileRepo.GetByID(fileID)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}
	if file.UserID != userID {
		return nil, fmt.Errorf("permission denied")
	}
	return s.versionRepo.ListByFile(fileID)
}

// Rollback handles the request.
func (s *VersionService) Rollback(ctx context.Context, userID string, fileID uint64, targetVersion int) error {
	file, err := s.fileRepo.GetByID(fileID)
	if err != nil {
		return fmt.Errorf("file not found: %w", err)
	}
	if file.UserID != userID {
		return fmt.Errorf("permission denied")
	}

	snapshot, err := s.versionRepo.GetByVersion(fileID, targetVersion)
	if err != nil {
		return fmt.Errorf("version not found: %w", err)
	}

	// Copy old version to current key
	if err := s.ossClient.Copy(ctx, snapshot.OSSKey, file.OSSKey); err != nil {
		return fmt.Errorf("rollback oss: %w", err)
	}

	file.Version = targetVersion
	file.FileSize = snapshot.FileSize
	file.MD5 = snapshot.MD5
	return s.fileRepo.Update(file)
}
