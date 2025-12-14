package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ModuleRepository handles module database operations
type ModuleRepository struct {
	db *DB
}

// NewModuleRepository creates a new module repository
func NewModuleRepository(db *DB) *ModuleRepository {
	return &ModuleRepository{db: db}
}

// Create inserts a new module
func (r *ModuleRepository) Create(ctx context.Context, m *Module) error {
	query := `
		INSERT INTO modules (
			namespace, name, system, version,
			s3_key, filename, size_bytes,
			original_source_url, deprecated, blocked
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.conn.ExecContext(ctx, query,
		m.Namespace, m.Name, m.System, m.Version,
		m.S3Key, m.Filename, m.SizeBytes,
		m.OriginalSourceURL, m.Deprecated, m.Blocked,
	)
	if err != nil {
		return fmt.Errorf("failed to create module: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	m.ID = id
	m.CreatedAt = time.Now()
	m.UpdatedAt = time.Now()
	return nil
}

// GetByID retrieves a module by ID
func (r *ModuleRepository) GetByID(ctx context.Context, id int64) (*Module, error) {
	query := `
		SELECT id, namespace, name, system, version,
			   s3_key, filename, size_bytes,
			   original_source_url, deprecated, blocked,
			   created_at, updated_at
		FROM modules
		WHERE id = ?
	`

	m := &Module{}
	err := r.db.conn.QueryRowContext(ctx, query, id).Scan(
		&m.ID, &m.Namespace, &m.Name, &m.System, &m.Version,
		&m.S3Key, &m.Filename, &m.SizeBytes,
		&m.OriginalSourceURL, &m.Deprecated, &m.Blocked,
		&m.CreatedAt, &m.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get module: %w", err)
	}

	return m, nil
}

// GetByIdentity retrieves a module by namespace, name, system, and version
func (r *ModuleRepository) GetByIdentity(ctx context.Context, namespace, name, system, version string) (*Module, error) {
	query := `
		SELECT id, namespace, name, system, version,
			   s3_key, filename, size_bytes,
			   original_source_url, deprecated, blocked,
			   created_at, updated_at
		FROM modules
		WHERE namespace = ? AND name = ? AND system = ? AND version = ?
	`

	m := &Module{}
	err := r.db.conn.QueryRowContext(ctx, query, namespace, name, system, version).Scan(
		&m.ID, &m.Namespace, &m.Name, &m.System, &m.Version,
		&m.S3Key, &m.Filename, &m.SizeBytes,
		&m.OriginalSourceURL, &m.Deprecated, &m.Blocked,
		&m.CreatedAt, &m.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get module: %w", err)
	}

	return m, nil
}

// ListVersions retrieves all versions of a module for a namespace, name, and system
func (r *ModuleRepository) ListVersions(ctx context.Context, namespace, name, system string) ([]*Module, error) {
	query := `
		SELECT id, namespace, name, system, version,
			   s3_key, filename, size_bytes,
			   original_source_url, deprecated, blocked,
			   created_at, updated_at
		FROM modules
		WHERE namespace = ? AND name = ? AND system = ?
		  AND blocked = 0
		ORDER BY version DESC
	`

	rows, err := r.db.conn.QueryContext(ctx, query, namespace, name, system)
	if err != nil {
		return nil, fmt.Errorf("failed to list module versions: %w", err)
	}
	defer rows.Close()

	var modules []*Module
	for rows.Next() {
		m := &Module{}
		if err := rows.Scan(
			&m.ID, &m.Namespace, &m.Name, &m.System, &m.Version,
			&m.S3Key, &m.Filename, &m.SizeBytes,
			&m.OriginalSourceURL, &m.Deprecated, &m.Blocked,
			&m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan module: %w", err)
		}
		modules = append(modules, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating modules: %w", err)
	}

	return modules, nil
}

// List retrieves all modules with optional pagination
func (r *ModuleRepository) List(ctx context.Context, limit, offset int) ([]*Module, error) {
	query := `
		SELECT id, namespace, name, system, version,
			   s3_key, filename, size_bytes,
			   original_source_url, deprecated, blocked,
			   created_at, updated_at
		FROM modules
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.conn.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list modules: %w", err)
	}
	defer rows.Close()

	var modules []*Module
	for rows.Next() {
		m := &Module{}
		if err := rows.Scan(
			&m.ID, &m.Namespace, &m.Name, &m.System, &m.Version,
			&m.S3Key, &m.Filename, &m.SizeBytes,
			&m.OriginalSourceURL, &m.Deprecated, &m.Blocked,
			&m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan module: %w", err)
		}
		modules = append(modules, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating modules: %w", err)
	}

	return modules, nil
}

// Update updates a module
func (r *ModuleRepository) Update(ctx context.Context, m *Module) error {
	query := `
		UPDATE modules
		SET deprecated = ?, blocked = ?, size_bytes = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	result, err := r.db.conn.ExecContext(ctx, query, m.Deprecated, m.Blocked, m.SizeBytes, m.ID)
	if err != nil {
		return fmt.Errorf("failed to update module: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("module not found")
	}

	m.UpdatedAt = time.Now()
	return nil
}

// Delete deletes a module
func (r *ModuleRepository) Delete(ctx context.Context, id int64) error {
	query := "DELETE FROM modules WHERE id = ?"

	result, err := r.db.conn.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete module: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("module not found")
	}

	return nil
}

// Count returns the total number of modules
func (r *ModuleRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM modules").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count modules: %w", err)
	}
	return count, nil
}

// ModuleStorageStats represents module storage statistics
type ModuleStorageStats struct {
	TotalModules     int64 `json:"total_modules"`
	TotalSizeBytes   int64 `json:"total_size_bytes"`
	UniqueNamespaces int64 `json:"unique_namespaces"`
	UniqueNames      int64 `json:"unique_names"`
	UniqueVersions   int64 `json:"unique_versions"`
	DeprecatedCount  int64 `json:"deprecated_count"`
	BlockedCount     int64 `json:"blocked_count"`
}

// GetStorageStats returns module storage statistics
func (r *ModuleRepository) GetStorageStats(ctx context.Context) (*ModuleStorageStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_modules,
			COALESCE(SUM(size_bytes), 0) as total_size_bytes,
			COUNT(DISTINCT namespace) as unique_namespaces,
			COUNT(DISTINCT namespace || '/' || name || '/' || system) as unique_names,
			COUNT(DISTINCT namespace || '/' || name || '/' || system || '/' || version) as unique_versions,
			SUM(CASE WHEN deprecated = 1 THEN 1 ELSE 0 END) as deprecated_count,
			SUM(CASE WHEN blocked = 1 THEN 1 ELSE 0 END) as blocked_count
		FROM modules
	`

	stats := &ModuleStorageStats{}
	err := r.db.conn.QueryRowContext(ctx, query).Scan(
		&stats.TotalModules,
		&stats.TotalSizeBytes,
		&stats.UniqueNamespaces,
		&stats.UniqueNames,
		&stats.UniqueVersions,
		&stats.DeprecatedCount,
		&stats.BlockedCount,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get module storage stats: %w", err)
	}

	return stats, nil
}
