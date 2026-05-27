package service

import (
	"fmt"
	"strings"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
	"github.com/agentdisk/agent-disk/pkg/oauth2client"
)

// oauth2ConfigRepo defines the interface for OAuth2 config data access.
type oauth2ConfigRepo interface {
	GetActive() (*model.DiskOAuth2Config, error)
	GetByName(name string) (*model.DiskOAuth2Config, error)
	Create(cfg *model.DiskOAuth2Config) error
	Update(cfg *model.DiskOAuth2Config) error
	ListAll() ([]model.DiskOAuth2Config, error)
	Delete(id uint64) error
}

// OAuth2ConfigService handles OAuth2 configuration management.
type OAuth2ConfigService struct {
	repo oauth2ConfigRepo
}

// NewOAuth2ConfigService creates a new OAuth2ConfigService.
func NewOAuth2ConfigService(repo *repository.OAuth2ConfigRepo) *OAuth2ConfigService {
	return &OAuth2ConfigService{repo: repo}
}

// GetActiveConfig returns the currently active OAuth2 configuration.
func (s *OAuth2ConfigService) GetActiveConfig() (*model.DiskOAuth2Config, error) {
	return s.repo.GetActive()
}

// GetConfig returns config by name or active config.
func (s *OAuth2ConfigService) GetConfig() (*model.DiskOAuth2Config, error) {
	return s.repo.GetActive()
}

// UpdateConfig updates or creates the OAuth2 configuration.
func (s *OAuth2ConfigService) UpdateConfig(adminUser string, cfg *model.DiskOAuth2Config) error {
	cfg.UpdatedBy = adminUser

	existing, err := s.repo.GetActive()
	if err != nil {
		// No active config, create one
		cfg.Enabled = true
		cfg.Name = "default"
		return s.repo.Create(cfg)
	}

	existing.ClientID = cfg.ClientID
	existing.ClientSecret = cfg.ClientSecret
	existing.AuthURL = cfg.AuthURL
	existing.TokenURL = cfg.TokenURL
	existing.UserInfoURL = cfg.UserInfoURL
	existing.RedirectURL = cfg.RedirectURL
	existing.FrontendURL = cfg.FrontendURL
	existing.Scopes = cfg.Scopes
	existing.Enabled = cfg.Enabled
	existing.UpdatedBy = adminUser
	return s.repo.Update(existing)
}

// ListConfigs returns all OAuth2 configurations.
func (s *OAuth2ConfigService) ListConfigs() ([]model.DiskOAuth2Config, error) {
	return s.repo.ListAll()
}

// BuildOAuth2Client creates an OAuth2 client from the active database config.
func (s *OAuth2ConfigService) BuildOAuth2Client() (*oauth2client.OAuthClient, error) {
	cfg, err := s.repo.GetActive()
	if err != nil {
		return nil, fmt.Errorf("no active OAuth2 config: %w", err)
	}

	scopes := []string{}
	if cfg.Scopes != "" {
		scopes = strings.Split(cfg.Scopes, ",")
	}

	return oauth2client.New(oauth2client.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		AuthURL:      cfg.AuthURL,
		TokenURL:     cfg.TokenURL,
		UserInfoURL:  cfg.UserInfoURL,
		RedirectURL:  cfg.RedirectURL,
		Scopes:       scopes,
	}), nil
}
