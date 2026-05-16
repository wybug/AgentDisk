package service

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
)

// ShareService represents a domain type.
type ShareService struct {
	repo *repository.ShareRepo
}

// NewShareService creates a new ShareService.
func NewShareService(repo *repository.ShareRepo) *ShareService {
	return &ShareService{repo: repo}
}

// CreateShare handles the request.
func (s *ShareService) CreateShare(userID string, resourceID uint64, resType, extractCode string, maxVisit, expireHours int) (*model.DiskShare, error) {
	code, err := generateShareCode()
	if err != nil {
		return nil, err
	}
	share := &model.DiskShare{
		UserID:      userID,
		ResourceID:  resourceID,
		ResType:     resType,
		ShareCode:   code,
		ExtractCode: extractCode,
		MaxVisit:    maxVisit,
		ExpireAt:    time.Now().Add(time.Duration(expireHours) * time.Hour),
		IsActive:    true,
	}
	if err := s.repo.Create(share); err != nil {
		return nil, fmt.Errorf("create share: %w", err)
	}
	return share, nil
}

// GetShareByCode handles the request.
func (s *ShareService) GetShareByCode(code string) (*model.DiskShare, error) {
	share, err := s.repo.GetByCode(code)
	if err != nil {
		return nil, fmt.Errorf("share not found: %w", err)
	}
	if !share.IsActive || time.Now().After(share.ExpireAt) {
		return nil, fmt.Errorf("share expired or revoked")
	}
	return share, nil
}

// AccessShare handles the request.
func (s *ShareService) AccessShare(code, extractCode, visitorIP, ua string) (*model.DiskShare, error) {
	share, err := s.GetShareByCode(code)
	if err != nil {
		return nil, err
	}
	if share.ExtractCode != "" && share.ExtractCode != extractCode {
		return nil, fmt.Errorf("invalid extract code")
	}
	if share.MaxVisit > 0 && share.VisitCount >= share.MaxVisit {
		return nil, fmt.Errorf("max visit limit reached")
	}
	share.VisitCount++
	if err := s.repo.Update(share); err != nil {
		return nil, fmt.Errorf("update share visit count: %w", err)
	}
	if err := s.repo.LogAccess(&model.ShareAccessLog{
		ShareID:   share.ID,
		VisitorIP: visitorIP,
		UserAgent: ua,
		Action:    "access",
	}); err != nil {
		return nil, fmt.Errorf("log access: %w", err)
	}
	return share, nil
}

// RevokeShare handles the request.
func (s *ShareService) RevokeShare(userID string, shareID uint64) error {
	share, err := s.repo.GetByID(shareID)
	if err != nil {
		return fmt.Errorf("share not found: %w", err)
	}
	if share.UserID != userID {
		return fmt.Errorf("permission denied")
	}
	share.IsActive = false
	return s.repo.Update(share)
}

// ListShares handles the request.
func (s *ShareService) ListShares(userID string) ([]model.DiskShare, error) {
	return s.repo.ListByUser(userID)
}

func generateShareCode() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}
