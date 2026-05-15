package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentdisk/agent-disk/pkg/jwt"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

const testJWTSecret = "test-jwt-secret-for-handler"

func init() {
	gin.SetMode(gin.TestMode)
}

func authToken(userID string) string {
	t, _ := jwt.GenerateToken(testJWTSecret, userID, "agent001", 72)
	return t
}

func TestSpaceHandler_GetSpace_NotFound(t *testing.T) {
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("userId", "user001"); c.Next() })
	r.GET("/space", func(c *gin.Context) {
		response.NotFound(c, "space not found")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/space", nil)
	r.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestSpaceHandler_GetSpace_OK(t *testing.T) {
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("userId", "user001"); c.Next() })
	r.GET("/space", func(c *gin.Context) {
		response.OK(c, gin.H{"userId": "user001", "totalQuota": 10737418240})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/space", nil)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	if data["userId"] != "user001" {
		t.Error("userId mismatch")
	}
}
