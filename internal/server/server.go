package server

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/ned1313/terraform-mirror/internal/auth"
	"github.com/ned1313/terraform-mirror/internal/cache"
	"github.com/ned1313/terraform-mirror/internal/config"
	"github.com/ned1313/terraform-mirror/internal/database"
	"github.com/ned1313/terraform-mirror/internal/metrics"
	"github.com/ned1313/terraform-mirror/internal/processor"
	"github.com/ned1313/terraform-mirror/internal/provider"
	"github.com/ned1313/terraform-mirror/internal/storage"
)

// Server represents the HTTP server
type Server struct {
	config  *config.Config
	db      *database.DB
	storage storage.Storage
	cache   cache.Cache
	router  *chi.Mux
	server  *http.Server
	logger  *log.Logger
	metrics *metrics.Metrics

	// Services
	authService         *auth.Service
	processorService    *processor.Service
	autoDownloadService *provider.AutoDownloadService

	// Repositories
	providerRepo *database.ProviderRepository
	jobRepo      *database.JobRepository
	auditRepo    *database.AuditRepository
}

// New creates a new HTTP server instance
func New(cfg *config.Config, db *database.DB, storage storage.Storage) *Server {
	return NewWithCache(cfg, db, storage, nil)
}

// NewWithCache creates a new HTTP server instance with an optional cache
func NewWithCache(cfg *config.Config, db *database.DB, storageBackend storage.Storage, c cache.Cache) *Server {
	// Create auth service
	authService := auth.NewService(
		cfg.Auth.JWTSecret,
		cfg.Auth.JWTExpirationHours,
		cfg.Auth.BCryptCost,
	)

	// Create processor service
	processorConfig := processor.Config{
		PollingInterval:    time.Duration(cfg.Processor.PollingIntervalSeconds) * time.Second,
		MaxConcurrentJobs:  cfg.Processor.MaxConcurrentJobs,
		RetryAttempts:      cfg.Processor.RetryAttempts,
		RetryDelay:         time.Duration(cfg.Processor.RetryDelaySeconds) * time.Second,
		WorkerShutdownTime: time.Duration(cfg.Processor.WorkerShutdownSeconds) * time.Second,
	}
	// Default hostname for provider storage keys
	hostname := "registry.terraform.io"
	processorService := processor.NewService(processorConfig, db, storageBackend, hostname)

	// Create auto-download service if enabled
	var autoDownloadSvc *provider.AutoDownloadService
	if cfg.AutoDownload != nil && cfg.AutoDownload.Enabled {
		autoDownloadSvc = provider.NewAutoDownloadService(
			cfg.AutoDownload,
			&cfg.Providers,
			storageBackend,
			db,
		)
		log.Printf("Auto-download enabled: rate limit %d/min, max concurrent %d",
			cfg.AutoDownload.RateLimitPerMinute, cfg.AutoDownload.MaxConcurrentDL)
	}

	// Use NoOp cache if none provided
	if c == nil {
		c = cache.NewNoOpCache()
	}

	// Initialize metrics if telemetry is enabled
	var m *metrics.Metrics
	if cfg.Telemetry.Enabled {
		m = metrics.New()
		log.Printf("Telemetry enabled: metrics available at /metrics")
	}

	s := &Server{
		config:              cfg,
		db:                  db,
		storage:             storageBackend,
		cache:               c,
		logger:              log.Default(),
		metrics:             m,
		authService:         authService,
		processorService:    processorService,
		autoDownloadService: autoDownloadSvc,
		providerRepo:        database.NewProviderRepository(db),
		jobRepo:             database.NewJobRepository(db),
		auditRepo:           database.NewAuditRepository(db),
	}

	s.setupRouter()
	return s
}

// setupRouter initializes the Chi router with all routes and middleware
func (s *Server) setupRouter() {
	r := chi.NewRouter()

	// Standard middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Set a timeout for all requests
	r.Use(middleware.Timeout(60 * time.Second))

	// CORS middleware (if behind proxy)
	if s.config.Server.BehindProxy {
		r.Use(corsMiddleware(s.config.Server.TrustedProxies))
	}

	// Health check endpoint (no auth required)
	r.Get("/health", s.handleHealth)

	// Terraform Provider Network Mirror Protocol endpoints
	r.Route("/.well-known", func(r chi.Router) {
		r.Get("/terraform.json", s.handleServiceDiscovery)
	})

	// Blob download endpoint for local storage (public, no auth)
	// This serves provider files when using local storage instead of S3
	r.Get("/blobs/*", s.handleBlobDownload)

	// Admin UI static files - served from web/dist directory
	// Must be before the catch-all route
	webDir := s.findWebDir()
	if webDir != "" {
		log.Printf("Serving admin UI from: %s", webDir)
		r.Route("/admin", func(r chi.Router) {
			r.Get("/*", s.serveAdminUI(webDir))
		})
	} else {
		log.Printf("Warning: Admin UI static files not found, admin UI will not be available")
	}

	// Metrics endpoint (if telemetry is enabled) - must be before catch-all
	if s.config.Telemetry.Enabled {
		r.Get("/metrics", s.handleMetrics)
	}

	// Provider Network Mirror Protocol endpoints (public, no auth)
	// Pattern: /{hostname}/{namespace}/{type}/index.json
	// Pattern: /{hostname}/{namespace}/{type}/{version}.json
	r.Get("/*", s.handleMirrorCatchAll)

	// Admin API endpoints (authentication required)
	r.Route("/admin/api", func(r chi.Router) {
		// Authentication endpoints (no auth required)
		r.Post("/login", s.handleLogin)
		r.Post("/logout", s.handleLogout)

		// Protected routes (authentication required)
		r.Group(func(r chi.Router) {
			r.Use(s.authMiddleware)

			// Provider management
			r.Post("/providers/load", s.handleLoadProviders)
			r.Get("/providers", s.handleListProviders)
			r.Post("/providers", s.handleUploadProvider)
			r.Get("/providers/{id}", s.handleGetProvider)
			r.Put("/providers/{id}", s.handleUpdateProvider)
			r.Delete("/providers/{id}", s.handleDeleteProvider)

			// Job management
			r.Get("/jobs", s.handleListJobs)
			r.Get("/jobs/{id}", s.handleGetJob)
			r.Post("/jobs/{id}/retry", s.handleRetryJob)
			r.Post("/jobs/{id}/cancel", s.handleCancelJob)

			// Processor status
			r.Get("/processor/status", s.handleProcessorStatus)

			// Statistics
			r.Get("/stats/storage", s.handleStorageStats)
			r.Get("/stats/audit", s.handleAuditLogs)
			r.Get("/stats/cache", s.handleCacheStats)
			r.Post("/stats/recalculate", s.handleRecalculateStats)
			r.Post("/stats/cache/clear", s.handleClearCache)

			// Configuration
			r.Get("/config", s.handleGetConfig)

			// Backup
			r.Post("/backup", s.handleTriggerBackup)
		})
	})

	s.router = r
}

