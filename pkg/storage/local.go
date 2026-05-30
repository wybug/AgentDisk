package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// LocalStorage implements Storage using the local filesystem.
type LocalStorage struct {
	rootDir string
	hmacKey string
}

// NewLocalStorage creates a new LocalStorage.
func NewLocalStorage(rootDir, hmacKey string) *LocalStorage {
	return &LocalStorage{rootDir: rootDir, hmacKey: hmacKey}
}

// Upload writes data to the local filesystem under the given key.
func (l *LocalStorage) Upload(_ context.Context, key string, reader io.Reader, _ int64, _ string) error {
	fullPath := filepath.Join(l.rootDir, key) //nolint:gosec // key is internally generated
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
		return fmt.Errorf("create directories: %w", err)
	}
	f, err := os.Create(fullPath) //nolint:gosec // key is internally generated
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer func() { _ = f.Close() }()
	_, err = io.Copy(f, reader)
	return err
}

// Download retrieves a file from the local filesystem.
func (l *LocalStorage) Download(_ context.Context, key string) (io.ReadCloser, error) {
	fullPath := filepath.Join(l.rootDir, key) //nolint:gosec // key is internally generated
	f, err := os.Open(fullPath)               //nolint:gosec // key is internally generated
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	return f, nil
}

// Delete removes a file from the local filesystem.
func (l *LocalStorage) Delete(_ context.Context, key string) error {
	fullPath := filepath.Join(l.rootDir, key) //nolint:gosec // key is internally generated
	return os.Remove(fullPath)
}

// Copy duplicates a file from srcKey to dstKey on the local filesystem.
func (l *LocalStorage) Copy(_ context.Context, srcKey, dstKey string) error {
	srcPath := filepath.Join(l.rootDir, srcKey) //nolint:gosec // key is internally generated
	dstPath := filepath.Join(l.rootDir, dstKey) //nolint:gosec // key is internally generated
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o750); err != nil {
		return fmt.Errorf("create directories: %w", err)
	}
	src, err := os.Open(srcPath) //nolint:gosec // key is internally generated
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer func() { _ = src.Close() }()
	dst, err := os.Create(dstPath) //nolint:gosec // key is internally generated
	if err != nil {
		return fmt.Errorf("create destination: %w", err)
	}
	defer func() { _ = dst.Close() }()
	_, err = io.Copy(dst, src)
	return err
}

// PresignedGetURL returns a signed URL served by the Go backend.
func (l *LocalStorage) PresignedGetURL(_ context.Context, key string, expires time.Duration) (string, error) {
	exp := time.Now().Add(expires).Unix()
	sig := l.sign(key, exp)
	return fmt.Sprintf("/v1/disk/local-storage/%s?exp=%d&sig=%s", key, exp, sig), nil
}

// EnsureBucket creates the root directory for local storage.
func (l *LocalStorage) EnsureBucket(_ context.Context) error {
	return os.MkdirAll(l.rootDir, 0o750)
}

func (l *LocalStorage) sign(key string, exp int64) string {
	mac := hmac.New(sha256.New, []byte(l.hmacKey))
	fmt.Fprintf(mac, "%s|%d", key, exp)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// VerifySignature checks the HMAC signature for local storage presigned URLs.
func (l *LocalStorage) VerifySignature(key string, exp int64, sig string) bool {
	expected := l.sign(key, exp)
	return hmac.Equal([]byte(expected), []byte(sig))
}
