package handler

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/gin-gonic/gin"
)

// stubOkfShareReader is the configurable stub of okfShareReader.
type stubOkfShareReader struct {
	getBundle func(share *model.DiskShare, bundleID uint64) (*model.OkfBundle, error)
	listNodes func(share *model.DiskShare, bundleID uint64, typeF, tagF string) ([]model.OkfNode, error)
	subgraph  func(ctx context.Context, share *model.DiskShare, req service.ShareSubgraphRequest) (*service.SubgraphResponse, error)
	neighbors func(ctx context.Context, share *model.DiskShare, req service.ShareNeighborsRequest) (*service.NeighborsResponse, error)
	getNode   func(share *model.DiskShare, nodeID uint64) (*model.OkfNode, []byte, error)
}

func (s *stubOkfShareReader) GetBundle(share *model.DiskShare, bundleID uint64) (*model.OkfBundle, error) {
	if s.getBundle == nil {
		return nil, errors.New("getBundle not stubbed")
	}
	return s.getBundle(share, bundleID)
}

func (s *stubOkfShareReader) ListNodes(share *model.DiskShare, bundleID uint64, typeF, tagF string) ([]model.OkfNode, error) {
	if s.listNodes == nil {
		return nil, errors.New("listNodes not stubbed")
	}
	return s.listNodes(share, bundleID, typeF, tagF)
}

func (s *stubOkfShareReader) Subgraph(ctx context.Context, share *model.DiskShare, req service.ShareSubgraphRequest) (*service.SubgraphResponse, error) {
	if s.subgraph == nil {
		return nil, errors.New("subgraph not stubbed")
	}
	return s.subgraph(ctx, share, req)
}

func (s *stubOkfShareReader) Neighbors(ctx context.Context, share *model.DiskShare, req service.ShareNeighborsRequest) (*service.NeighborsResponse, error) {
	if s.neighbors == nil {
		return nil, errors.New("neighbors not stubbed")
	}
	return s.neighbors(ctx, share, req)
}

func (s *stubOkfShareReader) GetNode(share *model.DiskShare, nodeID uint64) (*model.OkfNode, []byte, error) {
	if s.getNode == nil {
		return nil, nil, errors.New("getNode not stubbed")
	}
	return s.getNode(share, nodeID)
}

// stubOkfShareShareSvc stubs the ShareService subset the OKF share handler
// uses (AccessShare). The stub mirrors the real service's classification so
// tests can drive each error branch (extract code / max visit / unknown).
type stubOkfShareShareSvc struct {
	byCode map[string]*model.DiskShare
	// accessErr, if non-nil, is returned regardless of code lookup. Used to
	// simulate "max visit limit reached" without modeling VisitCount.
	accessErr error
}

func (s *stubOkfShareShareSvc) AccessShare(code, extractCode, _, _ string) (*model.DiskShare, error) {
	if s.accessErr != nil {
		return nil, s.accessErr
	}
	sh, ok := s.byCode[code]
	if !ok {
		return nil, errors.New("share not found")
	}
	if sh.ExtractCode != "" && sh.ExtractCode != extractCode {
		return nil, errors.New("invalid extract code")
	}
	return sh, nil
}

// okfShareHandlerWithStub wires the public OKF share routes to a stub reader
// + stub share svc. Mirrors the production path-tree so route-param behavior
// is exercised end-to-end (including the :code/nodes/:nodeId vs :code/nodes
// ordering concern).
func okfShareHandlerWithStub(t *testing.T, reader *stubOkfShareReader, shares *stubOkfShareShareSvc) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &OkfShareHandler{reader: reader, shares: shares}
	r.GET("/v1/disk/share/:code/bundle", h.GetShareBundle)
	r.GET("/v1/disk/share/:code/nodes", h.ListShareNodes)
	r.GET("/v1/disk/share/:code/subgraph", h.GetShareSubgraph)
	r.GET("/v1/disk/share/:code/nodes/:nodeId/neighbors", h.GetShareNodeNeighbors)
	r.GET("/v1/disk/share/:code/nodes/:nodeId", h.GetShareNode)
	return r
}

func newBundleShare(code string, bundleID uint64, extractCode string) *model.DiskShare {
	return &model.DiskShare{
		ID:          1,
		ShareCode:   code,
		ResourceID:  bundleID,
		ResType:     "bundle",
		ExtractCode: extractCode,
		IsActive:    true,
	}
}

