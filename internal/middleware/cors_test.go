package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCORS_Headers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, nil)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("CORS origin should be *")
	}
	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("CORS methods should be set")
	}
}

func TestCORS_Options(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS())
	r.OPTIONS("/test", func(c *gin.Context) {
		c.JSON(200, nil)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("OPTIONS", "/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != 204 {
		t.Errorf("expected 204, got %d", w.Code)
	}
}
