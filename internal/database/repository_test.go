package database

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *DB {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := New(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	return db
}

func TestProviderRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	repo := NewProviderRepository(db)
	ctx := context.Background()

	provider := &Provider{
		Namespace:   "hashicorp",
		Type:        "aws",
		Version:     "5.0.0",
		Platform:    "linux_amd64",
		Filename:    "terraform-provider-aws_5.0.0_linux_amd64.zip",
		DownloadURL: "https://releases.hashicorp.com/...",
		Shasum:      "abc123",
		S3Key:       "providers/hashicorp/aws/5.0.0/linux_amd64.zip",
		SizeBytes:   1024000,
		Deprecated:  false,
		Blocked:     false,
	}

	err := repo.Create(ctx, provider)
	require.NoError(t, err)
	assert.Greater(t, provider.ID, int64(0))
	assert.False(t, provider.CreatedAt.IsZero())
}

func TestProviderRepository_GetByID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewProviderRepository(db)
	ctx := context.Background()

	// Create a provider
	provider := &Provider{
		Namespace:   "hashicorp",
		Type:        "aws",
		Version:     "5.0.0",
		Platform:    "linux_amd64",
		Filename:    "terraform-provider-aws_5.0.0_linux_amd64.zip",
		DownloadURL: "https://releases.hashicorp.com/...",
		Shasum:      "abc123",
		S3Key:       "providers/hashicorp/aws/5.0.0/linux_amd64.zip",
		SizeBytes:   1024000,
	}

	err := repo.Create(ctx, provider)
	require.NoError(t, err)

	// Retrieve by ID
	found, err := repo.GetByID(ctx, provider.ID)
	require.NoError(t, err)
	require.NotNil(t, found)

	assert.Equal(t, provider.Namespace, found.Namespace)
	assert.Equal(t, provider.Type, found.Type)
	assert.Equal(t, provider.Version, found.Version)
	assert.Equal(t, provider.Platform, found.Platform)
}

func TestProviderRepository_GetByIdentity(t *testing.T) {
	db := setupTestDB(t)
	repo := NewProviderRepository(db)
	ctx := context.Background()

	// Create a provider
	provider := &Provider{
		Namespace:   "hashicorp",
		Type:        "aws",
		Version:     "5.0.0",
		Platform:    "linux_amd64",
		Filename:    "terraform-provider-aws_5.0.0_linux_amd64.zip",
		DownloadURL: "https://releases.hashicorp.com/...",
		Shasum:      "abc123",
		S3Key:       "providers/hashicorp/aws/5.0.0/linux_amd64.zip",
		SizeBytes:   1024000,
	}

	err := repo.Create(ctx, provider)
	require.NoError(t, err)

	// Retrieve by identity
	found, err := repo.GetByIdentity(ctx, "hashicorp", "aws", "5.0.0", "linux_amd64")
	require.NoError(t, err)
	require.NotNil(t, found)

	assert.Equal(t, provider.ID, found.ID)
	assert.Equal(t, provider.Namespace, found.Namespace)

	// Try non-existent
	notFound, err := repo.GetByIdentity(ctx, "hashicorp", "aws", "6.0.0", "linux_amd64")
	require.NoError(t, err)
	assert.Nil(t, notFound)
}

func TestProviderRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	repo := NewProviderRepository(db)
	ctx := context.Background()

	// Create a provider
	provider := &Provider{
		Namespace:   "hashicorp",
		Type:        "aws",
		Version:     "5.0.0",
		Platform:    "linux_amd64",
		Filename:    "terraform-provider-aws_5.0.0_linux_amd64.zip",
		DownloadURL: "https://releases.hashicorp.com/...",
		Shasum:      "abc123",
		S3Key:       "providers/hashicorp/aws/5.0.0/linux_amd64.zip",
		SizeBytes:   1024000,
	}

	err := repo.Create(ctx, provider)
	require.NoError(t, err)

	// Update
	provider.Deprecated = true
	provider.Blocked = true
	err = repo.Update(ctx, provider)
	require.NoError(t, err)

	// Verify update
	found, err := repo.GetByID(ctx, provider.ID)
	require.NoError(t, err)
	assert.True(t, found.Deprecated)
	assert.True(t, found.Blocked)
}

func TestProviderRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	repo := NewProviderRepository(db)
	ctx := context.Background()

	// Create a provider
	provider := &Provider{
		Namespace:   "hashicorp",
		Type:        "aws",
		Version:     "5.0.0",
		Platform:    "linux_amd64",
		Filename:    "terraform-provider-aws_5.0.0_linux_amd64.zip",
		DownloadURL: "https://releases.hashicorp.com/...",
		Shasum:      "abc123",
		S3Key:       "providers/hashicorp/aws/5.0.0/linux_amd64.zip",
		SizeBytes:   1024000,
	}

	err := repo.Create(ctx, provider)
	require.NoError(t, err)

	// Delete
	err = repo.Delete(ctx, provider.ID)
	require.NoError(t, err)

	// Verify deletion
	found, err := repo.GetByID(ctx, provider.ID)
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestProviderRepository_Count(t *testing.T) {
	db := setupTestDB(t)
	repo := NewProviderRepository(db)
	ctx := context.Background()

	// Initial count should be 0
	count, err := repo.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Create providers with different platforms
	platforms := []string{"linux_amd64", "darwin_amd64", "windows_amd64"}
	for i, platform := range platforms {
		provider := &Provider{
			Namespace:   "hashicorp",
			Type:        "aws",
			Version:     "5.0.0",
			Platform:    platform,
			Filename:    "terraform-provider-aws_5.0.0_" + platform + ".zip",
			DownloadURL: "https://releases.hashicorp.com/...",
			Shasum:      "abc123",
			S3Key:       "providers/hashicorp/aws/5.0.0/" + platform + ".zip",
			SizeBytes:   int64(1024000 + i*1000),
		}
		err := repo.Create(ctx, provider)
		require.NoError(t, err)
	}

	count, err = repo.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

func TestUserRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	user := &AdminUser{
		Username:     "admin",
		PasswordHash: "hashed_password",
		FullName:     sql.NullString{String: "Admin User", Valid: true},
		Email:        sql.NullString{String: "admin@example.com", Valid: true},
		Active:       true,
	}

	err := repo.Create(ctx, user)
	require.NoError(t, err)
	assert.Greater(t, user.ID, int64(0))
	assert.False(t, user.CreatedAt.IsZero())
}

func TestUserRepository_GetByUsername(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	// Create user
	user := &AdminUser{
		Username:     "admin",
		PasswordHash: "hashed_password",
		Active:       true,
	}

	err := repo.Create(ctx, user)
	require.NoError(t, err)

	// Retrieve by username
	found, err := repo.GetByUsername(ctx, "admin")
	require.NoError(t, err)
	require.NotNil(t, found)

	assert.Equal(t, user.Username, found.Username)
	assert.Equal(t, user.PasswordHash, found.PasswordHash)

	// Try non-existent
	notFound, err := repo.GetByUsername(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, notFound)
}

func TestUserRepository_UpdateLastLogin(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	// Create user
	user := &AdminUser{
		Username:     "admin",
		PasswordHash: "hashed_password",
		Active:       true,
	}

	err := repo.Create(ctx, user)
	require.NoError(t, err)
	assert.False(t, user.LastLoginAt.Valid)

	// Update last login
	err = repo.UpdateLastLogin(ctx, user.ID)
	require.NoError(t, err)

	// Verify
	found, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.True(t, found.LastLoginAt.Valid)
}

func TestUserRepository_UpdatePassword(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	// Create user
	user := &AdminUser{
		Username:     "admin",
		PasswordHash: "old_hash",
		Active:       true,
	}

	err := repo.Create(ctx, user)
	require.NoError(t, err)

	// Update password
	err = repo.UpdatePassword(ctx, user.ID, "new_hash")
	require.NoError(t, err)

	// Verify
	found, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, "new_hash", found.PasswordHash)
}
