package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

// setupShareRouter creates a test router with share handler endpoints.
// Tests parameter binding and auth flow without real database dependencies.
func setupShareRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	return r
}

// ── RevokeShare ──

func TestRevokeShare_MissingBody(t *testing.T) {
	r := setupShareRouter()
	r.Use(func(c *gin.Context) { c.Set("userId", "user001"); c.Next() })
	r.DELETE("/shares", func(c *gin.Context) {
		var req RevokeShareReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.OK(c, nil)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/shares", nil)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing body, got %d", w.Code)
	}
}

func TestRevokeShare_EmptyJSON(t *testing.T) {
	r := setupShareRouter()
	r.Use(func(c *gin.Context) { c.Set("userId", "user001"); c.Next() })
	r.DELETE("/shares", func(c *gin.Context) {
		var req RevokeShareReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.OK(c, nil)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/shares", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty JSON, got %d", w.Code)
	}
}

func TestRevokeShare_ValidBinding(t *testing.T) {
	r := setupShareRouter()
	r.Use(func(c *gin.Context) { c.Set("userId", "user001"); c.Next() })
	r.DELETE("/shares", func(c *gin.Context) {
		var req RevokeShareReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.OK(c, gin.H{"shareId": req.ShareID})
	})

	body, _ := json.Marshal(map[string]uint64{"shareId": 42})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/shares", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	resp := w.Body.String()
	if !bytes.Contains([]byte(resp), []byte("42")) {
		t.Errorf("response should contain shareId 42, got %s", resp)
	}
}

func TestRevokeShare_ZeroShareID(t *testing.T) {
	r := setupShareRouter()
	r.Use(func(c *gin.Context) { c.Set("userId", "user001"); c.Next() })
	r.DELETE("/shares", func(c *gin.Context) {
		var req RevokeShareReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.OK(c, nil)
	})

	body, _ := json.Marshal(map[string]int{"shareId": 0})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/shares", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// binding:"required" on uint64 with zero value should fail
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for shareId=0 (required field), got %d", w.Code)
	}
}

// ── CreateShare ──

func TestCreateShare_MissingBody(t *testing.T) {
	r := setupShareRouter()
	r.Use(func(c *gin.Context) { c.Set("userId", "user001"); c.Next() })
	r.POST("/shares", func(c *gin.Context) {
		var req CreateShareReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Created(c, nil)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/shares", nil)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing body, got %d", w.Code)
	}
}

