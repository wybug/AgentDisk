package handler

import (
	"strings"

	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

type TagHandler struct {
	svc *service.TagService
}

func NewTagHandler(svc *service.TagService) *TagHandler {
	return &TagHandler{svc: svc}
}

type TagFileReq struct {
	FileID  uint64 `json:"fileId" binding:"required"`
	TagName string `json:"tagName" binding:"required"`
}

func (h *TagHandler) BindTag(c *gin.Context) {
	var req TagFileReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	userID := c.GetString("userId")
	if err := h.svc.BindTag(userID, req.FileID, req.TagName); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, nil)
}

func (h *TagHandler) UnbindTag(c *gin.Context) {
	var req TagFileReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	userID := c.GetString("userId")
	if err := h.svc.UnbindTag(userID, req.FileID, req.TagName); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, nil)
}

func (h *TagHandler) SearchByTags(c *gin.Context) {
	userID := c.GetString("userId")
	tagsParam := c.Query("tags")
	if tagsParam == "" {
		response.BadRequest(c, "tags required")
		return
	}
	tagNames := strings.Split(tagsParam, ",")
	files, err := h.svc.SearchByTags(userID, tagNames)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, files)
}
