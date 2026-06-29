package handler

import (
	"context"
	"errors"
	"sort"
	"strconv"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

// okfHandlerService is the subset of OkfService the handler needs. Declared as
// an interface so handler tests can swap in a stub without spinning up the full
// service + OSS stack.
type okfHandlerService interface {
	RegisterBundle(ctx context.Context, publicDirectoryID uint64) (*model.OkfBundle, error)
	ListBundles(userID, department string) ([]model.OkfBundle, error)
	GetBundle(id uint64, userID, department string) (*model.OkfBundle, error)
	ListNodesByType(bundleID uint64, typeFilter, tagFilter, userID, department string) ([]model.OkfNode, error)
	AggregateByType(bundleID uint64, userID, department string) ([]repository.TypeCount, error)
	RefreshBundle(ctx context.Context, id uint64) (*model.OkfBundle, error)
	UnregisterBundle(id uint64) error
	WriteMarkdown(ctx context.Context, req service.WriteMarkdownRequest) (*model.OkfNode, error)
}

// OkfHandler exposes OKF v0.1 bundle reader/writer endpoints. All routes are
// mounted under /v1/disk/okf and require HybridAuth (JWT or API Key).
type OkfHandler struct {
	svc okfHandlerService
}

// NewOkfHandler creates a new OkfHandler.
func NewOkfHandler(svc *service.OkfService) *OkfHandler {
	return &OkfHandler{svc: svc}
}

// registerBundleRequest is the body of POST /bundles/register.
type registerBundleRequest struct {
	PublicDirectoryID uint64 `json:"publicDirectoryId" binding:"required"`
}

// RegisterBundle handles POST /v1/disk/okf/bundles/register.
//
// Registers a public directory as an OKF bundle. The directory must already
// contain an index.md with an okf_version field in its frontmatter.
func (h *OkfHandler) RegisterBundle(c *gin.Context) {
	var req registerBundleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "publicDirectoryId is required")
		return
	}
	bundle, err := h.svc.RegisterBundle(c.Request.Context(), req.PublicDirectoryID)
	if err != nil {
		h.respondOkfError(c, err)
		return
	}
	response.Created(c, bundleToResponse(bundle))
}

// ListBundles handles GET /v1/disk/okf/bundles.
func (h *OkfHandler) ListBundles(c *gin.Context) {
	userID, department := readUserContext(c)
	bundles, err := h.svc.ListBundles(userID, department)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	out := make([]gin.H, 0, len(bundles))
	for i := range bundles {
		out = append(out, bundleToResponse(&bundles[i]))
	}
	response.OK(c, out)
}

// GetBundle handles GET /v1/disk/okf/bundles/:id.
func (h *OkfHandler) GetBundle(c *gin.Context) {
	id, err := parseIDParam(c)
	if err != nil {
		return
	}
	userID, department := readUserContext(c)
	bundle, err := h.svc.GetBundle(id, userID, department)
	if err != nil {
		h.respondOkfError(c, err)
		return
	}
	response.OK(c, bundleToResponse(bundle))
}

// ListNodes handles GET /v1/disk/okf/bundles/:id/nodes.
//
// Query params:
//   - type: filter by OKF type (exact match)
//   - tag: filter by tag (exact match; node must include this tag)
//
// Both filters are optional. The response is wrapped as {"nodes": [...]} so the
// wire shape matches the OKF SDK docstring contract (an object envelope, not a
// bare array).
func (h *OkfHandler) ListNodes(c *gin.Context) {
	id, err := parseIDParam(c)
	if err != nil {
		return
	}
	typeFilter := c.Query("type")
	tagFilter := c.Query("tag")
	userID, department := readUserContext(c)
	nodes, err := h.svc.ListNodesByType(id, typeFilter, tagFilter, userID, department)
	if err != nil {
		h.respondOkfError(c, err)
		return
	}
	out := make([]gin.H, 0, len(nodes))
	for i := range nodes {
		out = append(out, nodeToResponse(&nodes[i]))
	}
	response.OK(c, gin.H{"nodes": out})
}

// AggregateTypes handles GET /v1/disk/okf/types.
//
// Returns a global type→count rollup across all registered bundles as an array
// of {"type": string, "count": int} objects. The array shape (rather than a
// map) matches the OKF SDK docstring contract and stays stable when callers
// iterate without depending on map ordering.
func (h *OkfHandler) AggregateTypes(c *gin.Context) {
	userID, department := readUserContext(c)
	bundles, err := h.svc.ListBundles(userID, department)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	rollup := map[string]uint32{}
	for i := range bundles {
		counts, cErr := h.svc.AggregateByType(bundles[i].ID, userID, department)
		if cErr != nil {
			response.InternalError(c, cErr.Error())
			return
		}
		for _, tc := range counts {
			rollup[tc.Type] += tc.Count
		}
	}
	// Emit a deterministic order: sorted by count desc, then type asc, so the
	// response is stable across requests and reproducible in tests.
	out := make([]gin.H, 0, len(rollup))
	for _, k := range sortedRollupKeys(rollup) {
		out = append(out, gin.H{"type": k, "count": rollup[k]})
	}
	response.OK(c, out)
}

