package handler

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

// okfShareReader is the subset of OkfShareReader the handler needs. Declared
// as an interface so handler tests can swap in a stub.
type okfShareReader interface {
	GetBundle(share *model.DiskShare, bundleID uint64) (*model.OkfBundle, error)
	ListNodes(share *model.DiskShare, bundleID uint64, typeFilter, tagFilter string) ([]model.OkfNode, error)
	Subgraph(ctx context.Context, share *model.DiskShare, req service.ShareSubgraphRequest) (*service.SubgraphResponse, error)
	Neighbors(ctx context.Context, share *model.DiskShare, req service.ShareNeighborsRequest) (*service.NeighborsResponse, error)
	GetNode(share *model.DiskShare, nodeID uint64) (*model.OkfNode, []byte, error)
}

// okfShareShareSvc is the subset of ShareService the OKF share handler needs.
// AccessShare (not GetShareByCode) is wired so the OKF share path enforces
// MaxVisit + increments VisitCount + writes the access log on every read —
// GetShareByCode alone leaves all three unchecked.
type okfShareShareSvc interface {
	AccessShare(code, extractCode, visitorIP, ua string) (*model.DiskShare, error)
}

// OkfShareHandler exposes read-only OKF bundle endpoints accessible via
// share code instead of JWT/API Key. All routes are mounted publicly (no
// HybridAuth middleware); the share code IS the credential. Each handler
// re-validates the share code + extract code (if set) so the same code that
// passed /share/access is the only one that can read bundle data.
type OkfShareHandler struct {
	reader okfShareReader
	shares okfShareShareSvc
}

// NewOkfShareHandler constructs an OkfShareHandler. reader is the read-only
// OKF accessor; shares is ShareService (used only for the code lookup).
func NewOkfShareHandler(reader okfShareReader, shares okfShareShareSvc) *OkfShareHandler {
	return &OkfShareHandler{reader: reader, shares: shares}
}

// resolveShare pulls the :code path param + ?extractCode= query param and
// invokes AccessShare, which checks active + expiry + extract code + MaxVisit
// and writes the access log atomically. Each OKF share endpoint calls this on
// every request so a one-shot share cannot be read repeatedly. Returns the
// share on success; writes a 4xx response and returns a non-nil error
// otherwise.
func (h *OkfShareHandler) resolveShare(c *gin.Context) (*model.DiskShare, error) {
	code := c.Param("code")
	if code == "" {
		response.BadRequest(c, "code is required")
		return nil, errors.New("missing code")
	}
	share, err := h.shares.AccessShare(code, c.Query("extractCode"), c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		// AccessShare returns fmt.Errorf-wrapped strings (no sentinels yet).
		// Classify by message so the OKF path returns the same status codes
		// the regular share path does.
		msg := err.Error()
		switch {
		case strings.Contains(msg, "extract code"):
			response.Forbidden(c, "invalid extract code")
		case strings.Contains(msg, "max visit"):
			response.Forbidden(c, "max visit limit reached")
		default:
			// "share not found" / "share expired or revoked" / wrapped DB error.
			response.NotFound(c, "share not found or expired")
		}
		return nil, err
	}
	if share.ResType != "bundle" {
		response.BadRequest(c, "share code is not for a bundle")
		return nil, errors.New("resType mismatch")
	}
	return share, nil
}

// GetShareBundle handles GET /v1/disk/share/:code/bundle?bundleId=...
//
// Returns bundle metadata for the share. bundleId query param must match
// the share's ResourceID.
func (h *OkfShareHandler) GetShareBundle(c *gin.Context) {
	share, err := h.resolveShare(c)
	if err != nil {
		return
	}
	bundleID, err := parseUintQuery(c, "bundleId")
	if err != nil {
		return
	}
	bundle, err := h.reader.GetBundle(share, bundleID)
	if err != nil {
		respondOkfShareError(c, err)
		return
	}
	response.OK(c, bundleToResponse(bundle))
}

// ListShareNodes handles GET /v1/disk/share/:code/nodes?bundleId=...&type=...&tag=...
func (h *OkfShareHandler) ListShareNodes(c *gin.Context) {
	share, err := h.resolveShare(c)
	if err != nil {
		return
	}
	bundleID, err := parseUintQuery(c, "bundleId")
	if err != nil {
		return
	}
	nodes, err := h.reader.ListNodes(share, bundleID, c.Query("type"), c.Query("tag"))
	if err != nil {
		respondOkfShareError(c, err)
		return
	}
	// The share recipient view does not paginate the node list (recipient
	// bundles are small); nextCursor stays 0 so the shared OkfNodeList renders
	// the full set in one page.
	response.OK(c, gin.H{"nodes": nodeSliceToResponse(nodes), "nextCursor": 0})
}

