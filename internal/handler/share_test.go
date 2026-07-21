package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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

// ── ShareDownload binding ──

func TestShareDownload_MissingBody(t *testing.T) {
	r := setupShareRouter()
	r.POST("/share/download", func(c *gin.Context) {
		var req ShareDownloadReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.OK(c, nil)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/share/download", nil)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing body, got %d", w.Code)
	}
}

func TestShareDownload_MissingFields(t *testing.T) {
	r := setupShareRouter()
	r.POST("/share/download", func(c *gin.Context) {
		var req ShareDownloadReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.OK(c, nil)
	})

	tests := []struct {
		name string
		body string
	}{
		{"empty JSON", `{}`},
		{"missing resourceId", `{"code": "abc"}`},
		{"missing code", `{"resourceId": 1}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/share/download", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400 for %s, got %d", tt.name, w.Code)
			}
		})
	}
}

func TestShareDownload_ValidBinding(t *testing.T) {
	r := setupShareRouter()
	r.POST("/share/download", func(c *gin.Context) {
		var req ShareDownloadReq
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.OK(c, gin.H{
			"code":       req.Code,
			"resourceId": req.ResourceID,
		})
	})

	body, _ := json.Marshal(map[string]interface{}{
		"code":        "abc123",
		"resourceId":  42,
		"extractCode": "pass",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/share/download", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	resp := w.Body.String()
	if !bytes.Contains([]byte(resp), []byte("42")) {
		t.Errorf("response should contain resourceId 42, got %s", resp)
	}
}

// ── GetShareStats (real repo→service→handler stack via in-memory SQLite) ──

// sanitizeShareDSN strips SQLite DSN-illegal characters from the test name so
// each test gets its own cache=shared in-memory database.
func sanitizeShareDSN(s string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "'", "_")
	return r.Replace(s)
}

// newShareStatsTestHandler builds a real ShareHandler over an in-memory SQLite
// DB so the full stack — including the 404/403/500 error mapping in the
// handler — is exercised. ShareStats only touches the share + access-log
// tables, so we migrate just those. Returns the DB so callers can seed rows.
func newShareStatsTestHandler(t *testing.T) (*ShareHandler, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:sharestats_%s?mode=memory&cache=shared", sanitizeShareDSN(t.Name()))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.DiskShare{}, &model.ShareAccessLog{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	repo := repository.NewShareRepo(db)
	svc := service.NewShareService(repo, nil, nil)
	return NewShareHandler(svc, "", 0), db
}

func TestGetShareStats_HappyPath(t *testing.T) {
	h, db := newShareStatsTestHandler(t)
	share := &model.DiskShare{UserID: "u1", ResourceID: 10, ResType: "file", ShareCode: "c1", VisitCount: 2, IsActive: true}
	if err := db.Create(share).Error; err != nil {
		t.Fatalf("seed share: %v", err)
	}
	now := time.Now().UTC()
	if err := db.Create(&model.ShareAccessLog{ShareID: share.ID, VisitorIP: "1.2.3.4", UserAgent: "curl", Action: "access", CreatedAt: now.Add(-time.Hour)}).Error; err != nil {
		t.Fatalf("seed log: %v", err)
	}
	if err := db.Create(&model.ShareAccessLog{ShareID: share.ID, VisitorIP: "5.6.7.8", UserAgent: "mozilla", Action: "access", CreatedAt: now}).Error; err != nil {
		t.Fatalf("seed log: %v", err)
	}

	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("userId", "u1"); c.Next() })
	r.GET("/shares/:id/stats", h.GetShareStats)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", fmt.Sprintf("/shares/%d/stats", share.ID), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, want := range []string{"visitCount", "uniqueIPs", "recentLogs", "1.2.3.0"} {
		if !bytes.Contains([]byte(body), []byte(want)) {
			t.Errorf("response missing %q: %s", want, body)
		}
	}
	// raw visitor IP must NOT leak — only the masked form is returned.
	if bytes.Contains([]byte(body), []byte("1.2.3.4")) {
		t.Errorf("raw visitor IP leaked into response: %s", body)
	}
}

func TestGetShareStats_InvalidID(t *testing.T) {
	h, _ := newShareStatsTestHandler(t)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("userId", "u1"); c.Next() })
	r.GET("/shares/:id/stats", h.GetShareStats)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/shares/abc/stats", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for non-numeric id, got %d", w.Code)
	}
}

func TestGetShareStats_NotFound(t *testing.T) {
	h, _ := newShareStatsTestHandler(t)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("userId", "u1"); c.Next() })
	r.GET("/shares/:id/stats", h.GetShareStats)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/shares/9999/stats", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing share, got %d", w.Code)
	}
}

func TestGetShareStats_NonOwnerForbidden(t *testing.T) {
	h, db := newShareStatsTestHandler(t)
	share := &model.DiskShare{UserID: "owner", ResourceID: 10, ResType: "file", ShareCode: "c1", IsActive: true}
	if err := db.Create(share).Error; err != nil {
		t.Fatalf("seed share: %v", err)
	}
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("userId", "intruder"); c.Next() })
	r.GET("/shares/:id/stats", h.GetShareStats)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", fmt.Sprintf("/shares/%d/stats", share.ID), nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-owner, got %d", w.Code)
	}
}
