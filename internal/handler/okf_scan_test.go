package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/gin-gonic/gin"
)

// stubOkfScanSvc is a configurable stub of okfScanHandlerService. Each field
// is a function the handler calls; tests set the ones they exercise and
// leave the rest nil. A nil function returns a zero/error value so
// unconfigured paths fail loudly.
type stubOkfScanSvc struct {
	scanBundleLinksForHandler func(ctx context.Context, bundleID uint64, userID, department string) (*service.ScanReport, error)
	listBrokenLinks           func(ctx context.Context, bundleID uint64, userID, department string, cursor uint64, limit int) ([]service.BrokenLink, uint64, error)
	regenerateIndex           func(ctx context.Context, bundleID uint64, userID, department string) (uint32, time.Time, error)
}

func (s *stubOkfScanSvc) ScanBundleLinksForHandler(ctx context.Context, bundleID uint64, userID, department string) (*service.ScanReport, error) {
	if s.scanBundleLinksForHandler == nil {
		return nil, errors.New("not stubbed")
	}
	return s.scanBundleLinksForHandler(ctx, bundleID, userID, department)
}

func (s *stubOkfScanSvc) ListBrokenLinks(ctx context.Context, bundleID uint64, userID, department string, cursor uint64, limit int) ([]service.BrokenLink, uint64, error) {
	if s.listBrokenLinks == nil {
		return nil, 0, errors.New("not stubbed")
	}
	return s.listBrokenLinks(ctx, bundleID, userID, department, cursor, limit)
}

func (s *stubOkfScanSvc) RegenerateIndex(ctx context.Context, bundleID uint64, userID, department string) (uint32, time.Time, error) {
	if s.regenerateIndex == nil {
		return 0, time.Time{}, errors.New("not stubbed")
	}
	return s.regenerateIndex(ctx, bundleID, userID, department)
}

// okfScanHandlerWithStub builds a gin engine with the P2 maintenance routes
// wired to a stub service. Mirrors the production router's URL shape.
func okfScanHandlerWithStub(t *testing.T, stub *stubOkfScanSvc) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("userId", "user001"); c.Next() })
	h := &OkfScanHandler{svc: stub}
	v1 := r.Group("/v1/disk/okf")
	v1.POST("/bundles/:id/scan", h.ScanBundle)
	v1.GET("/bundles/:id/broken-links", h.ListBrokenLinks)
	v1.POST("/bundles/:id/regenerate-index", h.RegenerateIndex)
	return r
}

