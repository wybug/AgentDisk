package middleware

import (
	"strings"

	"github.com/agentdisk/agent-disk/pkg/jwt"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

// AdminAuth validates admin JWT tokens for admin API routes.
func AdminAuth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			response.Unauthorized(c, "admin authentication required")
			c.Abort()
			return
		}
		tokenStr := strings.TrimPrefix(auth, "Bearer ")
		claims, err := jwt.ParseAdminToken(jwtSecret, tokenStr)
		if err != nil {
			response.Unauthorized(c, "invalid admin token")
			c.Abort()
			return
		}
		c.Set("adminUser", claims.Username)
		c.Set("adminRole", claims.Role)
		c.Set("authMethod", "admin_jwt")
		c.Next()
	}
}

// AdminOnly ensures the request has passed AdminAuth middleware.
func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString("adminUser") == "" {
			response.Forbidden(c, "admin access required")
			c.Abort()
			return
		}
		c.Next()
	}
}
