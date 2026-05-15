package handler

import (
	"strconv"

	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

type PermissionHandler struct {
	svc *service.PermissionService
}

func NewPermissionHandler(svc *service.PermissionService) *PermissionHandler {
	return &PermissionHandler{svc: svc}
}

type GrantPermReq struct {
	AgentID    string `json:"agentId" binding:"required"`
	ResourceID uint64 `json:"resourceId" binding:"required"`
	ResType    string `json:"resType" binding:"required"`
	Permission string `json:"permission" binding:"required"`
}

func (h *PermissionHandler) GrantPermission(c *gin.Context) {
	var req GrantPermReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	userID := c.GetString("userId")
	if err := h.svc.GrantPermission(userID, req.AgentID, req.ResourceID, req.ResType, req.Permission); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Created(c, nil)
}

func (h *PermissionHandler) CheckPermission(c *gin.Context) {
	agentID := c.Query("agentId")
	resourceID, _ := strconv.ParseUint(c.Query("resourceId"), 10, 64)
	resType := c.Query("resType")
	required := c.Query("permission")

	ok, err := h.svc.CheckPermission(agentID, resourceID, resType, required)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"allowed": ok})
}

type RevokePermReq struct {
	AgentID    string `json:"agentId" binding:"required"`
	ResourceID uint64 `json:"resourceId" binding:"required"`
	ResType    string `json:"resType" binding:"required"`
}

func (h *PermissionHandler) RevokePermission(c *gin.Context) {
	var req RevokePermReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	userID := c.GetString("userId")
	if err := h.svc.RevokePermission(userID, req.AgentID, req.ResourceID, req.ResType); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, nil)
}

func (h *PermissionHandler) ListPermissions(c *gin.Context) {
	userID := c.GetString("userId")
	perms, err := h.svc.ListPermissions(userID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, perms)
}
