package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

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
	GetByID(id uint64) (*model.OkfNode, error)
	GetByBundleAndRelPath(bundleID uint64, relPath string) (*model.OkfNode, error)
	ListByBundle(bundleID uint64, filter repository.NodeListFilter) ([]model.OkfNode, error)
	ListByBundleSQLite(bundleID uint64, filter repository.NodeListFilter) ([]model.OkfNode, error)
	ListByIDs(ids []uint64) ([]model.OkfNode, error)
	AggregateByType(bundleID uint64) ([]repository.TypeCount, error)
	DeleteByBundle(tx *gorm.DB, bundleID uint64) error
	CountByBundle(tx *gorm.DB, bundleID uint64) (uint32, error)
	Search(query string, filter repository.SearchFilter, limit int, cursor uint64) ([]model.OkfNode, uint64, error)
	SearchSQLite(query string, filter repository.SearchFilter, limit int, cursor uint64) ([]model.OkfNode, uint64, error)
}

// okfEdgeRepo is the storage interface for the OKF edge graph. Implementations
// cluster on public_dir_id so per-bundle reads and writes stay index-only.
type okfEdgeRepo interface {
	ReplaceForSrc(tx *gorm.DB, publicDirID, srcNodeID uint64, edges []model.OkfEdge) error
	ListBySrc(publicDirID, srcNodeID uint64, limit int) ([]model.OkfEdge, error)
	ListByDst(publicDirID, dstNodeID uint64, limit int) ([]model.OkfEdge, error)
	ListOutBySrcBatch(publicDirID uint64, srcIDs []uint64, limit int) ([]model.OkfEdge, error)
	ListInByDstBatch(publicDirID uint64, dstIDs []uint64, limit int) ([]model.OkfEdge, error)
	ListBrokenByBundle(publicDirID uint64, cursor uint64, limit int) ([]model.OkfEdge, uint64, error)
	CountByBundle(tx *gorm.DB, bundleID, publicDirID uint64) (uint32, error)
	StatsByBundle(bundleID, publicDirID uint64) (repository.EdgeStats, error)
	DeleteByBundle(tx *gorm.DB, publicDirID uint64) error
	AdjustBacklinks(tx *gorm.DB, increment, decrement []uint64) error
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
	edges    okfEdgeRepo
	pdSvc    okfPublicDir
	vis      okfVisibility
	dbDriver string
	// db is the GORM handle used to wrap multi-step materialization in a single
	// transaction. It is nil in unit tests that swap in fake repos; in that case
	// runTx falls back to executing the body without a real transaction.
	db *gorm.DB
	// lock serializes writers per bundle. Defaults to NoOpBundleLock when the
	// process is not configured with Redis; tests inject a Redis lock when
	// they want to exercise the contention path.
	lock BundleLock
	// cache stores per-node adjacency lists so P3b BFS expansions skip the
	// edge table on warm paths. Defaults to NoOpGraphCache; the router wires
	// a Redis-backed cache when cfg.Okf.RedisAddr is set, alongside the lock.
	cache GraphCache
	// autoIndexUpdate toggles the asynchronous index.md regeneration after a
	// write. Defaults to true. When false, writes still append log.md and
	// recompute has_broken_link, but skip the index regen — useful for
	// batch imports that want to rebuild the index once at the end.
	autoIndexUpdate bool
	// logUserID is the fallback user identifier written to log.md when the
	// caller does not supply one (e.g. system-initiated refresh). Empty is
	// allowed; log.md rows tolerate a blank user column.
	logUserID string
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
func NewOkfService(bundles *repository.OkfBundleRepo, nodes *repository.OkfNodeRepo, edges *repository.OkfEdgeRepo, pdSvc *PublicDirectoryService, dbDriver string, db *gorm.DB) *OkfService {
	return &OkfService{
		bundles:         bundles,
		nodes:           nodes,
		edges:           edges,
		pdSvc:           pdSvc,
		vis:             pdVisibility{pd: pdSvc},
		dbDriver:        dbDriver,
		db:              db,
		lock:            NoOpBundleLock{},
		cache:           NoOpGraphCache{},
		autoIndexUpdate: true,
	}
}

// NewOkfServiceFromRepo constructs an OkfService from raw repo interfaces.
// Intended for tests that swap in fake repos. The service runs without a real
// *gorm.DB; multi-step operations execute without a surrounding transaction.
// Callers that want reader-side visibility filtering should also use
// SetVisibility to inject a stub; otherwise readers see every bundle.
func NewOkfServiceFromRepo(bundles okfBundleRepo, nodes okfNodeRepo, edges okfEdgeRepo, pdSvc okfPublicDir, dbDriver string) *OkfService {
	return &OkfService{
		bundles:         bundles,
		nodes:           nodes,
		edges:           edges,
		pdSvc:           pdSvc,
		dbDriver:        dbDriver,
		lock:            NoOpBundleLock{},
		cache:           NoOpGraphCache{},
		autoIndexUpdate: true,
	}
}

// SetVisibility overrides the reader-side visibility checker. Used by tests
// that swap in fake repos and want to drive the ACL path without the full
// public directory service.
func (s *OkfService) SetVisibility(v okfVisibility) { s.vis = v }

// SetBundleLock overrides the writer serialization lock. Router wires a Redis
// lock when the deployment runs Redis; tests inject a NoOp or a fake lock to
// exercise the contention path without a real Redis.
func (s *OkfService) SetBundleLock(l BundleLock) {
	if l == nil {
		l = NoOpBundleLock{}
	}
	s.lock = l
}

// SetGraphCache overrides the BFS adjacency cache. Router wires a Redis cache
// when the deployment runs Redis alongside the bundle lock; tests inject a
// NoOp (the default) to keep unit tests Redis-free. Passing nil is treated
// as NoOp so a misconfiguration cannot nil-deref the BFS path.
func (s *OkfService) SetGraphCache(c GraphCache) {
	if c == nil {
		c = NoOpGraphCache{}
	}
	s.cache = c
}

// SetAutoIndexUpdate toggles asynchronous index.md regeneration after each
// write. Defaults to true. Router sets this from config.OkfConfig.AutoIndexUpdate.
func (s *OkfService) SetAutoIndexUpdate(v bool) { s.autoIndexUpdate = v }

// SetLogUserID sets the fallback user identifier written to log.md rows when
// a writer call does not supply one. Router injects the API key / JWT user.
func (s *OkfService) SetLogUserID(uid string) { s.logUserID = uid }

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
	// Best-effort log entry. A failure here is non-fatal: the bundle is
	// registered, and a missing log row is preferable to a rollback.
	_ = s.AppendLogEntry(ctx, bundle.ID, LogActionRegister, model.OkfReservedRootRelPath, s.logUserID)
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

	// Acquire the per-bundle writer lock so two concurrent writers cannot
	// interleave their materialize-and-count transactions. The release fn is
	// invoked on return; if Acquire returns ErrOkfLockHeld the caller surfaces
	// it as a 409 Conflict.
	release, err := s.acquireBundleLock(ctx, req.PublicDirectoryID)
	if err != nil {
		return nil, err
	}
	defer release()

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

	// Detect whether this is a create or an update so the log entry is honest.
	// We look up by (bundle_id, rel_path) before materialization; a miss means
	// create, a hit means update. The lookup is best-effort — a concurrent
	// writer could insert between this check and the upsert, but the lock
	// above makes that rare and the worst case is a mislabeled log row.
	_, existedBefore := s.lookupNodeForLog(bundle.ID, relPath)

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
		n, err := s.materializeNodeTx(ctx, tx, bundle, relPath, file, req.Content)
		if err != nil {
			return err
		}
		node = n
		return s.refreshBundleCounts(tx, bundle)
	}); err != nil {
		return nil, err
	}

	// Synchronous post-write hooks: log the action and recompute the node's
	// broken-link flag from the just-written body. Both are best-effort and
	// never fail the write — the OSS bytes + node row are already durable.
	s.postWriteSyncHooks(ctx, bundle, node, relPath, req.Content, existedBefore)

	// Drop the source node's cached adjacency. materializeEdges just rewrote
	// its outgoing edges; without this invalidate a BFS could serve the old
	// neighbor list for up to adjTTL. Best-effort — a Redis blip means the
	// entry ages out via TTL, which is the bounded-staleness contract.
	if s.cache != nil && node != nil {
		_ = s.cache.Invalidate(ctx, bundle.ID, []uint64{node.ID})
	}

	// Asynchronous index.md regeneration. Skipped when AutoIndexUpdate is
	// off (batch-import mode). The goroutine has its own timeout so a slow
	// regeneration cannot pin a goroutine forever.
	if s.autoIndexUpdate {
		s.scheduleIndexRegen(ctx, bundle.ID)
	}

	return node, nil
}

