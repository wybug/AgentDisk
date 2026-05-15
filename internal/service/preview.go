package service

import (
	"context"
	"strings"

	"github.com/agentdisk/agent-disk/internal/model"
	"github.com/agentdisk/agent-disk/pkg/oss"
)

type PreviewService struct {
	fileSvc   *FileService
	ossClient *oss.Client
}

func NewPreviewService(fileSvc *FileService, ossClient *oss.Client) *PreviewService {
	return &PreviewService{fileSvc: fileSvc, ossClient: ossClient}
}

type PreviewResult struct {
	FileType string `json:"fileType"`
	URL      string `json:"url"`
}

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

func classify(f *model.DiskFile) string {
	ext := strings.ToLower(f.FileType)
	switch {
	case ext == "md", ext == "markdown":
		return "markdown"
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
		"php": true, "sh": true, "sql": true, "html": true, "css": true,
		"json": true, "yaml": true, "yml": true, "xml": true, "toml": true,
	}
	return codeExts[ext]
}

func isImage(ext string) bool {
	imgExts := map[string]bool{"png": true, "jpg": true, "jpeg": true, "gif": true, "svg": true, "webp": true}
	return imgExts[ext]
}

func isText(ext string) bool {
	textExts := map[string]bool{"txt": true, "log": true, "csv": true, "md": true}
	return textExts[ext]
}
