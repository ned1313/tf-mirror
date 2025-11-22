package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/ned1313/terraform-mirror/internal/database"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	dbPath := flag.String("db", "terraform-mirror-dev.db", "Database path")
	username := flag.String("username", "admin", "Admin username")
	password := flag.String("password", "", "Admin password (required)")
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

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*password), 12)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	// Create the user using the seed function
	err = database.CreateInitialAdminUser(db, *username, string(hashedPassword))
	if err != nil {
		log.Fatalf("Failed to create admin user: %v", err)
	}

	fmt.Printf("✓ Admin user '%s' created successfully\n", *username)
}
