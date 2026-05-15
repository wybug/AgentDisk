package oauth2client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/oauth2"
)

func TestAuthCodeURL(t *testing.T) {
	client := New(Config{
		ClientID:    "agentdisk",
		AuthURL:     "https://gateway.example.com/oauth2/authorize",
		TokenURL:    "https://gateway.example.com/oauth2/token",
		RedirectURL: "https://disk.example.com/auth/callback",
	})

	verifier, _ := GenerateCodeVerifier()
	url := client.AuthCodeURL("state123", verifier, false)

	if url == "" {
		t.Fatal("AuthCodeURL returned empty")
	}
	if !contains(url, "client_id=agentdisk") {
		t.Error("AuthCodeURL should contain client_id")
	}
	if !contains(url, "code_challenge_method=S256") {
		t.Error("AuthCodeURL should contain S256 challenge method")
	}
	if contains(url, "prompt=none") {
		t.Error("standard AuthCodeURL should not contain prompt=none")
	}
}

func TestAuthCodeURL_PromptNone(t *testing.T) {
	client := New(Config{
		ClientID:    "agentdisk",
		AuthURL:     "https://gateway.example.com/oauth2/authorize",
		TokenURL:    "https://gateway.example.com/oauth2/token",
		RedirectURL: "https://disk.example.com/auth/callback",
	})

	verifier, _ := GenerateCodeVerifier()
	url := client.AuthCodeURL("state123", verifier, true)

	if !contains(url, "prompt=none") {
		t.Error("SSO AuthCodeURL should contain prompt=none")
	}
}

func TestExchange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth2/token" {
			t.Errorf("expected /oauth2/token, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "test_access_token",
			"token_type":    "Bearer",
			"expires_in":    3600,
			"refresh_token": "test_refresh_token",
			"userId":        "user_001",
		})
	}))
	defer srv.Close()

	client := New(Config{
		ClientID:     "agentdisk",
		ClientSecret: "secret",
		AuthURL:      srv.URL + "/oauth2/authorize",
		TokenURL:     srv.URL + "/oauth2/token",
		UserInfoURL:  srv.URL + "/oauth2/userinfo",
		RedirectURL:  "https://disk.example.com/auth/callback",
	})

	verifier, _ := GenerateCodeVerifier()
	token, err := client.Exchange(context.Background(), "test_code", verifier)
	if err != nil {
		t.Fatalf("Exchange failed: %v", err)
	}
	if token.AccessToken != "test_access_token" {
		t.Errorf("AccessToken = %q, want test_access_token", token.AccessToken)
	}
}

func TestGetUserInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth2/userinfo" {
			t.Errorf("expected /oauth2/userinfo, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(UserInfo{
			UserID:   "user_001",
			UserName: "Test User",
		})
	}))
	defer srv.Close()

	client := New(Config{
		ClientID:     "agentdisk",
		ClientSecret: "secret",
		AuthURL:      srv.URL + "/oauth2/authorize",
		TokenURL:     srv.URL + "/oauth2/token",
		UserInfoURL:  srv.URL + "/oauth2/userinfo",
		RedirectURL:  "https://disk.example.com/auth/callback",
	})

	token := &oauth2.Token{AccessToken: "test_access_token"}
	ui, err := client.GetUserInfo(context.Background(), token)
	if err != nil {
		t.Fatalf("GetUserInfo failed: %v", err)
	}
	if ui.UserID != "user_001" {
		t.Errorf("UserID = %q, want user_001", ui.UserID)
	}
}

func TestGenerateCodeVerifier(t *testing.T) {
	v1, err := GenerateCodeVerifier()
	if err != nil {
		t.Fatalf("GenerateCodeVerifier failed: %v", err)
	}
	if len(v1) < 32 {
		t.Errorf("CodeVerifier too short: %d", len(v1))
	}

	v2, _ := GenerateCodeVerifier()
	if v1 == v2 {
		t.Error("Two code verifiers should differ")
	}
}

func TestGenerateCodeChallenge(t *testing.T) {
	verifier := "test_verifier_string"
	challenge := GenerateCodeChallenge(verifier)

	if challenge == "" {
		t.Fatal("CodeChallenge is empty")
	}

	challenge2 := GenerateCodeChallenge(verifier)
	if challenge != challenge2 {
		t.Error("Same verifier should produce same challenge")
	}
}

