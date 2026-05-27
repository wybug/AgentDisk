package repository

import (
	"github.com/agentdisk/agent-disk/internal/model"
	"gorm.io/gorm"
)

// OAuth2ConfigRepo provides data access for OAuth2 configuration.
type OAuth2ConfigRepo struct {
	db *gorm.DB
}

// NewOAuth2ConfigRepo creates a new OAuth2ConfigRepo.
func NewOAuth2ConfigRepo(db *gorm.DB) *OAuth2ConfigRepo {
	return &OAuth2ConfigRepo{db: db}
}

// GetActive returns the first enabled OAuth2 configuration.
func (r *OAuth2ConfigRepo) GetActive() (*model.DiskOAuth2Config, error) {
	var cfg model.DiskOAuth2Config
	if err := r.db.Where("enabled = ?", true).First(&cfg).Error; err != nil {
		return nil, err
	}
	return &cfg, nil
}

// GetByName returns an OAuth2 configuration by name.
func (r *OAuth2ConfigRepo) GetByName(name string) (*model.DiskOAuth2Config, error) {
	var cfg model.DiskOAuth2Config
	if err := r.db.Where("name = ?", name).First(&cfg).Error; err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Create inserts a new OAuth2 configuration.
func (r *OAuth2ConfigRepo) Create(cfg *model.DiskOAuth2Config) error {
	return r.db.Create(cfg).Error
}

// Update saves an OAuth2 configuration.
func (r *OAuth2ConfigRepo) Update(cfg *model.DiskOAuth2Config) error {
	return r.db.Save(cfg).Error
}

// ListAll returns all OAuth2 configurations.
func (r *OAuth2ConfigRepo) ListAll() ([]model.DiskOAuth2Config, error) {
	var configs []model.DiskOAuth2Config
	err := r.db.Order("id ASC").Find(&configs).Error
	return configs, err
}

// Delete removes an OAuth2 configuration by ID.
func (r *OAuth2ConfigRepo) Delete(id uint64) error {
	return r.db.Where("id = ?", id).Delete(&model.DiskOAuth2Config{}).Error
}
