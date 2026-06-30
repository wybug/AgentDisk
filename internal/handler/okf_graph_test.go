package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/gin-gonic/gin"
)

// doJSON + decodeResponse + okfHandlerWithStub live in okf_test.go. The
// graph tests reuse them, so this file only defines the per-route tests.

// TestOkfHandler_Neighbors_Success covers the GET 1-hop happy path: the
// service returns 2 neighbors + 1 edge; the handler must surface them under
// the {nodes, edges} envelope.
func TestOkfHandler_Neighbors_Success(t *testing.T) {
	stub := &stubOkfSvc{
		neighbors: func(_ context.Context, req service.NeighborsRequest) (*service.NeighborsResponse, error) {
			if req.NodeID != 11 {
				t.Errorf("nodeId = %d, want 11", req.NodeID)
			}
			if req.Direction != "out" {
				t.Errorf("dir = %q, want out", req.Direction)
			}
			return &service.NeighborsResponse{
				Nodes: []model.OkfNode{{ID: 22, Title: "B"}},
				Edges: []model.OkfEdge{{ID: 1, SrcNodeID: 11, DstNodeID: 22, DstExists: true}},
			}, nil
		},
	}
	r := okfHandlerWithStub(t, stub)
	req, _ := http.NewRequest("GET", "/v1/disk/okf/nodes/11/neighbors?dir=out", nil)
	w := httptestOKF(t)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := decodeResponse(t, w)
	data, _ := body["data"].(map[string]any)
	nodes, _ := data["nodes"].([]any)
	if len(nodes) != 1 {
		t.Errorf("nodes len = %d, want 1", len(nodes))
	}
	edges, _ := data["edges"].([]any)
	if len(edges) != 1 {
		t.Errorf("edges len = %d, want 1", len(edges))
	}
}