// sortedRollupKeys returns the keys of m ordered by count desc then key asc.
func sortedRollupKeys(m map[string]uint32) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if m[keys[i]] != m[keys[j]] {
			return m[keys[i]] > m[keys[j]]
		}
		return keys[i] < keys[j]
	})
	return keys
}

// RefreshBundle handles POST /v1/disk/okf/bundles/:id/refresh.
//
// Re-scans the bundle's public directory and rebuilds the materialized node
// index. Use after bulk-importing markdown files outside the OKF writer.
func (h *OkfHandler) RefreshBundle(c *gin.Context) {
	id, err := parseIDParam(c)
	if err != nil {
		return
	}
	bundle, err := h.svc.RefreshBundle(c.Request.Context(), id)
	if err != nil {
		h.respondOkfError(c, err)
		return
	}
	response.OK(c, bundleToResponse(bundle))
}

// UnregisterBundle handles DELETE /v1/disk/okf/bundles/:id.
//
// Removes the bundle registration and its node index. The public directory and
// its files are not deleted; the directory can be re-registered later.
func (h *OkfHandler) UnregisterBundle(c *gin.Context) {
	id, err := parseIDParam(c)
	if err != nil {
		return
	}
	if err := h.svc.UnregisterBundle(id); err != nil {
		h.respondOkfError(c, err)
		return
	}
	response.OK(c, gin.H{"message": "bundle unregistered"})
}

// respondOkfError maps service-layer OKF errors onto HTTP responses. Client
// errors (bad input, not found, forbidden) get their semantic status; everything
// else is a 500 so we never leak internal details.
func (h *OkfHandler) respondOkfError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrOkfMissingType):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrOkfNotBundleRoot):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrOkfReservedName):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrOkfForbidden):
		response.Forbidden(c, "bundle not visible to caller")
	case errors.Is(err, service.ErrOkfBundleNotFound):
		response.NotFound(c, "bundle not found")
	default:
		response.InternalError(c, err.Error())
	}
}

// readUserContext pulls the caller's userId + department from the gin context.
// Both are populated by HybridAuth for JWT, OAuth2, and API Key auth, so the
// OKF reader ACL works uniformly across all three auth methods. The values may
// be empty for anonymous callers, in which case ListVisible still includes
// global-scope directories but nothing department-scoped or individually
// granted.
func readUserContext(c *gin.Context) (userID, department string) {
	return c.GetString("userId"), c.GetString("department")
}

// parseIDParam extracts and validates the :id path parameter as uint64. On
// failure it writes a 400 response and returns a non-nil error so the caller
// can early-return.
func parseIDParam(c *gin.Context) (uint64, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return 0, err
	}
	return id, nil
}

// bundleToResponse renders an OkfBundle as a JSON-friendly map. We use a map
// rather than the model struct directly so the response shape is decoupled
// from the table layout (the struct exposes GORM tags, not API tags). The
// primary key is exposed as "bundleId" to match the OKF SDK docstring contract.
func bundleToResponse(b *model.OkfBundle) gin.H {
	return gin.H{
		"bundleId":          b.ID,
		"publicDirectoryId": b.PublicDirectoryID,
		"okfVersion":        b.OkfVersion,
		"rootIndexFileId":   b.RootIndexFileID,
		"title":             b.Title,
		"description":       b.Description,
		"status":            b.Status,
		"nodeCount":         b.NodeCount,
		"edgeCount":         b.EdgeCount,
		"createdAt":         b.CreatedAt,
		"updatedAt":         b.UpdatedAt,
	}
}

// nodeToResponse renders an OkfNode as a JSON-friendly map. Tags and Extra are
// deserialized so the wire format matches the OKF frontmatter shape rather
// than the raw JSON column bytes. The primary key is exposed as "nodeId" to
// match the OKF SDK docstring contract.
func nodeToResponse(n *model.OkfNode) gin.H {
	tags, _ := n.GetTags()
	if tags == nil {
		tags = []string{}
	}
	extra, _ := n.GetExtra()
	if extra == nil {
		extra = map[string]any{}
	}
	return gin.H{
		"nodeId":        n.ID,
		"bundleId":      n.BundleID,
		"fileId":        n.FileID,
		"relPath":       n.RelPath,
		"type":          n.Type,
		"title":         n.Title,
		"description":   n.Description,
		"tags":          tags,
		"timestamp":     n.Timestamp,
		"hasBrokenLink": n.HasBrokenLink,
		"extra":         extra,
		"contentHash":   n.ContentHash,
		"createdAt":     n.CreatedAt,
		"updatedAt":     n.UpdatedAt,
	}
}
