package handler

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

// okfScanHandlerService is the slice of OkfService the scan handler needs.
// Declared as an interface so handler tests can swap in a stub without
// spinning up the full service + OSS stack.
type okfScanHandlerService interface {
	ScanBundleLinksForHandler(ctx context.Context, bundleID uint64, userID, department string) (*service.ScanReport, error)
	ListBrokenLinks(ctx context.Context, bundleID uint64, userID, department string, cursor uint64, limit int) ([]service.BrokenLink, uint64, error)
	RegenerateIndex(ctx context.Context, bundleID uint64, userID, department string) (indexVersion uint32, regeneratedAt time.Time, err error)
}

// OkfScanHandler exposes the P2 maintenance endpoints: dead-link scan, broken
// link listing, and index.md regeneration. All routes are mounted under
// /v1/disk/okf alongside the rest of the OKF group and inherit HybridAuth.
type OkfScanHandler struct {
	svc okfScanHandlerService
}

// NewOkfScanHandler creates a new OkfScanHandler.
func NewOkfScanHandler(svc okfScanHandlerService) *OkfScanHandler {
	return &OkfScanHandler{svc: svc}
}

// ScanBundle handles POST /v1/disk/okf/bundles/:id/scan.
//
// Triggers a dead-link scan for the bundle. Every node is walked, its inline
// links are classified (bundle / external / anchor), and bundle-relative
// links are resolved against the materialized node index. Nodes whose links
// miss are flagged via has_broken_link so readers can filter them cheaply on
// subsequent calls.
func (h *OkfScanHandler) ScanBundle(c *gin.Context) {
	id, err := parseIDParam(c)
	if err != nil {
		return
	}
	userID, department := readUserContext(c)
	report, err := h.svc.ScanBundleLinksForHandler(c.Request.Context(), id, userID, department)
	if err != nil {
		h.respondOkfScanError(c, err)
		return
	}
	brokenCount := 0
	if report != nil {
		brokenCount = len(report.BrokenLinks)
	}
	response.OK(c, gin.H{
		"scannedNodes": scannedNodesFrom(report),
		"brokenCount":  brokenCount,
		"scannedAt":    time.Now().UTC(),
	})
}

// ListBrokenLinks handles GET /v1/disk/okf/bundles/:id/broken-links.
//
// Returns the broken links recorded for a bundle, paged via cursor. The
// cursor is the source node ID of the last item in the previous page; pass it
// as the "cursor" query param to fetch the next page. When nextCursor is 0
// in the response, the bundle has been fully enumerated.
func (h *OkfScanHandler) ListBrokenLinks(c *gin.Context) {
	id, err := parseIDParam(c)
	if err != nil {
		return
	}
	cursor, _ := strconv.ParseUint(c.Query("cursor"), 10, 64)
	limit := 50
	if l, pErr := strconv.Atoi(c.Query("limit")); pErr == nil && l > 0 && l <= 50 {
		limit = l
	}
	userID, department := readUserContext(c)
	links, nextCursor, err := h.svc.ListBrokenLinks(c.Request.Context(), id, userID, department, cursor, limit)
	if err != nil {
		h.respondOkfScanError(c, err)
		return
	}
	if links == nil {
		links = []service.BrokenLink{}
	}
	response.OK(c, gin.H{
		"brokenLinks": brokenLinksToResponse(links),
		"nextCursor":  nextCursor,
	})
}

// RegenerateIndex handles POST /v1/disk/okf/bundles/:id/regenerate-index.
//
// Renders a fresh index.md for the bundle from the current node set and
// writes it to OSS via the same UploadFileAt path as the writer. Returns the
// new content version (the file row's Version, bumped by replaceFileContent)
// and the regeneration timestamp.
func (h *OkfScanHandler) RegenerateIndex(c *gin.Context) {
	id, err := parseIDParam(c)
	if err != nil {
		return
	}
	userID, department := readUserContext(c)
	version, at, err := h.svc.RegenerateIndex(c.Request.Context(), id, userID, department)
	if err != nil {
		h.respondOkfScanError(c, err)
		return
	}
	response.OK(c, gin.H{
		"indexVersion":  version,
		"regeneratedAt": at,
	})
}

// respondOkfScanError maps service-layer OKF scan errors onto HTTP responses.
// Reuses the existing OKF error mapping for the shared sentinels (not found /
// forbidden) and adds a 409 path for lock contention (not currently reachable
// from the scan endpoints, but wired so future write-style endpoints under
// this handler can opt in).
func (h *OkfScanHandler) respondOkfScanError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrOkfBundleNotFound):
		response.NotFound(c, "bundle not found")
	case errors.Is(err, service.ErrOkfForbidden):
		response.Forbidden(c, "bundle not visible to caller")
	case errors.Is(err, service.ErrOkfLockHeld):
		// 409 Conflict: another writer holds the bundle lock. Caller should
		// back off and retry. Use Fail rather than a bare c.JSON so the body
		// stays in the project's standard envelope.
		response.Fail(c, 409, 409, "bundle lock held by another caller")
	default:
		response.InternalError(c, err.Error())
	}
}

// scannedNodesFrom safely dereferences a nil report.
func scannedNodesFrom(r *service.ScanReport) int {
	if r == nil {
		return 0
	}
	return r.ScannedNodes
}

// brokenLinksToResponse projects the service BrokenLink slice onto the wire
// shape. Each entry exposes the source node ID + relPath, the link text, the
// destination relPath, the line number in the source file, and the reason
// the link was flagged broken.
func brokenLinksToResponse(links []service.BrokenLink) []gin.H {
	out := make([]gin.H, 0, len(links))
	for i := range links {
		l := links[i]
		out = append(out, gin.H{
			"srcNodeId":  l.SrcNodeID,
			"srcRelPath": l.SrcRelPath,
			"dstRelPath": l.DstRelPath,
			"srcLine":    l.SrcLine,
			"linkText":   l.LinkText,
			"linkKind":   l.LinkKind,
			"reason":     l.Reason,
		})
	}
	return out
}
