package service

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/internal/repository"
)

// Sentinel errors for the share service. Handlers map these to HTTP statuses
// (404 / 403) via errors.Is so clients get graded responses (CLAUDE.md §4.4).
var (
	ErrShareNotFound         = errors.New("share not found")
	ErrSharePermissionDenied = errors.New("permission denied")
)

type shareRepo interface {
	Create(s *model.DiskShare) error
	GetByCode(code string) (*model.DiskShare, error)
	GetByID(id uint64) (*model.DiskShare, error)
	IncrementVisitCount(id uint64) error
	RevokeByID(id uint64) error
	ListByUser(userID string) ([]model.DiskShare, error)
	LogAccess(log *model.ShareAccessLog) error
	ListAccessLogsByShare(shareID uint64, limit int) ([]model.ShareAccessLog, error)
	ShareAccessStats(shareID uint64) (repository.ShareAccessStatsRow, error)
}

type fileResourceRepo interface {
	GetByID(id uint64) (*model.DiskFile, error)
}

type folderResourceRepo interface {
	GetByID(id uint64) (*model.DiskFolder, error)
}

// publicDirGrantChecker checks if a user can access a public directory resource.
type publicDirGrantChecker interface {
	IsUserGrantedForFile(fileID uint64, userID string) (bool, error)
	// IsPublicDirVisibleToUser returns true when the user can see the public
	// directory through any visibility path (global/department scope or
	// explicit grant). Mirrors the OKF reader's visibility check so the
	// share-create gate matches what users see in the UI.
	IsPublicDirVisibleToUser(publicDirID uint64, userID string) (bool, error)
}

// ShareService represents a domain type.
type ShareService struct {
	repo         shareRepo
	fileRepo     fileResourceRepo
	folderRepo   folderResourceRepo
	grantChecker publicDirGrantChecker
	bundleRepo   okfBundleRepo
}

// NewShareService creates a new ShareService.
func NewShareService(repo *repository.ShareRepo, fileRepo *repository.FileRepo, folderRepo *repository.FolderRepo) *ShareService {
	return &ShareService{repo: repo, fileRepo: fileRepo, folderRepo: folderRepo}
}

// SetGrantChecker injects a public directory grant checker.
func (s *ShareService) SetGrantChecker(checker publicDirGrantChecker) {
	s.grantChecker = checker
}

// SetBundleRepo injects the OKF bundle repo. Required before bundle shares
// can be created; if unset, "bundle" resType is rejected as unsupported.
func (s *ShareService) SetBundleRepo(repo okfBundleRepo) {
	s.bundleRepo = repo
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
		if f.UserID == SystemUserID {
			if s.grantChecker == nil {
				return nil, fmt.Errorf("无权分享该文件")
			}
			granted, _ := s.grantChecker.IsUserGrantedForFile(resourceID, userID)
			if !granted {
				return nil, fmt.Errorf("无权分享该文件")
			}
		} else if f.UserID != userID {
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
	case "bundle":
		if s.bundleRepo == nil {
			return nil, fmt.Errorf("不支持的资源类型: %s", resType)
		}
		b, err := s.bundleRepo.GetByID(resourceID)
		if err != nil {
			return nil, fmt.Errorf("bundle 不存在")
		}
		if s.grantChecker == nil {
			return nil, fmt.Errorf("无权分享该 Bundle")
		}
		visible, err := s.grantChecker.IsPublicDirVisibleToUser(b.PublicDirectoryID, userID)
		if err != nil || !visible {
			return nil, fmt.Errorf("无权分享该 Bundle")
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
	if err := s.repo.IncrementVisitCount(share.ID); err != nil {
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

// shareStatsRecentLimit caps how many recent access-log rows ShareStats
// returns in one response. The full log is not paginated today; this window
// covers the "who accessed my share lately" use case. A follow-up can add a
// cursor-based /shares/:id/access-logs endpoint for full history.
const shareStatsRecentLimit = 50

// shareAccessLogRow is a single masked access-log entry in ShareStatsResponse.
type shareAccessLogRow struct {
	CreatedAt time.Time `json:"createdAt"`
	VisitorIP string    `json:"visitorIP"`
	UserAgent string    `json:"userAgent"`
	Action    string    `json:"action"`
}

// ShareStatsResponse summarizes a share's access activity for its owner.
// LastAccessAt is nil (→ JSON null) when the share has never been accessed.
type ShareStatsResponse struct {
	VisitCount   int                 `json:"visitCount"`
	UniqueIPs    uint64              `json:"uniqueIPs"`
	LastAccessAt *time.Time          `json:"lastAccessAt"`
	RecentLogs   []shareAccessLogRow `json:"recentLogs"`
}

// ShareStats returns aggregated access activity for a share, owner-only. The
// distinct-IP count and last-access time come from a SQL aggregate (accurate
// across the full log); the recent-logs window is capped at
// shareStatsRecentLimit. Visitor IPs are masked before return (CLAUDE.md §4.6
// 脱敏) so owners see access trends without the service exposing raw IPs.
func (s *ShareService) ShareStats(userID string, shareID uint64) (*ShareStatsResponse, error) {
	share, err := s.repo.GetByID(shareID)
	if err != nil {
		return nil, ErrShareNotFound
	}
	if share.UserID != userID {
		return nil, ErrSharePermissionDenied
	}
	stats, err := s.repo.ShareAccessStats(shareID)
	if err != nil {
		return nil, fmt.Errorf("share access stats: %w", err)
	}
	logs, err := s.repo.ListAccessLogsByShare(shareID, shareStatsRecentLimit)
	if err != nil {
		return nil, fmt.Errorf("list access logs: %w", err)
	}
	rows := make([]shareAccessLogRow, 0, len(logs))
	for i := range logs {
		rows = append(rows, shareAccessLogRow{
			CreatedAt: logs[i].CreatedAt,
			VisitorIP: maskVisitorIP(logs[i].VisitorIP),
			UserAgent: logs[i].UserAgent,
			Action:    logs[i].Action,
		})
	}
	// logs come back newest-first, so the first row is the most-recent access.
	var lastAccess *time.Time
	if len(logs) > 0 {
		t := logs[0].CreatedAt
		lastAccess = &t
	}
	return &ShareStatsResponse{
		VisitCount:   share.VisitCount,
		UniqueIPs:    stats.UniqueIPs,
		LastAccessAt: lastAccess,
		RecentLogs:   rows,
	}, nil
}

// maskVisitorIP zeroes the last IPv4 octet (192.0.2.42 → 192.0.2.0) or the
// last IPv6 group (2001:db8::1 → 2001:db8::0). Empty or non-IP strings are
// returned as-is (they are already non-identifying).
func maskVisitorIP(ip string) string {
	if ip == "" {
		return ""
	}
	if strings.Contains(ip, ":") {
		if i := strings.LastIndex(ip, ":"); i >= 0 {
			return ip[:i+1] + "0"
		}
	}
	if strings.Contains(ip, ".") {
		if i := strings.LastIndex(ip, "."); i >= 0 {
			return ip[:i+1] + "0"
		}
	}
	return ip
}

func generateShareCode() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}
