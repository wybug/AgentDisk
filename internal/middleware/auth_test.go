package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentdisk/agent-disk/pkg/jwt"
	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestJWTAuth_Valid(t *testing.T) {
	secret := "test-jwt-secret"
	token, _ := jwt.GenerateToken(secret, "user001", "agent001", 72)

	r := gin.New()
	r.Use(JWTAuth(secret))
	r.GET("/protected", func(c *gin.Context) {
		c.JSON(200, gin.H{"userId": c.GetString("userId")})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestJWTAuth_MissingHeader(t *testing.T) {
	r := gin.New()
	r.Use(JWTAuth("secret"))
	r.GET("/protected", func(c *gin.Context) {
		c.JSON(200, nil)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestJWTAuth_InvalidToken(t *testing.T) {
	r := gin.New()
	r.Use(JWTAuth("secret"))
	r.GET("/protected", func(c *gin.Context) {
		c.JSON(200, nil)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestJWTAuth_NoBearerPrefix(t *testing.T) {
	r := gin.New()
	r.Use(JWTAuth("secret"))
	r.GET("/protected", func(c *gin.Context) {
		c.JSON(200, nil)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Basic sometoken")
	r.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}
