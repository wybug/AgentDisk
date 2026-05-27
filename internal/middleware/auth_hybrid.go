package middleware

import (
	"strings"

	"github.com/agentdisk/agent-disk/internal/handler"
	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/pkg/download_token"
	"github.com/agentdisk/agent-disk/pkg/jwt"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

// APIKeyValidator validates raw API keys.
type APIKeyValidator interface {
	ValidateKey(rawKey string) (*model.DiskAPIKey, error)
}

// HybridAuth provides core functionality.
//
//nolint:gocognit // multi-branch auth dispatcher, splitting would reduce clarity
func HybridAuth(
	jwtSecret string,
	authHandler *handler.AuthHandler,
	dlSecret string,
	apiKeySvc APIKeyValidator,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Try JWT Bearer token (internal service calls)
		auth := c.GetHeader("Authorization")
		if auth != "" && strings.HasPrefix(auth, "Bearer ") {
			tokenStr := strings.TrimPrefix(auth, "Bearer ")
			claims, err := jwt.ParseToken(jwtSecret, tokenStr)
			if err == nil && claims != nil {
				c.Set("userId", claims.UserID)
				c.Set("agentId", claims.AgentID)
				c.Set("agentGroupId", claims.AgentGroupID)
				c.Set("department", claims.Department)
				c.Set("authMethod", "jwt")
				c.Next()
				return
			}
		}

		// 2. Try OAuth2 session cookie (web users)
		if authHandler != nil {
			sessionID, err := c.Cookie("agentdisk_session")
			if err == nil && sessionID != "" {
				session := authHandler.GetSession(sessionID)
				if session != nil {
					c.Set("userId", session.UserID)
					c.Set("userName", session.UserName)
					c.Set("authMethod", "oauth2")
					c.Next()
					return
				}
			}
		}

		// 3. Try download token (direct download links)
		if dlSecret != "" {
			dlToken := c.Query("t")
			if dlToken != "" {
				claims, err := download_token.Verify(dlSecret, dlToken)
				if err == nil && claims != nil {
					c.Set("userId", claims.UserID)
					c.Set("fileId", claims.FileID)
					c.Set("authMethod", "download_token")
					c.Next()
					return
				}
			}
		}

		// 4. Try API Key (X-API-Key header or apiKey query param)
		if apiKeySvc != nil {
			rawKey := c.GetHeader("X-API-Key")
			if rawKey == "" {
				rawKey = c.Query("apiKey")
			}
			if rawKey != "" {
				apiKey, err := apiKeySvc.ValidateKey(rawKey)
				if err == nil && apiKey != nil {
					c.Set("userId", "__system_public__")
					c.Set("authMethod", "api_key")
					c.Set("apiKeyScope", apiKey.Scope)
					c.Set("department", apiKey.Department)
					c.Next()
					return
				}
			}
		}

		response.Unauthorized(c, "authentication required")
		c.Abort()
	}
}

// IsAgentRequest returns true if the current request carries an agent identity.
func IsAgentRequest(c *gin.Context) bool {
	return c.GetString("agentId") != ""
}
