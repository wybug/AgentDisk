package middleware

import (
	"github.com/agentdisk/agent-disk/internal/feature"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

// RequireFeature returns a gin middleware that blocks requests when the named
// feature flag is off. The hot path is a single atomic.Pointer load, so the
// middleware is safe to put on every request without lock contention.
//
// Behavior:
//   - flag ON  → request proceeds
//   - flag OFF → 403 "feature disabled" (response.Forbidden)
//
// The flag check runs per-request, so admin-initiated toggles take effect on
// the very next request — no restart needed.
func RequireFeature(reg *feature.Registry, name feature.FlagName) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !reg.IsEnabled(name) {
			response.Forbidden(c, "feature disabled")
			c.Abort()
			return
		}
		c.Next()
	}
}
