package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

// stubOkfSvc is a configurable stub of okfHandlerService. Each field is a
// function the handler calls; tests set the ones they exercise and leave the
// rest nil. A nil function returns a zero/error value so unconfigured paths
// fail loudly rather than silently succeeding.
type stubOkfSvc struct {
	registerBundle   func(ctx context.Context, pdID uint64) (*model.OkfBundle, error)
	listBundles      func() ([]model.OkfBundle, error)
	getBundle        func(id uint64) (*model.OkfBundle, error)
	listNodesByType  func(bundleID uint64, typeF, tagF string) ([]model.OkfNode, error)
	aggregateByType  func(bundleID uint64) ([]repository.TypeCount, error)
	refreshBundle    func(ctx context.Context, id uint64) (*model.OkfBundle, error)
	unregisterBundle func(id uint64) error
	writeMarkdown    func(ctx context.Context, req service.WriteMarkdownRequest) (*model.OkfNode, error)
}

func (s *stubOkfSvc) RegisterBundle(ctx context.Context, pdID uint64) (*model.OkfBundle, error) {
	if s.registerBundle == nil {
		return nil, errors.New("not stubbed")
	}
	return s.registerBundle(ctx, pdID)
}

func (s *stubOkfSvc) ListBundles() ([]model.OkfBundle, error) {
	if s.listBundles == nil {
		return nil, errors.New("not stubbed")
	}
	return s.listBundles()
}

func (s *stubOkfSvc) GetBundle(id uint64) (*model.OkfBundle, error) {
	if s.getBundle == nil {
		return nil, errors.New("not stubbed")
	}
	return s.getBundle(id)
}

func (s *stubOkfSvc) ListNodesByType(bundleID uint64, typeF, tagF string) ([]model.OkfNode, error) {
	if s.listNodesByType == nil {
		return nil, errors.New("not stubbed")
	}
	return s.listNodesByType(bundleID, typeF, tagF)
}

func (s *stubOkfSvc) AggregateByType(bundleID uint64) ([]repository.TypeCount, error) {
	if s.aggregateByType == nil {
		return nil, errors.New("not stubbed")
	}
	return s.aggregateByType(bundleID)
}

func (s *stubOkfSvc) RefreshBundle(ctx context.Context, id uint64) (*model.OkfBundle, error) {
	if s.refreshBundle == nil {
		return nil, errors.New("not stubbed")
	}
	return s.refreshBundle(ctx, id)
}

func (s *stubOkfSvc) UnregisterBundle(id uint64) error {
	if s.unregisterBundle == nil {
		return errors.New("not stubbed")
	}
	return s.unregisterBundle(id)
}

func (s *stubOkfSvc) WriteMarkdown(ctx context.Context, req service.WriteMarkdownRequest) (*model.OkfNode, error) {
	if s.writeMarkdown == nil {
		return nil, errors.New("not stubbed")
	}
	return s.writeMarkdown(ctx, req)
}

// okfHandlerWithStub builds a gin engine with the OKF routes wired to a stub
// service. Routes mirror the production router so URL/path-param behavior is
// exercised end-to-end.
func okfHandlerWithStub(t *testing.T, stub *stubOkfSvc) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("userId", "user001"); c.Next() })
	h := &OkfHandler{svc: stub}
	v1 := r.Group("/v1/disk/okf")
	v1.POST("/bundles/register", h.RegisterBundle)
	v1.GET("/bundles", h.ListBundles)
	v1.GET("/bundles/:id", h.GetBundle)
	v1.GET("/bundles/:id/nodes", h.ListNodes)
	v1.GET("/types", h.AggregateTypes)
	v1.POST("/bundles/:id/refresh", h.RefreshBundle)
	v1.DELETE("/bundles/:id", h.UnregisterBundle)
	return r
}

// doJSON issues a JSON request and returns the request + recorder pair.
func doJSON(t *testing.T, method, url string, body any) (*http.Request, *httptest.ResponseRecorder) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		buf = *bytes.NewBuffer(b)
	}
	req, _ := http.NewRequest(method, url, &buf)
	req.Header.Set("Content-Type", "application/json")
	return req, httptest.NewRecorder()
}

// decodeResponse pulls the standard {code,message,data} envelope.
func decodeResponse(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response: %v (body=%q)", err, w.Body.String())
	}
	return out
}

