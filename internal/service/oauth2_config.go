package service

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
	"github.com/agentdisk/agent-disk/pkg/oauth2client"
)

// ErrOAuth2NotConfigured indicates no active OAuth2 configuration was found.
var ErrOAuth2NotConfigured = errors.New("oauth2 not configured")

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
	repo     oauth2ConfigRepo
	mu       sync.RWMutex
	cached   *oauth2client.OAuthClient
	cacheErr error
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
		cfg.Enabled = true
		cfg.Name = "default"
		createErr := s.repo.Create(cfg)
		if createErr != nil {
			return createErr
		}
		s.invalidateCache()
		return nil
	}

	existing.ClientID = cfg.ClientID
	existing.ClientSecret = cfg.ClientSecret
	existing.IssuerURL = cfg.IssuerURL
	existing.RedirectURL = cfg.RedirectURL
	existing.Scopes = cfg.Scopes
	existing.Enabled = cfg.Enabled
	existing.UpdatedBy = adminUser
	updateErr := s.repo.Update(existing)
	if updateErr != nil {
		return updateErr
	}
	s.invalidateCache()
	return nil
}

func (s *OAuth2ConfigService) invalidateCache() {
	s.mu.Lock()
	s.cached = nil
	s.cacheErr = nil
	s.mu.Unlock()
}

// ListConfigs returns all OAuth2 configurations.
func (s *OAuth2ConfigService) ListConfigs() ([]model.DiskOAuth2Config, error) {
	return s.repo.ListAll()
}

// BuildOAuth2Client creates an OAuth2 client from the active database config.
// Returns nil if no active config exists or it is disabled.
// Results are cached in-memory; cache is invalidated on UpdateConfig().
func (s *OAuth2ConfigService) BuildOAuth2Client() (*oauth2client.OAuthClient, error) {
	s.mu.RLock()
	if s.cached != nil {
		client := s.cached
		s.mu.RUnlock()
		return client, nil
	}
	if s.cacheErr != nil {
		err := s.cacheErr
		s.mu.RUnlock()
		return nil, err
	}
	s.mu.RUnlock()

	// Cache miss: acquire write lock and build
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if s.cached != nil {
		return s.cached, nil
	}

	cfg, err := s.repo.GetActive()
	if err != nil {
		s.cacheErr = ErrOAuth2NotConfigured
		return nil, ErrOAuth2NotConfigured
	}

	if !cfg.Enabled {
		s.cacheErr = ErrOAuth2NotConfigured
		return nil, ErrOAuth2NotConfigured
	}

	if cfg.IssuerURL == "" {
		return nil, fmt.Errorf("oauth2 config has empty issuer_url")
	}

	scopes := []string{}
	if cfg.Scopes != "" {
		scopes = strings.Split(cfg.Scopes, ",")
	}

	issuerURL := strings.TrimRight(cfg.IssuerURL, "/")

	client := oauth2client.New(oauth2client.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		AuthURL:      issuerURL + "/oauth2/authorize",
		TokenURL:     issuerURL + "/oauth2/token",
		UserInfoURL:  issuerURL + "/oauth2/userinfo",
		RedirectURL:  cfg.RedirectURL,
		Scopes:       scopes,
	})
	s.cached = client
	return client, nil
}
