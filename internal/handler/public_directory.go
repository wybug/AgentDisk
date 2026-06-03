package handler

import (
	"net/http"
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
	userID := c.GetString("userId")
	dirs, err := h.pdSvc.ListVisible(department, userID)
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

// --- File management ---

// ListFiles handles GET /v1/disk/public-directories/:id/files.
func (h *PublicDirectoryHandler) ListFiles(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	files, err := h.pdSvc.ListFiles(id)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, files)
}

// UploadFile handles POST /v1/disk/public-directories/:id/files/upload.
func (h *PublicDirectoryHandler) UploadFile(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.BadRequest(c, "file required")
		return
	}
	defer func() { _ = file.Close() }()

	result, err := h.pdSvc.UploadFile(c.Request.Context(), id, header.Filename, file, header.Size, header.Header.Get("Content-Type"))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Created(c, result)
}

// DeleteFile handles DELETE /v1/disk/public-directories/:id/files/:fileId.
func (h *PublicDirectoryHandler) DeleteFile(c *gin.Context) {
	publicDirID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid public directory id")
		return
	}
	fileID, err := strconv.ParseUint(c.Param("fileId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid file id")
		return
	}

	if err := h.pdSvc.DeleteFile(c.Request.Context(), publicDirID, fileID); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, nil)
}

// CreateDownloadToken handles POST /v1/disk/public-directories/:id/download-token.
func (h *PublicDirectoryHandler) CreateDownloadToken(c *gin.Context) {
	publicDirID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid public directory id")
		return
	}
	fileIDStr := c.PostForm("fileId")
	if fileIDStr == "" {
		fileIDStr = c.Query("fileId")
	}
	fileID, err := strconv.ParseUint(fileIDStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "fileId required")
		return
	}

	token, expire, err := h.pdSvc.CreateDownloadToken(publicDirID, fileID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	if acceptsJSON(c) {
		dlURL, urlErr := h.pdSvc.GetFileDownloadURL(c.Request.Context(), publicDirID, fileID)
		if urlErr != nil {
			response.InternalError(c, urlErr.Error())
			return
		}
		response.OK(c, gin.H{
			"downloadToken": token,
			"expiresIn":     expire,
			"downloadUrl":   dlURL,
		})
		return
	}

	response.OK(c, gin.H{
		"downloadToken": token,
		"expiresIn":     expire,
	})
}

// CreateSubFolder handles POST /v1/disk/public-directories/:id/folders.
func (h *PublicDirectoryHandler) CreateSubFolder(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	folderName := c.PostForm("folderName")
	if folderName == "" {
		var req struct {
			FolderName string `json:"folderName"`
		}
		if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
			response.BadRequest(c, "folderName required")
			return
		}
		folderName = req.FolderName
	}

	folder, err := h.pdSvc.CreateSubFolder(c.Request.Context(), id, folderName)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Created(c, folder)
}

// --- User authorization ---

// GrantAccess handles POST /v1/disk/public-directories/:id/grants.
func (h *PublicDirectoryHandler) GrantAccess(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	var req struct {
		UserID string `json:"userId" binding:"required"`
	}
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		response.BadRequest(c, "userId required")
		return
	}

	grantedBy := c.GetString("userId")
	if err := h.pdSvc.GrantUserAccess(id, req.UserID, grantedBy); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"message": "access granted"})
}

// RevokeAccess handles DELETE /v1/disk/public-directories/:id/grants/:userId.
func (h *PublicDirectoryHandler) RevokeAccess(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	userID := c.Param("userId")
	if userID == "" {
		response.BadRequest(c, "userId required")
		return
	}

	if err := h.pdSvc.RevokeUserAccess(id, userID); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{"message": "access revoked"})
}

// ListGrantedUsers handles GET /v1/disk/public-directories/:id/grants.
func (h *PublicDirectoryHandler) ListGrantedUsers(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	userIDs, err := h.pdSvc.ListGrantedUsers(id)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, userIDs)
}

// redirectDownload redirects to presigned URL.
func redirectDownload(c *gin.Context, url string) {
	c.Redirect(http.StatusFound, url)
}
