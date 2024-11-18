# Minio Simple Copier

A high-performance CLI tool for synchronizing files between Minio buckets or to local filesystem, optimized for large-scale operations with SQLite-based file tracking.

## Features

- Fast file listing and synchronization using SQLite database
- Project-based configuration for managing multiple sync scenarios
- Configuration file support for storing Minio connection details
- Concurrent file transfers with configurable worker count
- Resumable file transfers
- ETag-based file change detection
- Support for large files
- Graceful handling of interruptions
- Folder-specific copying support
- Automatic retry on network timeouts
- Support for importing file lists from MinIO Client (mc)

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

- Go 1.20 or later
- SQLite3
- Git (for cloning the repository)

## Usage

The tool provides six main commands:

1. `help`: Display usage information and examples
2. `config`: Save Minio connection details and destination settings for a project
3. `update-list`: Scan the source Minio bucket and update the local SQLite database with file information
4. `sync`: Copy files from source to destination (either Minio bucket or local folder)
5. `status`: Show current synchronization status, including file counts, sizes, and recent errors
6. `import-list`: Import file list from MinIO Client (mc) JSON output

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

### File List Management

You have two options for managing file lists:

#### Option 1: Direct MinIO Listing (`update-list`)

```bash
# Update file list directly from MinIO
minio-simple-copier -project myproject -command update-list
```

This command:

- Lists all objects in the configured bucket/folder
- Preserves full folder structure
- Updates file metadata (size, ETag, last modified)
- Skips unchanged files (same ETag)
- Updates files with different ETags

#### Option 2: MinIO Client Import (`import-list`)

If you prefer using MinIO Client (mc) or have connectivity issues, you can generate a file list and import it:

1. Generate file list using mc:

```bash
# List files and save JSON output
mc ls --recursive --json source/bucket/folder > file_list.txt
```

2. Import the file list:

```bash
minio-simple-copier -project myproject -command import-list -import-list=file_list.txt
```

This method:

- Reads file metadata from mc's JSON output
- Preserves full folder structure
- Skips existing files
- No need to query MinIO for file information
- Useful for large buckets or poor connectivity

Both methods maintain consistent file tracking in the SQLite database and support the same synchronization features.

### Running Sync Operations

After updating the file list (using either method), you can start synchronization:

```bash
# Start sync with 10 concurrent workers
minio-simple-copier -project myproject -command sync -workers=10
```

```bash
# Check sync status
minio-simple-copier -project myproject -command status
```

The status command shows:

- Total files and sizes
- Files by status (pending, completed, error)
- Recent errors with timestamps

### SSL Configuration

By default, SSL settings are read from your config file. You can override them using flags:

```bash
# Explicitly enable SSL
minio-simple-copier -project myproject -command sync -source-use-ssl=true
```

## Project Structure

- `config/`: Configuration handling
- `db/`: SQLite database operations
- `minio/`: MinIO client wrapper
- `local/`: Local filesystem operations
- `sync/`: Core synchronization logic

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.
