package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentdisk/agent-disk/config"
	"github.com/agentdisk/agent-disk/internal/feature"
	"github.com/gin-gonic/gin"
)

// newFeatureRouter wires a FeatureHandler against a fresh in-memory registry.
// cfgPath="" so the registry doesn't try to fsnotify-watch anything during
// tests; Set() will return a persistence error but still flip the in-memory
// flag, which is what the handler tests care about.
func newFeatureRouter(t *testing.T) (*gin.Engine, *feature.Registry) {
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

	h := NewFeatureHandler(reg)
	r := gin.New()
	r.GET("/features", h.List)
	r.PATCH("/features", h.Update)
	return r, reg
}

func TestFeatureHandler_ListReturnsAll(t *testing.T) {
	r, _ := newFeatureRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/features", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var env struct {
		Code int `json:"code"`
		Data struct {
			Features []feature.FlagState `json:"features"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(env.Data.Features) != 3 {
		t.Errorf("features len = %d, want 3", len(env.Data.Features))
	}
	for _, f := range env.Data.Features {
		if !f.Enabled {
			t.Errorf("flag %s default = false, want true", f.Name)
		}
	}
}

func TestFeatureHandler_UpdateFlipsFlag(t *testing.T) {
	r, reg := newFeatureRouter(t)

	w := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"name":"okfReader","enabled":false}`)
	req, _ := http.NewRequest("PATCH", "/features", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200 (body=%s)", w.Code, w.Body.String())
	}
	if reg.IsEnabled(feature.FlagOkfReader) {
		t.Errorf("after PATCH, registry still has reader=true")
	}
}

func TestFeatureHandler_UpdateUnknownFlag400(t *testing.T) {
	r, _ := newFeatureRouter(t)

	w := httptest.NewRecorder()
	body := bytes.NewBufferString(`{"name":"bogus","enabled":true}`)
	req, _ := http.NewRequest("PATCH", "/features", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("status = %d, want 400 for unknown flag", w.Code)
	}
}

func TestFeatureHandler_UpdateInvalidBody400(t *testing.T) {
	r, _ := newFeatureRouter(t)

	w := httptest.NewRecorder()
	body := bytes.NewBufferString(`{}`)
	req, _ := http.NewRequest("PATCH", "/features", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("status = %d, want 400 for empty body", w.Code)
	}
}
