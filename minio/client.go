package minio

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/chmdznr/minio-simple-copier/v2/config"
)

const (
	maxRetries    = 3
	retryInterval = 5 * time.Second
)

type MinioClient struct {
	client     *minio.Client
	bucketName string
}

func NewMinioClient(cfg config.MinioConfig) (*MinioClient, error) {
	// Make a deep copy of the config
	minioConfig := config.MinioConfig{
		Endpoint:        cfg.Endpoint,
		AccessKeyID:     cfg.AccessKeyID,
		SecretAccessKey: cfg.SecretAccessKey,
		UseSSL:         cfg.UseSSL,
		BucketName:     cfg.BucketName,
	}

	client, err := minio.New(minioConfig.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(minioConfig.AccessKeyID, minioConfig.SecretAccessKey, ""),
		Secure: minioConfig.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	return &MinioClient{
		client:     client,
		bucketName: minioConfig.BucketName,
	}, nil
}

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for timeout errors
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}

	// Check for HTTP timeouts and connection errors
	if urlErr, ok := err.(*url.Error); ok {
		if netErr, ok := urlErr.Err.(net.Error); ok && netErr.Timeout() {
			return true
		}
		if urlErr.Timeout() {
			return true
		}
	}

	// Check for specific HTTP errors
	if httpErr, ok := err.(*url.Error); ok {
		if httpErr.Err == context.DeadlineExceeded {
			return true
		}
	}

	return false
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
				return err // Don't retry if error is not retryable
			}
			continue
		}
		return nil // Success
	}
	return fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

type FileInfo struct {
	Path         string
	Size         int64
	ETag         string
	LastModified time.Time
}

func (m *MinioClient) ListFiles(ctx context.Context) (<-chan FileInfo, <-chan error) {
	filesChan := make(chan FileInfo)
	errorChan := make(chan error, 1)

	go func() {
		defer close(filesChan)
		defer close(errorChan)

		err := m.withRetry("ListObjects", func() error {
			objectCh := m.client.ListObjects(ctx, m.bucketName, minio.ListObjectsOptions{
				Recursive: true,
			})

			for object := range objectCh {
				if object.Err != nil {
					return fmt.Errorf("error listing objects: %w", object.Err)
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				case filesChan <- FileInfo{
					Path:         object.Key,
					Size:         object.Size,
					ETag:         object.ETag,
					LastModified: object.LastModified,
				}:
				}
			}
			return nil
		})

		if err != nil {
			errorChan <- fmt.Errorf("error listing files: %w", err)
		}
	}()

	return filesChan, errorChan
}

func (m *MinioClient) CopyFile(ctx context.Context, destClient *MinioClient, objectPath string) error {
	return m.withRetry("CopyFile", func() error {
		// Get source object
		object, err := m.GetObject(ctx, objectPath)
		if err != nil {
			return fmt.Errorf("failed to get source object: %w", err)
		}
		defer object.Close()

		// Get object info
		objectInfo, err := m.StatObject(ctx, objectPath)
		if err != nil {
			return fmt.Errorf("failed to get object info: %w", err)
		}

		destPath := objectPath
		if objectInfo.Size > 64*1024*1024 { // 64MB
			// For large files, use multipart copy
			_, err = destClient.client.ComposeObject(
				ctx,
				minio.CopyDestOptions{
					Bucket: destClient.bucketName,
					Object: destPath,
				},
				minio.CopySrcOptions{
					Bucket: m.bucketName,
					Object: objectPath,
				},
			)
		} else {
			// For smaller files, use regular upload
			_, err = destClient.client.PutObject(
				ctx,
				destClient.bucketName,
				destPath,
				object,
				objectInfo.Size,
				minio.PutObjectOptions{
					ContentType: objectInfo.ContentType,
				},
			)
		}
		return err
	})
}

func (m *MinioClient) GetObject(ctx context.Context, objectPath string) (*minio.Object, error) {
	var obj *minio.Object
	err := m.withRetry("GetObject", func() error {
		var err error
		obj, err = m.client.GetObject(ctx, m.bucketName, objectPath, minio.GetObjectOptions{})
		return err
	})
	return obj, err
}

func (m *MinioClient) StatObject(ctx context.Context, objectPath string) (minio.ObjectInfo, error) {
	var info minio.ObjectInfo
	err := m.withRetry("StatObject", func() error {
		var err error
		info, err = m.client.StatObject(ctx, m.bucketName, objectPath, minio.StatObjectOptions{})
		return err
	})
	return info, err
}
