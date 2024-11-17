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

	"github.com/chmdznr/minio-simple-copier/v2/config"
	"github.com/chmdznr/minio-simple-copier/v2/sync"
)

const projectsDir = "projects"

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

func printUsage() {
	fmt.Printf(`Minio Simple Copier - A tool for efficient file synchronization between Minio buckets or to local folders

Usage:
  %s [flags] -project <name> -command <command>

Available Commands:
  help        Show this help message
  config      Save Minio connection details and destination settings for a project
  update-list Scan source bucket and update local database with file information
  sync        Copy files from source to destination (Minio or local folder)
  status      Show current synchronization status and progress

Examples:
  # Configure Minio-to-Minio sync
  %s -project myproject -command config -dest-type minio \
      -source-endpoint source-minio:9000 -source-bucket source-bucket \
      -dest-endpoint dest-minio:9000 -dest-bucket dest-bucket

  # Configure Minio-to-Local sync
  %s -project mybackup -command config -dest-type local \
      -source-endpoint minio:9000 -source-bucket mybucket \
      -local-path "D:/backup/minio-files"

  # Run synchronization
  %s -project myproject -command sync -workers 10

  # Check status
  %s -project myproject -command status

Flags:
`, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])
	flag.PrintDefaults()
}

func main() {
	// Command line flags
	var (
		projectName     = flag.String("project", "", "Project name")
		sourceEndpoint  = flag.String("source-endpoint", "", "Source Minio endpoint")
		sourceAccessKey = flag.String("source-access-key", "", "Source Minio access key")
		sourceSecretKey = flag.String("source-secret-key", "", "Source Minio secret key")
		sourceBucket    = flag.String("source-bucket", "", "Source Minio bucket")
		sourceUseSSL    = flag.Bool("source-use-ssl", true, "Use SSL for source Minio")

		destType      = flag.String("dest-type", "minio", "Destination type (minio or local)")
		localDestPath = flag.String("local-path", "", "Local destination path (when dest-type is local)")

		destEndpoint  = flag.String("dest-endpoint", "", "Destination Minio endpoint (when dest-type is minio)")
		destAccessKey = flag.String("dest-access-key", "", "Destination Minio access key (when dest-type is minio)")
		destSecretKey = flag.String("dest-secret-key", "", "Destination Minio secret key (when dest-type is minio)")
		destBucket    = flag.String("dest-bucket", "", "Destination Minio bucket (when dest-type is minio)")
		destUseSSL    = flag.Bool("dest-use-ssl", true, "Use SSL for destination Minio (when dest-type is minio)")

		workers = flag.Int("workers", 5, "Number of concurrent workers")
		command = flag.String("command", "", "Command to execute (help, config, update-list, sync, status)")
	)

	flag.Parse()

	// Show help if no arguments or help command
	if len(os.Args) == 1 || (len(os.Args) == 2 && (os.Args[1] == "-h" || os.Args[1] == "--help")) || *command == "help" {
		printUsage()
		return
	}

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
		destTypeEnum := config.DestinationType(*destType)
		if destTypeEnum != config.DestinationMinio && destTypeEnum != config.DestinationLocal {
			log.Fatalf("Invalid destination type: %s. Must be 'minio' or 'local'", *destType)
		}

		// Save new config
		minioConfig := config.ProjectConfig{
			ProjectName: *projectName,
			SourceMinio: config.MinioConfig{
				Endpoint:        *sourceEndpoint,
				AccessKeyID:     *sourceAccessKey,
				SecretAccessKey: *sourceSecretKey,
				UseSSL:         *sourceUseSSL,
				BucketName:     *sourceBucket,
			},
			DestType: destTypeEnum,
		}

		switch destTypeEnum {
		case config.DestinationMinio:
			minioConfig.DestMinio = config.MinioConfig{
				Endpoint:        *destEndpoint,
				AccessKeyID:     *destAccessKey,
				SecretAccessKey: *destSecretKey,
				UseSSL:         *destUseSSL,
				BucketName:     *destBucket,
			}
		case config.DestinationLocal:
			if *localDestPath == "" {
				log.Fatal("Local destination path is required when dest-type is local")
			}
			minioConfig.DestLocal = config.LocalConfig{
				Path: *localDestPath,
			}
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
	cfg := projectConfig
	cfg.DatabasePath = filepath.Join(projectDir, "files.db")

	// Debug: Print config before overrides
	log.Printf("Config before overrides: UseSSL=%v", cfg.SourceMinio.UseSSL)

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
	// Only override UseSSL if the flag was explicitly set
	sourceUseSSLSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "source-use-ssl" {
			sourceUseSSLSet = true
		}
	})
	if sourceUseSSLSet {
		cfg.SourceMinio.UseSSL = *sourceUseSSL
	}

	// Debug: Print config after overrides
	log.Printf("Config after overrides: UseSSL=%v", cfg.SourceMinio.UseSSL)

	// Handle destination overrides based on type
	if *destType != "minio" && *destType != "local" {
		cfg.DestType = config.DestinationType(*destType)
	}

	switch cfg.DestType {
	case config.DestinationMinio:
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
		// Only override UseSSL if the flag was explicitly set
		destUseSSLSet := false
		flag.Visit(func(f *flag.Flag) {
			if f.Name == "dest-use-ssl" {
				destUseSSLSet = true
			}
		})
		if destUseSSLSet {
			cfg.DestMinio.UseSSL = *destUseSSL
		}
	case config.DestinationLocal:
		if *localDestPath != "" {
			cfg.DestLocal.Path = *localDestPath
		}
	}

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
