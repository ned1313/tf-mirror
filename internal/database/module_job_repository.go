package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ModuleJobRepository handles module job item database operations
type ModuleJobRepository struct {
	db *DB
}

// NewModuleJobRepository creates a new module job repository
func NewModuleJobRepository(db *DB) *ModuleJobRepository {
	return &ModuleJobRepository{db: db}
}

// CreateItem creates a new module job item
func (r *ModuleJobRepository) CreateItem(ctx context.Context, item *ModuleJobItem) error {
	query := `
		INSERT INTO module_job_items (job_id, namespace, name, system, version, status)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.conn.ExecContext(ctx, query,
		item.JobID,
		item.Namespace,
		item.Name,
		item.System,
		item.Version,
		item.Status,
	)
	if err != nil {
		return fmt.Errorf("failed to create module job item: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get module job item ID: %w", err)
	}

	item.ID = id
	item.CreatedAt = time.Now()
	return nil
}

// GetItem retrieves a module job item by ID
func (r *ModuleJobRepository) GetItem(ctx context.Context, id int64) (*ModuleJobItem, error) {
	query := `
		SELECT id, job_id, namespace, name, system, version, status,
		       module_id, error_message, retry_count, created_at, completed_at
		FROM module_job_items
		WHERE id = ?
	`

	item := &ModuleJobItem{}
	err := r.db.conn.QueryRowContext(ctx, query, id).Scan(
		&item.ID,
		&item.JobID,
		&item.Namespace,
		&item.Name,
		&item.System,
		&item.Version,
		&item.Status,
		&item.ModuleID,
		&item.ErrorMessage,
		&item.RetryCount,
		&item.CreatedAt,
		&item.CompletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get module job item: %w", err)
	}

	return item, nil
}

// ListByJob retrieves all module job items for a job
func (r *ModuleJobRepository) ListByJob(ctx context.Context, jobID int64) ([]*ModuleJobItem, error) {
	query := `
		SELECT id, job_id, namespace, name, system, version, status,
		       module_id, error_message, retry_count, created_at, completed_at
		FROM module_job_items
		WHERE job_id = ?
		ORDER BY id ASC
	`

	rows, err := r.db.conn.QueryContext(ctx, query, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to list module job items: %w", err)
	}
	defer rows.Close()

	var items []*ModuleJobItem
	for rows.Next() {
		item := &ModuleJobItem{}
		if err := rows.Scan(
			&item.ID,
			&item.JobID,
			&item.Namespace,
			&item.Name,
			&item.System,
			&item.Version,
			&item.Status,
			&item.ModuleID,
			&item.ErrorMessage,
			&item.RetryCount,
			&item.CreatedAt,
			&item.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan module job item: %w", err)
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating module job items: %w", err)
	}

	return items, nil
}

// ListPendingByJob retrieves pending module job items for a job
func (r *ModuleJobRepository) ListPendingByJob(ctx context.Context, jobID int64) ([]*ModuleJobItem, error) {
	query := `
		SELECT id, job_id, namespace, name, system, version, status,
		       module_id, error_message, retry_count, created_at, completed_at
		FROM module_job_items
		WHERE job_id = ? AND status = 'pending'
		ORDER BY id ASC
	`

	rows, err := r.db.conn.QueryContext(ctx, query, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to list pending module job items: %w", err)
	}
	defer rows.Close()

	var items []*ModuleJobItem
	for rows.Next() {
		item := &ModuleJobItem{}
		if err := rows.Scan(
			&item.ID,
			&item.JobID,
			&item.Namespace,
			&item.Name,
			&item.System,
			&item.Version,
			&item.Status,
			&item.ModuleID,
			&item.ErrorMessage,
			&item.RetryCount,
			&item.CreatedAt,
			&item.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan module job item: %w", err)
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating module job items: %w", err)
	}

	return items, nil
}

// UpdateItem updates a module job item
func (r *ModuleJobRepository) UpdateItem(ctx context.Context, item *ModuleJobItem) error {
	query := `
		UPDATE module_job_items
		SET status = ?, module_id = ?, error_message = ?, retry_count = ?, completed_at = ?
		WHERE id = ?
	`

	result, err := r.db.conn.ExecContext(ctx, query,
		item.Status,
		item.ModuleID,
		item.ErrorMessage,
		item.RetryCount,
		item.CompletedAt,
		item.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update module job item: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("module job item not found")
	}

	return nil
}

// CountByStatus counts module job items by status for a job
func (r *ModuleJobRepository) CountByStatus(ctx context.Context, jobID int64) (pending, downloading, completed, failed int, err error) {
	query := `
		SELECT 
			SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) as pending,
			SUM(CASE WHEN status = 'downloading' THEN 1 ELSE 0 END) as downloading,
			SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END) as completed,
			SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed
		FROM module_job_items
		WHERE job_id = ?
	`

	err = r.db.conn.QueryRowContext(ctx, query, jobID).Scan(&pending, &downloading, &completed, &failed)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("failed to count module job items: %w", err)
	}

	return pending, downloading, completed, failed, nil
}

// ResetFailedItems resets failed items to pending status for retry
func (r *ModuleJobRepository) ResetFailedItems(ctx context.Context, jobID int64) (int64, error) {
	query := `
		UPDATE module_job_items
		SET status = 'pending', error_message = NULL, completed_at = NULL
		WHERE job_id = ? AND status = 'failed'
	`

	result, err := r.db.conn.ExecContext(ctx, query, jobID)
	if err != nil {
		return 0, fmt.Errorf("failed to reset failed module items: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return count, nil
}

// DeleteByJob deletes all module job items for a job
func (r *ModuleJobRepository) DeleteByJob(ctx context.Context, jobID int64) error {
	query := "DELETE FROM module_job_items WHERE job_id = ?"

	_, err := r.db.conn.ExecContext(ctx, query, jobID)
	if err != nil {
		return fmt.Errorf("failed to delete module job items: %w", err)
	}

	return nil
}
