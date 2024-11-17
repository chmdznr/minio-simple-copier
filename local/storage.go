package local

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/chmdznr/minio-simple-copier/v2/config"
)

type Storage struct {
	basePath string
}

func NewStorage(cfg config.LocalConfig) (*Storage, error) {
	// Create base directory if it doesn't exist
	if err := os.MkdirAll(cfg.Path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	return &Storage{
		basePath: cfg.Path,
	}, nil
}

func (s *Storage) SaveFile(ctx context.Context, sourcePath string, reader io.Reader) error {
	// Preserve folder structure from source path
	destPath := filepath.Join(s.basePath, sourcePath)

	// Create parent directories if they don't exist
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// Create destination file
	file, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy data from reader to file
	if _, err := io.Copy(file, reader); err != nil {
		return fmt.Errorf("failed to copy data: %w", err)
	}

	return nil
}

func (s *Storage) FileExists(objectPath string) (bool, error) {
	fullPath := filepath.Join(s.basePath, filepath.FromSlash(objectPath))
	_, err := os.Stat(fullPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
