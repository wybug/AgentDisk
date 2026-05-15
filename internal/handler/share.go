package handler

import (
	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

type ShareHandler struct {
	svc *service.ShareService
}

func NewShareHandler(svc *service.ShareService) *ShareHandler {
	return &ShareHandler{svc: svc}
}

type CreateShareReq struct {
	ResourceID  uint64 `json:"resourceId" binding:"required"`
	ResType     string `json:"resType" binding:"required"`
	ExtractCode string `json:"extractCode"`
	MaxVisit    int    `json:"maxVisit"`
	ExpireHours int    `json:"expireHours"`
}

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

func (h *ShareHandler) GetShare(c *gin.Context) {
	code := c.Param("code")
	share, err := h.svc.GetShareByCode(code)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	response.OK(c, share)
}

type AccessShareReq struct {
	Code        string `json:"code" binding:"required"`
	ExtractCode string `json:"extractCode"`
}

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

type RevokeShareReq struct {
	ShareID uint64 `json:"shareId" binding:"required"`
}

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

func (h *ShareHandler) ListShares(c *gin.Context) {
	userID := c.GetString("userId")
	shares, err := h.svc.ListShares(userID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, shares)
}
