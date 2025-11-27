package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ned1313/terraform-mirror/internal/database"
)

// Health check response
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// jobResponse represents a download job with its items
type jobResponse struct {
	ID             int64             `json:"id"`
	SourceType     string            `json:"source_type"`
	Status         string            `json:"status"`
	Progress       int               `json:"progress"`
	TotalItems     int               `json:"total_items"`
	CompletedItems int               `json:"completed_items"`
	FailedItems    int               `json:"failed_items"`
	ErrorMessage   *string           `json:"error_message,omitempty"`
	CreatedAt      string            `json:"created_at"`
	StartedAt      *string           `json:"started_at,omitempty"`
	CompletedAt    *string           `json:"completed_at,omitempty"`
	Items          []jobItemResponse `json:"items,omitempty"`
}

// jobItemResponse represents a single download job item
type jobItemResponse struct {
	ID           int64   `json:"id"`
	Namespace    string  `json:"namespace"`
	Type         string  `json:"type"`
	Version      string  `json:"version"`
	Platform     string  `json:"platform"`
	Status       string  `json:"status"`
	ErrorMessage *string `json:"error_message,omitempty"`
}

// jobListResponse represents a list of jobs
type jobListResponse struct {
	Jobs   []jobResponse `json:"jobs"`
	Total  int           `json:"total"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
}

// convertJobToResponse converts database models to API response
func convertJobToResponse(job *database.DownloadJob, items []*database.DownloadJobItem) jobResponse {
	response := jobResponse{
		ID:             job.ID,
		SourceType:     job.SourceType,
		Status:         job.Status,
		Progress:       job.Progress,
		TotalItems:     job.TotalItems,
		CompletedItems: job.CompletedItems,
		FailedItems:    job.FailedItems,
		CreatedAt:      job.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if job.ErrorMessage.Valid {
		response.ErrorMessage = &job.ErrorMessage.String
	}

	if job.StartedAt.Valid {
		startedStr := job.StartedAt.Time.Format("2006-01-02T15:04:05Z07:00")
		response.StartedAt = &startedStr
	}

	if job.CompletedAt.Valid {
		completedStr := job.CompletedAt.Time.Format("2006-01-02T15:04:05Z07:00")
		response.CompletedAt = &completedStr
	}

	// Convert items if provided
	if items != nil {
		response.Items = make([]jobItemResponse, len(items))
		for i, item := range items {
			itemResponse := jobItemResponse{
				ID:        item.ID,
				Namespace: item.Namespace,
				Type:      item.Type,
				Version:   item.Version,
				Platform:  item.Platform,
				Status:    item.Status,
			}
			if item.ErrorMessage.Valid {
				itemResponse.ErrorMessage = &item.ErrorMessage.String
			}
			response.Items[i] = itemResponse
		}
	}

	return response
}

// handleHealth returns the health status of the server
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status:  "healthy",
		Version: "0.1.0",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// Service discovery response for Terraform
type ServiceDiscoveryResponse struct {
	ProvidersV1 string `json:"providers.v1"`
}

// handleServiceDiscovery implements the .well-known/terraform.json endpoint
func (s *Server) handleServiceDiscovery(w http.ResponseWriter, r *http.Request) {
	response := ServiceDiscoveryResponse{
		ProvidersV1: "/v1/providers/",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// Provider Mirror Protocol handlers are now in provider_mirror.go
// Authentication handlers are now in auth_handlers.go

// handleListProviders lists all providers
// TODO: Implement full logic
func (s *Server) handleListProviders(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get query parameters for filtering
	namespace := r.URL.Query().Get("namespace")
	providerType := r.URL.Query().Get("type")

	// Get all providers from database (with a reasonable limit)
	providers, err := s.providerRepo.List(ctx, 1000, 0)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "database_error", "Failed to list providers")
		return
	}

	// Ensure providers is never nil (empty slice instead)
	if providers == nil {
		providers = make([]*database.Provider, 0)
	}

	// Filter if parameters provided
	filtered := providers
	if namespace != "" || providerType != "" {
		filtered = make([]*database.Provider, 0)
		for _, p := range providers {
			if namespace != "" && p.Namespace != namespace {
				continue
			}
			if providerType != "" && p.Type != providerType {
				continue
			}
			filtered = append(filtered, p)
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"providers": filtered,
		"count":     len(filtered),
	})
}

// handleUploadProvider handles provider upload
// TODO: Implement full logic
func (s *Server) handleUploadProvider(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{
		"error": "not implemented yet",
	})
}

// handleGetProvider gets a specific provider by ID
// GET /admin/api/providers/{id}
func (s *Server) handleGetProvider(w http.ResponseWriter, r *http.Request) {
	// Get provider ID from URL
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid_id", "Invalid provider ID")
		return
	}

	// Get provider from database
	provider, err := s.providerRepo.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "database_error", "Failed to get provider")
		return
	}
	if provider == nil {
		respondError(w, http.StatusNotFound, "not_found", "Provider not found")
		return
	}

	respondJSON(w, http.StatusOK, provider)
}

// UpdateProviderRequest represents the request body for updating a provider
type UpdateProviderRequest struct {
	Deprecated *bool `json:"deprecated,omitempty"`
	Blocked    *bool `json:"blocked,omitempty"`
}

// handleUpdateProvider updates a provider (deprecated/blocked status)
// PUT /admin/api/providers/{id}
func (s *Server) handleUpdateProvider(w http.ResponseWriter, r *http.Request) {
	// Get provider ID from URL
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid_id", "Invalid provider ID")
		return
	}

	// Parse request body
	var req UpdateProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_body", "Invalid request body")
		return
	}

	// Get existing provider
	provider, err := s.providerRepo.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "database_error", "Failed to get provider")
		return
	}
	if provider == nil {
		respondError(w, http.StatusNotFound, "not_found", "Provider not found")
		return
	}

	// Update fields if provided
	if req.Deprecated != nil {
		provider.Deprecated = *req.Deprecated
	}
	if req.Blocked != nil {
		provider.Blocked = *req.Blocked
	}

	// Save changes
	if err := s.providerRepo.Update(r.Context(), provider); err != nil {
		respondError(w, http.StatusInternalServerError, "database_error", "Failed to update provider")
		return
	}

	respondJSON(w, http.StatusOK, provider)
}

// handleDeleteProvider deletes a provider
// DELETE /admin/api/providers/{id}
func (s *Server) handleDeleteProvider(w http.ResponseWriter, r *http.Request) {
	// Get provider ID from URL
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid_id", "Invalid provider ID")
		return
	}

	// Get provider first (to get S3 key for cleanup)
	provider, err := s.providerRepo.GetByID(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "database_error", "Failed to get provider")
		return
	}
	if provider == nil {
		respondError(w, http.StatusNotFound, "not_found", "Provider not found")
		return
	}

	// Delete from storage if S3 key exists
	if provider.S3Key != "" {
		if err := s.storage.Delete(r.Context(), provider.S3Key); err != nil {
			// Log but don't fail - the storage file might already be gone
			s.logger.Printf("Warning: failed to delete storage object %s: %v", provider.S3Key, err)
		}
	}

	// Delete from database
	if err := s.providerRepo.Delete(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, "database_error", "Failed to delete provider")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Provider deleted successfully",
	})
}

// handleListJobs retrieves all jobs with pagination
// GET /admin/api/jobs?limit=10&offset=0
func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 10 // default
	offset := 0

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Get jobs from database
	jobs, err := s.jobRepo.List(r.Context(), limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "database_error",
			"Failed to retrieve jobs")
		return
	}

	// Convert to response format (without items for list view)
	jobResponses := make([]jobResponse, len(jobs))
	for i, job := range jobs {
		jobResponses[i] = convertJobToResponse(job, nil)
	}

	response := jobListResponse{
		Jobs:   jobResponses,
		Total:  len(jobs),
		Limit:  limit,
		Offset: offset,
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleGetJob retrieves a specific job by ID
// GET /admin/api/jobs/{id}
func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	// Get job ID from URL
	jobIDStr := chi.URLParam(r, "id")
	jobID, err := strconv.ParseInt(jobIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid_job_id", "Invalid job ID")
		return
	}

	// Get job from database
	job, err := s.jobRepo.GetByID(r.Context(), jobID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "database_error",
			"Failed to retrieve job")
		return
	}
	if job == nil {
		respondError(w, http.StatusNotFound, "job_not_found", "Job not found")
		return
	}

	// Get job items
	items, err := s.jobRepo.GetItems(r.Context(), jobID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "database_error",
			"Failed to retrieve job items")
		return
	}

	// Convert to response format
	response := convertJobToResponse(job, items)

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleRetryJob retries failed items in a job
// POST /admin/api/jobs/{id}/retry
func (s *Server) handleRetryJob(w http.ResponseWriter, r *http.Request) {
	// Get job ID from URL
	idStr := chi.URLParam(r, "id")
	jobID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid_id", "Invalid job ID")
		return
	}

	// Get job from database
	job, err := s.jobRepo.GetByID(r.Context(), jobID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "database_error", "Failed to get job")
		return
	}
	if job == nil {
		respondError(w, http.StatusNotFound, "not_found", "Job not found")
		return
	}

	// Only allow retry on completed or failed jobs
	if job.Status != "completed" && job.Status != "failed" {
		respondError(w, http.StatusBadRequest, "invalid_status",
			"Can only retry jobs that are completed or failed")
		return
	}

	// Check if there are any failed items to retry
	if job.FailedItems == 0 {
		respondError(w, http.StatusBadRequest, "no_failed_items",
			"No failed items to retry")
		return
	}

	// Reset failed items to pending
	resetCount, err := s.jobRepo.ResetFailedItems(r.Context(), jobID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "database_error", "Failed to reset failed items")
		return
	}

	// Update job status to running
	job.Status = "running"
	job.FailedItems = job.FailedItems - int(resetCount)
	job.Progress = (job.CompletedItems * 100) / job.TotalItems
	job.ErrorMessage.Valid = false
	job.CompletedAt.Valid = false

	if err := s.jobRepo.Update(r.Context(), job); err != nil {
		respondError(w, http.StatusInternalServerError, "database_error", "Failed to update job")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":     "Job retry started",
		"reset_count": resetCount,
		"job_id":      jobID,
	})
}

// StorageStatsResponse represents storage statistics
type StorageStatsResponse struct {
	TotalProviders   int64  `json:"total_providers"`
	TotalSizeBytes   int64  `json:"total_size_bytes"`
	TotalSizeHuman   string `json:"total_size_human"`
	UniqueNamespaces int64  `json:"unique_namespaces"`
	UniqueTypes      int64  `json:"unique_types"`
	UniqueVersions   int64  `json:"unique_versions"`
	DeprecatedCount  int64  `json:"deprecated_count"`
	BlockedCount     int64  `json:"blocked_count"`
}

// formatBytes converts bytes to human-readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// handleStorageStats returns storage statistics
// GET /admin/api/stats/storage
func (s *Server) handleStorageStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.providerRepo.GetStorageStats(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "database_error", "Failed to get storage stats")
		return
	}

	response := StorageStatsResponse{
		TotalProviders:   stats.TotalProviders,
		TotalSizeBytes:   stats.TotalSizeBytes,
		TotalSizeHuman:   formatBytes(stats.TotalSizeBytes),
		UniqueNamespaces: stats.UniqueNamespaces,
		UniqueTypes:      stats.UniqueTypes,
		UniqueVersions:   stats.UniqueVersions,
		DeprecatedCount:  stats.DeprecatedCount,
		BlockedCount:     stats.BlockedCount,
	}

	respondJSON(w, http.StatusOK, response)
}

// AuditLogResponse represents the audit log response
type AuditLogResponse struct {
	Logs   []AuditLogEntry `json:"logs"`
	Total  int             `json:"total"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}

