package handler

import (
	"strconv"

	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

type FolderHandler struct {
	svc *service.FolderService
}

func NewFolderHandler(svc *service.FolderService) *FolderHandler {
	return &FolderHandler{svc: svc}
}

type CreateFolderReq struct {
	ParentID   uint64 `json:"parentId"`
	FolderName string `json:"folderName" binding:"required"`
}

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

func (h *FolderHandler) ListFolders(c *gin.Context) {
	userID := c.GetString("userId")
	parentID, _ := strconv.ParseUint(c.DefaultQuery("parentId", "0"), 10, 64)
	folders, err := h.svc.ListFolders(userID, parentID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, folders)
}

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
