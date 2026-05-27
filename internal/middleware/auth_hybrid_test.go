package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentdisk/agent-disk/internal/handler"
	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/pkg/download_token"
	"github.com/agentdisk/agent-disk/pkg/jwt"
	"github.com/agentdisk/agent-disk/pkg/oauth2client"
	"github.com/gin-gonic/gin"
)

// mockAPIKeyValidator is a test mock for APIKeyValidator.
type mockAPIKeyValidator struct {
	key *model.DiskAPIKey
	err error
}

func (m *mockAPIKeyValidator) ValidateKey(_ string) (*model.DiskAPIKey, error) {
	return m.key, m.err
}

func setupHybridRouter(jwtSecret string, authHandler *handler.AuthHandler, dlSecret string) *gin.Engine {
	return setupHybridRouterWithAPIKey(jwtSecret, authHandler, dlSecret, nil)
}

func setupHybridRouterWithAPIKey(jwtSecret string, authHandler *handler.AuthHandler, dlSecret string, apiKeySvc APIKeyValidator) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(HybridAuth(jwtSecret, authHandler, dlSecret, apiKeySvc))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"userId":     c.GetString("userId"),
			"authMethod": c.GetString("authMethod"),
		})
	})
	return r
}

// ── JWT Bearer ──

func TestHybridAuth_JWTValid(t *testing.T) {
	secret := "test_jwt_secret"
	token, _ := jwt.GenerateToken(secret, "user_001", "agent_001", 1)

	r := setupHybridRouter(secret, nil, "")
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestHybridAuth_JWTInvalid(t *testing.T) {
	r := setupHybridRouter("secret", nil, "")
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid_token")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestHybridAuth_JWTWrongSecret(t *testing.T) {
	token, _ := jwt.GenerateToken("secret_a", "user_001", "", 1)

	r := setupHybridRouter("secret_b", nil, "")
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestHybridAuth_JWTSetsContext(t *testing.T) {
	secret := "test_jwt_secret"
	token, _ := jwt.GenerateToken(secret, "user_ctx_test", "agent_ctx_test", 1)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(HybridAuth(secret, nil, "", nil))
	r.GET("/test", func(c *gin.Context) {
		if c.GetString("userId") != "user_ctx_test" {
			t.Errorf("userId = %q, want user_ctx_test", c.GetString("userId"))
		}
		if c.GetString("agentId") != "agent_ctx_test" {
			t.Errorf("agentId = %q, want agent_ctx_test", c.GetString("agentId"))
		}
		if c.GetString("authMethod") != "jwt" {
			t.Errorf("authMethod = %q, want jwt", c.GetString("authMethod"))
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
}

func TestHybridAuth_JWTWithAgentGroupId(t *testing.T) {
	secret := "test_jwt_secret"
	token, _ := jwt.GenerateTokenWithGroup(secret, "user_001", "agent_001", "group-a", 1)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(HybridAuth(secret, nil, "", nil))
	r.GET("/test", func(c *gin.Context) {
		if c.GetString("agentGroupId") != "group-a" {
			t.Errorf("agentGroupId = %q, want group-a", c.GetString("agentGroupId"))
		}
		if c.GetString("agentId") != "agent_001" {
			t.Errorf("agentId = %q, want agent_001", c.GetString("agentId"))
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestIsAgentRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("with agentId", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("agentId", "agent_001")
		if !IsAgentRequest(c) {
			t.Error("expected true when agentId is set")
		}
	})
	t.Run("without agentId", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		if IsAgentRequest(c) {
			t.Error("expected false when agentId is empty")
		}
	})
}

// ── OAuth2 Session ──

func TestHybridAuth_OAuth2Session_NilHandler(t *testing.T) {
	r := setupHybridRouter("secret", nil, "")
	req := httptest.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{Name: "agentdisk_session", Value: "any_value"})
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (nil authHandler)", w.Code)
	}
}

func TestHybridAuth_OAuth2Session_InvalidSessionID(t *testing.T) {
	oauthClient := oauth2client.New(oauth2client.Config{
		ClientID:     "agentdisk",
		ClientSecret: "secret",
		AuthURL:      "https://example.com/authorize",
		TokenURL:     "https://example.com/token",
		UserInfoURL:  "https://example.com/userinfo",
		RedirectURL:  "https://disk.example.com/auth/callback",
	})
	authH := handler.NewAuthHandler(oauthClient, "")

	r := setupHybridRouter("secret", authH, "")
	req := httptest.NewRequest("GET", "/test", nil)
	req.AddCookie(&http.Cookie{Name: "agentdisk_session", Value: "nonexistent_session"})
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (invalid session)", w.Code)
	}
}

func TestHybridAuth_OAuth2Session_NoCookie(t *testing.T) {
	oauthClient := oauth2client.New(oauth2client.Config{
		ClientID:     "agentdisk",
		ClientSecret: "secret",
		AuthURL:      "https://example.com/authorize",
		TokenURL:     "https://example.com/token",
		UserInfoURL:  "https://example.com/userinfo",
		RedirectURL:  "https://disk.example.com/auth/callback",
	})
	authH := handler.NewAuthHandler(oauthClient, "")

	r := setupHybridRouter("secret", authH, "")
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (no session cookie)", w.Code)
	}
}

// ── Download Token ──

func TestHybridAuth_DownloadTokenValid(t *testing.T) {
	secret := "test_dl_secret"
	dlToken, _ := download_token.Generate(secret, "user_dl_001", "12345", 300)

	r := setupHybridRouter("jwt_secret", nil, secret)
	req := httptest.NewRequest("GET", "/test?t="+dlToken, nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", w.Code, w.Body.String())
	}
}

func TestHybridAuth_DownloadTokenInvalid(t *testing.T) {
	r := setupHybridRouter("jwt_secret", nil, "dl_secret")
	req := httptest.NewRequest("GET", "/test?t=invalid_download_token", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestHybridAuth_DownloadTokenExpired(t *testing.T) {
	secret := "test_dl_secret"
	dlToken, _ := download_token.Generate(secret, "user_dl_001", "12345", -1)

	r := setupHybridRouter("jwt_secret", nil, secret)
	req := httptest.NewRequest("GET", "/test?t="+dlToken, nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (expired)", w.Code)
	}
}

func TestHybridAuth_DownloadTokenWrongSecret(t *testing.T) {
	dlToken, _ := download_token.Generate("secret_a", "user_dl_001", "12345", 300)

	r := setupHybridRouter("jwt_secret", nil, "secret_b")
	req := httptest.NewRequest("GET", "/test?t="+dlToken, nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (wrong secret)", w.Code)
	}
}

// ── API Key ──

func TestHybridAuth_APIKeyHeader(t *testing.T) {
	mock := &mockAPIKeyValidator{
		key: &model.DiskAPIKey{
			Scope:      "public_read",
			Department: "engineering",
		},
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(HybridAuth("secret", nil, "", mock))
	r.GET("/test", func(c *gin.Context) {
		if c.GetString("userId") != "__system_public__" {
			t.Errorf("userId = %q, want __system_public__", c.GetString("userId"))
		}
		if c.GetString("authMethod") != "api_key" {
			t.Errorf("authMethod = %q, want api_key", c.GetString("authMethod"))
		}
		if c.GetString("apiKeyScope") != "public_read" {
			t.Errorf("apiKeyScope = %q, want public_read", c.GetString("apiKeyScope"))
		}
		if c.GetString("department") != "engineering" {
			t.Errorf("department = %q, want engineering", c.GetString("department"))
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "adk_testkey123")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestHybridAuth_APIKeyQueryParam(t *testing.T) {
	mock := &mockAPIKeyValidator{
		key: &model.DiskAPIKey{
			Scope:      "public_read",
			Department: "",
		},
	}

	r := setupHybridRouterWithAPIKey("secret", nil, "", mock)
	req := httptest.NewRequest("GET", "/test?apiKey=adk_testkey123", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestHybridAuth_APIKeyInvalid(t *testing.T) {
	mock := &mockAPIKeyValidator{
		key: nil,
		err: fmt.Errorf("invalid key"),
	}

	r := setupHybridRouterWithAPIKey("secret", nil, "", mock)
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "adk_invalid")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestHybridAuth_APIKeyNilService(t *testing.T) {
	r := setupHybridRouter("secret", nil, "")
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-API-Key", "adk_somekey")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (nil apiKeySvc)", w.Code)
	}
}

// ── No Auth ──

func TestHybridAuth_NoAuth(t *testing.T) {
	r := setupHybridRouter("secret", nil, "dl_secret")
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestHybridAuth_EmptyBearer(t *testing.T) {
	r := setupHybridRouter("secret", nil, "")
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer ")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestHybridAuth_NonBearerAuth(t *testing.T) {
	r := setupHybridRouter("secret", nil, "")
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestHybridAuth_EmptyDLSecret(t *testing.T) {
	r := setupHybridRouter("secret", nil, "")
	req := httptest.NewRequest("GET", "/test?t=some_token", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (no dlSecret)", w.Code)
	}
}

// ── Priority ──

func TestHybridAuth_JWTPriorityOverDownloadToken(t *testing.T) {
	secret := "test_jwt_secret"
	token, _ := jwt.GenerateToken(secret, "user_jwt", "", 1)

	r := setupHybridRouter(secret, nil, "dl_secret")
	req := httptest.NewRequest("GET", "/test?t=any_token", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (JWT takes priority)", w.Code)
	}
}