// Start starts the HTTP server
func (s *Server) Start() error {
	// Start the background processor
	if err := s.processorService.Start(context.Background()); err != nil {
		return fmt.Errorf("failed to start processor: %w", err)
	}

	addr := fmt.Sprintf(":%d", s.config.Server.Port)

	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	fmt.Printf("Starting server on %s\n", addr)

	if s.config.Server.TLSEnabled {
		return s.server.ListenAndServeTLS(
			s.config.Server.TLSCertPath,
			s.config.Server.TLSKeyPath,
		)
	}

	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}

	fmt.Println("Shutting down server...")

	// Stop the processor first to prevent new job processing
	if err := s.processorService.Stop(); err != nil {
		s.logger.Printf("Error stopping processor: %v", err)
	}

	// Close the cache
	if s.cache != nil {
		if err := s.cache.Close(); err != nil {
			s.logger.Printf("Error closing cache: %v", err)
		}
	}

	// Shutdown the HTTP server
	return s.server.Shutdown(ctx)
}

// Router returns the underlying Chi router (useful for testing)
func (s *Server) Router() *chi.Mux {
	return s.router
}

// findWebDir looks for the web/dist directory in common locations
func (s *Server) findWebDir() string {
	// List of potential locations for the web/dist directory
	paths := []string{
		"web/dist",       // Running from project root
		"./web/dist",     // Explicit current directory
		"/app/web/dist",  // Docker container path
		"../web/dist",    // Running from cmd/terraform-mirror
		"../../web/dist", // Running from internal/server
	}

	// Get executable path and check relative to it
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		paths = append(paths, filepath.Join(execDir, "web/dist"))
		paths = append(paths, filepath.Join(execDir, "../web/dist"))
	}

	for _, p := range paths {
		absPath, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		if info, err := os.Stat(absPath); err == nil && info.IsDir() {
			// Check if index.html exists
			indexPath := filepath.Join(absPath, "index.html")
			if _, err := os.Stat(indexPath); err == nil {
				return absPath
			}
		}
	}

	return ""
}

// serveAdminUI creates a handler that serves the admin UI static files
func (s *Server) serveAdminUI(webDir string) http.HandlerFunc {
	fileServer := http.FileServer(http.Dir(webDir))

	return func(w http.ResponseWriter, r *http.Request) {
		// Get the path after /admin
		path := strings.TrimPrefix(r.URL.Path, "/admin")
		if path == "" {
			path = "/"
		}

		// Check if the requested file exists
		filePath := filepath.Join(webDir, path)
		if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
			// Serve the file directly
			http.StripPrefix("/admin", fileServer).ServeHTTP(w, r)
			return
		}

		// For SPA routing - serve index.html for all other routes
		http.ServeFile(w, r, filepath.Join(webDir, "index.html"))
	}
}

// handleBlobDownload serves provider binary files from local storage
func (s *Server) handleBlobDownload(w http.ResponseWriter, r *http.Request) {
	// Get the blob key from the URL path (strip /blobs/ prefix)
	key := strings.TrimPrefix(r.URL.Path, "/blobs/")
	if key == "" {
		http.NotFound(w, r)
		return
	}

	// Download from storage
	reader, err := s.storage.Download(r.Context(), key)
	if err != nil {
		s.logger.Printf("Failed to download blob %s: %v", key, err)
		http.NotFound(w, r)
		return
	}
	defer reader.Close()

	// Read the data
	data, err := io.ReadAll(reader)
	if err != nil {
		s.logger.Printf("Failed to read blob %s: %v", key, err)
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	// Set content type based on file extension
	contentType := "application/octet-stream"
	if strings.HasSuffix(key, ".zip") {
		contentType = "application/zip"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(key)))
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}
