package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
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
  import-list   Import file list from mc ls --recursive --json output

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

  7. Import file list:
     minio-simple-copier -project myproject -command import-list -import-list file_list.txt

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

		// New flag for importing file list
		importFile = flag.String("import-list", "", "Import file list from mc ls --recursive --json output")
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
		if destTypeStr == "" {
			destTypeStr = "minio" // Default to minio if not specified
		}
		log.Printf("Debug: dest-type flag value: %q", destTypeStr)

		destTypeEnum := config.DestinationType(destTypeStr)
		log.Printf("Debug: destTypeEnum after conversion: %q", destTypeEnum)

		if destTypeEnum != config.DestinationMinio && destTypeEnum != config.DestinationLocal {
			log.Fatalf("Invalid destination type: %s. Must be 'minio' or 'local'", destTypeStr)
		}

		// Save new config
		cfg := &config.ProjectConfig{
			ProjectName: *projectName,
			SourceMinio: config.MinioConfig{
				Endpoint:        *sourceEndpoint,
				AccessKeyID:     *sourceAccessKey,
				SecretAccessKey: *sourceSecretKey,
				UseSSL:          sourceUseSSL,
				BucketName:      *sourceBucket,
				FolderPath:      *sourceFolder,
			},
			DestType: destTypeEnum,
		}

		// Handle destination based on type
		switch destTypeEnum {
		case config.DestinationMinio:
			cfg.DestMinio = config.MinioConfig{
				Endpoint:        *destEndpoint,
				AccessKeyID:     *destAccessKey,
				SecretAccessKey: *destSecretKey,
				UseSSL:          *destUseSSL,
				BucketName:      *destBucket,
				FolderPath:      *destFolder,
			}
		case config.DestinationLocal:
			cfg.DestLocal = config.LocalConfig{
				Path: *localDestPath,
			}
		}

		fileConfig.SetProjectConfig(*projectName, *cfg)
		if err := config.SaveConfig(projectsDir, fileConfig); err != nil {
			log.Fatalf("Failed to save config: %v", err)
		}
		fmt.Printf("Configuration saved for project %s\n", *projectName)
		return
	}

	// Get project config
	cfg, err := fileConfig.GetProjectConfig(*projectName)
	if err != nil {
		log.Fatalf("Failed to get project config: %v", err)
	}

	// Set database path
	cfg.DatabasePath = filepath.Join(projectDir, "files.db")

	// Debug config
	log.Printf("Debug: Project config: %+v", cfg)
	log.Printf("Debug: Source Minio config: %+v", cfg.SourceMinio)
	if cfg.DestType == config.DestinationMinio {
		log.Printf("Debug: Destination Minio config: %+v", cfg.DestMinio)
	} else {
		log.Printf("Debug: Destination Local config: %+v", cfg.DestLocal)
	}

	// Execute command
	switch *command {
	case "update-list":
		fmt.Println("Updating source file list...")
		syncService, err := sync.NewService(cfg)
		if err != nil {
			log.Fatalf("Failed to create sync service: %v", err)
		}
		defer syncService.Close()

		if err := syncService.UpdateSourceList(context.Background()); err != nil {
			log.Fatalf("Failed to update source file list: %v", err)
		}
		fmt.Println("Source file list updated successfully")

	case "sync":
		fmt.Printf("Starting sync with %d workers...\n", *workers)
		syncService, err := sync.NewService(cfg)
		if err != nil {
			log.Fatalf("Failed to create sync service: %v", err)
		}
		defer syncService.Close()

		if err := syncService.StartSync(context.Background(), *workers); err != nil {
			log.Fatalf("Failed to sync files: %v", err)
		}

	case "status":
		syncService, err := sync.NewService(cfg)
		if err != nil {
			log.Fatalf("Failed to create sync service: %v", err)
		}
		defer syncService.Close()

		status, err := syncService.GetStatus()
		if err != nil {
			log.Fatalf("Failed to get sync status: %v", err)
		}
		printStatus(status)

	case "import-list":
		if *importFile == "" {
			log.Fatal("Import file path is required for import-list command")
		}

		// Get absolute path if relative
		importPath := *importFile
		if !filepath.IsAbs(importPath) {
			absPath, err := filepath.Abs(importPath)
			if err != nil {
				log.Fatalf("Failed to get absolute path: %v", err)
			}
			importPath = absPath
		}

		fmt.Printf("Importing file list from %s...\n", importPath)

		// Create sync service
		syncService, err := sync.NewService(cfg)
		if err != nil {
			log.Fatalf("Failed to create sync service: %v", err)
		}
		defer syncService.Close()

		// Import file list
		if err := syncService.ImportFileList(context.Background(), []string{importPath}); err != nil {
			log.Fatalf("Failed to import file list: %v", err)
		}

	default:
		log.Fatalf("Unknown command: %s", *command)
	}
}
