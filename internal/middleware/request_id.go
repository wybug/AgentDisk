package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"
)

// RequestIDHeader is the HTTP header carrying the correlation id both ways.
const RequestIDHeader = "X-Request-Id"

// RequestIDContextKey is the gin-context key under which the id is stored so
// log sites (e.g. response.InternalError) can pull it without re-reading the
// header. Kept as an untyped string (not a private type) so the low-level
// response package can read it without importing middleware.
const RequestIDContextKey = "requestId"

// RequestID ensures every request carries a correlation id. If the client sent
// X-Request-Id it is reused (so a trace spans hops); otherwise a fresh 32-hex
// id is generated. The id is stored on the gin context (RequestIDContextKey)
// for log sites and echoed on the response header so a client reporting an
// error can hand ops the exact id to grep.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(RequestIDHeader)
		if id == "" {
			id = newRequestID()
		}
		c.Set(RequestIDContextKey, id)
		c.Header(RequestIDHeader, id)
		c.Next()
	}
}

// newRequestID returns 32 hex chars from a cryptographically random 16 bytes.
// Failures to read /dev/urandom are effectively impossible here; in the worst
// case the id is all-zero, which is still a valid (if colliding) id.
func newRequestID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
