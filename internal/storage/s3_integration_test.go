//go:build integration
// +build integration

package storage

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests require MinIO to be running.
// Run: docker-compose -f docker-compose.test.yml up -d
// Then: go test -v -tags=integration ./internal/storage/...

func createMinIOStorage(t *testing.T) *S3Storage {
	ctx := context.Background()

	storage, err := NewS3Storage(ctx, S3Config{
		Region:         "us-east-1",
		Bucket:         "test-bucket",
		Endpoint:       "http://localhost:9000",
		AccessKey:      "minioadmin",
		SecretKey:      "minioadmin",
		ForcePathStyle: true,
	})
	require.NoError(t, err)

	return storage
}

func ensureBucketExists(t *testing.T, storage *S3Storage) {
	ctx := context.Background()

	// Try to create the bucket (will fail if it exists, which is fine)
	_, err := storage.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(storage.bucket),
	})

	// Ignore "bucket already exists" errors
	if err != nil {
		t.Logf("Bucket creation note: %v", err)
	}
}

func TestS3Integration_Upload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	storage := createMinIOStorage(t)
	defer storage.Close()
	ensureBucketExists(t, storage)

	ctx := context.Background()
	content := []byte("test content for S3")
	reader := bytes.NewReader(content)

	err := storage.Upload(ctx, "test/file.txt", reader, "text/plain", map[string]string{
		"test-key": "test-value",
	})
	require.NoError(t, err)
}

func TestS3Integration_Download(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	storage := createMinIOStorage(t)
	defer storage.Close()
	ensureBucketExists(t, storage)

	ctx := context.Background()
	content := []byte("test content for download")

	// Upload first
	err := storage.Upload(ctx, "test/download.txt", bytes.NewReader(content), "text/plain", nil)
	require.NoError(t, err)

	// Download
	reader, err := storage.Download(ctx, "test/download.txt")
	require.NoError(t, err)
	defer reader.Close()

	downloaded, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, content, downloaded)
}

func TestS3Integration_Exists(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	storage := createMinIOStorage(t)
	defer storage.Close()
	ensureBucketExists(t, storage)

	ctx := context.Background()
	testKey := "test/exists-test-" + time.Now().Format("20060102150405") + ".txt"

	// File doesn't exist yet
	exists, err := storage.Exists(ctx, testKey)
	require.NoError(t, err)
	assert.False(t, exists)

	// Upload file
	err = storage.Upload(ctx, testKey, bytes.NewReader([]byte("test")), "text/plain", nil)
	require.NoError(t, err)

	// Now it exists
	exists, err = storage.Exists(ctx, testKey)
	require.NoError(t, err)
	assert.True(t, exists)

	// Clean up
	_ = storage.Delete(ctx, testKey)
}

func TestS3Integration_Delete(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	storage := createMinIOStorage(t)
	defer storage.Close()
	ensureBucketExists(t, storage)

	ctx := context.Background()
	testKey := "test/delete-test-" + time.Now().Format("20060102150405") + ".txt"

	// Upload file
	err := storage.Upload(ctx, testKey, bytes.NewReader([]byte("test")), "text/plain", nil)
	require.NoError(t, err)

	// Verify it exists
	exists, err := storage.Exists(ctx, testKey)
	require.NoError(t, err)
	assert.True(t, exists)

	// Delete it
	err = storage.Delete(ctx, testKey)
	require.NoError(t, err)

	// Verify it's gone
	exists, err = storage.Exists(ctx, testKey)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestS3Integration_GetPresignedURL(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	storage := createMinIOStorage(t)
	defer storage.Close()
	ensureBucketExists(t, storage)

	ctx := context.Background()

	// Upload file
	content := []byte("presigned url test")
	err := storage.Upload(ctx, "test/presigned.txt", bytes.NewReader(content), "text/plain", nil)
	require.NoError(t, err)

	// Get presigned URL
	url, err := storage.GetPresignedURL(ctx, "test/presigned.txt", 1*time.Hour)
	require.NoError(t, err)
	assert.NotEmpty(t, url)
	assert.Contains(t, url, "localhost:9000")
	assert.Contains(t, url, "test/presigned.txt")
	assert.Contains(t, url, "X-Amz-Signature")
}

func TestS3Integration_GetMetadata(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	storage := createMinIOStorage(t)
	defer storage.Close()
	ensureBucketExists(t, storage)

	ctx := context.Background()

	// Upload file with metadata
	metadata := map[string]string{
		"version":  "1.0.0",
		"provider": "aws",
	}
	err := storage.Upload(ctx, "test/metadata.txt", bytes.NewReader([]byte("test")), "text/plain", metadata)
	require.NoError(t, err)

	// Get metadata
	retrieved, err := storage.GetMetadata(ctx, "test/metadata.txt")
	require.NoError(t, err)
	assert.Equal(t, "1.0.0", retrieved["version"])
	assert.Equal(t, "aws", retrieved["provider"])
}

func TestS3Integration_ListObjects(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	storage := createMinIOStorage(t)
	defer storage.Close()
	ensureBucketExists(t, storage)

	ctx := context.Background()

	// Upload multiple files
	files := []string{
		"providers/hashicorp/aws/1.0.0/file1.zip",
		"providers/hashicorp/aws/2.0.0/file2.zip",
		"providers/hashicorp/random/1.0.0/file3.zip",
	}

	for _, file := range files {
		err := storage.Upload(ctx, file, bytes.NewReader([]byte("test")), "application/zip", nil)
		require.NoError(t, err)
	}

	// List all providers
	keys, err := storage.ListObjects(ctx, "providers/")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(keys), 3)

	// List only AWS providers
	keys, err = storage.ListObjects(ctx, "providers/hashicorp/aws/")
	require.NoError(t, err)

	// Filter to only keys that match our test
	awsKeys := []string{}
	for _, key := range keys {
		if len(key) > 0 && (key == "providers/hashicorp/aws/1.0.0/file1.zip" || key == "providers/hashicorp/aws/2.0.0/file2.zip") {
			awsKeys = append(awsKeys, key)
		}
	}
	assert.GreaterOrEqual(t, len(awsKeys), 2)
}

func TestS3Integration_GetObjectSize(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	storage := createMinIOStorage(t)
	defer storage.Close()
	ensureBucketExists(t, storage)

	ctx := context.Background()

	content := []byte("test content with specific length")
	err := storage.Upload(ctx, "test/size-test.txt", bytes.NewReader(content), "text/plain", nil)
	require.NoError(t, err)

	size, err := storage.GetObjectSize(ctx, "test/size-test.txt")
	require.NoError(t, err)
	assert.Equal(t, int64(len(content)), size)
}

func TestS3Integration_UploadEmptyKey(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	storage := createMinIOStorage(t)
	defer storage.Close()

	ctx := context.Background()
	err := storage.Upload(ctx, "", bytes.NewReader([]byte("test")), "text/plain", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key cannot be empty")
}

func TestS3Integration_DownloadNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	storage := createMinIOStorage(t)
	defer storage.Close()
	ensureBucketExists(t, storage)

	ctx := context.Background()
	_, err := storage.Download(ctx, "nonexistent/file.txt")
	assert.Error(t, err)
}
