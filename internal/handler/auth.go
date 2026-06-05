package handler

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/agentdisk/agent-disk/pkg/oauth2client"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

// Session is a core domain type.
type Session struct {
	UserID    string `json:"userId"`
	UserName  string `json:"userName,omitempty"`
	Token     string `json:"accessToken"`
	ExpiresAt int64  `json:"expiresAt"`
}

// OAuth2ClientBuilder builds an OAuth2 client from dynamic config.
type OAuth2ClientBuilder interface {
	BuildOAuth2Client() (*oauth2client.OAuthClient, error)
}

// AuthHandler is a core domain type.
type AuthHandler struct {
	oauth2Svc    OAuth2ClientBuilder
	sessions     map[string]*Session // In production, use Redis
	cookieName   string
	cookieMaxAge int
	frontendURL  string
}

// NewAuthHandler creates and returns a new AuthHandler.
func NewAuthHandler(oauth2Svc OAuth2ClientBuilder, frontendURL string) *AuthHandler {
	return &AuthHandler{
		oauth2Svc:    oauth2Svc,
		sessions:     make(map[string]*Session),
		cookieName:   "agentdisk_session",
		cookieMaxAge: 86400,
		frontendURL:  frontendURL,
	}
}

func (h *AuthHandler) buildClient() *oauth2client.OAuthClient {
	if h.oauth2Svc == nil {
		return nil
	}
	client, _ := h.oauth2Svc.BuildOAuth2Client()
	return client
}

// Status returns whether OAuth2 is currently configured and enabled.
func (h *AuthHandler) Status(c *gin.Context) {
	response.OK(c, gin.H{"oauth2": h.buildClient() != nil})
}

// Login executes the Login use case.
func (h *AuthHandler) Login(c *gin.Context) {
	client := h.buildClient()
	if client == nil {
		response.BadRequest(c, "OAuth2 not configured")
		return
	}

	state, err := oauth2client.GenerateState()
	if err != nil {
		response.InternalError(c, "failed to generate state")
		return
	}

	verifier, err := oauth2client.GenerateCodeVerifier()
	if err != nil {
		response.InternalError(c, "failed to generate PKCE verifier")
		return
	}

	promptNone := c.Query("prompt") == "none" || c.Query("from") == "gateway"

	_ = oauth2client.GenerateCodeChallenge(verifier)
	authURL := client.AuthCodeURL(state, verifier, promptNone)

	// Store state and verifier in short-lived cookie for CSRF protection
	stateData, err := json.Marshal(map[string]string{
		"state":    state,
		"verifier": verifier,
	})
	if err != nil {
		response.InternalError(c, "failed to encode state")
		return
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("oauth2_state", base64.RawURLEncoding.EncodeToString(stateData), 600, "/", "", false, true)

	c.Redirect(http.StatusFound, authURL)
}

// Callback handles the request.
func (h *AuthHandler) Callback(c *gin.Context) {
	client := h.buildClient()
	if client == nil {
		response.BadRequest(c, "OAuth2 not configured")
		return
	}

	redirectURL := "/"
	if h.frontendURL != "" {
		redirectURL = h.frontendURL
	}

	// Check for OAuth2 error response (e.g., login_required)
	if errParam := c.Query("error"); errParam != "" {
		if errParam == "login_required" {
			// Provider requires login; do not retry to avoid infinite loop
			c.Redirect(http.StatusFound, redirectURL+"?error=login_required")
			return
		}
		response.Unauthorized(c, fmt.Sprintf("OAuth2 error: %s", errParam))
		return
	}

	code := c.Query("code")
	if code == "" {
		response.BadRequest(c, "missing authorization code")
		return
	}

	stateParam := c.Query("state")
	stateCookie, cookieErr := c.Cookie("oauth2_state")
	if cookieErr != nil {
		response.BadRequest(c, "missing state cookie")
		return
	}

	stateDataBytes, err := base64.RawURLEncoding.DecodeString(stateCookie)
	if err != nil {
		response.BadRequest(c, "invalid state cookie")
		return
	}
	var stateData struct {
		State    string `json:"state"`
		Verifier string `json:"verifier"`
	}
	if jsonErr := json.Unmarshal(stateDataBytes, &stateData); jsonErr != nil {
		response.BadRequest(c, "invalid state data")
		return
	}

	if stateParam != stateData.State {
		response.Unauthorized(c, "state mismatch")
		return
	}

	// Clear state cookie
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("oauth2_state", "", -1, "/", "", false, true)

	token, err := client.Exchange(c.Request.Context(), code, stateData.Verifier)
	if err != nil {
		response.Unauthorized(c, fmt.Sprintf("token exchange failed: %v", err))
		return
	}

	userInfo, err := client.GetUserInfo(c.Request.Context(), token)
	if err != nil {
		response.Unauthorized(c, fmt.Sprintf("get userinfo failed: %v", err))
		return
	}

	sessionID := generateSessionID()
	session := &Session{
		UserID:    userInfo.UserID,
		UserName:  userInfo.UserName,
		Token:     token.AccessToken,
		ExpiresAt: time.Now().Add(time.Duration(h.cookieMaxAge) * time.Second).Unix(),
	}
	h.sessions[sessionID] = session

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(h.cookieName, sessionID, h.cookieMaxAge, "/", "", false, true)
	redirectURL = "/"
	if h.frontendURL != "" {
		redirectURL = h.frontendURL
	}
	c.Redirect(http.StatusFound, redirectURL)
}

// Logout handles the request.
func (h *AuthHandler) Logout(c *gin.Context) {
	sessionID, err := c.Cookie(h.cookieName)
	if err == nil && sessionID != "" {
		delete(h.sessions, sessionID)
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(h.cookieName, "", -1, "/", "", false, true)
	response.OK(c, gin.H{"message": "logged out"})
}

// GetSession handles the request.
func (h *AuthHandler) GetSession(sessionID string) *Session {
	sess, ok := h.sessions[sessionID]
	if !ok {
		return nil
	}
	if time.Now().Unix() > sess.ExpiresAt {
		delete(h.sessions, sessionID)
		return nil
	}
	return sess
}

func generateSessionID() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
