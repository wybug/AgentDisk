package service

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/pkg/oss"
)

// PreviewService represents a domain type.
type PreviewService struct {
	fileSvc   *FileService
	ossClient *oss.Client
}

// NewPreviewService creates a new PreviewService.
func NewPreviewService(fileSvc *FileService, ossClient *oss.Client) *PreviewService {
	return &PreviewService{fileSvc: fileSvc, ossClient: ossClient}
}

// PreviewResult represents a domain type.
type PreviewResult struct {
	FileType string `json:"fileType"`
	URL      string `json:"url"`
}

// Preview handles the request.
func (s *PreviewService) Preview(ctx context.Context, userID string, fileID uint64) (*PreviewResult, error) {
	file, url, err := s.fileSvc.GetFile(ctx, userID, fileID)
	if err != nil {
		return nil, err
	}
	return &PreviewResult{
		FileType: classify(file),
		URL:      url,
	}, nil
}

const maxHTMLSize = 5 * 1024 * 1024 // 5MB

// PreviewHTML returns raw HTML content for secure sandboxed preview.
func (s *PreviewService) PreviewHTML(ctx context.Context, userID string, fileID uint64) (string, error) {
	file, err := s.fileSvc.GetFileRecord(userID, fileID)
	if err != nil {
		return "", err
	}

	obj, err := s.ossClient.Download(ctx, file.OSSKey)
	if err != nil {
		return "", fmt.Errorf("download from oss: %w", err)
	}
	defer func() { _ = obj.Close() }()

	data, err := io.ReadAll(io.LimitReader(obj, maxHTMLSize))
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	return string(data), nil
}

func classify(f *model.DiskFile) string {
	ext := strings.ToLower(f.FileType)
	switch {
	case ext == "md", ext == "markdown":
		return "markdown"
	case isHTML(ext):
		return "html"
	case isCode(ext):
		return "code"
	case isImage(ext):
		return "image"
	case isText(ext):
		return "text"
	default:
		return "binary"
	}
}

func isCode(ext string) bool {
	codeExts := map[string]bool{
		"go": true, "py": true, "js": true, "ts": true, "java": true,
		"c": true, "cpp": true, "h": true, "rs": true, "rb": true,
		"php": true, "sh": true, "sql": true, "css": true,
		"json": true, "yaml": true, "yml": true, "xml": true, "toml": true,
	}
	return codeExts[ext]
}

func isHTML(ext string) bool {
	return ext == "html" || ext == "htm"
}

func isImage(ext string) bool {
	imgExts := map[string]bool{"png": true, "jpg": true, "jpeg": true, "gif": true, "svg": true, "webp": true}
	return imgExts[ext]
}

func isText(ext string) bool {
	textExts := map[string]bool{"txt": true, "log": true, "csv": true, "md": true}
	return textExts[ext]
}
