package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"minio-simple-copier/config"
	"minio-simple-copier/sync"
)

const projectsDir = "projects"

func main() {
	// Command line flags
	var (
		projectName     = flag.String("project", "", "Project name")
		sourceEndpoint  = flag.String("source-endpoint", "", "Source Minio endpoint")
		sourceAccessKey = flag.String("source-access-key", "", "Source Minio access key")
		sourceSecretKey = flag.String("source-secret-key", "", "Source Minio secret key")
		sourceBucket    = flag.String("source-bucket", "", "Source Minio bucket")
		sourceUseSSL    = flag.Bool("source-use-ssl", true, "Use SSL for source Minio")

		destEndpoint  = flag.String("dest-endpoint", "", "Destination Minio endpoint")
		destAccessKey = flag.String("dest-access-key", "", "Destination Minio access key")
		destSecretKey = flag.String("dest-secret-key", "", "Destination Minio secret key")
		destBucket    = flag.String("dest-bucket", "", "Destination Minio bucket")
		destUseSSL    = flag.Bool("dest-use-ssl", true, "Use SSL for destination Minio")

		workers = flag.Int("workers", 5, "Number of concurrent workers")
		command = flag.String("command", "", "Command to execute (update-list, sync, status, config)")
	)

	flag.Parse()

	if *projectName == "" {
		log.Fatal("Project name is required")
	}

	if *command == "" {
		log.Fatal("Command is required")
	}

	// Load config file
	fileConfig, err := config.LoadConfig(projectsDir)
	if err != nil {
		log.Fatalf("Failed to load config file: %v", err)
	}

	// Create project directory if it doesn't exist
	projectDir := filepath.Join(projectsDir, *projectName)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		log.Fatalf("Failed to create project directory: %v", err)
	}

	// Handle config command first
	if *command == "config" {
		// Save new config
		minioConfig := config.ProjectMinioConfig{
			Source: config.MinioConfig{
				Endpoint:        *sourceEndpoint,
				AccessKeyID:     *sourceAccessKey,
				SecretAccessKey: *sourceSecretKey,
				UseSSL:         *sourceUseSSL,
				BucketName:     *sourceBucket,
			},
			Dest: config.MinioConfig{
				Endpoint:        *destEndpoint,
				AccessKeyID:     *destAccessKey,
				SecretAccessKey: *destSecretKey,
				UseSSL:         *destUseSSL,
				BucketName:     *destBucket,
			},
		}

		fileConfig.SetProjectConfig(*projectName, minioConfig)
		if err := config.SaveConfig(projectsDir, fileConfig); err != nil {
			log.Fatalf("Failed to save config: %v", err)
		}
		fmt.Printf("Configuration saved for project %s\n", *projectName)
		return
	}

	// Get project config
	projectConfig, exists := fileConfig.GetProjectConfig(*projectName)
	if !exists {
		log.Fatalf("No configuration found for project %s. Please run config command first.", *projectName)
	}

	// Create configuration using saved values and any overrides from command line
	cfg := config.ProjectConfig{
		ProjectName: *projectName,
		SourceMinio: projectConfig.Source,
		DestMinio:   projectConfig.Dest,
		DatabasePath: filepath.Join(projectDir, "files.db"),
	}

	// Override with command line values if provided
	if *sourceEndpoint != "" {
		cfg.SourceMinio.Endpoint = *sourceEndpoint
	}
	if *sourceAccessKey != "" {
		cfg.SourceMinio.AccessKeyID = *sourceAccessKey
	}
	if *sourceSecretKey != "" {
		cfg.SourceMinio.SecretAccessKey = *sourceSecretKey
	}
	if *sourceBucket != "" {
		cfg.SourceMinio.BucketName = *sourceBucket
	}
	cfg.SourceMinio.UseSSL = *sourceUseSSL

	if *destEndpoint != "" {
		cfg.DestMinio.Endpoint = *destEndpoint
	}
	if *destAccessKey != "" {
		cfg.DestMinio.AccessKeyID = *destAccessKey
	}
	if *destSecretKey != "" {
		cfg.DestMinio.SecretAccessKey = *destSecretKey
	}
	if *destBucket != "" {
		cfg.DestMinio.BucketName = *destBucket
	}
	cfg.DestMinio.UseSSL = *destUseSSL

	// Create sync service
	service, err := sync.NewService(cfg)
	if err != nil {
		log.Fatalf("Failed to create sync service: %v", err)
	}
	defer service.Close()

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interruption signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nReceived interrupt signal. Cleaning up...")
		cancel()
	}()

	// Helper function to format size
	func formatSize(size int64) string {
		const unit = 1024
		if size < unit {
			return fmt.Sprintf("%d B", size)
		}
		div, exp := int64(unit), 0
		for n := size / unit; n >= unit; n /= unit {
			div *= unit
			exp++
		}
		return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
	}

	// Helper function to print sync status
	func printStatus(status *sync.SyncStatus) {
		fmt.Println("\nSync Status:")
		fmt.Println("------------")
		
		var totalFiles, totalSize int64
		for _, count := range status.Counts {
			totalFiles += count.Count
			totalSize += count.Size
			fmt.Printf("%-10s: %5d files (%s)\n", 
				count.Status, 
				count.Count,
				formatSize(count.Size),
			)
		}
		
		fmt.Printf("\nTotal: %d files (%s)\n", totalFiles, formatSize(totalSize))

		if len(status.RecentErrors) > 0 {
			fmt.Println("\nRecent Errors:")
			fmt.Println("--------------")
			for _, err := range status.RecentErrors {
				fmt.Printf("File: %s\nError: %s\nTime: %s\n\n",
					err.Path,
					err.ErrorMessage,
					err.UpdatedAt.Format(time.RFC3339),
				)
			}
		}
	}

	// Execute command
	switch *command {
	case "update-list":
		fmt.Println("Updating source file list...")
		if err := service.UpdateSourceFileList(ctx); err != nil {
			log.Fatalf("Failed to update source file list: %v", err)
		}
		fmt.Println("Source file list updated successfully")

	case "sync":
		fmt.Printf("Starting sync with %d workers...\n", *workers)
		if err := service.SyncFiles(ctx, *workers); err != nil {
			log.Fatalf("Failed to sync files: %v", err)
		}
		fmt.Println("Sync completed successfully")

	case "status":
		status, err := service.GetStatus()
		if err != nil {
			log.Fatalf("Failed to get sync status: %v", err)
		}
		printStatus(status)

	default:
		log.Fatalf("Unknown command: %s", *command)
	}
}