// GetShareSubgraph handles GET /v1/disk/share/:code/subgraph?bundleId=...&types=...&maxNodes=...
//
// types is a comma-separated list of OKF types; maxNodes caps the result.
func (h *OkfShareHandler) GetShareSubgraph(c *gin.Context) {
	share, err := h.resolveShare(c)
	if err != nil {
		return
	}
	bundleID, err := parseUintQuery(c, "bundleId")
	if err != nil {
		return
	}
	out, err := h.reader.Subgraph(c.Request.Context(), share, service.ShareSubgraphRequest{
		BundleID: bundleID,
		Types:    parseTypesQuery(c),
		MaxNodes: parseIntQuery(c, "maxNodes"),
	})
	if err != nil {
		respondOkfShareError(c, err)
		return
	}
	response.OK(c, gin.H{
		"nodes": nodeSliceToResponse(out.Nodes),
		"edges": edgeSliceToResponse(out.Edges),
	})
}

// GetShareNodeNeighbors handles GET /v1/disk/share/:code/nodes/:nodeId/neighbors?dir=...&type=...
func (h *OkfShareHandler) GetShareNodeNeighbors(c *gin.Context) {
	share, err := h.resolveShare(c)
	if err != nil {
		return
	}
	nodeID, err := parseIDParamByName(c, "nodeId")
	if err != nil {
		return
	}
	out, err := h.reader.Neighbors(c.Request.Context(), share, service.ShareNeighborsRequest{
		NodeID:     nodeID,
		Direction:  c.Query("dir"),
		TypeFilter: c.Query("type"),
	})
	if err != nil {
		respondOkfShareError(c, err)
		return
	}
	response.OK(c, gin.H{
		"nodes": nodeSliceToResponse(out.Nodes),
		"edges": edgeSliceToResponse(out.Edges),
	})
}

// GetShareNode handles GET /v1/disk/share/:code/nodes/:nodeId
//
// Returns the node frontmatter + raw markdown body. Used by the front-end
// to render the markdown preview inside the frontmatter drawer.
func (h *OkfShareHandler) GetShareNode(c *gin.Context) {
	share, err := h.resolveShare(c)
	if err != nil {
		return
	}
	nodeID, err := parseIDParamByName(c, "nodeId")
	if err != nil {
		return
	}
	node, body, err := h.reader.GetNode(share, nodeID)
	if err != nil {
		respondOkfShareError(c, err)
		return
	}
	resp := nodeToResponse(node)
	resp["markdown"] = string(body)
	response.OK(c, resp)
}

// parseIDParamByName extracts a named uint64 path parameter (e.g. :nodeId).
// On failure it writes a 400 response and returns a non-nil error.
func parseIDParamByName(c *gin.Context, name string) (uint64, error) {
	id, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid "+name)
		return 0, err
	}
	return id, nil
}

// parseUintQuery parses a required uint64 query param. On failure it writes
// a 400 response and returns a non-nil error so the caller can early-return.
func parseUintQuery(c *gin.Context, key string) (uint64, error) {
	v := c.Query(key)
	if v == "" {
		response.BadRequest(c, key+" is required")
		return 0, errors.New(key + " missing")
	}
	n, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid "+key)
		return 0, err
	}
	return n, nil
}

// parseIntQuery parses an optional int query param. Returns 0 when missing.
func parseIntQuery(c *gin.Context, key string) int {
	v := c.Query(key)
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return n
}

// parseTypesQuery splits a comma-separated types query param into a slice.
// Empty values are dropped. Returns nil when the param is missing.
func parseTypesQuery(c *gin.Context) []string {
	v := c.Query("types")
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for i := range parts {
		if s := strings.TrimSpace(parts[i]); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// respondOkfShareError maps reader errors onto HTTP responses. Most share
// errors (mismatched code, unknown node) surface as 4xx; unexpected errors
// are 500 with no internal details leaked.
func respondOkfShareError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrOkfShareResTypeMismatch):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrOkfShareBundleMismatch):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrOkfBundleNotFound):
		response.NotFound(c, "bundle not found")
	case errors.Is(err, service.ErrOkfNodeNotFound):
		response.NotFound(c, "node not found")
	default:
		response.InternalError(c, err.Error())
	}
}
