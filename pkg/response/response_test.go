package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupRouter() *gin.Engine {
	r := gin.New()
	return r
}

func TestOK(t *testing.T) {
	r := setupRouter()
	r.GET("/test", func(c *gin.Context) {
		OK(c, gin.H{"key": "value"})
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp R
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
	if resp.Message != "success" {
		t.Errorf("expected success, got %s", resp.Message)
	}
}

func TestCreated(t *testing.T) {
	r := setupRouter()
	r.POST("/test", func(c *gin.Context) {
		Created(c, gin.H{"id": 1})
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
}

func TestBadRequest(t *testing.T) {
	r := setupRouter()
	r.GET("/test", func(c *gin.Context) {
		BadRequest(c, "invalid param")
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	var resp R
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != 400 {
		t.Errorf("expected code 400, got %d", resp.Code)
	}
}

func TestUnauthorized(t *testing.T) {
	r := setupRouter()
	r.GET("/test", func(c *gin.Context) {
		Unauthorized(c, "no token")
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestForbidden(t *testing.T) {
	r := setupRouter()
	r.GET("/test", func(c *gin.Context) {
		Forbidden(c, "denied")
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestNotFound(t *testing.T) {
	r := setupRouter()
	r.GET("/test", func(c *gin.Context) {
		NotFound(c, "not found")
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestInternalError(t *testing.T) {
	r := setupRouter()
	r.GET("/test", func(c *gin.Context) {
		InternalError(c, "some error")
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
	var resp R
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Message != "internal error" {
		t.Errorf("sensitive info should not be exposed, got %s", resp.Message)
	}
}
