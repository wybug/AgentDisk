package storage

import (
	"context"
	"io"
	"time"
)

// Storage abstracts file storage operations (MinIO, local filesystem, etc.)
type Storage interface {
	// Upload writes data to storage under the given key.
	Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error

	// Download retrieves an object from storage as a ReadCloser.
	Download(ctx context.Context, key string) (io.ReadCloser, error)

	// Delete removes an object from storage.
	Delete(ctx context.Context, key string) error

	// Copy duplicates an object from srcKey to dstKey.
	Copy(ctx context.Context, srcKey, dstKey string) error

	// PresignedGetURL returns a URL that grants temporary read access.
	PresignedGetURL(ctx context.Context, key string, expires time.Duration) (string, error)

	// PresignedDownloadURL returns a presigned URL with Content-Disposition set
	// to trigger browser download with the given filename.
	PresignedDownloadURL(ctx context.Context, key string, expires time.Duration, filename string) (string, error)

	// EnsureBucket initializes the storage backend (creates bucket or directory).
	EnsureBucket(ctx context.Context) error
}
