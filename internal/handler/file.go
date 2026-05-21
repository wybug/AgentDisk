package handler

import (
	"log"
	"strconv"

	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/download_token"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

// FileHandler represents a domain type.
type FileHandler struct {
	svc        *service.FileService
	permSvc    *service.PermissionService
	recycleSvc *service.RecycleService
	dlSecret   string
	dlExpire   int
}

// NewFileHandler creates a new FileHandler.
func NewFileHandler(svc *service.FileService, permSvc *service.PermissionService, recycleSvc *service.RecycleService, dlSecret string, dlExpire int) *FileHandler {
	return &FileHandler{svc: svc, permSvc: permSvc, recycleSvc: recycleSvc, dlSecret: dlSecret, dlExpire: dlExpire}
}

// UploadFile handles the request.
func (h *FileHandler) UploadFile(c *gin.Context) {
	userID := c.GetString("userId")
	log.Printf("UploadFile: userId=%s", userID)
	folderID, err := strconv.ParseUint(c.PostForm("folderId"), 10, 64)
	if err != nil {
		folderID = 0
	}
	// Prefer agentId from JWT context, fallback to form field
	agentID := c.GetString("agentId")
	if agentID == "" {
		agentID = c.PostForm("agentId")
	}
	agentGroupID := c.GetString("agentGroupId")

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		log.Printf("UploadFile: FormFile error: %v", err)
		response.BadRequest(c, "file required")
		return
	}
	defer func() { _ = file.Close() }()

	result, err := h.svc.UploadFileWithGroup(c.Request.Context(), userID, folderID, header.Filename, file, header.Size, header.Header.Get("Content-Type"), agentID, agentGroupID)
	if err != nil {
		log.Printf("UploadFile: service error: %v", err)
		response.InternalError(c, err.Error())
		return
	}
	response.Created(c, result)
}

// GetFile handles the request.
func (h *FileHandler) GetFile(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	userID := c.GetString("userId")
	agentID := c.GetString("agentId")
	agentGroupID := c.GetString("agentGroupId")

	if agentID != "" {
		permOK, permErr := h.permSvc.CheckOrAutoGrant(userID, agentID, agentGroupID, id, "file", "read")
		if permErr != nil || !permOK {
			response.Forbidden(c, "no permission")
			return
		}
	}

	file, url, err := h.svc.GetFile(c.Request.Context(), userID, id)
	if err != nil {
		log.Printf("GetFile error: userId=%s id=%d err=%v", userID, id, err)
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"file": file, "url": url})
}

// UpdateFile handles the request.
func (h *FileHandler) UpdateFile(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	userID := c.GetString("userId")
	agentID := c.GetString("agentId")
	agentGroupID := c.GetString("agentGroupId")

	if agentID != "" {
		permOK, permErr := h.permSvc.CheckOrAutoGrant(userID, agentID, agentGroupID, id, "file", "write")
		if permErr != nil || !permOK {
			response.Forbidden(c, "no permission")
			return
		}
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.BadRequest(c, "file required")
		return
	}
	defer func() { _ = file.Close() }()

	result, err := h.svc.UpdateFile(c.Request.Context(), userID, id, file, header.Size, header.Header.Get("Content-Type"))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, result)
}

// DeleteFile handles the request.
func (h *FileHandler) DeleteFile(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	userID := c.GetString("userId")
	agentID := c.GetString("agentId")
	agentGroupID := c.GetString("agentGroupId")

	if agentID != "" {
		ok, err := h.permSvc.CheckOrAutoGrant(userID, agentID, agentGroupID, id, "file", "delete")
		if err != nil || !ok {
			response.Forbidden(c, "no permission")
			return
		}
	}

	if err := h.svc.DeleteFile(userID, id); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	deletedBy := "user"
	if agentID != "" {
		deletedBy = agentID
	}
	_ = h.recycleSvc.MoveToRecycle(userID, id, "file", deletedBy)
	response.OK(c, nil)
}

// ListFiles handles the request.
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

// CreateDownloadToken handles the request.
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

// DownloadByToken handles the request.
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
	fileID, err := strconv.ParseUint(claims.FileID, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid file ID in token")
		return
	}

	file, url, err := h.svc.GetFile(c.Request.Context(), userID, fileID)
	if err != nil {
		response.Forbidden(c, "file not found or no permission")
		return
	}

	response.OK(c, gin.H{
		"file":        file,
		"downloadUrl": url,
	})
}
