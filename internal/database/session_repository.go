package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SessionRepository provides database access for admin sessions
type SessionRepository struct {
	db *DB
}

// NewSessionRepository creates a new session repository
func NewSessionRepository(db *DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// Create creates a new session
func (r *SessionRepository) Create(ctx context.Context, session *AdminSession) error {
	query := `
		INSERT INTO admin_sessions (user_id, token_jti, ip_address, user_agent, expires_at)
		VALUES (?, ?, ?, ?, ?)
	`

	result, err := r.db.conn.ExecContext(ctx, query,
		session.UserID,
		session.TokenJTI,
		session.IPAddress,
		session.UserAgent,
		session.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get session ID: %w", err)
	}

	session.ID = id
	session.CreatedAt = time.Now()
	return nil
}

// GetByTokenJTI retrieves a session by JWT ID
func (r *SessionRepository) GetByTokenJTI(ctx context.Context, jti string) (*AdminSession, error) {
	query := `
		SELECT id, user_id, token_jti, ip_address, user_agent, created_at, expires_at, revoked
		FROM admin_sessions
		WHERE token_jti = ?
	`

	var session AdminSession
	err := r.db.conn.QueryRowContext(ctx, query, jti).Scan(
		&session.ID,
		&session.UserID,
		&session.TokenJTI,
		&session.IPAddress,
		&session.UserAgent,
		&session.CreatedAt,
		&session.ExpiresAt,
		&session.Revoked,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return &session, nil
}

// GetByID retrieves a session by ID
func (r *SessionRepository) GetByID(ctx context.Context, id int64) (*AdminSession, error) {
	query := `
		SELECT id, user_id, token_jti, ip_address, user_agent, created_at, expires_at, revoked
		FROM admin_sessions
		WHERE id = ?
	`

	var session AdminSession
	err := r.db.conn.QueryRowContext(ctx, query, id).Scan(
		&session.ID,
		&session.UserID,
		&session.TokenJTI,
		&session.IPAddress,
		&session.UserAgent,
		&session.CreatedAt,
		&session.ExpiresAt,
		&session.Revoked,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return &session, nil
}

// ListByUserID retrieves all sessions for a user
func (r *SessionRepository) ListByUserID(ctx context.Context, userID int64) ([]*AdminSession, error) {
	query := `
		SELECT id, user_id, token_jti, ip_address, user_agent, created_at, expires_at, revoked
		FROM admin_sessions
		WHERE user_id = ?
		ORDER BY created_at DESC
	`

	rows, err := r.db.conn.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*AdminSession
	for rows.Next() {
		var session AdminSession
		if err := rows.Scan(
			&session.ID,
			&session.UserID,
			&session.TokenJTI,
			&session.IPAddress,
			&session.UserAgent,
			&session.CreatedAt,
			&session.ExpiresAt,
			&session.Revoked,
		); err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, &session)
	}

	return sessions, rows.Err()
}

// Delete deletes a session
func (r *SessionRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM admin_sessions WHERE id = ?`

	result, err := r.db.conn.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("session not found")
	}

	return nil
}

// DeleteByTokenJTI deletes a session by JWT ID
func (r *SessionRepository) DeleteByTokenJTI(ctx context.Context, jti string) error {
	query := `DELETE FROM admin_sessions WHERE token_jti = ?`

	result, err := r.db.conn.ExecContext(ctx, query, jti)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("session not found")
	}

	return nil
}

// RevokeByTokenJTI revokes a session by JWT ID
func (r *SessionRepository) RevokeByTokenJTI(ctx context.Context, jti string) error {
	query := `UPDATE admin_sessions SET revoked = 1 WHERE token_jti = ?`

	result, err := r.db.conn.ExecContext(ctx, query, jti)
	if err != nil {
		return fmt.Errorf("failed to revoke session: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("session not found")
	}

	return nil
}

// DeleteExpired deletes all expired sessions
func (r *SessionRepository) DeleteExpired(ctx context.Context) (int64, error) {
	query := `DELETE FROM admin_sessions WHERE expires_at < ?`

	result, err := r.db.conn.ExecContext(ctx, query, time.Now())
	if err != nil {
		return 0, fmt.Errorf("failed to delete expired sessions: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rows, nil
}

// DeleteByUserID deletes all sessions for a user
func (r *SessionRepository) DeleteByUserID(ctx context.Context, userID int64) error {
	query := `DELETE FROM admin_sessions WHERE user_id = ?`

	_, err := r.db.conn.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user sessions: %w", err)
	}

	return nil
}
