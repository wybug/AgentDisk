package handler

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agentdisk/agent-disk/pkg/oauth2client"
	"github.com/gin-gonic/gin"
)

func setupAuthRouter(authH *AuthHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/auth/login", authH.Login)
	r.GET("/auth/callback", authH.Callback)
	r.POST("/auth/logout", authH.Logout)
	return r
}

func newTestAuthHandler(tokenURL, userinfoURL string) *AuthHandler {
	client := oauth2client.New(oauth2client.Config{
		ClientID:     "agentdisk",
		ClientSecret: "secret",
		AuthURL:      "https://example.com/authorize",
		TokenURL:     tokenURL,
		UserInfoURL:  userinfoURL,
		RedirectURL:  "https://disk.example.com/auth/callback",
	})
	return NewAuthHandler(client)
}

// ── NewAuthHandler ──

func TestNewAuthHandler(t *testing.T) {
	h := NewAuthHandler(nil)
	if h == nil {
		t.Fatal("NewAuthHandler returned nil")
	}
	if h.cookieName != "agentdisk_session" {
		t.Errorf("cookieName = %q, want agentdisk_session", h.cookieName)
	}
	if h.cookieMaxAge != 86400 {
		t.Errorf("cookieMaxAge = %d, want 86400", h.cookieMaxAge)
	}
	if h.sessions == nil {
		t.Error("sessions map is nil")
	}
}

// ── Login ──

func TestAuth_Login_OAuth2NotConfigured(t *testing.T) {
	h := NewAuthHandler(nil)
	r := setupAuthRouter(h)

	req := httptest.NewRequest("GET", "/auth/login", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
}

func TestAuth_Login_Success(t *testing.T) {
	h := newTestAuthHandler("https://example.com/token", "https://example.com/userinfo")
	r := setupAuthRouter(h)

	req := httptest.NewRequest("GET", "/auth/login", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", w.Code)
	}

	location := w.Header().Get("Location")
	if !strings.Contains(location, "client_id=agentdisk") {
		t.Errorf("Location should contain client_id, got %s", location)
	}
	if !strings.Contains(location, "code_challenge_method=S256") {
		t.Error("Location should contain PKCE S256 challenge")
	}

	// Should set oauth2_state cookie
	cookies := w.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "oauth2_state" {
			found = true
			if c.MaxAge != 600 {
				t.Errorf("oauth2_state MaxAge = %d, want 600", c.MaxAge)
			}
		}
	}
	if !found {
		t.Error("oauth2_state cookie not set")
	}
}