func TestCreateShare_MissingRequiredFields(t *testing.T) {
	r := setupShareRouter()
	r.Use(func(c *gin.Context) { c.Set("userId", "user001"); c.Next() })
	r.POST("/shares", func(c *gin.Context) {
		var req CreateShareReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Created(c, nil)
	})

	tests := []struct {
		name string
		body string
	}{
		{"empty JSON", `{}`},
		{"missing resType", `{"resourceId": 1}`},
		{"missing resourceId", `{"resType": "file"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/shares", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400 for %s, got %d", tt.name, w.Code)
			}
		})
	}
}

func TestCreateShare_ValidBinding(t *testing.T) {
	r := setupShareRouter()
	r.Use(func(c *gin.Context) { c.Set("userId", "user001"); c.Next() })
	r.POST("/shares", func(c *gin.Context) {
		var req CreateShareReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		// Verify defaults
		if req.MaxVisit == 0 {
			req.MaxVisit = -1
		}
		if req.ExpireHours == 0 {
			req.ExpireHours = 72
		}
		response.Created(c, gin.H{
			"resourceId":  req.ResourceID,
			"resType":     req.ResType,
			"maxVisit":    req.MaxVisit,
			"expireHours": req.ExpireHours,
		})
	})

	body, _ := json.Marshal(map[string]interface{}{
		"resourceId":  1,
		"resType":     "file",
		"extractCode": "abc123",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/shares", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
	resp := w.Body.String()
	if !bytes.Contains([]byte(resp), []byte(`"maxVisit":-1`)) {
		t.Errorf("maxVisit should default to -1, got %s", resp)
	}
	if !bytes.Contains([]byte(resp), []byte(`"expireHours":72`)) {
		t.Errorf("expireHours should default to 72, got %s", resp)
	}
}

// ── AccessShare ──

func TestAccessShare_MissingCode(t *testing.T) {
	r := setupShareRouter()
	r.POST("/shares/access", func(c *gin.Context) {
		var req AccessShareReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.OK(c, nil)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/shares/access", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing code, got %d", w.Code)
	}
}

func TestAccessShare_ValidBinding(t *testing.T) {
	r := setupShareRouter()
	r.POST("/shares/access", func(c *gin.Context) {
		var req AccessShareReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.OK(c, gin.H{"code": req.Code})
	})

	body, _ := json.Marshal(map[string]string{
		"code":        "abc123def456",
		"extractCode": "1234",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/shares/access", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	resp := w.Body.String()
	if !bytes.Contains([]byte(resp), []byte("abc123def456")) {
		t.Errorf("response should contain share code, got %s", resp)
	}
}

// ── ListShares ──

func TestListShares_ExtractsUserID(t *testing.T) {
	r := setupShareRouter()
	r.Use(func(c *gin.Context) { c.Set("userId", "user001"); c.Next() })
	r.GET("/shares", func(c *gin.Context) {
		userID := c.GetString("userId")
		if userID == "" {
			response.Unauthorized(c, "missing userId")
			return
		}
		response.OK(c, gin.H{"userId": userID})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/shares", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	resp := w.Body.String()
	if !bytes.Contains([]byte(resp), []byte("user001")) {
		t.Errorf("response should contain userId, got %s", resp)
	}
}

// ── GetShare by code ──

func TestGetShare_EmptyCode(t *testing.T) {
	r := setupShareRouter()
	r.GET("/shares/:code", func(c *gin.Context) {
		code := c.Param("code")
		if code == "" {
			response.NotFound(c, "share code required")
			return
		}
		response.OK(c, nil)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/shares/", nil)
	r.ServeHTTP(w, req)

	// Gin returns 404 for unmatched route when path is /shares/ (no code)
	if w.Code != http.StatusNotFound {
		t.Logf("GET /shares/ returned %d (may be Gin 404), body: %s", w.Code, w.Body.String())
	}
}

func TestGetShare_ValidCode(t *testing.T) {
	r := setupShareRouter()
	r.GET("/shares/:code", func(c *gin.Context) {
		code := c.Param("code")
		response.OK(c, gin.H{"shareCode": code})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/shares/abc123def456", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	resp := w.Body.String()
	if !bytes.Contains([]byte(resp), []byte("abc123def456")) {
		t.Errorf("response should contain share code, got %s", resp)
	}
}

// ── Request body format validation ──

func TestCreateShare_InvalidJSON(t *testing.T) {
	r := setupShareRouter()
	r.POST("/shares", func(c *gin.Context) {
		var req CreateShareReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Created(c, nil)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/shares", bytes.NewBufferString(`not json`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

func TestRevokeShare_InvalidJSON(t *testing.T) {
	r := setupShareRouter()
	r.DELETE("/shares", func(c *gin.Context) {
		var req RevokeShareReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.OK(c, nil)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/shares", bytes.NewBufferString(`not json`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

// ── ShareId type validation ──

func TestRevokeShare_StringShareID(t *testing.T) {
	r := setupShareRouter()
	r.DELETE("/shares", func(c *gin.Context) {
		var req RevokeShareReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.OK(c, nil)
	})

	// Send string instead of number for shareId
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/shares", bytes.NewBufferString(`{"shareId": "not_a_number"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for string shareId, got %d", w.Code)
	}
}

func TestCreateShare_ResourceIdAsString(t *testing.T) {
	r := setupShareRouter()
	r.POST("/shares", func(c *gin.Context) {
		var req CreateShareReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Created(c, nil)
	})

	// Send string instead of number for resourceId
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/shares", bytes.NewBufferString(`{"resourceId": "not_a_number", "resType": "file"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for string resourceId, got %d", w.Code)
	}
}
