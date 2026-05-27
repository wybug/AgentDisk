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
	Department   string `json:"department,omitempty"`
	jwt.RegisteredClaims
}

// AdminClaims represents admin JWT claims.
type AdminClaims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	IsAdmin  bool   `json:"isAdmin"`
	jwt.RegisteredClaims
}

// GenerateToken handles HTTP requests.
func GenerateToken(secret, userID, agentID string, expireHours int) (string, error) {
	return GenerateTokenWithGroup(secret, userID, agentID, "", expireHours)
}

// GenerateTokenWithGroup generates a JWT token with agent group ID.
func GenerateTokenWithGroup(secret, userID, agentID, agentGroupID string, expireHours int) (string, error) {
	return GenerateTokenWithDepartment(secret, userID, agentID, agentGroupID, "", expireHours)
}

// GenerateTokenWithDepartment generates a JWT token with department claim.
func GenerateTokenWithDepartment(secret, userID, agentID, agentGroupID, department string, expireHours int) (string, error) {
	claims := Claims{
		UserID:       userID,
		AgentID:      agentID,
		AgentGroupID: agentGroupID,
		Department:   department,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expireHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// GenerateAdminToken generates a JWT token for admin authentication.
func GenerateAdminToken(secret, username, role string, expireHours int) (string, error) {
	claims := AdminClaims{
		Username: username,
		Role:     role,
		IsAdmin:  true,
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
	if claims.UserID == "" {
		return nil, errors.New("not a user token")
	}
	return claims, nil
}

// ParseAdminToken parses and validates an admin JWT token.
func ParseAdminToken(secret, tokenStr string) (*AdminClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &AdminClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*AdminClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid admin token")
	}
	if !claims.IsAdmin || claims.Username == "" {
		return nil, errors.New("not an admin token")
	}
	return claims, nil
}
