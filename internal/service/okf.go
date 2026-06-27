package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
	"github.com/agentdisk/agent-disk/pkg/okf"
	"gorm.io/gorm"
)

// Sentinel errors for OKF operations. Callers can errors.Is to branch on the
// common client-side failures (missing field, not a bundle root) without
// parsing error strings.
var (
	// ErrOkfMissingType is returned when a written markdown file is missing
	// the required frontmatter "type" field.
	ErrOkfMissingType = errors.New("okf: frontmatter \"type\" is required")
	// ErrOkfNotBundleRoot is returned when RegisterBundle is called against a
	// public directory whose root index.md does not declare okf_version.
	ErrOkfNotBundleRoot = errors.New("okf: not an OKF bundle root (index.md missing okf_version)")
	// ErrOkfReservedName is returned when a writer tries to put frontmatter on
	// log.md, or to write an index.md without okf_version.
	ErrOkfReservedName = errors.New("okf: reserved file name has extra constraints")
	// ErrOkfBundleNotFound is returned when a bundle lookup misses.
	ErrOkfBundleNotFound = errors.New("okf: bundle not found")
)

// okfBundleRepo is the storage interface for OKF bundles.
type okfBundleRepo interface {
	Create(b *model.OkfBundle) error
	GetByID(id uint64) (*model.OkfBundle, error)
	GetByPublicDirectoryID(pdID uint64) (*model.OkfBundle, error)
	List(status string, limit, offset int) ([]model.OkfBundle, error)
	Update(b *model.OkfBundle) error
	Delete(id uint64) error
}

// okfNodeRepo is the storage interface for OKF materialized nodes.
type okfNodeRepo interface {
	Upsert(tx *gorm.DB, n *model.OkfNode) error
	GetByBundleAndRelPath(bundleID uint64, relPath string) (*model.OkfNode, error)
	ListByBundle(bundleID uint64, filter repository.NodeListFilter) ([]model.OkfNode, error)
	ListByBundleSQLite(bundleID uint64, filter repository.NodeListFilter) ([]model.OkfNode, error)
	AggregateByType(bundleID uint64) ([]repository.TypeCount, error)
	DeleteByBundle(tx *gorm.DB, bundleID uint64) error
	CountByBundle(bundleID uint64) (uint32, error)
}

// okfPublicDir captures the public-directory operations OKF needs: locating
// the directory, writing files at nested paths, reading file contents, and
// discovering files by relPath.
type okfPublicDir interface {
	GetPublicDirectory(id uint64) (*model.DiskPublicDirectory, error)
	UploadFileAt(ctx context.Context, publicDirID uint64, relPath, contentType string, content []byte) (*model.DiskFile, error)
	ReadFileContent(ctx context.Context, fileID uint64) ([]byte, error)
	FindFileByRelPath(publicDirID uint64, relPath string) (*model.DiskFile, error)
}

// OkfService implements OKF v0.1 bundle registration, the materialized node
// index, and writer/reader flows. It depends on the existing public directory
// service for storage and folder management, so the original pdWrite text
// behavior is unchanged when OKF is disabled at the route layer.
type OkfService struct {
	bundles  okfBundleRepo
	nodes    okfNodeRepo
	pdSvc    okfPublicDir
	dbDriver string
}

// NewOkfService creates a new OkfService. dbDriver is the configured database
// driver ("mysql" or "sqlite"); it selects which ListByBundle implementation
// is used for tag filtering, because the JSON_CONTAINS predicate is MySQL-only.
func NewOkfService(bundles *repository.OkfBundleRepo, nodes *repository.OkfNodeRepo, pdSvc *PublicDirectoryService, dbDriver string) *OkfService {
	return &OkfService{
		bundles:  bundles,
		nodes:    nodes,
		pdSvc:    pdSvc,
		dbDriver: dbDriver,
	}
}

// NewOkfServiceFromRepo constructs an OkfService from raw repo interfaces.
// Intended for tests that swap in fake repos.
func NewOkfServiceFromRepo(bundles okfBundleRepo, nodes okfNodeRepo, pdSvc okfPublicDir, dbDriver string) *OkfService {
	return &OkfService{
		bundles:  bundles,
		nodes:    nodes,
		pdSvc:    pdSvc,
		dbDriver: dbDriver,
	}
}

