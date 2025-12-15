package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/ned1313/terraform-mirror/internal/database"
	"github.com/ned1313/terraform-mirror/internal/provider"
)

// LoadProvidersResponse represents the response after loading providers
type LoadProvidersResponse struct {
	JobID   int64  `json:"job_id"`
	Message string `json:"message"`
	Total   int    `json:"total_providers"`
}

// handleLoadProviders handles the provider definition upload and loading
// POST /admin/api/providers/load
// Accepts multipart/form-data with "file" field containing HCL content
// Creates a job and processes providers, returning the job ID for tracking
func (s *Server) handleLoadProviders(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form (max 10MB)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_form", fmt.Sprintf("Failed to parse form data: %v", err))
		return
	}

	// Get the uploaded file
	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "missing_file", fmt.Sprintf("No file uploaded: %v", err))
		return
	}
	defer file.Close()

	// Validate file extension (optional, but good practice)
	if header.Filename != "" {
		// You could check for .hcl or .tf extension here
		// For now, we'll accept any file
	}

	// Read file content
	content, err := io.ReadAll(file)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "read_error", fmt.Sprintf("Failed to read file: %v", err))
		return
	}

	// Validate content size (max 1MB for HCL definitions)
	if len(content) > 1<<20 {
		respondError(w, http.StatusBadRequest, "file_too_large",
			fmt.Sprintf("File too large (max 1MB, got %d bytes)", len(content)))
		return
	}

	// Parse HCL content
	defs, err := provider.ParseHCL(content)
	if err != nil {
		respondError(w, http.StatusBadRequest, "parse_error", fmt.Sprintf("Failed to parse HCL: %v", err))
		return
	}

	// Validate that we have providers to load
	if len(defs.Providers) == 0 {
		respondError(w, http.StatusBadRequest, "no_providers", "No providers defined in file")
		return
	}

	// Calculate total items (each version+platform combination)
	totalItems := defs.CountItems()

	// Create download job
	job := &database.DownloadJob{
		UserID:     sql.NullInt64{}, // No auth yet, leave null
		SourceType: "hcl",
		SourceData: string(content),
		Status:     "pending",
		Progress:   0,
		TotalItems: totalItems,
		CreatedAt:  time.Now(),
	}

	if err := s.jobRepo.Create(r.Context(), job); err != nil {
		respondError(w, http.StatusInternalServerError, "job_creation_error",
			fmt.Sprintf("Failed to create job: %v", err))
		return
	}

	// Create job items for each provider/version/platform
	for _, providerDef := range defs.Providers {
		for _, version := range providerDef.Versions {
			for _, platform := range providerDef.Platforms {
				item := &database.DownloadJobItem{
					JobID:     job.ID,
					Namespace: providerDef.Namespace,
					Type:      providerDef.Type,
					Version:   version,
					Platform:  platform,
					Status:    "pending",
				}
				if err := s.jobRepo.CreateItem(r.Context(), item); err != nil {
					respondError(w, http.StatusInternalServerError, "job_item_error",
						fmt.Sprintf("Failed to create job item: %v", err))
					return
				}
			}
		}
	}

	// Update job status to running and set start time
	job.Status = "running"
	job.StartedAt = sql.NullTime{Time: time.Now(), Valid: true}
	if err := s.jobRepo.Update(r.Context(), job); err != nil {
		respondError(w, http.StatusInternalServerError, "job_update_error",
			fmt.Sprintf("Failed to update job: %v", err))
		return
	}

	// Log the job creation
	s.logAuditEvent(r, "load_providers", "job", fmt.Sprintf("%d", job.ID), true, "", map[string]interface{}{
		"total_providers": len(defs.Providers),
		"total_items":     totalItems,
	})

	// Return response immediately - job will be processed in the background
	response := LoadProvidersResponse{
		JobID:   job.ID,
		Message: fmt.Sprintf("Provider loading job created: %d providers (%d items)", len(defs.Providers), totalItems),
		Total:   len(defs.Providers),
	}

	// Start async processing in a goroutine
	go s.processProviderLoadJob(job, defs)

	// Return success response immediately
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted) // 202 Accepted - job is queued
	json.NewEncoder(w).Encode(response)
}

// processProviderLoadJob handles provider loading in the background
func (s *Server) processProviderLoadJob(job *database.DownloadJob, defs *provider.ProviderDefinitions) {
	bgCtx := context.Background()

	log.Printf("Starting provider load job %d with %d items", job.ID, job.TotalItems)

	// Create provider service
	providerSvc := provider.NewService(s.storage, s.db)

	// Track progress during processing
	var completedCount, failedCount int

	results, err := providerSvc.LoadFromDefinitionsWithProgress(bgCtx, defs, func(result *provider.LoadResult) {
		// Update job item status
		items, listErr := s.jobRepo.GetItems(bgCtx, job.ID)
		if listErr == nil {
			for _, item := range items {
				if item.Namespace == result.Namespace &&
					item.Type == result.Type &&
					item.Version == result.Version &&
					item.Platform == result.Platform {
					if result.Success {
						item.Status = "completed"
						completedCount++
					} else if result.Error != nil {
						item.Status = "failed"
						item.ErrorMessage = sql.NullString{String: result.Error.Error(), Valid: true}
						failedCount++
					}
					s.jobRepo.UpdateItem(bgCtx, item)
					break
				}
			}
		}

		// Update job progress
		totalProcessed := completedCount + failedCount
		if job.TotalItems > 0 {
			job.Progress = (totalProcessed * 100) / job.TotalItems
		}
		job.CompletedItems = completedCount
		job.FailedItems = failedCount
		s.jobRepo.Update(bgCtx, job)
	})

	if err != nil {
		// Update job status to failed
		job.Status = "failed"
		job.ErrorMessage = sql.NullString{String: err.Error(), Valid: true}
		job.CompletedAt = sql.NullTime{Time: time.Now(), Valid: true}
		s.jobRepo.Update(bgCtx, job)
		log.Printf("Provider load job %d failed: %v", job.ID, err)
		return
	}

	// Calculate final statistics (progress was updated incrementally)
	stats := provider.CalculateStats(results)
	job.CompletedItems = stats.Success
	job.FailedItems = stats.Failed
	job.Progress = 100
	job.Status = "completed"
	job.CompletedAt = sql.NullTime{Time: time.Now(), Valid: true}

	if err := s.jobRepo.Update(bgCtx, job); err != nil {
		log.Printf("Failed to finalize job %d: %v", job.ID, err)
		return
	}

	log.Printf("Provider load job %d completed: %d success, %d failed", job.ID, stats.Success, stats.Failed)
}
