package service

import (
	"errors"
	"sync/atomic"
	"testing"

	"github.com/agentdisk/agent-disk/internal/model"
)

// mockOAuth2ConfigRepo implements oauth2ConfigRepo for testing.
type mockOAuth2ConfigRepo struct {
	active         *model.DiskOAuth2Config
	activeErr      error
	all            []model.DiskOAuth2Config
	getActiveCalls atomic.Int64
	createErr      error
	updateErr      error
	created        *model.DiskOAuth2Config
	updated        *model.DiskOAuth2Config
}

func (m *mockOAuth2ConfigRepo) GetActive() (*model.DiskOAuth2Config, error) {
	m.getActiveCalls.Add(1)
	if m.activeErr != nil {
		return nil, m.activeErr
	}
	return m.active, nil
}

func (m *mockOAuth2ConfigRepo) GetByName(_ string) (*model.DiskOAuth2Config, error) {
	return nil, errors.New("not implemented")
}

func (m *mockOAuth2ConfigRepo) Create(cfg *model.DiskOAuth2Config) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.created = cfg
	return nil
}

func (m *mockOAuth2ConfigRepo) Update(cfg *model.DiskOAuth2Config) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.updated = cfg
	return nil
}

func (m *mockOAuth2ConfigRepo) ListAll() ([]model.DiskOAuth2Config, error) {
	return m.all, nil
}

func (m *mockOAuth2ConfigRepo) Delete(_ uint64) error {
	return nil
}

func newTestOAuth2ConfigService(repo *mockOAuth2ConfigRepo) *OAuth2ConfigService {
	return &OAuth2ConfigService{repo: repo}
}

func TestBuildOAuth2Client_CacheHit(t *testing.T) {
	repo := &mockOAuth2ConfigRepo{
		active: &model.DiskOAuth2Config{
			Enabled:      true,
			ClientID:     "test-client",
			ClientSecret: "test-secret",
			IssuerURL:    "https://example.com",
			RedirectURL:  "https://app.example.com/callback",
			Scopes:       "openid,profile",
		},
	}
	svc := newTestOAuth2ConfigService(repo)

	client1, err1 := svc.BuildOAuth2Client()
	if err1 != nil {
		t.Fatalf("first call failed: %v", err1)
	}
	if client1 == nil {
		t.Fatal("first call returned nil client")
	}

	client2, err2 := svc.BuildOAuth2Client()
	if err2 != nil {
		t.Fatalf("second call failed: %v", err2)
	}
	if client2 == nil {
		t.Fatal("second call returned nil client")
	}

	calls := repo.getActiveCalls.Load()
	if calls != 1 {
		t.Fatalf("expected 1 GetActive call (cache hit on second), got %d", calls)
	}
}

func TestBuildOAuth2Client_CacheInvalidation(t *testing.T) {
	repo := &mockOAuth2ConfigRepo{
		active: &model.DiskOAuth2Config{
			Enabled:      true,
			ClientID:     "test-client",
			ClientSecret: "test-secret",
			IssuerURL:    "https://example.com",
			RedirectURL:  "https://app.example.com/callback",
			Scopes:       "openid,profile",
		},
	}
	svc := newTestOAuth2ConfigService(repo)

	// First call: cache miss → DB query
	_, err := svc.BuildOAuth2Client()
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	if repo.getActiveCalls.Load() != 1 {
		t.Fatalf("expected 1 call, got %d", repo.getActiveCalls.Load())
	}

	// Second call: cache hit
	svc.BuildOAuth2Client()
	if repo.getActiveCalls.Load() != 1 {
		t.Fatalf("expected 1 call (cached), got %d", repo.getActiveCalls.Load())
	}

	// Update config → invalidate cache
	repo.active.ClientID = "updated-client"
	err = svc.UpdateConfig("admin", &model.DiskOAuth2Config{
		Enabled:      true,
		ClientID:     "updated-client",
		ClientSecret: "test-secret",
		IssuerURL:    "https://example.com",
		RedirectURL:  "https://app.example.com/callback",
		Scopes:       "openid,profile",
	})
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	// Third call: cache miss after invalidation → DB query
	client, err := svc.BuildOAuth2Client()
	if err != nil {
		t.Fatalf("third call failed: %v", err)
	}
	if client == nil {
		t.Fatal("third call returned nil client")
	}
	// 3 total: 1 initial BuildOAuth2Client + 1 UpdateConfig.GetActive + 1 post-invalidation BuildOAuth2Client
	if repo.getActiveCalls.Load() != 3 {
		t.Fatalf("expected 3 calls total, got %d", repo.getActiveCalls.Load())
	}
}

