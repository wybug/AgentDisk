package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/jwt"
	"github.com/gin-gonic/gin"
)

const mfaTestSecret = "mfa-test-secret-key"

func init() {
	gin.SetMode(gin.TestMode)
}

type mockAdminService struct {
	loginAdmin *model.DiskAdminUser
	loginErr   error
}

func (m *mockAdminService) Login(_, _ string) (*model.DiskAdminUser, error) {
	if m.loginErr != nil {
		return nil, m.loginErr
	}
	return m.loginAdmin, nil
}

func (m *mockAdminService) Count() (int64, error) { return 0, nil }

func (m *mockAdminService) CreateAdmin(username, _, role, displayName, createdBy string) (*model.DiskAdminUser, error) {
	return &model.DiskAdminUser{
		Username:    username,
		Role:        role,
		DisplayName: displayName,
		CreatedBy:   createdBy,
	}, nil
}

func (m *mockAdminService) ListAdmins() ([]model.DiskAdminUser, error) { return nil, nil }

func (m *mockAdminService) ChangePassword(_, _ string) error { return nil }

func (m *mockAdminService) DeleteAdmin(_ string) error { return nil }

type mockAdminMFAService struct {
	hasPasskeys bool
	err         error
}

func (m *mockAdminMFAService) HasPasskeys(_ string) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	return m.hasPasskeys, nil
}

// ── DeletePasskey handler validation ──

func TestAdminMFAHandler_DeletePasskey_InvalidID(t *testing.T) {
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("adminUser", "admin") })

	mfaH := &AdminMFAHandler{jwtSecret: mfaTestSecret}
	r.DELETE("/mfa/credentials/:id", mfaH.DeletePasskey)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/mfa/credentials/invalid", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid id, got %d", w.Code)
	}
}

// ── Login handler MFA branch tests ──

func TestAdminHandler_Login_MissingFields(t *testing.T) {
	h := NewAdminHandler(nil, mfaTestSecret, 24)
	r := gin.New()
	r.POST("/login", h.Login)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/login", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// ── MFA Login handler validation tests ──

func TestAdminMFAHandler_BeginMFALogin_MissingToken(t *testing.T) {
	mfaH := &AdminMFAHandler{jwtSecret: mfaTestSecret}
	r := gin.New()
	r.POST("/mfa/login/begin", mfaH.BeginMFALogin)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/mfa/login/begin", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestAdminMFAHandler_BeginMFALogin_InvalidToken(t *testing.T) {
	mfaH := &AdminMFAHandler{jwtSecret: mfaTestSecret}
	r := gin.New()
	r.POST("/mfa/login/begin", mfaH.BeginMFALogin)

	body, _ := json.Marshal(map[string]string{"sessionToken": "invalid-token"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/mfa/login/begin", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAdminMFAHandler_BeginMFALogin_AdminTokenRejected(t *testing.T) {
	// A regular admin JWT should not be accepted as MFA session token
	adminToken, _ := jwt.GenerateAdminToken(mfaTestSecret, "admin", "admin", 24)

	mfaH := &AdminMFAHandler{jwtSecret: mfaTestSecret}
	r := gin.New()
	r.POST("/mfa/login/begin", mfaH.BeginMFALogin)

	body, _ := json.Marshal(map[string]string{"sessionToken": adminToken})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/mfa/login/begin", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for admin token, got %d", w.Code)
	}
}

func TestAdminMFAHandler_FinishMFALogin_MissingFields(t *testing.T) {
	mfaH := &AdminMFAHandler{jwtSecret: mfaTestSecret}
	r := gin.New()
	r.POST("/mfa/login/finish", mfaH.FinishMFALogin)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/mfa/login/finish", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestAdminMFAHandler_FinishRegistration_MissingFields(t *testing.T) {
	mfaH := &AdminMFAHandler{jwtSecret: mfaTestSecret}
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("adminUser", "admin") })
	r.POST("/mfa/registration/finish", mfaH.FinishRegistration)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/mfa/registration/finish", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestAdminMFAHandler_SetMFAEnabled_InvalidBody(t *testing.T) {
	mfaH := &AdminMFAHandler{jwtSecret: mfaTestSecret}
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("adminUser", "admin") })
	r.PUT("/mfa/enabled", mfaH.SetMFAEnabled)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/mfa/enabled", bytes.NewReader([]byte(`not json`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// ── MFA Session Token e2e through handler ──

