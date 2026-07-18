package response

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// R represents a r.
type R struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// OK handles HTTP requests.
func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, R{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

// Created handles HTTP requests.
func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, R{
		Code:    0,
		Message: "created",
		Data:    data,
	})
}

// Fail handles HTTP requests.
func Fail(c *gin.Context, httpCode, code int, msg string) {
	c.JSON(httpCode, R{
		Code:    code,
		Message: msg,
	})
}

// BadRequest handles HTTP requests.
func BadRequest(c *gin.Context, msg string) {
	Fail(c, http.StatusBadRequest, 400, msg)
}

// Unauthorized handles HTTP requests.
func Unauthorized(c *gin.Context, msg string) {
	Fail(c, http.StatusUnauthorized, 401, msg)
}

// Forbidden handles HTTP requests.
func Forbidden(c *gin.Context, msg string) {
	Fail(c, http.StatusForbidden, 403, msg)
}

// NotFound handles HTTP requests.
func NotFound(c *gin.Context, msg string) {
	Fail(c, http.StatusNotFound, 404, msg)
}

// InternalError responds with a 500 and logs the real detail (with request
// context) for diagnosis. The response body stays the generic "internal error"
// so internals never leak to the client. Before this the detail argument was
// discarded entirely, making 500s impossible to trace — the first slice of the
// Tier-3 observability track (full slog/metrics/tracing plumbing continues).
func InternalError(c *gin.Context, detail string) {
	slog.Error("internal error",
		slog.String("method", c.Request.Method),
		slog.String("path", c.Request.URL.Path),
		slog.String("detail", detail),
	)
	Fail(c, http.StatusInternalServerError, 500, "internal error")
}
