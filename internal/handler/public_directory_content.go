package handler

import (
	"context"
	"errors"
	"strings"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

// PublicDirectoryContentHandler handles plain-text writes to public
// directories. It is the OKF-flavored equivalent of UploadFile: instead of a
// multipart upload it accepts a JSON body with a bundle-relative path and the
// file content. Routes are mounted under pdWrite (API Key only) so agents can
// push knowledge updates with a single HTTP call.
type PublicDirectoryContentHandler struct {
	okfSvc okfContentWriter
}

// okfContentWriter is the narrow interface this handler needs from the OKF
// service. Declared separately from okfHandlerService so the content handler
// stays decoupled from the reader surface area.
type okfContentWriter interface {
	WriteMarkdown(ctx context.Context, req service.WriteMarkdownRequest) (*model.OkfNode, error)
}

// NewPublicDirectoryContentHandler creates a new PublicDirectoryContentHandler.
func NewPublicDirectoryContentHandler(okfSvc *service.OkfService) *PublicDirectoryContentHandler {
	return &PublicDirectoryContentHandler{okfSvc: okfSvc}
}

// writeContentRequest is the body of POST /public-directories/:id/files/content.
//
//   - relPath: bundle-relative path of the markdown file (e.g. "index.md" or
//     "concepts/gemma.md"); must end in .md.
//   - content: raw markdown bytes (UTF-8 text).
//   - contentType: optional; defaults to "text/markdown; charset=utf-8". Only
//     markdown is accepted in this iteration.
type writeContentRequest struct {
	RelPath     string `json:"relPath" binding:"required"`
	Content     string `json:"content" binding:"required"`
	ContentType string `json:"contentType"`
}

// WriteContent handles POST /v1/disk/public-directories/:id/files/content.
//
// Writes a markdown file into the public directory, auto-creating any nested
// folders, and updates the OKF materialized index for the bundle registered
// against the directory. If the directory has not been registered as a bundle
// and the written file is not index.md, the write still succeeds at the file
// level but returns an OKF-specific error so the caller knows materialization
// was skipped.
func (h *PublicDirectoryContentHandler) WriteContent(c *gin.Context) {
	publicDirID, err := parseIDParam(c)
	if err != nil {
		return
	}

	var req writeContentRequest
	if bErr := c.ShouldBindJSON(&req); bErr != nil {
		response.BadRequest(c, "relPath and content are required")
		return
	}

	// Reject unsupported content types up front. The default is markdown; any
	// explicit non-markdown value is reported as a client error.
	if req.ContentType != "" && !isMarkdownContentType(req.ContentType) {
		response.BadRequest(c, "only text/markdown content is supported")
		return
	}

	node, err := h.okfSvc.WriteMarkdown(c.Request.Context(), service.WriteMarkdownRequest{
		PublicDirectoryID: publicDirID,
		RelPath:           req.RelPath,
		Content:           []byte(req.Content),
		ContentType:       req.ContentType,
	})
	if err != nil {
		respondContentError(c, err)
		return
	}
	response.Created(c, nodeToResponse(node))
}

// respondContentError maps WriteMarkdown errors onto HTTP responses. Reuses
// the OKF handler's error mapping so the wire format is identical between the
// /okf/* and /public-directories/:id/files/content entry points.
func respondContentError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrOkfMissingType):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrOkfNotBundleRoot):
		response.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrOkfReservedName):
		response.BadRequest(c, err.Error())
	default:
		response.InternalError(c, err.Error())
	}
}

// isMarkdownContentType returns true when contentType is empty (default) or
// names markdown. The check is intentionally permissive: a value like
// "text/markdown; charset=utf-8" is accepted.
func isMarkdownContentType(contentType string) bool {
	if contentType == "" {
		return true
	}
	// Strip parameters at ';', trim whitespace, lowercase, then compare against
	// the two markdown media types accepted here. strings.ToLower + TrimSpace
	// keep this readable and avoid hand-rolled ASCII folding.
	for i := 0; i < len(contentType); i++ {
		if contentType[i] == ';' {
			contentType = contentType[:i]
			break
		}
	}
	ct := strings.ToLower(strings.TrimSpace(contentType))
	return ct == "text/markdown" || ct == "text/x-markdown"
}