func TestOkfHandler_RegisterBundle_Success(t *testing.T) {
	stub := &stubOkfSvc{
		registerBundle: func(_ context.Context, pdID uint64) (*model.OkfBundle, error) {
			if pdID != 7 {
				t.Errorf("pdID = %d, want 7", pdID)
			}
			return &model.OkfBundle{ID: 1, PublicDirectoryID: 7, OkfVersion: "0.1", Title: "KB"}, nil
		},
	}
	r := okfHandlerWithStub(t, stub)
	req, w := doJSON(t, "POST", "/v1/disk/okf/bundles/register", gin.H{"publicDirectoryId": 7})
	r.ServeHTTP(w, req)

	if w.Code != 201 {
		t.Errorf("status = %d, want 201", w.Code)
	}
	body := decodeResponse(t, w)
	data, _ := body["data"].(map[string]any)
	if data["okfVersion"] != "0.1" {
		t.Errorf("data.okfVersion = %v, want 0.1", data["okfVersion"])
	}
}

func TestOkfHandler_RegisterBundle_BadBody(t *testing.T) {
	r := okfHandlerWithStub(t, &stubOkfSvc{})
	req, w := doJSON(t, "POST", "/v1/disk/okf/bundles/register", gin.H{})
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestOkfHandler_RegisterBundle_NotRoot(t *testing.T) {
	stub := &stubOkfSvc{
		registerBundle: func(context.Context, uint64) (*model.OkfBundle, error) {
			return nil, service.ErrOkfNotBundleRoot
		},
	}
	r := okfHandlerWithStub(t, stub)
	req, w := doJSON(t, "POST", "/v1/disk/okf/bundles/register", gin.H{"publicDirectoryId": 7})
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("status = %d, want 400 (not bundle root)", w.Code)
	}
}

