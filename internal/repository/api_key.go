package repository

import (
	"time"

	"github.com/agentdisk/agent-disk/internal/model"
	"gorm.io/gorm"
)

// APIKeyRepo provides data access for API key operations.
type APIKeyRepo struct {
	db *gorm.DB
}

// NewAPIKeyRepo creates a new APIKeyRepo.
func NewAPIKeyRepo(db *gorm.DB) *APIKeyRepo {
	return &APIKeyRepo{db: db}
}

// Create inserts a new API key.
func (r *APIKeyRepo) Create(key *model.DiskAPIKey) error {
	return r.db.Create(key).Error
}

// GetByHash returns an API key by its SHA-256 hash.
func (r *APIKeyRepo) GetByHash(hash string) (*model.DiskAPIKey, error) {
	var key model.DiskAPIKey
	if err := r.db.Where("key_hash = ?", hash).First(&key).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

// List returns all API keys ordered by creation time.
func (r *APIKeyRepo) List() ([]model.DiskAPIKey, error) {
	var keys []model.DiskAPIKey
	err := r.db.Order("id DESC").Find(&keys).Error
	return keys, err
}

// Revoke marks an API key as revoked.
func (r *APIKeyRepo) Revoke(id uint64) error {
	return r.db.Model(&model.DiskAPIKey{}).Where("id = ?", id).Update("is_revoked", true).Error
}

// UpdateLastUsed sets the last_used_at timestamp.
func (r *APIKeyRepo) UpdateLastUsed(id uint64) error {
	now := time.Now()
	return r.db.Model(&model.DiskAPIKey{}).Where("id = ?", id).Update("last_used_at", now).Error
}
