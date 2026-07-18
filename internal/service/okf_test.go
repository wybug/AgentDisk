package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
	"github.com/agentdisk/agent-disk/pkg/okf"
	"github.com/agentdisk/agent-disk/pkg/storage"
	"gorm.io/gorm"
)

// ── Fake repositories ──

type fakeOkfBundleRepo struct {
	mu      sync.RWMutex
	bundles map[uint64]*model.OkfBundle
	nextID  uint64
}

func newFakeOkfBundleRepo() *fakeOkfBundleRepo {
	return &fakeOkfBundleRepo{bundles: map[uint64]*model.OkfBundle{}, nextID: 1}
}

func (r *fakeOkfBundleRepo) Create(b *model.OkfBundle) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if b.ID == 0 {
		b.ID = r.nextID
		r.nextID++
	}
	r.bundles[b.ID] = b
	return nil
}

func (r *fakeOkfBundleRepo) GetByID(id uint64) (*model.OkfBundle, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if b, ok := r.bundles[id]; ok {
		return b, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeOkfBundleRepo) GetByPublicDirectoryID(pdID uint64) (*model.OkfBundle, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, b := range r.bundles {
		if b.PublicDirectoryID == pdID {
			return b, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeOkfBundleRepo) List(_ string, _, _ int) ([]model.OkfBundle, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]model.OkfBundle, 0, len(r.bundles))
	for _, b := range r.bundles {
		out = append(out, *b)
	}
	return out, nil
}

func (r *fakeOkfBundleRepo) Update(b *model.OkfBundle) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bundles[b.ID] = b
	return nil
}

func (r *fakeOkfBundleRepo) Delete(id uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.bundles, id)
	return nil
}

type fakeOkfNodeRepo struct {
	mu               sync.RWMutex
	nodes            map[uint64]*model.OkfNode
	byKey            map[string]uint64 // bundleID:relPath -> node ID
	nextID           uint64
	batchLinkLookups atomic.Int64 // counts ListByBundleAndRelPaths calls (N+1 guard)
}

func newFakeOkfNodeRepo() *fakeOkfNodeRepo {
	return &fakeOkfNodeRepo{
		nodes:  map[uint64]*model.OkfNode{},
		byKey:  map[string]uint64{},
		nextID: 1,
	}
}

func keyFor(bundleID uint64, rel string) string {
	return strings.Join([]string{uintToStr(bundleID), rel}, ":")
}

func uintToStr(n uint64) string {
	if n == 0 {
		return "0"
	}
	out := make([]byte, 0, 20)
	for n > 0 {
		out = append([]byte{byte('0' + n%10)}, out...)
		n /= 10
	}
	return string(out)
}

func (r *fakeOkfNodeRepo) Upsert(_ *gorm.DB, n *model.OkfNode) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := keyFor(n.BundleID, n.RelPath)
	if id, ok := r.byKey[k]; ok {
		existing := r.nodes[id]
		n.ID = existing.ID
		n.CreatedAt = existing.CreatedAt
		r.nodes[n.ID] = n
		return nil
	}
	if n.ID == 0 {
		n.ID = r.nextID
		r.nextID++
	}
	r.nodes[n.ID] = n
	r.byKey[k] = n.ID
	return nil
}

func (r *fakeOkfNodeRepo) GetByBundleAndRelPath(bundleID uint64, rel string) (*model.OkfNode, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if id, ok := r.byKey[keyFor(bundleID, rel)]; ok {
		return r.nodes[id], nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeOkfNodeRepo) ListByBundleAndRelPaths(bundleID uint64, relPaths []string) ([]model.OkfNode, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	r.batchLinkLookups.Add(1)
	out := make([]model.OkfNode, 0, len(relPaths))
	seen := map[uint64]bool{}
	for _, rel := range relPaths {
		if id, ok := r.byKey[keyFor(bundleID, rel)]; ok && !seen[id] {
			out = append(out, *r.nodes[id])
			seen[id] = true
		}
	}
	return out, nil
}

func (r *fakeOkfNodeRepo) GetByID(id uint64) (*model.OkfNode, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if n, ok := r.nodes[id]; ok {
		return n, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fakeOkfNodeRepo) ListByIDs(ids []uint64) ([]model.OkfNode, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]model.OkfNode, 0, len(ids))
	for _, id := range ids {
		if n, ok := r.nodes[id]; ok {
			out = append(out, *n)
		}
	}
	return out, nil
}

func (r *fakeOkfNodeRepo) ListByBundle(bundleID uint64, filter repository.NodeListFilter, limit, offset int) ([]model.OkfNode, error) {
	return r.filterNodes(bundleID, filter, limit, offset), nil
}

func (r *fakeOkfNodeRepo) ListByBundleSQLite(bundleID uint64, filter repository.NodeListFilter, limit, offset int) ([]model.OkfNode, error) {
	return r.filterNodes(bundleID, filter, limit, offset), nil
}

