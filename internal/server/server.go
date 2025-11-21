package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/ned1313/terraform-mirror/internal/config"
	"github.com/ned1313/terraform-mirror/internal/database"
	"github.com/ned1313/terraform-mirror/internal/storage"
)

// Server represents the HTTP server
type Server struct {
	config  *config.Config
	db      *database.DB
	storage storage.Storage
	router  *chi.Mux
	server  *http.Server
}

// New creates a new HTTP server instance
func New(cfg *config.Config, db *database.DB, storage storage.Storage) *Server {
	s := &Server{
		config:  cfg,
		db:      db,
		storage: storage,
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

	// Provider mirror endpoints (public, no auth)
	r.Route("/v1/providers", func(r chi.Router) {
		r.Get("/{namespace}/{type}/versions", s.handleProviderVersions)
		r.Get("/{namespace}/{type}/{version}/download/{os}/{arch}", s.handleProviderDownload)
	})

	// Admin API endpoints (authentication required)
	r.Route("/admin/api", func(r chi.Router) {
		// TODO: Add authentication middleware
		// r.Use(s.authMiddleware)

		// Authentication
		r.Post("/login", s.handleLogin)
		r.Post("/logout", s.handleLogout)

		// Provider management
		r.Get("/providers", s.handleListProviders)
		r.Post("/providers", s.handleUploadProvider)
		r.Get("/providers/{id}", s.handleGetProvider)
		r.Put("/providers/{id}", s.handleUpdateProvider)
		r.Delete("/providers/{id}", s.handleDeleteProvider)

		// Job management
		r.Get("/jobs", s.handleListJobs)
		r.Get("/jobs/{id}", s.handleGetJob)
		r.Post("/jobs/{id}/retry", s.handleRetryJob)

		// Statistics
		r.Get("/stats/storage", s.handleStorageStats)
		r.Get("/stats/audit", s.handleAuditLogs)

		// Configuration
		r.Get("/config", s.handleGetConfig)

		// Backup
		r.Post("/backup", s.handleTriggerBackup)
	})

	// Metrics endpoint (if telemetry is enabled)
	if s.config.Telemetry.Enabled {
		r.Get("/metrics", s.handleMetrics)
	}

	s.router = r
}

// Start starts the HTTP server
func (s *Server) Start() error {
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
	return s.server.Shutdown(ctx)
}

// Router returns the underlying Chi router (useful for testing)
func (s *Server) Router() *chi.Mux {
	return s.router
}