func TestBuildOAuth2Client_NotConfigured(t *testing.T) {
	repo := &mockOAuth2ConfigRepo{
		activeErr: errors.New("record not found"),
	}
	svc := newTestOAuth2ConfigService(repo)

	client, err := svc.BuildOAuth2Client()
	if client != nil {
		t.Fatal("expected nil client")
	}
	if !errors.Is(err, ErrOAuth2NotConfigured) {
		t.Fatalf("expected ErrOAuth2NotConfigured, got %v", err)
	}

	// Error is cached
	callsBefore := repo.getActiveCalls.Load()
	client2, err2 := svc.BuildOAuth2Client()
	if client2 != nil {
		t.Fatal("expected nil client on second call")
	}
	if !errors.Is(err2, ErrOAuth2NotConfigured) {
		t.Fatalf("expected ErrOAuth2NotConfigured on second call, got %v", err2)
	}
	if repo.getActiveCalls.Load() != callsBefore {
		t.Fatalf("error should be cached, expected %d calls, got %d", callsBefore, repo.getActiveCalls.Load())
	}
}

func TestBuildOAuth2Client_DisabledConfig(t *testing.T) {
	repo := &mockOAuth2ConfigRepo{
		active: &model.DiskOAuth2Config{
			Enabled:   false,
			IssuerURL: "https://example.com",
		},
	}
	svc := newTestOAuth2ConfigService(repo)

	client, err := svc.BuildOAuth2Client()
	if client != nil {
		t.Fatal("expected nil client for disabled config")
	}
	if !errors.Is(err, ErrOAuth2NotConfigured) {
		t.Fatalf("expected ErrOAuth2NotConfigured, got %v", err)
	}
}

func TestBuildOAuth2Client_ErrorCacheClearedOnUpdate(t *testing.T) {
	repo := &mockOAuth2ConfigRepo{
		activeErr: errors.New("record not found"),
	}
	svc := newTestOAuth2ConfigService(repo)

	// First call: error cached
	_, err := svc.BuildOAuth2Client()
	if !errors.Is(err, ErrOAuth2NotConfigured) {
		t.Fatalf("expected ErrOAuth2NotConfigured, got %v", err)
	}

	// Simulate admin creates config in DB
	repo.activeErr = nil
	repo.active = &model.DiskOAuth2Config{
		Enabled:      true,
		ClientID:     "new-client",
		ClientSecret: "new-secret",
		IssuerURL:    "https://example.com",
		RedirectURL:  "https://app.example.com/callback",
		Scopes:       "openid",
	}

	// UpdateConfig clears error cache
	err = svc.UpdateConfig("admin", &model.DiskOAuth2Config{
		Enabled:      true,
		ClientID:     "new-client",
		ClientSecret: "new-secret",
		IssuerURL:    "https://example.com",
		RedirectURL:  "https://app.example.com/callback",
		Scopes:       "openid",
	})
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	// Now BuildOAuth2Client should succeed
	client, err := svc.BuildOAuth2Client()
	if err != nil {
		t.Fatalf("expected success after cache clear, got %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client after cache clear")
	}
}

func TestBuildOAuth2Client_EmptyIssuerURL(t *testing.T) {
	repo := &mockOAuth2ConfigRepo{
		active: &model.DiskOAuth2Config{
			Enabled:   true,
			IssuerURL: "",
		},
	}
	svc := newTestOAuth2ConfigService(repo)

	client, err := svc.BuildOAuth2Client()
	if client != nil {
		t.Fatal("expected nil client for empty issuer URL")
	}
	if err == nil {
		t.Fatal("expected error for empty issuer URL")
	}
}
