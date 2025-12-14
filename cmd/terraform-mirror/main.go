package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ned1313/terraform-mirror/internal/cache"
	"github.com/ned1313/terraform-mirror/internal/config"
	"github.com/ned1313/terraform-mirror/internal/database"
	"github.com/ned1313/terraform-mirror/internal/server"
	"github.com/ned1313/terraform-mirror/internal/storage"
	"github.com/ned1313/terraform-mirror/internal/version"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	// Check for subcommands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "healthcheck":
			os.Exit(runHealthCheck())
		case "version":
			fmt.Printf("Terraform Mirror %s (built %s, commit %s)\n",
				version.Version, version.BuildTime, version.GitCommit)
			os.Exit(0)
		}
	}

	log.Printf("Starting Terraform Mirror %s (built %s, commit %s)\n",
		version.Version, version.BuildTime, version.GitCommit)

	// Parse command line flags
	configPath := flag.String("config", "", "Path to configuration file (HCL)")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Configuration loaded successfully")
	log.Printf("Server will listen on port %d", cfg.Server.Port)
	log.Printf("Storage: %s (bucket: %s)", cfg.Storage.Type, cfg.Storage.Bucket)
	log.Printf("Database: %s", cfg.Database.Path)
	log.Printf("Cache: %dMB memory, %dGB disk", cfg.Cache.MemorySizeMB, cfg.Cache.DiskSizeGB)

	// Initialize database
	db, err := database.New(cfg.Database.Path)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Create initial admin user from environment variables if provided
	adminUsername := os.Getenv("TFM_ADMIN_USERNAME")
	adminPassword := os.Getenv("TFM_ADMIN_PASSWORD")
	if adminUsername != "" && adminPassword != "" {
		// Hash the password before storing
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(adminPassword), 12)
		if err != nil {
			log.Printf("Warning: Failed to hash admin password: %v", err)
		} else {
			if err := database.CreateInitialAdminUser(db, adminUsername, string(hashedPassword)); err != nil {
				log.Printf("Warning: Failed to create initial admin user: %v", err)
			}
		}
	}

	// Initialize storage
	ctx := context.Background()

	// Construct base URL for local storage to serve files via HTTP
	var storageBaseURL string
	if cfg.Storage.Type == "local" {
		scheme := "http"
		if cfg.Server.TLSEnabled {
			scheme = "https"
		}
		// Use localhost for local development; in production this should be configured
		storageBaseURL = fmt.Sprintf("%s://localhost:%d", scheme, cfg.Server.Port)
	}

	store, err := storage.NewFromConfigWithBaseURL(ctx, cfg.Storage, storageBaseURL)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()

	// Initialize cache
	var cacheInstance cache.Cache
	if cfg.Cache.MemorySizeMB > 0 || (cfg.Cache.DiskPath != "" && cfg.Cache.DiskSizeGB > 0) {
		cacheInstance, err = cache.NewFromConfig(cfg.Cache)
		if err != nil {
			log.Printf("Warning: Failed to initialize cache, running without cache: %v", err)
			cacheInstance = cache.NewNoOpCache()
		} else {
			log.Printf("Cache initialized: %dMB memory, %dGB disk at %s",
				cfg.Cache.MemorySizeMB, cfg.Cache.DiskSizeGB, cfg.Cache.DiskPath)
		}
	} else {
		log.Printf("Cache disabled (no memory or disk size configured)")
		cacheInstance = cache.NewNoOpCache()
	}

	// Initialize server
	srv := server.NewWithCache(cfg, db, store, cacheInstance)

	// Start server in goroutine
	go func() {
		if err := srv.Start(); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped")
}

// runHealthCheck performs a health check against the local server
func runHealthCheck() int {
	port := os.Getenv("TFM_SERVER_PORT")
	if port == "" {
		port = "8080"
	}

	url := fmt.Sprintf("http://localhost:%s/health", port)

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Health check failed: %v\n", err)
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Health check failed: status %d\n", resp.StatusCode)
		return 1
	}

	return 0
}
