package local

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/chmdznr/minio-simple-copier/v2/config"
)

type Storage struct {
	basePath   string
	folderPath string // Source folder path from config
}

func convertToWSLPath(windowsPath string) string {
	// Convert Windows path to WSL path
	// Example: D:/work/path -> /mnt/d/work/path
	if len(windowsPath) < 2 || !strings.Contains(windowsPath, ":") {
		return windowsPath // Not a Windows path
	}

	// Get drive letter and convert to lowercase
	drive := strings.ToLower(string(windowsPath[0]))
	
	// Remove drive letter and colon, convert backslashes to forward slashes
	path := filepath.ToSlash(windowsPath[2:])
	
	// Construct WSL path
	return filepath.Join("/mnt", drive, path)
}

func NewStorage(cfg *config.LocalConfig, sourceFolderPath string) (*Storage, error) {
	// Convert relative path to absolute
	absPath, err := filepath.Abs(cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	log.Printf("Debug: Using absolute path: %s", absPath)

	// Create all parent directories
	parentDir := filepath.Dir(absPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Create the final directory
	if err := os.MkdirAll(absPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	return &Storage{
		basePath:   absPath,
		folderPath: sourceFolderPath,
	}, nil
}

// SaveFile saves a file to the local storage
func (s *Storage) SaveFile(ctx context.Context, sourcePath string, reader io.Reader) error {
	// The sourcePath now includes the full path including folder structure
	// We need to maintain the same structure in the destination
	relativePath := sourcePath
	if s.folderPath != "" {
		// Remove the base folder path to get the relative structure
		relativePath = strings.TrimPrefix(sourcePath, s.folderPath+"/")
	}

	// Create the full destination path preserving folder structure
	fullPath := filepath.Join(s.basePath, relativePath)
	log.Printf("Debug: Saving file to: %s", fullPath)

	// Create all parent directories with full permissions first
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("Debug: Failed to create directory %s: %v", dir, err)
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Create file with explicit permissions
	file, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Printf("Debug: Failed to create file: %v (path: %s)", err, fullPath)
		return fmt.Errorf("failed to create file %s: %w", fullPath, err)
	}
	defer file.Close()

	// Copy data
	written, err := io.Copy(file, reader)
	if err != nil {
		log.Printf("Debug: Failed to write data: %v", err)
		return fmt.Errorf("failed to write file %s: %w", fullPath, err)
	}

	log.Printf("Debug: Successfully wrote %d bytes to %s", written, fullPath)
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