func TestOkfShareHandler_GetBundle_Success(t *testing.T) {
	reader := &stubOkfShareReader{
		getBundle: func(_ *model.DiskShare, id uint64) (*model.OkfBundle, error) {
			if id != 7 {
				t.Errorf("bundleID = %d, want 7", id)
			}
			return &model.OkfBundle{ID: 7, Title: "demo"}, nil
		},
	}
	shares := &stubOkfShareShareSvc{byCode: map[string]*model.DiskShare{
		"code123": newBundleShare("code123", 7, ""),
	}}
	r := okfShareHandlerWithStub(t, reader, shares)

	req, w := doJSON(t, "GET", "/v1/disk/share/code123/bundle?bundleId=7", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
}

func TestOkfShareHandler_GetBundle_UnknownCode(t *testing.T) {
	reader := &stubOkfShareReader{}
	shares := &stubOkfShareShareSvc{byCode: map[string]*model.DiskShare{}}
	r := okfShareHandlerWithStub(t, reader, shares)

	req, w := doJSON(t, "GET", "/v1/disk/share/missing/bundle?bundleId=7", nil)
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("status = %d, want 404 (unknown code)", w.Code)
	}
}

func TestOkfShareHandler_ExtractCodeMismatch(t *testing.T) {
	reader := &stubOkfShareReader{}
	shares := &stubOkfShareShareSvc{byCode: map[string]*model.DiskShare{
		"secret": newBundleShare("secret", 7, "1234"),
	}}
	r := okfShareHandlerWithStub(t, reader, shares)

	req, w := doJSON(t, "GET", "/v1/disk/share/secret/bundle?bundleId=7", nil)
	r.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Errorf("status = %d, want 403 (extract code mismatch)", w.Code)
	}
}

func TestOkfShareHandler_ExtractCodeMatch(t *testing.T) {
	reader := &stubOkfShareReader{
		getBundle: func(_ *model.DiskShare, _ uint64) (*model.OkfBundle, error) {
			return &model.OkfBundle{ID: 7, Title: "ok"}, nil
		},
	}
	shares := &stubOkfShareShareSvc{byCode: map[string]*model.DiskShare{
		"secret": newBundleShare("secret", 7, "1234"),
	}}
	r := okfShareHandlerWithStub(t, reader, shares)

	req, w := doJSON(t, "GET", "/v1/disk/share/secret/bundle?bundleId=7&extractCode=1234", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
}

func TestOkfShareHandler_ResTypeMismatch(t *testing.T) {
	// Share code is for a "file" but caller hits a bundle endpoint. Should
	// be rejected as 400 rather than leaking any data.
	reader := &stubOkfShareReader{}
	shares := &stubOkfShareShareSvc{byCode: map[string]*model.DiskShare{
		"fileshare": {ID: 2, ShareCode: "fileshare", ResourceID: 9, ResType: "file", IsActive: true},
	}}
	r := okfShareHandlerWithStub(t, reader, shares)

	req, w := doJSON(t, "GET", "/v1/disk/share/fileshare/bundle?bundleId=9", nil)
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("status = %d, want 400 (resType mismatch)", w.Code)
	}
}

