package service

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
	"github.com/agentdisk/agent-disk/pkg/download_token"
	"github.com/agentdisk/agent-disk/pkg/oss"
	"github.com/agentdisk/agent-disk/pkg/storage"
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

// grantRepo defines the interface for public directory user grants.
type grantRepo interface {
	Create(grant *model.DiskPublicDirectoryGrant) error
	Delete(publicDirID uint64, userID string) error
	ListByPublicDirID(publicDirID uint64) ([]model.DiskPublicDirectoryGrant, error)
	Exists(publicDirID uint64, userID string) (bool, error)
	ListByUserID(userID string) ([]uint64, error)
}

// fileDataRepo defines the interface for file data access needed by public directory.
type fileDataRepo interface {
	Create(file *model.DiskFile) error
	GetByID(id uint64) (*model.DiskFile, error)
	ListByFolder(userID string, folderID uint64) ([]model.DiskFile, error)
	UpdateOSSKey(id uint64, ossKey string) error
	SoftDelete(id uint64) error
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
	grantRepo  grantRepo
	fileRepo   fileDataRepo
	storage    storage.Storage
	dlSecret   string
	dlExpire   int
}

// NewPublicDirectoryService creates a new PublicDirectoryService.
func NewPublicDirectoryService(
	pdRepo *repository.PublicDirectoryRepo,
	folderRepo *repository.FolderRepo,
	grantRepo *repository.PublicDirectoryGrantRepo,
	fileRepo *repository.FileRepo,
	fileStorage storage.Storage,
	dlSecret string,
	dlExpire int,
) *PublicDirectoryService {
	return &PublicDirectoryService{
		pdRepo:     pdRepo,
		folderRepo: folderRepo,
		grantRepo:  grantRepo,
		fileRepo:   fileRepo,
		storage:    fileStorage,
		dlSecret:   dlSecret,
		dlExpire:   dlExpire,
	}
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

// ListVisible returns public directories visible to the given user.
// Includes: global scope, matching department scope, and individually granted directories.
func (s *PublicDirectoryService) ListVisible(department, userID string) ([]model.DiskPublicDirectory, error) {
	all, err := s.pdRepo.ListActive()
	if err != nil {
		return nil, err
	}

	grantSet := make(map[uint64]bool)
	if s.grantRepo != nil && userID != "" {
		grantedIDs, gErr := s.grantRepo.ListByUserID(userID)
		if gErr == nil {
			for _, id := range grantedIDs {
				grantSet[id] = true
			}
		}
	}

	seen := make(map[uint64]bool)
	var visible []model.DiskPublicDirectory
	for _, d := range all {
		if seen[d.ID] {
			continue
		}
		if d.Scope == ScopeGlobal {
			seen[d.ID] = true
			visible = append(visible, d)
		} else if d.Scope == ScopeDepartment && (d.Department == department || department == "") {
			seen[d.ID] = true
			visible = append(visible, d)
		} else if grantSet[d.ID] {
			seen[d.ID] = true
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

// --- File management ---

// ListFiles returns files in a public directory.
func (s *PublicDirectoryService) ListFiles(publicDirID uint64) ([]model.DiskFile, error) {
	pd, err := s.pdRepo.GetByID(publicDirID)
	if err != nil {
		return nil, fmt.Errorf("public directory not found: %w", err)
	}
	return s.fileRepo.ListByFolder(SystemUserID, pd.FolderID)
}

// UploadFile uploads a file to a public directory.
func (s *PublicDirectoryService) UploadFile(ctx context.Context, publicDirID uint64, fileName string, reader io.Reader, size int64, contentType string) (*model.DiskFile, error) {
	pd, err := s.pdRepo.GetByID(publicDirID)
	if err != nil {
		return nil, fmt.Errorf("public directory not found: %w", err)
	}

	file := &model.DiskFile{
		UserID:   SystemUserID,
		FolderID: pd.FolderID,
		FileName: fileName,
		FileSize: size,
		FileType: ext(fileName),
		Version:  1,
	}
	if err := s.fileRepo.Create(file); err != nil {
		return nil, fmt.Errorf("create file record: %w", err)
	}

	ossKey := oss.BuildKey(SystemUserID, pd.FixedPath, file.ID, fileName)
	if err := s.storage.Upload(ctx, ossKey, reader, size, contentType); err != nil {
		return nil, fmt.Errorf("upload to oss: %w", err)
	}
	file.OSSKey = ossKey
	if err := s.fileRepo.UpdateOSSKey(file.ID, ossKey); err != nil {
		return nil, fmt.Errorf("update file oss key: %w", err)
	}
	return file, nil
}

// DeleteFile deletes a file from a public directory.
func (s *PublicDirectoryService) DeleteFile(ctx context.Context, publicDirID, fileID uint64) error {
	pd, err := s.pdRepo.GetByID(publicDirID)
	if err != nil {
		return fmt.Errorf("public directory not found: %w", err)
	}
	file, err := s.fileRepo.GetByID(fileID)
	if err != nil {
		return fmt.Errorf("file not found: %w", err)
	}
	if file.FolderID != pd.FolderID {
		return fmt.Errorf("file does not belong to this public directory")
	}
	if file.IsDeleted {
		return fmt.Errorf("file already deleted")
	}
	return s.fileRepo.SoftDelete(fileID)
}

// CreateDownloadToken generates a download token for a file in a public directory.
func (s *PublicDirectoryService) CreateDownloadToken(publicDirID, fileID uint64) (string, int, error) {
	pd, err := s.pdRepo.GetByID(publicDirID)
	if err != nil {
		return "", 0, fmt.Errorf("public directory not found: %w", err)
	}
	file, err := s.fileRepo.GetByID(fileID)
	if err != nil {
		return "", 0, fmt.Errorf("file not found: %w", err)
	}
	if file.FolderID != pd.FolderID {
		return "", 0, fmt.Errorf("file does not belong to this public directory")
	}

	expire := s.dlExpire
	if expire <= 0 {
		expire = 300
	}
	token, err := download_token.Generate(s.dlSecret, SystemUserID, strconv.FormatUint(file.ID, 10), expire)
	if err != nil {
		return "", 0, fmt.Errorf("generate download token: %w", err)
	}
	return token, expire, nil
}

// GetFileDownloadURL returns a presigned download URL for a public directory file.
func (s *PublicDirectoryService) GetFileDownloadURL(ctx context.Context, publicDirID, fileID uint64) (string, error) {
	pd, err := s.pdRepo.GetByID(publicDirID)
	if err != nil {
		return "", fmt.Errorf("public directory not found: %w", err)
	}
	file, err := s.fileRepo.GetByID(fileID)
	if err != nil {
		return "", fmt.Errorf("file not found: %w", err)
	}
	if file.FolderID != pd.FolderID {
		return "", fmt.Errorf("file does not belong to this public directory")
	}
	dlURL, err := s.storage.PresignedDownloadURL(ctx, file.OSSKey, time.Hour, file.FileName)
	if err != nil {
		return "", fmt.Errorf("generate download url: %w", err)
	}
	return dlURL, nil
}

// CreateSubFolder creates a sub-folder within a public directory.
func (s *PublicDirectoryService) CreateSubFolder(ctx context.Context, publicDirID uint64, folderName string) (*model.DiskFolder, error) {
	pd, err := s.pdRepo.GetByID(publicDirID)
	if err != nil {
		return nil, fmt.Errorf("public directory not found: %w", err)
	}
	folder := &model.DiskFolder{
		UserID:     SystemUserID,
		ParentID:   pd.FolderID,
		FolderName: folderName,
		FullPath:   pd.FixedPath + "/" + folderName,
	}
	if err := s.folderRepo.Create(folder); err != nil {
		return nil, fmt.Errorf("create sub-folder: %w", err)
	}
	return folder, nil
}

// --- User authorization ---

// GrantUserAccess grants a user access to a public directory.
func (s *PublicDirectoryService) GrantUserAccess(publicDirID uint64, userID, grantedBy string) error {
	if _, err := s.pdRepo.GetByID(publicDirID); err != nil {
		return fmt.Errorf("public directory not found: %w", err)
	}
	exists, err := s.grantRepo.Exists(publicDirID, userID)
	if err != nil {
		return fmt.Errorf("check existing grant: %w", err)
	}
	if exists {
		return nil
	}
	grant := &model.DiskPublicDirectoryGrant{
		PublicDirectoryID: publicDirID,
		UserID:            userID,
		GrantedBy:         grantedBy,
	}
	return s.grantRepo.Create(grant)
}

// RevokeUserAccess revokes a user's access to a public directory.
func (s *PublicDirectoryService) RevokeUserAccess(publicDirID uint64, userID string) error {
	if _, err := s.pdRepo.GetByID(publicDirID); err != nil {
		return fmt.Errorf("public directory not found: %w", err)
	}
	return s.grantRepo.Delete(publicDirID, userID)
}

// ListGrantedUsers returns all user IDs granted access to a public directory.
func (s *PublicDirectoryService) ListGrantedUsers(publicDirID uint64) ([]string, error) {
	grants, err := s.grantRepo.ListByPublicDirID(publicDirID)
	if err != nil {
		return nil, err
	}
	userIDs := make([]string, 0, len(grants))
	for _, g := range grants {
		userIDs = append(userIDs, g.UserID)
	}
	return userIDs, nil
}

// IsUserGranted checks if a user has access to a public directory.
func (s *PublicDirectoryService) IsUserGranted(publicDirID uint64, userID string) (bool, error) {
	return s.grantRepo.Exists(publicDirID, userID)
}

// IsUserGrantedForFile checks if a user can access a file in a public directory.
func (s *PublicDirectoryService) IsUserGrantedForFile(fileID uint64, userID string) (bool, error) {
	if s.fileRepo == nil || s.grantRepo == nil || s.pdRepo == nil {
		return false, nil
	}
	file, err := s.fileRepo.GetByID(fileID)
	if err != nil || file.UserID != SystemUserID {
		return false, nil
	}
	pd, err := s.pdRepo.GetByFolderID(file.FolderID)
	if err != nil {
		return false, nil
	}
	return s.grantRepo.Exists(pd.ID, userID)
}
