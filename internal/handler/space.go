package handler

import (
	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

// SpaceHandler is a core domain type.
type SpaceHandler struct {
	svc *service.SpaceService
}

// NewSpaceHandler creates and returns a new SpaceHandler.
func NewSpaceHandler(svc *service.SpaceService) *SpaceHandler {
	return &SpaceHandler{svc: svc}
}

// GetSpace executes the GetSpace use case.
func (h *SpaceHandler) GetSpace(c *gin.Context) {
	userID := c.GetString("userId")
	space, err := h.svc.GetSpace(userID)
	if err != nil {
		response.NotFound(c, "space not found")
		return
	}
	response.OK(c, space)
}
