package service

import (
	"errors"
	"testing"

	"github.com/agentdisk/agent-disk/internal/model"
)

// ── Mock repository ──

type mockAdminRepo struct {
	admins map[string]*model.DiskAdminUser
	count  int64
	err    error
}

func newMockAdminRepo() *mockAdminRepo {
	return &mockAdminRepo{
		admins: make(map[string]*model.DiskAdminUser),
	}
}

func (m *mockAdminRepo) GetByUsername(username string) (*model.DiskAdminUser, error) {
	if m.err != nil {
		return nil, m.err
	}
	a, ok := m.admins[username]
	if !ok {
		return nil, errors.New("not found")
	}
	return a, nil
}

func (m *mockAdminRepo) Create(admin *model.DiskAdminUser) error {
	if m.err != nil {
		return m.err
	}
	m.admins[admin.Username] = admin
	m.count++
	return nil
}

func (m *mockAdminRepo) Delete(username string) error {
	if m.err != nil {
		return m.err
	}
	delete(m.admins, username)
	m.count--
	return nil
}

func (m *mockAdminRepo) ListAll() ([]model.DiskAdminUser, error) {
	if m.err != nil {
		return nil, m.err
	}
	var result []model.DiskAdminUser
	for _, a := range m.admins {
		result = append(result, *a)
	}
	return result, nil
}

func (m *mockAdminRepo) UpdatePassword(username, passwordHash string) error {
	if m.err != nil {
		return m.err
	}
	a, ok := m.admins[username]
	if !ok {
		return errors.New("not found")
	}
	a.PasswordHash = passwordHash
	return nil
}

func (m *mockAdminRepo) Count() (int64, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.count, nil
}

// ── Count tests ──

func TestAdminService_Count_NoAdmins(t *testing.T) {
	repo := newMockAdminRepo()
	svc := &AdminService{repo: repo}

	count, err := svc.Count()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

func TestAdminService_Count_WithAdmins(t *testing.T) {
	repo := newMockAdminRepo()
	repo.count = 2
	svc := &AdminService{repo: repo}

	count, err := svc.Count()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestAdminService_Count_DBError(t *testing.T) {
	repo := newMockAdminRepo()
	repo.err = errors.New("connection refused")
	svc := &AdminService{repo: repo}

	_, err := svc.Count()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
