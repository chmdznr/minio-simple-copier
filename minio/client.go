package minio

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/chmdznr/minio-simple-copier/v2/config"
)

type MinioClient struct {
	client     *minio.Client
	bucketName string
}

func NewMinioClient(cfg config.MinioConfig) (*MinioClient, error) {
	// Debug: Print input config
	log.Printf("NewMinioClient input config: UseSSL=%v", cfg.UseSSL)

	// Make a deep copy of the config
	minioConfig := config.MinioConfig{
		Endpoint:        cfg.Endpoint,
		AccessKeyID:     cfg.AccessKeyID,
		SecretAccessKey: cfg.SecretAccessKey,
		UseSSL:         cfg.UseSSL,
		BucketName:     cfg.BucketName,
	}

	// Debug: Print copied config
	log.Printf("NewMinioClient copied config: UseSSL=%v", minioConfig.UseSSL)

	// Debug: Print minio config before client creation
	log.Printf("Creating Minio client with config: %+v", minioConfig)

	opts := &minio.Options{
		Creds:  credentials.NewStaticV4(minioConfig.AccessKeyID, minioConfig.SecretAccessKey, ""),
		Secure: minioConfig.UseSSL,
	}

	// Debug: Print options
	log.Printf("Minio client options: Secure=%v", opts.Secure)

	client, err := minio.New(minioConfig.Endpoint, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	return &MinioClient{
		client:     client,
		bucketName: minioConfig.BucketName,
	}, nil
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

		objectCh := m.client.ListObjects(ctx, m.bucketName, minio.ListObjectsOptions{
			Recursive: true,
		})

		for object := range objectCh {
			if object.Err != nil {
				errorChan <- object.Err
				return
			}

			filesChan <- FileInfo{
				Path:         object.Key,
				Size:         object.Size,
				ETag:         object.ETag,
				LastModified: object.LastModified,
			}
		}
	}()

	return filesChan, errorChan
}

func (m *MinioClient) CopyFile(ctx context.Context, destClient *MinioClient, objectPath string) error {
	// Get source object
	object, err := m.client.GetObject(ctx, m.bucketName, objectPath, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to get source object: %w", err)
	}
	defer object.Close()

	objectInfo, err := object.Stat()
	if err != nil {
		return fmt.Errorf("failed to get object info: %w", err)
	}

	// Prepare destination path
	destPath := objectPath

	// For large files, use multipart upload
	if objectInfo.Size > 64*1024*1024 { // 64MB threshold
		_, err = destClient.client.PutObject(
			ctx,
			destClient.bucketName,
			destPath,
			object,
			objectInfo.Size,
			minio.PutObjectOptions{
				ContentType: objectInfo.ContentType,
				PartSize:    64 * 1024 * 1024, // 64MB parts
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

	if err != nil {
		return fmt.Errorf("failed to copy object: %w", err)
	}

	return nil
}

func (m *MinioClient) GetObject(ctx context.Context, objectPath string) (*minio.Object, error) {
	return m.client.GetObject(ctx, m.bucketName, objectPath, minio.GetObjectOptions{})
}

func (m *MinioClient) StatObject(ctx context.Context, objectPath string) (minio.ObjectInfo, error) {
	return m.client.StatObject(ctx, m.bucketName, objectPath, minio.StatObjectOptions{})
}
