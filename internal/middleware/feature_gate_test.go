package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentdisk/agent-disk/config"
	"github.com/agentdisk/agent-disk/internal/feature"
	"github.com/gin-gonic/gin"
)

// setupFlagRouter wires a gin engine with the RequireFeature middleware on
// a single GET /flagged route, plus an unflagged GET /open route for
// comparison. Returns the registry so each test can flip flags.
func setupFlagRouter(t *testing.T, name feature.FlagName) (*gin.Engine, *feature.Registry) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	cfg.Features.OkfReader = true
	cfg.Features.OkfWriter = true
	cfg.Features.OkfGraphBFS = true
	reg, err := feature.NewRegistry(cfg, "")
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })

	r := gin.New()
	r.GET("/open", func(c *gin.Context) { c.String(200, "ok") })
	r.GET("/flagged", RequireFeature(reg, name), func(c *gin.Context) {
		c.String(200, "allowed")
	})
	return r, reg
}

func TestRequireFeature_FlagOnAllows(t *testing.T) {
	r, _ := setupFlagRouter(t, feature.FlagOkfReader)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/flagged", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200 when flag on", w.Code)
	}
	if w.Body.String() != "allowed" {
		t.Errorf("body = %q, want %q", w.Body.String(), "allowed")
	}
}

func TestRequireFeature_FlagOffBlocks(t *testing.T) {
	r, reg := setupFlagRouter(t, feature.FlagOkfReader)
	if err := reg.Set(feature.FlagOkfReader, false); err != nil {
		t.Fatalf("Set: %v", err)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/flagged", nil)
	r.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Errorf("status = %d, want 403 when flag off", w.Code)
	}
}

func TestRequireFeature_OnlyNamedFlagMatters(t *testing.T) {
	// Flipping writer should not affect a reader-gated route.
	r, reg := setupFlagRouter(t, feature.FlagOkfReader)
	if err := reg.Set(feature.FlagOkfWriter, false); err != nil {
		t.Fatalf("Set writer: %v", err)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/flagged", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200 (writer off should not affect reader route)", w.Code)
	}
}

func TestRequireFeature_UnflaggedRouteUnaffected(t *testing.T) {
	r, reg := setupFlagRouter(t, feature.FlagOkfReader)
	if err := reg.Set(feature.FlagOkfReader, false); err != nil {
		t.Fatalf("Set: %v", err)
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/open", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200 on unflagged route", w.Code)
	}
}

func TestRequireFeature_FlipBackAllows(t *testing.T) {
	// Disable then re-enable — the route should be allowed again without a
	// restart, exercising the atomic.Pointer swap path.
	r, reg := setupFlagRouter(t, feature.FlagOkfReader)
	_ = reg.Set(feature.FlagOkfReader, false)
	_ = reg.Set(feature.FlagOkfReader, true)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/flagged", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200 after re-enable", w.Code)
	}
}
