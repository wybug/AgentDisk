package handler

import (
	"strconv"

	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

type VersionHandler struct {
	svc *service.VersionService
}

func NewVersionHandler(svc *service.VersionService) *VersionHandler {
	return &VersionHandler{svc: svc}
}

func (h *VersionHandler) ListVersions(c *gin.Context) {
	userID := c.GetString("userId")
	fileID, err := strconv.ParseUint(c.Query("fileId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "fileId required")
		return
	}
	versions, err := h.svc.ListVersions(userID, fileID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, versions)
}

type RollbackReq struct {
	FileID  uint64 `json:"fileId" binding:"required"`
	Version int    `json:"version" binding:"required"`
}

func (h *VersionHandler) RollbackVersion(c *gin.Context) {
	var req RollbackReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	userID := c.GetString("userId")
	if err := h.svc.Rollback(c.Request.Context(), userID, req.FileID, req.Version); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, nil)
}
