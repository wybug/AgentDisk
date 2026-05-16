package service

import (
	"errors"
	"fmt"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
	"gorm.io/gorm"
)

// SpaceService represents a domain type.
type SpaceService struct {
	repo *repository.SpaceRepo
}

// NewSpaceService creates a new SpaceService.
func NewSpaceService(repo *repository.SpaceRepo) *SpaceService {
	return &SpaceService{repo: repo}
}

// GetSpace handles the request.
func (s *SpaceService) GetSpace(userID string) (*model.UserDisk, error) {
	ud, err := s.repo.GetByUserID(userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if initErr := s.InitSpace(userID); initErr != nil {
				return nil, fmt.Errorf("init space: %w", initErr)
			}
			return s.repo.GetByUserID(userID)
		}
		return nil, fmt.Errorf("get space: %w", err)
	}
	return ud, nil
}

// InitSpace handles the request.
func (s *SpaceService) InitSpace(userID string) error {
	defaultQuota := int64(10 * 1024 * 1024 * 1024) // 10GB
	return s.repo.CreateQuota(userID, defaultQuota)
}
