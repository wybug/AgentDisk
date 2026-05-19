package jwt

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret-key-for-unit-test"

func TestGenerateAndParseToken(t *testing.T) {
	token, err := GenerateToken(testSecret, "user001", "agent001", 72)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	if token == "" {
		t.Fatal("token should not be empty")
	}

	claims, err := ParseToken(testSecret, token)
	if err != nil {
		t.Fatalf("ParseToken failed: %v", err)
	}
	if claims.UserID != "user001" {
		t.Errorf("expected userId user001, got %s", claims.UserID)
	}
	if claims.AgentID != "agent001" {
		t.Errorf("expected agentId agent001, got %s", claims.AgentID)
	}
}

func TestParseToken_InvalidToken(t *testing.T) {
	_, err := ParseToken(testSecret, "invalid-token")
	if err == nil {
		t.Fatal("should fail on invalid token")
	}
}

func TestParseToken_WrongSecret(t *testing.T) {
	token, _ := GenerateToken(testSecret, "user001", "agent001", 72)
	_, err := ParseToken("wrong-secret", token)
	if err == nil {
		t.Fatal("should fail with wrong secret")
	}
}

func TestParseToken_EmptyAgentID(t *testing.T) {
	token, err := GenerateToken(testSecret, "user001", "", 72)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	claims, err := ParseToken(testSecret, token)
	if err != nil {
		t.Fatalf("ParseToken failed: %v", err)
	}
	if claims.AgentID != "" {
		t.Errorf("expected empty agentId, got %s", claims.AgentID)
	}
}

func TestParseToken_UnexpectedSigningMethod(t *testing.T) {
	// Create a token with NONE signing method (not HMAC)
	claims := Claims{
		UserID:           "user_hack",
		AgentID:          "agent_hack",
		RegisteredClaims: jwt.RegisteredClaims{},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tokenStr, _ := token.SignedString(jwt.UnsafeAllowNoneSignatureType)

	_, err := ParseToken(testSecret, tokenStr)
	if err == nil {
		t.Fatal("should reject non-HMAC signing method")
	}
}

func TestParseToken_EmptyToken(t *testing.T) {
	_, err := ParseToken(testSecret, "")
	if err == nil {
		t.Fatal("should fail on empty token")
	}
}

func TestParseToken_ExpiredToken(t *testing.T) {
	token, _ := GenerateToken(testSecret, "user_expired", "", -1)
	_, err := ParseToken(testSecret, token)
	if err == nil {
		t.Fatal("should fail on expired token")
	}
}

func TestGenerateToken_DifferentUsers(t *testing.T) {
	t1, _ := GenerateToken(testSecret, "user_a", "agent_1", 1)
	t2, _ := GenerateToken(testSecret, "user_b", "agent_1", 1)

	c1, _ := ParseToken(testSecret, t1)
	c2, _ := ParseToken(testSecret, t2)

	if c1.UserID == c2.UserID {
		t.Error("different users should have different claims")
	}
}

func TestGenerateTokenWithGroup(t *testing.T) {
	token, err := GenerateTokenWithGroup(testSecret, "user001", "agent001", "group-a", 72)
	if err != nil {
		t.Fatalf("GenerateTokenWithGroup failed: %v", err)
	}

	claims, err := ParseToken(testSecret, token)
	if err != nil {
		t.Fatalf("ParseToken failed: %v", err)
	}
	if claims.UserID != "user001" {
		t.Errorf("expected userId user001, got %s", claims.UserID)
	}
	if claims.AgentID != "agent001" {
		t.Errorf("expected agentId agent001, got %s", claims.AgentID)
	}
	if claims.AgentGroupID != "group-a" {
		t.Errorf("expected agentGroupId group-a, got %s", claims.AgentGroupID)
	}
}

func TestGenerateTokenWithGroup_EmptyGroup(t *testing.T) {
	token, err := GenerateTokenWithGroup(testSecret, "user001", "agent001", "", 72)
	if err != nil {
		t.Fatalf("GenerateTokenWithGroup failed: %v", err)
	}

	claims, err := ParseToken(testSecret, token)
	if err != nil {
		t.Fatalf("ParseToken failed: %v", err)
	}
	if claims.AgentGroupID != "" {
		t.Errorf("expected empty agentGroupId, got %s", claims.AgentGroupID)
	}
}

func TestGenerateToken_BackwardCompatible(t *testing.T) {
	token, err := GenerateToken(testSecret, "user001", "agent001", 72)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	claims, err := ParseToken(testSecret, token)
	if err != nil {
		t.Fatalf("ParseToken failed: %v", err)
	}
	if claims.AgentGroupID != "" {
		t.Errorf("GenerateToken should produce empty agentGroupId, got %s", claims.AgentGroupID)
	}
}
