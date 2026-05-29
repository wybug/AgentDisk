package store

import (
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

func TestWebAuthnSessionStore_SetAndGet(t *testing.T) {
	s := NewWebAuthnSessionStore()
	session := &webauthn.SessionData{
		Challenge: "test-challenge",
		UserID:    []byte("admin"),
	}

	s.Set("key1", session, 5*time.Minute)

	got, ok := s.Get("key1")
	if !ok {
		t.Fatal("expected to find session")
	}
	if string(got.UserID) != "admin" {
		t.Errorf("expected UserID admin, got %s", string(got.UserID))
	}
}

func TestWebAuthnSessionStore_GetMissing(t *testing.T) {
	s := NewWebAuthnSessionStore()
	_, ok := s.Get("nonexistent")
	if ok {
		t.Fatal("should not find nonexistent key")
	}
}

func TestWebAuthnSessionStore_Delete(t *testing.T) {
	s := NewWebAuthnSessionStore()
	session := &webauthn.SessionData{Challenge: "challenge"}

	s.Set("key1", session, 5*time.Minute)
	s.Delete("key1")

	_, ok := s.Get("key1")
	if ok {
		t.Fatal("should not find deleted key")
	}
}

func TestWebAuthnSessionStore_TTLExpiry(t *testing.T) {
	s := NewWebAuthnSessionStore()
	session := &webauthn.SessionData{Challenge: "challenge"}

	s.Set("key1", session, 100*time.Millisecond)

	time.Sleep(150 * time.Millisecond)

	_, ok := s.Get("key1")
	if ok {
		t.Fatal("expired session should not be found")
	}
}

func TestWebAuthnSessionStore_Overwrite(t *testing.T) {
	s := NewWebAuthnSessionStore()

	s1 := &webauthn.SessionData{Challenge: "challenge-1"}
	s2 := &webauthn.SessionData{Challenge: "challenge-2"}

	s.Set("key1", s1, 5*time.Minute)
	s.Set("key1", s2, 5*time.Minute)

	got, ok := s.Get("key1")
	if !ok {
		t.Fatal("expected to find session")
	}
	if got.Challenge != "challenge-2" {
		t.Errorf("expected challenge-2, got %s", got.Challenge)
	}
}