// acquireBundleLock resolves the bundle ID for a public directory and acquires
// the per-bundle writer lock. The release fn is always non-nil when err is
// nil so callers can `defer release()` unconditionally.
func (s *OkfService) acquireBundleLock(ctx context.Context, publicDirectoryID uint64) (func(), error) {
	bundle, err := s.bundles.GetByPublicDirectoryID(publicDirectoryID)
	if err != nil {
		// Bundle not yet registered: no lock to take. The writer will
		// auto-register it inside the critical section; until then there is
		// nothing to protect. We swallow the lookup error deliberately — a
		// gorm.ErrRecordNotFound here means "first write to this PD", not a
		// real failure, and surfacing it would force every cold-start write
		// to handle the not-found path.
		return func() {}, nil //nolint:nilerr // intentional: not-found is the "no bundle yet" path
	}
	release, err := s.lock.Acquire(ctx, bundle.ID)
	if err != nil {
		return nil, err
	}
	if release == nil {
		release = func() {}
	}
	return release, nil
}

// lookupNodeForLog returns (node, true) when the (bundle_id, rel_path) row
// already exists. Used by WriteMarkdown to decide whether the log row is
// "create" or "update". Errors are mapped to (nil, false) so a transient
// lookup failure degrades to "create" rather than failing the write.
func (s *OkfService) lookupNodeForLog(bundleID uint64, relPath string) (*model.OkfNode, bool) {
	n, err := s.nodes.GetByBundleAndRelPath(bundleID, relPath)
	if err != nil || n == nil {
		return nil, false
	}
	return n, true
}

