package database

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Job Repository Additional Tests

func TestJobRepository_ListPending(t *testing.T) {
	db := setupTestDB(t)
	repo := NewJobRepository(db)
	ctx := context.Background()

	user := createTestUser(t, db, "testuser")

	// Create pending jobs
	for i := 0; i < 3; i++ {
		job := &DownloadJob{
			UserID:     sql.NullInt64{Int64: user.ID, Valid: true},
			SourceType: "hcl",
			SourceData: "provider \"aws\" {}",
			Status:     "pending",
			TotalItems: 10,
		}
		err := repo.Create(ctx, job)
		require.NoError(t, err)
	}

	// Create running job (should not be included)
	runningJob := &DownloadJob{
		UserID:     sql.NullInt64{Int64: user.ID, Valid: true},
		SourceType: "hcl",
		SourceData: "provider \"azure\" {}",
		Status:     "running",
		TotalItems: 5,
	}
	err := repo.Create(ctx, runningJob)
	require.NoError(t, err)

	// List pending jobs
	pendingJobs, err := repo.ListPending(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, pendingJobs, 3)

	for _, job := range pendingJobs {
		assert.Equal(t, "pending", job.Status)
	}
}

func TestJobRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	repo := NewJobRepository(db)
	ctx := context.Background()

	user := createTestUser(t, db, "testuser")

	job := &DownloadJob{
		UserID:     sql.NullInt64{Int64: user.ID, Valid: true},
		SourceType: "hcl",
		SourceData: "provider \"aws\" {}",
		Status:     "pending",
		TotalItems: 10,
	}

	err := repo.Create(ctx, job)
	require.NoError(t, err)

	// Update job
	job.Status = "completed"
	job.CompletedItems = 10
	job.Progress = 100.0
	job.CompletedAt = sql.NullTime{Time: time.Now(), Valid: true}

	err = repo.Update(ctx, job)
	require.NoError(t, err)

	// Verify update
	found, err := repo.GetByID(ctx, job.ID)
	require.NoError(t, err)
	assert.Equal(t, "completed", found.Status)
	assert.Equal(t, 10, found.CompletedItems)
	assert.True(t, found.CompletedAt.Valid)
}

func TestJobRepository_Update_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewJobRepository(db)
	ctx := context.Background()

	job := &DownloadJob{
		ID:     99999,
		Status: "completed",
	}

	err := repo.Update(ctx, job)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestJobRepository_CreateItem(t *testing.T) {
	db := setupTestDB(t)
	jobRepo := NewJobRepository(db)
	ctx := context.Background()

	user := createTestUser(t, db, "testuser")

	// Create job first
	job := &DownloadJob{
		UserID:     sql.NullInt64{Int64: user.ID, Valid: true},
		SourceType: "hcl",
		SourceData: "provider \"aws\" {}",
		Status:     "pending",
		TotalItems: 1,
	}
	err := jobRepo.Create(ctx, job)
	require.NoError(t, err)

	// Create item
	item := &DownloadJobItem{
		JobID:     job.ID,
		Namespace: "hashicorp",
		Type:      "aws",
		Version:   "5.0.0",
		Platform:  "linux_amd64",
		Status:    "pending",
	}

	err = jobRepo.CreateItem(ctx, item)
	require.NoError(t, err)
	assert.NotZero(t, item.ID)
}

func TestJobRepository_UpdateItem(t *testing.T) {
	db := setupTestDB(t)
	jobRepo := NewJobRepository(db)
	ctx := context.Background()

	user := createTestUser(t, db, "testuser")

	// Create job and item
	job := &DownloadJob{
		UserID:     sql.NullInt64{Int64: user.ID, Valid: true},
		SourceType: "hcl",
		Status:     "pending",
		TotalItems: 1,
	}
	err := jobRepo.Create(ctx, job)
	require.NoError(t, err)

	item := &DownloadJobItem{
		JobID:     job.ID,
		Namespace: "hashicorp",
		Type:      "aws",
		Version:   "5.0.0",
		Platform:  "linux_amd64",
		Status:    "pending",
	}
	err = jobRepo.CreateItem(ctx, item)
	require.NoError(t, err)

	// Update item
	item.Status = "completed"
	item.SizeBytes = sql.NullInt64{Int64: 1024000, Valid: true}
	item.DownloadedBytes = sql.NullInt64{Int64: 1024000, Valid: true}
	item.CompletedAt = sql.NullTime{Time: time.Now(), Valid: true}

	err = jobRepo.UpdateItem(ctx, item)
	require.NoError(t, err)
}

