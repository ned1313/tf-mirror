package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalStorage_Upload(t *testing.T) {
	tempDir := t.TempDir()
	storage, err := NewLocalStorage(LocalConfig{BasePath: tempDir})
	require.NoError(t, err)
	defer storage.Close()

	ctx := context.Background()
	content := []byte("test content")
	reader := bytes.NewReader(content)

	err = storage.Upload(ctx, "test/file.txt", reader, "text/plain", map[string]string{"foo": "bar"})
	require.NoError(t, err)

	// Verify file was created
	filePath := filepath.Join(tempDir, "test", "file.txt")
	data, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Equal(t, content, data)

	// Verify metadata was created
	metadataPath := filePath + ".metadata"
	metadata, err := os.ReadFile(metadataPath)
	require.NoError(t, err)
	assert.Contains(t, string(metadata), "foo=bar")
}

func TestLocalStorage_Upload_EmptyKey(t *testing.T) {
	tempDir := t.TempDir()
	storage, err := NewLocalStorage(LocalConfig{BasePath: tempDir})
	require.NoError(t, err)
	defer storage.Close()

	ctx := context.Background()
	reader := bytes.NewReader([]byte("test"))

	err = storage.Upload(ctx, "", reader, "text/plain", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key cannot be empty")
}

func TestLocalStorage_Upload_DirectoryTraversal(t *testing.T) {
	tempDir := t.TempDir()
	storage, err := NewLocalStorage(LocalConfig{BasePath: tempDir})
	require.NoError(t, err)
	defer storage.Close()

	ctx := context.Background()
	reader := bytes.NewReader([]byte("test"))

	err = storage.Upload(ctx, "../../../etc/passwd", reader, "text/plain", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "directory traversal")
}

func TestLocalStorage_Download(t *testing.T) {
	tempDir := t.TempDir()
	storage, err := NewLocalStorage(LocalConfig{BasePath: tempDir})
	require.NoError(t, err)
	defer storage.Close()

	ctx := context.Background()
	content := []byte("test content")

	// Upload file first
	err = storage.Upload(ctx, "test/file.txt", bytes.NewReader(content), "text/plain", nil)
	require.NoError(t, err)

	// Download it
	reader, err := storage.Download(ctx, "test/file.txt")
	require.NoError(t, err)
	defer reader.Close()

	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, content, data)
}

