# Minio Simple Copier

A high-performance CLI tool for synchronizing files between Minio buckets, optimized for large-scale operations with SQLite-based file tracking.

## Features

- Fast file listing and synchronization using SQLite database
- Project-based configuration for managing multiple sync scenarios
- Configuration file support for storing Minio connection details
- Concurrent file transfers with configurable worker count
- Resumable file transfers
- ETag-based file change detection
- Support for large files using multipart upload
- Graceful handling of interruptions
- Folder-specific copying support
- Automatic retry on network timeouts

## Installation

### Option 1: Using go install
```bash
# Make sure CGO is enabled
export CGO_ENABLED=1

# Install the tool
go install github.com/chmdznr/minio-simple-copier/v2@latest
```

### Option 2: Building from Source

1. Clone the repository:
```bash
git clone https://github.com/chmdznr/minio-simple-copier.git
cd minio-simple-copier
```

2. Install dependencies:
```bash
# On Debian/Ubuntu, install SQLite development files
sudo apt-get install gcc sqlite3 libsqlite3-dev

# On CentOS/RHEL
sudo yum install gcc sqlite-devel

# On macOS with Homebrew
brew install sqlite3

# On Windows with MSYS2
pacman -S mingw-w64-x86_64-gcc mingw-w64-x86_64-sqlite3

# Download Go dependencies
go mod download
```

3. Build the binary:
```bash
# Enable CGO and build
export CGO_ENABLED=1
go build -o minio-simple-copier

# For Windows
set CGO_ENABLED=1
go build -o minio-simple-copier.exe
```

4. (Optional) Install to your system:
```bash
# Install to $GOPATH/bin
CGO_ENABLED=1 go install

# Or copy the binary to a location in your PATH
# Windows example:
copy minio-simple-copier.exe %USERPROFILE%\go\bin\

# Linux/Mac example:
cp minio-simple-copier ~/go/bin/
```

### Requirements

- Go 1.23 or later
- SQLite3
- Git (for cloning the repository)

## Usage

The tool provides five main commands:

1. `help`: Display usage information and examples
2. `config`: Save Minio connection details and destination settings for a project
3. `update-list`: Scan the source Minio bucket and update the local SQLite database with file information
4. `sync`: Copy files from source to destination (either Minio bucket or local folder)
5. `status`: Show current synchronization status, including file counts, sizes, and recent errors

### Getting Started

To see all available options and examples:
```bash
# Show help message
minio-simple-copier -command help

# Or use -h/--help flag
minio-simple-copier -h
```

### Configuration Examples

#### 1. Minio-to-Minio Sync (Full Bucket)
```bash
minio-simple-copier -project backup -command config \
  -source-endpoint=source:9000 \
  -source-access-key=admin \
  -source-secret-key=password \
  -source-bucket=bucket1 \
  -source-use-ssl=false \
  -dest-type=minio \
  -dest-endpoint=dest:9000 \
  -dest-access-key=admin \
  -dest-secret-key=password \
  -dest-bucket=bucket2 \
  -dest-use-ssl=false
```

#### 2. Minio-to-Local Sync (Full Bucket)
```bash
minio-simple-copier -project local-backup -command config \
  -source-endpoint=minio:9000 \
  -source-access-key=admin \
  -source-secret-key=password \
  -source-bucket=mybucket \
  -source-use-ssl=false \
  -dest-type=local \
  -local-path=/data/backup
```

#### 3. Folder-Specific Sync
```bash
minio-simple-copier -project folder-backup -command config \
  -source-endpoint=minio:9000 \
  -source-access-key=admin \
  -source-secret-key=password \
  -source-bucket=mybucket \
  -source-folder=docs/2024 \
  -source-use-ssl=false \
  -dest-type=local \
  -local-path=/data/backup/2024-docs
```

### Using MinIO Client (mc) for File Lists

If you're experiencing issues with listing files directly from MinIO, you can use the MinIO Client (mc) to generate a file list and import it into the tool:

1. Configure mc:
```bash
mc alias set srikandi http://10.31.3.74:9000 admin SrikandiV2.2021s
```

2. Generate file list:
```bash
# List files and save paths to a file
mc ls --recursive --json srikandi/persuratan/naskah-keluar | jq -r .key > file_list.txt
```

3. Import the file list:
```bash
minio-simple-copier -project folder-backup -command import-list -import-list=file_list.txt
```

This is useful when:
- You have network connectivity issues
- The bucket contains a very large number of files
- You want to sync a specific subset of files

### Running Sync Operations

After configuring a project, you can run sync operations:

```bash
# 1. Update the file list
minio-simple-copier -project myproject -command update-list
```

```bash
# 2. Start synchronization with 10 concurrent workers
minio-simple-copier -project myproject -command sync -workers=10
```

```bash
# 3. Check sync status
minio-simple-copier -project myproject -command status
```

### SSL Configuration

By default, SSL settings are read from your config file. You can override them using flags:

```bash
# Explicitly enable SSL
minio-simple-copier -project myproject -command sync -source-use-ssl=true

# Explicitly disable SSL
minio-simple-copier -project myproject -command sync -source-use-ssl=false
```

### Error Handling

The tool includes automatic retry logic for network timeouts and connection errors:
- Maximum retries: 3
- Retry interval: 5 seconds
- Retryable errors: Network timeouts, connection errors, context deadline exceeded

## Configuration File

The tool stores configuration in `projects/config.yaml`. Example configuration:

```yaml
projects:
  backup:
    source:
      endpoint: source-minio:9000
      accesskeyid: admin
      secretaccesskey: secret123
      usessl: false
      bucketname: source-bucket
      folderpath: documents/2024  # Optional: sync specific folder
    destType: local
    local:
      path: /data/backup/2024-docs
```

## License

MIT License - see LICENSE file for details.
