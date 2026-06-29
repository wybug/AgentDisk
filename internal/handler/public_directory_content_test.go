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
	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/gin-gonic/gin"
)

// stubContentWriter is a configurable stub of okfContentWriter. The
// writeMarkdown field captures the request so tests can assert on it.
type stubContentWriter struct {
	writeMarkdown func(ctx context.Context, req service.WriteMarkdownRequest) (*model.OkfNode, error)
	lastReq       *service.WriteMarkdownRequest
}

func (s *stubContentWriter) WriteMarkdown(ctx context.Context, req service.WriteMarkdownRequest) (*model.OkfNode, error) {
	s.lastReq = &req
	if s.writeMarkdown == nil {
		return nil, errors.New("not stubbed")
	}
	return s.writeMarkdown(ctx, req)
}

func newContentHandlerWithStub(stub *stubContentWriter) (*gin.Engine, *PublicDirectoryContentHandler) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("userId", "user001"); c.Next() })
	h := &PublicDirectoryContentHandler{okfSvc: stub}
	pd := r.Group("/v1/disk/public-directories")
	pd.POST("/:id/files/content", h.WriteContent)
	return r, h
}

func TestPublicDirectoryContentHandler_WriteContent_Success(t *testing.T) {
	stub := &stubContentWriter{
		writeMarkdown: func(_ context.Context, req service.WriteMarkdownRequest) (*model.OkfNode, error) {
			if req.PublicDirectoryID != 7 {
				t.Errorf("publicDirectoryId = %d, want 7", req.PublicDirectoryID)
			}
			if req.RelPath != "concepts/gemma.md" {
				t.Errorf("relPath = %q, want concepts/gemma.md", req.RelPath)
			}
			if string(req.Content) != "hello" {
				t.Errorf("content = %q, want hello", string(req.Content))
			}
			return &model.OkfNode{ID: 9, BundleID: 1, RelPath: req.RelPath, Type: "concept"}, nil
		},
	}
	r, _ := newContentHandlerWithStub(stub)

	body, _ := json.Marshal(gin.H{"relPath": "concepts/gemma.md", "content": "hello"})
	req, _ := http.NewRequest("POST", "/v1/disk/public-directories/7/files/content", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 201 {
		t.Fatalf("status = %d, want 201 (body=%s)", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	data, _ := resp["data"].(map[string]any)
	if data["relPath"] != "concepts/gemma.md" {
		t.Errorf("data.relPath = %v", data["relPath"])
	}
}

func TestPublicDirectoryContentHandler_WriteContent_BadBody(t *testing.T) {
	r, _ := newContentHandlerWithStub(&stubContentWriter{})
	body, _ := json.Marshal(gin.H{"content": "x"}) // missing relPath
	req, _ := http.NewRequest("POST", "/v1/disk/public-directories/7/files/content", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestPublicDirectoryContentHandler_WriteContent_InvalidID(t *testing.T) {
	r, _ := newContentHandlerWithStub(&stubContentWriter{})
	req, _ := http.NewRequest("POST", "/v1/disk/public-directories/abc/files/content", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("status = %d, want 400 (invalid id)", w.Code)
	}
}

func TestPublicDirectoryContentHandler_WriteContent_RejectsNonMarkdown(t *testing.T) {
	r, _ := newContentHandlerWithStub(&stubContentWriter{})
	body, _ := json.Marshal(gin.H{"relPath": "x.md", "content": "x", "contentType": "application/pdf"})
	req, _ := http.NewRequest("POST", "/v1/disk/public-directories/7/files/content", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("status = %d, want 400 (only markdown accepted)", w.Code)
	}
}

func TestPublicDirectoryContentHandler_WriteContent_AcceptsMarkdownVariants(t *testing.T) {
	cases := []string{
		"",
		"text/markdown",
		"text/markdown; charset=utf-8",
		"text/x-markdown",
		"TEXT/MARKDOWN",
	}
	for _, ct := range cases {
		stub := &stubContentWriter{
			writeMarkdown: func(context.Context, service.WriteMarkdownRequest) (*model.OkfNode, error) {
				return &model.OkfNode{ID: 1}, nil
			},
		}
		r, _ := newContentHandlerWithStub(stub)
		body, _ := json.Marshal(gin.H{"relPath": "x.md", "content": "x", "contentType": ct})
		req, _ := http.NewRequest("POST", "/v1/disk/public-directories/7/files/content", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 201 {
			t.Errorf("contentType=%q: status = %d, want 201", ct, w.Code)
		}
	}
}

func TestPublicDirectoryContentHandler_WriteContent_MissingTypeMaps400(t *testing.T) {
	stub := &stubContentWriter{
		writeMarkdown: func(context.Context, service.WriteMarkdownRequest) (*model.OkfNode, error) {
			return nil, service.ErrOkfMissingType
		},
	}
	r, _ := newContentHandlerWithStub(stub)
	body, _ := json.Marshal(gin.H{"relPath": "x.md", "content": "x"})
	req, _ := http.NewRequest("POST", "/v1/disk/public-directories/7/files/content", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("status = %d, want 400 (missing type)", w.Code)
	}
}

func TestPublicDirectoryContentHandler_WriteContent_NotRootMaps400(t *testing.T) {
	stub := &stubContentWriter{
		writeMarkdown: func(context.Context, service.WriteMarkdownRequest) (*model.OkfNode, error) {
			return nil, service.ErrOkfNotBundleRoot
		},
	}
	r, _ := newContentHandlerWithStub(stub)
	body, _ := json.Marshal(gin.H{"relPath": "x.md", "content": "x"})
	req, _ := http.NewRequest("POST", "/v1/disk/public-directories/7/files/content", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("status = %d, want 400 (not bundle root)", w.Code)
	}
}

func TestPublicDirectoryContentHandler_WriteContent_ReservedNameMaps400(t *testing.T) {
	stub := &stubContentWriter{
		writeMarkdown: func(context.Context, service.WriteMarkdownRequest) (*model.OkfNode, error) {
			return nil, service.ErrOkfReservedName
		},
	}
	r, _ := newContentHandlerWithStub(stub)
	body, _ := json.Marshal(gin.H{"relPath": "log.md", "content": "x"})
	req, _ := http.NewRequest("POST", "/v1/disk/public-directories/7/files/content", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("status = %d, want 400 (reserved name)", w.Code)
	}
}

func TestPublicDirectoryContentHandler_WriteContent_InternalError(t *testing.T) {
	stub := &stubContentWriter{
		writeMarkdown: func(context.Context, service.WriteMarkdownRequest) (*model.OkfNode, error) {
			return nil, errors.New("oss down")
		},
	}
	r, _ := newContentHandlerWithStub(stub)
	body, _ := json.Marshal(gin.H{"relPath": "x.md", "content": "x"})
	req, _ := http.NewRequest("POST", "/v1/disk/public-directories/7/files/content", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 500 {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestPublicDirectoryContentHandler_IsMarkdownContentType(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", true},
		{"text/markdown", true},
		{"text/markdown; charset=utf-8", true},
		{"text/x-markdown", true},
		{"  text/markdown  ", true},
		{"TEXT/MARKDOWN", true},
		{"application/pdf", false},
		{"text/plain", false},
		{"text/markdownx", false},
	}
	for _, tc := range cases {
		got := isMarkdownContentType(tc.in)
		if got != tc.want {
			t.Errorf("isMarkdownContentType(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
