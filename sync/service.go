package sync

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/chmdznr/minio-simple-copier/v2/config"
	"github.com/chmdznr/minio-simple-copier/v2/db"
	"github.com/chmdznr/minio-simple-copier/v2/local"
	"github.com/chmdznr/minio-simple-copier/v2/minio"
)

type MCListEntry struct {
	Status       string    `json:"status"`
	Type         string    `json:"type"`
	LastModified time.Time `json:"lastModified"`
	Size         int64     `json:"size"`
	Key          string    `json:"key"`
	ETag         string    `json:"etag"`
}

// Service handles file synchronization
type Service struct {
	projectName  string
	sourceClient *minio.MinioClient
	destType     config.DestinationType
	destClient   *minio.MinioClient
	localDest    *local.Storage
	database     *db.Database
}

// NewService creates a new sync service
func NewService(cfg *config.ProjectConfig) (*Service, error) {
	// Create source client
	sourceClient, err := minio.NewMinioClient(&cfg.SourceMinio)
	if err != nil {
		return nil, fmt.Errorf("failed to create source client: %w", err)
	}

	// Create destination client based on type
	var destClient *minio.MinioClient
	var localDest *local.Storage

	switch cfg.DestType {
	case config.DestinationMinio:
		destClient, err = minio.NewMinioClient(&cfg.DestMinio)
		if err != nil {
			return nil, fmt.Errorf("failed to create destination client: %w", err)
		}
	case config.DestinationLocal:
		localDest, err = local.NewStorage(&cfg.DestLocal, cfg.SourceMinio.FolderPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create local storage: %w", err)
		}
	}

	// Create database
	database, err := db.NewDatabase(cfg.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}

	if err := database.Initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return &Service{
		projectName:  cfg.ProjectName,
		sourceClient: sourceClient,
		destType:     cfg.DestType,
		destClient:   destClient,
		localDest:    localDest,
		database:     database,
	}, nil
}

func (s *Service) Close() error {
	if s.database != nil {
		return s.database.Close()
	}
	return nil
}

func (s *Service) UpdateSourceList(ctx context.Context) error {
	log.Printf("Updating source file list...")

	// List objects in source bucket
	objects, err := s.sourceClient.ListObjects(ctx)
	if err != nil {
		return fmt.Errorf("failed to list objects: %w", err)
	}

	log.Printf("Found %d files in source bucket", len(objects))

	// Add each file to database
	var added, skipped int
	for _, obj := range objects {
		// Skip if file already exists in database
		exists, err := s.database.GetFileByPath(s.projectName, obj.Key)
		if err != nil {
			log.Printf("Warning: Failed to check file existence: %v", err)
			continue
		}

		if exists != nil {
			// If file exists but ETag is different, update it
			if exists.ETag != obj.ETag {
				log.Printf("Debug: Updating file %s (ETag changed: %s -> %s)", obj.Key, exists.ETag, obj.ETag)
				exists.Size = obj.Size
				exists.ETag = obj.ETag
				exists.LastModified = obj.LastModified
				exists.Status = db.StatusPending
				exists.ErrorMessage = ""
				if err := s.database.UpdateFileStatus(exists.ID, exists.Status, exists.ErrorMessage); err != nil {
					log.Printf("Warning: Failed to update file status: %v", err)
				}
				added++
			} else {
				log.Printf("Debug: Skipping file %s (already exists with same ETag)", obj.Key)
				skipped++
			}
			continue
		}

		// Add new file to database
		entry := &db.FileEntry{
			ProjectName:  s.projectName,
			Path:         obj.Key,
			Size:         obj.Size,
			ETag:         obj.ETag,
			LastModified: obj.LastModified,
			Status:       db.StatusPending,
		}

		if err := s.database.InsertFileEntry(entry); err != nil {
			log.Printf("Warning: Failed to insert file entry: %v", err)
			continue
		}

		log.Printf("Debug: Added file to database: %s (size: %d, etag: %s)", obj.Key, obj.Size, obj.ETag)
		added++
	}

	log.Printf("Summary: Added/Updated %d files, Skipped %d files", added, skipped)

	// Print status distribution
	counts, err := s.database.GetStatusCounts(s.projectName)
	if err != nil {
		log.Printf("Warning: Failed to get status counts: %v", err)
	} else {
		log.Printf("Debug: Status distribution after update:")
		for _, count := range counts {
			log.Printf("  - Status %s: %d files (%d bytes)", count.Status, count.Count, count.Size)
		}
	}

	return nil
}

