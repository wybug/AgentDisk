package service

import (
	"errors"
	"testing"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/go-webauthn/webauthn/protocol"
)

// ── Mock passkey repository ──

type mockPasskeyRepo struct {
	passkeys map[uint64]*model.DiskAdminPasskey
	nextID   uint64
	err      error
}

func newMockPasskeyRepo() *mockPasskeyRepo {
	return &mockPasskeyRepo{
		passkeys: make(map[uint64]*model.DiskAdminPasskey),
		nextID:   1,
	}
}

func (m *mockPasskeyRepo) ListByAdmin(username string) ([]model.DiskAdminPasskey, error) {
	if m.err != nil {
		return nil, m.err
	}
	var result []model.DiskAdminPasskey
	for _, pk := range m.passkeys {
		if pk.AdminUsername == username && pk.IsActive {
			result = append(result, *pk)
		}
	}
	return result, nil
}

func (m *mockPasskeyRepo) GetByCredentialID(credID string) (*model.DiskAdminPasskey, error) {
	if m.err != nil {
		return nil, m.err
	}
	for _, pk := range m.passkeys {
		if pk.CredentialID == credID && pk.IsActive {
			return pk, nil
		}
	}
	return nil, errors.New("not found")
}

func (m *mockPasskeyRepo) GetByID(id uint64) (*model.DiskAdminPasskey, error) {
	if m.err != nil {
		return nil, m.err
	}
	pk, ok := m.passkeys[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return pk, nil
}

func (m *mockPasskeyRepo) Create(passkey *model.DiskAdminPasskey) error {
	if m.err != nil {
		return m.err
	}
	passkey.ID = m.nextID
	m.nextID++
	m.passkeys[passkey.ID] = passkey
	return nil
}

func (m *mockPasskeyRepo) UpdateSignCount(id uint64, signCount uint32) error {
	if m.err != nil {
		return m.err
	}
	pk, ok := m.passkeys[id]
	if !ok {
		return errors.New("not found")
	}
	pk.SignCount = uint64(signCount)
	return nil
}

func (m *mockPasskeyRepo) UpdateLastUsed(_ uint64) error {
	return nil
}

func (m *mockPasskeyRepo) Delete(id uint64) error {
	if m.err != nil {
		return m.err
	}
	pk, ok := m.passkeys[id]
	if !ok {
		return errors.New("not found")
	}
	pk.IsActive = false
	return nil
}

func (m *mockPasskeyRepo) CountByAdmin(username string) (int64, error) {
	if m.err != nil {
		return 0, m.err
	}
	var count int64
	for _, pk := range m.passkeys {
		if pk.AdminUsername == username && pk.IsActive {
			count++
		}
	}
	return count, nil
}

func (m *mockPasskeyRepo) UpdateName(id uint64, name string) error {
	if m.err != nil {
		return m.err
	}
	pk, ok := m.passkeys[id]
	if !ok {
		return errors.New("not found")
	}
	pk.Name = name
	return nil
}

// ── Mock MFA admin repository ──

type mockMFAAdminRepo struct {
	mfaEnabled map[string]bool
	err        error
}

func newMockMFAAdminRepo() *mockMFAAdminRepo {
	return &mockMFAAdminRepo{
		mfaEnabled: make(map[string]bool),
	}
}

func (m *mockMFAAdminRepo) UpdateMFAEnabled(username string, enabled bool) error {
	if m.err != nil {
		return m.err
	}
	m.mfaEnabled[username] = enabled
	return nil
}

func (m *mockMFAAdminRepo) GetMFAEnabled(username string) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return m.mfaEnabled[username], nil
}

// ── SetMFAEnabled tests ──

