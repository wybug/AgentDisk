package service

import (
	"testing"

	"github.com/agentdisk/agent-disk/internal/model"
)

type mockPublicDirRepo struct {
	dirs   map[uint64]*model.DiskPublicDirectory
	nextID uint64
}

func newMockPublicDirRepo() *mockPublicDirRepo {
	return &mockPublicDirRepo{
		dirs:   make(map[uint64]*model.DiskPublicDirectory),
		nextID: 1,
	}
}

func (m *mockPublicDirRepo) Create(pd *model.DiskPublicDirectory) error {
	pd.ID = m.nextID
	m.nextID++
	m.dirs[pd.ID] = pd
	return nil
}

func (m *mockPublicDirRepo) GetByID(id uint64) (*model.DiskPublicDirectory, error) {
	if pd, ok := m.dirs[id]; ok {
		return pd, nil
	}
	return nil, errTestNotFound
}

func (m *mockPublicDirRepo) GetByFolderID(folderID uint64) (*model.DiskPublicDirectory, error) {
	for _, pd := range m.dirs {
		if pd.FolderID == folderID {
			return pd, nil
		}
	}
	return nil, errTestNotFound
}

func (m *mockPublicDirRepo) ListActive() ([]model.DiskPublicDirectory, error) {
	var result []model.DiskPublicDirectory
	for _, pd := range m.dirs {
		if pd.IsActive {
			result = append(result, *pd)
		}
	}
	return result, nil
}

func (m *mockPublicDirRepo) ListByScope(scope string) ([]model.DiskPublicDirectory, error) {
	var result []model.DiskPublicDirectory
	for _, pd := range m.dirs {
		if pd.IsActive && pd.Scope == scope {
			result = append(result, *pd)
		}
	}
	return result, nil
}

func (m *mockPublicDirRepo) Update(pd *model.DiskPublicDirectory) error {
	m.dirs[pd.ID] = pd
	return nil
}

func (m *mockPublicDirRepo) Delete(id uint64) error {
	delete(m.dirs, id)
	return nil
}

type mockFolderCreator struct {
	folders map[uint64]*model.DiskFolder
	nextID  uint64
}

func newMockFolderCreator() *mockFolderCreator {
	return &mockFolderCreator{
		folders: make(map[uint64]*model.DiskFolder),
		nextID:  1,
	}
}

func (m *mockFolderCreator) Create(folder *model.DiskFolder) error {
	folder.ID = m.nextID
	m.nextID++
	m.folders[folder.ID] = folder
	return nil
}

func (m *mockFolderCreator) GetByID(id uint64) (*model.DiskFolder, error) {
	if f, ok := m.folders[id]; ok {
		return f, nil
	}
	return nil, errTestNotFound
}

func (m *mockFolderCreator) ListByParent(userID string, parentID uint64) ([]model.DiskFolder, error) {
	var result []model.DiskFolder
	for _, f := range m.folders {
		if f.UserID == userID && f.ParentID == parentID && !f.IsDeleted {
			result = append(result, *f)
		}
	}
	return result, nil
}

func (m *mockFolderCreator) SoftDelete(id uint64) error {
	if f, ok := m.folders[id]; ok {
		f.IsDeleted = true
		return nil
	}
	return errTestNotFound
}

func TestPublicDirectoryService_CreateGlobal(t *testing.T) {
	svc := NewPublicDirectoryServiceFromRepo(newMockPublicDirRepo(), newMockFolderCreator())

	pd, err := svc.CreatePublicDirectory("reports", ScopeGlobal, "", "admin")
	if err != nil {
		t.Fatalf("CreatePublicDirectory failed: %v", err)
	}
	if pd.Scope != ScopeGlobal {
		t.Errorf("scope = %q, want global", pd.Scope)
	}
	if pd.FixedPath != "/public/reports" {
		t.Errorf("fixedPath = %q, want /public/reports", pd.FixedPath)
	}
	if pd.DisplayName != "reports" {
		t.Errorf("displayName = %q, want reports", pd.DisplayName)
	}
}

