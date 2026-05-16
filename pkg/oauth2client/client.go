package oauth2client

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2"
)

// UserInfo represents a userinfo.
type UserInfo struct {
	UserID   string `json:"userId"`
	UserName string `json:"userName,omitempty"`
}

// OAuthClient represents a oauthclient.
type OAuthClient struct {
	config      *oauth2.Config
	userInfoURL string
}

// Config represents a configuration.
type Config struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
	RedirectURL  string
	Scopes       []string
}

// New creates a new  instance.
func New(cfg Config) *OAuthClient {
	scopes := cfg.Scopes
	if len(scopes) == 0 {
		scopes = []string{"openid", "profile"}
	}

	return &OAuthClient{
		config: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Endpoint: oauth2.Endpoint{
				AuthURL:  cfg.AuthURL,
				TokenURL: cfg.TokenURL,
			},
			RedirectURL: cfg.RedirectURL,
			Scopes:      scopes,
		},
		userInfoURL: cfg.UserInfoURL,
	}
}

// AuthCodeURL handles the AuthCodeURL endpoint.
func (c *OAuthClient) AuthCodeURL(state, codeVerifier string, promptNone bool) string {
	opts := []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("code_challenge", codeVerifier),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	}
	if promptNone {
		opts = append(opts, oauth2.SetAuthURLParam("prompt", "none"))
	}
	return c.config.AuthCodeURL(state, opts...)
}

// Exchange handles the Exchange endpoint.
func (c *OAuthClient) Exchange(ctx context.Context, code, codeVerifier string) (*oauth2.Token, error) {
	return c.config.Exchange(ctx, code,
		oauth2.SetAuthURLParam("code_verifier", codeVerifier),
	)
}

// GetUserInfo handles the GetUserInfo endpoint.
func (c *OAuthClient) GetUserInfo(ctx context.Context, token *oauth2.Token) (*UserInfo, error) {
	client := c.config.Client(ctx, token)
	resp, err := client.Get(c.userInfoURL)
	if err != nil {
		return nil, fmt.Errorf("get userinfo: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("userinfo returned %d: %s", resp.StatusCode, string(body))
	}

	var ui UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&ui); err != nil {
		return nil, fmt.Errorf("decode userinfo: %w", err)
	}
	return &ui, nil
}

// GenerateCodeVerifier handles HTTP requests.
func GenerateCodeVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// GenerateCodeChallenge handles HTTP requests.
func GenerateCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// GenerateState handles HTTP requests.
func GenerateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// ExtractUserIDFromToken handles HTTP requests.
func ExtractUserIDFromToken(token *oauth2.Token) string {
	if token == nil {
		return ""
	}
	val := token.Extra("userId")
	if val == nil {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	return ""
}