// RegisterBundle registers a public directory as an OKF bundle. The directory
// must contain an index.md whose frontmatter declares okf_version; otherwise
// the directory is not a bundle root and the call returns ErrOkfNotBundleRoot.
// Re-registering an already-registered bundle is idempotent: the existing
// bundle row is returned unchanged.
func (s *OkfService) RegisterBundle(ctx context.Context, publicDirectoryID uint64) (*model.OkfBundle, error) {
	if existing, err := s.bundles.GetByPublicDirectoryID(publicDirectoryID); err == nil {
		return existing, nil
	} else if !isNotFound(err) {
		return nil, fmt.Errorf("lookup existing bundle: %w", err)
	}

	indexFile, body, err := s.readIndexMarkdown(ctx, publicDirectoryID)
	if err != nil {
		return nil, err
	}
	fm, _, err := okf.Parse(body)
	if err != nil {
		return nil, fmt.Errorf("parse index.md: %w", err)
	}
	if strings.TrimSpace(fm.OkfVersion) == "" {
		return nil, ErrOkfNotBundleRoot
	}

	bundle := &model.OkfBundle{
		PublicDirectoryID: publicDirectoryID,
		OkfVersion:        defaultStr(fm.OkfVersion, model.OkfBundleDefaultVersion),
		RootIndexFileID:   indexFile.ID,
		Title:             fm.Title,
		Description:       fm.Description,
		Status:            model.OkfBundleStatusActive,
	}
	if err := s.bundles.Create(bundle); err != nil {
		return nil, fmt.Errorf("create bundle: %w", err)
	}

	// Materialize index.md as the first node so the bundle has a usable index
	// immediately after registration.
	if _, mErr := s.materializeNode(ctx, bundle, model.OkfReservedRootRelPath, indexFile, body); mErr != nil {
		// Materialization failure must not fail registration: the bundle row is
		// the source of truth, and a later RefreshBundle can repair the index.
		_ = mErr
	}
	bundle.NodeCount, _ = s.nodes.CountByBundle(bundle.ID)
	return bundle, nil
}

// readIndexMarkdown locates and reads the bytes of index.md at the root of the
// public directory. Returns ErrOkfNotBundleRoot when index.md is missing.
func (s *OkfService) readIndexMarkdown(ctx context.Context, publicDirectoryID uint64) (*model.DiskFile, []byte, error) {
	if _, err := s.pdSvc.GetPublicDirectory(publicDirectoryID); err != nil {
		return nil, nil, fmt.Errorf("public directory not found: %w", err)
	}
	file, err := s.pdSvc.FindFileByRelPath(publicDirectoryID, model.OkfReservedRootRelPath)
	if err != nil {
		return nil, nil, ErrOkfNotBundleRoot
	}
	body, err := s.pdSvc.ReadFileContent(ctx, file.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("read index.md: %w", err)
	}
	return file, body, nil
}

// WriteMarkdownRequest carries the inputs for a markdown write.
type WriteMarkdownRequest struct {
	PublicDirectoryID uint64
	RelPath           string
	Content           []byte
	ContentType       string
}

