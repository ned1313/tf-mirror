package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/ned1313/terraform-mirror/internal/database"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	dbPath := flag.String("db", "terraform-mirror-dev.db", "Database path")
	username := flag.String("username", "admin", "Admin username")
	password := flag.String("password", "", "New password (required)")
	flag.Parse()

	if *password == "" {
		log.Fatal("Password is required. Use -password flag")
	}

	// Open database
	db, err := database.New(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	userRepo := database.NewUserRepository(db)

	// Find user by username
	user, err := userRepo.GetByUsername(ctx, *username)
	if err != nil {
		log.Fatalf("Failed to find user: %v", err)
	}
	if user == nil {
		log.Fatalf("User '%s' not found", *username)
	}

	// Hash the new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*password), 12)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	// Update password
	err = userRepo.UpdatePassword(ctx, user.ID, string(hashedPassword))
	if err != nil {
		log.Fatalf("Failed to update password: %v", err)
	}

	fmt.Printf("✓ Password updated for user '%s'\n", *username)
}
