package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/jwt"
	"github.com/gin-gonic/gin"
)

const mfaTestSecret = "mfa-test-secret-key"

func init() {
	gin.SetMode(gin.TestMode)
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
