package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps the database connection and provides access to repositories
type DB struct {
	conn *sql.DB
	path string
}

// New creates a new database connection and runs migrations
func New(dbPath string) (*DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open database connection
	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(5 * time.Minute)

	// Enable WAL mode for better concurrency
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Enable foreign keys
	if _, err := conn.Exec("PRAGMA foreign_keys=ON"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	db := &DB{
		conn: conn,
		path: dbPath,
	}

	// Run migrations
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	log.Printf("Database initialized: %s", dbPath)
	return db, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	if db.conn != nil {
		return db.conn.Close()
	}
	return nil
}

// Conn returns the underlying database connection
func (db *DB) Conn() *sql.DB {
	return db.conn
}

// Path returns the database file path
func (db *DB) Path() string {
	return db.path
}

// Ping checks if the database connection is alive
func (db *DB) Ping(ctx context.Context) error {
	return db.conn.PingContext(ctx)
}

// BeginTx starts a new transaction
func (db *DB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return db.conn.BeginTx(ctx, opts)
}

// migrate runs database migrations
func (db *DB) migrate() error {
	// Create migrations table if it doesn't exist
	createMigrationsTable := `
	CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	`
	if _, err := db.conn.Exec(createMigrationsTable); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get current version
	var currentVersion int
	err := db.conn.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&currentVersion)
	if err != nil {
		return fmt.Errorf("failed to get current schema version: %w", err)
	}

	log.Printf("Current schema version: %d", currentVersion)

	// Apply migrations
	migrations := getMigrations()
	for version, migration := range migrations {
		if version <= currentVersion {
			continue
		}

		log.Printf("Applying migration %d...", version)

		tx, err := db.conn.Begin()
		if err != nil {
			return fmt.Errorf("failed to start transaction for migration %d: %w", version, err)
		}

		// Execute migration
		if _, err := tx.Exec(migration); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d failed: %w", version, err)
		}

		// Record migration
		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", version); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %d: %w", version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %d: %w", version, err)
		}

		log.Printf("Migration %d applied successfully", version)
	}

	return nil
}

// getMigrations returns all database migrations
func getMigrations() map[int]string {
	return map[int]string{
		1: migration001Initial,
	}
}

// migration001Initial is the initial database schema
const migration001Initial = `
-- Providers table
CREATE TABLE providers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    namespace TEXT NOT NULL,
    type TEXT NOT NULL,
    version TEXT NOT NULL,
    platform TEXT NOT NULL,
    
    -- Provider metadata
    filename TEXT NOT NULL,
    download_url TEXT NOT NULL,
    shasum TEXT NOT NULL,
    
    -- GPG signature verification
    signing_keys TEXT,
    
    -- Storage information
    s3_key TEXT NOT NULL,
    size_bytes INTEGER NOT NULL,
    
    -- Status flags
    deprecated BOOLEAN NOT NULL DEFAULT 0,
    blocked BOOLEAN NOT NULL DEFAULT 0,
    
    -- Timestamps
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Unique constraint on provider identity
    UNIQUE(namespace, type, version, platform)
);

CREATE INDEX idx_providers_lookup ON providers(namespace, type, version);
CREATE INDEX idx_providers_platform ON providers(platform);
CREATE INDEX idx_providers_created ON providers(created_at DESC);

-- Admin users table
CREATE TABLE admin_users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    
    -- User metadata
    full_name TEXT,
    email TEXT,
    
    -- Status
    active BOOLEAN NOT NULL DEFAULT 1,
    
    -- Timestamps
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_login_at DATETIME
);

CREATE INDEX idx_admin_users_username ON admin_users(username);
CREATE INDEX idx_admin_users_active ON admin_users(active);

-- Admin sessions table
CREATE TABLE admin_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    
    -- Session metadata
    ip_address TEXT,
    user_agent TEXT,
    
    -- Expiration
    expires_at DATETIME NOT NULL,
    revoked BOOLEAN NOT NULL DEFAULT 0,
    
    -- Timestamps
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (user_id) REFERENCES admin_users(id) ON DELETE CASCADE
);

CREATE INDEX idx_admin_sessions_token ON admin_sessions(token_hash);
CREATE INDEX idx_admin_sessions_user ON admin_sessions(user_id);
CREATE INDEX idx_admin_sessions_expires ON admin_sessions(expires_at);

-- Admin actions audit log
CREATE TABLE admin_actions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    
    -- Action details
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT,
    
    -- Request context
    ip_address TEXT,
    user_agent TEXT,
    
    -- Action result
    success BOOLEAN NOT NULL DEFAULT 1,
    error_message TEXT,
    
    -- Additional data (JSON)
    metadata TEXT,
    
    -- Timestamp
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (user_id) REFERENCES admin_users(id) ON DELETE SET NULL
);

CREATE INDEX idx_admin_actions_user ON admin_actions(user_id);
CREATE INDEX idx_admin_actions_resource ON admin_actions(resource_type, resource_id);
CREATE INDEX idx_admin_actions_created ON admin_actions(created_at DESC);

-- Download jobs table
CREATE TABLE download_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER,
    
    -- Job configuration
    source_type TEXT NOT NULL, -- 'hcl', 'api'
    source_data TEXT NOT NULL, -- HCL content or API parameters
    
    -- Job status
    status TEXT NOT NULL DEFAULT 'pending', -- pending, running, completed, failed
    progress INTEGER NOT NULL DEFAULT 0, -- percentage 0-100
    
    -- Results
    total_items INTEGER NOT NULL DEFAULT 0,
    completed_items INTEGER NOT NULL DEFAULT 0,
    failed_items INTEGER NOT NULL DEFAULT 0,
    
    -- Error handling
    error_message TEXT,
    
    -- Timestamps
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at DATETIME,
    completed_at DATETIME,
    
    FOREIGN KEY (user_id) REFERENCES admin_users(id) ON DELETE SET NULL
);

CREATE INDEX idx_download_jobs_user ON download_jobs(user_id);
CREATE INDEX idx_download_jobs_status ON download_jobs(status);
CREATE INDEX idx_download_jobs_created ON download_jobs(created_at DESC);

-- Download job items table
CREATE TABLE download_job_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id INTEGER NOT NULL,
    
    -- Item details
    namespace TEXT NOT NULL,
    type TEXT NOT NULL,
    version TEXT NOT NULL,
    platform TEXT NOT NULL,
    
    -- Item status
    status TEXT NOT NULL DEFAULT 'pending', -- pending, downloading, completed, failed
    
    -- Progress
    download_url TEXT,
    size_bytes INTEGER,
    downloaded_bytes INTEGER DEFAULT 0,
    
    -- Results
    provider_id INTEGER,
    error_message TEXT,
    
    -- Retry tracking
    retry_count INTEGER NOT NULL DEFAULT 0,
    
    -- Timestamps
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at DATETIME,
    completed_at DATETIME,
    
    FOREIGN KEY (job_id) REFERENCES download_jobs(id) ON DELETE CASCADE,
    FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE SET NULL
);

CREATE INDEX idx_download_job_items_job ON download_job_items(job_id);
CREATE INDEX idx_download_job_items_status ON download_job_items(status);
CREATE INDEX idx_download_job_items_provider ON download_job_items(namespace, type, version, platform);
`
