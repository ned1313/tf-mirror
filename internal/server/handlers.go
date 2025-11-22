package server

import (
	"encoding/json"
	"net/http"
	"strconv"

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

// handleGetProvider gets a specific provider
// TODO: Implement full logic
func (s *Server) handleGetProvider(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{
		"error": "not implemented yet",
	})
}

// handleUpdateProvider updates a provider
// TODO: Implement full logic
func (s *Server) handleUpdateProvider(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{
		"error": "not implemented yet",
	})
}

// handleDeleteProvider deletes a provider
// TODO: Implement full logic
func (s *Server) handleDeleteProvider(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{
		"error": "not implemented yet",
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

// handleRetryJob retries a failed job
// TODO: Implement full logic
func (s *Server) handleRetryJob(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{
		"error": "not implemented yet",
	})
}

// handleStorageStats returns storage statistics
// TODO: Implement full logic
func (s *Server) handleStorageStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{
		"error": "not implemented yet",
	})
}

// handleAuditLogs returns audit logs
// TODO: Implement full logic
func (s *Server) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{
		"error": "not implemented yet",
	})
}

// handleGetConfig returns configuration (sanitized)
// TODO: Implement full logic
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{
		"error": "not implemented yet",
	})
}

// handleTriggerBackup triggers a database backup
// TODO: Implement full logic
func (s *Server) handleTriggerBackup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	json.NewEncoder(w).Encode(map[string]string{
		"error": "not implemented yet",
	})
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
