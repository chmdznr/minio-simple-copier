package local

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"minio-simple-copier/config"
)

type Storage struct {
	basePath string
	mu       sync.Mutex
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

func (s *Storage) ensureDir(path string) error {
	dir := filepath.Dir(path)
	s.mu.Lock()
	defer s.mu.Unlock()
	return os.MkdirAll(dir, 0755)
}

func (s *Storage) WriteFile(ctx context.Context, objectPath string, reader io.Reader, size int64) error {
	fullPath := filepath.Join(s.basePath, filepath.FromSlash(objectPath))

	// Ensure directory exists
	if err := s.ensureDir(fullPath); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create temporary file
	tmpPath := fullPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer func() {
		f.Close()
		if err != nil {
			os.Remove(tmpPath)
		}
	}()

	// Copy data
	_, err = io.Copy(f, reader)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Sync to disk
	if err := f.Sync(); err != nil {
		return fmt.Errorf("failed to sync file: %w", err)
	}

	// Close file before renaming
	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close file: %w", err)
	}

	// Rename temporary file to final name
	if err := os.Rename(tmpPath, fullPath); err != nil {
		return fmt.Errorf("failed to rename file: %w", err)
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
