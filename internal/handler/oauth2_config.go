package handler

import (
	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

// OAuth2ConfigHandler handles OAuth2 configuration management endpoints.
type OAuth2ConfigHandler struct {
	oauth2Svc *service.OAuth2ConfigService
}

// NewOAuth2ConfigHandler creates a new OAuth2ConfigHandler.
func NewOAuth2ConfigHandler(oauth2Svc *service.OAuth2ConfigService) *OAuth2ConfigHandler {
	return &OAuth2ConfigHandler{oauth2Svc: oauth2Svc}
}

// Get handles GET /v1/disk/admin/oauth2.
func (h *OAuth2ConfigHandler) Get(c *gin.Context) {
	configs, err := h.oauth2Svc.ListConfigs()
	if err != nil || len(configs) == 0 {
		response.OK(c, nil)
		return
	}

	cfg := configs[0]
	response.OK(c, gin.H{
		"id":          cfg.ID,
		"name":        cfg.Name,
		"enabled":     cfg.Enabled,
		"clientId":    cfg.ClientID,
		"authUrl":     cfg.AuthURL,
		"tokenUrl":    cfg.TokenURL,
		"userInfoUrl": cfg.UserInfoURL,
		"redirectUrl": cfg.RedirectURL,
		"frontendUrl": cfg.FrontendURL,
		"scopes":      cfg.Scopes,
		"updatedBy":   cfg.UpdatedBy,
		"createdAt":   cfg.CreatedAt,
		"updatedAt":   cfg.UpdatedAt,
	})
}

type updateOAuth2Request struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	AuthURL      string `json:"authUrl"`
	TokenURL     string `json:"tokenUrl"`
	UserInfoURL  string `json:"userInfoUrl"`
	RedirectURL  string `json:"redirectUrl"`
	FrontendURL  string `json:"frontendUrl"`
	Scopes       string `json:"scopes"`
	Enabled      bool   `json:"enabled"`
}

// Update handles PUT /v1/disk/admin/oauth2.
func (h *OAuth2ConfigHandler) Update(c *gin.Context) {
	var req updateOAuth2Request
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	adminUser := c.GetString("adminUser")
	cfg := &model.DiskOAuth2Config{
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
		AuthURL:      req.AuthURL,
		TokenURL:     req.TokenURL,
		UserInfoURL:  req.UserInfoURL,
		RedirectURL:  req.RedirectURL,
		FrontendURL:  req.FrontendURL,
		Scopes:       req.Scopes,
		Enabled:      req.Enabled,
	}

	if err := h.oauth2Svc.UpdateConfig(adminUser, cfg); err != nil {
		response.InternalError(c, "failed to update OAuth2 config")
		return
	}
	response.OK(c, gin.H{"message": "OAuth2 config updated"})
}

// Test handles POST /v1/disk/admin/oauth2/test.
func (h *OAuth2ConfigHandler) Test(c *gin.Context) {
	_, err := h.oauth2Svc.BuildOAuth2Client()
	if err != nil {
		response.OK(c, gin.H{"status": "error", "message": err.Error()})
		return
	}
	response.OK(c, gin.H{"status": "ok", "message": "OAuth2 client can be built from config"})
}
