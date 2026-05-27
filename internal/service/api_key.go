package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
)

// apiKeyRepo defines the interface for API key data access.
type apiKeyRepo interface {
	Create(key *model.DiskAPIKey) error
	GetByHash(hash string) (*model.DiskAPIKey, error)
	List() ([]model.DiskAPIKey, error)
	Revoke(id uint64) error
	UpdateLastUsed(id uint64) error
	Update(id uint64, updates map[string]interface{}) error
}

// APIKeyService handles API key operations.
type APIKeyService struct {
	repo apiKeyRepo
}

// NewAPIKeyService creates a new APIKeyService.
func NewAPIKeyService(repo *repository.APIKeyRepo) *APIKeyService {
	return &APIKeyService{repo: repo}
}

// NewAPIKeyServiceFromRepo creates a new APIKeyService from any apiKeyRepo (for testing).
func NewAPIKeyServiceFromRepo(repo apiKeyRepo) *APIKeyService {
	return &APIKeyService{repo: repo}
}

// CreateAPIKey generates a new API key and stores its hash.
// The full key is only returned once upon creation.
func (s *APIKeyService) CreateAPIKey(name, department, createdBy string) (string, *model.DiskAPIKey, error) {
	rawKey, err := generateRawKey()
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate key: %w", err)
	}

	hash := sha256.Sum256([]byte(rawKey))
	hashStr := hex.EncodeToString(hash[:])
	prefix := rawKey[:8]

	record := &model.DiskAPIKey{
		KeyName:    name,
		KeyHash:    hashStr,
		KeyPrefix:  prefix,
		Scope:      "public_read",
		Department: department,
		CreatedBy:  createdBy,
		IsRevoked:  false,
	}

	if err := s.repo.Create(record); err != nil {
		return "", nil, fmt.Errorf("failed to store key: %w", err)
	}

	return rawKey, record, nil
}

// ValidateKey checks a raw API key and returns the record if valid.
func (s *APIKeyService) ValidateKey(rawKey string) (*model.DiskAPIKey, error) {
	if len(rawKey) < 8 {
		return nil, errors.New("invalid key format")
	}

	hash := sha256.Sum256([]byte(rawKey))
	hashStr := hex.EncodeToString(hash[:])

	record, err := s.repo.GetByHash(hashStr)
	if err != nil {
		return nil, errors.New("invalid key")
	}

	if record.IsRevoked {
		return nil, errors.New("key has been revoked")
	}

	if record.ExpiresAt != nil && record.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("key has expired")
	}

	// Best-effort update of last_used_at; ignore errors.
	_ = s.repo.UpdateLastUsed(record.ID)

	return record, nil
}

// ListAPIKeys returns all API keys.
func (s *APIKeyService) ListAPIKeys() ([]model.DiskAPIKey, error) {
	return s.repo.List()
}

// RevokeAPIKey revokes an API key by ID.
func (s *APIKeyService) RevokeAPIKey(id uint64) error {
	return s.repo.Revoke(id)
}

// RenameAPIKey updates the name of an API key.
func (s *APIKeyService) RenameAPIKey(id uint64, name string) error {
	return s.repo.Update(id, map[string]interface{}{"key_name": name})
}

// generateRawKey creates a random key with adk_ prefix (32 bytes = 64 hex chars).
func generateRawKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "adk_" + hex.EncodeToString(bytes), nil
}