// AuditLogEntry represents a single audit log entry
type AuditLogEntry struct {
	ID           int64   `json:"id"`
	UserID       *int64  `json:"user_id,omitempty"`
	Action       string  `json:"action"`
	ResourceType string  `json:"resource_type"`
	ResourceID   *string `json:"resource_id,omitempty"`
	IPAddress    *string `json:"ip_address,omitempty"`
	Success      bool    `json:"success"`
	ErrorMessage *string `json:"error_message,omitempty"`
	CreatedAt    string  `json:"created_at"`
}

// handleAuditLogs returns audit logs with filtering
// GET /admin/api/stats/audit?action=login&limit=50&offset=0
func (s *Server) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	actionFilter := r.URL.Query().Get("action")
	resourceType := r.URL.Query().Get("resource_type")
	resourceID := r.URL.Query().Get("resource_id")

	limit := 50 // default
	offset := 0

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 500 {
			limit = l
		}
	}

	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	var logs []*database.AdminAction
	var err error

	// Apply filters
	switch {
	case actionFilter != "":
		logs, err = s.auditRepo.ListByAction(r.Context(), actionFilter, limit, offset)
	case resourceType != "" && resourceID != "":
		logs, err = s.auditRepo.ListByResource(r.Context(), resourceType, resourceID, limit, offset)
	default:
		logs, err = s.auditRepo.List(r.Context(), limit, offset)
	}

	if err != nil {
		respondError(w, http.StatusInternalServerError, "database_error", "Failed to get audit logs")
		return
	}

	// Convert to response format
	entries := make([]AuditLogEntry, len(logs))
	for i, log := range logs {
		entry := AuditLogEntry{
			ID:           log.ID,
			Action:       log.Action,
			ResourceType: log.ResourceType,
			Success:      log.Success,
			CreatedAt:    log.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}

		if log.UserID.Valid {
			uid := log.UserID.Int64
			entry.UserID = &uid
		}
		if log.ResourceID.Valid {
			entry.ResourceID = &log.ResourceID.String
		}
		if log.IPAddress.Valid {
			entry.IPAddress = &log.IPAddress.String
		}
		if log.ErrorMessage.Valid {
			entry.ErrorMessage = &log.ErrorMessage.String
		}

		entries[i] = entry
	}

	respondJSON(w, http.StatusOK, AuditLogResponse{
		Logs:   entries,
		Total:  len(entries),
		Limit:  limit,
		Offset: offset,
	})
}

