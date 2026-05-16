package response

import (
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

// InternalError responds with a 500 status code.
func InternalError(c *gin.Context, _ string) {
	Fail(c, http.StatusInternalServerError, 500, "internal error")
}
