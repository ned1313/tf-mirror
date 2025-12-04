package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// UserRepository handles admin user database operations
type UserRepository struct {
	db *DB
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create inserts a new admin user
func (r *UserRepository) Create(ctx context.Context, u *AdminUser) error {
	query := `
		INSERT INTO admin_users (username, password_hash, full_name, email, active)
		VALUES (?, ?, ?, ?, ?)
	`

	result, err := r.db.conn.ExecContext(ctx, query,
		u.Username, u.PasswordHash, u.FullName, u.Email, u.Active,
	)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	u.ID = id
	u.CreatedAt = time.Now()
	u.UpdatedAt = time.Now()
	return nil
}

// GetByID retrieves a user by ID
func (r *UserRepository) GetByID(ctx context.Context, id int64) (*AdminUser, error) {
	query := `
		SELECT id, username, password_hash, full_name, email, active,
			   created_at, updated_at, last_login_at
		FROM admin_users
		WHERE id = ?
	`

	u := &AdminUser{}
	err := r.db.conn.QueryRowContext(ctx, query, id).Scan(
		&u.ID, &u.Username, &u.PasswordHash, &u.FullName, &u.Email, &u.Active,
		&u.CreatedAt, &u.UpdatedAt, &u.LastLoginAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return u, nil
}

// GetByUsername retrieves a user by username
func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*AdminUser, error) {
	query := `
		SELECT id, username, password_hash, full_name, email, active,
			   created_at, updated_at, last_login_at
		FROM admin_users
		WHERE username = ?
	`

	u := &AdminUser{}
	err := r.db.conn.QueryRowContext(ctx, query, username).Scan(
		&u.ID, &u.Username, &u.PasswordHash, &u.FullName, &u.Email, &u.Active,
		&u.CreatedAt, &u.UpdatedAt, &u.LastLoginAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return u, nil
}

// UpdateLastLogin updates the last login timestamp
func (r *UserRepository) UpdateLastLogin(ctx context.Context, id int64) error {
	query := `
		UPDATE admin_users
		SET last_login_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	result, err := r.db.conn.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to update last login: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// Update updates a user
func (r *UserRepository) Update(ctx context.Context, u *AdminUser) error {
	query := `
		UPDATE admin_users
		SET full_name = ?, email = ?, active = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	result, err := r.db.conn.ExecContext(ctx, query,
		u.FullName, u.Email, u.Active, u.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("user not found")
	}

	u.UpdatedAt = time.Now()
	return nil
}

// UpdatePassword updates a user's password hash
func (r *UserRepository) UpdatePassword(ctx context.Context, id int64, passwordHash string) error {
	query := `
		UPDATE admin_users
		SET password_hash = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	result, err := r.db.conn.ExecContext(ctx, query, passwordHash, id)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// List retrieves all users
func (r *UserRepository) List(ctx context.Context) ([]*AdminUser, error) {
	query := `
		SELECT id, username, password_hash, full_name, email, active,
			   created_at, updated_at, last_login_at
		FROM admin_users
		ORDER BY username ASC
	`

	rows, err := r.db.conn.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []*AdminUser
	for rows.Next() {
		u := &AdminUser{}
		if err := rows.Scan(
			&u.ID, &u.Username, &u.PasswordHash, &u.FullName, &u.Email, &u.Active,
			&u.CreatedAt, &u.UpdatedAt, &u.LastLoginAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, u)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating users: %w", err)
	}

	return users, nil
}

// Delete deletes a user
func (r *UserRepository) Delete(ctx context.Context, id int64) error {
	query := "DELETE FROM admin_users WHERE id = ?"

	result, err := r.db.conn.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}