func TestOkfHandler_ListBundles_Success(t *testing.T) {
	stub := &stubOkfSvc{
		listBundles: func() ([]model.OkfBundle, error) {
			return []model.OkfBundle{{ID: 1, Title: "A"}, {ID: 2, Title: "B"}}, nil
		},
	}
	r := okfHandlerWithStub(t, stub)
	req, w := doJSON(t, "GET", "/v1/disk/okf/bundles", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
	body := decodeResponse(t, w)
	data, _ := body["data"].([]any)
	if len(data) != 2 {
		t.Errorf("len(data) = %d, want 2", len(data))
	}
}

func TestOkfHandler_GetBundle_Success(t *testing.T) {
	stub := &stubOkfSvc{
		getBundle: func(id uint64) (*model.OkfBundle, error) {
			if id != 5 {
				t.Errorf("id = %d, want 5", id)
			}
			return &model.OkfBundle{ID: 5, Title: "X"}, nil
		},
	}
	r := okfHandlerWithStub(t, stub)
	req, w := doJSON(t, "GET", "/v1/disk/okf/bundles/5", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestOkfHandler_GetBundle_NotFound(t *testing.T) {
	stub := &stubOkfSvc{
		getBundle: func(uint64) (*model.OkfBundle, error) { return nil, service.ErrOkfBundleNotFound },
	}
	r := okfHandlerWithStub(t, stub)
	req, w := doJSON(t, "GET", "/v1/disk/okf/bundles/99", nil)
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestOkfHandler_GetBundle_InvalidID(t *testing.T) {
	r := okfHandlerWithStub(t, &stubOkfSvc{})
	req, w := doJSON(t, "GET", "/v1/disk/okf/bundles/abc", nil)
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("status = %d, want 400 (invalid id)", w.Code)
	}
}

func TestOkfHandler_GetBundle_InternalError(t *testing.T) {
	stub := &stubOkfSvc{
		getBundle: func(uint64) (*model.OkfBundle, error) { return nil, errors.New("db down") },
	}
	r := okfHandlerWithStub(t, stub)
	req, w := doJSON(t, "GET", "/v1/disk/okf/bundles/5", nil)
	r.ServeHTTP(w, req)
	if w.Code != 500 {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestOkfHandler_ListNodes_FiltersAndParses(t *testing.T) {
	var seenType, seenTag string
	stub := &stubOkfSvc{
		listNodesByType: func(_ uint64, typeF, tagF string) ([]model.OkfNode, error) {
			seenType, seenTag = typeF, tagF
			n := &model.OkfNode{ID: 3, BundleID: 1, RelPath: "a.md", Type: "concept"}
			return []model.OkfNode{*n}, nil
		},
	}
	r := okfHandlerWithStub(t, stub)
	req, _ := http.NewRequest("GET", "/v1/disk/okf/bundles/1/nodes?type=concept&tag=llm", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if seenType != "concept" {
		t.Errorf("type filter = %q, want concept", seenType)
	}
	if seenTag != "llm" {
		t.Errorf("tag filter = %q, want llm", seenTag)
	}
	body := decodeResponse(t, w)
	data, _ := body["data"].([]any)
	if len(data) != 1 {
		t.Errorf("len(data) = %d, want 1", len(data))
	}
}

func TestOkfHandler_AggregateTypes_Rollup(t *testing.T) {
	stub := &stubOkfSvc{
		listBundles: func() ([]model.OkfBundle, error) {
			return []model.OkfBundle{{ID: 1}, {ID: 2}}, nil
		},
		aggregateByType: func(bundleID uint64) ([]repository.TypeCount, error) {
			if bundleID == 1 {
				return []repository.TypeCount{{Type: "concept", Count: 2}}, nil
			}
			return []repository.TypeCount{{Type: "concept", Count: 1}, {Type: "guide", Count: 3}}, nil
		},
	}
	r := okfHandlerWithStub(t, stub)
	req, w := doJSON(t, "GET", "/v1/disk/okf/types", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
	body := decodeResponse(t, w)
	data, _ := body["data"].(map[string]any)
	// concept: 2 + 1 = 3 across both bundles; guide: 3.
	if data["concept"].(float64) != 3 {
		t.Errorf("concept = %v, want 3", data["concept"])
	}
	if data["guide"].(float64) != 3 {
		t.Errorf("guide = %v, want 3", data["guide"])
	}
}

func TestOkfHandler_AggregateTypes_ListError(t *testing.T) {
	stub := &stubOkfSvc{
		listBundles: func() ([]model.OkfBundle, error) { return nil, errors.New("db down") },
	}
	r := okfHandlerWithStub(t, stub)
	req, w := doJSON(t, "GET", "/v1/disk/okf/types", nil)
	r.ServeHTTP(w, req)
	if w.Code != 500 {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestOkfHandler_RefreshBundle_Success(t *testing.T) {
	stub := &stubOkfSvc{
		refreshBundle: func(_ context.Context, id uint64) (*model.OkfBundle, error) {
			if id != 4 {
				t.Errorf("id = %d, want 4", id)
			}
			return &model.OkfBundle{ID: 4, NodeCount: 7}, nil
		},
	}
	r := okfHandlerWithStub(t, stub)
	req, w := doJSON(t, "POST", "/v1/disk/okf/bundles/4/refresh", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestOkfHandler_RefreshBundle_NotFound(t *testing.T) {
	stub := &stubOkfSvc{
		refreshBundle: func(context.Context, uint64) (*model.OkfBundle, error) {
			return nil, service.ErrOkfBundleNotFound
		},
	}
	r := okfHandlerWithStub(t, stub)
	req, w := doJSON(t, "POST", "/v1/disk/okf/bundles/4/refresh", nil)
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestOkfHandler_UnregisterBundle_Success(t *testing.T) {
	called := false
	stub := &stubOkfSvc{
		unregisterBundle: func(id uint64) error {
			called = true
			if id != 3 {
				t.Errorf("id = %d, want 3", id)
			}
			return nil
		},
	}
	r := okfHandlerWithStub(t, stub)
	req, w := doJSON(t, "DELETE", "/v1/disk/okf/bundles/3", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if !called {
		t.Error("UnregisterBundle was not called")
	}
}

func TestOkfHandler_UnregisterBundle_NotFound(t *testing.T) {
	stub := &stubOkfSvc{
		unregisterBundle: func(uint64) error { return service.ErrOkfBundleNotFound },
	}
	r := okfHandlerWithStub(t, stub)
	req, w := doJSON(t, "DELETE", "/v1/disk/okf/bundles/3", nil)
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestOkfHandler_MissingTypeError(t *testing.T) {
	// RefreshBundle surfacing ErrOkfMissingType should map to 400 even though
	// the canonical source is WriteMarkdown; verify the error mapper covers it.
	stub := &stubOkfSvc{
		refreshBundle: func(context.Context, uint64) (*model.OkfBundle, error) {
			return nil, service.ErrOkfMissingType
		},
	}
	r := okfHandlerWithStub(t, stub)
	req, w := doJSON(t, "POST", "/v1/disk/okf/bundles/3/refresh", nil)
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("status = %d, want 400 (missing type)", w.Code)
	}
}

func TestOkfHandler_ReservedNameError(t *testing.T) {
	stub := &stubOkfSvc{
		refreshBundle: func(context.Context, uint64) (*model.OkfBundle, error) {
			return nil, service.ErrOkfReservedName
		},
	}
	r := okfHandlerWithStub(t, stub)
	req, w := doJSON(t, "POST", "/v1/disk/okf/bundles/3/refresh", nil)
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("status = %d, want 400 (reserved name)", w.Code)
	}
}

// Silence unused import warning if response ends up unused after refactor.
var _ = response.OK
