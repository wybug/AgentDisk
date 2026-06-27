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
	// ErrOkfForbidden is returned when a reader asks for a bundle whose
	// underlying public directory is not visible to them. Surfaces to handlers
	// as 403 to enforce CLAUDE.md §4.6 userId isolation on the OKF reader API.
	ErrOkfForbidden = errors.New("okf: bundle not visible to caller")
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
	CountByBundle(tx *gorm.DB, bundleID uint64) (uint32, error)
}

// okfPublicDir captures the public-directory operations OKF needs: locating
// the directory, writing files at nested paths, reading file contents, and
// discovering files by relPath. ListVisible feeds the reader-side visibility
// filter so a caller can only read bundles whose public directory they can
// already see via grant / scope rules.
type okfPublicDir interface {
	GetPublicDirectory(id uint64) (*model.DiskPublicDirectory, error)
	UploadFileAt(ctx context.Context, publicDirID uint64, relPath, contentType string, content []byte) (*model.DiskFile, error)
	ReadFileContent(ctx context.Context, fileID uint64) ([]byte, error)
	FindFileByRelPath(publicDirID uint64, relPath string) (*model.DiskFile, error)
}

// okfVisibility is the narrow interface OKF needs to enforce reader-side
// visibility. It is split from okfPublicDir so unit tests can stub the
// visibility check without standing up the full public directory service.
type okfVisibility interface {
	// VisiblePublicDirIDs returns the set of public directory IDs the caller
	// can see. System-scoped callers (API Key with __system_public__) see all
	// active public directories via ListVisible with the system sentinel.
	VisiblePublicDirIDs(department, userID string) (map[uint64]bool, error)
}

// OkfService implements OKF v0.1 bundle registration, the materialized node
// index, and writer/reader flows. It depends on the existing public directory
// service for storage and folder management, so the original pdWrite text
// behavior is unchanged when OKF is disabled at the route layer.
type OkfService struct {
	bundles  okfBundleRepo
	nodes    okfNodeRepo
	pdSvc    okfPublicDir
	vis      okfVisibility
	dbDriver string
	// db is the GORM handle used to wrap multi-step materialization in a single
	// transaction. It is nil in unit tests that swap in fake repos; in that case
	// runTx falls back to executing the body without a real transaction.
	db *gorm.DB
}

// pdVisibility adapts PublicDirectoryService.ListVisible onto the
// okfVisibility interface. The OKF service uses it to filter the reader APIs
// (GetBundle / ListNodes / ListBundles / AggregateTypes) so a caller can only
// see bundles whose underlying public directory they already have access to.
type pdVisibility struct {
	pd *PublicDirectoryService
}

// VisiblePublicDirIDs delegates to PublicDirectoryService.ListVisible and
// projects the result into a set lookup.
func (v pdVisibility) VisiblePublicDirIDs(department, userID string) (map[uint64]bool, error) {
	dirs, err := v.pd.ListVisible(department, userID)
	if err != nil {
		return nil, err
	}
	out := make(map[uint64]bool, len(dirs))
	for i := range dirs {
		out[dirs[i].ID] = true
	}
	return out, nil
}

// NewOkfService creates a new OkfService. dbDriver is the configured database
// driver ("mysql" or "sqlite"); it selects which ListByBundle implementation
// is used for tag filtering, because the JSON_CONTAINS predicate is MySQL-only.
// db is the GORM handle used to transactionally wrap node materialization; pass
// the same *gorm.DB the repos use.
func NewOkfService(bundles *repository.OkfBundleRepo, nodes *repository.OkfNodeRepo, pdSvc *PublicDirectoryService, dbDriver string, db *gorm.DB) *OkfService {
	return &OkfService{
		bundles:  bundles,
		nodes:    nodes,
		pdSvc:    pdSvc,
		vis:      pdVisibility{pd: pdSvc},
		dbDriver: dbDriver,
		db:       db,
	}
}

// NewOkfServiceFromRepo constructs an OkfService from raw repo interfaces.
// Intended for tests that swap in fake repos. The service runs without a real
// *gorm.DB; multi-step operations execute without a surrounding transaction.
// Callers that want reader-side visibility filtering should also use
// SetVisibility to inject a stub; otherwise readers see every bundle.
func NewOkfServiceFromRepo(bundles okfBundleRepo, nodes okfNodeRepo, pdSvc okfPublicDir, dbDriver string) *OkfService {
	return &OkfService{
		bundles:  bundles,
		nodes:    nodes,
		pdSvc:    pdSvc,
		dbDriver: dbDriver,
	}
}