func TestPublicDirectoryService_CreateDepartment(t *testing.T) {
	svc := NewPublicDirectoryServiceFromRepo(newMockPublicDirRepo(), newMockFolderCreator())

	pd, err := svc.CreatePublicDirectory("guides", ScopeDepartment, "engineering", "admin")
	if err != nil {
		t.Fatalf("CreatePublicDirectory failed: %v", err)
	}
	if pd.FixedPath != "/department/engineering/guides" {
		t.Errorf("fixedPath = %q, want /department/engineering/guides", pd.FixedPath)
	}
}

func TestPublicDirectoryService_CreateDepartmentMissingDept(t *testing.T) {
	svc := NewPublicDirectoryServiceFromRepo(newMockPublicDirRepo(), newMockFolderCreator())

	_, err := svc.CreatePublicDirectory("guides", ScopeDepartment, "", "admin")
	if err == nil {
		t.Error("expected error for department scope without department")
	}
}

func TestPublicDirectoryService_CreateInvalidScope(t *testing.T) {
	svc := NewPublicDirectoryServiceFromRepo(newMockPublicDirRepo(), newMockFolderCreator())

	_, err := svc.CreatePublicDirectory("test", "invalid", "", "admin")
	if err == nil {
		t.Error("expected error for invalid scope")
	}
}

func TestPublicDirectoryService_ListVisible(t *testing.T) {
	repo := newMockPublicDirRepo()
	svc := NewPublicDirectoryServiceFromRepo(repo, newMockFolderCreator())

	svc.CreatePublicDirectory("global-reports", ScopeGlobal, "", "admin")
	svc.CreatePublicDirectory("eng-docs", ScopeDepartment, "engineering", "admin")
	svc.CreatePublicDirectory("mkt-docs", ScopeDepartment, "marketing", "admin")

	t.Run("engineering user sees global + engineering", func(t *testing.T) {
		visible, err := svc.ListVisible("engineering")
		if err != nil {
			t.Fatalf("ListVisible failed: %v", err)
		}
		if len(visible) != 2 {
			t.Errorf("got %d visible, want 2", len(visible))
		}
	})

	t.Run("no department sees global + all departments", func(t *testing.T) {
		visible, err := svc.ListVisible("")
		if err != nil {
			t.Fatalf("ListVisible failed: %v", err)
		}
		if len(visible) != 3 {
			t.Errorf("got %d visible, want 3", len(visible))
		}
	})
}

func TestPublicDirectoryService_Delete(t *testing.T) {
	repo := newMockPublicDirRepo()
	folderRepo := newMockFolderCreator()
	svc := NewPublicDirectoryServiceFromRepo(repo, folderRepo)

	pd, _ := svc.CreatePublicDirectory("test", ScopeGlobal, "", "admin")
	err := svc.DeletePublicDirectory(pd.ID)
	if err != nil {
		t.Fatalf("DeletePublicDirectory failed: %v", err)
	}

	_, err = svc.GetPublicDirectory(pd.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestPublicDirectoryService_Update(t *testing.T) {
	repo := newMockPublicDirRepo()
	svc := NewPublicDirectoryServiceFromRepo(repo, newMockFolderCreator())

	pd, _ := svc.CreatePublicDirectory("test", ScopeGlobal, "", "admin")
	updated, err := svc.UpdatePublicDirectory(pd.ID, "renamed", false)
	if err != nil {
		t.Fatalf("UpdatePublicDirectory failed: %v", err)
	}
	if updated.DisplayName != "renamed" {
		t.Errorf("displayName = %q, want renamed", updated.DisplayName)
	}
	if updated.IsActive {
		t.Error("isActive should be false")
	}
}

func TestIsReservedPath(t *testing.T) {
	if !IsReservedPath("public") {
		t.Error("'public' should be reserved")
	}
	if !IsReservedPath("department") {
		t.Error("'department' should be reserved")
	}
	if IsReservedPath("my-folder") {
		t.Error("'my-folder' should not be reserved")
	}
	if IsReservedPath("public-docs") {
		t.Error("'public-docs' should not be reserved")
	}
}