func (r *fakeOkfNodeRepo) filterNodes(bundleID uint64, filter repository.NodeListFilter, limit, offset int) []model.OkfNode {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []model.OkfNode{}
	for _, n := range r.nodes {
		if n.BundleID != bundleID {
			continue
		}
		if filter.Type != "" && n.Type != filter.Type {
			continue
		}
		if filter.Tag != "" {
			tags, _ := n.GetTags()
			found := false
			for _, t := range tags {
				if t == filter.Tag {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		out = append(out, *n)
	}
	// Stable order by ID so P2 pagination tests get a deterministic walk.
	// The real repo orders by rel_path; for the fake we need a stable key and
	// ID is the natural one. Callers that want rel_path order can sort the
	// result themselves.
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if limit > 0 {
		if offset >= len(out) {
			return []model.OkfNode{}
		}
		out = out[offset:]
		if len(out) > limit {
			out = out[:limit]
		}
	}
	return out
}

func (r *fakeOkfNodeRepo) AggregateByType(bundleID uint64) ([]repository.TypeCount, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	counts := map[string]uint32{}
	for _, n := range r.nodes {
		if n.BundleID != bundleID {
			continue
		}
		counts[n.Type]++
	}
	out := make([]repository.TypeCount, 0, len(counts))
	for t, c := range counts {
		out = append(out, repository.TypeCount{Type: t, Count: c})
	}
	return out, nil
}

func (r *fakeOkfNodeRepo) AggregateTypesByBundles(bundleIDs []uint64) ([]repository.TypeCount, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	allowed := make(map[uint64]bool, len(bundleIDs))
	for _, id := range bundleIDs {
		allowed[id] = true
	}
	counts := map[string]uint32{}
	for _, n := range r.nodes {
		if !allowed[n.BundleID] {
			continue
		}
		counts[n.Type]++
	}
	out := make([]repository.TypeCount, 0, len(counts))
	for t, c := range counts {
		out = append(out, repository.TypeCount{Type: t, Count: c})
	}
	return out, nil
}

func (r *fakeOkfNodeRepo) DeleteByBundle(_ *gorm.DB, bundleID uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, n := range r.nodes {
		if n.BundleID == bundleID {
			delete(r.nodes, id)
			delete(r.byKey, keyFor(n.BundleID, n.RelPath))
		}
	}
	return nil
}

func (r *fakeOkfNodeRepo) CountByBundle(_ *gorm.DB, bundleID uint64) (uint32, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var count uint32
	for _, n := range r.nodes {
		if n.BundleID == bundleID {
			count++
		}
	}
	return count, nil
}

// Search is the fake equivalent of OkfNodeRepo.Search. It mirrors the
// MySQL FULLTEXT behavior — case-insensitive substring match on title or
// description — so service-layer ACL filtering can be exercised without
// standing up a real DB. Both Search and SearchSQLite share the body since
// the fake doesn't have a real tokenizer.
func (r *fakeOkfNodeRepo) Search(query string, filter repository.SearchFilter, limit int, cursor uint64) ([]model.OkfNode, uint64, error) {
	return r.searchFake(query, filter, limit, cursor)
}

// SearchSQLite mirrors Search on the fake; the real impls diverge (FULLTEXT
// vs. FTS5) but the fake's substring scan is dialect-agnostic.
func (r *fakeOkfNodeRepo) SearchSQLite(query string, filter repository.SearchFilter, limit int, cursor uint64) ([]model.OkfNode, uint64, error) {
	return r.searchFake(query, filter, limit, cursor)
}

func (r *fakeOkfNodeRepo) searchFake(query string, filter repository.SearchFilter, limit int, cursor uint64) ([]model.OkfNode, uint64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if query == "" {
		return nil, 0, nil
	}
	needle := strings.ToLower(query)
	bundleSet := map[uint64]bool{}
	for _, id := range filter.BundleIDs {
		bundleSet[id] = true
	}
	var ids []uint64
	for _, n := range r.nodes {
		if cursor > 0 && n.ID <= cursor {
			continue
		}
		if len(bundleSet) > 0 && !bundleSet[n.BundleID] {
			continue
		}
		if filter.Type != "" && n.Type != filter.Type {
			continue
		}
		if strings.Contains(strings.ToLower(n.Title), needle) ||
			strings.Contains(strings.ToLower(n.Description), needle) {
			ids = append(ids, n.ID)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	next := uint64(0)
	if len(ids) > limit {
		next = ids[limit-1]
		ids = ids[:limit]
	}
	out := make([]model.OkfNode, 0, len(ids))
	for _, id := range ids {
		out = append(out, *r.nodes[id])
	}
	return out, next, nil
}

// fakeOkfEdgeRepo is an in-memory okfEdgeRepo for service-layer tests. The
// state is keyed by (publicDirID, srcNodeID) → slice of edges, mirroring the
// real repo's clustering. Backlink bumps are applied straight onto the
// corresponding fakeOkfNodeRepo's node rows.
type fakeOkfEdgeRepo struct {
	mu     sync.RWMutex
	nodes  *fakeOkfNodeRepo
	edges  map[uint64][]model.OkfEdge // keyed by srcNodeID
	nextID uint64
}

func newFakeOkfEdgeRepo(nodes *fakeOkfNodeRepo) *fakeOkfEdgeRepo {
	return &fakeOkfEdgeRepo{nodes: nodes, edges: map[uint64][]model.OkfEdge{}}
}

func (r *fakeOkfEdgeRepo) ReplaceForSrc(_ *gorm.DB, publicDirID, srcNodeID uint64, edges []model.OkfEdge) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range edges {
		if edges[i].ID == 0 {
			r.nextID++
			edges[i].ID = r.nextID
		}
		edges[i].PublicDirID = publicDirID
		edges[i].SrcNodeID = srcNodeID
	}
	r.edges[srcNodeID] = append([]model.OkfEdge(nil), edges...)
	return nil
}

func (r *fakeOkfEdgeRepo) ListBySrc(_, srcNodeID uint64, _ int) ([]model.OkfEdge, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := append([]model.OkfEdge(nil), r.edges[srcNodeID]...)
	return out, nil
}

func (r *fakeOkfEdgeRepo) ListByDst(_, dstNodeID uint64, _ int) ([]model.OkfEdge, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []model.OkfEdge
	for _, group := range r.edges {
		for _, e := range group {
			if e.DstExists && e.DstNodeID == dstNodeID {
				out = append(out, e)
			}
		}
	}
	return out, nil
}

func (r *fakeOkfEdgeRepo) ListOutBySrcBatch(_ uint64, srcIDs []uint64, _ int) ([]model.OkfEdge, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	want := map[uint64]bool{}
	for _, id := range srcIDs {
		want[id] = true
	}
	var out []model.OkfEdge
	for srcID, group := range r.edges {
		if !want[srcID] {
			continue
		}
		for _, e := range group {
			if e.DstExists {
				out = append(out, e)
			}
		}
	}
	return out, nil
}

func (r *fakeOkfEdgeRepo) ListInByDstBatch(_ uint64, dstIDs []uint64, _ int) ([]model.OkfEdge, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	want := map[uint64]bool{}
	for _, id := range dstIDs {
		want[id] = true
	}
	var out []model.OkfEdge
	for _, group := range r.edges {
		for _, e := range group {
			if e.DstExists && want[e.DstNodeID] {
				out = append(out, e)
			}
		}
	}
	return out, nil
}

func (r *fakeOkfEdgeRepo) StatsByBundle(_, _ uint64) (repository.EdgeStats, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var stats repository.EdgeStats
	for _, group := range r.edges {
		for _, e := range group {
			stats.Total++
			if e.DstExists {
				stats.Live++
			} else {
				stats.Broken++
			}
		}
	}
	return stats, nil
}

func (r *fakeOkfEdgeRepo) ListBrokenByBundle(_ uint64, _ uint64, _ int) ([]model.OkfEdge, uint64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []model.OkfEdge
	for _, group := range r.edges {
		for _, e := range group {
			if !e.DstExists {
				out = append(out, e)
			}
		}
	}
	return out, 0, nil
}

func (r *fakeOkfEdgeRepo) CountBrokenByBundle(_ uint64) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var count int64
	for _, group := range r.edges {
		for _, e := range group {
			if !e.DstExists {
				count++
			}
		}
	}
	return count, nil
}

func (r *fakeOkfEdgeRepo) ListBrokenBundleLinks(publicDirID, cursor uint64, limit int) ([]repository.BrokenLinkRow, uint64, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	type cand struct {
		edgeID, srcNodeID    uint64
		dstRelPath, linkText string
		srcLine              int
		linkKind             string
	}
	var cands []cand
	r.mu.RLock()
	for _, group := range r.edges {
		for _, e := range group {
			if e.PublicDirID != publicDirID || e.DstExists || e.LinkKind != "bundle" {
				continue
			}
			if cursor > 0 && e.ID <= cursor {
				continue
			}
			cands = append(cands, cand{e.ID, e.SrcNodeID, e.DstRelPath, e.LinkText, e.SrcLine, e.LinkKind})
		}
	}
	r.mu.RUnlock() // release edge lock before touching the node fake (no nested locks)
	sort.Slice(cands, func(i, j int) bool { return cands[i].edgeID < cands[j].edgeID })
	rows := make([]repository.BrokenLinkRow, 0, len(cands))
	for _, c := range cands {
		srcRel := ""
		if n, err := r.nodes.GetByID(c.srcNodeID); err == nil {
			srcRel = n.RelPath
		}
		rows = append(rows, repository.BrokenLinkRow{
			EdgeID: c.edgeID, SrcNodeID: c.srcNodeID, SrcRelPath: srcRel,
			DstRelPath: c.dstRelPath, SrcLine: c.srcLine, LinkText: c.linkText, LinkKind: c.linkKind,
		})
	}
	next := uint64(0)
	if len(rows) > limit {
		next = rows[limit-1].EdgeID
		rows = rows[:limit]
	}
	return rows, next, nil
}

func (r *fakeOkfEdgeRepo) CountByBundle(_ *gorm.DB, _ uint64, _ uint64) (uint32, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var count int
	for _, group := range r.edges {
		count += len(group)
	}
	if count > int(^uint32(0)) {
		return ^uint32(0), nil
	}
	return uint32(count), nil
}

func (r *fakeOkfEdgeRepo) DeleteByBundle(_ *gorm.DB, _ uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.edges = map[uint64][]model.OkfEdge{}
	return nil
}

func (r *fakeOkfEdgeRepo) AdjustBacklinks(_ *gorm.DB, increment, decrement []uint64) error {
	r.nodes.mu.Lock()
	defer r.nodes.mu.Unlock()
	apply := func(ids []uint64, delta int32) {
		for _, id := range ids {
			if n, ok := r.nodes.nodes[id]; ok {
				if delta < 0 && n.BacklinkCount == 0 {
					continue
				}
				n.BacklinkCount = uint32(int32(n.BacklinkCount) + delta)
			}
		}
	}
	apply(increment, +1)
	apply(decrement, -1)
	return nil
}

// in-memory map of relPath -> file content and exposes the operations the OKF
// service calls.
type fakeOkfPublicDir struct {
	mu            sync.RWMutex
	pd            *model.DiskPublicDirectory
	files         map[string]*model.DiskFile
	nextFID       uint64
	rawContent    map[string][]byte
	readFailureOn string // when non-empty, ReadFileContent for this relPath errors
	uploadErr     error  // when non-nil, UploadFileAt returns this error
}

func newFakeOkfPublicDir(pd *model.DiskPublicDirectory) *fakeOkfPublicDir {
	return &fakeOkfPublicDir{pd: pd, files: map[string]*model.DiskFile{}, nextFID: 1}
}

func (p *fakeOkfPublicDir) GetPublicDirectory(_ uint64) (*model.DiskPublicDirectory, error) {
	if p.pd == nil {
		return nil, errors.New("not found")
	}
	return p.pd, nil
}

func (p *fakeOkfPublicDir) UploadFileAt(_ context.Context, _ uint64, relPath, _ string, content []byte) (*model.DiskFile, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.uploadErr != nil {
		return nil, p.uploadErr
	}
	if p.rawContent == nil {
		p.rawContent = map[string][]byte{}
	}
	if existing, ok := p.files[relPath]; ok {
		existing.FileSize = int64(len(content))
		existing.Version++
		p.rawContent[relPath] = content
		snap := *existing
		return &snap, nil
	}
	f := &model.DiskFile{
		ID:       p.nextFID,
		FileName: relPath,
		FileSize: int64(len(content)),
		Version:  1,
	}
	p.nextFID++
	p.files[relPath] = f
	p.rawContent[relPath] = content
	snap := *f
	return &snap, nil
}

func (p *fakeOkfPublicDir) ReadFileContent(_ context.Context, fileID uint64) ([]byte, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for rel, f := range p.files {
		if f.ID == fileID {
			if p.readFailureOn != "" && rel == p.readFailureOn {
				return nil, errors.New("simulated read failure")
			}
			return p.contentForLocked(rel), nil
		}
	}
	return nil, errors.New("not found")
}

func (p *fakeOkfPublicDir) FindFileByRelPath(_ uint64, relPath string) (*model.DiskFile, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if f, ok := p.files[relPath]; ok {
		snap := *f
		return &snap, nil
	}
	return nil, errors.New("not found")
}

// seedContent stores raw bytes against a relPath for ReadFileContent.
func (p *fakeOkfPublicDir) seedContent(rel string, body []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.rawContent == nil {
		p.rawContent = map[string][]byte{}
	}
	if _, ok := p.files[rel]; !ok {
		p.files[rel] = &model.DiskFile{ID: p.nextFID, FileName: rel, Version: 1}
		p.nextFID++
	}
	p.files[rel].FileSize = int64(len(body))
	p.rawContent[rel] = body
}

// contentForLocked is the lock-free inner helper; caller holds the read lock.
func (p *fakeOkfPublicDir) contentForLocked(rel string) []byte {
	if p.rawContent == nil {
		return nil
	}
	return p.rawContent[rel]
}

// ── Helpers ──

func mustIndexMD(t *testing.T, okfVersion, title string) []byte {
	t.Helper()
	body := "---\n"
	body += "type: bundle\n"
	if title != "" {
		body += "title: " + title + "\n"
	}
	if okfVersion != "" {
		body += "okf_version: \"" + okfVersion + "\"\n"
	}
	body += "---\n\n# Bundle\n"
	return []byte(body)
}

func mustTypedMD(typ, title string) []byte {
	body := "---\ntype: " + typ + "\ntitle: " + title + "\n---\n\nBody\n"
	return []byte(body)
}

// ── Tests ──

func TestOkfService_RegisterBundle_Success(t *testing.T) {
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	pd := &model.DiskPublicDirectory{ID: 7, FolderID: 100, DisplayName: "kb", FixedPath: "/public/kb"}
	pub := newFakeOkfPublicDir(pd)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", "My Bundle"))

	svc := NewOkfServiceFromRepo(bundles, nodes, newFakeOkfEdgeRepo(nodes), pub, "sqlite")
	bundle, err := svc.RegisterBundle(context.Background(), 7)
	if err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}
	if bundle.PublicDirectoryID != 7 {
		t.Errorf("publicDirectoryID = %d, want 7", bundle.PublicDirectoryID)
	}
	if bundle.OkfVersion != "0.1" {
		t.Errorf("okfVersion = %q, want 0.1", bundle.OkfVersion)
	}
	if bundle.Title != "My Bundle" {
		t.Errorf("title = %q, want My Bundle", bundle.Title)
	}
	if bundle.NodeCount != 1 {
		t.Errorf("nodeCount = %d, want 1 (index.md materialized)", bundle.NodeCount)
	}
}

func TestOkfService_RegisterBundle_NotRoot(t *testing.T) {
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	pd := &model.DiskPublicDirectory{ID: 7, FolderID: 100, FixedPath: "/public/kb"}
	pub := newFakeOkfPublicDir(pd)
	// index.md without okf_version
	pub.seedContent("index.md", []byte("---\ntype: bundle\n---\nbody"))

	svc := NewOkfServiceFromRepo(bundles, nodes, newFakeOkfEdgeRepo(nodes), pub, "sqlite")
	_, err := svc.RegisterBundle(context.Background(), 7)
	if !errors.Is(err, ErrOkfNotBundleRoot) {
		t.Fatalf("expected ErrOkfNotBundleRoot, got %v", err)
	}
}

func TestOkfService_RegisterBundle_Idempotent(t *testing.T) {
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	pd := &model.DiskPublicDirectory{ID: 7, FolderID: 100, FixedPath: "/public/kb"}
	pub := newFakeOkfPublicDir(pd)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", ""))

	svc := NewOkfServiceFromRepo(bundles, nodes, newFakeOkfEdgeRepo(nodes), pub, "sqlite")
	first, err := svc.RegisterBundle(context.Background(), 7)
	if err != nil {
		t.Fatalf("first RegisterBundle: %v", err)
	}
	second, err := svc.RegisterBundle(context.Background(), 7)
	if err != nil {
		t.Fatalf("second RegisterBundle: %v", err)
	}
	if first.ID != second.ID {
		t.Errorf("idempotent: first.ID=%d second.ID=%d", first.ID, second.ID)
	}
}

func TestOkfService_WriteMarkdown_MissingType(t *testing.T) {
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	pd := &model.DiskPublicDirectory{ID: 7, FolderID: 100, FixedPath: "/public/kb"}
	pub := newFakeOkfPublicDir(pd)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", ""))
	svc := NewOkfServiceFromRepo(bundles, nodes, newFakeOkfEdgeRepo(nodes), pub, "sqlite")

	// Pre-register bundle so we hit the writer path directly.
	if _, err := svc.RegisterBundle(context.Background(), 7); err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}

	// Missing type → 400-equivalent error.
	_, err := svc.WriteMarkdown(context.Background(), WriteMarkdownRequest{
		PublicDirectoryID: 7,
		RelPath:           "note.md",
		Content:           []byte("---\ntitle: missing type\n---\nbody"),
	})
	if !errors.Is(err, ErrOkfMissingType) {
		t.Fatalf("expected ErrOkfMissingType, got %v", err)
	}
}

func TestOkfService_WriteMarkdown_Success(t *testing.T) {
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	pd := &model.DiskPublicDirectory{ID: 7, FolderID: 100, FixedPath: "/public/kb"}
	pub := newFakeOkfPublicDir(pd)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", ""))
	svc := NewOkfServiceFromRepo(bundles, nodes, newFakeOkfEdgeRepo(nodes), pub, "sqlite")

	if _, err := svc.RegisterBundle(context.Background(), 7); err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}

	node, err := svc.WriteMarkdown(context.Background(), WriteMarkdownRequest{
		PublicDirectoryID: 7,
		RelPath:           "concepts/gemma.md",
		Content:           mustTypedMD("concept", "Gemma"),
	})
	if err != nil {
		t.Fatalf("WriteMarkdown: %v", err)
	}
	if node.Type != "concept" {
		t.Errorf("type = %q, want concept", node.Type)
	}
	if node.Title != "Gemma" {
		t.Errorf("title = %q, want Gemma", node.Title)
	}
	if node.RelPath != "concepts/gemma.md" {
		t.Errorf("relPath = %q, want concepts/gemma.md", node.RelPath)
	}
	if node.ContentHash == "" {
		t.Error("contentHash should be populated")
	}

	// Node must be retrievable.
	got, err := nodes.GetByBundleAndRelPath(1, "concepts/gemma.md")
	if err != nil {
		t.Fatalf("GetByBundleAndRelPath: %v", err)
	}
	if got.ID != node.ID {
		t.Errorf("stored node ID mismatch: got %d want %d", got.ID, node.ID)
	}
}

