package service

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/agentdisk/agent-disk/config"
	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
	"github.com/agentdisk/agent-disk/internal/store"
	"github.com/agentdisk/agent-disk/pkg/jwt"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

// adminPasskeyRepo defines the interface for passkey data access.
type adminPasskeyRepo interface {
	ListByAdmin(username string) ([]model.DiskAdminPasskey, error)
	GetByCredentialID(credID string) (*model.DiskAdminPasskey, error)
	GetByID(id uint64) (*model.DiskAdminPasskey, error)
	Create(passkey *model.DiskAdminPasskey) error
	UpdateSignCount(id uint64, signCount uint32) error
	UpdateLastUsed(id uint64) error
	Delete(id uint64) error
	CountByAdmin(username string) (int64, error)
	UpdateName(id uint64, name string) error
}

// adminMFARepo defines the interface for admin MFA flag updates.
type adminMFARepo interface {
	UpdateMFAEnabled(username string, enabled bool) error
	GetMFAEnabled(username string) (bool, error)
}

// AdminMFAService handles WebAuthn MFA operations.
type AdminMFAService struct {
	webAuthn     *webauthn.WebAuthn
	passkeyRepo  adminPasskeyRepo
	adminRepo    adminMFARepo
	sessionStore *store.WebAuthnSessionStore
	jwtSecret    string
	jwtExpireHrs int
}

// adminUserAdapter adapts admin data to the webauthn.User interface.
type adminUserAdapter struct {
	username    string
	id          []byte
	credentials []webauthn.Credential
}

// WebAuthnID provides the user handle.
func (u *adminUserAdapter) WebAuthnID() []byte { return u.id }

// WebAuthnName provides the user name.
func (u *adminUserAdapter) WebAuthnName() string { return u.username }

// WebAuthnDisplayName provides the display name.
func (u *adminUserAdapter) WebAuthnDisplayName() string { return u.username }

// WebAuthnCredentials provides the user credentials.
func (u *adminUserAdapter) WebAuthnCredentials() []webauthn.Credential { return u.credentials }

// NewAdminMFAService creates a new AdminMFAService.
func NewAdminMFAService(cfg config.WebAuthnConfig, passkeyRepo *repository.AdminPasskeyRepo, adminRepo *repository.AdminRepo, sessionStore *store.WebAuthnSessionStore, jwtSecret string, jwtExpireHrs int) (*AdminMFAService, error) {
	origins := strings.Split(cfg.RPOrigins, ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}

	wconfig := &webauthn.Config{
		RPDisplayName: cfg.RPDisplayName,
		RPID:          cfg.RPID,
		RPOrigins:     origins,
		Timeouts: webauthn.TimeoutsConfig{
			Login:        webauthn.TimeoutConfig{Enforce: true, Timeout: time.Duration(cfg.Timeout) * time.Millisecond},
			Registration: webauthn.TimeoutConfig{Enforce: true, Timeout: time.Duration(cfg.Timeout) * time.Millisecond},
		},
	}

	w, err := webauthn.New(wconfig)
	if err != nil {
		return nil, err
	}

	return &AdminMFAService{
		webAuthn:     w,
		passkeyRepo:  passkeyRepo,
		adminRepo:    adminRepo,
		sessionStore: sessionStore,
		jwtSecret:    jwtSecret,
		jwtExpireHrs: jwtExpireHrs,
	}, nil
}

// userAdapter creates a webauthn.User adapter for the given admin username.
func (s *AdminMFAService) userAdapter(username string) (*adminUserAdapter, error) {
	keys, err := s.passkeyRepo.ListByAdmin(username)
	if err != nil {
		return nil, err
	}
	creds := make([]webauthn.Credential, 0, len(keys))
	for _, k := range keys {
		credID, err := base64.RawURLEncoding.DecodeString(k.CredentialID)
		if err != nil {
			continue
		}
		creds = append(creds, webauthn.Credential{
			ID:              credID,
			PublicKey:       k.PublicKey,
			AttestationType: k.AttestationType,
			Transport:       parseTransport(k.Transport),
			Flags: webauthn.CredentialFlags{
				BackupEligible: k.BackupEligible,
				BackupState:    k.BackupState,
			},
		})
	}
	return &adminUserAdapter{
		username:    username,
		id:          deterministicUserID(username),
		credentials: creds,
	}, nil
}

func deterministicUserID(username string) []byte {
	buf := make([]byte, 32)
	copy(buf, username)
	return buf
}

func parseTransport(s string) []protocol.AuthenticatorTransport {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	transports := make([]protocol.AuthenticatorTransport, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			transports = append(transports, protocol.AuthenticatorTransport(p))
		}
	}
	return transports
}

func encodeTransport(t []protocol.AuthenticatorTransport) string {
	parts := make([]string, len(t))
	for i, v := range t {
		parts[i] = string(v)
	}
	return strings.Join(parts, ",")
}

// BeginRegistration starts the WebAuthn registration flow.
func (s *AdminMFAService) BeginRegistration(username string) (*protocol.CredentialCreation, string, error) {
	user, err := s.userAdapter(username)
	if err != nil {
		return nil, "", err
	}

	options, session, err := s.webAuthn.BeginRegistration(user)
	if err != nil {
		return nil, "", err
	}

	sessionKey := generateSessionKey()
	s.sessionStore.Set(sessionKey, session, 5*time.Minute)
	return options, sessionKey, nil
}

