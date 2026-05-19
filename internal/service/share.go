package service

import (
	"crypto/rand"
	"fmt"
	"time"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
)

type shareRepo interface {
	Create(s *model.DiskShare) error
	GetByCode(code string) (*model.DiskShare, error)
	GetByID(id uint64) (*model.DiskShare, error)
	Update(s *model.DiskShare) error
	RevokeByID(id uint64) error
	ListByUser(userID string) ([]model.DiskShare, error)
	LogAccess(log *model.ShareAccessLog) error
}

type fileResourceRepo interface {
	GetByID(id uint64) (*model.DiskFile, error)
}

type folderResourceRepo interface {
	GetByID(id uint64) (*model.DiskFolder, error)
}

// ShareService represents a domain type.
type ShareService struct {
	repo       shareRepo
	fileRepo   fileResourceRepo
	folderRepo folderResourceRepo
}

// NewShareService creates a new ShareService.
func NewShareService(repo *repository.ShareRepo, fileRepo *repository.FileRepo, folderRepo *repository.FolderRepo) *ShareService {
	return &ShareService{repo: repo, fileRepo: fileRepo, folderRepo: folderRepo}
}

// CreateShare handles the request.
func (s *ShareService) CreateShare(userID string, resourceID uint64, resType, extractCode string, maxVisit, expireHours int) (*model.DiskShare, error) {
	// 校验资源是否存在
	switch resType {
	case "file":
		f, err := s.fileRepo.GetByID(resourceID)
		if err != nil {
			return nil, fmt.Errorf("文件不存在")
		}
		if f.UserID != userID {
			return nil, fmt.Errorf("无权分享该文件")
		}
	case "folder":
		f, err := s.folderRepo.GetByID(resourceID)
		if err != nil {
			return nil, fmt.Errorf("文件夹不存在")
		}
		if f.UserID != userID {
			return nil, fmt.Errorf("无权分享该文件夹")
		}
	default:
		return nil, fmt.Errorf("不支持的资源类型: %s", resType)
	}

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
	return s.repo.RevokeByID(shareID)
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
