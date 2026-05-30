package storage

import (
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// LocalStorageHandler serves files from local storage with HMAC verification.
type LocalStorageHandler struct {
	storage *LocalStorage
}

// NewLocalStorageHandler creates a new LocalStorageHandler.
func NewLocalStorageHandler(s *LocalStorage) *LocalStorageHandler {
	return &LocalStorageHandler{storage: s}
}

// ServeFile handles GET /v1/disk/local-storage/*key
func (h *LocalStorageHandler) ServeFile(c *gin.Context) {
	key := c.Param("key")
	if key != "" && key[0] == '/' {
		key = key[1:]
	}

	expStr := c.Query("exp")
	sig := c.Query("sig")

	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "invalid expiration")
		return
	}

	if time.Now().Unix() > exp {
		c.String(http.StatusForbidden, "url expired")
		return
	}

	if !h.storage.VerifySignature(key, exp, sig) {
		c.String(http.StatusForbidden, "invalid signature")
		return
	}

	fullPath := filepath.Join(h.storage.rootDir, key)

	if !strings.HasPrefix(filepath.Clean(fullPath), filepath.Clean(h.storage.rootDir)+string(filepath.Separator)) {
		c.String(http.StatusForbidden, "access denied")
		return
	}

	c.File(fullPath)
}