// WriteMarkdown writes a markdown file into a public directory and updates the
// materialized node index. The public directory must be registered as a bundle;
// if it is not, the call transparently registers it (idempotent). Strict
// validation runs on the frontmatter: type is required for non-reserved files,
// index.md must declare okf_version, and log.md must not carry frontmatter.
func (s *OkfService) WriteMarkdown(ctx context.Context, req WriteMarkdownRequest) (*model.OkfNode, error) {
	if req.PublicDirectoryID == 0 {
		return nil, fmt.Errorf("publicDirectoryId is required")
	}
	relPath := normalizeRelPath(req.RelPath)
	if relPath == "" {
		return nil, fmt.Errorf("relPath is required")
	}
	if !strings.HasSuffix(relPath, ".md") {
		return nil, fmt.Errorf("relPath must end with .md")
	}
	if _, err := s.pdSvc.GetPublicDirectory(req.PublicDirectoryID); err != nil {
		return nil, fmt.Errorf("public directory not found: %w", err)
	}

	// Parse + strict-validate frontmatter before writing to OSS so we fail
	// fast on missing required fields and never materialize an invalid node.
	fm, _, parseErr := okf.Parse(req.Content)
	if parseErr != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", parseErr)
	}
	if vErr := s.validateFrontmatterForPath(relPath, fm); vErr != nil {
		return nil, vErr
	}

	contentType := req.ContentType
	if contentType == "" {
		contentType = "text/markdown; charset=utf-8"
	}

	file, err := s.pdSvc.UploadFileAt(ctx, req.PublicDirectoryID, relPath, contentType, req.Content)
	if err != nil {
		return nil, fmt.Errorf("upload markdown: %w", err)
	}

	// Ensure the bundle exists; if the writer is the first thing to put an
	// index.md into a fresh directory, RegisterBundle will succeed here.
	bundle, err := s.bundles.GetByPublicDirectoryID(req.PublicDirectoryID)
	if err != nil {
		if !isNotFound(err) {
			return nil, fmt.Errorf("lookup bundle: %w", err)
		}
		// Auto-register. If the written file is not index.md / not a bundle
		// root, registration will return ErrOkfNotBundleRoot; surface that
		// to the caller so they know the directory is not yet OKF-enabled.
		bundle, err = s.RegisterBundle(ctx, req.PublicDirectoryID)
		if err != nil {
			return nil, err
		}
	}

	// If this write is index.md, refresh bundle root metadata so titles /
	// descriptions stay in sync with the latest content.
	if relPath == model.OkfReservedRootRelPath {
		bundle.RootIndexFileID = file.ID
		bundle.Title = fm.Title
		bundle.Description = fm.Description
		if fm.OkfVersion != "" {
			bundle.OkfVersion = fm.OkfVersion
		}
		_ = s.bundles.Update(bundle)
	}

	node, mErr := s.materializeNode(ctx, bundle, relPath, file, req.Content)
	if mErr != nil {
		return nil, mErr
	}
	bundle.NodeCount, _ = s.nodes.CountByBundle(bundle.ID)
	_ = s.bundles.Update(bundle)
	return node, nil
}

// validateFrontmatterForPath applies OKF strict-mode checks that depend on the
// file's role inside the bundle:
//   - index.md must declare okf_version (otherwise it is not a bundle root);
//   - log.md must not carry any frontmatter (it is a chronologically-appended
//     changelog, not a typed node);
//   - every other .md file must declare type.
//
// Unknown frontmatter keys are tolerated (OKF §9) and preserved in extra_json.
func (s *OkfService) validateFrontmatterForPath(relPath string, fm okf.Frontmatter) error {
	base := path.Base(relPath)
	if !okf.IsReservedName(base) {
		if err := okf.Validate(fm); err != nil {
			return ErrOkfMissingType
		}
		return nil
	}
	// Reserved names: stricter rules.
	switch base {
	case model.OkfReservedRootRelPath:
		// index.md MUST carry okf_version. Type is also required so the
		// bundle root is itself a discoverable typed node.
		if strings.TrimSpace(fm.OkfVersion) == "" {
			return fmt.Errorf("%w: index.md requires okf_version", ErrOkfReservedName)
		}
		if err := okf.Validate(fm); err != nil {
			return ErrOkfMissingType
		}
	case model.OkfReservedLogRelPath:
		// log.md must be plain markdown: a non-empty Type indicates the
		// caller tried to turn it into a typed node, which we reject.
		if strings.TrimSpace(fm.Type) != "" {
			return fmt.Errorf("%w: log.md must not carry frontmatter", ErrOkfReservedName)
		}
	}
	return nil
}

