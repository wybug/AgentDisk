package oss

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Client is a core domain type.
type Client struct {
	mc     *minio.Client
	bucket string
}

// NewClient creates and returns a new Client.
func NewClient(endpoint, accessKey, secretKey, bucket, region string, useSSL bool) (*Client, error) {
	mc, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
		Region: region,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}
	return &Client{mc: mc, bucket: bucket}, nil
}

// EnsureBucket executes the EnsureBucket use case.
func (c *Client) EnsureBucket(ctx context.Context) error {
	exists, err := c.mc.BucketExists(ctx, c.bucket)
	if err != nil {
		return fmt.Errorf("check bucket: %w", err)
	}
	if !exists {
		if err := c.mc.MakeBucket(ctx, c.bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("create bucket: %w", err)
		}
	}
	return nil
}

// Upload executes the Upload use case.
func (c *Client) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	_, err := c.mc.PutObject(ctx, c.bucket, key, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

// Download executes the Download use case.
func (c *Client) Download(ctx context.Context, key string) (*minio.Object, error) {
	return c.mc.GetObject(ctx, c.bucket, key, minio.GetObjectOptions{})
}

// Delete executes the Delete use case.
func (c *Client) Delete(ctx context.Context, key string) error {
	return c.mc.RemoveObject(ctx, c.bucket, key, minio.RemoveObjectOptions{})
}

// Copy executes the Copy use case.
func (c *Client) Copy(ctx context.Context, srcKey, dstKey string) error {
	src := minio.CopySrcOptions{Bucket: c.bucket, Object: srcKey}
	dst := minio.CopyDestOptions{Bucket: c.bucket, Object: dstKey}
	_, err := c.mc.CopyObject(ctx, dst, src)
	return err
}

// PresignedGetURL executes the PresignedGetURL use case.
func (c *Client) PresignedGetURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	reqParams := make(url.Values)
	u, err := c.mc.PresignedGetObject(ctx, c.bucket, key, expires, reqParams)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

// BuildKey constructs OSS key following the path convention:
// disk/user_{userId}/{fullPath}/{fileId}_{fileName}
func BuildKey(userID, fullPath string, fileID uint64, fileName string) string {
	if fullPath != "" {
		return fmt.Sprintf("disk/user_%s/%s/%d_%s", userID, fullPath, fileID, fileName)
	}
	return fmt.Sprintf("disk/user_%s/%d_%s", userID, fileID, fileName)
}
