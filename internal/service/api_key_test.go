package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/agentdisk/agent-disk/internal/model"
)

var errTestNotFound = errors.New("not found")

// mockAPIKeyRepo implements apiKeyRepo for testing.
type mockAPIKeyRepo struct {
	keys   map[string]*model.DiskAPIKey
	nextID uint64
}

func newMockAPIKeyRepo() *mockAPIKeyRepo {
	return &mockAPIKeyRepo{
		keys:   make(map[string]*model.DiskAPIKey),
		nextID: 1,
	}
}

func (m *mockAPIKeyRepo) Create(key *model.DiskAPIKey) error {
	key.ID = m.nextID
	m.nextID++
	m.keys[key.KeyHash] = key
	return nil
}

func (m *mockAPIKeyRepo) GetByHash(hash string) (*model.DiskAPIKey, error) {
	if k, ok := m.keys[hash]; ok {
		return k, nil
	}
	return nil, errTestNotFound
}

func (m *mockAPIKeyRepo) List() ([]model.DiskAPIKey, error) {
	var result []model.DiskAPIKey
	for _, k := range m.keys {
		result = append(result, *k)
	}
	return result, nil
}

func (m *mockAPIKeyRepo) Revoke(id uint64) error {
	for _, k := range m.keys {
		if k.ID == id {
			k.IsRevoked = true
			return nil
		}
	}
	return errTestNotFound
}

func (m *mockAPIKeyRepo) UpdateLastUsed(id uint64) error {
	for _, k := range m.keys {
		if k.ID == id {
			now := time.Now()
			k.LastUsedAt = &now
			return nil
		}
	}
	return errTestNotFound
}

func (m *mockAPIKeyRepo) Update(id uint64, updates map[string]interface{}) error {
	for _, k := range m.keys {
		if k.ID == id {
			if name, ok := updates["key_name"].(string); ok {
				k.KeyName = name
			}
			return nil
		}
	}
	return errTestNotFound
}

func TestAPIKeyService_Create(t *testing.T) {
	repo := newMockAPIKeyRepo()
	svc := NewAPIKeyServiceFromRepo(repo)

	rawKey, record, err := svc.CreateAPIKey("test-key", "engineering", "admin")
	if err != nil {
		t.Fatalf("CreateAPIKey failed: %v", err)
	}

	if !strings.HasPrefix(rawKey, "adk_") {
		t.Errorf("key should have adk_ prefix, got %s", rawKey[:10])
	}
	if len(rawKey) != 68 { // adk_ (4) + 64 hex chars
		t.Errorf("key length = %d, want 68", len(rawKey))
	}
	if record.KeyName != "test-key" {
		t.Errorf("keyName = %q, want test-key", record.KeyName)
	}
	if record.Scope != "public_read" {
		t.Errorf("scope = %q, want public_read", record.Scope)
	}
	if record.Department != "engineering" {
		t.Errorf("department = %q, want engineering", record.Department)
	}
	if record.IsRevoked {
		t.Error("new key should not be revoked")
	}
	if record.KeyPrefix != rawKey[:8] {
		t.Errorf("prefix = %q, want %q", record.KeyPrefix, rawKey[:8])
	}
}

func TestAPIKeyService_Validate(t *testing.T) {
	repo := newMockAPIKeyRepo()
	svc := NewAPIKeyServiceFromRepo(repo)

	rawKey, _, _ := svc.CreateAPIKey("test-key", "", "admin")

	validated, err := svc.ValidateKey(rawKey)
	if err != nil {
		t.Fatalf("ValidateKey failed: %v", err)
	}
	if validated.KeyName != "test-key" {
		t.Errorf("keyName = %q, want test-key", validated.KeyName)
	}
}

func TestAPIKeyService_ValidateWrongKey(t *testing.T) {
	repo := newMockAPIKeyRepo()
	svc := NewAPIKeyServiceFromRepo(repo)

	svc.CreateAPIKey("test-key", "", "admin")

	_, err := svc.ValidateKey("adk_wrongkey000000000000000000000000000000000000000000000000000000000000")
	if err == nil {
		t.Error("expected error for wrong key")
	}
}

func TestAPIKeyService_ValidateRevokedKey(t *testing.T) {
	repo := newMockAPIKeyRepo()
	svc := NewAPIKeyServiceFromRepo(repo)

	rawKey, record, _ := svc.CreateAPIKey("test-key", "", "admin")
	svc.RevokeAPIKey(record.ID)

	_, err := svc.ValidateKey(rawKey)
	if err == nil {
		t.Error("expected error for revoked key")
	}
}

func TestAPIKeyService_ValidateExpiredKey(t *testing.T) {
	repo := newMockAPIKeyRepo()
	svc := NewAPIKeyServiceFromRepo(repo)

	rawKey, record, _ := svc.CreateAPIKey("test-key", "", "admin")
	// Simulate expiration
	past := time.Now().Add(-1 * time.Hour)
	for _, k := range repo.keys {
		k.ExpiresAt = &past
	}
	_ = record

	_, err := svc.ValidateKey(rawKey)
	if err == nil {
		t.Error("expected error for expired key")
	}
}

func TestAPIKeyService_Revoke(t *testing.T) {
	repo := newMockAPIKeyRepo()
	svc := NewAPIKeyServiceFromRepo(repo)

	_, record, _ := svc.CreateAPIKey("test-key", "", "admin")
	err := svc.RevokeAPIKey(record.ID)
	if err != nil {
		t.Fatalf("RevokeAPIKey failed: %v", err)
	}

	keys, _ := svc.ListAPIKeys()
	if !keys[0].IsRevoked {
		t.Error("key should be revoked")
	}
}

func TestAPIKeyService_List(t *testing.T) {
	repo := newMockAPIKeyRepo()
	svc := NewAPIKeyServiceFromRepo(repo)

	svc.CreateAPIKey("key1", "", "admin")
	svc.CreateAPIKey("key2", "marketing", "admin")

	keys, err := svc.ListAPIKeys()
	if err != nil {
		t.Fatalf("ListAPIKeys failed: %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("got %d keys, want 2", len(keys))
	}
}

func TestAPIKeyService_ValidateShortKey(t *testing.T) {
	repo := newMockAPIKeyRepo()
	svc := NewAPIKeyServiceFromRepo(repo)

	_, err := svc.ValidateKey("short")
	if err == nil {
		t.Error("expected error for short key")
	}
}