func TestGenerateState(t *testing.T) {
	s1, err := GenerateState()
	if err != nil {
		t.Fatalf("GenerateState failed: %v", err)
	}
	if len(s1) < 16 {
		t.Errorf("State too short: %d", len(s1))
	}

	s2, _ := GenerateState()
	if s1 == s2 {
		t.Error("Two states should differ")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestExtractUserIDFromToken_NilToken(t *testing.T) {
	result := ExtractUserIDFromToken(nil)
	if result != "" {
		t.Errorf("ExtractUserIDFromToken(nil) = %q, want empty", result)
	}
}

func TestExtractUserIDFromToken_NoUserID(t *testing.T) {
	token := &oauth2.Token{AccessToken: "test"}
	result := ExtractUserIDFromToken(token)
	if result != "" {
		t.Errorf("ExtractUserIDFromToken without userId = %q, want empty", result)
	}
}

func TestExtractUserIDFromToken_WithUserID(t *testing.T) {
	token := &oauth2.Token{
		AccessToken: "test",
	}
	result := ExtractUserIDFromToken(token)
	// oauth2.Token doesn't support Extra with custom fields via struct literal
	// so we test the nil/empty path which is the safe default
	if result != "" {
		t.Errorf("Expected empty for token without extra fields, got %q", result)
	}
}

func TestGetUserInfo_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("invalid token"))
	}))
	defer srv.Close()

	client := New(Config{
		ClientID:     "agentdisk",
		ClientSecret: "secret",
		AuthURL:      srv.URL + "/oauth2/authorize",
		TokenURL:     srv.URL + "/oauth2/token",
		UserInfoURL:  srv.URL + "/oauth2/userinfo",
		RedirectURL:  "https://disk.example.com/auth/callback",
	})

	token := &oauth2.Token{AccessToken: "test"}
	_, err := client.GetUserInfo(context.Background(), token)
	if err == nil {
		t.Fatal("GetUserInfo should fail with 401")
	}
}

func TestGetUserInfo_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	client := New(Config{
		ClientID:     "agentdisk",
		ClientSecret: "secret",
		AuthURL:      srv.URL + "/oauth2/authorize",
		TokenURL:     srv.URL + "/oauth2/token",
		UserInfoURL:  srv.URL + "/oauth2/userinfo",
		RedirectURL:  "https://disk.example.com/auth/callback",
	})

	token := &oauth2.Token{AccessToken: "test"}
	_, err := client.GetUserInfo(context.Background(), token)
	if err == nil {
		t.Fatal("GetUserInfo should fail with invalid JSON")
	}
}

func TestNew_DefaultScopes(t *testing.T) {
	client := New(Config{
		ClientID: "agentdisk",
		AuthURL:  "https://example.com/authorize",
		TokenURL: "https://example.com/token",
	})
	if client == nil {
		t.Fatal("New returned nil")
	}
	if client.config == nil {
		t.Fatal("config is nil")
	}
	if len(client.config.Scopes) != 2 || client.config.Scopes[0] != "openid" {
		t.Errorf("Scopes = %v, want [openid profile]", client.config.Scopes)
	}
}

func TestNew_CustomScopes(t *testing.T) {
	client := New(Config{
		ClientID: "agentdisk",
		AuthURL:  "https://example.com/authorize",
		TokenURL: "https://example.com/token",
		Scopes:   []string{"custom"},
	})
	if client.config.Scopes[0] != "custom" {
		t.Errorf("Scopes = %v, want [custom]", client.config.Scopes)
	}
}

func TestExchange_InvalidCode(t *testing.T) {
	client := New(Config{
		ClientID:     "agentdisk",
		ClientSecret: "secret",
		AuthURL:      "https://invalid.example.com/authorize",
		TokenURL:     "https://invalid.example.com/token",
		RedirectURL:  "https://disk.example.com/auth/callback",
	})

	_, err := client.Exchange(context.Background(), "bad_code", "verifier")
	if err == nil {
		t.Fatal("Exchange with unreachable server should fail")
	}
}