// FinishRegistration completes the WebAuthn registration flow.
func (s *AdminMFAService) FinishRegistration(username, sessionKey, name string, response *http.Request) (*model.DiskAdminPasskey, error) {
	session, ok := s.sessionStore.Get(sessionKey)
	if !ok {
		return nil, errors.New("session expired or not found")
	}
	defer s.sessionStore.Delete(sessionKey)

	user, err := s.userAdapter(username)
	if err != nil {
		return nil, err
	}

	credential, err := s.webAuthn.FinishRegistration(user, *session, response)
	if err != nil {
		return nil, err
	}

	passkey := &model.DiskAdminPasskey{
		AdminUsername:   username,
		CredentialID:    base64.RawURLEncoding.EncodeToString(credential.ID),
		PublicKey:       credential.PublicKey,
		AttestationType: credential.AttestationType,
		Transport:       encodeTransport(credential.Transport),
		Name:            name,
		AAGUID:          credential.Authenticator.AAGUID,
		BackupEligible:  credential.Flags.BackupEligible,
		BackupState:     credential.Flags.BackupState,
		IsActive:        true,
	}

	if err := s.passkeyRepo.Create(passkey); err != nil {
		return nil, err
	}

	return passkey, nil
}

// BeginLogin starts the WebAuthn login flow.
func (s *AdminMFAService) BeginLogin(username string) (*protocol.CredentialAssertion, string, error) {
	user, err := s.userAdapter(username)
	if err != nil {
		return nil, "", err
	}
	if len(user.credentials) == 0 {
		return nil, "", errors.New("no passkeys registered")
	}

	options, session, err := s.webAuthn.BeginLogin(user)
	if err != nil {
		return nil, "", err
	}

	sessionKey := generateSessionKey()
	s.sessionStore.Set(sessionKey, session, 5*time.Minute)
	return options, sessionKey, nil
}

// FinishLogin completes the WebAuthn login flow and returns a JWT.
func (s *AdminMFAService) FinishLogin(sessionKey string, response *http.Request) (token, username string, err error) {
	session, ok := s.sessionStore.Get(sessionKey)
	if !ok {
		return "", "", errors.New("session expired or not found")
	}
	defer s.sessionStore.Delete(sessionKey)

	username = string(session.UserID)
	if username == "" {
		return "", "", errors.New("invalid session")
	}

	user, err := s.userAdapter(username)
	if err != nil {
		return "", "", err
	}

	credential, err := s.webAuthn.FinishLogin(user, *session, response)
	if err != nil {
		return "", "", err
	}

	credIDStr := base64.RawURLEncoding.EncodeToString(credential.ID)
	pk, err := s.passkeyRepo.GetByCredentialID(credIDStr)
	if err != nil {
		return "", "", err
	}

	_ = s.passkeyRepo.UpdateSignCount(pk.ID, credential.Authenticator.SignCount)
	_ = s.passkeyRepo.UpdateLastUsed(pk.ID)

	token, err = jwt.GenerateAdminToken(s.jwtSecret, username, "admin", s.jwtExpireHrs)
	if err != nil {
		return "", "", err
	}

	return token, username, nil
}

// ListPasskeys returns passkeys for an admin.
func (s *AdminMFAService) ListPasskeys(username string) ([]model.DiskAdminPasskey, error) {
	return s.passkeyRepo.ListByAdmin(username)
}

// DeletePasskey deletes a passkey and auto-disables MFA if it was the last one.
func (s *AdminMFAService) DeletePasskey(username string, id uint64) error {
	pk, err := s.passkeyRepo.GetByID(id)
	if err != nil {
		return errors.New("passkey not found")
	}
	if pk.AdminUsername != username {
		return errors.New("passkey does not belong to this admin")
	}

	if delErr := s.passkeyRepo.Delete(id); delErr != nil {
		return delErr
	}

	count, countErr := s.passkeyRepo.CountByAdmin(username)
	if countErr != nil {
		return countErr
	}
	if count == 0 {
		_ = s.adminRepo.UpdateMFAEnabled(username, false)
	}

	return nil
}

// GetMFAStatus returns the MFA status for an admin.
func (s *AdminMFAService) GetMFAStatus(username string) (passkeyCount int64, mfaEnabled bool, err error) {
	count, err := s.passkeyRepo.CountByAdmin(username)
	if err != nil {
		return 0, false, err
	}
	enabled, err := s.adminRepo.GetMFAEnabled(username)
	if err != nil {
		return count, false, err
	}
	return count, enabled, nil
}

// SetMFAEnabled enables or disables MFA for an admin.
func (s *AdminMFAService) SetMFAEnabled(username string, enabled bool) error {
	if enabled {
		count, err := s.passkeyRepo.CountByAdmin(username)
		if err != nil {
			return err
		}
		if count == 0 {
			return errors.New("at least one passkey required to enable MFA")
		}
	}
	return s.adminRepo.UpdateMFAEnabled(username, enabled)
}

// HasPasskeys reports whether the admin has at least one active passkey.
func (s *AdminMFAService) HasPasskeys(username string) (bool, error) {
	count, err := s.passkeyRepo.CountByAdmin(username)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// RenamePasskey updates the display name of a passkey.
func (s *AdminMFAService) RenamePasskey(username string, id uint64, name string) error {
	pk, err := s.passkeyRepo.GetByID(id)
	if err != nil {
		return errors.New("passkey not found")
	}
	if pk.AdminUsername != username {
		return errors.New("passkey does not belong to this admin")
	}
	return s.passkeyRepo.UpdateName(id, name)
}

func generateSessionKey() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