// TestOkfHandler_Neighbors_InvalidID surfaces the parse failure as 400.
func TestOkfHandler_Neighbors_InvalidID(t *testing.T) {
	r := okfHandlerWithStub(t, &stubOkfSvc{})
	req, _ := http.NewRequest("GET", "/v1/disk/okf/nodes/abc/neighbors", nil)
	w := httptestOKF(t)
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// TestOkfHandler_Neighbors_NodeNotFoundMaps404 confirms ErrOkfNodeNotFound
// maps to 404 via the graph error mapper.
func TestOkfHandler_Neighbors_NodeNotFoundMaps404(t *testing.T) {
	stub := &stubOkfSvc{
		neighbors: func(context.Context, service.NeighborsRequest) (*service.NeighborsResponse, error) {
			return nil, service.ErrOkfNodeNotFound
		},
	}
	r := okfHandlerWithStub(t, stub)
	req, _ := http.NewRequest("GET", "/v1/disk/okf/nodes/99/neighbors", nil)
	w := httptestOKF(t)
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// TestOkfHandler_Neighbors_ForbiddenMaps403 confirms the ACL error maps to
// 403 on the graph path too — same behavior as the rest of the OKF reader.
func TestOkfHandler_Neighbors_ForbiddenMaps403(t *testing.T) {
	stub := &stubOkfSvc{
		neighbors: func(context.Context, service.NeighborsRequest) (*service.NeighborsResponse, error) {
			return nil, service.ErrOkfForbidden
		},
	}
	r := okfHandlerWithStub(t, stub)
	req, _ := http.NewRequest("GET", "/v1/disk/okf/nodes/11/neighbors", nil)
	w := httptestOKF(t)
	r.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

// TestOkfHandler_Reachable_Success walks the BFS response shape: {nodes:[...]}.
func TestOkfHandler_Reachable_Success(t *testing.T) {
	stub := &stubOkfSvc{
		reachable: func(_ context.Context, req service.ReachableRequest) (*service.ReachableResponse, error) {
			if req.Depth != 2 {
				t.Errorf("depth = %d, want 2", req.Depth)
			}
			return &service.ReachableResponse{Nodes: []model.OkfNode{{ID: 22, Title: "B"}}}, nil
		},
	}
	r := okfHandlerWithStub(t, stub)
	req, w := doJSON(t, "POST", "/v1/disk/okf/nodes/11/reachable", gin.H{"depth": 2})
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := decodeResponse(t, w)
	data, _ := body["data"].(map[string]any)
	nodes, _ := data["nodes"].([]any)
	if len(nodes) != 1 {
		t.Errorf("nodes len = %d, want 1", len(nodes))
	}
}

// TestOkfHandler_Reachable_MissingDepthReturns400 confirms the binding
// catches a body without the required depth field.
func TestOkfHandler_Reachable_MissingDepthReturns400(t *testing.T) {
	r := okfHandlerWithStub(t, &stubOkfSvc{})
	req, w := doJSON(t, "POST", "/v1/disk/okf/nodes/11/reachable", gin.H{"types": []string{"concept"}})
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("status = %d, want 400 (missing depth)", w.Code)
	}
}

// TestOkfHandler_ShortestPath_Success confirms the path is returned as an
// ordered node slice under the "path" key, plus a found flag.
func TestOkfHandler_ShortestPath_Success(t *testing.T) {
	stub := &stubOkfSvc{
		shortestPath: func(_ context.Context, req service.ShortestPathRequest) (*service.ShortestPathResponse, error) {
			if req.SrcNodeID != 11 || req.DstNodeID != 13 {
				t.Errorf("src=%d dst=%d, want 11/13", req.SrcNodeID, req.DstNodeID)
			}
			return &service.ShortestPathResponse{
				Nodes: []model.OkfNode{{ID: 11}, {ID: 12}, {ID: 13}},
				Found: true,
			}, nil
		},
	}
	r := okfHandlerWithStub(t, stub)
	req, w := doJSON(t, "POST", "/v1/disk/okf/paths/shortest", gin.H{"src": 11, "dst": 13})
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := decodeResponse(t, w)
	data, _ := body["data"].(map[string]any)
	if data["found"] != true {
		t.Errorf("found = %v, want true", data["found"])
	}
	path, _ := data["path"].([]any)
	if len(path) != 3 {
		t.Errorf("path len = %d, want 3", len(path))
	}
}

// TestOkfHandler_ShortestPath_NotFoundReturns200 confirms the not-found case
// is a 200 with found=false rather than a 404. The endpoint exists and ran;
// it just found no path.
func TestOkfHandler_ShortestPath_NotFoundReturns200(t *testing.T) {
	stub := &stubOkfSvc{
		shortestPath: func(context.Context, service.ShortestPathRequest) (*service.ShortestPathResponse, error) {
			return &service.ShortestPathResponse{Found: false}, nil
		},
	}
	r := okfHandlerWithStub(t, stub)
	req, w := doJSON(t, "POST", "/v1/disk/okf/paths/shortest", gin.H{"src": 11, "dst": 99})
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("status = %d, want 200 (found=false is still OK)", w.Code)
	}
}

// TestOkfHandler_ShortestPath_InternalErrorMaps500 confirms a generic service
// failure maps to 500.
func TestOkfHandler_ShortestPath_InternalErrorMaps500(t *testing.T) {
	stub := &stubOkfSvc{
		shortestPath: func(context.Context, service.ShortestPathRequest) (*service.ShortestPathResponse, error) {
			return nil, errors.New("bfs overflow")
		},
	}
	r := okfHandlerWithStub(t, stub)
	req, w := doJSON(t, "POST", "/v1/disk/okf/paths/shortest", gin.H{"src": 11, "dst": 13})
	r.ServeHTTP(w, req)
	if w.Code != 500 {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

// TestOkfHandler_Subgraph_Success verifies the {nodes, edges} envelope and
// that the bundleId is forwarded to the service.
func TestOkfHandler_Subgraph_Success(t *testing.T) {
	stub := &stubOkfSvc{
		subgraph: func(_ context.Context, req service.SubgraphRequest) (*service.SubgraphResponse, error) {
			if req.BundleID != 7 {
				t.Errorf("bundleId = %d, want 7", req.BundleID)
			}
			return &service.SubgraphResponse{
				Nodes: []model.OkfNode{{ID: 1, Title: "A"}},
				Edges: []model.OkfEdge{{ID: 1, SrcNodeID: 1, DstNodeID: 2, DstExists: true}},
			}, nil
		},
	}
	r := okfHandlerWithStub(t, stub)
	req, w := doJSON(t, "POST", "/v1/disk/okf/subgraph", gin.H{"bundleId": 7})
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := decodeResponse(t, w)
	data, _ := body["data"].(map[string]any)
	if _, ok := data["nodes"]; !ok {
		t.Errorf("data missing nodes key: %v", data)
	}
	if _, ok := data["edges"]; !ok {
		t.Errorf("data missing edges key: %v", data)
	}
}

// TestOkfHandler_Subgraph_MissingBundleReturns400 confirms the binding catches
// a body without bundleId.
func TestOkfHandler_Subgraph_MissingBundleReturns400(t *testing.T) {
	r := okfHandlerWithStub(t, &stubOkfSvc{})
	req, w := doJSON(t, "POST", "/v1/disk/okf/subgraph", gin.H{"types": []string{"concept"}})
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// TestOkfHandler_Stats_Success covers the GET stats path: the response
// carries nodeCount, edgeTotal/Live/Broken, and a types breakdown.
func TestOkfHandler_Stats_Success(t *testing.T) {
	stub := &stubOkfSvc{
		stats: func(_ context.Context, req service.StatsRequest) (*service.StatsResponse, error) {
			if req.BundleID != 7 {
				t.Errorf("bundleId = %d, want 7", req.BundleID)
			}
			return &service.StatsResponse{
				NodeCount:  3,
				EdgeTotal:  4,
				EdgeLive:   3,
				EdgeBroken: 1,
			}, nil
		},
	}
	r := okfHandlerWithStub(t, stub)
	req, _ := http.NewRequest("GET", "/v1/disk/okf/bundles/7/stats", nil)
	w := httptestOKF(t)
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := decodeResponse(t, w)
	data, _ := body["data"].(map[string]any)
	if data["nodeCount"].(float64) != 3 {
		t.Errorf("nodeCount = %v, want 3", data["nodeCount"])
	}
	if data["edgeBroken"].(float64) != 1 {
		t.Errorf("edgeBroken = %v, want 1", data["edgeBroken"])
	}
}

// TestOkfHandler_Stats_BundleNotFoundMaps404 confirms a missing bundle on
// stats surfaces as 404.
func TestOkfHandler_Stats_BundleNotFoundMaps404(t *testing.T) {
	stub := &stubOkfSvc{
		stats: func(context.Context, service.StatsRequest) (*service.StatsResponse, error) {
			return nil, service.ErrOkfBundleNotFound
		},
	}
	r := okfHandlerWithStub(t, stub)
	req, _ := http.NewRequest("GET", "/v1/disk/okf/bundles/99/stats", nil)
	w := httptestOKF(t)
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// httptestOKF allocates a fresh recorder. Wraps httptest.NewRecorder so the
// call sites stay short; the indirection also keeps the import explicit.
func httptestOKF(_ *testing.T) *httptest.ResponseRecorder {
	return httptest.NewRecorder()
}
