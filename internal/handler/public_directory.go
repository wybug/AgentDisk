package handler

import (
	"strconv"

	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

// PublicDirectoryHandler handles public directory endpoints.
type PublicDirectoryHandler struct {
	pdSvc *service.PublicDirectoryService
}

// NewPublicDirectoryHandler creates a new PublicDirectoryHandler.
func NewPublicDirectoryHandler(pdSvc *service.PublicDirectoryService) *PublicDirectoryHandler {
	return &PublicDirectoryHandler{pdSvc: pdSvc}
}

type createPublicDirRequest struct {
	DisplayName string `json:"displayName" binding:"required"`
	Scope       string `json:"scope" binding:"required"`
	Department  string `json:"department"`
}

// Create handles POST /v1/disk/admin/public-directories.
func (h *PublicDirectoryHandler) Create(c *gin.Context) {
	var req createPublicDirRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "displayName and scope are required")
		return
	}

	operator := c.GetString("adminUser")
	pd, err := h.pdSvc.CreatePublicDirectory(req.DisplayName, req.Scope, req.Department, operator)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Created(c, gin.H{
		"id":          pd.ID,
		"folderId":    pd.FolderID,
		"scope":       pd.Scope,
		"department":  pd.Department,
		"displayName": pd.DisplayName,
		"fixedPath":   pd.FixedPath,
		"isActive":    pd.IsActive,
		"createdBy":   pd.CreatedBy,
	})
}

// List handles GET /v1/disk/admin/public-directories.
func (h *PublicDirectoryHandler) List(c *gin.Context) {
	dirs, err := h.pdSvc.ListAdminAll()
	if err != nil {
		response.InternalError(c, "failed to list public directories")
		return
	}
	response.OK(c, dirs)
}

// Update handles PUT /v1/disk/admin/public-directories/:id.
func (h *PublicDirectoryHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	var req struct {
		DisplayName string `json:"displayName"`
		IsActive    *bool  `json:"isActive"`
	}
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	pd, err := h.pdSvc.UpdatePublicDirectory(id, req.DisplayName, isActive)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, pd)
}

// Delete handles DELETE /v1/disk/admin/public-directories/:id.
func (h *PublicDirectoryHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	if err := h.pdSvc.DeletePublicDirectory(id); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"message": "public directory deleted"})
}

// ListVisible handles GET /v1/disk/public-directories.
func (h *PublicDirectoryHandler) ListVisible(c *gin.Context) {
	department := c.GetString("department")
	dirs, err := h.pdSvc.ListVisible(department)
	if err != nil {
		response.InternalError(c, "failed to list visible directories")
		return
	}
	response.OK(c, dirs)
}

// Get handles GET /v1/disk/public-directories/:id.
func (h *PublicDirectoryHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	pd, err := h.pdSvc.GetPublicDirectory(id)
	if err != nil {
		response.NotFound(c, "public directory not found")
		return
	}
	response.OK(c, pd)
}

// ListSubFolders handles GET /v1/disk/public-directories/:id/folders.
func (h *PublicDirectoryHandler) ListSubFolders(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	folders, err := h.pdSvc.ListSubFolders(id)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, folders)
}
