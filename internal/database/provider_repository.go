package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ProviderRepository handles provider database operations
type ProviderRepository struct {
	db *DB
}

// NewProviderRepository creates a new provider repository
func NewProviderRepository(db *DB) *ProviderRepository {
	return &ProviderRepository{db: db}
}

// Create inserts a new provider
func (r *ProviderRepository) Create(ctx context.Context, p *Provider) error {
	query := `
		INSERT INTO providers (
			namespace, type, version, platform,
			filename, download_url, shasum, signing_keys,
			s3_key, size_bytes, deprecated, blocked
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.conn.ExecContext(ctx, query,
		p.Namespace, p.Type, p.Version, p.Platform,
		p.Filename, p.DownloadURL, p.Shasum, p.SigningKeys,
		p.S3Key, p.SizeBytes, p.Deprecated, p.Blocked,
	)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	p.ID = id
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	return nil
}

// GetByID retrieves a provider by ID
func (r *ProviderRepository) GetByID(ctx context.Context, id int64) (*Provider, error) {
	query := `
		SELECT id, namespace, type, version, platform,
			   filename, download_url, shasum, signing_keys,
			   s3_key, size_bytes, deprecated, blocked,
			   created_at, updated_at
		FROM providers
		WHERE id = ?
	`

	p := &Provider{}
	err := r.db.conn.QueryRowContext(ctx, query, id).Scan(
		&p.ID, &p.Namespace, &p.Type, &p.Version, &p.Platform,
		&p.Filename, &p.DownloadURL, &p.Shasum, &p.SigningKeys,
		&p.S3Key, &p.SizeBytes, &p.Deprecated, &p.Blocked,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}

	return p, nil
}

// GetByIdentity retrieves a provider by namespace, type, version, and platform
func (r *ProviderRepository) GetByIdentity(ctx context.Context, namespace, typ, version, platform string) (*Provider, error) {
	query := `
		SELECT id, namespace, type, version, platform,
			   filename, download_url, shasum, signing_keys,
			   s3_key, size_bytes, deprecated, blocked,
			   created_at, updated_at
		FROM providers
		WHERE namespace = ? AND type = ? AND version = ? AND platform = ?
	`

	p := &Provider{}
	err := r.db.conn.QueryRowContext(ctx, query, namespace, typ, version, platform).Scan(
		&p.ID, &p.Namespace, &p.Type, &p.Version, &p.Platform,
		&p.Filename, &p.DownloadURL, &p.Shasum, &p.SigningKeys,
		&p.S3Key, &p.SizeBytes, &p.Deprecated, &p.Blocked,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}

	return p, nil
}

// ListVersions retrieves all versions of a provider for a namespace and type
func (r *ProviderRepository) ListVersions(ctx context.Context, namespace, typ string) ([]*Provider, error) {
	query := `
		SELECT id, namespace, type, version, platform,
			   filename, download_url, shasum, signing_keys,
			   s3_key, size_bytes, deprecated, blocked,
			   created_at, updated_at
		FROM providers
		WHERE namespace = ? AND type = ?
		ORDER BY version DESC, platform ASC
	`

	rows, err := r.db.conn.QueryContext(ctx, query, namespace, typ)
	if err != nil {
		return nil, fmt.Errorf("failed to list provider versions: %w", err)
	}
	defer rows.Close()

	var providers []*Provider
	for rows.Next() {
		p := &Provider{}
		if err := rows.Scan(
			&p.ID, &p.Namespace, &p.Type, &p.Version, &p.Platform,
			&p.Filename, &p.DownloadURL, &p.Shasum, &p.SigningKeys,
			&p.S3Key, &p.SizeBytes, &p.Deprecated, &p.Blocked,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan provider: %w", err)
		}
		providers = append(providers, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating providers: %w", err)
	}

	return providers, nil
}

// List retrieves all providers with optional filters
func (r *ProviderRepository) List(ctx context.Context, limit, offset int) ([]*Provider, error) {
	query := `
		SELECT id, namespace, type, version, platform,
			   filename, download_url, shasum, signing_keys,
			   s3_key, size_bytes, deprecated, blocked,
			   created_at, updated_at
		FROM providers
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.conn.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list providers: %w", err)
	}
	defer rows.Close()

	var providers []*Provider
	for rows.Next() {
		p := &Provider{}
		if err := rows.Scan(
			&p.ID, &p.Namespace, &p.Type, &p.Version, &p.Platform,
			&p.Filename, &p.DownloadURL, &p.Shasum, &p.SigningKeys,
			&p.S3Key, &p.SizeBytes, &p.Deprecated, &p.Blocked,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan provider: %w", err)
		}
		providers = append(providers, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating providers: %w", err)
	}

	return providers, nil
}

// Update updates a provider
func (r *ProviderRepository) Update(ctx context.Context, p *Provider) error {
	query := `
		UPDATE providers
		SET deprecated = ?, blocked = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	result, err := r.db.conn.ExecContext(ctx, query, p.Deprecated, p.Blocked, p.ID)
	if err != nil {
		return fmt.Errorf("failed to update provider: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("provider not found")
	}

	p.UpdatedAt = time.Now()
	return nil
}

// Delete deletes a provider
func (r *ProviderRepository) Delete(ctx context.Context, id int64) error {
	query := "DELETE FROM providers WHERE id = ?"

	result, err := r.db.conn.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete provider: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("provider not found")
	}

	return nil
}

// Count returns the total number of providers
func (r *ProviderRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM providers").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count providers: %w", err)
	}
	return count, nil
}

// StorageStats represents storage statistics
type StorageStats struct {
	TotalProviders   int64 `json:"total_providers"`
	TotalSizeBytes   int64 `json:"total_size_bytes"`
	UniqueNamespaces int64 `json:"unique_namespaces"`
	UniqueTypes      int64 `json:"unique_types"`
	UniqueVersions   int64 `json:"unique_versions"`
	DeprecatedCount  int64 `json:"deprecated_count"`
	BlockedCount     int64 `json:"blocked_count"`
}

// GetStorageStats returns storage statistics
func (r *ProviderRepository) GetStorageStats(ctx context.Context) (*StorageStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_providers,
			COALESCE(SUM(size_bytes), 0) as total_size_bytes,
			COUNT(DISTINCT namespace) as unique_namespaces,
			COUNT(DISTINCT type) as unique_types,
			COUNT(DISTINCT namespace || '/' || type || '/' || version) as unique_versions,
			SUM(CASE WHEN deprecated = 1 THEN 1 ELSE 0 END) as deprecated_count,
			SUM(CASE WHEN blocked = 1 THEN 1 ELSE 0 END) as blocked_count
		FROM providers
	`

	stats := &StorageStats{}
	err := r.db.conn.QueryRowContext(ctx, query).Scan(
		&stats.TotalProviders,
		&stats.TotalSizeBytes,
		&stats.UniqueNamespaces,
		&stats.UniqueTypes,
		&stats.UniqueVersions,
		&stats.DeprecatedCount,
		&stats.BlockedCount,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage stats: %w", err)
	}

	return stats, nil
}
