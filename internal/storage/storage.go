package storage

import (
	"context"
	"io"
	"time"
)

// Storage defines the interface for object storage operations
type Storage interface {
	// Upload uploads a file to storage
	// key: the object key/path in storage
	// reader: the content to upload
	// contentType: MIME type (e.g., "application/zip")
	// metadata: optional metadata tags
	Upload(ctx context.Context, key string, reader io.Reader, contentType string, metadata map[string]string) error

	// Download downloads a file from storage
	// key: the object key/path in storage
	// Returns a ReadCloser that must be closed by the caller
	Download(ctx context.Context, key string) (io.ReadCloser, error)

	// Delete removes a file from storage
	Delete(ctx context.Context, key string) error

	// Exists checks if a file exists in storage
	Exists(ctx context.Context, key string) (bool, error)

	// GetPresignedURL generates a presigned URL for downloading
	// key: the object key/path in storage
	// expiration: how long the URL should remain valid
	GetPresignedURL(ctx context.Context, key string, expiration time.Duration) (string, error)

	// GetMetadata retrieves metadata for an object
	GetMetadata(ctx context.Context, key string) (map[string]string, error)

	// ListObjects lists objects with a given prefix
	// prefix: the prefix to filter by
	// Returns a list of object keys
	ListObjects(ctx context.Context, prefix string) ([]string, error)

	// GetObjectSize returns the size of an object in bytes
	GetObjectSize(ctx context.Context, key string) (int64, error)

	// Close closes any open connections
	Close() error
}

// ObjectInfo contains metadata about a stored object
type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
	ContentType  string
	ETag         string
	Metadata     map[string]string
}
