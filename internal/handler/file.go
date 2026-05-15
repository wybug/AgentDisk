package handler

import (
	"strconv"

	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

type FileHandler struct {
	svc *service.FileService
}

func NewFileHandler(svc *service.FileService) *FileHandler {
	return &FileHandler{svc: svc}
}

func (h *FileHandler) UploadFile(c *gin.Context) {
	userID := c.GetString("userId")
	folderID, _ := strconv.ParseUint(c.PostForm("folderId"), 10, 64)
	agentID := c.PostForm("agentId")

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.BadRequest(c, "file required")
		return
	}
	defer file.Close()

	result, err := h.svc.UploadFile(c.Request.Context(), userID, folderID, header.Filename, file, header.Size, header.Header.Get("Content-Type"), agentID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Created(c, result)
}

func (h *FileHandler) GetFile(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	userID := c.GetString("userId")
	file, url, err := h.svc.GetFile(c.Request.Context(), userID, id)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"file": file, "url": url})
}

func (h *FileHandler) UpdateFile(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	userID := c.GetString("userId")
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.BadRequest(c, "file required")
		return
	}
	defer file.Close()

	result, err := h.svc.UpdateFile(c.Request.Context(), userID, id, file, header.Size, header.Header.Get("Content-Type"))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, result)
}

func (h *FileHandler) DeleteFile(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	userID := c.GetString("userId")
	if err := h.svc.DeleteFile(userID, id); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, nil)
}

func (h *FileHandler) ListFiles(c *gin.Context) {
	userID := c.GetString("userId")
	folderID, err := strconv.ParseUint(c.Query("folderId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "folderId required")
		return
	}
	files, err := h.svc.ListFiles(userID, folderID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, files)
}