// SanitizedConfig represents configuration without secrets
type SanitizedConfig struct {
	Server    SanitizedServerConfig    `json:"server"`
	Storage   SanitizedStorageConfig   `json:"storage"`
	Database  SanitizedDatabaseConfig  `json:"database"`
	Cache     SanitizedCacheConfig     `json:"cache"`
	Features  SanitizedFeaturesConfig  `json:"features"`
	Processor SanitizedProcessorConfig `json:"processor"`
	Logging   SanitizedLoggingConfig   `json:"logging"`
	Telemetry SanitizedTelemetryConfig `json:"telemetry"`
}

type SanitizedServerConfig struct {
	Port        int  `json:"port"`
	TLSEnabled  bool `json:"tls_enabled"`
	BehindProxy bool `json:"behind_proxy"`
}

type SanitizedStorageConfig struct {
	Type           string `json:"type"`
	Bucket         string `json:"bucket"`
	Region         string `json:"region"`
	Endpoint       string `json:"endpoint,omitempty"`
	ForcePathStyle bool   `json:"force_path_style"`
}

type SanitizedDatabaseConfig struct {
	Path                string `json:"path"`
	BackupEnabled       bool   `json:"backup_enabled"`
	BackupIntervalHours int    `json:"backup_interval_hours"`
	BackupToS3          bool   `json:"backup_to_s3"`
}