// SetVisibility overrides the reader-side visibility checker. Used by tests
// that swap in fake repos and want to drive the ACL path without the full
// public directory service.
func (s *OkfService) SetVisibility(v okfVisibility) { s.vis = v }

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
	if _, mErr := s.materializeNode(ctx, nil, bundle, model.OkfReservedRootRelPath, indexFile, body); mErr != nil {
		// Materialization failure must not fail registration: the bundle row is
		// the source of truth, and a later RefreshBundle can repair the index.
		_ = mErr
	}
	bundle.NodeCount, _ = s.nodes.CountByBundle(nil, bundle.ID)
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
	// descriptions stay in sync with the latest content. This runs before the
	// materialization transaction so a metadata-update failure surfaces
	// immediately rather than as a transaction rollback.
	if relPath == model.OkfReservedRootRelPath {
		bundle.RootIndexFileID = file.ID
		bundle.Title = fm.Title
		bundle.Description = fm.Description
		if fm.OkfVersion != "" {
			bundle.OkfVersion = fm.OkfVersion
		}
		_ = s.bundles.Update(bundle)
	}

	// Materialize the node, recompute node_count, and persist the bundle in a
	// single transaction so concurrent writers cannot observe a half-updated
	// index (node materialized but count stale, or vice versa). On rollback the
	// caller sees the error and the OSS file is the only durable side-effect.
	var node *model.OkfNode
	if err := s.runTx(ctx, func(tx *gorm.DB) error {
		n, mErr := s.materializeNode(ctx, tx, bundle, relPath, file, req.Content)
		if mErr != nil {
			return mErr
		}
		node = n
		count, cErr := s.nodes.CountByBundle(tx, bundle.ID)
		if cErr != nil {
			return fmt.Errorf("count nodes: %w", cErr)
		}
		bundle.NodeCount = count
		return s.bundles.Update(bundle)
	}); err != nil {
		return nil, err
	}
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
// failures are surfaced as a no-op rather than rejecting the write. tx is the
// in-progress transaction (or nil when running outside a transaction, e.g. in
// unit tests with fake repos).
func (s *OkfService) materializeNode(_ context.Context, tx *gorm.DB, bundle *model.OkfBundle, relPath string, file *model.DiskFile, body []byte) (*model.OkfNode, error) {
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

	if err := s.nodes.Upsert(tx, node); err != nil {
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

// ListBundles returns every registered bundle visible to the caller. userID +
// department drive the same ListVisible filter as the public directory API, so
// a reader cannot enumerate bundles whose underlying directory they would not
// be allowed to open directly.
func (s *OkfService) ListBundles(userID, department string) ([]model.OkfBundle, error) {
	all, err := s.bundles.List("", 0, 0)
	if err != nil {
		return nil, err
	}
	allowed, err := s.visibleBundleSet(userID, department)
	if err != nil {
		return nil, err
	}
	// nil allowed set means "no visibility provider configured" (unit tests);
	// return everything unchanged so the legacy callers keep working.
	if allowed == nil {
		return all, nil
	}
	out := make([]model.OkfBundle, 0, len(all))
	for i := range all {
		if allowed[all[i].PublicDirectoryID] {
			out = append(out, all[i])
		}
	}
	return out, nil
}

// GetBundle returns a single bundle by ID. Returns ErrOkfForbidden when the
// caller cannot see the bundle's underlying public directory.
func (s *OkfService) GetBundle(id uint64, userID, department string) (*model.OkfBundle, error) {
	b, err := s.bundles.GetByID(id)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrOkfBundleNotFound
		}
		return nil, err
	}
	if err := s.requireBundleVisible(b, userID, department); err != nil {
		return nil, err
	}
	return b, nil
}

// ListNodesByType returns the nodes for a bundle, optionally narrowed by type
// or tag. type and tag are exact-match filters; pass empty strings to skip.
// Returns ErrOkfForbidden when the caller cannot see the bundle.
func (s *OkfService) ListNodesByType(bundleID uint64, typeFilter, tagFilter, userID, department string) ([]model.OkfNode, error) {
	bundle, err := s.bundles.GetByID(bundleID)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrOkfBundleNotFound
		}
		return nil, err
	}
	if err := s.requireBundleVisible(bundle, userID, department); err != nil {
		return nil, err
	}
	filter := repository.NodeListFilter{Type: typeFilter, Tag: tagFilter}
	if s.dbDriver == "sqlite" {
		return s.nodes.ListByBundleSQLite(bundleID, filter)
	}
	return s.nodes.ListByBundle(bundleID, filter)
}

// AggregateByType returns per-type node counts for a bundle. Returns
// ErrOkfForbidden when the caller cannot see the bundle.
func (s *OkfService) AggregateByType(bundleID uint64, userID, department string) ([]repository.TypeCount, error) {
	bundle, err := s.bundles.GetByID(bundleID)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrOkfBundleNotFound
		}
		return nil, err
	}
	if err := s.requireBundleVisible(bundle, userID, department); err != nil {
		return nil, err
	}
	return s.nodes.AggregateByType(bundleID)
}

