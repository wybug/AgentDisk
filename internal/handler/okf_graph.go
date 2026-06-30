package handler

import (
	"errors"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

// reachableRequest is the body of POST /okf/nodes/:id/reachable. Depth is the
// only required field; the rest fall back to safe defaults.
type reachableRequest struct {
	Depth     int      `json:"depth"     binding:"required,min=1"`
	Types     []string `json:"types"`
	MaxNodes  int      `json:"maxNodes"`
	Direction string   `json:"direction"`
}

// shortestPathRequest is the body of POST /okf/paths/shortest. Both src and
// dst must point to existing nodes; maxDepth is optional.
type shortestPathRequest struct {
	Src      uint64 `json:"src"      binding:"required"`
	Dst      uint64 `json:"dst"      binding:"required"`
	MaxDepth int    `json:"maxDepth"`
}

// subgraphRequest is the body of POST /okf/subgraph. bundleId picks the
// bundle; types and maxNodes narrow the slice.
type subgraphRequest struct {
	BundleID uint64   `json:"bundleId" binding:"required"`
	Types    []string `json:"types"`
	MaxNodes int      `json:"maxNodes"`
}

// Neighbors handles GET /v1/disk/okf/nodes/:id/neighbors.
//
// Returns the 1-hop neighbor set of a node. dir query param controls
// direction (out | in | both; default out). type narrows the returned
// neighbors to a single OKF type.
func (h *OkfHandler) Neighbors(c *gin.Context) {
	id, err := parseIDParam(c)
	if err != nil {
		return
	}
	userID, department := readUserContext(c)
	out, err := h.svc.Neighbors(c.Request.Context(), service.NeighborsRequest{
		NodeID:     id,
		Direction:  c.Query("dir"),
		TypeFilter: c.Query("type"),
		UserID:     userID,
		Department: department,
	})
	if err != nil {
		h.respondOkfGraphError(c, err)
		return
	}
	response.OK(c, gin.H{
		"nodes": nodeSliceToResponse(out.Nodes),
		"edges": edgeSliceToResponse(out.Edges),
	})
}

// Reachable handles POST /v1/disk/okf/nodes/:id/reachable.
//
// Runs an N-hop BFS from the start node and returns every reachable node
// (excluding the start). Depth is clamped to service.MaxBFSDepth.
func (h *OkfHandler) Reachable(c *gin.Context) {
	id, err := parseIDParam(c)
	if err != nil {
		return
	}
	var body reachableRequest
	if bindErr := c.ShouldBindJSON(&body); bindErr != nil {
		response.BadRequest(c, "invalid body: "+bindErr.Error())
		return
	}
	userID, department := readUserContext(c)
	out, err := h.svc.Reachable(c.Request.Context(), service.ReachableRequest{
		NodeID:     id,
		Depth:      body.Depth,
		Types:      body.Types,
		MaxNodes:   body.MaxNodes,
		Direction:  body.Direction,
		UserID:     userID,
		Department: department,
	})
	if err != nil {
		h.respondOkfGraphError(c, err)
		return
	}
	response.OK(c, gin.H{"nodes": nodeSliceToResponse(out.Nodes)})
}

// ShortestPath handles POST /v1/disk/okf/paths/shortest.
//
// Runs a bidirectional BFS between src and dst. Returns the path as an
// ordered node slice (src first, dst last). When no path exists within the
// depth budget the response is {path: [], found: false}.
func (h *OkfHandler) ShortestPath(c *gin.Context) {
	var body shortestPathRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "invalid body: "+err.Error())
		return
	}
	userID, department := readUserContext(c)
	out, err := h.svc.ShortestPath(c.Request.Context(), service.ShortestPathRequest{
		SrcNodeID:  body.Src,
		DstNodeID:  body.Dst,
		MaxDepth:   body.MaxDepth,
		UserID:     userID,
		Department: department,
	})
	if err != nil {
		h.respondOkfGraphError(c, err)
		return
	}
	response.OK(c, gin.H{
		"path":  nodeSliceToResponse(out.Nodes),
		"found": out.Found,
	})
}

// Subgraph handles POST /v1/disk/okf/subgraph.
//
// Returns the nodes and edges of a bundle filtered by type. Used by the
// front-end graph visualizer to render a self-contained slice.
func (h *OkfHandler) Subgraph(c *gin.Context) {
	var body subgraphRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "invalid body: "+err.Error())
		return
	}
	userID, department := readUserContext(c)
	out, err := h.svc.Subgraph(c.Request.Context(), service.SubgraphRequest{
		BundleID:   body.BundleID,
		Types:      body.Types,
		MaxNodes:   body.MaxNodes,
		UserID:     userID,
		Department: department,
	})
	if err != nil {
		h.respondOkfGraphError(c, err)
		return
	}
	response.OK(c, gin.H{
		"nodes": nodeSliceToResponse(out.Nodes),
		"edges": edgeSliceToResponse(out.Edges),
	})
}