func TestOkfService_WriteMarkdown_NestedPath(t *testing.T) {
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	pd := &model.DiskPublicDirectory{ID: 7, FolderID: 100, FixedPath: "/public/kb"}
	pub := newFakeOkfPublicDir(pd)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", ""))
	svc := NewOkfServiceFromRepo(bundles, nodes, newFakeOkfEdgeRepo(nodes), pub, "sqlite")

	if _, err := svc.RegisterBundle(context.Background(), 7); err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}

	// Multi-segment path; the service must normalize backslashes and dots.
	rel := "./concepts/llm/gemma.md"
	node, err := svc.WriteMarkdown(context.Background(), WriteMarkdownRequest{
		PublicDirectoryID: 7,
		RelPath:           rel,
		Content:           mustTypedMD("concept", "Nested"),
	})
	if err != nil {
		t.Fatalf("WriteMarkdown nested: %v", err)
	}
	if node.RelPath != "concepts/llm/gemma.md" {
		t.Errorf("relPath = %q, want concepts/llm/gemma.md", node.RelPath)
	}
}

func TestOkfService_WriteMarkdown_AutoRegistersBundle(t *testing.T) {
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	pd := &model.DiskPublicDirectory{ID: 7, FolderID: 100, FixedPath: "/public/kb"}
	pub := newFakeOkfPublicDir(pd)
	// No index.md pre-seeded: the writer will be the first to put one down.
	svc := NewOkfServiceFromRepo(bundles, nodes, newFakeOkfEdgeRepo(nodes), pub, "sqlite")

	// Writing index.md as the first file must auto-register the bundle.
	_, err := svc.WriteMarkdown(context.Background(), WriteMarkdownRequest{
		PublicDirectoryID: 7,
		RelPath:           "index.md",
		Content:           mustIndexMD(t, "0.1", "From Writer"),
	})
	if err != nil {
		t.Fatalf("WriteMarkdown index.md: %v", err)
	}
	b, err := bundles.GetByPublicDirectoryID(7)
	if err != nil {
		t.Fatalf("bundle not auto-registered: %v", err)
	}
	if b.Title != "From Writer" {
		t.Errorf("title = %q, want From Writer", b.Title)
	}
}