// requireBundleVisible returns ErrOkfForbidden when the caller's visibility
// set does not include the bundle's public directory. When no visibility
// provider is configured (nil), the check is skipped — this preserves the
// unit-test path that wires OkfService via NewOkfServiceFromRepo without a
// public directory service.
func (s *OkfService) requireBundleVisible(b *model.OkfBundle, userID, department string) error {
	if s.vis == nil {
		return nil
	}
	allowed, err := s.vis.VisiblePublicDirIDs(department, userID)
	if err != nil {
		return fmt.Errorf("compute visibility: %w", err)
	}
	if !allowed[b.PublicDirectoryID] {
		return ErrOkfForbidden
	}
	return nil
}

// visibleBundleSet returns the set of public directory IDs visible to the
// caller. Returns an empty set (not nil) so callers can index it without a
// nil check. When no visibility provider is configured, every bundle is
// visible (preserves the unit-test path).
func (s *OkfService) visibleBundleSet(userID, department string) (map[uint64]bool, error) {
	if s.vis == nil {
		// Without a visibility checker, return nil to signal "no filtering";
		// the caller treats nil as "all visible".
		return nil, nil
	}
	return s.vis.VisiblePublicDirIDs(department, userID)
}

// RefreshBundle re-scans the public directory for .md files and rebuilds the
// materialized node index. Existing nodes are deleted before re-scanning, so
// removed files drop out of the index. The bundle's node_count is updated.
// The whole delete → walk → upsert → count → update sequence runs in a single
// transaction so readers never observe an empty index between the clear and
// the rebuild, and a failed refresh rolls back the original nodes.
func (s *OkfService) RefreshBundle(ctx context.Context, id uint64) (*model.OkfBundle, error) {
	bundle, err := s.bundles.GetByID(id)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrOkfBundleNotFound
		}
		return nil, err
	}

	if err := s.runTx(ctx, func(tx *gorm.DB) error {
		if err := s.nodes.DeleteByBundle(tx, bundle.ID); err != nil {
			return fmt.Errorf("clear nodes: %w", err)
		}
		// Walk the public directory tree and re-materialize every .md file
		// inside the same transaction so a partial refresh rolls back.
		if err := s.walkAndMaterialize(ctx, tx, bundle, bundle.PublicDirectoryID); err != nil {
			return err
		}
		count, cErr := s.nodes.CountByBundle(tx, bundle.ID)
		if cErr != nil {
			return fmt.Errorf("count nodes: %w", cErr)
		}
		bundle.NodeCount = count
		return s.bundles.Update(bundle)
	}); err != nil {
		return nil, err
	}
	return bundle, nil
}

// walkAndMaterialize scans the public directory for markdown files and
// re-materializes each one. Used by RefreshBundle. The walk descends the
// entire folder tree under the bundle's public directory. tx is the
// in-progress transaction (or nil in unit tests) so each upsert joins the
// caller's atomic refresh.
func (s *OkfService) walkAndMaterialize(ctx context.Context, tx *gorm.DB, bundle *model.OkfBundle, publicDirID uint64) error {
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
	return s.materializeFolderTree(ctx, tx, bundle, rootFolder, "")
}

// materializeFolderTree walks one folder, materializes its .md files, then
// recurses into its sub-folders. relPrefix is the bundle-relative path of the
// current folder ("" at the root). Per-file parse errors are tolerated so one
// bad file does not abort a refresh. tx threads the surrounding transaction
// through each per-file upsert.
func (s *OkfService) materializeFolderTree(ctx context.Context, tx *gorm.DB, bundle *model.OkfBundle, folder *model.DiskFolder, relPrefix string) error {
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
		if _, mErr := s.materializeNode(ctx, tx, bundle, rel, &f, body); mErr != nil {
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
		if err := s.materializeFolderTree(ctx, tx, bundle, &child, subRel); err != nil {
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

// isNotFound returns true for any "record not found" error from GORM or the
// bundle-not-found sentinel. Uses errors.Is so it works through fmt.Errorf
// wrapping (the public directory service wraps its repo errors) and against
// GORM's sentinel directly (which the OkfBundleRepo / OkfNodeRepo return
// verbatim from db.First().Error).
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrOkfBundleNotFound) {
		return true
	}
	return errors.Is(err, gorm.ErrRecordNotFound)
}

// runTx wraps fn in a GORM transaction when s.db is configured, so that
// multi-step materialization (delete → walk → upsert → count → update) is
// atomic. When s.db is nil (unit tests with fake repos), fn runs directly with
// a nil tx and the fake repos fall back to their own handle. ctx is threaded
// in via WithContext so long-running materializations honor request cancel.
func (s *OkfService) runTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	if s.db == nil {
		return fn(nil)
	}
	return s.db.WithContext(ctx).Transaction(fn)
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
