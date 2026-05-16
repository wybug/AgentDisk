package handler

import (
	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

// ShareHandler is a core domain type.
type ShareHandler struct {
	svc *service.ShareService
}

// NewShareHandler creates and returns a new ShareHandler.
func NewShareHandler(svc *service.ShareService) *ShareHandler {
	return &ShareHandler{svc: svc}
}

// CreateShareReq is a core domain type.
type CreateShareReq struct {
	ResourceID  uint64 `json:"resourceId" binding:"required"`
	ResType     string `json:"resType" binding:"required"`
	ExtractCode string `json:"extractCode"`
	MaxVisit    int    `json:"maxVisit"`
	ExpireHours int    `json:"expireHours"`
}

// CreateShare executes the CreateShare use case.
func (h *ShareHandler) CreateShare(c *gin.Context) {
	var req CreateShareReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	userID := c.GetString("userId")
	if req.MaxVisit == 0 {
		req.MaxVisit = -1
	}
	if req.ExpireHours == 0 {
		req.ExpireHours = 72
	}
	share, err := h.svc.CreateShare(userID, req.ResourceID, req.ResType, req.ExtractCode, req.MaxVisit, req.ExpireHours)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Created(c, share)
}

// GetShare executes the GetShare use case.
func (h *ShareHandler) GetShare(c *gin.Context) {
	code := c.Param("code")
	share, err := h.svc.GetShareByCode(code)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.OK(c, share)
}

// AccessShareReq is a core domain type.
type AccessShareReq struct {
	Code        string `json:"code" binding:"required"`
	ExtractCode string `json:"extractCode"`
}

// AccessShare handles the request.
func (h *ShareHandler) AccessShare(c *gin.Context) {
	var req AccessShareReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	share, err := h.svc.AccessShare(req.Code, req.ExtractCode, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		response.Forbidden(c, err.Error())
		return
	}
	response.OK(c, share)
}

// RevokeShareReq represents a domain type.
type RevokeShareReq struct {
	ShareID uint64 `json:"shareId" binding:"required"`
}

// RevokeShare handles the request.
func (h *ShareHandler) RevokeShare(c *gin.Context) {
	var req RevokeShareReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	userID := c.GetString("userId")
	if err := h.svc.RevokeShare(userID, req.ShareID); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, nil)
}

// ListShares handles the request.
func (h *ShareHandler) ListShares(c *gin.Context) {
	userID := c.GetString("userId")
	shares, err := h.svc.ListShares(userID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, shares)
}
