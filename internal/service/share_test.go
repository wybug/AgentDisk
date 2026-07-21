package service

import (
	"errors"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
)

// ── Mock repositories ──

type mockShareRepo struct {
	shares  map[uint64]*model.DiskShare
	nextID  uint64
	byCode  map[string]*model.DiskShare
	access  []*model.ShareAccessLog
	updates map[uint64]int
}

func newMockShareRepo() *mockShareRepo {
	return &mockShareRepo{
		shares:  make(map[uint64]*model.DiskShare),
		nextID:  1,
		byCode:  make(map[string]*model.DiskShare),
		updates: make(map[uint64]int),
	}
}

func (m *mockShareRepo) addShare(s *model.DiskShare) *model.DiskShare {
	if s.ID == 0 {
		s.ID = m.nextID
		m.nextID++
	}
	m.shares[s.ID] = s
	m.byCode[s.ShareCode] = s
	return s
}

func (m *mockShareRepo) Create(s *model.DiskShare) error {
	s.ID = m.nextID
	m.nextID++
	m.shares[s.ID] = s
	m.byCode[s.ShareCode] = s
	return nil
}

func (m *mockShareRepo) GetByCode(code string) (*model.DiskShare, error) {
	s, ok := m.byCode[code]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return s, nil
}

func (m *mockShareRepo) GetByID(id uint64) (*model.DiskShare, error) {
	s, ok := m.shares[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return s, nil
}

func (m *mockShareRepo) IncrementVisitCount(id uint64) error {
	_, ok := m.shares[id]
	if !ok {
		return fmt.Errorf("not found")
	}
	m.updates[id]++
	return nil
}

func (m *mockShareRepo) RevokeByID(id uint64) error {
	s, ok := m.shares[id]
	if !ok {
		return fmt.Errorf("not found")
	}
	s.IsActive = false
	m.updates[id]++
	return nil
}

func (m *mockShareRepo) ListByUser(userID string) ([]model.DiskShare, error) {
	var result []model.DiskShare
	for _, s := range m.shares {
		if s.UserID == userID {
			result = append(result, *s)
		}
	}
	return result, nil
}

func (m *mockShareRepo) LogAccess(log *model.ShareAccessLog) error {
	m.access = append(m.access, log)
	return nil
}

func (m *mockShareRepo) ListAccessLogsByShare(shareID uint64, limit int) ([]model.ShareAccessLog, error) {
	var rows []model.ShareAccessLog
	for _, l := range m.access {
		if l.ShareID == shareID {
			rows = append(rows, *l)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].CreatedAt.After(rows[j].CreatedAt) })
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows, nil
}

func (m *mockShareRepo) ShareAccessStats(shareID uint64) (repository.ShareAccessStatsRow, error) {
	seen := make(map[string]struct{})
	for _, l := range m.access {
		if l.ShareID == shareID && l.VisitorIP != "" {
			seen[l.VisitorIP] = struct{}{}
		}
	}
	return repository.ShareAccessStatsRow{UniqueIPs: uint64(len(seen))}, nil
}

type mockFileResourceRepo struct {
	files map[uint64]*model.DiskFile
}

func newMockFileResourceRepo() *mockFileResourceRepo {
	return &mockFileResourceRepo{files: make(map[uint64]*model.DiskFile)}
}

func (m *mockFileResourceRepo) addFile(id uint64, userID, fileName string) {
	m.files[id] = &model.DiskFile{ID: id, UserID: userID, FileName: fileName}
}

