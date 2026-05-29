package handler

import (
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/jwt"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

// AdminMFAHandler handles MFA/WebAuthn endpoints.
type AdminMFAHandler struct {
	mfaSvc    *service.AdminMFAService
	jwtSecret string
}

// NewAdminMFAHandler creates a new AdminMFAHandler.
func NewAdminMFAHandler(mfaSvc *service.AdminMFAService, jwtSecret string) *AdminMFAHandler {
	return &AdminMFAHandler{mfaSvc: mfaSvc, jwtSecret: jwtSecret}
}

// BeginRegistration handles POST /v1/disk/admin/mfa/registration/begin.
func (h *AdminMFAHandler) BeginRegistration(c *gin.Context) {
	username := c.GetString("adminUser")
	options, sessionKey, err := h.mfaSvc.BeginRegistration(username)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c, gin.H{
		"options":    options,
		"sessionKey": sessionKey,
	})
}

type finishRegistrationRequest struct {
	SessionKey string `json:"sessionKey" binding:"required"`
	Name       string `json:"name"`
	Credential string `json:"credential" binding:"required"`
}

// FinishRegistration handles POST /v1/disk/admin/mfa/registration/finish.
func (h *AdminMFAHandler) FinishRegistration(c *gin.Context) {
	username := c.GetString("adminUser")
	var req finishRegistrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "sessionKey and credential required")
		return
	}

	passkey, err := h.mfaSvc.FinishRegistration(username, req.SessionKey, req.Name, newWebAuthnRequest(req.Credential))
	if err != nil {
		log.Printf("MFA FinishRegistration failed for %s: %v", username, err)
		response.BadRequest(c, err.Error())
		return
	}

	response.Created(c, gin.H{
		"id":        passkey.ID,
		"name":      passkey.Name,
		"createdAt": passkey.CreatedAt,
	})
}

// newWebAuthnRequest creates an *http.Request with the credential JSON body for go-webauthn parsing.
func newWebAuthnRequest(credentialJSON string) *http.Request {
	req, _ := http.NewRequest("POST", "/", io.NopCloser(strings.NewReader(credentialJSON)))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// ListPasskeys handles GET /v1/disk/admin/mfa/credentials.
func (h *AdminMFAHandler) ListPasskeys(c *gin.Context) {
	username := c.GetString("adminUser")
	passkeys, err := h.mfaSvc.ListPasskeys(username)
	if err != nil {
		response.InternalError(c, "failed to list passkeys")
		return
	}

	type passkeyItem struct {
		ID        uint64  `json:"id"`
		Name      string  `json:"name"`
		CreatedAt string  `json:"createdAt"`
		LastUsed  *string `json:"lastUsedAt"`
	}

	items := make([]passkeyItem, 0, len(passkeys))
	for _, pk := range passkeys {
		var lastUsed *string
		if pk.LastUsedAt != nil {
			s := pk.LastUsedAt.Format("2006-01-02T15:04:05Z07:00")
			lastUsed = &s
		}
		items = append(items, passkeyItem{
			ID:        pk.ID,
			Name:      pk.Name,
			CreatedAt: pk.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			LastUsed:  lastUsed,
		})
	}
	response.OK(c, items)
}

// DeletePasskey handles DELETE /v1/disk/admin/mfa/credentials/:id.
func (h *AdminMFAHandler) DeletePasskey(c *gin.Context) {
	username := c.GetString("adminUser")
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid passkey id")
		return
	}

	if err := h.mfaSvc.DeletePasskey(username, id); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"message": "passkey deleted"})
}

type renamePasskeyRequest struct {
	Name string `json:"name" binding:"required"`
}

// RenamePasskey handles PUT /v1/disk/admin/mfa/credentials/:id.
func (h *AdminMFAHandler) RenamePasskey(c *gin.Context) {
	username := c.GetString("adminUser")
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid passkey id")
		return
	}

	var req renamePasskeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "name required")
		return
	}

	if err := h.mfaSvc.RenamePasskey(username, id, req.Name); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"message": "passkey renamed"})
}

// GetMFAStatus handles GET /v1/disk/admin/mfa/status.
func (h *AdminMFAHandler) GetMFAStatus(c *gin.Context) {
	username := c.GetString("adminUser")
	count, enabled, err := h.mfaSvc.GetMFAStatus(username)
	if err != nil {
		response.InternalError(c, "failed to get MFA status")
		return
	}
	response.OK(c, gin.H{
		"passkeyCount": count,
		"mfaEnabled":   enabled,
	})
}

type setMFAEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

// SetMFAEnabled handles PUT /v1/disk/admin/mfa/enabled.
func (h *AdminMFAHandler) SetMFAEnabled(c *gin.Context) {
	username := c.GetString("adminUser")
	var req setMFAEnabledRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "enabled field required")
		return
	}

	if err := h.mfaSvc.SetMFAEnabled(username, req.Enabled); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"message": "mfa updated"})
}

type beginMFALoginRequest struct {
	SessionToken string `json:"sessionToken" binding:"required"`
}

// BeginMFALogin handles POST /v1/disk/admin/mfa/login/begin.
func (h *AdminMFAHandler) BeginMFALogin(c *gin.Context) {
	var req beginMFALoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "sessionToken required")
		return
	}

	claims, err := jwt.ParseMFASessionToken(h.jwtSecret, req.SessionToken)
	if err != nil {
		response.Unauthorized(c, "invalid or expired session token")
		return
	}

	options, sessionKey, err := h.mfaSvc.BeginLogin(claims.Username)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{
		"options":    options,
		"sessionKey": sessionKey,
	})
}

type finishMFALoginRequest struct {
	SessionKey string `json:"sessionKey" binding:"required"`
	Credential string `json:"credential" binding:"required"`
}

// FinishMFALogin handles POST /v1/disk/admin/mfa/login/finish.
func (h *AdminMFAHandler) FinishMFALogin(c *gin.Context) {
	var req finishMFALoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "sessionKey and credential required")
		return
	}

	token, username, err := h.mfaSvc.FinishLogin(req.SessionKey, newWebAuthnRequest(req.Credential))
	if err != nil {
		log.Printf("MFA FinishLogin failed: %v", err)
		response.Unauthorized(c, err.Error())
		return
	}
	response.OK(c, gin.H{
		"token":    token,
		"username": username,
	})
}
