package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
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
	fmt.Println(`Minio Simple Copier - A high-performance file synchronization tool

Usage:
  minio-simple-copier -command <command> [options]

Commands:
  help          Show this help message
  config        Save configuration for a project
  update-list   Update source file list
  sync          Start file synchronization
  status        Show current sync status

Examples:
  1. Configure Minio-to-Minio sync:
     minio-simple-copier -project backup -command config \
       -source-endpoint source:9000 -source-bucket bucket1 \
       -dest-type minio -dest-endpoint dest:9000 -dest-bucket bucket2

  2. Configure Minio-to-Local sync:
     minio-simple-copier -project local-backup -command config \
       -source-endpoint minio:9000 -source-bucket mybucket \
       -dest-type local -local-path "/data/backup"

  3. Configure folder-specific sync:
     minio-simple-copier -project folder-backup -command config \
       -source-endpoint minio:9000 -source-bucket mybucket \
       -source-folder "documents/2024" \
       -dest-type local -local-path "/data/backup/2024-docs"

  4. Update file list:
     minio-simple-copier -project myproject -command update-list

  5. Start sync with 10 workers:
     minio-simple-copier -project myproject -command sync -workers 10

  6. Check sync status:
     minio-simple-copier -project myproject -command status

For more information, visit: https://github.com/chmdznr/minio-simple-copier`)
}

func main() {
	// Command line flags
	var (
		projectName     = flag.String("project", "", "Project name")
		sourceEndpoint  = flag.String("source-endpoint", "", "Source Minio endpoint")
		sourceAccessKey = flag.String("source-access-key", "", "Source Minio access key")
		sourceSecretKey = flag.String("source-secret-key", "", "Source Minio secret key")
		sourceBucket    = flag.String("source-bucket", "", "Source Minio bucket")
		sourceFolder    = flag.String("source-folder", "", "Source folder path (e.g., naskah-keluar)")

		destType      = flag.String("dest-type", "minio", "Destination type (minio or local)")
		localDestPath = flag.String("local-path", "", "Local destination path (when dest-type is local)")

		destEndpoint  = flag.String("dest-endpoint", "", "Destination Minio endpoint (when dest-type is minio)")
		destAccessKey = flag.String("dest-access-key", "", "Destination Minio access key (when dest-type is minio)")
		destSecretKey = flag.String("dest-secret-key", "", "Destination Minio secret key (when dest-type is minio)")
		destBucket    = flag.String("dest-bucket", "", "Destination Minio bucket (when dest-type is minio)")
		destUseSSL    = flag.Bool("dest-use-ssl", true, "Use SSL for destination Minio (when dest-type is minio)")
		destFolder    = flag.String("dest-folder", "", "Destination folder path (when dest-type is minio)")

		workers = flag.Int("workers", 5, "Number of concurrent workers")
		command = flag.String("command", "", "Command to execute (help, config, update-list, sync, status)")
	)

	// Handle SSL flag separately
	var sourceUseSSL bool
	flag.BoolVar(&sourceUseSSL, "source-use-ssl", true, "Use SSL for source Minio")

	flag.Parse()

	// Debug: Print all arguments
	log.Println("Debug: Command line arguments:")
	for i, arg := range os.Args {
		log.Printf("Arg[%d]: %q", i, arg)
	}

	// Debug: Print all flags
	log.Println("Debug: Printing all flags:")
	flag.Visit(func(f *flag.Flag) {
		log.Printf("Flag: %s = %q", f.Name, f.Value.String())
	})

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
		// Determine destination type
		destTypeStr := strings.ToLower(*destType)
		log.Printf("Debug: Raw dest-type flag value: %q", *destType)
		log.Printf("Debug: Lowercased dest-type value: %q", destTypeStr)
		
		destTypeEnum := config.DestinationType(destTypeStr)
		log.Printf("Debug: destTypeEnum after conversion: %q", destTypeEnum)
		
		if destTypeEnum != config.DestinationMinio && destTypeEnum != config.DestinationLocal {
			log.Fatalf("Invalid destination type: %s. Must be 'minio' or 'local'", destTypeStr)
		}

		// Save new config
		minioConfig := config.ProjectConfig{
			ProjectName: *projectName,
			SourceMinio: config.MinioConfig{
				Endpoint:        *sourceEndpoint,
				AccessKeyID:     *sourceAccessKey,
				SecretAccessKey: *sourceSecretKey,
				UseSSL:         sourceUseSSL,
				BucketName:     *sourceBucket,
				FolderPath:     *sourceFolder,
			},
			DestType: destTypeEnum,
		}

		// Handle destination based on type
		log.Printf("Debug: handling destination for type: %q", destTypeEnum)
		switch destTypeEnum {
		case config.DestinationMinio:
			minioConfig.DestMinio = config.MinioConfig{
				Endpoint:        *destEndpoint,
				AccessKeyID:     *destAccessKey,
				SecretAccessKey: *destSecretKey,
				UseSSL:         *destUseSSL,
				BucketName:     *destBucket,
				FolderPath:     *destFolder,
			}
		case config.DestinationLocal:
			if *localDestPath == "" {
				log.Fatal("Local destination path is required when dest-type is local")
			}
			minioConfig.DestLocal = config.LocalConfig{
				Path: *localDestPath,
			}
			minioConfig.DestMinio = config.MinioConfig{} // Empty Minio config for local destination
		}

		log.Printf("Debug: final config before save: %+v", minioConfig)
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
	if *sourceFolder != "" {
		cfg.SourceMinio.FolderPath = *sourceFolder
	}
	// Only override UseSSL if the flag was explicitly set
	sourceUseSSLSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "source-use-ssl" {
			sourceUseSSLSet = true
		}
	})
	if sourceUseSSLSet {
		cfg.SourceMinio.UseSSL = sourceUseSSL
	}

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
		if *destFolder != "" {
			cfg.DestMinio.FolderPath = *destFolder
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
