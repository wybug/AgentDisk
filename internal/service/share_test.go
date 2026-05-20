package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/agentdisk/agent-disk/internal/model"
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
