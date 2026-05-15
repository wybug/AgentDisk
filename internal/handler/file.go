package handler

import (
	"net/http"
	"strconv"

	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/download_token"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

type FileHandler struct {
	svc        *service.FileService
	dlSecret   string
	dlExpire   int
}

func NewFileHandler(svc *service.FileService, dlSecret string, dlExpire int) *FileHandler {
	return &FileHandler{svc: svc, dlSecret: dlSecret, dlExpire: dlExpire}
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

func (h *FileHandler) CreateDownloadToken(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	userID := c.GetString("userId")

	file, _, err := h.svc.GetFile(c.Request.Context(), userID, id)
	if err != nil {
		response.Forbidden(c, "file not found or no permission")
		return
	}

	expire := h.dlExpire
	if expire <= 0 {
		expire = 300
	}

	token, err := download_token.Generate(h.dlSecret, userID, strconv.FormatUint(file.ID, 10), expire)
	if err != nil {
		response.InternalError(c, "failed to generate download token")
		return
	}

	response.OK(c, gin.H{
		"downloadToken": token,
		"expiresIn":     expire,
	})
}

func (h *FileHandler) DownloadByToken(c *gin.Context) {
	dlToken := c.Query("t")
	if dlToken == "" {
		response.BadRequest(c, "download token required")
		return
	}

	claims, err := download_token.Verify(h.dlSecret, dlToken)
	if err != nil {
		response.Unauthorized(c, err.Error())
		return
	}

	userID := claims.UserID
	fileID, _ := strconv.ParseUint(claims.FileID, 10, 64)

	file, url, err := h.svc.GetFile(c.Request.Context(), userID, fileID)
	if err != nil {
		response.Forbidden(c, "file not found or no permission")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"file":        file,
		"downloadUrl": url,
	})
}