func TestAdminMFAService_SetMFAEnabled_EnableWithPasskeys(t *testing.T) {
	passkeyRepo := newMockPasskeyRepo()
	passkeyRepo.Create(&model.DiskAdminPasskey{
		AdminUsername: "admin",
		CredentialID:  "cred1",
		IsActive:      true,
	})
	adminRepo := newMockMFAAdminRepo()
	svc := &AdminMFAService{passkeyRepo: passkeyRepo, adminRepo: adminRepo}

	err := svc.SetMFAEnabled("admin", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !adminRepo.mfaEnabled["admin"] {
		t.Error("expected MFA to be enabled")
	}
}

func TestAdminMFAService_SetMFAEnabled_EnableWithoutPasskeys(t *testing.T) {
	passkeyRepo := newMockPasskeyRepo()
	adminRepo := newMockMFAAdminRepo()
	svc := &AdminMFAService{passkeyRepo: passkeyRepo, adminRepo: adminRepo}

	err := svc.SetMFAEnabled("admin", true)
	if err == nil {
		t.Fatal("expected error when enabling MFA without passkeys")
	}
}

func TestAdminMFAService_SetMFAEnabled_Disable(t *testing.T) {
	passkeyRepo := newMockPasskeyRepo()
	adminRepo := newMockMFAAdminRepo()
	svc := &AdminMFAService{passkeyRepo: passkeyRepo, adminRepo: adminRepo}

	err := svc.SetMFAEnabled("admin", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if adminRepo.mfaEnabled["admin"] {
		t.Error("expected MFA to be disabled")
	}
}

func TestAdminMFAService_SetMFAEnabled_DBError(t *testing.T) {
	passkeyRepo := newMockPasskeyRepo()
	passkeyRepo.Create(&model.DiskAdminPasskey{
		AdminUsername: "admin",
		CredentialID:  "cred1",
		IsActive:      true,
	})
	adminRepo := newMockMFAAdminRepo()
	adminRepo.err = errors.New("db error")
	svc := &AdminMFAService{passkeyRepo: passkeyRepo, adminRepo: adminRepo}

	err := svc.SetMFAEnabled("admin", true)
	if err == nil {
		t.Fatal("expected error on DB failure")
	}
}

// ── GetMFAStatus tests ──

func TestAdminMFAService_GetMFAStatus_NoPasskeys(t *testing.T) {
	passkeyRepo := newMockPasskeyRepo()
	adminRepo := newMockMFAAdminRepo()
	svc := &AdminMFAService{passkeyRepo: passkeyRepo, adminRepo: adminRepo}

	count, enabled, err := svc.GetMFAStatus("admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 passkeys, got %d", count)
	}
	if enabled {
		t.Error("expected MFA disabled")
	}
}

func TestAdminMFAService_GetMFAStatus_WithPasskeys(t *testing.T) {
	passkeyRepo := newMockPasskeyRepo()
	passkeyRepo.Create(&model.DiskAdminPasskey{AdminUsername: "admin", IsActive: true})
	passkeyRepo.Create(&model.DiskAdminPasskey{AdminUsername: "admin", IsActive: true})
	adminRepo := newMockMFAAdminRepo()
	adminRepo.mfaEnabled["admin"] = true
	svc := &AdminMFAService{passkeyRepo: passkeyRepo, adminRepo: adminRepo}

	count, enabled, err := svc.GetMFAStatus("admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 passkeys, got %d", count)
	}
	if !enabled {
		t.Error("expected MFA enabled")
	}
}

// ── DeletePasskey tests ──

