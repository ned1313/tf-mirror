package storage

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewS3Storage_EmptyBucket(t *testing.T) {
	ctx := context.Background()

	_, err := NewS3Storage(ctx, S3Config{
		Region: "us-east-1",
		Bucket: "",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "bucket name is required")
}

func TestNewS3Storage_WithAccessKeys(t *testing.T) {
	ctx := context.Background()

	// This test verifies that the client creation doesn't fail with access keys
	// We can't actually test S3 operations without real credentials or a mock server
	_, err := NewS3Storage(ctx, S3Config{
		Region:    "us-east-1",
		Bucket:    "test-bucket",
		AccessKey: "test-access-key",
		SecretKey: "test-secret-key",
	})

	// The client should be created successfully even with fake credentials
	// Actual S3 operations would fail, but construction should succeed
	assert.NoError(t, err)
}

func TestNewS3Storage_WithEndpoint(t *testing.T) {
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
	assert.NotNil(t, storage)
	assert.Equal(t, "test-bucket", storage.bucket)
	assert.Equal(t, "us-east-1", storage.region)
	assert.True(t, storage.forcePathStyle)
}

func TestNewS3Storage_IAMRole(t *testing.T) {
	ctx := context.Background()

	// Test creation with IAM role (no access keys)
	_, err := NewS3Storage(ctx, S3Config{
		Region: "us-east-1",
		Bucket: "test-bucket",
	})

	// Should not error during construction
	// IAM role resolution happens at runtime
	assert.NoError(t, err)
}

func TestS3Storage_Close(t *testing.T) {
	ctx := context.Background()

	storage, err := NewS3Storage(ctx, S3Config{
		Region:    "us-east-1",
		Bucket:    "test-bucket",
		AccessKey: "test",
		SecretKey: "test",
	})

	require.NoError(t, err)

	// Close should not error
	err = storage.Close()
	assert.NoError(t, err)
}

// Note: Full integration tests for S3 operations (Upload, Download, Delete, etc.)
// should be implemented in integration tests using MinIO or LocalStack.
// These would require:
// 1. Starting a MinIO container
// 2. Creating a test bucket
// 3. Running actual S3 operations
// 4. Cleaning up
//
// Example integration test structure:
// func TestS3Storage_Integration(t *testing.T) {
//     if testing.Short() {
//         t.Skip("Skipping integration test")
//     }
//
//     // Start MinIO container with testcontainers-go
//     // Create storage client
//     // Test Upload, Download, Delete, Exists, etc.
//     // Clean up
// }
