package storage

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockStorage_Upload(t *testing.T) {
	storage := NewMockStorage()
	ctx := context.Background()

	content := []byte("test content")
	err := storage.Upload(ctx, "test/file.txt", bytes.NewReader(content), "text/plain", nil)
	require.NoError(t, err)

	// Verify file was stored
	data, ok := storage.GetData("test/file.txt")
	assert.True(t, ok)
	assert.Equal(t, content, data)
}

func TestMockStorage_Download(t *testing.T) {
	storage := NewMockStorage()
	ctx := context.Background()

	content := []byte("test content")
	storage.SetData("test/file.txt", content)

	reader, err := storage.Download(ctx, "test/file.txt")
	require.NoError(t, err)
	defer reader.Close()

	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, content, data)
}

func TestMockStorage_Download_NotFound(t *testing.T) {
	storage := NewMockStorage()
	ctx := context.Background()

	_, err := storage.Download(ctx, "nonexistent.txt")
	assert.Error(t, err)
}

func TestMockStorage_Delete(t *testing.T) {
	storage := NewMockStorage()
	ctx := context.Background()

	storage.SetData("test/file.txt", []byte("test"))

	err := storage.Delete(ctx, "test/file.txt")
	require.NoError(t, err)

	exists, _ := storage.Exists(ctx, "test/file.txt")
	assert.False(t, exists)
}

func TestMockStorage_Exists(t *testing.T) {
	storage := NewMockStorage()
	ctx := context.Background()

	// File doesn't exist
	exists, err := storage.Exists(ctx, "test/file.txt")
	require.NoError(t, err)
	assert.False(t, exists)

	// Set data
	storage.SetData("test/file.txt", []byte("test"))

	// Now it exists
	exists, err = storage.Exists(ctx, "test/file.txt")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestMockStorage_GetPresignedURL(t *testing.T) {
	storage := NewMockStorage()
	ctx := context.Background()

	url, err := storage.GetPresignedURL(ctx, "test/file.txt", time.Hour)
	require.NoError(t, err)
	assert.Contains(t, url, "test/file.txt")
}

func TestMockStorage_GetMetadata(t *testing.T) {
	storage := NewMockStorage()
	ctx := context.Background()

	storage.SetData("test/file.txt", []byte("test"))

	metadata, err := storage.GetMetadata(ctx, "test/file.txt")
	require.NoError(t, err)
	assert.NotNil(t, metadata)
}

func TestMockStorage_ListObjects(t *testing.T) {
	storage := NewMockStorage()
	ctx := context.Background()

	storage.SetData("prefix/file1.txt", []byte("test1"))
	storage.SetData("prefix/file2.txt", []byte("test2"))
	storage.SetData("other/file3.txt", []byte("test3"))

	keys, err := storage.ListObjects(ctx, "prefix")
	require.NoError(t, err)
	assert.Len(t, keys, 2)
}

func TestMockStorage_GetObjectSize(t *testing.T) {
	storage := NewMockStorage()
	ctx := context.Background()

	content := []byte("test content")
	storage.SetData("test/file.txt", content)

	size, err := storage.GetObjectSize(ctx, "test/file.txt")
	require.NoError(t, err)
	assert.Equal(t, int64(len(content)), size)
}

func TestMockStorage_GetObjectSize_NotFound(t *testing.T) {
	storage := NewMockStorage()
	ctx := context.Background()

	size, err := storage.GetObjectSize(ctx, "nonexistent.txt")
	require.NoError(t, err)
	assert.Equal(t, int64(0), size)
}

func TestMockStorage_Close(t *testing.T) {
	storage := NewMockStorage()
	err := storage.Close()
	require.NoError(t, err)
}

func TestMockReadCloser_Read(t *testing.T) {
	storage := NewMockStorage()
	storage.SetData("test.txt", []byte("hello world"))

	ctx := context.Background()
	reader, err := storage.Download(ctx, "test.txt")
	require.NoError(t, err)

	buf := make([]byte, 5)
	n, err := reader.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, "hello", string(buf))

	// Read more
	n, err = reader.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, " worl", string(buf))

	// Read remaining
	n, err = reader.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	// EOF
	_, err = reader.Read(buf)
	assert.Equal(t, io.EOF, err)

	reader.Close()
}
