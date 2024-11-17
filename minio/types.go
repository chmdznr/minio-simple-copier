package minio

import "time"

// MCListEntry represents a single entry from `mc ls --json` output
type MCListEntry struct {
	Status        string    `json:"status"`
	Type          string    `json:"type"`
	LastModified  time.Time `json:"lastModified"`
	Size          int64     `json:"size"`
	Key           string    `json:"key"`
	ETag          string    `json:"etag"`
	URL           string    `json:"url"`
	VersionOrdinal int      `json:"versionOrdinal"`
	StorageClass  string    `json:"storageClass"`
}
