package service

import "testing"

func TestGenerateShareCode(t *testing.T) {
	code1, err := generateShareCode()
	if err != nil {
		t.Fatalf("generateShareCode failed: %v", err)
	}
	if len(code1) != 32 {
		t.Errorf("expected 32 char code, got %d", len(code1))
	}

	code2, _ := generateShareCode()
	if code1 == code2 {
		t.Error("two generated codes should be different")
	}
}