// materializeNodeTx wraps the in-transaction work for WriteMarkdown: upsert
// the node, then materialize its outgoing edges when an edge repo is wired.
// Split out from WriteMarkdown to keep the parent function's cognitive
// complexity under the lint ceiling.
func (s *OkfService) materializeNodeTx(ctx context.Context, tx *gorm.DB, bundle *model.OkfBundle, relPath string, file *model.DiskFile, content []byte) (*model.OkfNode, error) {
	node, err := s.materializeNode(ctx, tx, bundle, relPath, file, content)
	if err != nil {
		return nil, err
	}
	if s.edges != nil {
		if gErr := s.materializeEdges(ctx, tx, bundle, node, content); gErr != nil {
			return nil, gErr
		}
	}
	return node, nil
}

// refreshBundleCounts recomputes node_count (always) and edge_count (when an
// edge repo is wired) and persists the bundle row. Called inside the
// WriteMarkdown transaction so the counts commit atomically with the node.
// The Save runs on tx — going through s.bundles.Update here would re-acquire
// the write lock on a different pool connection and deadlock against the
// already-open transaction, surfacing as "database is locked" after the
// busy_timeout expires.
func (s *OkfService) refreshBundleCounts(tx *gorm.DB, bundle *model.OkfBundle) error {
	count, err := s.nodes.CountByBundle(tx, bundle.ID)
	if err != nil {
		return fmt.Errorf("count nodes: %w", err)
	}
	bundle.NodeCount = count
	if s.edges != nil {
		edgeCount, err := s.edges.CountByBundle(tx, bundle.ID, bundle.PublicDirectoryID)
		if err != nil {
			return fmt.Errorf("count edges: %w", err)
		}
		bundle.EdgeCount = edgeCount
	}
	// In unit tests s.db is nil and runTx hands us a nil tx; fall back to the
	// repo's own handle there. In production tx is always non-nil and we must
	// Save on it to avoid deadlocking against the open transaction.
	if tx == nil {
		return s.bundles.Update(bundle)
	}
	return tx.Save(bundle).Error
}

