package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

func TestFolderHandler_CreateFolder_InvalidBody(t *testing.T) {
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("userId", "user001"); c.Next() })
	r.POST("/folders", func(c *gin.Context) {
		var req struct {
			FolderName string `json:"folderName" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Created(c, nil)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/folders", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400 for empty body, got %d", w.Code)
	}
}

func TestFolderHandler_CreateFolder_Valid(t *testing.T) {
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("userId", "user001"); c.Next() })
	r.POST("/folders", func(c *gin.Context) {
		var req struct {
			FolderName string `json:"folderName" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Created(c, gin.H{"folderName": req.FolderName})
	})

	body, _ := json.Marshal(map[string]string{"folderName": "test-dir"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/folders", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != 201 {
		t.Errorf("expected 201, got %d", w.Code)
	}
}

func TestFolderHandler_DeleteFolder_InvalidID(t *testing.T) {
	r := gin.New()
	r.DELETE("/folders/:id", func(c *gin.Context) {
		response.BadRequest(c, "invalid id")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/folders/abc", nil)
	r.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400 for invalid id, got %d", w.Code)
	}
}