func TestOkfService_WriteMarkdown_LogMdmustNotHaveFrontmatter(t *testing.T) {
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	pd := &model.DiskPublicDirectory{ID: 7, FolderID: 100, FixedPath: "/public/kb"}
	pub := newFakeOkfPublicDir(pd)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", ""))
	svc := NewOkfServiceFromRepo(bundles, nodes, newFakeOkfEdgeRepo(nodes), pub, "sqlite")

	if _, err := svc.RegisterBundle(context.Background(), 7); err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}

	// log.md with a Type field is rejected.
	_, err := svc.WriteMarkdown(context.Background(), WriteMarkdownRequest{
		PublicDirectoryID: 7,
		RelPath:           "log.md",
		Content:           []byte("---\ntype: changelog\n---\nentry"),
	})
	if !errors.Is(err, ErrOkfReservedName) {
		t.Fatalf("expected ErrOkfReservedName, got %v", err)
	}

	// log.md plain body is allowed.
	_, err = svc.WriteMarkdown(context.Background(), WriteMarkdownRequest{
		PublicDirectoryID: 7,
		RelPath:           "log.md",
		Content:           []byte("plain entry"),
	})
	if err != nil {
		t.Fatalf("plain log.md should be accepted, got %v", err)
	}
}

func TestOkfService_WriteMarkdown_ExtraFrontmatterPreserved(t *testing.T) {
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	pd := &model.DiskPublicDirectory{ID: 7, FolderID: 100, FixedPath: "/public/kb"}
	pub := newFakeOkfPublicDir(pd)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", ""))
	svc := NewOkfServiceFromRepo(bundles, nodes, newFakeOkfEdgeRepo(nodes), pub, "sqlite")

	if _, err := svc.RegisterBundle(context.Background(), 7); err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}

	body := []byte("---\ntype: concept\nauthor: alice\nreviewers:\n  - bob\n---\nbody")
	node, err := svc.WriteMarkdown(context.Background(), WriteMarkdownRequest{
		PublicDirectoryID: 7,
		RelPath:           "c.md",
		Content:           body,
	})
	if err != nil {
		t.Fatalf("WriteMarkdown: %v", err)
	}
	extra, _ := node.GetExtra()
	if extra["author"] != "alice" {
		t.Errorf("extra[author] = %v, want alice", extra["author"])
	}
	if _, ok := extra["reviewers"]; !ok {
		t.Errorf("extra[reviewers] missing; got %v", extra)
	}
}

