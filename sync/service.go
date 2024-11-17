package sync

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/chmdznr/minio-simple-copier/v2/config"
	"github.com/chmdznr/minio-simple-copier/v2/db"
	"github.com/chmdznr/minio-simple-copier/v2/local"
	"github.com/chmdznr/minio-simple-copier/v2/minio"
)

type Service struct {
	sourceClient *minio.MinioClient
	destClient   *minio.MinioClient
	localDest    *local.Storage
	destType     config.DestinationType
	database     *db.Database
	projectName  string
}

type SyncStatus struct {
	Counts       []db.StatusCount
	RecentErrors []*db.FileEntry
}

func NewService(cfg config.ProjectConfig) (*Service, error) {
	sourceClient, err := minio.NewMinioClient(cfg.SourceMinio)
	if err != nil {
		return nil, fmt.Errorf("failed to create source minio client: %w", err)
	}

	var destClient *minio.MinioClient
	var localDest *local.Storage

	switch cfg.DestType {
	case config.DestinationMinio:
		destClient, err = minio.NewMinioClient(cfg.DestMinio)
		if err != nil {
			return nil, fmt.Errorf("failed to create destination minio client: %w", err)
		}
	case config.DestinationLocal:
		localDest, err = local.NewStorage(cfg.DestLocal)
		if err != nil {
			return nil, fmt.Errorf("failed to create local storage: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported destination type: %s", cfg.DestType)
	}

	database, err := db.NewDatabase(cfg.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}

	if err := database.Initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return &Service{
		sourceClient: sourceClient,
		destClient:  destClient,
		localDest:   localDest,
		destType:    cfg.DestType,
		database:    database,
		projectName: cfg.ProjectName,
	}, nil
}

func (s *Service) Close() error {
	return s.database.Close()
}

func (s *Service) UpdateSourceFileList(ctx context.Context) error {
	filesChan, errorsChan := s.sourceClient.ListFiles(ctx)

	for {
		select {
		case file, ok := <-filesChan:
			if !ok {
				return nil
			}

			existingFile, err := s.database.GetFileByPath(s.projectName, file.Path)
			if err != nil {
				return fmt.Errorf("failed to check existing file: %w", err)
			}

			if existingFile == nil {
				// New file, insert it
				entry := &db.FileEntry{
					ProjectName:  s.projectName,
					Path:        file.Path,
					Size:        file.Size,
					ETag:        file.ETag,
					LastModified: file.LastModified,
					Status:      db.StatusPending,
				}
				if err := s.database.InsertFileEntry(entry); err != nil {
					return fmt.Errorf("failed to insert file entry: %w", err)
				}
			} else if existingFile.ETag != file.ETag {
				// File changed, update status to pending
				if err := s.database.UpdateFileStatus(existingFile.ID, db.StatusPending, ""); err != nil {
					return fmt.Errorf("failed to update file status: %w", err)
				}
			}

		case err := <-errorsChan:
			if err != nil {
				return fmt.Errorf("error listing files: %w", err)
			}
			return nil

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (s *Service) SyncFiles(ctx context.Context, workers int) error {
	var wg sync.WaitGroup
	errorsChan := make(chan error, workers)

	// Start worker goroutines
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				// Get pending files in batches
				files, err := s.database.GetPendingFiles(s.projectName, 10)
				if err != nil {
					errorsChan <- fmt.Errorf("failed to get pending files: %w", err)
					return
				}

				if len(files) == 0 {
					return
				}

				for _, file := range files {
					select {
					case <-ctx.Done():
						return
					default:
						if err := s.syncFile(ctx, file); err != nil {
							log.Printf("Error syncing file %s: %v", file.Path, err)
							if err := s.database.UpdateFileStatus(file.ID, db.StatusError, err.Error()); err != nil {
								errorsChan <- fmt.Errorf("failed to update file status: %w", err)
								return
							}
						}
					}
				}
			}
		}()
	}

	// Wait for all workers to finish
	wg.Wait()
	close(errorsChan)

	// Check for any errors
	for err := range errorsChan {
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) syncFile(ctx context.Context, file *db.FileEntry) error {
	// Update status to copying
	if err := s.database.UpdateFileStatus(file.ID, db.StatusCopying, ""); err != nil {
		return fmt.Errorf("failed to update file status: %w", err)
	}

	var exists bool
	var err error

	// Check if file exists in destination
	switch s.destType {
	case config.DestinationMinio:
		_, err = s.destClient.StatObject(ctx, file.Path)
		exists = err == nil
	case config.DestinationLocal:
		exists, err = s.localDest.FileExists(file.Path)
		if err != nil {
			return fmt.Errorf("failed to check file existence: %w", err)
		}
	}

	if exists {
		// File exists, update status
		return s.database.UpdateFileStatus(file.ID, db.StatusExists, "")
	}

	// Get source object
	object, err := s.sourceClient.GetObject(ctx, file.Path)
	if err != nil {
		return fmt.Errorf("failed to get source object: %w", err)
	}
	defer object.Close()

	objectInfo, err := object.Stat()
	if err != nil {
		return fmt.Errorf("failed to get object info: %w", err)
	}

	// Copy file based on destination type
	switch s.destType {
	case config.DestinationMinio:
		err = s.sourceClient.CopyFile(ctx, s.destClient, file.Path)
	case config.DestinationLocal:
		err = s.localDest.WriteFile(ctx, file.Path, object, objectInfo.Size)
	}

	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// Update status to completed
	return s.database.UpdateFileStatus(file.ID, db.StatusCompleted, "")
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