func TestJobRepository_UpdateItem_NotFound(t *testing.T) {
	db := setupTestDB(t)
	repo := NewJobRepository(db)
	ctx := context.Background()

	item := &DownloadJobItem{
		ID:     99999,
		Status: "completed",
	}

	err := repo.UpdateItem(ctx, item)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestJobRepository_GetItems(t *testing.T) {
	db := setupTestDB(t)
	jobRepo := NewJobRepository(db)
	ctx := context.Background()

	user := createTestUser(t, db, "testuser")

	// Create job
	job := &DownloadJob{
		UserID:     sql.NullInt64{Int64: user.ID, Valid: true},
		SourceType: "hcl",
		Status:     "pending",
		TotalItems: 3,
	}
	err := jobRepo.Create(ctx, job)
	require.NoError(t, err)

	// Create items
	for i := 0; i < 3; i++ {
		item := &DownloadJobItem{
			JobID:     job.ID,
			Namespace: "hashicorp",
			Type:      "aws",
			Version:   "5.0.0",
			Platform:  []string{"linux_amd64", "darwin_amd64", "windows_amd64"}[i],
			Status:    "pending",
		}
		err = jobRepo.CreateItem(ctx, item)
		require.NoError(t, err)
	}

	// Get items
	items, err := jobRepo.GetItems(ctx, job.ID)
	require.NoError(t, err)
	assert.Len(t, items, 3)
}

func TestJobRepository_ResetFailedItems(t *testing.T) {
	db := setupTestDB(t)
	jobRepo := NewJobRepository(db)
	ctx := context.Background()

	user := createTestUser(t, db, "testuser")

	// Create job
	job := &DownloadJob{
		UserID:      sql.NullInt64{Int64: user.ID, Valid: true},
		SourceType:  "hcl",
		Status:      "failed",
		TotalItems:  2,
		FailedItems: 2,
	}
	err := jobRepo.Create(ctx, job)
	require.NoError(t, err)

	// Create failed items
	for i := 0; i < 2; i++ {
		item := &DownloadJobItem{
			JobID:     job.ID,
			Namespace: "hashicorp",
			Type:      "aws",
			Version:   "5.0.0",
			Platform:  []string{"linux_amd64", "darwin_amd64"}[i],
			Status:    "failed",
		}
		err = jobRepo.CreateItem(ctx, item)
		require.NoError(t, err)
	}

	// Reset failed items
	count, err := jobRepo.ResetFailedItems(ctx, job.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	// Verify items are now pending
	items, err := jobRepo.GetItems(ctx, job.ID)
	require.NoError(t, err)
	for _, item := range items {
		assert.Equal(t, "pending", item.Status)
	}
}

// Provider Repository Additional Tests

func TestProviderRepository_List(t *testing.T) {
	db := setupTestDB(t)
	repo := NewProviderRepository(db)
	ctx := context.Background()

	// Create providers
	namespaces := []string{"hashicorp", "aws", "azure"}
	for _, ns := range namespaces {
		provider := &Provider{
			Namespace:   ns,
			Type:        "example",
			Version:     "1.0.0",
			Platform:    "linux_amd64",
			Filename:    "terraform-provider-example_1.0.0_linux_amd64.zip",
			DownloadURL: "https://example.com/...",
			Shasum:      "abc123",
			S3Key:       "providers/" + ns + "/example/1.0.0/linux_amd64.zip",
			SizeBytes:   1024000,
		}
		err := repo.Create(ctx, provider)
		require.NoError(t, err)
	}

	// List with pagination
	providers, err := repo.List(ctx, 2, 0)
	require.NoError(t, err)
	assert.Len(t, providers, 2)

	providers, err = repo.List(ctx, 2, 2)
	require.NoError(t, err)
	assert.Len(t, providers, 1)
}

func TestProviderRepository_ListVersions(t *testing.T) {
	db := setupTestDB(t)
	repo := NewProviderRepository(db)
	ctx := context.Background()

	// Create providers with different versions
	versions := []string{"1.0.0", "2.0.0", "3.0.0"}
	for _, ver := range versions {
		provider := &Provider{
			Namespace:   "hashicorp",
			Type:        "aws",
			Version:     ver,
			Platform:    "linux_amd64",
			Filename:    "terraform-provider-aws_" + ver + "_linux_amd64.zip",
			DownloadURL: "https://example.com/...",
			Shasum:      "abc123",
			S3Key:       "providers/hashicorp/aws/" + ver + "/linux_amd64.zip",
			SizeBytes:   1024000,
		}
		err := repo.Create(ctx, provider)
		require.NoError(t, err)
	}

	// List versions
	providerVersions, err := repo.ListVersions(ctx, "hashicorp", "aws")
	require.NoError(t, err)
	assert.Len(t, providerVersions, 3)
}

func TestProviderRepository_GetStorageStats(t *testing.T) {
	db := setupTestDB(t)
	repo := NewProviderRepository(db)
	ctx := context.Background()

	// Create providers with different sizes
	for i := 0; i < 3; i++ {
		provider := &Provider{
			Namespace:   "hashicorp",
			Type:        "aws",
			Version:     "5.0.0",
			Platform:    []string{"linux_amd64", "darwin_amd64", "windows_amd64"}[i],
			Filename:    "terraform-provider-aws.zip",
			DownloadURL: "https://example.com/...",
			Shasum:      "abc123",
			S3Key:       "providers/hashicorp/aws/5.0.0/" + []string{"linux_amd64", "darwin_amd64", "windows_amd64"}[i] + ".zip",
			SizeBytes:   int64(1024000 * (i + 1)),
		}
		err := repo.Create(ctx, provider)
		require.NoError(t, err)
	}

	stats, err := repo.GetStorageStats(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(3), stats.TotalProviders)
	assert.Equal(t, int64(1024000+2048000+3072000), stats.TotalSizeBytes) // 1+2+3 MB
}

// User Repository Additional Tests

func TestUserRepository_GetByID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	user := &AdminUser{
		Username:     "admin",
		PasswordHash: "hashed_password",
		Active:       true,
	}

	err := repo.Create(ctx, user)
	require.NoError(t, err)

	// Get by ID
	found, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, user.Username, found.Username)

	// Not found
	notFound, err := repo.GetByID(ctx, 99999)
	require.NoError(t, err)
	assert.Nil(t, notFound)
}

func TestUserRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	user := &AdminUser{
		Username:     "admin",
		PasswordHash: "hashed_password",
		Active:       true,
	}

	err := repo.Create(ctx, user)
	require.NoError(t, err)

	// Update
	user.FullName = sql.NullString{String: "Updated Name", Valid: true}
	user.Email = sql.NullString{String: "updated@example.com", Valid: true}
	user.Active = false

	err = repo.Update(ctx, user)
	require.NoError(t, err)

	// Verify
	found, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", found.FullName.String)
	assert.Equal(t, "updated@example.com", found.Email.String)
	assert.False(t, found.Active)
}

func TestUserRepository_List(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	// Create users
	for i := 0; i < 5; i++ {
		user := &AdminUser{
			Username:     "user" + string(rune('1'+i)),
			PasswordHash: "hashed_password",
			Active:       true,
		}
		err := repo.Create(ctx, user)
		require.NoError(t, err)
	}

	// List all users
	users, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, users, 5)
}

func TestUserRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	user := &AdminUser{
		Username:     "todelete",
		PasswordHash: "hashed_password",
		Active:       true,
	}

	err := repo.Create(ctx, user)
	require.NoError(t, err)

	// Delete
	err = repo.Delete(ctx, user.ID)
	require.NoError(t, err)

	// Verify
	found, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Nil(t, found)
}

// Session Repository Additional Tests

func TestSessionRepository_GetByID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSessionRepository(db)
	ctx := context.Background()

	user := createTestUser(t, db, "testuser")

	session := &AdminSession{
		UserID:    user.ID,
		TokenJTI:  "test-jti-123",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	err := repo.Create(ctx, session)
	require.NoError(t, err)

	// Get by ID
	found, err := repo.GetByID(ctx, session.ID)
	require.NoError(t, err)
	assert.Equal(t, session.TokenJTI, found.TokenJTI)

	// Not found
	notFound, err := repo.GetByID(ctx, 99999)
	require.NoError(t, err)
	assert.Nil(t, notFound)
}

func TestSessionRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSessionRepository(db)
	ctx := context.Background()

	user := createTestUser(t, db, "testuser")

	session := &AdminSession{
		UserID:    user.ID,
		TokenJTI:  "test-jti-123",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	err := repo.Create(ctx, session)
	require.NoError(t, err)

	// Delete
	err = repo.Delete(ctx, session.ID)
	require.NoError(t, err)

	// Verify
	found, err := repo.GetByID(ctx, session.ID)
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestSessionRepository_DeleteByTokenJTI(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSessionRepository(db)
	ctx := context.Background()

	user := createTestUser(t, db, "testuser")

	session := &AdminSession{
		UserID:    user.ID,
		TokenJTI:  "test-jti-to-delete",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	err := repo.Create(ctx, session)
	require.NoError(t, err)

	// Delete by JTI
	err = repo.DeleteByTokenJTI(ctx, "test-jti-to-delete")
	require.NoError(t, err)

	// Verify
	found, err := repo.GetByTokenJTI(ctx, "test-jti-to-delete")
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestSessionRepository_RevokeByTokenJTI(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSessionRepository(db)
	ctx := context.Background()

	user := createTestUser(t, db, "testuser")

	session := &AdminSession{
		UserID:    user.ID,
		TokenJTI:  "test-jti-to-revoke",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	err := repo.Create(ctx, session)
	require.NoError(t, err)

	// Revoke by JTI
	err = repo.RevokeByTokenJTI(ctx, "test-jti-to-revoke")
	require.NoError(t, err)

	// Verify session is revoked
	found, err := repo.GetByTokenJTI(ctx, "test-jti-to-revoke")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.True(t, found.Revoked)
}

func TestSessionRepository_DeleteByUserID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSessionRepository(db)
	ctx := context.Background()

	user := createTestUser(t, db, "testuser")

	// Create multiple sessions
	for i := 0; i < 3; i++ {
		session := &AdminSession{
			UserID:    user.ID,
			TokenJTI:  "user-jti-" + string(rune('1'+i)),
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}
		err := repo.Create(ctx, session)
		require.NoError(t, err)
	}

	// Delete by user ID
	err := repo.DeleteByUserID(ctx, user.ID)
	require.NoError(t, err)

	// Verify all sessions are gone
	sessions, err := repo.ListByUserID(ctx, user.ID)
	require.NoError(t, err)
	assert.Len(t, sessions, 0)
}

// Audit Repository Additional Tests

func TestAuditRepository_ListByResource(t *testing.T) {
	db := setupTestDB(t)
	repo := NewAuditRepository(db)
	ctx := context.Background()

	user := createTestUser(t, db, "testuser")

	// Create actions with different resources
	resources := []string{"provider-1", "provider-2", "provider-1"}
	for _, res := range resources {
		action := &AdminAction{
			UserID:       sql.NullInt64{Int64: user.ID, Valid: true},
			Action:       "provider.update",
			ResourceType: "provider",
			ResourceID:   sql.NullString{String: res, Valid: true},
			Success:      true,
		}
		err := repo.Log(ctx, action)
		require.NoError(t, err)
	}

	// List by resource
	actions, err := repo.ListByResource(ctx, "provider", "provider-1", 10, 0)
	require.NoError(t, err)
	assert.Len(t, actions, 2)
}

func TestAuditRepository_ListByAction(t *testing.T) {
	db := setupTestDB(t)
	repo := NewAuditRepository(db)
	ctx := context.Background()

	user := createTestUser(t, db, "testuser")

	// Create actions with different types
	actions := []string{"login", "logout", "login"}
	for _, act := range actions {
		action := &AdminAction{
			UserID:       sql.NullInt64{Int64: user.ID, Valid: true},
			Action:       act,
			ResourceType: "session",
			Success:      true,
		}
		err := repo.Log(ctx, action)
		require.NoError(t, err)
	}

	// List by action type
	loginActions, err := repo.ListByAction(ctx, "login", 10, 0)
	require.NoError(t, err)
	assert.Len(t, loginActions, 2)
}

func TestAuditRepository_DeleteOlderThan(t *testing.T) {
	db := setupTestDB(t)
	repo := NewAuditRepository(db)
	ctx := context.Background()

	user := createTestUser(t, db, "testuser")

	// Create actions
	for i := 0; i < 3; i++ {
		action := &AdminAction{
			UserID:       sql.NullInt64{Int64: user.ID, Valid: true},
			Action:       "test.action",
			ResourceType: "test",
			Success:      true,
		}
		err := repo.Log(ctx, action)
		require.NoError(t, err)
	}

	// Delete older than past date (should delete nothing since records are new)
	count, err := repo.DeleteOlderThan(ctx, time.Now().Add(-24*time.Hour))
	require.NoError(t, err)
	// Just test that the function runs without error
	_ = count
}

// Database Tests

func TestDatabase_Backup(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create some data
	userRepo := NewUserRepository(db)
	user := &AdminUser{
		Username:     "admin",
		PasswordHash: "hashed",
		Active:       true,
	}
	err := userRepo.Create(ctx, user)
	require.NoError(t, err)

	// Backup to another file
	backupPath := t.TempDir() + "/backup.db"
	err = db.Backup(ctx, backupPath)
	require.NoError(t, err)

	// Open backup and verify data exists
	backupDB, err := New(backupPath)
	require.NoError(t, err)
	defer backupDB.Close()

	backupUserRepo := NewUserRepository(backupDB)
	found, err := backupUserRepo.GetByUsername(ctx, "admin")
	require.NoError(t, err)
	assert.NotNil(t, found)
}
