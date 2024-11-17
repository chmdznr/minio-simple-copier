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

## Installation

### Option 1: Using go get
```bash
go get github.com/chmdznr/minio-simple-copier
```

### Option 2: Building from Source

1. Clone the repository:
```bash
git clone https://github.com/chmdznr/minio-simple-copier.git
cd minio-simple-copier
```

2. Install dependencies:
```bash
go mod download
```

3. Build the binary:
```bash
# For Windows
go build -o minio-simple-copier.exe

# For Linux/Mac
go build -o minio-simple-copier
```

4. (Optional) Install to your system:
```bash
# Install to $GOPATH/bin
go install

# Or copy the binary to a location in your PATH
# Windows example:
copy minio-simple-copier.exe %USERPROFILE%\go\bin\

# Linux/Mac example:
cp minio-simple-copier ~/go/bin/
```

The compiled binary will be available in the current directory. Make sure it's in your system's PATH to run it from anywhere.

### Requirements

- Go 1.23 or later
- SQLite3
- Git (for cloning the repository)

## Usage

The tool provides four main commands:

1. `config`: Save Minio connection details and destination settings for a project
2. `update-list`: Scan the source Minio bucket and update the local SQLite database with file information
3. `sync`: Copy files from source to destination (either Minio bucket or local folder)
4. `status`: Show current synchronization status, including file counts, sizes, and recent errors

### Basic Usage

First, save your configuration. You can choose between two destination types:

#### Option 1: Minio-to-Minio Sync
```bash
# Save project configuration for Minio-to-Minio sync
minio-simple-copier -project myproject \
    -command config \
    -dest-type minio \
    -source-endpoint source-minio:9000 \
    -source-access-key YOUR_ACCESS_KEY \
    -source-secret-key YOUR_SECRET_KEY \
    -source-bucket source-bucket \
    -dest-endpoint dest-minio:9000 \
    -dest-access-key YOUR_ACCESS_KEY \
    -dest-secret-key YOUR_SECRET_KEY \
    -dest-bucket dest-bucket
```

#### Option 2: Minio-to-Local Sync
```bash
# Save project configuration for Minio-to-Local sync
minio-simple-copier -project myproject \
    -command config \
    -dest-type local \
    -source-endpoint source-minio:9000 \
    -source-access-key YOUR_ACCESS_KEY \
    -source-secret-key YOUR_SECRET_KEY \
    -source-bucket source-bucket \
    -local-path "D:/backup/minio-files"
```

Then you can run commands without specifying connection details each time:
```bash
# Update file list
minio-simple-copier -project myproject -command update-list

# Start synchronization
minio-simple-copier -project myproject -command sync -workers 10

# Check status
minio-simple-copier -project myproject -command status
```

You can also override saved configuration values by providing them in the command line:
```bash
# Use different source bucket and local path for this run
minio-simple-copier -project myproject \
    -command sync \
    -source-bucket different-bucket \
    -local-path "E:/different-backup"
```

### Configuration File

The tool stores connection details in `projects/config.yaml`:

```yaml
projects:
  myproject-minio:
    source:
      endpoint: source-minio:9000
      accessKeyID: YOUR_ACCESS_KEY
      secretAccessKey: YOUR_SECRET_KEY
      useSSL: true
      bucketName: source-bucket
    destType: minio
    dest:
      endpoint: dest-minio:9000
      accessKeyID: YOUR_ACCESS_KEY
      secretAccessKey: YOUR_SECRET_KEY
      useSSL: true
      bucketName: dest-bucket
  myproject-local:
    source:
      endpoint: source-minio:9000
      accessKeyID: YOUR_ACCESS_KEY
      secretAccessKey: YOUR_SECRET_KEY
      useSSL: true
      bucketName: source-bucket
    destType: local
    local:
      path: "D:/backup/minio-files"
```

### Command Line Options

```
  -project string
        Project name (required)
  -command string
        Command to execute (config, update-list, sync, status)
  -workers int
        Number of concurrent workers (default 5)

  Source Minio Configuration (optional after config):
  -source-endpoint string
        Source Minio endpoint
  -source-access-key string
        Source Minio access key
  -source-secret-key string
        Source Minio secret key
  -source-bucket string
        Source Minio bucket
  -source-use-ssl
        Use SSL for source Minio (default true)

  Destination Configuration:
  -dest-type string
        Destination type (minio or local) (default "minio")
  -local-path string
        Local destination path (when dest-type is local)

  Destination Minio Configuration (when dest-type is minio):
  -dest-endpoint string
        Destination Minio endpoint
  -dest-access-key string
        Destination Minio access key
  -dest-secret-key string
        Destination Minio secret key
  -dest-bucket string
        Destination Minio bucket
  -dest-use-ssl
        Use SSL for destination Minio (default true)
```

## Project Structure

The tool creates a project directory under `./projects/<project-name>/` containing:
- `files.db`: SQLite database storing file information and sync status
- `config.yaml`: Configuration file storing Minio connection details

## File States

Files in the database can have the following states:
- `pending`: File needs to be synced
- `copying`: File is currently being copied
- `completed`: File has been successfully copied
- `exists`: File already exists in destination
- `error`: Error occurred during sync

## Performance Considerations

- Uses SQLite for efficient file tracking
- Implements concurrent file transfers
- Supports multipart upload for large files
- Uses ETag comparison to detect changes
- Batch processing of file operations

## License

MIT License
