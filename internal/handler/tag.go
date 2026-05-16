package handler

import (
	"strings"

	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

// TagHandler is a core domain type.
type TagHandler struct {
	svc *service.TagService
}

// NewTagHandler creates and returns a new TagHandler.
func NewTagHandler(svc *service.TagService) *TagHandler {
	return &TagHandler{svc: svc}
}

// TagFileReq is a core domain type.
type TagFileReq struct {
	FileID  uint64 `json:"fileId" binding:"required"`
	TagName string `json:"tagName" binding:"required"`
}

// BindTag executes the BindTag use case.
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

// UnbindTag executes the UnbindTag use case.
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

// SearchByTags executes the SearchByTags use case.
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
