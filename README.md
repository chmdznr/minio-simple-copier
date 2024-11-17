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

```bash
go get github.com/chmdznr/minio-simple-copier
```

## Usage

The tool provides four main commands:

1. `config`: Save Minio connection details for a project
2. `update-list`: Scan the source Minio bucket and update the local SQLite database with file information
3. `sync`: Copy files from source to destination bucket based on the database
4. `status`: Show current synchronization status, including file counts, sizes, and recent errors

### Basic Usage

First, save your Minio connection details:
```bash
# Save project configuration
minio-simple-copier -project myproject \
    -command config \
    -source-endpoint source-minio:9000 \
    -source-access-key YOUR_ACCESS_KEY \
    -source-secret-key YOUR_SECRET_KEY \
    -source-bucket source-bucket \
    -dest-endpoint dest-minio:9000 \
    -dest-access-key YOUR_ACCESS_KEY \
    -dest-secret-key YOUR_SECRET_KEY \
    -dest-bucket dest-bucket
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
# Use different source bucket for this run
minio-simple-copier -project myproject \
    -command sync \
    -source-bucket different-bucket
```

### Configuration File

The tool stores connection details in `projects/config.yaml`:

```yaml
projects:
  myproject:
    source:
      endpoint: source-minio:9000
      accessKeyID: YOUR_ACCESS_KEY
      secretAccessKey: YOUR_SECRET_KEY
      useSSL: true
      bucketName: source-bucket
    dest:
      endpoint: dest-minio:9000
      accessKeyID: YOUR_ACCESS_KEY
      secretAccessKey: YOUR_SECRET_KEY
      useSSL: true
      bucketName: dest-bucket
```

### Status Output Example

```
Sync Status:
------------
completed : 1234 files (2.5 GB)
copying   :    5 files (100 MB)
error     :    2 files (50 MB)
exists    :  500 files (1.2 GB)
pending   :   50 files (200 MB)

Total: 1791 files (4.05 GB)

Recent Errors:
--------------
File: path/to/file1.dat
Error: connection timeout
Time: 2023-08-10T15:04:05Z

File: path/to/file2.dat
Error: insufficient permissions
Time: 2023-08-10T15:03:02Z
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

  Destination Minio Configuration (optional after config):
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

## Development

### Requirements

- Go 1.16 or later
- SQLite3

### Building from Source

```bash
git clone https://github.com/chmdznr/minio-simple-copier.git
cd minio-simple-copier
go build
```

## License

MIT License
