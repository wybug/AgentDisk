package service

import (
	"fmt"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
)

type TagService struct {
	repo *repository.TagRepo
}

func NewTagService(repo *repository.TagRepo) *TagService {
	return &TagService{repo: repo}
}

func (s *TagService) BindTag(userID string, fileID uint64, tagName string) error {
	tag, err := s.repo.FindOrCreate(userID, tagName)
	if err != nil {
		return fmt.Errorf("find or create tag: %w", err)
	}
	return s.repo.BindFile(tag.ID, fileID)
}

func (s *TagService) UnbindTag(userID string, fileID uint64, tagName string) error {
	tags, err := s.repo.ListByUser(userID)
	if err != nil {
		return err
	}
	for _, t := range tags {
		if t.TagName == tagName {
			return s.repo.UnbindFile(t.ID, fileID)
		}
	}
	return fmt.Errorf("tag not found")
}

func (s *TagService) SearchByTags(userID string, tagNames []string) ([]model.DiskFile, error) {
	var tagIDs []uint64
	for _, name := range tagNames {
		tags, _ := s.repo.ListByUser(userID)
		for _, t := range tags {
			if t.TagName == name {
				tagIDs = append(tagIDs, t.ID)
			}
		}
	}
	if len(tagIDs) == 0 {
		return nil, nil
	}
	return s.repo.SearchFiles(userID, tagIDs)
}