func TestMFASessionToken_ValidTokenNotRejected(t *testing.T) {
	// Generate a session token
	token, err := jwt.GenerateMFASessionToken(mfaTestSecret, "admin")
	if err != nil {
		t.Fatalf("GenerateMFASessionToken failed: %v", err)
	}

	// Verify the token parses correctly (this is the key test - handler integration
	// requires a full WebAuthn service which we test via browser tests)
	claims, err := jwt.ParseMFASessionToken(mfaTestSecret, token)
	if err != nil {
		t.Fatalf("ParseMFASessionToken failed: %v", err)
	}
	if claims.Username != "admin" {
		t.Errorf("expected admin, got %s", claims.Username)
	}
	if !claims.MFAPending {
		t.Error("expected MFAPending=true")
	}
}

// ── MFA with service mock ──

func TestAdminHandler_Login_WithMFACheck(t *testing.T) {
	// Test that when mfaSvc is nil, login works normally (no MFA)
	h := NewAdminHandler(nil, mfaTestSecret, 24)
	if h.mfaSvc != nil {
		t.Error("mfaSvc should be nil by default")
	}
}

func TestAdminHandler_SetMFAService(t *testing.T) {
	h := NewAdminHandler(nil, mfaTestSecret, 24)
	if h.mfaSvc != nil {
		t.Error("mfaSvc should be nil initially")
	}

	h.SetMFAService(&service.AdminMFAService{})
	if h.mfaSvc == nil {
		t.Error("mfaSvc should not be nil after SetMFAService")
	}
}

func TestAdminHandler_Login_MFAEnabledWithoutPasskeysFallsBackToPasswordLogin(t *testing.T) {
	adminSvc := &mockAdminService{
		loginAdmin: &model.DiskAdminUser{
			Username:   "admin",
			Role:       "super_admin",
			MfaEnabled: true,
		},
	}
	h := NewAdminHandler(adminSvc, mfaTestSecret, 24)
	h.SetMFAService(&mockAdminMFAService{hasPasskeys: false})

	r := gin.New()
	r.POST("/login", h.Login)

	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "secret"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Data struct {
			Token       string `json:"token"`
			Role        string `json:"role"`
			MFARequired bool   `json:"mfaRequired"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Data.MFARequired {
		t.Fatal("did not expect MFA to be required without passkeys")
	}
	if resp.Data.Token == "" {
		t.Fatal("expected password login token when no passkeys exist")
	}
	if resp.Data.Role != "super_admin" {
		t.Fatalf("expected role super_admin, got %s", resp.Data.Role)
	}
}

func TestAdminHandler_Login_MFAEnabledWithPasskeysRequiresMFA(t *testing.T) {
	adminSvc := &mockAdminService{
		loginAdmin: &model.DiskAdminUser{
			Username:   "admin",
			Role:       "admin",
			MfaEnabled: true,
		},
	}
	h := NewAdminHandler(adminSvc, mfaTestSecret, 24)
	h.SetMFAService(&mockAdminMFAService{hasPasskeys: true})

	r := gin.New()
	r.POST("/login", h.Login)

	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "secret"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Data struct {
			SessionToken string `json:"sessionToken"`
			MFARequired  bool   `json:"mfaRequired"`
			Token        string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Data.MFARequired {
		t.Fatal("expected MFA to be required when passkeys exist")
	}
	if resp.Data.SessionToken == "" {
		t.Fatal("expected MFA session token")
	}
	if resp.Data.Token != "" {
		t.Fatal("should not issue admin token before MFA finishes")
	}
}

func TestAdminHandler_Login_MFAStatusCheckError(t *testing.T) {
	adminSvc := &mockAdminService{
		loginAdmin: &model.DiskAdminUser{
			Username:   "admin",
			Role:       "admin",
			MfaEnabled: true,
		},
	}
	h := NewAdminHandler(adminSvc, mfaTestSecret, 24)
	h.SetMFAService(&mockAdminMFAService{err: errors.New("db error")})

	r := gin.New()
	r.POST("/login", h.Login)

	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "secret"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}