func (m *mockFileResourceRepo) GetByID(id uint64) (*model.DiskFile, error) {
	f, ok := m.files[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return f, nil
}

type mockFolderResourceRepo struct {
	folders map[uint64]*model.DiskFolder
}

func newMockFolderResourceRepo() *mockFolderResourceRepo {
	return &mockFolderResourceRepo{folders: make(map[uint64]*model.DiskFolder)}
}

func (m *mockFolderResourceRepo) addFolder(id uint64, userID, folderName string) {
	m.folders[id] = &model.DiskFolder{ID: id, UserID: userID, FolderName: folderName}
}

func (m *mockFolderResourceRepo) GetByID(id uint64) (*model.DiskFolder, error) {
	f, ok := m.folders[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return f, nil
}

func newTestShareService() (*ShareService, *mockShareRepo, *mockFileResourceRepo, *mockFolderResourceRepo) {
	sr := newMockShareRepo()
	fr := newMockFileResourceRepo()
	fdr := newMockFolderResourceRepo()
	return &ShareService{repo: sr, fileRepo: fr, folderRepo: fdr}, sr, fr, fdr
}

// mockBundleResourceRepo is the bundle-case stub for ShareService.bundleRepo.
type mockBundleResourceRepo struct {
	bundles map[uint64]*model.OkfBundle
}

func newMockBundleResourceRepo() *mockBundleResourceRepo {
	return &mockBundleResourceRepo{bundles: make(map[uint64]*model.OkfBundle)}
}

func (m *mockBundleResourceRepo) addBundle(id, pdID uint64, title string) {
	m.bundles[id] = &model.OkfBundle{ID: id, PublicDirectoryID: pdID, Title: title}
}

func (m *mockBundleResourceRepo) GetByID(id uint64) (*model.OkfBundle, error) {
	b, ok := m.bundles[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return b, nil
}

func (m *mockBundleResourceRepo) GetByPublicDirectoryID(pdID uint64) (*model.OkfBundle, error) {
	for _, b := range m.bundles {
		if b.PublicDirectoryID == pdID {
			return b, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

// Implement the rest of okfBundleRepo so mock satisfies the interface even
// though ShareService only calls GetByID. These are panics-on-use so a test
// that reaches them fails loudly rather than silently returning zero values.
func (m *mockBundleResourceRepo) Create(_ *model.OkfBundle) error { panic("unused") }
func (m *mockBundleResourceRepo) List(_ string, _, _ int) ([]model.OkfBundle, error) {
	panic("unused")
}

func (m *mockBundleResourceRepo) Update(_ *model.OkfBundle) error { panic("unused") }
func (m *mockBundleResourceRepo) Delete(_ uint64) error           { panic("unused") }

// mockBundleGrantChecker stubs the publicDirGrantChecker interface for the
// bundle path. The two maps let tests branch on (a) explicit grant for the
// public directory and (b) explicit grant for a file inside it.
type mockBundleGrantChecker struct {
	pdGrants   map[uint64]bool
	fileGrants map[uint64]bool
}

func newMockBundleGrantChecker() *mockBundleGrantChecker {
	return &mockBundleGrantChecker{
		pdGrants:   make(map[uint64]bool),
		fileGrants: make(map[uint64]bool),
	}
}

func (m *mockBundleGrantChecker) IsUserGrantedForFile(fileID uint64, _ string) (bool, error) {
	return m.fileGrants[fileID], nil
}

func (m *mockBundleGrantChecker) IsPublicDirVisibleToUser(publicDirID uint64, _ string) (bool, error) {
	return m.pdGrants[publicDirID], nil
}

// ── generateShareCode tests ──

func TestGenerateShareCode(t *testing.T) {
	code1, err := generateShareCode()
	if err != nil {
		t.Fatalf("generateShareCode failed: %v", err)
	}
	if len(code1) != 32 {
		t.Errorf("expected 32 char code, got %d", len(code1))
	}

	code2, _ := generateShareCode()
	if code1 == code2 {
		t.Error("two generated codes should be different")
	}
}

// ── CreateShare tests ──

func TestCreateShare_File(t *testing.T) {
	svc, _, fr, _ := newTestShareService()
	fr.addFile(1, "user001", "test.txt")

	share, err := svc.CreateShare("user001", 1, "file", "", -1, 72)
	if err != nil {
		t.Fatalf("CreateShare failed: %v", err)
	}
	if share.UserID != "user001" {
		t.Errorf("UserID = %q, want user001", share.UserID)
	}
	if share.ResourceID != 1 {
		t.Errorf("ResourceID = %d, want 1", share.ResourceID)
	}
	if share.ResType != "file" {
		t.Errorf("ResType = %q, want file", share.ResType)
	}
	if !share.IsActive {
		t.Error("IsActive should be true for new share")
	}
	if share.ShareCode == "" {
		t.Error("ShareCode should not be empty")
	}
	if len(share.ShareCode) != 32 {
		t.Errorf("ShareCode length = %d, want 32", len(share.ShareCode))
	}
}

func TestCreateShare_Folder(t *testing.T) {
	svc, _, _, fdr := newTestShareService()
	fdr.addFolder(5, "user001", "my-folder")

	share, err := svc.CreateShare("user001", 5, "folder", "abc", 100, 24)
	if err != nil {
		t.Fatalf("CreateShare failed: %v", err)
	}
	if share.ResType != "folder" {
		t.Errorf("ResType = %q, want folder", share.ResType)
	}
	if share.ExtractCode != "abc" {
		t.Errorf("ExtractCode = %q, want abc", share.ExtractCode)
	}
	if share.MaxVisit != 100 {
		t.Errorf("MaxVisit = %d, want 100", share.MaxVisit)
	}
}

func TestCreateShare_UnsupportedResType(t *testing.T) {
	svc, _, _, _ := newTestShareService()

	_, err := svc.CreateShare("user001", 1, "image", "", -1, 72)
	if err == nil {
		t.Error("expected error for unsupported resType")
	}
}

func TestCreateShare_FileNotFound(t *testing.T) {
	svc, _, _, _ := newTestShareService()

	_, err := svc.CreateShare("user001", 999, "file", "", -1, 72)
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestCreateShare_FolderNotFound(t *testing.T) {
	svc, _, _, _ := newTestShareService()

	_, err := svc.CreateShare("user001", 999, "folder", "", -1, 72)
	if err == nil {
		t.Error("expected error for non-existent folder")
	}
}

func TestCreateShare_CrossUserFile(t *testing.T) {
	svc, _, fr, _ := newTestShareService()
	fr.addFile(1, "user002", "other.txt")

	_, err := svc.CreateShare("user001", 1, "file", "", -1, 72)
	if err == nil {
		t.Error("expected error for sharing another user's file")
	}
}

func TestCreateShare_CrossUserFolder(t *testing.T) {
	svc, _, _, fdr := newTestShareService()
	fdr.addFolder(1, "user002", "other-folder")

	_, err := svc.CreateShare("user001", 1, "folder", "", -1, 72)
	if err == nil {
		t.Error("expected error for sharing another user's folder")
	}
}

func TestCreateShare_ExpireTime(t *testing.T) {
	svc, _, fr, _ := newTestShareService()
	fr.addFile(1, "user001", "test.txt")

	before := time.Now()
	share, err := svc.CreateShare("user001", 1, "file", "", -1, 48)
	if err != nil {
		t.Fatalf("CreateShare failed: %v", err)
	}

	expectedExpiry := before.Add(48 * time.Hour)
	if share.ExpireAt.Before(expectedExpiry.Add(-time.Second)) || share.ExpireAt.After(expectedExpiry.Add(time.Second)) {
		t.Errorf("ExpireAt = %v, expected near %v", share.ExpireAt, expectedExpiry)
	}
}

// ── GetShareByCode tests ──

func TestGetShareByCode_Active(t *testing.T) {
	svc, sr, _, _ := newTestShareService()
	sr.addShare(&model.DiskShare{
		ShareCode: "abc123",
		IsActive:  true,
		ExpireAt:  time.Now().Add(24 * time.Hour),
	})

	share, err := svc.GetShareByCode("abc123")
	if err != nil {
		t.Fatalf("GetShareByCode failed: %v", err)
	}
	if share.ShareCode != "abc123" {
		t.Errorf("ShareCode = %q, want abc123", share.ShareCode)
	}
}

func TestGetShareByCode_NotFound(t *testing.T) {
	svc, _, _, _ := newTestShareService()

	_, err := svc.GetShareByCode("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent share code")
	}
}

func TestGetShareByCode_Revoked(t *testing.T) {
	svc, sr, _, _ := newTestShareService()
	sr.addShare(&model.DiskShare{
		ShareCode: "revoked123",
		IsActive:  false,
		ExpireAt:  time.Now().Add(24 * time.Hour),
	})

	_, err := svc.GetShareByCode("revoked123")
	if err == nil {
		t.Error("expected error for revoked share")
	}
}

func TestGetShareByCode_Expired(t *testing.T) {
	svc, sr, _, _ := newTestShareService()
	sr.addShare(&model.DiskShare{
		ShareCode: "expired123",
		IsActive:  true,
		ExpireAt:  time.Now().Add(-1 * time.Hour),
	})

	_, err := svc.GetShareByCode("expired123")
	if err == nil {
		t.Error("expected error for expired share")
	}
}

// ── AccessShare tests ──

func TestAccessShare_Valid(t *testing.T) {
	svc, sr, _, _ := newTestShareService()
	sr.addShare(&model.DiskShare{
		ID:         1,
		ShareCode:  "access123",
		IsActive:   true,
		ExpireAt:   time.Now().Add(24 * time.Hour),
		MaxVisit:   -1,
		VisitCount: 0,
	})

	share, err := svc.AccessShare("access123", "", "127.0.0.1", "TestAgent")
	if err != nil {
		t.Fatalf("AccessShare failed: %v", err)
	}
	if share.VisitCount != 1 {
		t.Errorf("VisitCount = %d, want 1", share.VisitCount)
	}
	if len(sr.access) != 1 {
		t.Errorf("expected 1 access log, got %d", len(sr.access))
	}
	if sr.access[0].VisitorIP != "127.0.0.1" {
		t.Errorf("VisitorIP = %q, want 127.0.0.1", sr.access[0].VisitorIP)
	}
}

func TestAccessShare_WithExtractCode(t *testing.T) {
	svc, sr, _, _ := newTestShareService()
	sr.addShare(&model.DiskShare{
		ID:          1,
		ShareCode:   "code123",
		IsActive:    true,
		ExpireAt:    time.Now().Add(24 * time.Hour),
		MaxVisit:    -1,
		ExtractCode: "1234",
	})

	share, err := svc.AccessShare("code123", "1234", "127.0.0.1", "TestAgent")
	if err != nil {
		t.Fatalf("AccessShare with correct extract code failed: %v", err)
	}
	if share.VisitCount != 1 {
		t.Errorf("VisitCount = %d, want 1", share.VisitCount)
	}
}

func TestAccessShare_WrongExtractCode(t *testing.T) {
	svc, sr, _, _ := newTestShareService()
	sr.addShare(&model.DiskShare{
		ID:          1,
		ShareCode:   "code456",
		IsActive:    true,
		ExpireAt:    time.Now().Add(24 * time.Hour),
		MaxVisit:    -1,
		ExtractCode: "1234",
	})

	_, err := svc.AccessShare("code456", "wrong", "127.0.0.1", "TestAgent")
	if err == nil {
		t.Error("expected error for wrong extract code")
	}
}

func TestAccessShare_MaxVisitReached(t *testing.T) {
	svc, sr, _, _ := newTestShareService()
	sr.addShare(&model.DiskShare{
		ID:         1,
		ShareCode:  "limited123",
		IsActive:   true,
		ExpireAt:   time.Now().Add(24 * time.Hour),
		MaxVisit:   2,
		VisitCount: 2,
	})

	_, err := svc.AccessShare("limited123", "", "127.0.0.1", "TestAgent")
	if err == nil {
		t.Error("expected error when max visit reached")
	}
}

func TestAccessShare_VisitCountIncrement(t *testing.T) {
	svc, sr, _, _ := newTestShareService()
	sr.addShare(&model.DiskShare{
		ID:         1,
		ShareCode:  "counter123",
		IsActive:   true,
		ExpireAt:   time.Now().Add(24 * time.Hour),
		MaxVisit:   10,
		VisitCount: 5,
	})

	share, err := svc.AccessShare("counter123", "", "127.0.0.1", "TestAgent")
	if err != nil {
		t.Fatalf("AccessShare failed: %v", err)
	}
	if share.VisitCount != 6 {
		t.Errorf("VisitCount = %d, want 6", share.VisitCount)
	}
	if sr.updates[1] != 1 {
		t.Errorf("Update should have been called once for share ID 1, got %d", sr.updates[1])
	}
}

// ── RevokeShare tests ──

func TestRevokeShare_Success(t *testing.T) {
	svc, sr, _, _ := newTestShareService()
	sr.addShare(&model.DiskShare{
		ID:       1,
		UserID:   "user001",
		IsActive: true,
	})

	err := svc.RevokeShare("user001", 1)
	if err != nil {
		t.Fatalf("RevokeShare failed: %v", err)
	}
	if sr.shares[1].IsActive {
		t.Error("IsActive should be false after revoke")
	}
	if sr.updates[1] != 1 {
		t.Errorf("Update should have been called once, got %d", sr.updates[1])
	}
}

func TestRevokeShare_NotFound(t *testing.T) {
	svc, _, _, _ := newTestShareService()

	err := svc.RevokeShare("user001", 999)
	if err == nil {
		t.Error("expected error for non-existent share")
	}
}

func TestRevokeShare_CrossUser(t *testing.T) {
	svc, sr, _, _ := newTestShareService()
	sr.addShare(&model.DiskShare{
		ID:       1,
		UserID:   "user002",
		IsActive: true,
	})

	err := svc.RevokeShare("user001", 1)
	if err == nil {
		t.Error("expected error for cross-user revoke")
	}
	if !sr.shares[1].IsActive {
		t.Error("IsActive should remain true after failed revoke")
	}
}

func TestRevokeShare_AlreadyRevoked(t *testing.T) {
	svc, sr, _, _ := newTestShareService()
	sr.addShare(&model.DiskShare{
		ID:       1,
		UserID:   "user001",
		IsActive: false,
	})

	err := svc.RevokeShare("user001", 1)
	if err != nil {
		t.Fatalf("RevokeShare on already-revoked share should not error: %v", err)
	}
	if sr.shares[1].IsActive {
		t.Error("IsActive should remain false")
	}
}

// ── ListShares tests ──

func TestListShares_ByUser(t *testing.T) {
	svc, sr, _, _ := newTestShareService()
	sr.addShare(&model.DiskShare{UserID: "user001", ShareCode: "code1"})
	sr.addShare(&model.DiskShare{UserID: "user001", ShareCode: "code2"})
	sr.addShare(&model.DiskShare{UserID: "user002", ShareCode: "code3"})

	shares, err := svc.ListShares("user001")
	if err != nil {
		t.Fatalf("ListShares failed: %v", err)
	}
	if len(shares) != 2 {
		t.Errorf("expected 2 shares for user001, got %d", len(shares))
	}
}

func TestListShares_NoShares(t *testing.T) {
	svc, _, _, _ := newTestShareService()

	shares, err := svc.ListShares("user_empty")
	if err != nil {
		t.Fatalf("ListShares failed: %v", err)
	}
	if len(shares) != 0 {
		t.Errorf("expected 0 shares, got %d", len(shares))
	}
}

func TestListShares_IncludesRevoked(t *testing.T) {
	svc, sr, _, _ := newTestShareService()
	sr.addShare(&model.DiskShare{UserID: "user001", ShareCode: "active1", IsActive: true})
	sr.addShare(&model.DiskShare{UserID: "user001", ShareCode: "revoked1", IsActive: false})

	shares, err := svc.ListShares("user001")
	if err != nil {
		t.Fatalf("ListShares failed: %v", err)
	}
	if len(shares) != 2 {
		t.Errorf("ListByUser should return all shares including revoked, got %d", len(shares))
	}
}

// ── CreateShare bundle tests ──

func TestCreateShare_BundleSuccess(t *testing.T) {
	svc, _, _, _ := newTestShareService()
	br := newMockBundleResourceRepo()
	gc := newMockBundleGrantChecker()
	br.addBundle(7, 42, "demo bundle")
	gc.pdGrants[42] = true
	svc.bundleRepo = br
	svc.grantChecker = gc

	share, err := svc.CreateShare("user001", 7, "bundle", "", -1, 72)
	if err != nil {
		t.Fatalf("CreateShare bundle failed: %v", err)
	}
	if share.ResType != "bundle" {
		t.Errorf("ResType = %q, want bundle", share.ResType)
	}
	if share.ResourceID != 7 {
		t.Errorf("ResourceID = %d, want 7", share.ResourceID)
	}
}

func TestCreateShare_BundleNoBundleRepo(t *testing.T) {
	// Without SetBundleRepo, "bundle" is rejected as unsupported. Guards
	// against the case where OKF is disabled at runtime but a caller still
	// tries the endpoint.
	svc, _, _, _ := newTestShareService()
	gc := newMockBundleGrantChecker()
	svc.grantChecker = gc

	_, err := svc.CreateShare("user001", 7, "bundle", "", -1, 72)
	if err == nil {
		t.Error("expected error when bundleRepo is nil")
	}
}

func TestCreateShare_BundleNotFound(t *testing.T) {
	svc, _, _, _ := newTestShareService()
	br := newMockBundleResourceRepo()
	gc := newMockBundleGrantChecker()
	svc.bundleRepo = br
	svc.grantChecker = gc

	_, err := svc.CreateShare("user001", 999, "bundle", "", -1, 72)
	if err == nil {
		t.Error("expected error for non-existent bundle")
	}
}

func TestCreateShare_BundleNotGranted(t *testing.T) {
	svc, _, _, _ := newTestShareService()
	br := newMockBundleResourceRepo()
	gc := newMockBundleGrantChecker()
	br.addBundle(7, 42, "demo bundle")
	// pdGrants[42] intentionally not set
	svc.bundleRepo = br
	svc.grantChecker = gc

	_, err := svc.CreateShare("user001", 7, "bundle", "", -1, 72)
	if err == nil {
		t.Error("expected error when user lacks grant for bundle's public dir")
	}
}

func TestCreateShare_BundleNoGrantChecker(t *testing.T) {
	// GrantChecker is required for bundle path (it's optional for files
	// because file ownership falls back to UserID comparison).
	svc, _, _, _ := newTestShareService()
	br := newMockBundleResourceRepo()
	br.addBundle(7, 42, "demo bundle")
	svc.bundleRepo = br

	_, err := svc.CreateShare("user001", 7, "bundle", "", -1, 72)
	if err == nil {
		t.Error("expected error when grantChecker is nil")
	}
}

// ── ShareStats tests ──

func TestShareStats_OwnerSeesAggregatesAndLogs(t *testing.T) {
	svc, sr, _, _ := newTestShareService()
	sr.addShare(&model.DiskShare{ID: 1, UserID: "u1", ShareCode: "c1", VisitCount: 3})
	base := time.Now().UTC()
	sr.access = append(sr.access,
		&model.ShareAccessLog{ShareID: 1, VisitorIP: "10.0.0.1", UserAgent: "curl/8", Action: "access", CreatedAt: base.Add(-2 * time.Hour)},
		&model.ShareAccessLog{ShareID: 1, VisitorIP: "10.0.0.2", UserAgent: "Mozilla", Action: "access", CreatedAt: base.Add(-1 * time.Hour)},
		&model.ShareAccessLog{ShareID: 1, VisitorIP: "10.0.0.1", UserAgent: "curl/8", Action: "access", CreatedAt: base},
	)

	out, err := svc.ShareStats("u1", 1)
	if err != nil {
		t.Fatalf("ShareStats: %v", err)
	}
	if out.VisitCount != 3 {
		t.Errorf("visitCount = %d, want 3", out.VisitCount)
	}
	// 2 distinct raw IPs (10.0.0.1 counted once), even though both mask to 10.0.0.0.
	if out.UniqueIPs != 2 {
		t.Errorf("uniqueIPs = %d, want 2", out.UniqueIPs)
	}
	if out.LastAccessAt == nil || !out.LastAccessAt.Equal(base) {
		t.Errorf("lastAccessAt = %v, want %v", out.LastAccessAt, base)
	}
	if len(out.RecentLogs) != 3 {
		t.Fatalf("recentLogs len = %d, want 3", len(out.RecentLogs))
	}
	if !out.RecentLogs[0].CreatedAt.Equal(base) {
		t.Errorf("recentLogs not newest-first; [0] = %v", out.RecentLogs[0].CreatedAt)
	}
	for i, l := range out.RecentLogs {
		if l.VisitorIP != "10.0.0.0" {
			t.Errorf("recentLogs[%d].visitorIP = %q, want masked 10.0.0.0", i, l.VisitorIP)
		}
	}
}

func TestShareStats_NonOwnerDenied(t *testing.T) {
	svc, sr, _, _ := newTestShareService()
	sr.addShare(&model.DiskShare{ID: 1, UserID: "u1", ShareCode: "c1"})
	_, err := svc.ShareStats("u2", 1)
	if !errors.Is(err, ErrSharePermissionDenied) {
		t.Fatalf("err = %v, want ErrSharePermissionDenied", err)
	}
}

func TestShareStats_NotFound(t *testing.T) {
	svc, _, _, _ := newTestShareService()
	_, err := svc.ShareStats("u1", 999)
	if !errors.Is(err, ErrShareNotFound) {
		t.Fatalf("err = %v, want ErrShareNotFound", err)
	}
}

func TestShareStats_EmptyLogs(t *testing.T) {
	svc, sr, _, _ := newTestShareService()
	sr.addShare(&model.DiskShare{ID: 1, UserID: "u1", ShareCode: "c1"})
	out, err := svc.ShareStats("u1", 1)
	if err != nil {
		t.Fatalf("ShareStats: %v", err)
	}
	if out.UniqueIPs != 0 {
		t.Errorf("uniqueIPs = %d, want 0", out.UniqueIPs)
	}
	if out.LastAccessAt != nil {
		t.Errorf("lastAccessAt = %v, want nil", out.LastAccessAt)
	}
	if len(out.RecentLogs) != 0 {
		t.Errorf("recentLogs len = %d, want 0", len(out.RecentLogs))
	}
}

func TestShareStats_MasksVisitorIP(t *testing.T) {
	svc, sr, _, _ := newTestShareService()
	sr.addShare(&model.DiskShare{ID: 1, UserID: "u1", ShareCode: "c1"})
	sr.access = append(sr.access, &model.ShareAccessLog{
		ShareID: 1, VisitorIP: "192.168.1.42", UserAgent: "ua", Action: "access", CreatedAt: time.Now(),
	})
	out, err := svc.ShareStats("u1", 1)
	if err != nil {
		t.Fatalf("ShareStats: %v", err)
	}
	if len(out.RecentLogs) != 1 || out.RecentLogs[0].VisitorIP != "192.168.1.0" {
		t.Fatalf("masked visitorIP = %q, want 192.168.1.0", out.RecentLogs[0].VisitorIP)
	}
}

func TestMaskVisitorIP(t *testing.T) {
	cases := map[string]string{
		"192.168.1.42": "192.168.1.0",
		"10.0.0.1":     "10.0.0.0",
		"2001:db8::1":  "2001:db8::0",
		"":             "",
		"unknown":      "unknown",
	}
	for in, want := range cases {
		if got := maskVisitorIP(in); got != want {
			t.Errorf("maskVisitorIP(%q) = %q, want %q", in, got, want)
		}
	}
}
