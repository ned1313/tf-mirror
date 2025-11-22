package database

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to create a test user for foreign key requirements
func createTestUser(t *testing.T, db *DB, username string) *AdminUser {
	userRepo := NewUserRepository(db)
	user := &AdminUser{
		Username:     username,
		PasswordHash: "hash123",
		Active:       true,
	}
	err := userRepo.Create(context.Background(), user)
	require.NoError(t, err)
	return user
}

// Session Repository Tests

func TestSessionRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSessionRepository(db)
	ctx := context.Background()

	user := createTestUser(t, db, "testuser")

	session := &AdminSession{
		UserID:    user.ID,
		TokenJTI:  "jti-hash123",
		IPAddress: sql.NullString{String: "192.168.1.1", Valid: true},
		UserAgent: sql.NullString{String: "Mozilla/5.0", Valid: true},
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	err := repo.Create(ctx, session)
	require.NoError(t, err)
	assert.NotZero(t, session.ID)
	assert.NotZero(t, session.CreatedAt)
}

func TestSessionRepository_GetByTokenJTI(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSessionRepository(db)
	ctx := context.Background()

	user := createTestUser(t, db, "testuser")

	session := &AdminSession{
		UserID:    user.ID,
		TokenJTI:  "jti-hash123",
		IPAddress: sql.NullString{String: "192.168.1.1", Valid: true},
		UserAgent: sql.NullString{String: "Mozilla/5.0", Valid: true},
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	err := repo.Create(ctx, session)
	require.NoError(t, err)

	found, err := repo.GetByTokenJTI(ctx, "jti-hash123")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, session.ID, found.ID)
	assert.Equal(t, session.TokenJTI, found.TokenJTI)

	// Test not found
	notFound, err := repo.GetByTokenJTI(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, notFound)
}

func TestSessionRepository_ListByUserID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSessionRepository(db)
	ctx := context.Background()

	user1 := createTestUser(t, db, "user1")
	user2 := createTestUser(t, db, "user2")

	// Create sessions for user 1
	for i := 0; i < 3; i++ {
		session := &AdminSession{
			UserID:    user1.ID,
			TokenJTI:  "user1_jti" + string(rune('1'+i)),
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}
		err := repo.Create(ctx, session)
		require.NoError(t, err)
	}

	// Create session for user 2
	session := &AdminSession{
		UserID:    user2.ID,
		TokenJTI:  "user2_jti",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	err := repo.Create(ctx, session)
	require.NoError(t, err)

	// List sessions for user 1
	sessions, err := repo.ListByUserID(ctx, user1.ID)
	require.NoError(t, err)
	assert.Len(t, sessions, 3)

	// List sessions for user 2
	sessions, err = repo.ListByUserID(ctx, user2.ID)
	require.NoError(t, err)
	assert.Len(t, sessions, 1)
}