// TestOkfScanHandler_ScanBundle_Success verifies the scan returns the
// scanned-node count + broken-link count from the service report.
func TestOkfScanHandler_ScanBundle_Success(t *testing.T) {
	stub := &stubOkfScanSvc{
		scanBundleLinksForHandler: func(_ context.Context, bundleID uint64, userID, _ string) (*service.ScanReport, error) {
			if bundleID != 7 {
				t.Errorf("bundleID = %d, want 7", bundleID)
			}
			if userID != "user001" {
				t.Errorf("userID = %q, want user001", userID)
			}
			return &service.ScanReport{
				ScannedNodes: 4,
				BrokenLinks: []service.BrokenLink{
					{LinkInfo: service.LinkInfo{SrcRelPath: "a.md", DstRelPath: "b.md"}, Reason: service.BrokenReasonTargetNotFound},
				},
			}, nil
		},
	}
	r := okfScanHandlerWithStub(t, stub)
	req, w := doJSON(t, "POST", "/v1/disk/okf/bundles/7/scan", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	body := decodeResponse(t, w)
	data, _ := body["data"].(map[string]any)
	if data["scannedNodes"].(float64) != 4 {
		t.Errorf("scannedNodes = %v, want 4", data["scannedNodes"])
	}
	if data["brokenCount"].(float64) != 1 {
		t.Errorf("brokenCount = %v, want 1", data["brokenCount"])
	}
}

// TestOkfScanHandler_ScanBundle_NotFound covers the missing-bundle path. The
// handler must surface 404, not a 500.
func TestOkfScanHandler_ScanBundle_NotFound(t *testing.T) {
	stub := &stubOkfScanSvc{
		scanBundleLinksForHandler: func(_ context.Context, _ uint64, _, _ string) (*service.ScanReport, error) {
			return nil, service.ErrOkfBundleNotFound
		},
	}
	r := okfScanHandlerWithStub(t, stub)
	req, w := doJSON(t, "POST", "/v1/disk/okf/bundles/99/scan", nil)
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// TestOkfScanHandler_ScanBundle_BadID verifies an invalid :id is rejected
// before the service is called.
func TestOkfScanHandler_ScanBundle_BadID(t *testing.T) {
	stub := &stubOkfScanSvc{}
	r := okfScanHandlerWithStub(t, stub)
	req, w := doJSON(t, "POST", "/v1/disk/okf/bundles/abc/scan", nil)
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// TestOkfScanHandler_ListBrokenLinks_Success verifies pagination params parse
// correctly and the broken-link wire shape matches the contract.
func TestOkfScanHandler_ListBrokenLinks_Success(t *testing.T) {
	stub := &stubOkfScanSvc{
		listBrokenLinks: func(_ context.Context, bundleID uint64, userID, _ string, cursor uint64, limit int) ([]service.BrokenLink, uint64, error) {
			if bundleID != 7 {
				t.Errorf("bundleID = %d, want 7", bundleID)
			}
			if userID != "user001" {
				t.Errorf("userID = %q, want user001", userID)
			}
			if cursor != 0 {
				t.Errorf("cursor = %d, want 0", cursor)
			}
			if limit != 25 {
				t.Errorf("limit = %d, want 25", limit)
			}
			return []service.BrokenLink{
				{LinkInfo: service.LinkInfo{SrcNodeID: 5, SrcRelPath: "a.md", DstRelPath: "b.md", SrcLine: 3, LinkText: "b", LinkKind: service.LinkKindBundle}, Reason: service.BrokenReasonTargetNotFound},
			}, uint64(5), nil
		},
	}
	r := okfScanHandlerWithStub(t, stub)
	req, w := doJSON(t, "GET", "/v1/disk/okf/bundles/7/broken-links?limit=25", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	body := decodeResponse(t, w)
	data, _ := body["data"].(map[string]any)
	if data["nextCursor"].(float64) != 5 {
		t.Errorf("nextCursor = %v, want 5", data["nextCursor"])
	}
	links, _ := data["brokenLinks"].([]any)
	if len(links) != 1 {
		t.Fatalf("brokenLinks len = %d, want 1", len(links))
	}
	first, _ := links[0].(map[string]any)
	if first["reason"] != service.BrokenReasonTargetNotFound {
		t.Errorf("reason = %v, want %q", first["reason"], service.BrokenReasonTargetNotFound)
	}
	if first["srcRelPath"] != "a.md" {
		t.Errorf("srcRelPath = %v, want a.md", first["srcRelPath"])
	}
}

// TestOkfScanHandler_ListBrokenLinks_LimitClampedTo50 verifies a malicious
// "limit=9999" is clamped to the 50-entry ceiling.
func TestOkfScanHandler_ListBrokenLinks_LimitClampedTo50(t *testing.T) {
	var captured int
	stub := &stubOkfScanSvc{
		listBrokenLinks: func(_ context.Context, _ uint64, _, _ string, _ uint64, limit int) ([]service.BrokenLink, uint64, error) {
			captured = limit
			return nil, 0, nil
		},
	}
	r := okfScanHandlerWithStub(t, stub)
	req, w := doJSON(t, "GET", "/v1/disk/okf/bundles/7/broken-links?limit=9999", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if captured != 50 {
		t.Errorf("limit clamped to %d, want 50", captured)
	}
}

// TestOkfScanHandler_ListBrokenLinks_Forbidden covers the visibility-denied
// path. The handler must return 403, not a 500.
func TestOkfScanHandler_ListBrokenLinks_Forbidden(t *testing.T) {
	stub := &stubOkfScanSvc{
		listBrokenLinks: func(_ context.Context, _ uint64, _, _ string, _ uint64, _ int) ([]service.BrokenLink, uint64, error) {
			return nil, 0, service.ErrOkfForbidden
		},
	}
	r := okfScanHandlerWithStub(t, stub)
	req, w := doJSON(t, "GET", "/v1/disk/okf/bundles/7/broken-links", nil)
	r.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

// TestOkfScanHandler_RegenerateIndex_Success verifies the wire shape includes
// both indexVersion and regeneratedAt.
func TestOkfScanHandler_RegenerateIndex_Success(t *testing.T) {
	stub := &stubOkfScanSvc{
		regenerateIndex: func(_ context.Context, bundleID uint64, userID, _ string) (uint32, time.Time, error) {
			if bundleID != 7 {
				t.Errorf("bundleID = %d, want 7", bundleID)
			}
			if userID != "user001" {
				t.Errorf("userID = %q, want user001", userID)
			}
			return 3, time.Date(2026, 6, 27, 10, 0, 0, 0, time.UTC), nil
		},
	}
	r := okfScanHandlerWithStub(t, stub)
	req, w := doJSON(t, "POST", "/v1/disk/okf/bundles/7/regenerate-index", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	body := decodeResponse(t, w)
	data, _ := body["data"].(map[string]any)
	if data["indexVersion"].(float64) != 3 {
		t.Errorf("indexVersion = %v, want 3", data["indexVersion"])
	}
	if data["regeneratedAt"] == nil {
		t.Errorf("regeneratedAt missing")
	}
}

// TestOkfScanHandler_RegenerateIndex_NotFound covers the missing-bundle path.
func TestOkfScanHandler_RegenerateIndex_NotFound(t *testing.T) {
	stub := &stubOkfScanSvc{
		regenerateIndex: func(_ context.Context, _ uint64, _, _ string) (uint32, time.Time, error) {
			return 0, time.Time{}, service.ErrOkfBundleNotFound
		},
	}
	r := okfScanHandlerWithStub(t, stub)
	req, w := doJSON(t, "POST", "/v1/disk/okf/bundles/99/regenerate-index", nil)
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// TestOkfScanHandler_ScanBundle_Forbidden covers the cross-user visibility-denied
// path. The service returns ErrOkfForbidden when the caller is not in the
// bundle's department — the handler must surface 403, never 500, so the
// privilege boundary stays legible to clients.
func TestOkfScanHandler_ScanBundle_Forbidden(t *testing.T) {
	stub := &stubOkfScanSvc{
		scanBundleLinksForHandler: func(_ context.Context, _ uint64, _, _ string) (*service.ScanReport, error) {
			return nil, service.ErrOkfForbidden
		},
	}
	r := okfScanHandlerWithStub(t, stub)
	req, w := doJSON(t, "POST", "/v1/disk/okf/bundles/7/scan", nil)
	r.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

// TestOkfScanHandler_RegenerateIndex_Forbidden mirrors the scan forbidden test
// on the regenerate-index endpoint. ACL denial must map to 403, not the 500
// path that would leak the underlying error.
func TestOkfScanHandler_RegenerateIndex_Forbidden(t *testing.T) {
	stub := &stubOkfScanSvc{
		regenerateIndex: func(_ context.Context, _ uint64, _, _ string) (uint32, time.Time, error) {
			return 0, time.Time{}, service.ErrOkfForbidden
		},
	}
	r := okfScanHandlerWithStub(t, stub)
	req, w := doJSON(t, "POST", "/v1/disk/okf/bundles/7/regenerate-index", nil)
	r.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

// Ensure http import stays referenced (httptest is used by doJSON in another
// test file; this var keeps the file self-contained when filters change).
var _ = http.Request{Method: http.MethodGet}
var _ = httptest.NewRecorder
