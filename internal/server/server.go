package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/ned1313/terraform-mirror/internal/auth"
	"github.com/ned1313/terraform-mirror/internal/config"
	"github.com/ned1313/terraform-mirror/internal/database"
	"github.com/ned1313/terraform-mirror/internal/processor"
	"github.com/ned1313/terraform-mirror/internal/storage"
)

// Server represents the HTTP server
type Server struct {
	config  *config.Config
	db      *database.DB
	storage storage.Storage
	router  *chi.Mux
	server  *http.Server
	logger  *log.Logger

	// Services
	authService      *auth.Service
	processorService *processor.Service

	// Repositories
	providerRepo *database.ProviderRepository
	jobRepo      *database.JobRepository
	auditRepo    *database.AuditRepository
}

// New creates a new HTTP server instance
func New(cfg *config.Config, db *database.DB, storage storage.Storage) *Server {
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
	processorService := processor.NewService(processorConfig, db, storage, hostname)

	s := &Server{
		config:           cfg,
		db:               db,
		storage:          storage,
		logger:           log.Default(),
		authService:      authService,
		processorService: processorService,
		providerRepo:     database.NewProviderRepository(db),
		jobRepo:          database.NewJobRepository(db),
		auditRepo:        database.NewAuditRepository(db),
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

	// Provider Registry Protocol endpoints (public, no auth)
	r.Route("/v1/providers", func(r chi.Router) {
		r.Get("/{namespace}/{type}/versions", s.handleProviderVersions)
		r.Get("/{namespace}/{type}/{version}/download/{os}/{arch}", s.handleProviderDownload)
	})

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

			// Processor status
			r.Get("/processor/status", s.handleProcessorStatus)

			// Statistics
			r.Get("/stats/storage", s.handleStorageStats)
			r.Get("/stats/audit", s.handleAuditLogs)

			// Configuration
			r.Get("/config", s.handleGetConfig)

			// Backup
			r.Post("/backup", s.handleTriggerBackup)
		})
	})

	// Metrics endpoint (if telemetry is enabled)
	if s.config.Telemetry.Enabled {
		r.Get("/metrics", s.handleMetrics)
	}

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

	// Shutdown the HTTP server
	return s.server.Shutdown(ctx)
}

// Router returns the underlying Chi router (useful for testing)
func (s *Server) Router() *chi.Mux {
	return s.router
}
