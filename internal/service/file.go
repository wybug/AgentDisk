package service

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
	"github.com/agentdisk/agent-disk/pkg/oss"
)

// FileService represents a domain type.
type FileService struct {
	fileRepo    *repository.FileRepo
	folderRepo  *repository.FolderRepo
	versionRepo *repository.VersionRepo
	spaceRepo   *repository.SpaceRepo
	ossClient   *oss.Client
}

// NewFileService creates a new FileService.
func NewFileService(
	fileRepo *repository.FileRepo,
	folderRepo *repository.FolderRepo,
	versionRepo *repository.VersionRepo,
	spaceRepo *repository.SpaceRepo,
	ossClient *oss.Client,
) *FileService {
	return &FileService{
		fileRepo:    fileRepo,
		folderRepo:  folderRepo,
		versionRepo: versionRepo,
		spaceRepo:   spaceRepo,
		ossClient:   ossClient,
	}
}

// UploadFile handles the request.
func (s *FileService) UploadFile(ctx context.Context, userID string, folderID uint64, fileName string, reader io.Reader, size int64, contentType, agentID string) (*model.DiskFile, error) {
	var fullPath string
	if folderID > 0 {
		folder, err := s.folderRepo.GetByID(folderID)
		if err != nil {
			return nil, fmt.Errorf("folder not found: %w", err)
		}
		if folder.UserID != userID {
			return nil, fmt.Errorf("permission denied")
		}
		fullPath = folder.FullPath
	}

	file := &model.DiskFile{
		UserID:      userID,
		FolderID:    folderID,
		FileName:    fileName,
		FileSize:    size,
		FileType:    ext(fileName),
		OSSKey:      "", // set after create to get ID
		Version:     1,
		SourceAgent: agentID,
		IsArtifact:  agentID != "",
	}
	if err := s.fileRepo.Create(file); err != nil {
		return nil, fmt.Errorf("create file record: %w", err)
	}

	ossKey := oss.BuildKey(userID, fullPath, file.ID, fileName)
	if err := s.ossClient.Upload(ctx, ossKey, reader, size, contentType); err != nil {
		return nil, fmt.Errorf("upload to oss: %w", err)
	}
	file.OSSKey = ossKey
	if err := s.fileRepo.Update(file); err != nil {
		return nil, fmt.Errorf("update file oss key: %w", err)
	}

	if err := s.ensureQuota(userID); err != nil {
		return nil, fmt.Errorf("init quota: %w", err)
	}
	if err := s.spaceRepo.UpdateUsedQuota(userID, size); err != nil {
		return nil, fmt.Errorf("update used quota: %w", err)
	}
	return file, nil
}

func (s *FileService) ensureQuota(userID string) error {
	_, err := s.spaceRepo.GetByUserID(userID)
	if err != nil {
		defaultQuota := int64(10 * 1024 * 1024 * 1024) // 10GB
		return s.spaceRepo.CreateQuota(userID, defaultQuota)
	}
	return nil
}

// GetFile handles the request.
func (s *FileService) GetFile(ctx context.Context, userID string, fileID uint64) (*model.DiskFile, string, error) {
	file, err := s.fileRepo.GetByID(fileID)
	if err != nil {
		return nil, "", fmt.Errorf("file not found: %w", err)
	}
	if file.UserID != userID {
		return nil, "", fmt.Errorf("permission denied")
	}
	url, err := s.ossClient.PresignedGetURL(ctx, file.OSSKey, time.Hour)
	if err != nil {
		return nil, "", fmt.Errorf("generate presigned url: %w", err)
	}
	return file, url, nil
}

// UpdateFile handles the request.
func (s *FileService) UpdateFile(ctx context.Context, userID string, fileID uint64, reader io.Reader, size int64, contentType string) (*model.DiskFile, error) {
	file, err := s.fileRepo.GetByID(fileID)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}
	if file.UserID != userID {
		return nil, fmt.Errorf("permission denied")
	}

	// Create version snapshot before update
	snapshot := &model.DiskFileVersion{
		FileID:   file.ID,
		UserID:   userID,
		Version:  file.Version,
		OSSKey:   file.OSSKey,
		FileSize: file.FileSize,
		MD5:      file.MD5,
	}
	if err := s.versionRepo.Create(snapshot); err != nil {
		return nil, fmt.Errorf("create version snapshot: %w", err)
	}

	newVersion := file.Version + 1
	ossKey := file.OSSKey
	if err := s.ossClient.Upload(ctx, ossKey, reader, size, contentType); err != nil {
		return nil, fmt.Errorf("upload to oss: %w", err)
	}

	delta := size - file.FileSize
	file.FileSize = size
	file.Version = newVersion
	file.OSSKey = ossKey
	if err := s.fileRepo.Update(file); err != nil {
		return nil, fmt.Errorf("update file: %w", err)
	}
	if err := s.spaceRepo.UpdateUsedQuota(userID, delta); err != nil {
		return nil, fmt.Errorf("update used quota: %w", err)
	}
	return file, nil
}

// DeleteFile handles the request.
func (s *FileService) DeleteFile(userID string, fileID uint64) error {
	file, err := s.fileRepo.GetByID(fileID)
	if err != nil {
		return fmt.Errorf("file not found: %w", err)
	}
	if file.UserID != userID {
		return fmt.Errorf("permission denied")
	}
	if err := s.spaceRepo.UpdateUsedQuota(userID, -file.FileSize); err != nil {
		return fmt.Errorf("update used quota: %w", err)
	}
	return s.fileRepo.SoftDelete(fileID)
}

// ListFiles handles the request.
func (s *FileService) ListFiles(userID string, folderID uint64) ([]model.DiskFile, error) {
	return s.fileRepo.ListByFolder(userID, folderID)
}

func ext(name string) string {
	return strings.TrimPrefix(filepath.Ext(name), ".")
}