// Stats handles GET /v1/disk/okf/bundles/:id/stats.
//
// Returns a per-bundle summary: node count, edge total/live/broken, and a
// per-type breakdown of nodes.
func (h *OkfHandler) Stats(c *gin.Context) {
	id, err := parseIDParam(c)
	if err != nil {
		return
	}
	userID, department := readUserContext(c)
	out, err := h.svc.Stats(c.Request.Context(), service.StatsRequest{
		BundleID:   id,
		UserID:     userID,
		Department: department,
	})
	if err != nil {
		h.respondOkfGraphError(c, err)
		return
	}
	typeRows := make([]gin.H, 0, len(out.TypeCounts))
	for i := range out.TypeCounts {
		typeRows = append(typeRows, gin.H{
			"type":  out.TypeCounts[i].Type,
			"count": out.TypeCounts[i].Count,
		})
	}
	response.OK(c, gin.H{
		"nodeCount":  out.NodeCount,
		"edgeTotal":  out.EdgeTotal,
		"edgeLive":   out.EdgeLive,
		"edgeBroken": out.EdgeBroken,
		"types":      typeRows,
	})
}

// RebuildGraph handles POST /v1/disk/okf/bundles/:id/rebuild-graph.
//
// Re-derives every node and edge in the bundle from the current contents of
// the public directory. This is the admin "kick everything" path: it walks
// the folder tree, re-parses every .md body, and re-materializes both
// nodes and edges. Mounted under AdminAuth + AdminOnly middleware so only
// operators can trigger it.
func (h *OkfHandler) RebuildGraph(c *gin.Context) {
	id, err := parseIDParam(c)
	if err != nil {
		return
	}
	bundle, err := h.svc.RefreshBundle(c.Request.Context(), id)
	if err != nil {
		h.respondOkfGraphError(c, err)
		return
	}
	response.OK(c, gin.H{
		"bundleId":  bundle.ID,
		"nodeCount": bundle.NodeCount,
		"edgeCount": bundle.EdgeCount,
	})
}

// respondOkfGraphError maps service-layer graph errors onto HTTP responses.
// Node-not-found is 404; invalid direction / param is 400; ACL is 403;
// everything else is 500 so internal details never leak.
//
// The handler shares this with the OKF writer/reader error mapper. Errors
// that fall through to default 500 here (e.g. ErrOkfMissingType) never
// surface from a graph query, but the branch keeps the switch exhaustive.
func (h *OkfHandler) respondOkfGraphError(c *gin.Context, err error) {
	switch {
	case isOkfError(err, service.ErrOkfInvalidDirection):
		response.BadRequest(c, err.Error())
	case isOkfError(err, service.ErrOkfNodeNotFound):
		response.NotFound(c, "node not found")
	case isOkfError(err, service.ErrOkfBundleNotFound):
		response.NotFound(c, "bundle not found")
	case isOkfError(err, service.ErrOkfForbidden):
		response.Forbidden(c, "bundle not visible to caller")
	case isOkfError(err, service.ErrOkfMissingType),
		isOkfError(err, service.ErrOkfNotBundleRoot),
		isOkfError(err, service.ErrOkfReservedName):
		response.BadRequest(c, err.Error())
	default:
		response.InternalError(c, err.Error())
	}
}

// isOkfError is a thin wrapper around errors.Is so the switch above reads
// cleanly. Errors surface from the service layer wrapped in fmt.Errorf; the
// errors.Is walk handles the unwrap chain.
func isOkfError(err, target error) bool {
	return errors.Is(err, target)
}

// nodeSliceToResponse renders a slice of OkfNode as a JSON-friendly array.
// Reuses the single-node renderer so the wire shape stays consistent with
// the rest of the OKF reader API.
func nodeSliceToResponse(nodes []model.OkfNode) []gin.H {
	out := make([]gin.H, 0, len(nodes))
	for i := range nodes {
		out = append(out, nodeToResponse(&nodes[i]))
	}
	return out
}

// edgeSliceToResponse renders a slice of OkfEdge for the graph payload. The
// shape is intentionally close to the table layout: the front-end graph
// visualizer consumes (srcNode, dstNode, linkText, broken) and renders.
func edgeSliceToResponse(edges []model.OkfEdge) []gin.H {
	out := make([]gin.H, 0, len(edges))
	for i := range edges {
		out = append(out, gin.H{
			"edgeId":     edges[i].ID,
			"srcNodeId":  edges[i].SrcNodeID,
			"dstNodeId":  edges[i].DstNodeID,
			"dstRelPath": edges[i].DstRelPath,
			"linkText":   edges[i].LinkText,
			"srcLine":    edges[i].SrcLine,
			"linkKind":   edges[i].LinkKind,
			"dstExists":  edges[i].DstExists,
		})
	}
	return out
}
