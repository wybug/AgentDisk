package handler

import (
	"strconv"

	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

// APIKeyHandler handles API key management endpoints.
type APIKeyHandler struct {
	apiKeySvc *service.APIKeyService
}

// NewAPIKeyHandler creates a new APIKeyHandler.
func NewAPIKeyHandler(apiKeySvc *service.APIKeyService) *APIKeyHandler {
	return &APIKeyHandler{apiKeySvc: apiKeySvc}
}

type createAPIKeyRequest struct {
	Name       string `json:"name" binding:"required"`
	Department string `json:"department"`
}

// Create handles POST /v1/disk/admin/api-keys.
func (h *APIKeyHandler) Create(c *gin.Context) {
	var req createAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "name is required")
		return
	}

	operator := c.GetString("adminUser")
	rawKey, record, err := h.apiKeySvc.CreateAPIKey(req.Name, req.Department, operator)
	if err != nil {
		response.InternalError(c, "failed to create API key")
		return
	}

	response.Created(c, gin.H{
		"id":         record.ID,
		"key":        rawKey,
		"keyPrefix":  record.KeyPrefix,
		"keyName":    record.KeyName,
		"scope":      record.Scope,
		"department": record.Department,
		"createdAt":  record.CreatedAt,
	})
}

// List handles GET /v1/disk/admin/api-keys.
func (h *APIKeyHandler) List(c *gin.Context) {
	keys, err := h.apiKeySvc.ListAPIKeys()
	if err != nil {
		response.InternalError(c, "failed to list API keys")
		return
	}

	type keyItem struct {
		ID         uint64  `json:"id"`
		KeyName    string  `json:"keyName"`
		KeyPrefix  string  `json:"keyPrefix"`
		Scope      string  `json:"scope"`
		Department string  `json:"department"`
		IsRevoked  bool    `json:"isRevoked"`
		LastUsedAt *string `json:"lastUsedAt,omitempty"`
		ExpiresAt  *string `json:"expiresAt,omitempty"`
		CreatedBy  string  `json:"createdBy"`
		CreatedAt  string  `json:"createdAt"`
	}

	items := make([]keyItem, 0, len(keys))
	for _, k := range keys {
		item := keyItem{
			ID:         k.ID,
			KeyName:    k.KeyName,
			KeyPrefix:  k.KeyPrefix,
			Scope:      k.Scope,
			Department: k.Department,
			IsRevoked:  k.IsRevoked,
			CreatedBy:  k.CreatedBy,
			CreatedAt:  k.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if k.LastUsedAt != nil {
			s := k.LastUsedAt.Format("2006-01-02T15:04:05Z07:00")
			item.LastUsedAt = &s
		}
		if k.ExpiresAt != nil {
			s := k.ExpiresAt.Format("2006-01-02T15:04:05Z07:00")
			item.ExpiresAt = &s
		}
		items = append(items, item)
	}
	response.OK(c, items)
}

// Revoke handles DELETE /v1/disk/admin/api-keys/:id.
func (h *APIKeyHandler) Revoke(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid key id")
		return
	}

	if err := h.apiKeySvc.RevokeAPIKey(id); err != nil {
		response.InternalError(c, "failed to revoke API key")
		return
	}
	response.OK(c, gin.H{"message": "API key revoked"})
}
