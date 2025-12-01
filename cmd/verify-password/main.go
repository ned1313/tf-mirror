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
	password := flag.String("password", "admin123", "Password to verify")
	flag.Parse()

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

	fmt.Printf("User found: ID=%d, Username=%s, Active=%v\n", user.ID, user.Username, user.Active)
	fmt.Printf("Password hash (first 60 chars): %s\n", user.PasswordHash[:min(60, len(user.PasswordHash))])

	// Verify password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(*password))
	if err != nil {
		fmt.Printf("❌ Password verification FAILED: %v\n", err)
	} else {
		fmt.Printf("✓ Password verification SUCCESS\n")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
