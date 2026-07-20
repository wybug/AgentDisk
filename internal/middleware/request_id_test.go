package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestID_GeneratesAndEchoes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/x", func(c *gin.Context) {
		if c.GetString(RequestIDContextKey) == "" {
			t.Errorf("no request id on context")
		}
		c.Status(200)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/x", nil)
	r.ServeHTTP(w, req)

	if w.Header().Get(RequestIDHeader) == "" {
		t.Errorf("response missing %s header", RequestIDHeader)
	}
}

func TestRequestID_ReusesClientHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/x", func(c *gin.Context) {
		if c.GetString(RequestIDContextKey) != "client-trace-123" {
			t.Errorf("context id = %q, want client-trace-123", c.GetString(RequestIDContextKey))
		}
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/x", nil)
	req.Header.Set(RequestIDHeader, "client-trace-123")
	r.ServeHTTP(w, req)

	if got := w.Header().Get(RequestIDHeader); got != "client-trace-123" {
		t.Errorf("echoed id = %q, want client-trace-123", got)
	}
}