func TestOkfService_ListBundles(t *testing.T) {
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	pd1 := &model.DiskPublicDirectory{ID: 1, FolderID: 10, FixedPath: "/public/a"}
	pub := newFakeOkfPublicDir(pd1)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", ""))
	svc := NewOkfServiceFromRepo(bundles, nodes, newFakeOkfEdgeRepo(nodes), pub, "sqlite")

	b1 := &model.OkfBundle{PublicDirectoryID: 1, OkfVersion: "0.1", Title: "A"}
	b2 := &model.OkfBundle{PublicDirectoryID: 2, OkfVersion: "0.1", Title: "B"}
	_ = bundles.Create(b1)
	_ = bundles.Create(b2)

	got, err := svc.ListBundles("", "")
	if err != nil {
		t.Fatalf("ListBundles: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
}

func TestOkfService_GetBundle_NotFound(t *testing.T) {
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	pub := newFakeOkfPublicDir(&model.DiskPublicDirectory{ID: 1, FolderID: 10, FixedPath: "/public/a"})
	svc := NewOkfServiceFromRepo(bundles, nodes, newFakeOkfEdgeRepo(nodes), pub, "sqlite")

	_, err := svc.GetBundle(999, "", "")
	if !errors.Is(err, ErrOkfBundleNotFound) {
		t.Errorf("expected ErrOkfBundleNotFound, got %v", err)
	}
}

func TestOkfService_ListNodes_Filters(t *testing.T) {
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	pd := &model.DiskPublicDirectory{ID: 7, FolderID: 100, FixedPath: "/public/kb"}
	pub := newFakeOkfPublicDir(pd)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", ""))
	svc := NewOkfServiceFromRepo(bundles, nodes, newFakeOkfEdgeRepo(nodes), pub, "sqlite")

	bundle, err := svc.RegisterBundle(context.Background(), 7)
	if err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}

	// Seed three nodes.
	seeds := []struct {
		rel  string
		typ  string
		tags []string
	}{
		{"a.md", "concept", []string{"llm"}},
		{"b.md", "guide", []string{"llm", "gemma"}},
		{"c.md", "concept", []string{"search"}},
	}
	for _, s := range seeds {
		n := &model.OkfNode{BundleID: bundle.ID, RelPath: s.rel, Type: s.typ, Title: s.rel}
		_ = n.SetTags(s.tags)
		if err := nodes.Upsert(nil, n); err != nil {
			t.Fatalf("Upsert %s: %v", s.rel, err)
		}
	}

	// Filter by type.
	got, _, err := svc.ListNodesByType(bundle.ID, "concept", "", "", "", 0, 0)
	if err != nil {
		t.Fatalf("ListNodesByType concept: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("type=concept: got %d, want 2", len(got))
	}

	// Filter by tag.
	got, _, err = svc.ListNodesByType(bundle.ID, "", "llm", "", "", 0, 0)
	if err != nil {
		t.Fatalf("ListNodesByType llm: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("tag=llm: got %d, want 2", len(got))
	}

	// Filter by both.
	got, _, err = svc.ListNodesByType(bundle.ID, "concept", "llm", "", "", 0, 0)
	if err != nil {
		t.Fatalf("ListNodesByType concept+llm: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("type=concept,tag=llm: got %d, want 1", len(got))
	}
}

func TestOkfService_ListNodesByType_Pagination(t *testing.T) {
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	pd := &model.DiskPublicDirectory{ID: 7, FolderID: 100, FixedPath: "/public/kb"}
	pub := newFakeOkfPublicDir(pd)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", ""))
	svc := NewOkfServiceFromRepo(bundles, nodes, newFakeOkfEdgeRepo(nodes), pub, "sqlite")

	bundle, err := svc.RegisterBundle(context.Background(), 7)
	if err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}
	// Seed 5 nodes (the fake orders by ID, so the page walk is deterministic).
	for _, rel := range []string{"a.md", "b.md", "c.md", "d.md", "e.md"} {
		if err := nodes.Upsert(nil, &model.OkfNode{BundleID: bundle.ID, RelPath: rel, Type: "concept"}); err != nil {
			t.Fatalf("Upsert %s: %v", rel, err)
		}
	}

	// RegisterBundle materializes index.md as a node too, so the bundle has
	// 1 (index) + 5 seeded = 6 nodes. The fake orders by ID, deterministic.
	const limit = 4
	// Page 1 (offset 0): 4 nodes, nextCursor = 4.
	page1, next, err := svc.ListNodesByType(bundle.ID, "", "", "", "", limit, 0)
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if len(page1) != 4 || next != 4 {
		t.Fatalf("page1: len=%d nextCursor=%d, want 4 / 4", len(page1), next)
	}
	// Page 2 (offset 4): the remaining 2, nextCursor = 0 (last page).
	page2, next, err := svc.ListNodesByType(bundle.ID, "", "", "", "", limit, limit)
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if len(page2) != 2 || next != 0 {
		t.Fatalf("page2: len=%d nextCursor=%d, want 2 / 0", len(page2), next)
	}
	// Unpaginated (limit 0) returns the full set of 6.
	all, _, err := svc.ListNodesByType(bundle.ID, "", "", "", "", 0, 0)
	if err != nil {
		t.Fatalf("unpaginated: %v", err)
	}
	if len(all) != 6 {
		t.Errorf("unpaginated len=%d, want 6", len(all))
	}
}

func TestOkfService_AggregateByType(t *testing.T) {
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	pd := &model.DiskPublicDirectory{ID: 7, FolderID: 100, FixedPath: "/public/kb"}
	pub := newFakeOkfPublicDir(pd)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", ""))
	svc := NewOkfServiceFromRepo(bundles, nodes, newFakeOkfEdgeRepo(nodes), pub, "sqlite")

	bundle, err := svc.RegisterBundle(context.Background(), 7)
	if err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}

	for _, rel := range []string{"a.md", "b.md"} {
		_ = nodes.Upsert(nil, &model.OkfNode{BundleID: bundle.ID, RelPath: rel, Type: "concept"})
	}
	_ = nodes.Upsert(nil, &model.OkfNode{BundleID: bundle.ID, RelPath: "c.md", Type: "guide"})

	got, err := svc.AggregateByType(bundle.ID, "", "")
	if err != nil {
		t.Fatalf("AggregateByType: %v", err)
	}
	want := map[string]uint32{"concept": 2, "guide": 1}
	gotMap := map[string]uint32{}
	for _, tc := range got {
		gotMap[tc.Type] = tc.Count
	}
	for k, v := range want {
		if gotMap[k] != v {
			t.Errorf("count[%s] = %d, want %d", k, gotMap[k], v)
		}
	}
}

func TestOkfService_UnregisterBundle(t *testing.T) {
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	pd := &model.DiskPublicDirectory{ID: 7, FolderID: 100, FixedPath: "/public/kb"}
	pub := newFakeOkfPublicDir(pd)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", ""))
	svc := NewOkfServiceFromRepo(bundles, nodes, newFakeOkfEdgeRepo(nodes), pub, "sqlite")

	bundle, err := svc.RegisterBundle(context.Background(), 7)
	if err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}
	if err := svc.UnregisterBundle(bundle.ID); err != nil {
		t.Fatalf("UnregisterBundle: %v", err)
	}
	if _, err := bundles.GetByID(bundle.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("expected record not found after unregister, got %v", err)
	}
}

func TestOkfService_UnregisterBundle_NotFound(t *testing.T) {
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	pub := newFakeOkfPublicDir(&model.DiskPublicDirectory{ID: 1, FolderID: 10, FixedPath: "/public/a"})
	svc := NewOkfServiceFromRepo(bundles, nodes, newFakeOkfEdgeRepo(nodes), pub, "sqlite")

	if err := svc.UnregisterBundle(999); !errors.Is(err, ErrOkfBundleNotFound) {
		t.Errorf("expected ErrOkfBundleNotFound, got %v", err)
	}
}

// stubVisibility is a hand-rolled okfVisibility for the ACL tests. It returns
// a fixed map of which public directories the caller can see.
type stubVisibility struct {
	visible map[uint64]bool
	err     error
}

func (s stubVisibility) VisiblePublicDirIDs(_, _ string) (map[uint64]bool, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.visible, nil
}

// newOkfSvcWithVisibility wires an OkfService against the fake repos and a
// stub visibility checker. The caller controls which public directory IDs the
// caller is allowed to see.
func newOkfSvcWithVisibility(t *testing.T, visible map[uint64]bool) *OkfService {
	t.Helper()
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	pub := newFakeOkfPublicDir(&model.DiskPublicDirectory{ID: 1, FolderID: 10, FixedPath: "/public/a"})
	svc := NewOkfServiceFromRepo(bundles, nodes, newFakeOkfEdgeRepo(nodes), pub, "sqlite")
	svc.SetVisibility(stubVisibility{visible: visible})
	return svc
}

func TestOkfService_ReaderACL_RejectsCrossUser(t *testing.T) {
	// Bundle 1 → public directory 7. Caller "alice" has NO grant on PD 7.
	svc := newOkfSvcWithVisibility(t, map[uint64]bool{}) // nothing visible
	// Seed bundle pointing at PD 7 directly (bypass RegisterBundle so we don't
	// have to materialize index.md for this ACL test).
	bundle := &model.OkfBundle{ID: 1, PublicDirectoryID: 7, OkfVersion: "0.1"}
	if err := svc.bundles.Create(bundle); err != nil {
		t.Fatalf("seed bundle: %v", err)
	}
	// Seed one node so the not-found path is not the failure mode.
	_ = svc.nodes.Upsert(nil, &model.OkfNode{BundleID: 1, RelPath: "a.md", Type: "concept"})

	// GetBundle rejects.
	if _, err := svc.GetBundle(1, "alice", "eng"); !errors.Is(err, ErrOkfForbidden) {
		t.Errorf("GetBundle: expected ErrOkfForbidden, got %v", err)
	}
	// ListNodesByType rejects.
	if _, _, err := svc.ListNodesByType(1, "", "", "alice", "eng", 0, 0); !errors.Is(err, ErrOkfForbidden) {
		t.Errorf("ListNodesByType: expected ErrOkfForbidden, got %v", err)
	}
	// AggregateByType rejects.
	if _, err := svc.AggregateByType(1, "alice", "eng"); !errors.Is(err, ErrOkfForbidden) {
		t.Errorf("AggregateByType: expected ErrOkfForbidden, got %v", err)
	}
	// ListBundles filters the bundle out.
	got, err := svc.ListBundles("alice", "eng")
	if err != nil {
		t.Fatalf("ListBundles: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ListBundles cross-user: got %d bundles, want 0", len(got))
	}
}

func TestOkfService_ReaderACL_AllowsGrantedUser(t *testing.T) {
	// Bundle 1 → PD 7. Caller "alice" has a grant on PD 7.
	svc := newOkfSvcWithVisibility(t, map[uint64]bool{7: true})
	bundle := &model.OkfBundle{ID: 1, PublicDirectoryID: 7, OkfVersion: "0.1"}
	if err := svc.bundles.Create(bundle); err != nil {
		t.Fatalf("seed bundle: %v", err)
	}
	_ = svc.nodes.Upsert(nil, &model.OkfNode{BundleID: 1, RelPath: "a.md", Type: "concept"})

	if _, err := svc.GetBundle(1, "alice", "eng"); err != nil {
		t.Errorf("GetBundle granted: %v", err)
	}
	if _, _, err := svc.ListNodesByType(1, "", "", "alice", "eng", 0, 0); err != nil {
		t.Errorf("ListNodesByType granted: %v", err)
	}
	if _, err := svc.AggregateByType(1, "alice", "eng"); err != nil {
		t.Errorf("AggregateByType granted: %v", err)
	}
	got, err := svc.ListBundles("alice", "eng")
	if err != nil {
		t.Fatalf("ListBundles: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("ListBundles granted: got %d bundles, want 1", len(got))
	}
}

func TestOkfService_ReaderACL_VisibilityErrorPropagates(t *testing.T) {
	// When the visibility check itself errors (e.g. grant repo down), the
	// reader must surface the wrapped error rather than accidentally allowing
	// or hiding data via the not-found branch.
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	pub := newFakeOkfPublicDir(&model.DiskPublicDirectory{ID: 1, FolderID: 10, FixedPath: "/public/a"})
	svc := NewOkfServiceFromRepo(bundles, nodes, newFakeOkfEdgeRepo(nodes), pub, "sqlite")
	svc.SetVisibility(stubVisibility{err: errors.New("grant repo down")})
	bundle := &model.OkfBundle{ID: 1, PublicDirectoryID: 7, OkfVersion: "0.1"}
	_ = bundles.Create(bundle)

	if _, err := svc.GetBundle(1, "alice", "eng"); err == nil {
		t.Error("GetBundle: expected visibility error, got nil")
	}
}

func TestOkfService_ReaderACL_APIAKeySystemUserSeesAll(t *testing.T) {
	// The HybridAuth middleware sets userId=__system_public__ + department from
	// the API key. ListVisible treats __system_public__ as a system caller and
	// returns every active public directory, so the OKF reader ACL should let
	// API-keyed callers see every bundle. We simulate that by setting visible
	// to the full set the production pdVisibility would produce.
	svc := newOkfSvcWithVisibility(t, map[uint64]bool{7: true, 8: true})
	_ = svc.bundles.Create(&model.OkfBundle{ID: 1, PublicDirectoryID: 7, OkfVersion: "0.1"})
	_ = svc.bundles.Create(&model.OkfBundle{ID: 2, PublicDirectoryID: 8, OkfVersion: "0.1"})

	got, err := svc.ListBundles("__system_public__", "")
	if err != nil {
		t.Fatalf("ListBundles: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("ListBundles system user: got %d, want 2 (all bundles)", len(got))
	}
}

func TestOkfService_NormalizeRelPath(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"index.md", "index.md"},
		{"./index.md", "index.md"},
		{"/a/b.md", "a/b.md"},
		{"a/b/../c.md", "a/c.md"},
		{"a\\b.md", "a/b.md"},
		{"", ""},
		{"../escape.md", "escape.md"},
		{"a/../../x.md", "x.md"},
	}
	for _, tc := range cases {
		got := normalizeRelPath(tc.in)
		if got != tc.want {
			t.Errorf("normalizeRelPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestOkfService_WriteMarkdown_RejectsNonMarkdown(t *testing.T) {
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	pd := &model.DiskPublicDirectory{ID: 7, FolderID: 100, FixedPath: "/public/kb"}
	pub := newFakeOkfPublicDir(pd)
	svc := NewOkfServiceFromRepo(bundles, nodes, newFakeOkfEdgeRepo(nodes), pub, "sqlite")

	_, err := svc.WriteMarkdown(context.Background(), WriteMarkdownRequest{
		PublicDirectoryID: 7,
		RelPath:           "notes.txt",
		Content:           []byte("x"),
	})
	if err == nil {
		t.Error("expected error for non-markdown relPath")
	}
}

func TestOkfService_WriteMarkdown_MissingPublicDirectory(t *testing.T) {
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	pub := newFakeOkfPublicDir(nil) // nil ⇒ GetPublicDirectory errors
	svc := NewOkfServiceFromRepo(bundles, nodes, newFakeOkfEdgeRepo(nodes), pub, "sqlite")

	_, err := svc.WriteMarkdown(context.Background(), WriteMarkdownRequest{
		PublicDirectoryID: 999,
		RelPath:           "x.md",
		Content:           mustTypedMD("t", ""),
	})
	if err == nil {
		t.Error("expected error for missing public directory")
	}
}

func TestOkfService_ValidateFrontmatterForPath(t *testing.T) {
	svc := &OkfService{}
	cases := []struct {
		name    string
		rel     string
		typeF   string
		okfVer  string
		wantErr bool
	}{
		{"plain file with type", "x.md", "concept", "", false},
		{"plain file no type", "x.md", "", "", true},
		{"index with okf_version", "index.md", "bundle", "0.1", false},
		{"index missing okf_version", "index.md", "bundle", "", true},
		{"index missing type", "index.md", "", "0.1", true},
		{"log with no frontmatter", "log.md", "", "", false},
		{"log with type", "log.md", "changelog", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			front := okf.Frontmatter{Type: tc.typeF, OkfVersion: tc.okfVer}
			err := svc.validateFrontmatterForPath(tc.rel, front)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

func TestOkfService_GetBundle_NotFoundBranch(t *testing.T) {
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	pub := newFakeOkfPublicDir(&model.DiskPublicDirectory{ID: 1, FolderID: 10, FixedPath: "/public/a"})
	svc := NewOkfServiceFromRepo(bundles, nodes, newFakeOkfEdgeRepo(nodes), pub, "sqlite")

	if _, err := svc.AggregateByType(999, "", ""); !errors.Is(err, ErrOkfBundleNotFound) {
		t.Errorf("AggregateByType: expected ErrOkfBundleNotFound, got %v", err)
	}
	if _, _, err := svc.ListNodesByType(999, "", "", "", "", 0, 0); !errors.Is(err, ErrOkfBundleNotFound) {
		t.Errorf("ListNodesByType: expected ErrOkfBundleNotFound, got %v", err)
	}
	if _, err := svc.RefreshBundle(context.Background(), 999); !errors.Is(err, ErrOkfBundleNotFound) {
		t.Errorf("RefreshBundle: expected ErrOkfBundleNotFound, got %v", err)
	}
}

func TestOkfService_RefreshBundle_StubPDNoOps(t *testing.T) {
	// When pdSvc is a stub (not *PublicDirectoryService), walkAndMaterialize
	// returns nil immediately and RefreshBundle succeeds with zero nodes.
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	pd := &model.DiskPublicDirectory{ID: 7, FolderID: 100, FixedPath: "/public/kb"}
	pub := newFakeOkfPublicDir(pd)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", ""))
	svc := NewOkfServiceFromRepo(bundles, nodes, newFakeOkfEdgeRepo(nodes), pub, "sqlite")

	bundle, err := svc.RegisterBundle(context.Background(), 7)
	if err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}
	// First refresh clears nodes (the index.md materialization) and the stub
	// walk cannot re-materialize, so NodeCount drops to 0.
	refreshed, err := svc.RefreshBundle(context.Background(), bundle.ID)
	if err != nil {
		t.Fatalf("RefreshBundle: %v", err)
	}
	if refreshed.NodeCount != 0 {
		t.Errorf("NodeCount = %d, want 0 (stub walk is a no-op)", refreshed.NodeCount)
	}
}

func TestOkfService_WriteMarkdown_RapidRewrite(t *testing.T) {
	// Re-writing the same relPath must upsert rather than duplicate the node.
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	pd := &model.DiskPublicDirectory{ID: 7, FolderID: 100, FixedPath: "/public/kb"}
	pub := newFakeOkfPublicDir(pd)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", ""))
	svc := NewOkfServiceFromRepo(bundles, nodes, newFakeOkfEdgeRepo(nodes), pub, "sqlite")

	if _, err := svc.RegisterBundle(context.Background(), 7); err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}
	first, err := svc.WriteMarkdown(context.Background(), WriteMarkdownRequest{
		PublicDirectoryID: 7,
		RelPath:           "note.md",
		Content:           mustTypedMD("concept", "v1"),
	})
	if err != nil {
		t.Fatalf("first WriteMarkdown: %v", err)
	}
	second, err := svc.WriteMarkdown(context.Background(), WriteMarkdownRequest{
		PublicDirectoryID: 7,
		RelPath:           "note.md",
		Content:           mustTypedMD("concept", "v2"),
	})
	if err != nil {
		t.Fatalf("second WriteMarkdown: %v", err)
	}
	if first.ID != second.ID {
		t.Errorf("upsert should preserve node ID: first=%d second=%d", first.ID, second.ID)
	}
	if second.Title != "v2" {
		t.Errorf("title = %q, want v2", second.Title)
	}
	// Bundle still has index.md + note.md = 2 nodes; second write must upsert
	// in place rather than adding a third.
	count, _ := nodes.CountByBundle(nil, first.BundleID)
	if count != 2 {
		t.Errorf("node count = %d, want 2 (upsert should not duplicate)", count)
	}
}

func TestOkfService_DefaultStr(t *testing.T) {
	if got := defaultStr("x", "y"); got != "x" {
		t.Errorf("defaultStr(x,y) = %q, want x", got)
	}
	if got := defaultStr("  ", "y"); got != "y" {
		t.Errorf("defaultStr(blank,y) = %q, want y", got)
	}
}

func TestOkfService_IsNotFound(t *testing.T) {
	if isNotFound(nil) {
		t.Error("nil should not be not-found")
	}
	if !isNotFound(ErrOkfBundleNotFound) {
		t.Error("ErrOkfBundleNotFound should be not-found")
	}
	if !isNotFound(gorm.ErrRecordNotFound) {
		t.Error("gorm.ErrRecordNotFound should be not-found")
	}
	// Wrapped sentinel must still be detected (public directory service wraps
	// its repo errors with fmt.Errorf).
	wrapped := fmt.Errorf("lookup failed: %w", gorm.ErrRecordNotFound)
	if !isNotFound(wrapped) {
		t.Error("wrapped gorm.ErrRecordNotFound should be not-found")
	}
	// Unrelated errors must not be treated as not-found.
	if isNotFound(errors.New("disk full")) {
		t.Error("unrelated error should not be not-found")
	}
	if isNotFound(errors.New("record not found")) {
		t.Error("literal string match must not be treated as not-found (use errors.Is)")
	}
}

func TestOkfService_RegisterBundle_ReadContentFailure(t *testing.T) {
	// index.md exists but its content read fails; RegisterBundle surfaces that
	// as a wrapped error rather than ErrOkfNotBundleRoot.
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	pd := &model.DiskPublicDirectory{ID: 7, FolderID: 100, FixedPath: "/public/kb"}
	pub := newFakeOkfPublicDir(pd)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", ""))
	pub.readFailureOn = "index.md"
	svc := NewOkfServiceFromRepo(bundles, nodes, newFakeOkfEdgeRepo(nodes), pub, "sqlite")

	if _, err := svc.RegisterBundle(context.Background(), 7); err == nil {
		t.Error("expected error when reading index.md fails")
	}
}

func TestOkfService_WriteMarkdown_UploadFailure(t *testing.T) {
	// Upload fails: WriteMarkdown must surface the error and not auto-register.
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	pd := &model.DiskPublicDirectory{ID: 7, FolderID: 100, FixedPath: "/public/kb"}
	pub := newFakeOkfPublicDir(pd)
	pub.uploadErr = errors.New("oss unavailable")
	svc := NewOkfServiceFromRepo(bundles, nodes, newFakeOkfEdgeRepo(nodes), pub, "sqlite")

	if _, err := svc.WriteMarkdown(context.Background(), WriteMarkdownRequest{
		PublicDirectoryID: 7,
		RelPath:           "index.md",
		Content:           mustIndexMD(t, "0.1", "x"),
	}); err == nil {
		t.Error("expected upload error to surface")
	}
}

func TestOkfService_WriteMarkdown_BundleLookupNonNotFound(t *testing.T) {
	// Drive WriteMarkdown through the generic bundle-lookup error branch by
	// using a bundle repo whose GetByPublicDirectoryID fails with a non-not-found
	// error after a successful upload.
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	pd := &model.DiskPublicDirectory{ID: 7, FolderID: 100, FixedPath: "/public/kb"}
	pub := newFakeOkfPublicDir(pd)
	svc := NewOkfServiceFromRepo(&flakyBundleRepo{base: bundles, failOnPD: 7}, nodes, newFakeOkfEdgeRepo(nodes), pub, "sqlite")

	if _, err := svc.WriteMarkdown(context.Background(), WriteMarkdownRequest{
		PublicDirectoryID: 7,
		RelPath:           "note.md",
		Content:           mustTypedMD("concept", "x"),
	}); err == nil {
		t.Error("expected lookup error to surface")
	}
}

// flakyBundleRepo wraps fakeOkfBundleRepo and forces a non-not-found error on
// GetByPublicDirectoryID for the given PD. Used to exercise the generic-error
// branch in WriteMarkdown.
type flakyBundleRepo struct {
	base     *fakeOkfBundleRepo
	failOnPD uint64
}

func (r *flakyBundleRepo) Create(b *model.OkfBundle) error             { return r.base.Create(b) }
func (r *flakyBundleRepo) GetByID(id uint64) (*model.OkfBundle, error) { return r.base.GetByID(id) }
func (r *flakyBundleRepo) List(s string, l, o int) ([]model.OkfBundle, error) {
	return r.base.List(s, l, o)
}
func (r *flakyBundleRepo) Update(b *model.OkfBundle) error { return r.base.Update(b) }
func (r *flakyBundleRepo) Delete(id uint64) error          { return r.base.Delete(id) }
func (r *flakyBundleRepo) GetByPublicDirectoryID(pdID uint64) (*model.OkfBundle, error) {
	if pdID == r.failOnPD {
		return nil, errors.New("connection refused")
	}
	return r.base.GetByPublicDirectoryID(pdID)
}

// ── In-memory storage for RefreshBundle integration tests ──

// memStorage is a minimal storage.Storage backed by a map. It is enough to
// drive the real PublicDirectoryService through UploadFileAt + ReadFileContent
// so the RefreshBundle walk path is exercised end-to-end without OSS.
type memStorage struct {
	objects map[string][]byte
}

func newMemStorage() *memStorage {
	return &memStorage{objects: map[string][]byte{}}
}

func (m *memStorage) Upload(_ context.Context, key string, reader io.Reader, _ int64, _ string) error {
	b, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	m.objects[key] = b
	return nil
}

func (m *memStorage) Download(_ context.Context, key string) (io.ReadCloser, error) {
	b, ok := m.objects[key]
	if !ok {
		return nil, errors.New("object not found")
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func (m *memStorage) Delete(_ context.Context, key string) error {
	delete(m.objects, key)
	return nil
}

func (m *memStorage) Copy(_ context.Context, srcKey, dstKey string) error {
	b, ok := m.objects[srcKey]
	if !ok {
		return errors.New("source not found")
	}
	cp := make([]byte, len(b))
	copy(cp, b)
	m.objects[dstKey] = cp
	return nil
}

func (m *memStorage) PresignedGetURL(_ context.Context, _ string, _ time.Duration) (string, error) {
	return "http://localhost/mock", nil
}

func (m *memStorage) PresignedDownloadURL(_ context.Context, _ string, _ time.Duration, _ string) (string, error) {
	return "http://localhost/mock", nil
}

func (m *memStorage) EnsureBucket(_ context.Context) error { return nil }

var _ storage.Storage = (*memStorage)(nil)

// fullFileRepo is a fileDataRepo backed by in-memory maps. It supports every
// method PublicDirectoryService.UploadFileAt / ReadFileContent needs.
type fullFileRepo struct {
	files  map[uint64]*model.DiskFile
	byKey  map[string]uint64 // folderID:name -> file ID
	nextID uint64
}

func newFullFileRepo() *fullFileRepo {
	return &fullFileRepo{files: map[uint64]*model.DiskFile{}, byKey: map[string]uint64{}, nextID: 1}
}

func (r *fullFileRepo) Create(f *model.DiskFile) error {
	f.ID = r.nextID
	r.nextID++
	r.files[f.ID] = f
	r.byKey[fullFileKey(f.FolderID, f.FileName)] = f.ID
	return nil
}

func (r *fullFileRepo) GetByID(id uint64) (*model.DiskFile, error) {
	if f, ok := r.files[id]; ok {
		return f, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fullFileRepo) GetByFolderAndName(folderID uint64, name string) (*model.DiskFile, error) {
	if id, ok := r.byKey[fullFileKey(folderID, name)]; ok {
		return r.files[id], nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *fullFileRepo) ListByFolder(userID string, folderID uint64) ([]model.DiskFile, error) {
	var out []model.DiskFile
	for _, f := range r.files {
		if f.UserID == userID && f.FolderID == folderID && !f.IsDeleted {
			out = append(out, *f)
		}
	}
	return out, nil
}

func (r *fullFileRepo) UpdateOSSKey(id uint64, ossKey string) error {
	if f, ok := r.files[id]; ok {
		f.OSSKey = ossKey
	}
	return nil
}

func (r *fullFileRepo) UpdateVersion(id uint64, size int64, version int, ossKey, md5 string) error {
	if f, ok := r.files[id]; ok {
		f.FileSize = size
		f.Version = version
		f.OSSKey = ossKey
		f.MD5 = md5
	}
	return nil
}

func (r *fullFileRepo) SoftDelete(id uint64) error {
	if f, ok := r.files[id]; ok {
		f.IsDeleted = true
	}
	return nil
}

func fullFileKey(folderID uint64, name string) string {
	return uintToStr(folderID) + ":" + name
}

// newOkfServiceWithRealPD builds an OkfService wired to a fully-functional
// PublicDirectoryService (in-memory storage + mock repos). Used by the
// RefreshBundle integration test to walk the folder tree.
func newOkfServiceWithRealPD(t *testing.T) (*OkfService, *PublicDirectoryService, *model.DiskPublicDirectory) {
	t.Helper()
	pdRepo := newMockPublicDirRepo()
	folderRepo := newMockFolderCreator()
	fileRepo := newFullFileRepo()
	store := newMemStorage()

	pdSvc := &PublicDirectoryService{
		pdRepo:     pdRepo,
		folderRepo: folderRepo,
		fileRepo:   fileRepo,
		storage:    store,
	}
	// Create the public directory + its root folder so UploadFileAt works.
	pd, err := pdSvc.CreatePublicDirectory("kb", ScopeGlobal, "", "admin")
	if err != nil {
		t.Fatalf("CreatePublicDirectory: %v", err)
	}
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	svc := &OkfService{bundles: bundles, nodes: nodes, pdSvc: pdSvc, dbDriver: "sqlite"}
	return svc, pdSvc, pd
}

func TestOkfService_RefreshBundle_WalksTree(t *testing.T) {
	svc, pdSvc, pd := newOkfServiceWithRealPD(t)

	// Seed index.md + two nested typed files via the real writer path so the
	// folder tree and OSS objects exist.
	ctx := context.Background()
	files := []struct {
		rel     string
		content []byte
	}{
		{"index.md", mustIndexMD(t, "0.1", "Bundle")},
		{"concepts/gemma.md", mustTypedMD("concept", "Gemma")},
		{"guides/quickstart.md", mustTypedMD("guide", "Quickstart")},
	}
	for _, f := range files {
		if _, err := pdSvc.UploadFileAt(ctx, pd.ID, f.rel, "text/markdown", f.content); err != nil {
			t.Fatalf("UploadFileAt %s: %v", f.rel, err)
		}
	}

	bundle, err := svc.RegisterBundle(ctx, pd.ID)
	if err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}
	// RegisterBundle materializes index.md only; the other two files are
	// outside the writer path and must be picked up by RefreshBundle.
	if bundle.NodeCount != 1 {
		t.Errorf("after register: nodeCount = %d, want 1", bundle.NodeCount)
	}

	refreshed, err := svc.RefreshBundle(ctx, bundle.ID)
	if err != nil {
		t.Fatalf("RefreshBundle: %v", err)
	}
	if refreshed.NodeCount != 3 {
		t.Errorf("after refresh: nodeCount = %d, want 3 (index + 2 typed)", refreshed.NodeCount)
	}

	// The walk must descend both nested folders.
	agg, err := svc.AggregateByType(bundle.ID, "", "")
	if err != nil {
		t.Fatalf("AggregateByType: %v", err)
	}
	gotTypes := map[string]uint32{}
	for _, a := range agg {
		gotTypes[a.Type] = a.Count
	}
	wantTypes := map[string]uint32{"bundle": 1, "concept": 1, "guide": 1}
	for k, v := range wantTypes {
		if gotTypes[k] != v {
			t.Errorf("type[%s] = %d, want %d", k, gotTypes[k], v)
		}
	}
}

func TestOkfService_RefreshBundle_WalkFailsOnMissingPD(t *testing.T) {
	// Walk path whose public directory lookup fails. We register against one
	// PD then delete it so the in-walk GetPublicDirectory errors.
	svc, _, pd := newOkfServiceWithRealPD(t)
	ctx := context.Background()

	if _, err := svc.pdSvc.(*PublicDirectoryService).UploadFileAt(ctx, pd.ID, "index.md", "text/markdown", mustIndexMD(t, "0.1", "x")); err != nil {
		t.Fatalf("UploadFileAt: %v", err)
	}
	bundle, err := svc.RegisterBundle(ctx, pd.ID)
	if err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}
	// Force the walk's GetPublicDirectory to fail by swapping the pdSvc for a
	// stub whose GetPublicDirectory errors but Upload/Read still work via the
	// real one. The simplest way is to delete the directory from the repo.
	realPD := svc.pdSvc.(*PublicDirectoryService)
	_ = realPD.pdRepo.Delete(pd.ID)

	if _, err := svc.RefreshBundle(ctx, bundle.ID); err == nil {
		t.Error("expected RefreshBundle to fail when public directory is missing")
	}
}

// ── P2 integration tests ──

// fakeBundleLock is a test-only BundleLock that records Acquire calls and
// can be configured to reject the second caller. Used by the WriteMarkdown
// lock-integration tests.
type fakeBundleLock struct {
	held      bool
	acquireFn func(bundleID uint64) error
}

func (f *fakeBundleLock) Acquire(_ context.Context, bundleID uint64) (func(), error) {
	if f.acquireFn != nil {
		if err := f.acquireFn(bundleID); err != nil {
			return nil, err
		}
	}
	if f.held {
		return nil, ErrOkfLockHeld
	}
	f.held = true
	released := false
	return func() {
		if released {
			return
		}
		released = true
		f.held = false
	}, nil
}

// TestWriteMarkdown_AcquiresBundleLock verifies the writer takes the bundle
// lock for the duration of a write. We install a fake lock whose Acquire
// fails when called twice; if the writer released the first lock, the second
// write succeeds, otherwise it sees ErrOkfLockHeld.
func TestWriteMarkdown_AcquiresBundleLock(t *testing.T) {
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	pd := &model.DiskPublicDirectory{ID: 7, FolderID: 100, FixedPath: "/public/kb"}
	pub := newFakeOkfPublicDir(pd)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", ""))
	svc := NewOkfServiceFromRepo(bundles, nodes, newFakeOkfEdgeRepo(nodes), pub, "sqlite")
	if _, err := svc.RegisterBundle(context.Background(), 7); err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}

	lock := &fakeBundleLock{}
	svc.SetBundleLock(lock)

	if _, err := svc.WriteMarkdown(context.Background(), WriteMarkdownRequest{
		PublicDirectoryID: 7, RelPath: "a.md",
		Content: mustTypedMD("concept", "A"),
	}); err != nil {
		t.Fatalf("WriteMarkdown: %v", err)
	}
	// After release the lock must be free for the next writer.
	if lock.held {
		t.Errorf("lock still held after WriteMarkdown returned")
	}

	// Force the lock to deny; the writer must surface ErrOkfLockHeld.
	lock.acquireFn = func(_ uint64) error { return ErrOkfLockHeld }
	_, err := svc.WriteMarkdown(context.Background(), WriteMarkdownRequest{
		PublicDirectoryID: 7, RelPath: "b.md",
		Content: mustTypedMD("concept", "B"),
	})
	if !errors.Is(err, ErrOkfLockHeld) {
		t.Errorf("expected ErrOkfLockHeld, got %v", err)
	}
}

// TestWriteMarkdown_AppendsLogEntry verifies a successful write produces a
// log.md row tagged "create" for a fresh file and "update" for a re-write.
func TestWriteMarkdown_AppendsLogEntry(t *testing.T) {
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	pd := &model.DiskPublicDirectory{ID: 7, FolderID: 100, FixedPath: "/public/kb"}
	pub := newFakeOkfPublicDir(pd)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", ""))
	svc := NewOkfServiceFromRepo(bundles, nodes, newFakeOkfEdgeRepo(nodes), pub, "sqlite")
	svc.SetAutoIndexUpdate(false) // skip async regen so the test stays focused
	if _, err := svc.RegisterBundle(context.Background(), 7); err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}

	if _, err := svc.WriteMarkdown(context.Background(), WriteMarkdownRequest{
		PublicDirectoryID: 7, RelPath: "a.md",
		Content: mustTypedMD("concept", "A"),
	}); err != nil {
		t.Fatalf("first WriteMarkdown: %v", err)
	}
	if _, err := svc.WriteMarkdown(context.Background(), WriteMarkdownRequest{
		PublicDirectoryID: 7, RelPath: "a.md",
		Content: mustTypedMD("concept", "A v2"),
	}); err != nil {
		t.Fatalf("second WriteMarkdown: %v", err)
	}

	file, err := pub.FindFileByRelPath(7, "log.md")
	if err != nil {
		t.Fatalf("log.md missing: %v", err)
	}
	body, _ := pub.ReadFileContent(context.Background(), file.ID)
	s := string(body)
	// register + create + update rows.
	rows := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(rows) < 3 {
		t.Fatalf("log.md rows = %d, want >= 3", len(rows))
	}
	// Walk rows in order; expect register, create, update.
	want := []string{LogActionRegister, LogActionCreate, LogActionUpdate}
	for i, wantAction := range want {
		fields := strings.Split(rows[i], "\t")
		if len(fields) < 2 {
			t.Errorf("row %d fields = %d, want >= 2", i, len(fields))
			continue
		}
		if fields[1] != wantAction {
			t.Errorf("row %d action = %q, want %q", i, fields[1], wantAction)
		}
	}
}

// TestWriteMarkdown_UpdatesBrokenLinkFlag verifies that a WriteMarkdown call
// whose body contains a broken link flips has_broken_link on the resulting
// node, and that fixing the link on a subsequent write clears the flag.
func TestWriteMarkdown_UpdatesBrokenLinkFlag(t *testing.T) {
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	pd := &model.DiskPublicDirectory{ID: 7, FolderID: 100, FixedPath: "/public/kb"}
	pub := newFakeOkfPublicDir(pd)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", ""))
	svc := NewOkfServiceFromRepo(bundles, nodes, newFakeOkfEdgeRepo(nodes), pub, "sqlite")
	svc.SetAutoIndexUpdate(false)
	if _, err := svc.RegisterBundle(context.Background(), 7); err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}

	// Write with a broken link.
	body1 := []byte("---\ntype: concept\ntitle: A\n---\n[miss](./missing.md)\n")
	node, err := svc.WriteMarkdown(context.Background(), WriteMarkdownRequest{
		PublicDirectoryID: 7, RelPath: "a.md", Content: body1,
	})
	if err != nil {
		t.Fatalf("WriteMarkdown: %v", err)
	}
	if !node.HasBrokenLink {
		t.Errorf("HasBrokenLink after broken write = false, want true")
	}

	// Re-write with no links; the flag should clear.
	body2 := []byte("---\ntype: concept\ntitle: A v2\n---\nplain body\n")
	node2, err := svc.WriteMarkdown(context.Background(), WriteMarkdownRequest{
		PublicDirectoryID: 7, RelPath: "a.md", Content: body2,
	})
	if err != nil {
		t.Fatalf("WriteMarkdown v2: %v", err)
	}
	if node2.HasBrokenLink {
		t.Errorf("HasBrokenLink after clean write = true, want false")
	}
}

// TestWriteMarkdown_AutoIndexDisabled verifies that disabling AutoIndexUpdate
// skips index regeneration but still appends log + computes broken-link.
func TestWriteMarkdown_AutoIndexDisabled(t *testing.T) {
	bundles := newFakeOkfBundleRepo()
	nodes := newFakeOkfNodeRepo()
	pd := &model.DiskPublicDirectory{ID: 7, FolderID: 100, FixedPath: "/public/kb"}
	pub := newFakeOkfPublicDir(pd)
	pub.seedContent("index.md", mustIndexMD(t, "0.1", "Original"))
	svc := NewOkfServiceFromRepo(bundles, nodes, newFakeOkfEdgeRepo(nodes), pub, "sqlite")
	svc.SetAutoIndexUpdate(false)
	if _, err := svc.RegisterBundle(context.Background(), 7); err != nil {
		t.Fatalf("RegisterBundle: %v", err)
	}

	if _, err := svc.WriteMarkdown(context.Background(), WriteMarkdownRequest{
		PublicDirectoryID: 7, RelPath: "a.md",
		Content: mustTypedMD("concept", "A"),
	}); err != nil {
		t.Fatalf("WriteMarkdown: %v", err)
	}
	// The auto-index is off, so the user-authored index.md must be untouched.
	file, err := pub.FindFileByRelPath(7, "index.md")
	if err != nil {
		t.Fatalf("index.md missing: %v", err)
	}
	body, _ := pub.ReadFileContent(context.Background(), file.ID)
	// The body should still carry the original frontmatter from the seeded
	// index.md, not the generated header.
	if !strings.Contains(string(body), "Original") {
		t.Errorf("auto-index overwrote user content: %q", string(body))
	}
}
