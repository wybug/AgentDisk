package middleware

import (
	"strings"

	"github.com/agentdisk/agent-disk/internal/handler"
	"github.com/agentdisk/agent-disk/pkg/download_token"
	"github.com/agentdisk/agent-disk/pkg/jwt"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

// HybridAuth provides core functionality.
func HybridAuth(
	jwtSecret string,
	authHandler *handler.AuthHandler,
	dlSecret string,
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

		response.Unauthorized(c, "authentication required")
		c.Abort()
	}
}

// IsAgentRequest returns true if the current request carries an agent identity.
func IsAgentRequest(c *gin.Context) bool {
	return c.GetString("agentId") != ""
}