// postWriteSyncHooks runs the synchronous post-write side-effects:
//   - AppendLogEntry: records "create" / "update" so log.md stays in sync.
//   - broken-link scan for this single node, when the edge graph is not wired.
//     With P3a the edge materializer updates has_broken_link inside the write
//     transaction, so this single-node scan becomes redundant — it only runs
//     in the unit-test path (no edge repo) to keep the flag fresh.
//
// Both are best-effort; failures are swallowed to keep the write durable.
func (s *OkfService) postWriteSyncHooks(ctx context.Context, _ *model.OkfBundle, node *model.OkfNode, relPath string, body []byte, existedBefore bool) {
	if node == nil {
		return
	}
	action := LogActionCreate
	if existedBefore {
		action = LogActionUpdate
	}
	_ = s.AppendLogEntry(ctx, node.BundleID, action, relPath, s.logUserID)

	if s.edges != nil {
		// Edge materializer already maintained has_broken_link; skip the
		// redundant scan to avoid a second ExtractLinks pass.
		return
	}
	broken := s.nodeHasBrokenLink(node.BundleID, relPath, body)
	_ = s.updateNodeBrokenFlag(node, broken)
}

// nodeHasBrokenLink reports whether any bundle-relative link in body fails to
// resolve to an existing node. External / anchor links are never broken.
func (s *OkfService) nodeHasBrokenLink(bundleID uint64, relPath string, body []byte) bool {
	links := ExtractLinks(body, relPath)
	for _, li := range links {
		if li.LinkKind != LinkKindBundle {
			continue
		}
		if _, err := s.nodes.GetByBundleAndRelPath(bundleID, li.DstRelPath); err != nil {
			return true
		}
	}
	return false
}

// scheduleIndexRegen launches a goroutine that regenerates index.md. The
// goroutine uses a fresh context with a 30s timeout so the request handler
// can return immediately while the regen runs in the background. A failure
// is silent: the index will be rebuilt on the next write or by an explicit
// /regenerate-index call.
//
// The request-scoped context is intentionally not propagated: the regen must
// outlive the HTTP request that triggered it.
func (s *OkfService) scheduleIndexRegen(_ context.Context, bundleID uint64) {
	go func() { //nolint:gosec // G118: intentional — async regen outlives the request
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		bundle, err := s.bundles.GetByID(bundleID)
		if err != nil {
			return
		}
		_, _, _ = s.regenerateIndexInternal(ctx, bundle)
	}()
}

// OnFileDeleted is the post-delete hook called by the public-directory writer
// path when a bundle file is removed. It re-runs the broken-link scan (so
// surviving nodes that linked to the deleted file get their has_broken_link
// flag flipped), appends a "delete" log row, and asynchronously regenerates
// index.md.
//
// The hook is best-effort: failures are swallowed because the underlying file
// deletion has already succeeded by the time this runs. Callers that want
// strict consistency should issue a RefreshBundle + RegenerateIndex pair.
func (s *OkfService) OnFileDeleted(ctx context.Context, publicDirectoryID uint64, relPath string) {
	bundle, err := s.bundles.GetByPublicDirectoryID(publicDirectoryID)
	if err != nil {
		// Not a registered bundle: nothing for OKF to do.
		return
	}
	// Best-effort log + full broken-link rescan. The rescan is O(N) in the
	// node count, which is acceptable because deletes are rare relative to
	// writes.
	_ = s.AppendLogEntry(ctx, bundle.ID, LogActionDelete, relPath, s.logUserID)
	_, _ = s.ScanBundleLinks(ctx, bundle.ID)
	if s.autoIndexUpdate {
		s.scheduleIndexRegen(ctx, bundle.ID)
	}
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
		HasBrokenLink: s.nodeHasBrokenLink(bundle.ID, relPath, body),
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
		if s.edges != nil {
			if err := s.edges.DeleteByBundle(tx, bundle.PublicDirectoryID); err != nil {
				return fmt.Errorf("clear edges: %w", err)
			}
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
		if s.edges != nil {
			edgeCount, eErr := s.edges.CountByBundle(tx, bundle.ID, bundle.PublicDirectoryID)
			if eErr != nil {
				return fmt.Errorf("count edges: %w", eErr)
			}
			bundle.EdgeCount = edgeCount
		}
		// In unit tests tx is nil; fall back to the repo's own handle there.
		// In production tx must be used to avoid deadlock against the
		// transaction's write lock.
		if tx == nil {
			return s.bundles.Update(bundle)
		}
		return tx.Save(bundle).Error
	}); err != nil {
		return nil, err
	}
	// Best-effort log entry marking a refresh-as-scan. Non-fatal: a missing
	// row does not undo the refresh.
	_ = s.AppendLogEntry(ctx, bundle.ID, LogActionScan, "", s.logUserID)
	return bundle, nil
}