func (s *Service) StartSync(ctx context.Context, workers int) error {
	log.Printf("Starting sync with %d workers...", workers)

	// Get pending files
	files, err := s.database.GetPendingFiles(s.projectName, 0) // 0 means get all pending files
	if err != nil {
		return fmt.Errorf("failed to get pending files: %w", err)
	}

	log.Printf("Found %d pending files to sync", len(files))
	if len(files) == 0 {
		return nil
	}

	// Create worker pool
	var wg sync.WaitGroup
	filesChan := make(chan *db.FileEntry, workers)
	errorsChan := make(chan error, workers)
	doneChan := make(chan bool)

	// Start workers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for file := range filesChan {
				log.Printf("Worker %d: Processing file: %s", workerID, file.Path)

				// Get file from source
				reader, err := s.sourceClient.GetObject(ctx, file.Path)
				if err != nil {
					log.Printf("Worker %d: Failed to get file %s: %v", workerID, file.Path, err)
					errorsChan <- fmt.Errorf("failed to get file %s: %w", file.Path, err)
					continue
				}
				defer reader.Close()

				log.Printf("Worker %d: Got file %s", workerID, file.Path)

				// Save file to destination
				if s.destType == config.DestinationLocal {
					if err := s.localDest.SaveFile(ctx, file.Path, reader); err != nil {
						log.Printf("Worker %d: Failed to save file %s: %v", workerID, file.Path, err)
						errorsChan <- fmt.Errorf("failed to save file %s: %w", file.Path, err)
						continue
					}
				} else {
					if err := s.destClient.PutObject(ctx, file.Path, reader, file.Size); err != nil {
						log.Printf("Worker %d: Failed to save file %s: %v", workerID, file.Path, err)
						errorsChan <- fmt.Errorf("failed to save file %s: %w", file.Path, err)
						continue
					}
				}

				log.Printf("Worker %d: Successfully saved file %s", workerID, file.Path)

				// Update file status
				if err := s.database.UpdateFileStatus(file.ID, db.StatusCompleted, ""); err != nil {
					log.Printf("Worker %d: Failed to update file status for %s: %v", workerID, file.Path, err)
					errorsChan <- fmt.Errorf("failed to update file status: %w", err)
					continue
				}

				log.Printf("Worker %d: Updated status for file %s", workerID, file.Path)
			}
		}(i)
	}

	// Send files to workers
	go func() {
		for _, file := range files {
			filesChan <- file
		}
		close(filesChan)
	}()

	// Wait for workers to finish
	go func() {
		wg.Wait()
		close(errorsChan)
		doneChan <- true
	}()

	// Collect errors
	var errors []error
	for {
		select {
		case err, ok := <-errorsChan:
			if !ok {
				continue
			}
			errors = append(errors, err)
		case <-doneChan:
			if len(errors) > 0 {
				return fmt.Errorf("sync completed with %d errors", len(errors))
			}
			log.Println("Sync completed successfully")
			return nil
		}
	}
}

func (s *Service) GetStatus() (*SyncStatus, error) {
	counts, err := s.database.GetStatusCounts(s.projectName)
	if err != nil {
		return nil, fmt.Errorf("failed to get status counts: %w", err)
	}

	recentErrors, err := s.database.GetRecentErrors(s.projectName, 10)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent errors: %w", err)
	}

	return &SyncStatus{
		Counts:       counts,
		RecentErrors: recentErrors,
	}, nil
}

type SyncStatus struct {
	Counts       []db.StatusCount
	RecentErrors []*db.FileEntry
}

// ImportFileList imports a list of file paths into the database
func (s *Service) ImportFileList(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("no file paths provided")
	}

	// Read JSON entries from the file
	file, err := os.Open(paths[0]) // paths[0] is the import file path
	if err != nil {
		return fmt.Errorf("failed to open import file: %w", err)
	}
	defer file.Close()

	var entries []MCListEntry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var entry MCListEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			log.Printf("Warning: Failed to parse JSON line: %v", err)
			continue
		}

		// Skip non-file entries
		if entry.Type != "file" {
			continue
		}

		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading import file: %w", err)
	}

	log.Printf("Importing %d files...", len(entries))

	// Process each entry
	for _, entry := range entries {
		// Add folder prefix to path if set
		filePath := entry.Key
		if s.sourceClient.GetFolderPath() != "" {
			filePath = path.Join(s.sourceClient.GetFolderPath(), entry.Key)
		}

		// Skip if file already exists in database
		exists, err := s.database.GetFileByPath(s.projectName, filePath)
		if err != nil {
			log.Printf("Warning: Failed to check file existence: %v", err)
			continue
		}
		if exists != nil {
			log.Printf("Debug: Skipping file %s (already exists in database)", filePath)
			continue
		}

		// Add file to database
		dbEntry := &db.FileEntry{
			ProjectName:  s.projectName,
			Path:         filePath,
			Size:         entry.Size,
			ETag:         entry.ETag,
			LastModified: entry.LastModified,
			Status:       db.StatusPending,
		}

		if err := s.database.InsertFileEntry(dbEntry); err != nil {
			log.Printf("Warning: Failed to insert file entry: %v", err)
			continue
		}

		log.Printf("Debug: Added file to database: %s (size: %d, etag: %s)", filePath, entry.Size, entry.ETag)
	}

	// Print status distribution
	counts, err := s.database.GetStatusCounts(s.projectName)
	if err != nil {
		log.Printf("Warning: Failed to get status counts: %v", err)
	} else {
		log.Printf("Debug: Status distribution after import:")
		for _, count := range counts {
			log.Printf("  - Status %s: %d files (%d bytes)", count.Status, count.Count, count.Size)
		}
	}

	return nil
}
