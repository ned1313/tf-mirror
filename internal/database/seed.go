package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
)

// CreateInitialAdminUser creates the default admin user if no users exist
// Returns the generated password if a new user was created, or empty string if users already exist
func CreateInitialAdminUser(db *DB, username, password string) error {
	ctx := context.Background()
	userRepo := NewUserRepository(db)

	// Check if any admin users already exist
	var count int
	err := db.conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM admin_users").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check for existing users: %w", err)
	}

	if count > 0 {
		log.Printf("Admin users already exist (%d users), skipping initial user creation", count)
		return nil
	}

	// Create the initial admin user
	user := &AdminUser{
		Username:     username,
		PasswordHash: password, // This will be hashed by the repository
		FullName:     sql.NullString{String: "System Administrator", Valid: true},
		Email:        sql.NullString{String: "admin@localhost", Valid: true},
		Active:       true,
	}

	if err := userRepo.Create(ctx, user); err != nil {
		return fmt.Errorf("failed to create initial admin user: %w", err)
	}

	log.Printf("Created initial admin user: %s", username)
	return nil
}
