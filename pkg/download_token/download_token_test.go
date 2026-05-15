package download_token

import (
	"testing"
	"time"
)

func TestGenerateAndVerify(t *testing.T) {
	secret := "test_download_secret"
	userID := "user_001"
	fileID := "12345"
	expireSeconds := 300

	token, err := Generate(secret, userID, fileID, expireSeconds)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	claims, err := Verify(secret, token)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("UserID = %q, want %q", claims.UserID, userID)
	}
	if claims.FileID != fileID {
		t.Errorf("FileID = %q, want %q", claims.FileID, fileID)
	}
	if claims.Nonce == "" {
		t.Error("Nonce should not be empty")
	}
}

func TestVerify_ExpiredToken(t *testing.T) {
	secret := "test_download_secret"
	userID := "user_001"
	fileID := "12345"

	token, err := Generate(secret, userID, fileID, -1)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	_, err = Verify(secret, token)
	if err != ErrExpiredToken {
		t.Errorf("Verify expired token error = %v, want ErrExpiredToken", err)
	}
}

func TestVerify_InvalidFormat(t *testing.T) {
	_, err := Verify("secret", "no_dot_here")
	if err != ErrInvalidFormat {
		t.Errorf("Verify no-dot token error = %v, want ErrInvalidFormat", err)
	}
}

func TestVerify_InvalidSignature(t *testing.T) {
	secret := "test_download_secret"
	token, _ := Generate(secret, "user_001", "12345", 300)

	_, err := Verify("wrong_secret", token)
	if err != ErrInvalidSignature {
		t.Errorf("Verify wrong secret error = %v, want ErrInvalidSignature", err)
	}
}

func TestVerify_TamperedPayload(t *testing.T) {
	secret := "test_download_secret"
	token, _ := Generate(secret, "user_001", "12345", 300)

	tampered := token[:len(token)-4] + "XXXX"
	_, err := Verify(secret, tampered)
	if err == nil {
		t.Error("Verify tampered token should fail")
	}
}

func TestVerify_DifferentUsers(t *testing.T) {
	secret := "test_download_secret"
	tokenA, _ := Generate(secret, "user_a", "12345", 300)
	tokenB, _ := Generate(secret, "user_b", "12345", 300)

	claimsA, _ := Verify(secret, tokenA)
	claimsB, _ := Verify(secret, tokenB)

	if claimsA.UserID == claimsB.UserID {
		t.Error("Tokens for different users should have different UserID")
	}
}

func TestGenerate_UniqueTokens(t *testing.T) {
	secret := "test_download_secret"
	token1, _ := Generate(secret, "user_001", "12345", 300)

	time.Sleep(10 * time.Millisecond)
	token2, _ := Generate(secret, "user_001", "12345", 300)

	if token1 == token2 {
		t.Error("Two tokens for same user/file should differ (nonce)")
	}
}