func TestLocalStorage_Download_NotFound(t *testing.T) {
	tempDir := t.TempDir()
	storage, err := NewLocalStorage(LocalConfig{BasePath: tempDir})
	require.NoError(t, err)
	defer storage.Close()

	ctx := context.Background()
	_, err = storage.Download(ctx, "nonexistent.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestLocalStorage_Delete(t *testing.T) {
	tempDir := t.TempDir()
	storage, err := NewLocalStorage(LocalConfig{BasePath: tempDir})
	require.NoError(t, err)
	defer storage.Close()

	ctx := context.Background()

	// Upload file
	err = storage.Upload(ctx, "test/file.txt", bytes.NewReader([]byte("test")), "text/plain", map[string]string{"key": "value"})
	require.NoError(t, err)

	// Delete it
	err = storage.Delete(ctx, "test/file.txt")
	require.NoError(t, err)

	// Verify it's gone
	exists, err := storage.Exists(ctx, "test/file.txt")
	require.NoError(t, err)
	assert.False(t, exists)

	// Verify metadata is also gone
	metadataPath := filepath.Join(tempDir, "test", "file.txt.metadata")
	_, err = os.Stat(metadataPath)
	assert.True(t, os.IsNotExist(err))
}

func TestLocalStorage_Exists(t *testing.T) {
	tempDir := t.TempDir()
	storage, err := NewLocalStorage(LocalConfig{BasePath: tempDir})
	require.NoError(t, err)
	defer storage.Close()

	ctx := context.Background()

	// File doesn't exist yet
	exists, err := storage.Exists(ctx, "test/file.txt")
	require.NoError(t, err)
	assert.False(t, exists)

	// Upload file
	err = storage.Upload(ctx, "test/file.txt", bytes.NewReader([]byte("test")), "text/plain", nil)
	require.NoError(t, err)

	// Now it exists
	exists, err = storage.Exists(ctx, "test/file.txt")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestLocalStorage_GetPresignedURL(t *testing.T) {
	tempDir := t.TempDir()
	storage, err := NewLocalStorage(LocalConfig{BasePath: tempDir})
	require.NoError(t, err)
	defer storage.Close()

	ctx := context.Background()

	// Upload file
	err = storage.Upload(ctx, "test/file.txt", bytes.NewReader([]byte("test")), "text/plain", nil)
	require.NoError(t, err)

	// Get presigned URL
	url, err := storage.GetPresignedURL(ctx, "test/file.txt", 1*time.Hour)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(url, "file://"))
	assert.Contains(t, url, "test/file.txt")
}

func TestLocalStorage_GetMetadata(t *testing.T) {
	tempDir := t.TempDir()
	storage, err := NewLocalStorage(LocalConfig{BasePath: tempDir})
	require.NoError(t, err)
	defer storage.Close()

	ctx := context.Background()

	// Upload file with metadata
	metadata := map[string]string{
		"foo": "bar",
		"baz": "qux",
	}
	err = storage.Upload(ctx, "test/file.txt", bytes.NewReader([]byte("test")), "text/plain", metadata)
	require.NoError(t, err)

	// Get metadata
	retrieved, err := storage.GetMetadata(ctx, "test/file.txt")
	require.NoError(t, err)
	assert.Equal(t, "bar", retrieved["foo"])
	assert.Equal(t, "qux", retrieved["baz"])
}

func TestLocalStorage_GetMetadata_NoMetadata(t *testing.T) {
	tempDir := t.TempDir()
	storage, err := NewLocalStorage(LocalConfig{BasePath: tempDir})
	require.NoError(t, err)
	defer storage.Close()

	ctx := context.Background()

	// Upload file without metadata
	err = storage.Upload(ctx, "test/file.txt", bytes.NewReader([]byte("test")), "text/plain", nil)
	require.NoError(t, err)

	// Get metadata (should be empty)
	retrieved, err := storage.GetMetadata(ctx, "test/file.txt")
	require.NoError(t, err)
	assert.Empty(t, retrieved)
}

func TestLocalStorage_ListObjects(t *testing.T) {
	tempDir := t.TempDir()
	storage, err := NewLocalStorage(LocalConfig{BasePath: tempDir})
	require.NoError(t, err)
	defer storage.Close()

	ctx := context.Background()

	// Upload multiple files
	files := []string{
		"providers/hashicorp/aws/1.0.0/file1.zip",
		"providers/hashicorp/aws/2.0.0/file2.zip",
		"providers/hashicorp/random/1.0.0/file3.zip",
	}

	for _, file := range files {
		err = storage.Upload(ctx, file, bytes.NewReader([]byte("test")), "application/zip", nil)
		require.NoError(t, err)
	}

	// List all providers
	keys, err := storage.ListObjects(ctx, "providers")
	require.NoError(t, err)
	assert.Len(t, keys, 3)

	// List only AWS providers
	keys, err = storage.ListObjects(ctx, "providers/hashicorp/aws")
	require.NoError(t, err)
	assert.Len(t, keys, 2)
	for _, key := range keys {
		assert.Contains(t, key, "hashicorp/aws")
	}
}

func TestLocalStorage_ListObjects_EmptyPrefix(t *testing.T) {
	tempDir := t.TempDir()
	storage, err := NewLocalStorage(LocalConfig{BasePath: tempDir})
	require.NoError(t, err)
	defer storage.Close()

	ctx := context.Background()

	// Upload a file
	err = storage.Upload(ctx, "test.txt", bytes.NewReader([]byte("test")), "text/plain", nil)
	require.NoError(t, err)

	// List with empty prefix (should list all)
	keys, err := storage.ListObjects(ctx, "")
	require.NoError(t, err)
	assert.Len(t, keys, 1)
	assert.Contains(t, keys, "test.txt")
}

func TestLocalStorage_GetObjectSize(t *testing.T) {
	tempDir := t.TempDir()
	storage, err := NewLocalStorage(LocalConfig{BasePath: tempDir})
	require.NoError(t, err)
	defer storage.Close()

	ctx := context.Background()

	content := []byte("test content with some length")
	err = storage.Upload(ctx, "test/file.txt", bytes.NewReader(content), "text/plain", nil)
	require.NoError(t, err)

	size, err := storage.GetObjectSize(ctx, "test/file.txt")
	require.NoError(t, err)
	assert.Equal(t, int64(len(content)), size)
}

func TestNewLocalStorage_EmptyBasePath(t *testing.T) {
	_, err := NewLocalStorage(LocalConfig{BasePath: ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "base path is required")
}

func TestNewLocalStorage_CreatesDirectory(t *testing.T) {
	tempDir := t.TempDir()
	newDir := filepath.Join(tempDir, "new", "storage", "path")

	storage, err := NewLocalStorage(LocalConfig{BasePath: newDir})
	require.NoError(t, err)
	defer storage.Close()

	// Verify directory was created
	info, err := os.Stat(newDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}
