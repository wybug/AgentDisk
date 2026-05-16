package service

import (
	"fmt"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
)

// TagService represents a domain type.
type TagService struct {
	repo *repository.TagRepo
}

// NewTagService creates a new TagService.
func NewTagService(repo *repository.TagRepo) *TagService {
	return &TagService{repo: repo}
}

// BindTag handles the request.
func (s *TagService) BindTag(userID string, fileID uint64, tagName string) error {
	tag, err := s.repo.FindOrCreate(userID, tagName)
	if err != nil {
		return fmt.Errorf("find or create tag: %w", err)
	}
	return s.repo.BindFile(tag.ID, fileID)
}

// UnbindTag handles the request.
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

// SearchByTags handles the request.
func (s *TagService) SearchByTags(userID string, tagNames []string) ([]model.DiskFile, error) {
	var tagIDs []uint64
	for _, name := range tagNames {
		tags, err := s.repo.ListByUser(userID)
		if err != nil {
			return nil, fmt.Errorf("list tags: %w", err)
		}
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
