package database

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := New(dbPath)
	require.NoError(t, err)
	require.NotNil(t, db)
	defer db.Close()

	assert.Equal(t, dbPath, db.Path())
	assert.NotNil(t, db.Conn())
}

func TestNewCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "subdir", "nested", "test.db")

	db, err := New(dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Check that the nested directory was created
	_, err = os.Stat(filepath.Dir(dbPath))
	assert.NoError(t, err)
}

func TestPing(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := New(dbPath)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	err = db.Ping(ctx)
	assert.NoError(t, err)
}

func TestMigrations(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := New(dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Check that migrations table exists
	var tableName string
	err = db.conn.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='schema_migrations'").Scan(&tableName)
	require.NoError(t, err)
	assert.Equal(t, "schema_migrations", tableName)

	// Check current version
	var version int
	err = db.conn.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version)
	require.NoError(t, err)
	assert.Equal(t, 1, version)

	// Check that all expected tables exist
	expectedTables := []string{
		"providers",
		"admin_users",
		"admin_sessions",
		"admin_actions",
		"download_jobs",
		"download_job_items",
	}

	for _, table := range expectedTables {
		var name string
		err = db.conn.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		assert.NoError(t, err, "table %s should exist", table)
		assert.Equal(t, table, name)
	}
}

func TestMigrationsIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create database first time
	db1, err := New(dbPath)
	require.NoError(t, err)
	db1.Close()

	// Open again - migrations should not run again
	db2, err := New(dbPath)
	require.NoError(t, err)
	defer db2.Close()

	// Check version is still 1
	var version int
	err = db2.conn.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version)
	require.NoError(t, err)
	assert.Equal(t, 1, version)

	// Check count of migration records
	var count int
	err = db2.conn.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestWALMode(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := New(dbPath)
	require.NoError(t, err)
	defer db.Close()

	var journalMode string
	err = db.conn.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	require.NoError(t, err)
	assert.Equal(t, "wal", journalMode)
}

func TestForeignKeys(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := New(dbPath)
	require.NoError(t, err)
	defer db.Close()

	var foreignKeys int
	err = db.conn.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys)
	require.NoError(t, err)
	assert.Equal(t, 1, foreignKeys)
}

func TestBeginTx(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := New(dbPath)
	require.NoError(t, err)
	defer db.Close()

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, tx)

	// Rollback to clean up
	err = tx.Rollback()
	assert.NoError(t, err)
}

func TestClose(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := New(dbPath)
	require.NoError(t, err)

	err = db.Close()
	assert.NoError(t, err)

	// Ping should fail after close
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = db.Ping(ctx)
	assert.Error(t, err)
}

func TestIndexes(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := New(dbPath)
	require.NoError(t, err)
	defer db.Close()

	// Check that indexes were created
	expectedIndexes := []string{
		"idx_providers_lookup",
		"idx_providers_platform",
		"idx_providers_created",
		"idx_admin_users_username",
		"idx_admin_sessions_token",
		"idx_admin_actions_user",
		"idx_download_jobs_status",
		"idx_download_job_items_job",
	}

	for _, index := range expectedIndexes {
		var name string
		err = db.conn.QueryRow("SELECT name FROM sqlite_master WHERE type='index' AND name=?", index).Scan(&name)
		assert.NoError(t, err, "index %s should exist", index)
		assert.Equal(t, index, name)
	}
}
