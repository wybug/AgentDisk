package handler

import (
	"log"
	"net/http"
	"strconv"

	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

// PreviewHandler is a core domain type.
type PreviewHandler struct {
	svc *service.PreviewService
}

// NewPreviewHandler creates and returns a new PreviewHandler.
func NewPreviewHandler(svc *service.PreviewService) *PreviewHandler {
	return &PreviewHandler{svc: svc}
}

// PreviewFile executes the PreviewFile use case.
func (h *PreviewHandler) PreviewFile(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	userID := c.GetString("userId")
	result, err := h.svc.Preview(c.Request.Context(), userID, id)
	if err != nil {
		log.Printf("Preview error: userId=%s id=%d err=%v", userID, id, err)
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, result)
}

// PreviewHTMLFile serves raw HTML content with strict security headers for sandboxed iframe preview.
func (h *PreviewHandler) PreviewHTMLFile(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	userID := c.GetString("userId")
	content, err := h.svc.PreviewHTML(c.Request.Context(), userID, id)
	if err != nil {
		log.Printf("PreviewHTML error: userId=%s id=%d err=%v", userID, id, err)
		response.InternalError(c, err.Error())
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy",
		"default-src 'none'; "+
			"style-src 'unsafe-inline'; "+
			"img-src * data: blob:; "+
			"connect-src 'none'; "+
			"form-action 'none'; "+
			"frame-ancestors 'none'; "+
			"base-uri 'none'; "+
			"object-src 'none'")
	c.Header("X-Frame-Options", "DENY")
	c.Header("Referrer-Policy", "no-referrer")
	c.String(http.StatusOK, content)
}