// walkAndMaterialize scans the public directory for markdown files and
// re-materializes each one. Used by RefreshBundle. The walk descends the
// entire folder tree under the bundle's public directory. tx is the
// in-progress transaction (or nil in unit tests) so each upsert joins the
// caller's atomic refresh.
//
// Edges are materialized in a second pass once every node exists, so a
// forward-reference from a.md to b.md resolves correctly even if b.md is
// visited after a.md in the walk order.
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
	if err := s.materializeFolderTree(ctx, tx, bundle, rootFolder, ""); err != nil {
		return err
	}
	if s.edges == nil {
		return nil
	}
	// Second pass: with every node in place, derive edges from each body.
	// Cross-references resolve cleanly now that the full node set is durable.
	return s.materializeEdgeTree(ctx, tx, bundle, rootFolder, "")
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
		// log.md is the auto-generated append-only changelog and must not be
		// materialized as a typed node. OKF mandates it stay frontmatter-free,
		// so the upsert below would otherwise create a degenerate Type=""
		// node. Skip it here so NodeCount reflects only the meaningful index.
		if rel == model.OkfReservedLogRelPath {
			continue
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

// materializeEdgeTree is the second-pass edge walker paired with
// materializeFolderTree. It visits the same folders in the same order, reads
// each .md body again, and runs materializeEdges against the node that
// already exists at that relPath. The first pass guarantees every node is
// durable, so bundle-relative links resolve cleanly.
func (s *OkfService) materializeEdgeTree(ctx context.Context, tx *gorm.DB, bundle *model.OkfBundle, folder *model.DiskFolder, relPrefix string) error {
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
		if rel == model.OkfReservedLogRelPath {
			continue
		}
		node, nErr := s.nodes.GetByBundleAndRelPath(bundle.ID, rel)
		if nErr != nil {
			continue
		}
		body, rErr := pdSvc.ReadFileContent(ctx, f.ID)
		if rErr != nil {
			continue
		}
		if mErr := s.materializeEdges(ctx, tx, bundle, node, body); mErr != nil {
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
		if err := s.materializeEdgeTree(ctx, tx, bundle, &child, subRel); err != nil {
			return err
		}
	}
	return nil
}

// UnregisterBundle removes a bundle registration. The underlying public
// directory and its files are untouched: only the OKF bundle + node index rows
// are deleted, so the directory can be re-registered later.
func (s *OkfService) UnregisterBundle(id uint64) error {
	bundle, err := s.bundles.GetByID(id)
	if err != nil {
		if isNotFound(err) {
			return ErrOkfBundleNotFound
		}
		return err
	}
	// Best-effort log entry before the bundle row goes away. After unregister
	// the bundle row no longer exists, so AppendLogEntry cannot run after.
	// Use context.Background() since this entry point has no request context.
	_ = s.AppendLogEntry(context.Background(), id, LogActionUnregister, "", s.logUserID)
	if err := s.nodes.DeleteByBundle(nil, id); err != nil {
		return fmt.Errorf("clear nodes: %w", err)
	}
	// Best-effort edge cleanup. The bundle row is going away; leaving edges
	// behind would corrupt a future re-registration of the same public
	// directory. Swallow the error: a failed edge delete should not un-delete
	// the bundle row we are about to drop.
	if s.edges != nil {
		_ = s.edges.DeleteByBundle(nil, bundle.PublicDirectoryID)
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