type SanitizedCacheConfig struct {
	MemorySizeMB int    `json:"memory_size_mb"`
	DiskPath     string `json:"disk_path"`
	DiskSizeGB   int    `json:"disk_size_gb"`
	TTLSeconds   int    `json:"ttl_seconds"`
}

type SanitizedFeaturesConfig struct {
	AutoDownloadProviders bool `json:"auto_download_providers"`
	AutoDownloadModules   bool `json:"auto_download_modules"`
	MaxDownloadSizeMB     int  `json:"max_download_size_mb"`
}

type SanitizedProcessorConfig struct {
	PollingIntervalSeconds int `json:"polling_interval_seconds"`
	MaxConcurrentJobs      int `json:"max_concurrent_jobs"`
	RetryAttempts          int `json:"retry_attempts"`
	RetryDelaySeconds      int `json:"retry_delay_seconds"`
}

type SanitizedLoggingConfig struct {
	Level  string `json:"level"`
	Format string `json:"format"`
	Output string `json:"output"`
}

type SanitizedTelemetryConfig struct {
	Enabled       bool `json:"enabled"`
	OtelEnabled   bool `json:"otel_enabled"`
	ExportTraces  bool `json:"export_traces"`
	ExportMetrics bool `json:"export_metrics"`
}

// handleGetConfig returns configuration (sanitized - no secrets)
// GET /admin/api/config
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	sanitized := SanitizedConfig{
		Server: SanitizedServerConfig{
			Port:        s.config.Server.Port,
			TLSEnabled:  s.config.Server.TLSEnabled,
			BehindProxy: s.config.Server.BehindProxy,
		},
		Storage: SanitizedStorageConfig{
			Type:           s.config.Storage.Type,
			Bucket:         s.config.Storage.Bucket,
			Region:         s.config.Storage.Region,
			Endpoint:       s.config.Storage.Endpoint,
			ForcePathStyle: s.config.Storage.ForcePathStyle,
		},
		Database: SanitizedDatabaseConfig{
			Path:                s.config.Database.Path,
			BackupEnabled:       s.config.Database.BackupEnabled,
			BackupIntervalHours: s.config.Database.BackupIntervalHours,
			BackupToS3:          s.config.Database.BackupToS3,
		},
		Cache: SanitizedCacheConfig{
			MemorySizeMB: s.config.Cache.MemorySizeMB,
			DiskPath:     s.config.Cache.DiskPath,
			DiskSizeGB:   s.config.Cache.DiskSizeGB,
			TTLSeconds:   s.config.Cache.TTLSeconds,
		},
		Features: SanitizedFeaturesConfig{
			AutoDownloadProviders: s.config.Features.AutoDownloadProviders,
			AutoDownloadModules:   s.config.Features.AutoDownloadModules,
			MaxDownloadSizeMB:     s.config.Features.MaxDownloadSizeMB,
		},
		Processor: SanitizedProcessorConfig{
			PollingIntervalSeconds: s.config.Processor.PollingIntervalSeconds,
			MaxConcurrentJobs:      s.config.Processor.MaxConcurrentJobs,
			RetryAttempts:          s.config.Processor.RetryAttempts,
			RetryDelaySeconds:      s.config.Processor.RetryDelaySeconds,
		},
		Logging: SanitizedLoggingConfig{
			Level:  s.config.Logging.Level,
			Format: s.config.Logging.Format,
			Output: s.config.Logging.Output,
		},
		Telemetry: SanitizedTelemetryConfig{
			Enabled:       s.config.Telemetry.Enabled,
			OtelEnabled:   s.config.Telemetry.OtelEnabled,
			ExportTraces:  s.config.Telemetry.ExportTraces,
			ExportMetrics: s.config.Telemetry.ExportMetrics,
		},
	}

	respondJSON(w, http.StatusOK, sanitized)
}

