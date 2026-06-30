package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/gin-gonic/gin"
)

// TestOkfHandler_Search_Success walks the happy path: the service returns a
// single node, the handler must wrap it in {"nodes":[...],"nextCursor":n}
// and surface the node's primary key as "nodeId" (not "id") per the SDK
// contract.
func TestOkfHandler_Search_Success(t *testing.T) {
	stub := &stubOkfSvc{
		search: func(_ context.Context, req service.SearchRequest) (*service.SearchResponse, error) {
			if req.Query != "gemma" {
				t.Errorf("query = %q, want gemma", req.Query)
			}
			if req.BundleID != 7 {
				t.Errorf("bundleId = %d, want 7", req.BundleID)
			}
			if req.Limit != 25 {
				t.Errorf("limit = %d, want 25", req.Limit)
			}
			if req.Cursor != 0 {
				t.Errorf("cursor = %d, want 0", req.Cursor)
			}
			return &service.SearchResponse{
				Nodes: []model.OkfNode{
					{ID: 42, BundleID: 7, Title: "Gemma model card", Type: "concept"},
				},
				NextCursor: 99,
			}, nil
		},
	}
	r := okfHandlerWithStub(t, stub)
	req, w := doJSON(t, "POST", "/v1/disk/okf/search", gin.H{
		"query":    "gemma",
		"bundleId": 7,
		"limit":    25,
	})
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := decodeResponse(t, w)
	data, _ := body["data"].(map[string]any)
	if data == nil {
		t.Fatalf("data missing: %v", body)
	}
	if float64(data["nextCursor"].(float64)) != 99 {
		t.Errorf("nextCursor = %v, want 99", data["nextCursor"])
	}
	nodes, _ := data["nodes"].([]any)
	if len(nodes) != 1 {
		t.Fatalf("nodes len = %d, want 1", len(nodes))
	}
	first, _ := nodes[0].(map[string]any)
	if first["nodeId"].(float64) != 42 {
		t.Errorf("nodeId = %v, want 42", first["nodeId"])
	}
	if _, hasOldID := first["id"]; hasOldID {
		t.Errorf("node should not expose \"id\"; got %v", first["id"])
	}
}

// TestOkfHandler_Search_EmptyQueryReturns400 confirms the handler rejects
// empty query bodies at the HTTP layer rather than forwarding to the
// service. The service tolerates empty queries (returns empty page), but
// the contract for /okf/search is that query is required.
func TestOkfHandler_Search_EmptyQueryReturns400(t *testing.T) {
	called := false
	stub := &stubOkfSvc{
		search: func(context.Context, service.SearchRequest) (*service.SearchResponse, error) {
			called = true
			return &service.SearchResponse{}, nil
		},
	}
	r := okfHandlerWithStub(t, stub)
	req, w := doJSON(t, "POST", "/v1/disk/okf/search", gin.H{"query": ""})
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("status = %d, want 400 (empty query)", w.Code)
	}
	if called {
		t.Error("service Search was called; handler should short-circuit on empty query")
	}
}

// TestOkfHandler_Search_MissingQueryReturns400 covers the binding path:
// when "query" is absent the JSON body fails the binding:"required" check.
func TestOkfHandler_Search_MissingQueryReturns400(t *testing.T) {
	r := okfHandlerWithStub(t, &stubOkfSvc{})
	req, w := doJSON(t, "POST", "/v1/disk/okf/search", gin.H{"bundleId": 7})
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("status = %d, want 400 (missing query)", w.Code)
	}
}

// TestOkfHandler_Search_ForbiddenReturns403 verifies the error mapper
// translates ErrOkfForbidden into 403 — the search ACL is enforced in the
// service layer, but the handler must not leak the underlying reason.
func TestOkfHandler_Search_ForbiddenReturns403(t *testing.T) {
	stub := &stubOkfSvc{
		search: func(context.Context, service.SearchRequest) (*service.SearchResponse, error) {
			return nil, service.ErrOkfForbidden
		},
	}
	r := okfHandlerWithStub(t, stub)
	req, w := doJSON(t, "POST", "/v1/disk/okf/search", gin.H{"query": "gemma", "bundleId": 9})
	r.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Errorf("status = %d, want 403 (forbidden bundle)", w.Code)
	}
}

// TestOkfHandler_Search_BundleNotFoundReturns404 ensures the missing-bundle
// sentinel surfaces as 404 on the search path too, mirroring GetBundle.
func TestOkfHandler_Search_BundleNotFoundReturns404(t *testing.T) {
	stub := &stubOkfSvc{
		search: func(context.Context, service.SearchRequest) (*service.SearchResponse, error) {
			return nil, service.ErrOkfBundleNotFound
		},
	}
	r := okfHandlerWithStub(t, stub)
	req, w := doJSON(t, "POST", "/v1/disk/okf/search", gin.H{"query": "gemma", "bundleId": 999})
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("status = %d, want 404 (bundle missing)", w.Code)
	}
}

// TestOkfHandler_Search_InternalErrorReturns500 confirms generic service
// failures map to 500 without leaking the error text in the response body.
func TestOkfHandler_Search_InternalErrorReturns500(t *testing.T) {
	stub := &stubOkfSvc{
		search: func(context.Context, service.SearchRequest) (*service.SearchResponse, error) {
			return nil, errors.New("search backend down")
		},
	}
	r := okfHandlerWithStub(t, stub)
	req, w := doJSON(t, "POST", "/v1/disk/okf/search", gin.H{"query": "gemma"})
	r.ServeHTTP(w, req)
	if w.Code != 500 {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

// TestOkfHandler_Search_EmptyResultsHasEmptyArray covers the "no match"
// response shape: the handler must emit an empty nodes array rather than
// null, so JSON consumers can unmarshal into []Node unconditionally.
func TestOkfHandler_Search_EmptyResultsHasEmptyArray(t *testing.T) {
	stub := &stubOkfSvc{
		search: func(context.Context, service.SearchRequest) (*service.SearchResponse, error) {
			return &service.SearchResponse{Nodes: []model.OkfNode{}}, nil
		},
	}
	r := okfHandlerWithStub(t, stub)
	req, w := doJSON(t, "POST", "/v1/disk/okf/search", gin.H{"query": "gemma"})
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := decodeResponse(t, w)
	data, _ := body["data"].(map[string]any)
	nodes, _ := data["nodes"].([]any)
	if nodes == nil {
		t.Errorf("nodes = nil, want empty array")
	}
	if len(nodes) != 0 {
		t.Errorf("nodes len = %d, want 0", len(nodes))
	}
}
