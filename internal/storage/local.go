package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LocalStorage implements Storage interface using local filesystem
// This is primarily for testing and development
type LocalStorage struct {
	basePath string
	baseURL  string // Base URL for generating download URLs (e.g., "http://localhost:8080")
}

// LocalConfig contains configuration for local filesystem storage
type LocalConfig struct {
	BasePath string // Base directory for storage
	BaseURL  string // Base URL for generating download URLs
}

// NewLocalStorage creates a new local filesystem storage
func NewLocalStorage(cfg LocalConfig) (*LocalStorage, error) {
	if cfg.BasePath == "" {
		return nil, fmt.Errorf("base path is required")
	}

	// Create base directory if it doesn't exist
	if err := os.MkdirAll(cfg.BasePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	return &LocalStorage{
		basePath: cfg.BasePath,
		baseURL:  cfg.BaseURL,
	}, nil
}

// Upload uploads a file to local storage
func (l *LocalStorage) Upload(ctx context.Context, key string, reader io.Reader, contentType string, metadata map[string]string) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	// Sanitize key to prevent directory traversal
	key = filepath.Clean(key)
	if strings.Contains(key, "..") {
		return fmt.Errorf("invalid key: contains directory traversal")
	}

	fullPath := filepath.Join(l.basePath, key)

	// Create parent directories
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create file
	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy content
	if _, err := io.Copy(file, reader); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Store metadata in a separate file (simplified approach)
	if len(metadata) > 0 {
		metadataPath := fullPath + ".metadata"
		metadataFile, err := os.Create(metadataPath)
		if err != nil {
			return fmt.Errorf("failed to create metadata file: %w", err)
		}
		defer metadataFile.Close()

		for key, value := range metadata {
			fmt.Fprintf(metadataFile, "%s=%s\n", key, value)
		}
	}

	return nil
}

// Download downloads a file from local storage
func (l *LocalStorage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	if key == "" {
		return nil, fmt.Errorf("key cannot be empty")
	}

	// Sanitize key
	key = filepath.Clean(key)
	if strings.Contains(key, "..") {
		return nil, fmt.Errorf("invalid key: contains directory traversal")
	}

	fullPath := filepath.Join(l.basePath, key)

	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("object not found: %s", key)
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return file, nil
}

// Delete removes a file from local storage
func (l *LocalStorage) Delete(ctx context.Context, key string) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	// Sanitize key
	key = filepath.Clean(key)
	if strings.Contains(key, "..") {
		return fmt.Errorf("invalid key: contains directory traversal")
	}

	fullPath := filepath.Join(l.basePath, key)

	// Delete main file
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	// Delete metadata file if it exists
	metadataPath := fullPath + ".metadata"
	os.Remove(metadataPath) // Ignore error

	return nil
}

// Exists checks if a file exists in local storage
func (l *LocalStorage) Exists(ctx context.Context, key string) (bool, error) {
	if key == "" {
		return false, fmt.Errorf("key cannot be empty")
	}

	// Sanitize key
	key = filepath.Clean(key)
	if strings.Contains(key, "..") {
		return false, fmt.Errorf("invalid key: contains directory traversal")
	}

	fullPath := filepath.Join(l.basePath, key)

	_, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check file existence: %w", err)
	}

	return true, nil
}

// GetPresignedURL generates a download URL for local storage
// Returns an HTTP URL via the /blobs/ endpoint if baseURL is configured,
// otherwise falls back to file:// URL (which only works for local testing)
func (l *LocalStorage) GetPresignedURL(ctx context.Context, key string, expiration time.Duration) (string, error) {
	if key == "" {
		return "", fmt.Errorf("key cannot be empty")
	}

	// Sanitize key
	key = filepath.Clean(key)
	if strings.Contains(key, "..") {
		return "", fmt.Errorf("invalid key: contains directory traversal")
	}

	fullPath := filepath.Join(l.basePath, key)

	// Check if file exists
	if _, err := os.Stat(fullPath); err != nil {
		return "", fmt.Errorf("file not found: %w", err)
	}

	// If baseURL is configured, return HTTP URL via blob endpoint
	if l.baseURL != "" {
		// Normalize key for URL (use forward slashes)
		urlKey := strings.ReplaceAll(key, "\\", "/")
		return fmt.Sprintf("%s/blobs/%s", strings.TrimSuffix(l.baseURL, "/"), urlKey), nil
	}

	// Fall back to file:// URL (only works for local testing)
	return "file://" + filepath.ToSlash(fullPath), nil
}

// GetMetadata retrieves metadata for an object
func (l *LocalStorage) GetMetadata(ctx context.Context, key string) (map[string]string, error) {
	if key == "" {
		return nil, fmt.Errorf("key cannot be empty")
	}

	// Sanitize key
	key = filepath.Clean(key)
	if strings.Contains(key, "..") {
		return nil, fmt.Errorf("invalid key: contains directory traversal")
	}

	fullPath := filepath.Join(l.basePath, key)
	metadataPath := fullPath + ".metadata"

	metadata := make(map[string]string)

	// Read metadata file if it exists
	content, err := os.ReadFile(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return metadata, nil // No metadata
		}
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	// Parse metadata (simple key=value format)
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			metadata[parts[0]] = parts[1]
		}
	}

	return metadata, nil
}

// ListObjects lists objects with a given prefix
func (l *LocalStorage) ListObjects(ctx context.Context, prefix string) ([]string, error) {
	// Sanitize prefix
	prefix = filepath.Clean(prefix)
	if strings.Contains(prefix, "..") {
		return nil, fmt.Errorf("invalid prefix: contains directory traversal")
	}

	searchPath := filepath.Join(l.basePath, prefix)
	var keys []string

	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Ignore errors for non-existent paths
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}

		// Skip directories and metadata files
		if info.IsDir() || strings.HasSuffix(path, ".metadata") {
			return nil
		}

		// Get relative path from base
		relPath, err := filepath.Rel(l.basePath, path)
		if err != nil {
			return err
		}

		// Convert to forward slashes for consistency
		keys = append(keys, filepath.ToSlash(relPath))
		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	return keys, nil
}

// GetObjectSize returns the size of an object in bytes
func (l *LocalStorage) GetObjectSize(ctx context.Context, key string) (int64, error) {
	if key == "" {
		return 0, fmt.Errorf("key cannot be empty")
	}

	// Sanitize key
	key = filepath.Clean(key)
	if strings.Contains(key, "..") {
		return 0, fmt.Errorf("invalid key: contains directory traversal")
	}

	fullPath := filepath.Join(l.basePath, key)

	info, err := os.Stat(fullPath)
	if err != nil {
		return 0, fmt.Errorf("failed to get file info: %w", err)
	}

	return info.Size(), nil
}

// Close closes any open connections (no-op for local storage)
func (l *LocalStorage) Close() error {
	return nil
}
