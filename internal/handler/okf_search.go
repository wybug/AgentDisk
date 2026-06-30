package handler

import (
	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

// searchRequest is the JSON body for POST /v1/disk/okf/search. BundleID,
// Type, Limit, and Cursor are optional; Query is required.
type searchRequest struct {
	Query    string `json:"query"    binding:"required"`
	BundleID uint64 `json:"bundleId"`
	Type     string `json:"type"`
	Limit    int    `json:"limit"`
	Cursor   uint64 `json:"cursor"`
}

// Search handles POST /v1/disk/okf/search.
//
// Runs a full-text query against the title + description of every node in
// the caller's visible bundles (or a single bundle when bundleId is set).
// Results are paginated via cursor — pass the returned nextCursor back as
// the next request's cursor to walk the next page; nextCursor of 0 means
// the page is the last.
//
// ACL: the service layer filters by the caller's visibility set so a
// reader cannot use search to discover nodes in a bundle they could not
// open directly.
func (h *OkfHandler) Search(c *gin.Context) {
	var req searchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid body: "+err.Error())
		return
	}
	if req.Query == "" {
		response.BadRequest(c, "query is required")
		return
	}
	userID, department := readUserContext(c)
	out, err := h.svc.Search(c.Request.Context(), service.SearchRequest{
		Query:      req.Query,
		BundleID:   req.BundleID,
		Type:       req.Type,
		Limit:      req.Limit,
		Cursor:     req.Cursor,
		UserID:     userID,
		Department: department,
	})
	if err != nil {
		h.respondOkfError(c, err)
		return
	}
	nodes := make([]gin.H, 0, len(out.Nodes))
	for i := range out.Nodes {
		nodes = append(nodes, nodeToResponse(&out.Nodes[i]))
	}
	response.OK(c, gin.H{
		"nodes":      nodes,
		"nextCursor": out.NextCursor,
	})
}
