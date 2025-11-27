package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// JobRepository provides database access for download jobs
type JobRepository struct {
	db *DB
}

// NewJobRepository creates a new job repository
func NewJobRepository(db *DB) *JobRepository {
	return &JobRepository{db: db}
}

// Create creates a new download job
func (r *JobRepository) Create(ctx context.Context, job *DownloadJob) error {
	query := `
		INSERT INTO download_jobs (user_id, source_type, source_data, status, total_items, completed_items, failed_items, started_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.conn.ExecContext(ctx, query,
		job.UserID,
		job.SourceType,
		job.SourceData,
		job.Status,
		job.TotalItems,
		job.CompletedItems,
		job.FailedItems,
		job.StartedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get job ID: %w", err)
	}

	job.ID = id
	job.CreatedAt = time.Now()
	return nil
}

// GetByID retrieves a job by ID
func (r *JobRepository) GetByID(ctx context.Context, id int64) (*DownloadJob, error) {
	query := `
		SELECT id, user_id, source_type, source_data, status, progress, total_items, 
		       completed_items, failed_items, error_message, created_at, started_at, completed_at
		FROM download_jobs
		WHERE id = ?
	`

	var job DownloadJob
	err := r.db.conn.QueryRowContext(ctx, query, id).Scan(
		&job.ID,
		&job.UserID,
		&job.SourceType,
		&job.SourceData,
		&job.Status,
		&job.Progress,
		&job.TotalItems,
		&job.CompletedItems,
		&job.FailedItems,
		&job.ErrorMessage,
		&job.CreatedAt,
		&job.StartedAt,
		&job.CompletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	return &job, nil
}

// List retrieves all jobs ordered by creation time
func (r *JobRepository) List(ctx context.Context, limit, offset int) ([]*DownloadJob, error) {
	query := `
		SELECT id, user_id, source_type, source_data, status, progress, total_items, 
		       completed_items, failed_items, error_message, created_at, started_at, completed_at
		FROM download_jobs
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.conn.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*DownloadJob
	for rows.Next() {
		var job DownloadJob
		if err := rows.Scan(
			&job.ID,
			&job.UserID,
			&job.SourceType,
			&job.SourceData,
			&job.Status,
			&job.Progress,
			&job.TotalItems,
			&job.CompletedItems,
			&job.FailedItems,
			&job.ErrorMessage,
			&job.CreatedAt,
			&job.StartedAt,
			&job.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan job: %w", err)
		}
		jobs = append(jobs, &job)
	}

	return jobs, rows.Err()
}

// ListPending retrieves pending jobs ordered by creation time
func (r *JobRepository) ListPending(ctx context.Context, limit int) ([]*DownloadJob, error) {
	query := `
		SELECT id, user_id, source_type, source_data, status, progress, total_items, 
		       completed_items, failed_items, error_message, created_at, started_at, completed_at
		FROM download_jobs
		WHERE status = 'pending'
		ORDER BY created_at ASC
		LIMIT ?
	`

	rows, err := r.db.conn.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list pending jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*DownloadJob
	for rows.Next() {
		var job DownloadJob
		if err := rows.Scan(
			&job.ID,
			&job.UserID,
			&job.SourceType,
			&job.SourceData,
			&job.Status,
			&job.Progress,
			&job.TotalItems,
			&job.CompletedItems,
			&job.FailedItems,
			&job.ErrorMessage,
			&job.CreatedAt,
			&job.StartedAt,
			&job.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan job: %w", err)
		}
		jobs = append(jobs, &job)
	}

	return jobs, rows.Err()
}

// Update updates a job's status and counters
func (r *JobRepository) Update(ctx context.Context, job *DownloadJob) error {
	query := `
		UPDATE download_jobs
		SET status = ?, progress = ?, completed_items = ?, failed_items = ?, 
		    error_message = ?, completed_at = ?
		WHERE id = ?
	`

	result, err := r.db.conn.ExecContext(ctx, query,
		job.Status,
		job.Progress,
		job.CompletedItems,
		job.FailedItems,
		job.ErrorMessage,
		job.CompletedAt,
		job.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("job not found")
	}

	return nil
}

// CreateItem creates a new job item
func (r *JobRepository) CreateItem(ctx context.Context, item *DownloadJobItem) error {
	query := `
		INSERT INTO download_job_items (job_id, namespace, type, version, platform, status)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.conn.ExecContext(ctx, query,
		item.JobID,
		item.Namespace,
		item.Type,
		item.Version,
		item.Platform,
		item.Status,
	)
	if err != nil {
		return fmt.Errorf("failed to create job item: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get job item ID: %w", err)
	}

	item.ID = id
	item.CreatedAt = time.Now()
	return nil
}

// UpdateItem updates a job item
func (r *JobRepository) UpdateItem(ctx context.Context, item *DownloadJobItem) error {
	query := `
		UPDATE download_job_items
		SET status = ?, provider_id = ?, error_message = ?, 
		    download_url = ?, size_bytes = ?, downloaded_bytes = ?, 
		    started_at = ?, completed_at = ?
		WHERE id = ?
	`

	result, err := r.db.conn.ExecContext(ctx, query,
		item.Status,
		item.ProviderID,
		item.ErrorMessage,
		item.DownloadURL,
		item.SizeBytes,
		item.DownloadedBytes,
		item.StartedAt,
		item.CompletedAt,
		item.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update job item: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("job item not found")
	}

	return nil
}

// GetItems retrieves all items for a job
func (r *JobRepository) GetItems(ctx context.Context, jobID int64) ([]*DownloadJobItem, error) {
	query := `
		SELECT id, job_id, namespace, type, version, platform, status, 
		       download_url, size_bytes, downloaded_bytes, provider_id, error_message, 
		       retry_count, created_at, started_at, completed_at
		FROM download_job_items
		WHERE job_id = ?
		ORDER BY created_at ASC
	`

	rows, err := r.db.conn.QueryContext(ctx, query, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get job items: %w", err)
	}
	defer rows.Close()

	var items []*DownloadJobItem
	for rows.Next() {
		var item DownloadJobItem
		if err := rows.Scan(
			&item.ID,
			&item.JobID,
			&item.Namespace,
			&item.Type,
			&item.Version,
			&item.Platform,
			&item.Status,
			&item.DownloadURL,
			&item.SizeBytes,
			&item.DownloadedBytes,
			&item.ProviderID,
			&item.ErrorMessage,
			&item.RetryCount,
			&item.CreatedAt,
			&item.StartedAt,
			&item.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan job item: %w", err)
		}
		items = append(items, &item)
	}

	return items, rows.Err()
}

// CountByStatus counts jobs by status
func (r *JobRepository) CountByStatus(ctx context.Context, status string) (int64, error) {
	query := `SELECT COUNT(*) FROM download_jobs WHERE status = ?`

	var count int64
	err := r.db.conn.QueryRowContext(ctx, query, status).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count jobs: %w", err)
	}

	return count, nil
}

// ResetFailedItems resets all failed items in a job back to pending status
func (r *JobRepository) ResetFailedItems(ctx context.Context, jobID int64) (int64, error) {
	query := `
		UPDATE download_job_items
		SET status = 'pending', 
		    error_message = NULL,
		    started_at = NULL,
		    completed_at = NULL,
		    retry_count = retry_count + 1
		WHERE job_id = ? AND status = 'failed'
	`

	result, err := r.db.conn.ExecContext(ctx, query, jobID)
	if err != nil {
		return 0, fmt.Errorf("failed to reset failed items: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rows, nil
}
