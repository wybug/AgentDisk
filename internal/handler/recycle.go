package handler

import (
	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

// RecycleHandler is a core domain type.
type RecycleHandler struct {
	svc *service.RecycleService
}

// NewRecycleHandler creates a new RecycleHandler.
func NewRecycleHandler(svc *service.RecycleService) *RecycleHandler {
	return &RecycleHandler{svc: svc}
}

// ListRecycle handles the request.
func (h *RecycleHandler) ListRecycle(c *gin.Context) {
	userID := c.GetString("userId")
	items, err := h.svc.ListRecycle(userID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, items)
}

// RecycleIDReq represents a domain type.
type RecycleIDReq struct {
	RecycleID uint64 `json:"recycleId" binding:"required"`
}

// RestoreItem handles the request.
func (h *RecycleHandler) RestoreItem(c *gin.Context) {
	var req RecycleIDReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	userID := c.GetString("userId")
	if err := h.svc.Restore(userID, req.RecycleID); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, nil)
}

// DeletePermanent handles the request.
func (h *RecycleHandler) DeletePermanent(c *gin.Context) {
	var req RecycleIDReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	userID := c.GetString("userId")
	if err := h.svc.PermanentlyDelete(c.Request.Context(), userID, req.RecycleID); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, nil)
}
