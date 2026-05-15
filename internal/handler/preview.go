package handler

import (
	"strconv"

	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

type PreviewHandler struct {
	svc *service.PreviewService
}

func NewPreviewHandler(svc *service.PreviewService) *PreviewHandler {
	return &PreviewHandler{svc: svc}
}

func (h *PreviewHandler) PreviewFile(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	userID := c.GetString("userId")
	result, err := h.svc.Preview(c.Request.Context(), userID, id)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, result)
}