func TestAdminMFAService_DeletePasskey_Success(t *testing.T) {
	passkeyRepo := newMockPasskeyRepo()
	passkeyRepo.Create(&model.DiskAdminPasskey{AdminUsername: "admin", IsActive: true})
	adminRepo := newMockMFAAdminRepo()
	svc := &AdminMFAService{passkeyRepo: passkeyRepo, adminRepo: adminRepo}

	err := svc.DeletePasskey("admin", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAdminMFAService_DeletePasskey_NotFound(t *testing.T) {
	passkeyRepo := newMockPasskeyRepo()
	svc := &AdminMFAService{passkeyRepo: passkeyRepo}

	err := svc.DeletePasskey("admin", 999)
	if err == nil {
		t.Fatal("expected error for non-existent passkey")
	}
}

func TestAdminMFAService_DeletePasskey_WrongOwner(t *testing.T) {
	passkeyRepo := newMockPasskeyRepo()
	passkeyRepo.Create(&model.DiskAdminPasskey{AdminUsername: "other-admin", IsActive: true})
	svc := &AdminMFAService{passkeyRepo: passkeyRepo}

	err := svc.DeletePasskey("admin", 1)
	if err == nil {
		t.Fatal("expected error when deleting another admin's passkey")
	}
}

func TestAdminMFAService_DeletePasskey_LastPasskeyAutoDisablesMFA(t *testing.T) {
	passkeyRepo := newMockPasskeyRepo()
	passkeyRepo.Create(&model.DiskAdminPasskey{AdminUsername: "admin", IsActive: true})
	adminRepo := newMockMFAAdminRepo()
	adminRepo.mfaEnabled["admin"] = true
	svc := &AdminMFAService{passkeyRepo: passkeyRepo, adminRepo: adminRepo}

	err := svc.DeletePasskey("admin", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if adminRepo.mfaEnabled["admin"] {
		t.Error("MFA should be auto-disabled when last passkey is deleted")
	}
}

func TestAdminMFAService_DeletePasskey_NotLastPasskey(t *testing.T) {
	passkeyRepo := newMockPasskeyRepo()
	passkeyRepo.Create(&model.DiskAdminPasskey{AdminUsername: "admin", IsActive: true})
	passkeyRepo.Create(&model.DiskAdminPasskey{AdminUsername: "admin", IsActive: true})
	adminRepo := newMockMFAAdminRepo()
	adminRepo.mfaEnabled["admin"] = true
	svc := &AdminMFAService{passkeyRepo: passkeyRepo, adminRepo: adminRepo}

	err := svc.DeletePasskey("admin", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !adminRepo.mfaEnabled["admin"] {
		t.Error("MFA should remain enabled when other passkeys exist")
	}
}

// ── ListPasskeys tests ──

func TestAdminMFAService_ListPasskeys_Empty(t *testing.T) {
	passkeyRepo := newMockPasskeyRepo()
	svc := &AdminMFAService{passkeyRepo: passkeyRepo}

	keys, err := svc.ListPasskeys("admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected 0 passkeys, got %d", len(keys))
	}
}

func TestAdminMFAService_ListPasskeys_WithKeys(t *testing.T) {
	passkeyRepo := newMockPasskeyRepo()
	passkeyRepo.Create(&model.DiskAdminPasskey{
		AdminUsername: "admin",
		CredentialID:  "cred1",
		Name:          "MacBook",
		IsActive:      true,
	})
	svc := &AdminMFAService{passkeyRepo: passkeyRepo}

	keys, err := svc.ListPasskeys("admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected 1 passkey, got %d", len(keys))
	}
	if keys[0].Name != "MacBook" {
		t.Errorf("expected name MacBook, got %s", keys[0].Name)
	}
}

// ── deterministicUserID tests ──

func TestDeterministicUserID_SameInput(t *testing.T) {
	id1 := deterministicUserID("admin")
	id2 := deterministicUserID("admin")
	if string(id1) != string(id2) {
		t.Error("same input should produce same ID")
	}
}

func TestDeterministicUserID_DifferentInput(t *testing.T) {
	id1 := deterministicUserID("admin1")
	id2 := deterministicUserID("admin2")
	if string(id1) == string(id2) {
		t.Error("different input should produce different ID")
	}
}

func TestDeterministicUserID_Length(t *testing.T) {
	id := deterministicUserID("admin")
	if len(id) != 32 {
		t.Errorf("expected 32 bytes, got %d", len(id))
	}
}

// ── transport helpers tests ──

func TestParseTransport_Empty(t *testing.T) {
	result := parseTransport("")
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestParseTransport_Single(t *testing.T) {
	result := parseTransport("internal")
	if len(result) != 1 || string(result[0]) != "internal" {
		t.Errorf("expected [internal], got %v", result)
	}
}

func TestParseTransport_Multiple(t *testing.T) {
	result := parseTransport("internal, hybrid")
	if len(result) != 2 {
		t.Fatalf("expected 2 transports, got %d", len(result))
	}
	if string(result[0]) != "internal" {
		t.Errorf("expected internal, got %s", result[0])
	}
	if string(result[1]) != "hybrid" {
		t.Errorf("expected hybrid, got %s", result[1])
	}
}

func TestEncodeTransport(t *testing.T) {
	result := encodeTransport([]protocol.AuthenticatorTransport{"internal", "hybrid"})
	if result != "internal,hybrid" {
		t.Errorf("expected 'internal,hybrid', got %s", result)
	}
}

// ── RenamePasskey tests ──

func TestAdminMFAService_RenamePasskey_Success(t *testing.T) {
	passkeyRepo := newMockPasskeyRepo()
	passkeyRepo.Create(&model.DiskAdminPasskey{AdminUsername: "admin", Name: "Old", IsActive: true})
	svc := &AdminMFAService{passkeyRepo: passkeyRepo}

	err := svc.RenamePasskey("admin", 1, "New Name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if passkeyRepo.passkeys[1].Name != "New Name" {
		t.Errorf("expected name 'New Name', got %s", passkeyRepo.passkeys[1].Name)
	}
}

func TestAdminMFAService_RenamePasskey_NotFound(t *testing.T) {
	passkeyRepo := newMockPasskeyRepo()
	svc := &AdminMFAService{passkeyRepo: passkeyRepo}

	err := svc.RenamePasskey("admin", 999, "Name")
	if err == nil {
		t.Fatal("expected error for non-existent passkey")
	}
}

func TestAdminMFAService_RenamePasskey_WrongOwner(t *testing.T) {
	passkeyRepo := newMockPasskeyRepo()
	passkeyRepo.Create(&model.DiskAdminPasskey{AdminUsername: "other-admin", IsActive: true})
	svc := &AdminMFAService{passkeyRepo: passkeyRepo}

	err := svc.RenamePasskey("admin", 1, "Name")
	if err == nil {
		t.Fatal("expected error when renaming another admin's passkey")
	}
}