// BackupResponse represents the backup response
type BackupResponse struct {
	Message    string `json:"message"`
	BackupPath string `json:"backup_path,omitempty"`
	S3Key      string `json:"s3_key,omitempty"`
	SizeBytes  int64  `json:"size_bytes"`
	CreatedAt  string `json:"created_at"`
}

// handleTriggerBackup triggers a database backup
// POST /admin/api/backup
func (s *Server) handleTriggerBackup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Check if backup is enabled
	if !s.config.Database.BackupEnabled {
		respondError(w, http.StatusBadRequest, "backup_disabled", "Database backup is disabled in configuration")
		return
	}

	// Generate backup filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	backupFilename := fmt.Sprintf("terraform-mirror-backup-%s.db", timestamp)

	// Create local backup first
	localBackupPath := filepath.Join(filepath.Dir(s.config.Database.Path), "backups", backupFilename)

	if err := s.db.Backup(ctx, localBackupPath); err != nil {
		respondError(w, http.StatusInternalServerError, "backup_error", "Failed to create backup: "+err.Error())
		return
	}

	// Get backup file size
	fileInfo, err := os.Stat(localBackupPath)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "backup_error", "Failed to stat backup file")
		return
	}

	response := BackupResponse{
		Message:    "Backup created successfully",
		BackupPath: localBackupPath,
		SizeBytes:  fileInfo.Size(),
		CreatedAt:  time.Now().Format("2006-01-02T15:04:05Z07:00"),
	}

	// Upload to S3 if configured
	if s.config.Database.BackupToS3 {
		file, err := os.Open(localBackupPath)
		if err != nil {
			s.logger.Printf("Warning: failed to open backup file for S3 upload: %v", err)
		} else {
			defer file.Close()

			s3Key := filepath.Join(s.config.Database.BackupS3Prefix, backupFilename)
			if err := s.storage.Upload(ctx, s3Key, file, "application/octet-stream", map[string]string{
				"backup-type": "manual",
				"created-at":  timestamp,
			}); err != nil {
				s.logger.Printf("Warning: failed to upload backup to S3: %v", err)
			} else {
				response.S3Key = s3Key
				response.Message = "Backup created and uploaded to S3 successfully"
			}
		}
	}

	respondJSON(w, http.StatusOK, response)
}

// handleMetrics returns Prometheus metrics
// TODO: Implement full logic
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusNotImplemented)
	w.Write([]byte("# Metrics not implemented yet\n"))
}

// handleProcessorStatus returns the current processor status
func (s *Server) handleProcessorStatus(w http.ResponseWriter, r *http.Request) {
	status := s.processorService.GetStatus()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}