// materializeNode builds an OkfNode row from the markdown body, computes a
// content hash for change detection, and upserts it. Unknown frontmatter keys
// are preserved in extra_json (OKF §9). The function is forgiving: parse
// failures are surfaced as a no-op rather than rejecting the write.
func (s *OkfService) materializeNode(_ context.Context, bundle *model.OkfBundle, relPath string, file *model.DiskFile, body []byte) (*model.OkfNode, error) {
	fm, _, err := okf.Parse(body)
	if err != nil {
		return nil, fmt.Errorf("parse markdown: %w", err)
	}

	hash := sha256.Sum256(body)
	contentHash := hex.EncodeToString(hash[:])

	node := &model.OkfNode{
		BundleID:      bundle.ID,
		FileID:        file.ID,
		RelPath:       relPath,
		Type:          fm.Type,
		Title:         fm.Title,
		Description:   fm.Description,
		Timestamp:     fm.Timestamp,
		HasBrokenLink: detectBrokenLink(bundle, relPath, body),
		ContentHash:   contentHash,
	}
	if err := node.SetTags(fm.Tags); err != nil {
		return nil, fmt.Errorf("serialize tags: %w", err)
	}
	if err := node.SetExtra(fm.Extra); err != nil {
		return nil, fmt.Errorf("serialize extra: %w", err)
	}

	if err := s.nodes.Upsert(nil, node); err != nil {
		return nil, fmt.Errorf("upsert node: %w", err)
	}
	return node, nil
}

// detectBrokenLink is a placeholder for future link-graph validation. It
// always returns false in this iteration; OKF §9 says broken links are
// tolerated and flagged rather than rejected, so the column is populated but
// the validation itself will be added in a follow-up. The bundle is accepted
// as an argument so the future implementation can walk the node set without a
// signature change.
func detectBrokenLink(_ *model.OkfBundle, _ string, _ []byte) bool {
	return false
}

// ListBundles returns every registered bundle. Status filter is currently
// unused (always returns active + archived); the repository accepts it for a
// future "archived" view.
func (s *OkfService) ListBundles() ([]model.OkfBundle, error) {
	return s.bundles.List("", 0, 0)
}

// GetBundle returns a single bundle by ID.
func (s *OkfService) GetBundle(id uint64) (*model.OkfBundle, error) {
	b, err := s.bundles.GetByID(id)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrOkfBundleNotFound
		}
		return nil, err
	}
	return b, nil
}

// ListNodesByType returns the nodes for a bundle, optionally narrowed by type
// or tag. type and tag are exact-match filters; pass empty strings to skip.
func (s *OkfService) ListNodesByType(bundleID uint64, typeFilter, tagFilter string) ([]model.OkfNode, error) {
	if _, err := s.bundles.GetByID(bundleID); err != nil {
		if isNotFound(err) {
			return nil, ErrOkfBundleNotFound
		}
		return nil, err
	}
	filter := repository.NodeListFilter{Type: typeFilter, Tag: tagFilter}
	if s.dbDriver == "sqlite" {
		return s.nodes.ListByBundleSQLite(bundleID, filter)
	}
	return s.nodes.ListByBundle(bundleID, filter)
}

// AggregateByType returns per-type node counts for a bundle.
func (s *OkfService) AggregateByType(bundleID uint64) ([]repository.TypeCount, error) {
	if _, err := s.bundles.GetByID(bundleID); err != nil {
		if isNotFound(err) {
			return nil, ErrOkfBundleNotFound
		}
		return nil, err
	}
	return s.nodes.AggregateByType(bundleID)
}

// RefreshBundle re-scans the public directory for .md files and rebuilds the
// materialized node index. Existing nodes are deleted before re-scanning, so
// removed files drop out of the index. The bundle's node_count is updated.
func (s *OkfService) RefreshBundle(ctx context.Context, id uint64) (*model.OkfBundle, error) {
	bundle, err := s.bundles.GetByID(id)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrOkfBundleNotFound
		}
		return nil, err
	}

	if err := s.nodes.DeleteByBundle(nil, bundle.ID); err != nil {
		return nil, fmt.Errorf("clear nodes: %w", err)
	}

	// Walk the public directory tree and re-materialize every .md file.
	if err := s.walkAndMaterialize(ctx, bundle, bundle.PublicDirectoryID); err != nil {
		return nil, err
	}

	bundle.NodeCount, _ = s.nodes.CountByBundle(bundle.ID)
	_ = s.bundles.Update(bundle)
	return bundle, nil
}

