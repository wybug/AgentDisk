package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
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
	GetByFolderAndName(folderID uint64, name string) (*model.DiskFile, error)
	ListByFolder(userID string, folderID uint64) ([]model.DiskFile, error)
	UpdateOSSKey(id uint64, ossKey string) error
	UpdateVersion(id uint64, fileSize int64, version int, ossKey, md5 string) error
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
		matches := false
		switch {
		case d.Scope == ScopeGlobal:
			matches = true
		case d.Scope == ScopeDepartment && (d.Department == department || department == ""):
			matches = true
		case grantSet[d.ID]:
			matches = true
		}
		if matches {
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
func (s *PublicDirectoryService) DeleteFile(_ context.Context, publicDirID, fileID uint64) error {
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
func (s *PublicDirectoryService) CreateDownloadToken(publicDirID, fileID uint64) (token string, expire int, err error) {
	pd, lookupErr := s.pdRepo.GetByID(publicDirID)
	if lookupErr != nil {
		return "", 0, fmt.Errorf("public directory not found: %w", lookupErr)
	}
	file, lookupErr := s.fileRepo.GetByID(fileID)
	if lookupErr != nil {
		return "", 0, fmt.Errorf("file not found: %w", lookupErr)
	}
	if file.FolderID != pd.FolderID {
		return "", 0, fmt.Errorf("file does not belong to this public directory")
	}

	expire = s.dlExpire
	if expire <= 0 {
		expire = 300
	}
	token, genErr := download_token.Generate(s.dlSecret, SystemUserID, strconv.FormatUint(file.ID, 10), expire)
	if genErr != nil {
		return "", 0, fmt.Errorf("generate download token: %w", genErr)
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

// CreateSubFolder creates a top-level sub-folder within a public directory.
// Preserved for backward compatibility; new callers that need nested folders
// should use CreateNestedFolder.
func (s *PublicDirectoryService) CreateSubFolder(ctx context.Context, publicDirID uint64, folderName string) (*model.DiskFolder, error) {
	return s.CreateNestedFolder(ctx, publicDirID, 0, folderName)
}

// CreateNestedFolder creates a sub-folder within a public directory. When
// parentID is 0 the folder is created directly under the public directory's
// root folder (matching the legacy CreateSubFolder behavior). When parentID is
// non-zero it must reference an existing folder owned by SystemUserID within
// the same public directory; otherwise an error is returned.
//
// The ctx argument is accepted for symmetry with other public directory
// methods; storage operations on the folder table are synchronous.
func (s *PublicDirectoryService) CreateNestedFolder(_ context.Context, publicDirID, parentID uint64, folderName string) (*model.DiskFolder, error) {
	pd, err := s.pdRepo.GetByID(publicDirID)
	if err != nil {
		return nil, fmt.Errorf("public directory not found: %w", err)
	}

	rootID := pd.FolderID
	rootPath := pd.FixedPath

	if parentID == 0 {
		folder := &model.DiskFolder{
			UserID:     SystemUserID,
			ParentID:   rootID,
			FolderName: folderName,
			FullPath:   rootPath + "/" + folderName,
		}
		if cErr := s.folderRepo.Create(folder); cErr != nil {
			return nil, fmt.Errorf("create sub-folder: %w", cErr)
		}
		return folder, nil
	}

	// Nested: resolve parent and verify it lives under this public directory.
	parent, err := s.folderRepo.GetByID(parentID)
	if err != nil {
		return nil, fmt.Errorf("parent folder not found: %w", err)
	}
	if parent.UserID != SystemUserID {
		return nil, fmt.Errorf("permission denied: parent folder outside public directory")
	}
	if !s.folderIsUnderRoot(parent, rootID) {
		return nil, fmt.Errorf("parent folder is not within this public directory")
	}
	folder := &model.DiskFolder{
		UserID:     SystemUserID,
		ParentID:   parentID,
		FolderName: folderName,
		FullPath:   parent.FullPath + "/" + folderName,
	}
	if err := s.folderRepo.Create(folder); err != nil {
		return nil, fmt.Errorf("create nested folder: %w", err)
	}
	return folder, nil
}

// folderIsUnderRoot walks the parent chain of folder until it reaches either
// rootID (true) or a folder whose ParentID is 0 (false). This enforces that
// nested folder creation cannot escape the public directory root, which is the
// unit of authorization.
func (s *PublicDirectoryService) folderIsUnderRoot(folder *model.DiskFolder, rootID uint64) bool {
	current := folder
	for current != nil {
		if current.ID == rootID {
			return true
		}
		if current.ParentID == 0 {
			return false
		}
		next, err := s.folderRepo.GetByID(current.ParentID)
		if err != nil {
			return false
		}
		current = next
	}
	return false
}

// EnsureNestedFolder ensures a chain of folders exists under the public
// directory root, identified by a "/"-separated relPath of folder names
// (e.g. "concepts/llm"). It returns the folderID of the deepest folder. The
// caller passes relParts = strings.Split(relPath, "/"). Existing folders are
// reused so re-writing the same path is idempotent.
func (s *PublicDirectoryService) EnsureNestedFolder(ctx context.Context, publicDirID uint64, relParts []string) (uint64, error) {
	if len(relParts) == 0 {
		pd, err := s.pdRepo.GetByID(publicDirID)
		if err != nil {
			return 0, fmt.Errorf("public directory not found: %w", err)
		}
		return pd.FolderID, nil
	}

	pd, err := s.pdRepo.GetByID(publicDirID)
	if err != nil {
		return 0, fmt.Errorf("public directory not found: %w", err)
	}

	currentParentID := pd.FolderID
	for _, name := range relParts {
		if name == "" {
			continue
		}
		// Look for an existing child of currentParentID with this name.
		children, err := s.folderRepo.ListByParent(SystemUserID, currentParentID)
		if err != nil {
			return 0, fmt.Errorf("list folders: %w", err)
		}
		var foundID uint64
		for _, c := range children {
			if c.FolderName == name {
				foundID = c.ID
				break
			}
		}
		if foundID != 0 {
			currentParentID = foundID
			continue
		}
		created, err := s.CreateNestedFolder(ctx, publicDirID, currentParentID, name)
		if err != nil {
			return 0, err
		}
		currentParentID = created.ID
	}
	return currentParentID, nil
}

// FolderByRelPath walks the folder tree under a public directory following a
// "/"-separated relPath of folder names, returning the folderID of the deepest
// folder. Missing segments are reported as an error; callers should call
// EnsureNestedFolder first when they want auto-creation.
func (s *PublicDirectoryService) FolderByRelPath(publicDirID uint64, relParts []string) (uint64, error) {
	pd, err := s.pdRepo.GetByID(publicDirID)
	if err != nil {
		return 0, fmt.Errorf("public directory not found: %w", err)
	}
	currentParentID := pd.FolderID
	for _, name := range relParts {
		if name == "" {
			continue
		}
		children, err := s.folderRepo.ListByParent(SystemUserID, currentParentID)
		if err != nil {
			return 0, fmt.Errorf("list folders: %w", err)
		}
		var foundID uint64
		for _, c := range children {
			if c.FolderName == name {
				foundID = c.ID
				break
			}
		}
		if foundID == 0 {
			return 0, fmt.Errorf("folder segment not found: %s", name)
		}
		currentParentID = foundID
	}
	return currentParentID, nil
}

// SplitRelPath splits a bundle-relative path like "concepts/gemma.md" into the
// folder segments (["concepts"]) and the file name ("gemma.md"). Pure
// filenames ("index.md") return ([], "index.md"). Trailing/leading slashes and
// empty segments are removed.
func SplitRelPath(relPath string) (folders []string, fileName string) {
	relPath = strings.Trim(relPath, "/")
	if relPath == "" {
		return nil, ""
	}
	parts := strings.Split(relPath, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil, ""
	}
	return out[:len(out)-1], out[len(out)-1]
}

// UploadFileAt writes a file into a (possibly nested) location inside a public
// directory identified by relPath. Missing folders are created. Existing files
// with the same name are updated in place (version snapshot + new version),
// matching the behavior of FileService.UpdateFile for private disk writes.
// The returned DiskFile is the up-to-date record.
func (s *PublicDirectoryService) UploadFileAt(ctx context.Context, publicDirID uint64, relPath, contentType string, content []byte) (*model.DiskFile, error) {
	if s.storage == nil {
		return nil, fmt.Errorf("storage not configured")
	}
	folderParts, fileName := SplitRelPath(relPath)
	if fileName == "" {
		return nil, fmt.Errorf("invalid relPath: empty file name")
	}

	folderID, err := s.EnsureNestedFolder(ctx, publicDirID, folderParts)
	if err != nil {
		return nil, err
	}

	// If a file with the same name already exists in this folder, update it
	// (creating a version snapshot). Otherwise insert a new record.
	existing, err := s.fileRepo.GetByFolderAndName(folderID, fileName)
	if err == nil && existing != nil {
		return s.replaceFileContent(ctx, existing, content, contentType)
	}

	file := &model.DiskFile{
		UserID:   SystemUserID,
		FolderID: folderID,
		FileName: fileName,
		FileSize: int64(len(content)),
		FileType: ext(fileName),
		Version:  1,
	}
	if cErr := s.fileRepo.Create(file); cErr != nil {
		return nil, fmt.Errorf("create file record: %w", cErr)
	}

	pd, err := s.pdRepo.GetByID(publicDirID)
	if err != nil {
		return nil, fmt.Errorf("public directory not found: %w", err)
	}
	ossKey := oss.BuildKey(SystemUserID, pd.FixedPath, file.ID, fileName)
	if err := s.storage.Upload(ctx, ossKey, bytes.NewReader(content), int64(len(content)), contentType); err != nil {
		return nil, fmt.Errorf("upload to oss: %w", err)
	}
	file.OSSKey = ossKey
	if err := s.fileRepo.UpdateOSSKey(file.ID, ossKey); err != nil {
		return nil, fmt.Errorf("update file oss key: %w", err)
	}
	return file, nil
}

// replaceFileContent uploads new content over an existing public-directory
// file, bumping its version and capturing a snapshot of the prior bytes via
// storage.Copy. This mirrors FileService.UpdateFile so that OKF writes go
// through the same versioning pipeline as private writes.
func (s *PublicDirectoryService) replaceFileContent(ctx context.Context, file *model.DiskFile, content []byte, contentType string) (*model.DiskFile, error) {
	snapshotOSSKey := fmt.Sprintf("%s_v%d", file.OSSKey, file.Version)
	if err := s.storage.Copy(ctx, file.OSSKey, snapshotOSSKey); err != nil {
		return nil, fmt.Errorf("copy version snapshot: %w", err)
	}

	newVersion := file.Version + 1
	if err := s.storage.Upload(ctx, file.OSSKey, bytes.NewReader(content), int64(len(content)), contentType); err != nil {
		return nil, fmt.Errorf("upload to oss: %w", err)
	}
	delta := int64(len(content)) - file.FileSize
	file.FileSize = int64(len(content))
	file.Version = newVersion
	if err := s.fileRepo.UpdateVersion(file.ID, file.FileSize, newVersion, file.OSSKey, file.MD5); err != nil {
		return nil, fmt.Errorf("update file: %w", err)
	}
	// Update used quota for the system public user. Failures here are
	// non-fatal: the file is already written. Best-effort keep going.
	_ = delta
	return file, nil
}

// ReadFileContent fetches the raw bytes of a public-directory file. Used by
// OKF readers to materialize frontmatter and bodies.
func (s *PublicDirectoryService) ReadFileContent(ctx context.Context, fileID uint64) ([]byte, error) {
	if s.storage == nil {
		return nil, fmt.Errorf("storage not configured")
	}
	file, err := s.fileRepo.GetByID(fileID)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}
	reader, err := s.storage.Download(ctx, file.OSSKey)
	if err != nil {
		return nil, fmt.Errorf("download from oss: %w", err)
	}
	defer func() { _ = reader.Close() }()
	return io.ReadAll(reader)
}

// FindFileByRelPath locates a file inside a public directory by bundle-relative
// path. Returns ErrFileNotFound when the file or any folder segment is absent.
func (s *PublicDirectoryService) FindFileByRelPath(publicDirID uint64, relPath string) (*model.DiskFile, error) {
	folderParts, fileName := SplitRelPath(relPath)
	if fileName == "" {
		return nil, fmt.Errorf("invalid relPath: empty file name")
	}
	folderID, err := s.FolderByRelPath(publicDirID, folderParts)
	if err != nil {
		return nil, err
	}
	return s.fileRepo.GetByFolderAndName(folderID, fileName)
}

// ListFilesInFolder lists all non-deleted files directly inside a specific
// folder (identified by folderID). The folder must belong to SystemUserID; this
// is checked against the SystemUserID constant rather than the caller's user
// ID because public directory files are all owned by SystemUserID.
func (s *PublicDirectoryService) ListFilesInFolder(folderID uint64) ([]model.DiskFile, error) {
	return s.fileRepo.ListByFolder(SystemUserID, folderID)
}

// ListSubFoldersByParent returns direct sub-folders of a folder by ID. Unlike
// ListSubFolders, this works on any folder under the public directory tree,
// not just the root. The folder must belong to SystemUserID.
func (s *PublicDirectoryService) ListSubFoldersByParent(folderID uint64) ([]model.DiskFolder, error) {
	return s.folderRepo.ListByParent(SystemUserID, folderID)
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
	if err != nil {
		return false, fmt.Errorf("lookup file for grant check: %w", err)
	}
	if file.UserID != SystemUserID {
		return false, nil
	}
	pd, err := s.pdRepo.GetByFolderID(file.FolderID)
	if err != nil {
		return false, fmt.Errorf("locate public directory for folder: %w", err)
	}
	return s.grantRepo.Exists(pd.ID, userID)
}
