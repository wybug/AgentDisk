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

type Session struct {
	UserID    string `json:"userId"`
	UserName  string `json:"userName,omitempty"`
	Token     string `json:"accessToken"`
	ExpiresAt int64  `json:"expiresAt"`
}

type AuthHandler struct {
	oauthClient *oauth2client.OAuthClient
	sessions    map[string]*Session // In production, use Redis
	cookieName  string
	cookieMaxAge int
}

func NewAuthHandler(oauthClient *oauth2client.OAuthClient) *AuthHandler {
	return &AuthHandler{
		oauthClient:  oauthClient,
		sessions:     make(map[string]*Session),
		cookieName:   "agentdisk_session",
		cookieMaxAge: 86400,
	}
}

func (h *AuthHandler) Login(c *gin.Context) {
	if h.oauthClient == nil {
		response.InternalError(c, "OAuth2 not configured")
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

	challenge := oauth2client.GenerateCodeChallenge(verifier)
	_ = challenge

	authURL := h.oauthClient.AuthCodeURL(state, verifier, promptNone)

	// Store state and verifier in short-lived cookie for CSRF protection
	stateData, _ := json.Marshal(map[string]string{
		"state":    state,
		"verifier": verifier,
	})
	c.SetCookie("oauth2_state", base64.RawURLEncoding.EncodeToString(stateData), 600, "/", "", false, true)

	c.Redirect(http.StatusFound, authURL)
}

func (h *AuthHandler) Callback(c *gin.Context) {
	if h.oauthClient == nil {
		response.InternalError(c, "OAuth2 not configured")
		return
	}

	// Check for OAuth2 error response (e.g., login_required)
	if errParam := c.Query("error"); errParam != "" {
		if errParam == "login_required" {
			// SSO failed, redirect to standard login
			h.Login(c)
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
	stateCookie, err := c.Cookie("oauth2_state")
	if err != nil {
		response.BadRequest(c, "missing state cookie")
		return
	}

	stateDataBytes, _ := base64.RawURLEncoding.DecodeString(stateCookie)
	var stateData struct {
		State    string `json:"state"`
		Verifier string `json:"verifier"`
	}
	if err := json.Unmarshal(stateDataBytes, &stateData); err != nil {
		response.BadRequest(c, "invalid state data")
		return
	}

	if stateParam != stateData.State {
		response.Unauthorized(c, "state mismatch")
		return
	}

	// Clear state cookie
	c.SetCookie("oauth2_state", "", -1, "/", "", false, true)

	token, err := h.oauthClient.Exchange(c.Request.Context(), code, stateData.Verifier)
	if err != nil {
		response.Unauthorized(c, fmt.Sprintf("token exchange failed: %v", err))
		return
	}

	userInfo, err := h.oauthClient.GetUserInfo(c.Request.Context(), token)
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

	c.SetCookie(h.cookieName, sessionID, h.cookieMaxAge, "/", "", false, true)
	c.Redirect(http.StatusFound, "/")
}

func (h *AuthHandler) Logout(c *gin.Context) {
	sessionID, err := c.Cookie(h.cookieName)
	if err == nil && sessionID != "" {
		delete(h.sessions, sessionID)
	}
	c.SetCookie(h.cookieName, "", -1, "/", "", false, true)
	response.OK(c, gin.H{"message": "logged out"})
}

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
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
