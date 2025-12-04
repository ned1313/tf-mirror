package storage

import (
	"context"
	"testing"

	"github.com/ned1313/terraform-mirror/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFromConfig_S3(t *testing.T) {
	ctx := context.Background()

	cfg := config.StorageConfig{
		Type:           "s3",
		Bucket:         "test-bucket",
		Region:         "us-east-1",
		AccessKey:      "test",
		SecretKey:      "test",
		ForcePathStyle: true,
	}

	storage, err := NewFromConfig(ctx, cfg)
	require.NoError(t, err)
	assert.NotNil(t, storage)

	// Verify it's an S3 storage
	s3Storage, ok := storage.(*S3Storage)
	assert.True(t, ok)
	assert.Equal(t, "test-bucket", s3Storage.bucket)
	assert.Equal(t, "us-east-1", s3Storage.region)
	assert.True(t, s3Storage.forcePathStyle)
}

func TestNewFromConfig_Local(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	cfg := config.StorageConfig{
		Type:     "local",
		Endpoint: tempDir, // Use endpoint as base path for local storage
	}

	storage, err := NewFromConfig(ctx, cfg)
	require.NoError(t, err)
	assert.NotNil(t, storage)

	// Verify it's a local storage
	localStorage, ok := storage.(*LocalStorage)
	assert.True(t, ok)
	assert.Equal(t, tempDir, localStorage.basePath)
}

func TestNewFromConfig_LocalDefaultPath(t *testing.T) {
	ctx := context.Background()

	cfg := config.StorageConfig{
		Type: "local",
		// No endpoint specified, should use default
	}

	storage, err := NewFromConfig(ctx, cfg)
	require.NoError(t, err)
	assert.NotNil(t, storage)

	// Verify it's a local storage with default path
	localStorage, ok := storage.(*LocalStorage)
	assert.True(t, ok)
	assert.Equal(t, "/var/lib/tf-mirror/storage", localStorage.basePath)
}

func TestNewFromConfig_UnsupportedType(t *testing.T) {
	ctx := context.Background()

	cfg := config.StorageConfig{
		Type: "azure",
	}

	_, err := NewFromConfig(ctx, cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported storage type")
	assert.Contains(t, err.Error(), "azure")
}

func TestBuildProviderKey(t *testing.T) {
	tests := []struct {
		name         string
		hostname     string
		namespace    string
		providerType string
		version      string
		os           string
		arch         string
		filename     string
		expected     string
	}{
		{
			name:         "AWS provider",
			hostname:     "registry.terraform.io",
			namespace:    "hashicorp",
			providerType: "aws",
			version:      "5.31.0",
			os:           "linux",
			arch:         "amd64",
			filename:     "terraform-provider-aws_v5.31.0_linux_amd64.zip",
			expected:     "providers/registry.terraform.io/hashicorp/aws/5.31.0/linux_amd64/terraform-provider-aws_v5.31.0_linux_amd64.zip",
		},
		{
			name:         "Random provider on Windows",
			hostname:     "registry.terraform.io",
			namespace:    "hashicorp",
			providerType: "random",
			version:      "3.6.0",
			os:           "windows",
			arch:         "amd64",
			filename:     "terraform-provider-random_v3.6.0_windows_amd64.zip",
			expected:     "providers/registry.terraform.io/hashicorp/random/3.6.0/windows_amd64/terraform-provider-random_v3.6.0_windows_amd64.zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildProviderKey(
				tt.hostname,
				tt.namespace,
				tt.providerType,
				tt.version,
				tt.os,
				tt.arch,
				tt.filename,
			)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildModuleKey(t *testing.T) {
	tests := []struct {
		name      string
		hostname  string
		namespace string
		modName   string
		provider  string
		version   string
		filename  string
		expected  string
	}{
		{
			name:      "VPC module",
			hostname:  "registry.terraform.io",
			namespace: "terraform-aws-modules",
			modName:   "vpc",
			provider:  "aws",
			version:   "5.1.0",
			filename:  "module.tar.gz",
			expected:  "modules/registry.terraform.io/terraform-aws-modules/vpc/aws/5.1.0/module.tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildModuleKey(
				tt.hostname,
				tt.namespace,
				tt.modName,
				tt.provider,
				tt.version,
				tt.filename,
			)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildBackupKey(t *testing.T) {
	timestamp := "2024-01-15T10-30-00Z"
	expected := "backups/2024-01-15T10-30-00Z.db"

	result := BuildBackupKey(timestamp)
	assert.Equal(t, expected, result)
}
