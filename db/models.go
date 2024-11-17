package db

import (
	"database/sql"
	"fmt"
	"time"
)

type FileStatus string

const (
	StatusPending   FileStatus = "pending"
	StatusExists    FileStatus = "exists"
	StatusNotFound  FileStatus = "not_found"
	StatusCopying   FileStatus = "copying"
	StatusCompleted FileStatus = "completed"
	StatusError     FileStatus = "error"
)

type FileEntry struct {
	ID           int64
	ProjectName  string
	Path         string
	Size         int64
	ETag         string
	LastModified time.Time
	Status       FileStatus
	ErrorMessage string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type StatusCount struct {
	Status FileStatus
	Count  int64
	Size   int64
}

type Database struct {
	db *sql.DB
}

func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &Database{db: db}, nil
}

func (d *Database) Initialize() error {
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS file_entries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		project_name TEXT NOT NULL,
		path TEXT NOT NULL,
		size INTEGER NOT NULL,
		etag TEXT NOT NULL,
		last_modified DATETIME NOT NULL,
		status TEXT NOT NULL,
		error_message TEXT,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_project_path ON file_entries(project_name, path);
	CREATE INDEX IF NOT EXISTS idx_status ON file_entries(status);
	`

	_, err := d.db.Exec(createTableSQL)
	return err
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) InsertFileEntry(entry *FileEntry) error {
	query := `
	INSERT INTO file_entries (
		project_name, path, size, etag, last_modified, status, error_message, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	now := time.Now()
	entry.CreatedAt = now
	entry.UpdatedAt = now

	result, err := d.db.Exec(query,
		entry.ProjectName,
		entry.Path,
		entry.Size,
		entry.ETag,
		entry.LastModified,
		entry.Status,
		entry.ErrorMessage,
		entry.CreatedAt,
		entry.UpdatedAt,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	entry.ID = id
	return nil
}

func (d *Database) UpdateFileStatus(id int64, status FileStatus, errorMessage string) error {
	query := `
	UPDATE file_entries 
	SET status = ?, error_message = ?, updated_at = ?
	WHERE id = ?`

	_, err := d.db.Exec(query, status, errorMessage, time.Now(), id)
	return err
}

func (d *Database) GetPendingFiles(projectName string, limit int) ([]*FileEntry, error) {
	query := `
	SELECT id, project_name, path, size, etag, last_modified, status, error_message, created_at, updated_at
	FROM file_entries
	WHERE project_name = ? AND status IN (?, ?)
	ORDER BY created_at ASC
	LIMIT ?`

	rows, err := d.db.Query(query, projectName, StatusPending, StatusError, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*FileEntry
	for rows.Next() {
		entry := &FileEntry{}
		err := rows.Scan(
			&entry.ID,
			&entry.ProjectName,
			&entry.Path,
			&entry.Size,
			&entry.ETag,
			&entry.LastModified,
			&entry.Status,
			&entry.ErrorMessage,
			&entry.CreatedAt,
			&entry.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func (d *Database) GetFileByPath(projectName, path string) (*FileEntry, error) {
	query := `
	SELECT id, project_name, path, size, etag, last_modified, status, error_message, created_at, updated_at
	FROM file_entries
	WHERE project_name = ? AND path = ?
	LIMIT 1`

	entry := &FileEntry{}
	err := d.db.QueryRow(query, projectName, path).Scan(
		&entry.ID,
		&entry.ProjectName,
		&entry.Path,
		&entry.Size,
		&entry.ETag,
		&entry.LastModified,
		&entry.Status,
		&entry.ErrorMessage,
		&entry.CreatedAt,
		&entry.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return entry, nil
}

func (d *Database) GetStatusCounts(projectName string) ([]StatusCount, error) {
	query := `
	SELECT status, COUNT(*) as count, SUM(size) as total_size
	FROM file_entries
	WHERE project_name = ?
	GROUP BY status
	ORDER BY status`

	rows, err := d.db.Query(query, projectName)
	if err != nil {
		return nil, fmt.Errorf("failed to get status counts: %w", err)
	}
	defer rows.Close()

	var counts []StatusCount
	for rows.Next() {
		var count StatusCount
		var status string
		if err := rows.Scan(&status, &count.Count, &count.Size); err != nil {
			return nil, fmt.Errorf("failed to scan status count: %w", err)
		}
		count.Status = FileStatus(status)
		counts = append(counts, count)
	}

	return counts, nil
}

func (d *Database) GetRecentErrors(projectName string, limit int) ([]*FileEntry, error) {
	query := `
	SELECT id, project_name, path, size, etag, last_modified, status, error_message, created_at, updated_at
	FROM file_entries
	WHERE project_name = ? AND status = ? AND error_message IS NOT NULL
	ORDER BY updated_at DESC
	LIMIT ?`

	rows, err := d.db.Query(query, projectName, StatusError, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent errors: %w", err)
	}
	defer rows.Close()

	var entries []*FileEntry
	for rows.Next() {
		entry := &FileEntry{}
		err := rows.Scan(
			&entry.ID,
			&entry.ProjectName,
			&entry.Path,
			&entry.Size,
			&entry.ETag,
			&entry.LastModified,
			&entry.Status,
			&entry.ErrorMessage,
			&entry.CreatedAt,
			&entry.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan error entry: %w", err)
		}
		entries = append(entries, entry)
	}

	return entries, nil
}
