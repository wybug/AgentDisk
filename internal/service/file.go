package service

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
	"github.com/agentdisk/agent-disk/pkg/oss"
)

type FileService struct {
	fileRepo    *repository.FileRepo
	folderRepo  *repository.FolderRepo
	versionRepo *repository.VersionRepo
	spaceRepo   *repository.SpaceRepo
	ossClient   *oss.Client
}

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

func (s *FileService) UploadFile(ctx context.Context, userID string, folderID uint64, fileName string, reader io.Reader, size int64, contentType string, agentID string) (*model.DiskFile, error) {
	folder, err := s.folderRepo.GetByID(folderID)
	if err != nil {
		return nil, fmt.Errorf("folder not found: %w", err)
	}
	if folder.UserID != userID {
		return nil, fmt.Errorf("permission denied")
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

	ossKey := oss.BuildKey(userID, folder.FullPath, file.ID, fileName)
	if err := s.ossClient.Upload(ctx, ossKey, reader, size, contentType); err != nil {
		return nil, fmt.Errorf("upload to oss: %w", err)
	}
	file.OSSKey = ossKey
	_ = s.fileRepo.Update(file)

	_ = s.spaceRepo.UpdateUsedQuota(userID, size)
	return file, nil
}

func (s *FileService) GetFile(ctx context.Context, userID string, fileID uint64) (*model.DiskFile, string, error) {
	file, err := s.fileRepo.GetByID(fileID)
	if err != nil {
		return nil, "", fmt.Errorf("file not found: %w", err)
	}
	if file.UserID != userID {
		return nil, "", fmt.Errorf("permission denied")
	}
	url, err := s.ossClient.PresignedGetURL(ctx, file.OSSKey, 60*60) // 1 hour
	if err != nil {
		return nil, "", fmt.Errorf("generate presigned url: %w", err)
	}
	return file, url, nil
}

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
	_ = s.versionRepo.Create(snapshot)

	newVersion := file.Version + 1
	ossKey := strings.Replace(file.OSSKey, fmt.Sprintf("/%d_%s", file.ID, file.FileName), fmt.Sprintf("/%d_%s", file.ID, file.FileName), 1)
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
	_ = s.spaceRepo.UpdateUsedQuota(userID, delta)
	return file, nil
}

func (s *FileService) DeleteFile(userID string, fileID uint64) error {
	file, err := s.fileRepo.GetByID(fileID)
	if err != nil {
		return fmt.Errorf("file not found: %w", err)
	}
	if file.UserID != userID {
		return fmt.Errorf("permission denied")
	}
	_ = s.spaceRepo.UpdateUsedQuota(userID, -file.FileSize)
	return s.fileRepo.SoftDelete(fileID)
}

func (s *FileService) ListFiles(userID string, folderID uint64) ([]model.DiskFile, error) {
	return s.fileRepo.ListByFolder(userID, folderID)
}

func ext(name string) string {
	return strings.TrimPrefix(filepath.Ext(name), ".")
}
