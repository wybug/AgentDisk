package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims represents a claims.
type Claims struct {
	UserID       string `json:"userId"`
	AgentID      string `json:"agentId,omitempty"`
	AgentGroupID string `json:"agentGroupId,omitempty"`
	jwt.RegisteredClaims
}

// GenerateToken handles HTTP requests.
func GenerateToken(secret, userID, agentID string, expireHours int) (string, error) {
	return GenerateTokenWithGroup(secret, userID, agentID, "", expireHours)
}

// GenerateTokenWithGroup generates a JWT token with agent group ID.
func GenerateTokenWithGroup(secret, userID, agentID, agentGroupID string, expireHours int) (string, error) {
	claims := Claims{
		UserID:       userID,
		AgentID:      agentID,
		AgentGroupID: agentGroupID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expireHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ParseToken handles HTTP requests.
func ParseToken(secret, tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