func TestSessionRepository_DeleteExpired(t *testing.T) {
	db := setupTestDB(t)
	repo := NewSessionRepository(db)
	ctx := context.Background()

	user := createTestUser(t, db, "testuser")

	// Create expired session
	expiredSession := &AdminSession{
		UserID:    user.ID,
		TokenJTI:  "expired-jti",
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	err := repo.Create(ctx, expiredSession)
	require.NoError(t, err)

	// Create valid session
	validSession := &AdminSession{
		UserID:    user.ID,
		TokenJTI:  "valid-jti",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	err = repo.Create(ctx, validSession)
	require.NoError(t, err)

	// Delete expired sessions
	count, err := repo.DeleteExpired(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Verify expired is gone
	found, err := repo.GetByTokenJTI(ctx, "expired-jti")
	require.NoError(t, err)
	assert.Nil(t, found)

	// Verify valid still exists
	found, err = repo.GetByTokenJTI(ctx, "valid-jti")
	require.NoError(t, err)
	assert.NotNil(t, found)
}

// Job Repository Tests

func TestJobRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	repo := NewJobRepository(db)
	ctx := context.Background()

	user := createTestUser(t, db, "testuser")

	job := &DownloadJob{
		UserID:         sql.NullInt64{Int64: user.ID, Valid: true},
		SourceType:     "hcl",
		SourceData:     "provider \"aws\" {}",
		Status:         "pending",
		TotalItems:     10,
		CompletedItems: 0,
		FailedItems:    0,
		StartedAt:      sql.NullTime{Time: time.Now(), Valid: true},
	}

	err := repo.Create(ctx, job)
	require.NoError(t, err)
	assert.NotZero(t, job.ID)
	assert.NotZero(t, job.CreatedAt)
}

func TestJobRepository_GetByID(t *testing.T) {
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
		StartedAt:  sql.NullTime{Time: time.Now(), Valid: true},
	}

	err := repo.Create(ctx, job)
	require.NoError(t, err)

	found, err := repo.GetByID(ctx, job.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, job.ID, found.ID)
	assert.Equal(t, job.Status, found.Status)

	// Test not found
	notFound, err := repo.GetByID(ctx, 99999)
	require.NoError(t, err)
	assert.Nil(t, notFound)
}

func TestJobRepository_List(t *testing.T) {
	db := setupTestDB(t)
	repo := NewJobRepository(db)
	ctx := context.Background()

	user := createTestUser(t, db, "testuser")

	// Create jobs
	for i := 0; i < 5; i++ {
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

	// List with pagination
	jobs, err := repo.List(ctx, 3, 0)
	require.NoError(t, err)
	assert.Len(t, jobs, 3)

	jobs, err = repo.List(ctx, 3, 3)
	require.NoError(t, err)
	assert.Len(t, jobs, 2)
}

func TestJobRepository_CountByStatus(t *testing.T) {
	db := setupTestDB(t)
	repo := NewJobRepository(db)
	ctx := context.Background()

	user := createTestUser(t, db, "testuser")

	// Create jobs with different statuses
	statuses := []string{"pending", "running", "completed", "failed"}
	for _, status := range statuses {
		for i := 0; i < 2; i++ {
			job := &DownloadJob{
				UserID:     sql.NullInt64{Int64: user.ID, Valid: true},
				SourceType: "hcl",
				Status:     status,
				TotalItems: 10,
			}
			err := repo.Create(ctx, job)
			require.NoError(t, err)
		}
	}

	// Count by status
	count, err := repo.CountByStatus(ctx, "pending")
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	count, err = repo.CountByStatus(ctx, "completed")
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

// Audit Repository Tests

func TestAuditRepository_Log(t *testing.T) {
	db := setupTestDB(t)
	repo := NewAuditRepository(db)
	ctx := context.Background()

	user := createTestUser(t, db, "testuser")

	action := &AdminAction{
		UserID:       sql.NullInt64{Int64: user.ID, Valid: true},
		Action:       "provider.create",
		ResourceType: "provider",
		ResourceID:   sql.NullString{String: "123", Valid: true},
		IPAddress:    sql.NullString{String: "192.168.1.1", Valid: true},
		UserAgent:    sql.NullString{String: "Mozilla/5.0", Valid: true},
		Success:      true,
		Metadata:     sql.NullString{String: `{"key":"value"}`, Valid: true},
	}

	err := repo.Log(ctx, action)
	require.NoError(t, err)
	assert.NotZero(t, action.ID)
	assert.NotZero(t, action.CreatedAt)
}

func TestAuditRepository_ListByUser(t *testing.T) {
	db := setupTestDB(t)
	repo := NewAuditRepository(db)
	ctx := context.Background()

	user1 := createTestUser(t, db, "user1")
	user2 := createTestUser(t, db, "user2")

	// Create actions for user 1
	for i := 0; i < 3; i++ {
		action := &AdminAction{
			UserID:       sql.NullInt64{Int64: user1.ID, Valid: true},
			Action:       "test.action",
			ResourceType: "test",
			Success:      true,
		}
		err := repo.Log(ctx, action)
		require.NoError(t, err)
	}

	// Create action for user 2
	action := &AdminAction{
		UserID:       sql.NullInt64{Int64: user2.ID, Valid: true},
		Action:       "test.action",
		ResourceType: "test",
		Success:      true,
	}
	err := repo.Log(ctx, action)
	require.NoError(t, err)

	// List actions for user 1
	actions, err := repo.ListByUser(ctx, user1.ID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, actions, 3)

	// List actions for user 2
	actions, err = repo.ListByUser(ctx, user2.ID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, actions, 1)
}

func TestAuditRepository_List(t *testing.T) {
	db := setupTestDB(t)
	repo := NewAuditRepository(db)
	ctx := context.Background()

	user := createTestUser(t, db, "testuser")

	// Create actions
	for i := 0; i < 5; i++ {
		action := &AdminAction{
			UserID:       sql.NullInt64{Int64: user.ID, Valid: true},
			Action:       "test.action",
			ResourceType: "test",
			Success:      true,
		}
		err := repo.Log(ctx, action)
		require.NoError(t, err)
	}

	// List with pagination
	actions, err := repo.List(ctx, 3, 0)
	require.NoError(t, err)
	assert.Len(t, actions, 3)

	actions, err = repo.List(ctx, 3, 3)
	require.NoError(t, err)
	assert.Len(t, actions, 2)
}
