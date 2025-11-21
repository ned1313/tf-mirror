package database

import (
	"context"
	"fmt"
	"time"
)

// AuditRepository provides database access for admin action logs
type AuditRepository struct {
	db *DB
}

// NewAuditRepository creates a new audit repository
func NewAuditRepository(db *DB) *AuditRepository {
	return &AuditRepository{db: db}
}

// Log creates a new audit log entry
func (r *AuditRepository) Log(ctx context.Context, action *AdminAction) error {
	query := `
		INSERT INTO admin_actions (user_id, action, resource_type, resource_id, ip_address, 
		                           user_agent, success, error_message, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := r.db.conn.ExecContext(ctx, query,
		action.UserID,
		action.Action,
		action.ResourceType,
		action.ResourceID,
		action.IPAddress,
		action.UserAgent,
		action.Success,
		action.ErrorMessage,
		action.Metadata,
	)
	if err != nil {
		return fmt.Errorf("failed to log action: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get action ID: %w", err)
	}

	action.ID = id
	action.CreatedAt = time.Now()
	return nil
}

// ListByUser retrieves all actions for a user
func (r *AuditRepository) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]*AdminAction, error) {
	query := `
		SELECT id, user_id, action, resource_type, resource_id, ip_address, user_agent, 
		       success, error_message, metadata, created_at
		FROM admin_actions
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.conn.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list actions: %w", err)
	}
	defer rows.Close()

	var actions []*AdminAction
	for rows.Next() {
		var action AdminAction
		if err := rows.Scan(
			&action.ID,
			&action.UserID,
			&action.Action,
			&action.ResourceType,
			&action.ResourceID,
			&action.IPAddress,
			&action.UserAgent,
			&action.Success,
			&action.ErrorMessage,
			&action.Metadata,
			&action.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan action: %w", err)
		}
		actions = append(actions, &action)
	}

	return actions, rows.Err()
}

// ListByResource retrieves all actions for a resource
func (r *AuditRepository) ListByResource(ctx context.Context, resourceType, resourceID string, limit, offset int) ([]*AdminAction, error) {
	query := `
		SELECT id, user_id, action, resource_type, resource_id, ip_address, user_agent, 
		       success, error_message, metadata, created_at
		FROM admin_actions
		WHERE resource_type = ? AND resource_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.conn.QueryContext(ctx, query, resourceType, resourceID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list actions: %w", err)
	}
	defer rows.Close()

	var actions []*AdminAction
	for rows.Next() {
		var action AdminAction
		if err := rows.Scan(
			&action.ID,
			&action.UserID,
			&action.Action,
			&action.ResourceType,
			&action.ResourceID,
			&action.IPAddress,
			&action.UserAgent,
			&action.Success,
			&action.ErrorMessage,
			&action.Metadata,
			&action.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan action: %w", err)
		}
		actions = append(actions, &action)
	}

	return actions, rows.Err()
}

// List retrieves all audit log entries
func (r *AuditRepository) List(ctx context.Context, limit, offset int) ([]*AdminAction, error) {
	query := `
		SELECT id, user_id, action, resource_type, resource_id, ip_address, user_agent, 
		       success, error_message, metadata, created_at
		FROM admin_actions
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.conn.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list actions: %w", err)
	}
	defer rows.Close()

	var actions []*AdminAction
	for rows.Next() {
		var action AdminAction
		if err := rows.Scan(
			&action.ID,
			&action.UserID,
			&action.Action,
			&action.ResourceType,
			&action.ResourceID,
			&action.IPAddress,
			&action.UserAgent,
			&action.Success,
			&action.ErrorMessage,
			&action.Metadata,
			&action.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan action: %w", err)
		}
		actions = append(actions, &action)
	}

	return actions, rows.Err()
}

// ListByAction retrieves all actions of a specific type
func (r *AuditRepository) ListByAction(ctx context.Context, action string, limit, offset int) ([]*AdminAction, error) {
	query := `
		SELECT id, user_id, action, resource_type, resource_id, ip_address, user_agent, 
		       success, error_message, metadata, created_at
		FROM admin_actions
		WHERE action = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.conn.QueryContext(ctx, query, action, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list actions: %w", err)
	}
	defer rows.Close()

	var actions []*AdminAction
	for rows.Next() {
		var act AdminAction
		if err := rows.Scan(
			&act.ID,
			&act.UserID,
			&act.Action,
			&act.ResourceType,
			&act.ResourceID,
			&act.IPAddress,
			&act.UserAgent,
			&act.Success,
			&act.ErrorMessage,
			&act.Metadata,
			&act.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan action: %w", err)
		}
		actions = append(actions, &act)
	}

	return actions, rows.Err()
}

// DeleteOlderThan deletes audit logs older than the specified time
func (r *AuditRepository) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	query := `DELETE FROM admin_actions WHERE created_at < ?`

	result, err := r.db.conn.ExecContext(ctx, query, before)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old actions: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rows, nil
}
