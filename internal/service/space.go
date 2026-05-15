package service

import (
	"fmt"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
)

type SpaceService struct {
	repo *repository.SpaceRepo
}

func NewSpaceService(repo *repository.SpaceRepo) *SpaceService {
	return &SpaceService{repo: repo}
}

func (s *SpaceService) GetSpace(userID string) (*model.UserDisk, error) {
	ud, err := s.repo.GetByUserID(userID)
	if err != nil {
		return nil, fmt.Errorf("get space: %w", err)
	}
	return ud, nil
}

func (s *SpaceService) InitSpace(userID string) error {
	defaultQuota := int64(10 * 1024 * 1024 * 1024) // 10GB
	return s.repo.CreateQuota(userID, defaultQuota)
}