func TestAuth_Login_PromptNone(t *testing.T) {
	h := newTestAuthHandler("https://example.com/token", "https://example.com/userinfo")
	r := setupAuthRouter(h)

	req := httptest.NewRequest("GET", "/auth/login?prompt=none", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	location := w.Header().Get("Location")
	if !strings.Contains(location, "prompt=none") {
		t.Errorf("Location should contain prompt=none, got %s", location)
	}
}

func TestAuth_Login_FromGateway(t *testing.T) {
	h := newTestAuthHandler("https://example.com/token", "https://example.com/userinfo")
	r := setupAuthRouter(h)

	req := httptest.NewRequest("GET", "/auth/login?from=gateway", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	location := w.Header().Get("Location")
	if !strings.Contains(location, "prompt=none") {
		t.Errorf("from=gateway should trigger prompt=none, got %s", location)
	}
}

func TestAuth_Login_StandardFlow(t *testing.T) {
	h := newTestAuthHandler("https://example.com/token", "https://example.com/userinfo")
	r := setupAuthRouter(h)

	req := httptest.NewRequest("GET", "/auth/login", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	location := w.Header().Get("Location")
	if strings.Contains(location, "prompt=none") {
		t.Error("standard login should NOT contain prompt=none")
	}
}

// ── Callback ──

func TestAuth_Callback_OAuth2NotConfigured(t *testing.T) {
	h := NewAuthHandler(nil)
	r := setupAuthRouter(h)

	req := httptest.NewRequest("GET", "/auth/callback?code=abc&state=xyz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
}

func TestAuth_Callback_MissingCode(t *testing.T) {
	h := newTestAuthHandler("https://example.com/token", "https://example.com/userinfo")
	r := setupAuthRouter(h)

	req := httptest.NewRequest("GET", "/auth/callback?state=xyz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestAuth_Callback_MissingStateCookie(t *testing.T) {
	h := newTestAuthHandler("https://example.com/token", "https://example.com/userinfo")
	r := setupAuthRouter(h)

	req := httptest.NewRequest("GET", "/auth/callback?code=abc&state=xyz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (missing state cookie)", w.Code)
	}
}

func TestAuth_Callback_StateMismatch(t *testing.T) {
	h := newTestAuthHandler("https://example.com/token", "https://example.com/userinfo")
	r := setupAuthRouter(h)

	// First, do a login to get a valid state cookie
	loginReq := httptest.NewRequest("GET", "/auth/login", nil)
	loginW := httptest.NewRecorder()
	r.ServeHTTP(loginW, loginReq)

	// Extract state cookie
	var stateCookie *http.Cookie
	for _, c := range loginW.Result().Cookies() {
		if c.Name == "oauth2_state" {
			stateCookie = c
		}
	}
	if stateCookie == nil {
		t.Fatal("oauth2_state cookie not set during login")
	}

	// Decode state to verify it's different from what we'll send
	stateDataBytes, _ := base64.RawURLEncoding.DecodeString(stateCookie.Value)
	var stateData struct {
		State    string `json:"state"`
		Verifier string `json:"verifier"`
	}
	json.Unmarshal(stateDataBytes, &stateData)

	// Send callback with wrong state
	req := httptest.NewRequest("GET", "/auth/callback?code=abc&state=wrong_state", nil)
	req.AddCookie(stateCookie)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (state mismatch)", w.Code)
	}
}

func TestAuth_Callback_OAuth2Error(t *testing.T) {
	h := newTestAuthHandler("https://example.com/token", "https://example.com/userinfo")
	r := setupAuthRouter(h)

	req := httptest.NewRequest("GET", "/auth/callback?error=access_denied", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (access_denied)", w.Code)
	}
}

func TestAuth_Callback_LoginRequiredRedirect(t *testing.T) {
	h := newTestAuthHandler("https://example.com/token", "https://example.com/userinfo")
	r := setupAuthRouter(h)

	req := httptest.NewRequest("GET", "/auth/callback?error=login_required", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// login_required should redirect to standard login
	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302 (redirect to login)", w.Code)
	}
}

func TestAuth_Callback_InvalidStateData(t *testing.T) {
	h := newTestAuthHandler("https://example.com/token", "https://example.com/userinfo")
	r := setupAuthRouter(h)

	req := httptest.NewRequest("GET", "/auth/callback?code=abc&state=test_state", nil)
	// Set invalid base64 state cookie
	req.AddCookie(&http.Cookie{
		Name:  "oauth2_state",
		Value: base64.RawURLEncoding.EncodeToString([]byte("not valid json")),
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (invalid state data)", w.Code)
	}
}

// ── Full OAuth2 flow with mock server ──

func TestAuth_Callback_FullFlow(t *testing.T) {
	// Create mock OAuth2 server
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token":  "mock_access_token",
				"token_type":    "Bearer",
				"expires_in":    3600,
				"refresh_token": "mock_refresh",
			})
		case "/oauth2/userinfo":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"userId":   "user_fullflow_001",
				"userName": "Full Flow User",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	h := newTestAuthHandler(srv.URL+"/oauth2/token", srv.URL+"/oauth2/userinfo")
	r := setupAuthRouter(h)

	// Step 1: Login to get state cookie
	loginReq := httptest.NewRequest("GET", "/auth/login", nil)
	loginW := httptest.NewRecorder()
	r.ServeHTTP(loginW, loginReq)

	var stateCookie *http.Cookie
	for _, c := range loginW.Result().Cookies() {
		if c.Name == "oauth2_state" {
			stateCookie = c
		}
	}

	// Decode state to get the state value
	stateDataBytes, _ := base64.RawURLEncoding.DecodeString(stateCookie.Value)
	var stateData struct {
		State    string `json:"state"`
		Verifier string `json:"verifier"`
	}
	json.Unmarshal(stateDataBytes, &stateData)

	// Step 2: Callback with valid state
	callbackURL := "/auth/callback?code=mock_code&state=" + stateData.State
	callbackReq := httptest.NewRequest("GET", callbackURL, nil)
	callbackReq.AddCookie(stateCookie)
	callbackW := httptest.NewRecorder()
	r.ServeHTTP(callbackW, callbackReq)

	if callbackW.Code != http.StatusFound {
		t.Fatalf("callback status = %d, want 302, body=%s", callbackW.Code, callbackW.Body.String())
	}

	// Should redirect to /
	if callbackW.Header().Get("Location") != "/" {
		t.Errorf("Location = %q, want /", callbackW.Header().Get("Location"))
	}

	// Should set session cookie
	var sessionCookie *http.Cookie
	for _, c := range callbackW.Result().Cookies() {
		if c.Name == "agentdisk_session" {
			sessionCookie = c
		}
	}
	if sessionCookie == nil {
		t.Fatal("agentdisk_session cookie not set")
	}

	// Verify session was stored
	sess := h.GetSession(sessionCookie.Value)
	if sess == nil {
		t.Fatal("GetSession returned nil for valid session")
	}
	if sess.UserID != "user_fullflow_001" {
		t.Errorf("session UserID = %q, want user_fullflow_001", sess.UserID)
	}
	if sess.UserName != "Full Flow User" {
		t.Errorf("session UserName = %q, want Full Flow User", sess.UserName)
	}

	// Verify state cookie was cleared
	var clearedState bool
	for _, c := range callbackW.Result().Cookies() {
		if c.Name == "oauth2_state" && c.MaxAge < 0 {
			clearedState = true
		}
	}
	if !clearedState {
		t.Error("oauth2_state cookie should be cleared after callback")
	}
}

// ── Logout ──

func TestAuth_Logout(t *testing.T) {
	h := newTestAuthHandler("https://example.com/token", "https://example.com/userinfo")
	r := setupAuthRouter(h)

	// Manually inject a session
	sessionID := generateSessionID()
	h.sessions[sessionID] = &Session{
		UserID:    "user_logout_test",
		ExpiresAt: 9999999999,
	}

	req := httptest.NewRequest("POST", "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "agentdisk_session", Value: sessionID})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	// Session should be deleted
	sess := h.GetSession(sessionID)
	if sess != nil {
		t.Error("session should be deleted after logout")
	}

	// Cookie should be cleared
	var cleared bool
	for _, c := range w.Result().Cookies() {
		if c.Name == "agentdisk_session" && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Error("session cookie should be cleared after logout")
	}
}

func TestAuth_Logout_NoCookie(t *testing.T) {
	h := newTestAuthHandler("https://example.com/token", "https://example.com/userinfo")
	r := setupAuthRouter(h)

	req := httptest.NewRequest("POST", "/auth/logout", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

// ── GetSession ──

func TestAuth_GetSession_Valid(t *testing.T) {
	h := NewAuthHandler(nil)
	sessionID := generateSessionID()
	h.sessions[sessionID] = &Session{
		UserID:    "user_getsession",
		ExpiresAt: 9999999999,
	}

	sess := h.GetSession(sessionID)
	if sess == nil {
		t.Fatal("GetSession returned nil")
	}
	if sess.UserID != "user_getsession" {
		t.Errorf("UserID = %q, want user_getsession", sess.UserID)
	}
}

func TestAuth_GetSession_NotFound(t *testing.T) {
	h := NewAuthHandler(nil)
	sess := h.GetSession("nonexistent")
	if sess != nil {
		t.Error("GetSession should return nil for nonexistent session")
	}
}

func TestAuth_GetSession_Expired(t *testing.T) {
	h := NewAuthHandler(nil)
	sessionID := generateSessionID()
	h.sessions[sessionID] = &Session{
		UserID:    "user_expired",
		ExpiresAt: 1, // already expired
	}

	sess := h.GetSession(sessionID)
	if sess != nil {
		t.Error("GetSession should return nil for expired session")
	}

	// Should also delete from map
	if _, ok := h.sessions[sessionID]; ok {
		t.Error("expired session should be removed from map")
	}
}

// ── generateSessionID ──

func TestGenerateSessionID(t *testing.T) {
	id1 := generateSessionID()
	id2 := generateSessionID()

	if id1 == "" {
		t.Error("session ID is empty")
	}
	if id1 == id2 {
		t.Error("two session IDs should differ")
	}

	// Should be valid base64
	_, err := base64.RawURLEncoding.DecodeString(id1)
	if err != nil {
		t.Errorf("session ID is not valid base64: %v", err)
	}
}
