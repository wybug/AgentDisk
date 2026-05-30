package storage

import (
	"context"
	"io"
	"time"

	"github.com/agentdisk/agent-disk/pkg/oss"
)

// MinioStorage wraps oss.Client to implement Storage.
type MinioStorage struct {
	client *oss.Client
}

// NewMinioStorage creates a new MinioStorage.
func NewMinioStorage(client *oss.Client) *MinioStorage {
	return &MinioStorage{client: client}
}

// Upload delegates to oss.Client.Upload.
func (m *MinioStorage) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	return m.client.Upload(ctx, key, reader, size, contentType)
}

// Download delegates to oss.Client.Download.
func (m *MinioStorage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	return m.client.Download(ctx, key)
}

// Delete delegates to oss.Client.Delete.
func (m *MinioStorage) Delete(ctx context.Context, key string) error {
	return m.client.Delete(ctx, key)
}

// Copy delegates to oss.Client.Copy.
func (m *MinioStorage) Copy(ctx context.Context, srcKey, dstKey string) error {
	return m.client.Copy(ctx, srcKey, dstKey)
}

// PresignedGetURL delegates to oss.Client.PresignedGetURL.
func (m *MinioStorage) PresignedGetURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	return m.client.PresignedGetURL(ctx, key, expires)
}

// EnsureBucket delegates to oss.Client.EnsureBucket.
func (m *MinioStorage) EnsureBucket(ctx context.Context) error {
	return m.client.EnsureBucket(ctx)
}
