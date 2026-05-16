package handler

import (
	"strconv"

	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

// FolderHandler is a core domain type.
type FolderHandler struct {
	svc *service.FolderService
}

// NewFolderHandler creates and returns a new FolderHandler.
func NewFolderHandler(svc *service.FolderService) *FolderHandler {
	return &FolderHandler{svc: svc}
}

// CreateFolderReq is a core domain type.
type CreateFolderReq struct {
	ParentID   uint64 `json:"parentId"`
	FolderName string `json:"folderName" binding:"required"`
}

// CreateFolder executes the CreateFolder use case.
func (h *FolderHandler) CreateFolder(c *gin.Context) {
	var req CreateFolderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	userID := c.GetString("userId")
	folder, err := h.svc.CreateFolder(userID, req.ParentID, req.FolderName)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Created(c, folder)
}

// GetFolder returns a single folder by ID.
func (h *FolderHandler) GetFolder(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	userID := c.GetString("userId")
	folder, err := h.svc.GetFolder(userID, id)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.OK(c, folder)
}

// GetAncestors returns the path from root to the given folder.
func (h *FolderHandler) GetAncestors(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	userID := c.GetString("userId")
	ancestors, err := h.svc.GetAncestors(userID, id)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.OK(c, ancestors)
}

// RenameFolderReq is the rename request body.
type RenameFolderReq struct {
	FolderName string `json:"folderName" binding:"required"`
}

// RenameFolder renames a folder.
func (h *FolderHandler) RenameFolder(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	var req RenameFolderReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	userID := c.GetString("userId")
	folder, err := h.svc.RenameFolder(userID, id, req.FolderName)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, folder)
}

// ListFolders executes the ListFolders use case.
func (h *FolderHandler) ListFolders(c *gin.Context) {
	userID := c.GetString("userId")
	parentID, err := strconv.ParseUint(c.DefaultQuery("parentId", "0"), 10, 64)
	if err != nil {
		parentID = 0
	}
	folders, err := h.svc.ListFolders(userID, parentID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, folders)
}

// DeleteFolder executes the DeleteFolder use case.
func (h *FolderHandler) DeleteFolder(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	userID := c.GetString("userId")
	if err := h.svc.DeleteFolder(userID, id); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, nil)
}
