package handler

import (
	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

type SpaceHandler struct {
	svc *service.SpaceService
}

func NewSpaceHandler(svc *service.SpaceService) *SpaceHandler {
	return &SpaceHandler{svc: svc}
}

func (h *SpaceHandler) GetSpace(c *gin.Context) {
	userID := c.GetString("userId")
	space, err := h.svc.GetSpace(userID)
	if err != nil {
		response.NotFound(c, "space not found")
		return
	}
	response.OK(c, space)
}
