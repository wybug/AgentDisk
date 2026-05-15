package jwt

import (
	"testing"
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
