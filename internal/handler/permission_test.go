package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/gin-gonic/gin"
)

type mockPermHandlerRepo struct {
	perms map[string]*model.DiskPermission
}

func newMockPermHandlerRepo() *mockPermHandlerRepo {
	return &mockPermHandlerRepo{perms: make(map[string]*model.DiskPermission)}
}

func (m *mockPermHandlerRepo) Create(p *model.DiskPermission) error {
	m.perms[p.AgentID+p.ResourcePath] = p
	return nil
}
func (m *mockPermHandlerRepo) GetByAgentAndResource(string, uint64, string) (*model.DiskPermission, error) {
	return nil, nil
}
func (m *mockPermHandlerRepo) GetByAgentAndResourcePath(string, string) (*model.DiskPermission, error) {
	return nil, nil
}
func (m *mockPermHandlerRepo) GetByGroupAndResource(string, uint64, string) (*model.DiskPermission, error) {
	return nil, nil
}
func (m *mockPermHandlerRepo) GetByGroupAndResourcePath(string, string) (*model.DiskPermission, error) {
	return nil, nil
}
func (m *mockPermHandlerRepo) GetResourceDetail(uint64, string) (*repository.ResourceOwner, error) {
	return &repository.ResourceOwner{}, nil
}
func (m *mockPermHandlerRepo) GetResourcePath(uint64, string) (string, error) {
	return "/test/file.txt", nil
}
func (m *mockPermHandlerRepo) ListByUser(string) ([]model.DiskPermission, error) {
	return nil, nil
}
func (m *mockPermHandlerRepo) ListPathPermissionsByAgent(string) ([]model.DiskPermission, error) {
	return nil, nil
}
func (m *mockPermHandlerRepo) ListGroupPermissions(string) ([]model.DiskPermission, error) {
	return nil, nil
}
func (m *mockPermHandlerRepo) Delete(uint64) error { return nil }

func setupPermHandlerRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("userId", "user001"); c.Next() })

	repo := newMockPermHandlerRepo()
	svc := service.NewPermissionServiceWithRepo(repo)
	h := NewPermissionHandler(svc)

	r.POST("/permissions", h.GrantPermission)
	r.GET("/permissions", h.ListPermissions)
	r.DELETE("/permissions", h.RevokePermission)
	r.GET("/permissions/check", h.CheckPermission)
	return r
}

func TestGrantPermission_ResourceIDMode(t *testing.T) {
	r := setupPermHandlerRouter()

	body, _ := json.Marshal(map[string]any{
		"agentId":    "agent-01",
		"resourceId": 1,
		"resType":    "file",
		"permission": "read",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/permissions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGrantPermission_PathMode(t *testing.T) {
	r := setupPermHandlerRouter()

	body, _ := json.Marshal(map[string]any{
		"agentId":      "agent-01",
		"resourcePath": "/**",
		"permission":   "read",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/permissions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGrantPermission_GroupMode(t *testing.T) {
	r := setupPermHandlerRouter()

	body, _ := json.Marshal(map[string]any{
		"agentGroupId": "group-a",
		"resourcePath": "/Documents/**",
		"permission":   "write",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/permissions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGrantPermission_MissingAgentTarget(t *testing.T) {
	r := setupPermHandlerRouter()

	body, _ := json.Marshal(map[string]any{
		"resourcePath": "/**",
		"permission":   "read",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/permissions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGrantPermission_MissingResource(t *testing.T) {
	r := setupPermHandlerRouter()

	body, _ := json.Marshal(map[string]any{
		"agentId":    "agent-01",
		"permission": "read",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/permissions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGrantPermission_ResourceIDWithoutResType(t *testing.T) {
	r := setupPermHandlerRouter()

	body, _ := json.Marshal(map[string]any{
		"agentId":    "agent-01",
		"resourceId": 1,
		"permission": "read",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/permissions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGrantPermission_InvalidPathFormat(t *testing.T) {
	r := setupPermHandlerRouter()

	body, _ := json.Marshal(map[string]any{
		"agentId":      "agent-01",
		"resourcePath": "no-leading-slash",
		"permission":   "read",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/permissions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGrantPermission_MissingPermission(t *testing.T) {
	r := setupPermHandlerRouter()

	body, _ := json.Marshal(map[string]any{
		"agentId":      "agent-01",
		"resourcePath": "/**",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/permissions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing permission, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGrantPermission_BothResourceIDAndPath(t *testing.T) {
	r := setupPermHandlerRouter()

	body, _ := json.Marshal(map[string]any{
		"agentId":      "agent-01",
		"resourceId":   1,
		"resType":      "file",
		"resourcePath": "/**",
		"permission":   "read",
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/permissions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGrantPermission_InvalidJSON(t *testing.T) {
	r := setupPermHandlerRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/permissions", bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", w.Code)
	}
}
