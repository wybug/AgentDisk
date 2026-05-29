package store

import (
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

// entry holds a session and its expiration time.
type entry struct {
	session *webauthn.SessionData
	expiry  time.Time
}

// WebAuthnSessionStore is a thread-safe in-memory store for WebAuthn session data.
type WebAuthnSessionStore struct {
	mu      sync.RWMutex
	entries map[string]entry
}

// NewWebAuthnSessionStore creates a new session store and starts a background cleanup goroutine.
func NewWebAuthnSessionStore() *WebAuthnSessionStore {
	s := &WebAuthnSessionStore{entries: make(map[string]entry)}
	go s.cleanup()
	return s
}

// Set stores a session with the given key and TTL.
func (s *WebAuthnSessionStore) Set(key string, session *webauthn.SessionData, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[key] = entry{session: session, expiry: time.Now().Add(ttl)}
}

// Get retrieves a session by key. Returns nil if not found or expired.
func (s *WebAuthnSessionStore) Get(key string) (*webauthn.SessionData, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.entries[key]
	if !ok || time.Now().After(e.expiry) {
		return nil, false
	}
	return e.session, true
}

// Delete removes a session by key.
func (s *WebAuthnSessionStore) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, key)
}

// cleanup removes expired entries every 60 seconds.
func (s *WebAuthnSessionStore) cleanup() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for k, e := range s.entries {
			if now.After(e.expiry) {
				delete(s.entries, k)
			}
		}
		s.mu.Unlock()
	}
}
