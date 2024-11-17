package minio

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/chmdznr/minio-simple-copier/v2/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	maxRetries    = 3
	retryInterval = 5 * time.Second
)

type MinioClient struct {
	client     *minio.Client
	bucketName string
	folderPath string
}

type ObjectInfo struct {
	Key          string
	Size         int64
	ETag         string
	LastModified time.Time
}

func NewMinioClient(cfg *config.MinioConfig) (*MinioClient, error) {
	// Initialize minio client
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	return &MinioClient{
		client:     client,
		bucketName: cfg.BucketName,
		folderPath: cfg.FolderPath,
	}, nil
}

func (m *MinioClient) GetFolderPath() string {
	return m.folderPath
}

func (m *MinioClient) ListObjects(ctx context.Context) ([]ObjectInfo, error) {
	log.Printf("Debug: Listing objects in bucket %s with prefix %s", m.bucketName, m.folderPath)

	// Create done channel to control the listing
	objectCh := m.client.ListObjects(ctx, m.bucketName, minio.ListObjectsOptions{
		Prefix:    m.folderPath,
		Recursive: true,
	})

	var objects []ObjectInfo
	for object := range objectCh {
		if object.Err != nil {
			return nil, fmt.Errorf("error listing objects: %w", object.Err)
		}

		// Skip folders
		if strings.HasSuffix(object.Key, "/") {
			continue
		}

		// Keep the full path including folder structure
		objects = append(objects, ObjectInfo{
			Key:          object.Key,
			Size:         object.Size,
			ETag:         object.ETag,
			LastModified: object.LastModified,
		})

		log.Printf("Debug: Found object: %s (size: %d, etag: %s)", object.Key, object.Size, object.ETag)
	}

	return objects, nil
}

func (m *MinioClient) GetObject(ctx context.Context, objectPath string) (io.ReadCloser, error) {
	// The objectPath should already include the full path
	log.Printf("Debug: Getting object: %s", objectPath)

	var obj *minio.Object
	err := m.withRetry("GetObject", func() error {
		var err error
		obj, err = m.client.GetObject(ctx, m.bucketName, objectPath, minio.GetObjectOptions{})
		return err
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get object %s: %w", objectPath, err)
	}

	return obj, nil
}

func (m *MinioClient) PutObject(ctx context.Context, objectPath string, reader io.Reader, size int64) error {
	// The objectPath should already include the full path
	log.Printf("Debug: Putting object: %s (size: %d)", objectPath, size)

	// Put object with retry
	err := m.withRetry("PutObject", func() error {
		_, err := m.client.PutObject(ctx, m.bucketName, objectPath, reader, size, minio.PutObjectOptions{})
		return err
	})

	if err != nil {
		return fmt.Errorf("failed to put object: %w", err)
	}

	return nil
}

func (m *MinioClient) StatObject(ctx context.Context, objectPath string) (*ObjectInfo, error) {
	log.Printf("Debug: Getting object info: %s", objectPath)

	var info minio.ObjectInfo
	err := m.withRetry("StatObject", func() error {
		var err error
		info, err = m.client.StatObject(ctx, m.bucketName, objectPath, minio.StatObjectOptions{})
		return err
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get object info %s: %w", objectPath, err)
	}

	return &ObjectInfo{
		Key:          info.Key,
		Size:         info.Size,
		ETag:         info.ETag,
		LastModified: info.LastModified,
	}, nil
}

func (m *MinioClient) withRetry(operation string, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("Retrying %s (attempt %d/%d) after error: %v", operation, attempt+1, maxRetries, lastErr)
			time.Sleep(retryInterval)
		}

		if err := fn(); err != nil {
			lastErr = err
			if !isRetryableError(err) {
				return err
			}
			continue
		}
		return nil
	}
	return fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}

	if urlErr, ok := err.(*url.Error); ok {
		if netErr, ok := urlErr.Err.(net.Error); ok && netErr.Timeout() {
			return true
		}
		if urlErr.Timeout() {
			return true
		}
	}

	if httpErr, ok := err.(*url.Error); ok {
		if httpErr.Err == context.DeadlineExceeded {
			return true
		}
	}

	return false
}
