package handler

import (
	"strconv"
	"strings"

	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

// PermissionHandler is a core domain type.
type PermissionHandler struct {
	svc *service.PermissionService
}

// NewPermissionHandler creates and returns a new PermissionHandler.
func NewPermissionHandler(svc *service.PermissionService) *PermissionHandler {
	return &PermissionHandler{svc: svc}
}

// GrantPermReq is a core domain type.
type GrantPermReq struct {
	AgentID      string `json:"agentId"`
	AgentGroupID string `json:"agentGroupId"`
	ResourceID   uint64 `json:"resourceId"`
	ResType      string `json:"resType"`
	ResourcePath string `json:"resourcePath"`
	Permission   string `json:"permission" binding:"required"`
}

// GrantPermission executes the GrantPermission use case.
func (h *PermissionHandler) GrantPermission(c *gin.Context) {
	var req GrantPermReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if req.AgentID == "" && req.AgentGroupID == "" {
		response.BadRequest(c, "agentId or agentGroupId is required")
		return
	}
	if req.ResourceID == 0 && req.ResourcePath == "" {
		response.BadRequest(c, "resourceId or resourcePath is required")
		return
	}
	if req.ResourceID != 0 && req.ResType == "" {
		response.BadRequest(c, "resType is required when resourceId is provided")
		return
	}
	if req.ResourcePath != "" && !strings.HasPrefix(req.ResourcePath, "/") {
		response.BadRequest(c, "resourcePath must start with /")
		return
	}
	userID := c.GetString("userId")
	if err := h.svc.GrantPermission(userID, req.AgentID, req.AgentGroupID, req.ResourceID, req.ResType, req.ResourcePath, req.Permission); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Created(c, nil)
}

// CheckPermission executes the CheckPermission use case.
func (h *PermissionHandler) CheckPermission(c *gin.Context) {
	agentID := c.Query("agentId")
	agentGroupID := c.Query("agentGroupId")
	resourceID, err := strconv.ParseUint(c.Query("resourceId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid resourceId")
		return
	}
	resType := c.Query("resType")
	required := c.Query("permission")

	var ok bool
	if agentID != "" {
		ok, _ = h.svc.CheckPermission(agentID, resourceID, resType, required)
	}
	if !ok && agentGroupID != "" {
		groupOk, _ := h.svc.CheckGroupPermission(agentGroupID, resourceID, resType, required)
		ok = groupOk
	}

	response.OK(c, gin.H{"allowed": ok})
}

// RevokePermReq is a core domain type.
type RevokePermReq struct {
	AgentID      string `json:"agentId"`
	AgentGroupID string `json:"agentGroupId"`
	ResourceID   uint64 `json:"resourceId"`
	ResType      string `json:"resType"`
	ResourcePath string `json:"resourcePath"`
}

// RevokePermission executes the RevokePermission use case.
func (h *PermissionHandler) RevokePermission(c *gin.Context) {
	var req RevokePermReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if req.AgentID == "" && req.AgentGroupID == "" {
		response.BadRequest(c, "agentId or agentGroupId is required")
		return
	}
	if req.ResourceID == 0 && req.ResourcePath == "" {
		response.BadRequest(c, "resourceId or resourcePath is required")
		return
	}
	userID := c.GetString("userId")
	if err := h.svc.RevokePermission(userID, req.AgentID, req.AgentGroupID, req.ResourceID, req.ResType, req.ResourcePath); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, nil)
}

// ListPermissions executes the ListPermissions use case.
func (h *PermissionHandler) ListPermissions(c *gin.Context) {
	userID := c.GetString("userId")
	perms, err := h.svc.ListPermissions(userID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, perms)
}