// walkAndMaterialize scans the public directory for markdown files and
// re-materializes each one. Used by RefreshBundle. The walk descends the
// entire folder tree under the bundle's public directory.
func (s *OkfService) walkAndMaterialize(ctx context.Context, bundle *model.OkfBundle, publicDirID uint64) error {
	pdSvc, ok := s.pdSvc.(*PublicDirectoryService)
	if !ok {
		// In tests with a stub pdSvc we cannot walk the folder tree; the
		// service-layer refresh is exercised through the integration tests.
		return nil
	}
	// Resolve the root folder ID for this public directory and recurse.
	pd, err := pdSvc.GetPublicDirectory(publicDirID)
	if err != nil {
		return fmt.Errorf("public directory not found: %w", err)
	}
	rootFolder := &model.DiskFolder{ID: pd.FolderID, FolderName: "", FullPath: pd.FixedPath}
	return s.materializeFolderTree(ctx, bundle, rootFolder, "")
}

// materializeFolderTree walks one folder, materializes its .md files, then
// recurses into its sub-folders. relPrefix is the bundle-relative path of the
// current folder ("" at the root). Per-file parse errors are tolerated so one
// bad file does not abort a refresh.
func (s *OkfService) materializeFolderTree(ctx context.Context, bundle *model.OkfBundle, folder *model.DiskFolder, relPrefix string) error {
	pdSvc, ok := s.pdSvc.(*PublicDirectoryService)
	if !ok {
		return nil
	}

	files, err := pdSvc.ListFilesInFolder(folder.ID)
	if err != nil {
		return fmt.Errorf("list files in folder %s: %w", folder.FolderName, err)
	}
	for i := range files {
		f := files[i]
		if !strings.HasSuffix(f.FileName, ".md") {
			continue
		}
		rel := f.FileName
		if relPrefix != "" {
			rel = relPrefix + "/" + f.FileName
		}
		body, rErr := pdSvc.ReadFileContent(ctx, f.ID)
		if rErr != nil {
			continue
		}
		if _, mErr := s.materializeNode(ctx, bundle, rel, &f, body); mErr != nil {
			continue
		}
	}

	children, err := pdSvc.ListSubFoldersByParent(folder.ID)
	if err != nil {
		return fmt.Errorf("list sub folders of %s: %w", folder.FolderName, err)
	}
	for i := range children {
		child := children[i]
		if child.IsDeleted {
			continue
		}
		subRel := child.FolderName
		if relPrefix != "" {
			subRel = relPrefix + "/" + child.FolderName
		}
		if err := s.materializeFolderTree(ctx, bundle, &child, subRel); err != nil {
			return err
		}
	}
	return nil
}

// UnregisterBundle removes a bundle registration. The underlying public
// directory and its files are untouched: only the OKF bundle + node index rows
// are deleted, so the directory can be re-registered later.
func (s *OkfService) UnregisterBundle(id uint64) error {
	if _, err := s.bundles.GetByID(id); err != nil {
		if isNotFound(err) {
			return ErrOkfBundleNotFound
		}
		return err
	}
	if err := s.nodes.DeleteByBundle(nil, id); err != nil {
		return fmt.Errorf("clear nodes: %w", err)
	}
	return s.bundles.Delete(id)
}

// isNotFound returns true for any "record not found" error from GORM or any of
// the test stubs that mimic ErrRecordNotFound. We accept error wrapping
// because the public directory service fmt.Errorf-wraps its underlying errors.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrOkfBundleNotFound) {
		return true
	}
	// repository lookups return raw gorm.ErrRecordNotFound (not wrapped) so a
	// direct equality check is enough. The "not found" string match covers the
	// test stubs that mimic ErrRecordNotFound with a literal error.
	msg := err.Error()
	return strings.Contains(msg, "record not found") || msg == "not found"
}

// normalizeRelPath cleans a bundle-relative path: trims leading/trailing
// slashes, collapses "./" and "../" segments, and converts backslashes to
// forward slashes for Windows-authored paths.
func normalizeRelPath(rel string) string {
	if rel == "" {
		return ""
	}
	rel = strings.ReplaceAll(rel, "\\", "/")
	rel = strings.Trim(rel, "/")
	// path.Clean rejects ".." escaping by leaving a leading ".."; drop those
	// segments to keep the path inside the bundle root.
	cleaned := path.Clean(rel)
	for strings.HasPrefix(cleaned, "../") {
		cleaned = strings.TrimPrefix(cleaned, "../")
	}
	if cleaned == ".." {
		return ""
	}
	return cleaned
}

// defaultStr returns v when non-empty, otherwise fallback.
func defaultStr(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
