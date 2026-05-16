package download_token

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Claims represents a claims.
type Claims struct {
	UserID   string `json:"uid"`
	FileID   string `json:"fid"`
	IssuedAt int64  `json:"iat"`
	Exp      int64  `json:"exp"`
	Nonce    string `json:"nonce"`
}

var (
	// ErrInvalidToken is a sentinel error.
	ErrInvalidToken = errors.New("invalid download token")
	// ErrExpiredToken is a sentinel error.
	ErrExpiredToken = errors.New("download token expired")
	// ErrInvalidFormat is a sentinel error.
	ErrInvalidFormat = errors.New("invalid token format")
	// ErrInvalidSignature is a sentinel error.
	ErrInvalidSignature = errors.New("invalid token signature")
)

// Generate handles HTTP requests.
func Generate(secret, userID, fileID string, expireSeconds int) (string, error) {
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	now := time.Now().Unix()
	claims := Claims{
		UserID:   userID,
		FileID:   fileID,
		IssuedAt: now,
		Exp:      now + int64(expireSeconds),
		Nonce:    base64.RawURLEncoding.EncodeToString(nonce),
	}

	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}

	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)
	sig := sign(secret, payloadB64)

	return payloadB64 + "." + sig, nil
}

// Verify handles HTTP requests.
func Verify(secret, token string) (*Claims, error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return nil, ErrInvalidFormat
	}

	payloadB64, sig := parts[0], parts[1]
	expectedSig := sign(secret, payloadB64)
	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return nil, ErrInvalidSignature
	}

	payload, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return nil, ErrInvalidToken
	}

	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, ErrInvalidToken
	}

	if time.Now().Unix() > claims.Exp {
		return nil, ErrExpiredToken
	}

	return &claims, nil
}

func sign(secret, payloadB64 string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payloadB64))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