func TestOkfShareHandler_ListNodes_Success(t *testing.T) {
	reader := &stubOkfShareReader{
		listNodes: func(_ *model.DiskShare, bundleID uint64, typeF, tagF string) ([]model.OkfNode, error) {
			if bundleID != 7 || typeF != "concept" || tagF != "core" {
				t.Errorf("unexpected args: bundle=%d type=%q tag=%q", bundleID, typeF, tagF)
			}
			return []model.OkfNode{{ID: 1, BundleID: 7, Type: "concept", Title: "n1"}}, nil
		},
	}
	shares := &stubOkfShareShareSvc{byCode: map[string]*model.DiskShare{
		"c": newBundleShare("c", 7, ""),
	}}
	r := okfShareHandlerWithStub(t, reader, shares)

	req, w := doJSON(t, "GET", "/v1/disk/share/c/nodes?bundleId=7&type=concept&tag=core", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
}

func TestOkfShareHandler_GetShareNode_Success(t *testing.T) {
	reader := &stubOkfShareReader{
		getNode: func(_ *model.DiskShare, nodeID uint64) (*model.OkfNode, []byte, error) {
			if nodeID != 42 {
				t.Errorf("nodeID = %d, want 42", nodeID)
			}
			return &model.OkfNode{ID: 42, BundleID: 7, Title: "n", FileID: 100}, []byte("# title\nbody"), nil
		},
	}
	shares := &stubOkfShareShareSvc{byCode: map[string]*model.DiskShare{
		"c": newBundleShare("c", 7, ""),
	}}
	r := okfShareHandlerWithStub(t, reader, shares)

	req, w := doJSON(t, "GET", "/v1/disk/share/c/nodes/42", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	resp := decodeResponse(t, w)
	data, _ := resp["data"].(map[string]any)
	if data["markdown"] != "# title\nbody" {
		t.Errorf("markdown = %q, want body", data["markdown"])
	}
}

func TestOkfShareHandler_GetShareNode_BundleMismatch(t *testing.T) {
	// Share is for bundle 7, node belongs to bundle 8. Reader returns
	// ErrOkfShareBundleMismatch → 400.
	reader := &stubOkfShareReader{
		getNode: func(_ *model.DiskShare, _ uint64) (*model.OkfNode, []byte, error) {
			return nil, nil, service.ErrOkfShareBundleMismatch
		},
	}
	shares := &stubOkfShareShareSvc{byCode: map[string]*model.DiskShare{
		"c": newBundleShare("c", 7, ""),
	}}
	r := okfShareHandlerWithStub(t, reader, shares)

	req, w := doJSON(t, "GET", "/v1/disk/share/c/nodes/42", nil)
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("status = %d, want 400 (bundle mismatch)", w.Code)
	}
}

func TestOkfShareHandler_Neighbors_Success(t *testing.T) {
	reader := &stubOkfShareReader{
		neighbors: func(_ context.Context, _ *model.DiskShare, req service.ShareNeighborsRequest) (*service.NeighborsResponse, error) {
			if req.NodeID != 42 {
				t.Errorf("nodeID = %d, want 42", req.NodeID)
			}
			return &service.NeighborsResponse{
				Nodes: []model.OkfNode{{ID: 43, BundleID: 7}},
				Edges: []model.OkfEdge{{ID: 1, SrcNodeID: 42, DstNodeID: 43}},
			}, nil
		},
	}
	shares := &stubOkfShareShareSvc{byCode: map[string]*model.DiskShare{
		"c": newBundleShare("c", 7, ""),
	}}
	r := okfShareHandlerWithStub(t, reader, shares)

	req, w := doJSON(t, "GET", "/v1/disk/share/c/nodes/42/neighbors?dir=out", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
}

func TestOkfShareHandler_Subgraph_Success(t *testing.T) {
	reader := &stubOkfShareReader{
		subgraph: func(_ context.Context, _ *model.DiskShare, req service.ShareSubgraphRequest) (*service.SubgraphResponse, error) {
			if req.BundleID != 7 {
				t.Errorf("bundleID = %d, want 7", req.BundleID)
			}
			return &service.SubgraphResponse{
				Nodes: []model.OkfNode{{ID: 1, BundleID: 7}},
				Edges: []model.OkfEdge{},
			}, nil
		},
	}
	shares := &stubOkfShareShareSvc{byCode: map[string]*model.DiskShare{
		"c": newBundleShare("c", 7, ""),
	}}
	r := okfShareHandlerWithStub(t, reader, shares)

	req, w := doJSON(t, "GET", "/v1/disk/share/c/subgraph?bundleId=7&maxNodes=100", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
}

func TestOkfShareHandler_MissingBundleId(t *testing.T) {
	reader := &stubOkfShareReader{}
	shares := &stubOkfShareShareSvc{byCode: map[string]*model.DiskShare{
		"c": newBundleShare("c", 7, ""),
	}}
	r := okfShareHandlerWithStub(t, reader, shares)

	req, w := doJSON(t, "GET", "/v1/disk/share/c/bundle", nil)
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("status = %d, want 400 (missing bundleId)", w.Code)
	}
}

func TestOkfShareHandler_InvalidNodeId(t *testing.T) {
	reader := &stubOkfShareReader{}
	shares := &stubOkfShareShareSvc{byCode: map[string]*model.DiskShare{
		"c": newBundleShare("c", 7, ""),
	}}
	r := okfShareHandlerWithStub(t, reader, shares)

	req := httptest.NewRequest("GET", "/v1/disk/share/c/nodes/abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("status = %d, want 400 (invalid nodeId)", w.Code)
	}
}

// TestOkfShareHandler_MaxVisitReached guards the AccessShare wiring: a share
// whose MaxVisit is exhausted must be rejected even on the OKF read paths
// (bundle / nodes / subgraph), which the previous GetShareByCode-based code
// bypassed.
func TestOkfShareHandler_MaxVisitReached(t *testing.T) {
	reader := &stubOkfShareReader{}
	shares := &stubOkfShareShareSvc{
		byCode: map[string]*model.DiskShare{
			"c": newBundleShare("c", 7, ""),
		},
		accessErr: errors.New("max visit limit reached"),
	}
	r := okfShareHandlerWithStub(t, reader, shares)

	req, w := doJSON(t, "GET", "/v1/disk/share/c/bundle?bundleId=7", nil)
	r.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Errorf("status = %d, want 403 (max visit reached); body=%s", w.Code, w.Body.String())
	}
}
