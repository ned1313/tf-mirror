package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ned1313/terraform-mirror/internal/database"
	"github.com/ned1313/terraform-mirror/internal/module"
)

// LoadModulesResponse represents the response after loading modules
type LoadModulesResponse struct {
	JobID   int64  `json:"job_id"`
	Message string `json:"message"`
	Total   int    `json:"total_modules"`
}

// ModuleResponse represents a single module in API responses
type ModuleResponse struct {
	ID                int64     `json:"id"`
	Namespace         string    `json:"namespace"`
	Name              string    `json:"name"`
	System            string    `json:"system"`
	Version           string    `json:"version"`
	S3Key             string    `json:"s3_key"`
	Filename          string    `json:"filename"`
	SizeBytes         int64     `json:"size_bytes"`
	OriginalSourceURL string    `json:"original_source_url,omitempty"`
	Deprecated        bool      `json:"deprecated"`
	Blocked           bool      `json:"blocked"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// ModuleListResponse represents a paginated list of modules
type ModuleListResponse struct {
	Modules    []ModuleResponse `json:"modules"`
	Total      int              `json:"total"`
	Page       int              `json:"page"`
	PageSize   int              `json:"page_size"`
	TotalPages int              `json:"total_pages"`
}

// handleLoadModules handles the module definition upload and loading
// POST /admin/api/modules/load
// Accepts multipart/form-data with "file" field containing HCL content
// Creates a job and processes modules, returning the job ID for tracking
func (s *Server) handleLoadModules(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form (max 10MB)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_form", fmt.Sprintf("Failed to parse form data: %v", err))
		return
	}

	// Get the uploaded file
	file, _, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "missing_file", fmt.Sprintf("No file uploaded: %v", err))
		return
	}
	defer file.Close()

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
	defs, err := module.ParseModuleHCL(content)
	if err != nil {
		respondError(w, http.StatusBadRequest, "parse_error", fmt.Sprintf("Failed to parse HCL: %v", err))
		return
	}

	// Validate that we have modules to load
	if len(defs.Modules) == 0 {
		respondError(w, http.StatusBadRequest, "no_modules", "No modules defined in file")
		return
	}

	// Calculate total items (each module version)
	totalItems := defs.CountItems()

	// Create download job for modules
	job := &database.DownloadJob{
		JobType:    "module",
		UserID:     sql.NullInt64{},
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

	// Create module job items for each module/version
	moduleJobRepo := database.NewModuleJobRepository(s.db)
	for _, moduleDef := range defs.Modules {
		for _, version := range moduleDef.Versions {
			item := &database.ModuleJobItem{
				JobID:     job.ID,
				Namespace: moduleDef.Namespace,
				Name:      moduleDef.Name,
				System:    moduleDef.System,
				Version:   version,
				Status:    "pending",
			}
			if err := moduleJobRepo.CreateItem(r.Context(), item); err != nil {
				respondError(w, http.StatusInternalServerError, "job_item_error",
					fmt.Sprintf("Failed to create job item: %v", err))
				return
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

	// Process modules synchronously
	moduleSvc := module.NewServiceWithRegistry(
		s.storage,
		s.db,
		s.config.Modules.GetUpstreamRegistry(),
		s.config.Modules.MirrorHostname,
	)

	// Load modules
	results, err := moduleSvc.LoadFromDefinitions(r.Context(), defs)
	if err != nil {
		// Update job status to failed
		job.Status = "failed"
		job.ErrorMessage = sql.NullString{String: err.Error(), Valid: true}
		job.CompletedAt = sql.NullTime{Time: time.Now(), Valid: true}
		s.jobRepo.Update(r.Context(), job)

		respondError(w, http.StatusInternalServerError, "load_error",
			fmt.Sprintf("Failed to load modules: %v", err))
		return
	}

	// Update job items based on results
	items, err := moduleJobRepo.ListByJob(r.Context(), job.ID)
	if err == nil {
		for _, result := range results {
			for _, item := range items {
				if item.Namespace == result.Namespace &&
					item.Name == result.Name &&
					item.System == result.System &&
					item.Version == result.Version {
					if result.Success {
						item.Status = "completed"
					} else if result.Error != nil {
						item.Status = "failed"
						item.ErrorMessage = sql.NullString{String: result.Error.Error(), Valid: true}
					}
					moduleJobRepo.UpdateItem(r.Context(), item)
				}
			}
		}
	}

	// Calculate final statistics
	stats := module.CalculateStats(results)
	job.CompletedItems = stats.Success
	job.FailedItems = stats.Failed
	job.Progress = 100
	job.Status = "completed"
	job.CompletedAt = sql.NullTime{Time: time.Now(), Valid: true}

	if err := s.jobRepo.Update(r.Context(), job); err != nil {
		respondError(w, http.StatusInternalServerError, "job_finalize_error",
			fmt.Sprintf("Failed to finalize job: %v", err))
		return
	}

	// Prepare response with job ID
	response := LoadModulesResponse{
		JobID: job.ID,
		Message: fmt.Sprintf("Module loading job created and completed: %d total modules",
			len(defs.Modules)),
		Total: len(defs.Modules),
	}

	// Log successful module load
	s.logAuditEvent(r, "load_modules", "job", fmt.Sprintf("%d", job.ID), true, "", map[string]interface{}{
		"total_modules": len(defs.Modules),
		"total_items":   totalItems,
		"success":       stats.Success,
		"failed":        stats.Failed,
	})

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// handleListModules lists all modules with pagination
// GET /admin/api/modules
func (s *Server) handleListModules(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse pagination parameters
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// Get query parameters for filtering
	namespace := r.URL.Query().Get("namespace")
	name := r.URL.Query().Get("name")
	system := r.URL.Query().Get("system")

	// Get total count for pagination
	total, err := s.moduleRepo.Count(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "database_error", "Failed to count modules")
		return
	}

	// Calculate offset
	offset := (page - 1) * pageSize

	// Get modules from database
	modules, err := s.moduleRepo.List(ctx, pageSize, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "database_error", "Failed to list modules")
		return
	}

	// Filter if parameters provided
	if namespace != "" || name != "" || system != "" {
		filtered := make([]*database.Module, 0)
		for _, m := range modules {
			if namespace != "" && m.Namespace != namespace {
				continue
			}
			if name != "" && m.Name != name {
				continue
			}
			if system != "" && m.System != system {
				continue
			}
			filtered = append(filtered, m)
		}
		modules = filtered
	}

	// Convert to response format
	moduleResponses := make([]ModuleResponse, len(modules))
	for i, m := range modules {
		moduleResponses[i] = moduleToResponse(m)
	}

	// Calculate total pages
	totalPages := (int(total) + pageSize - 1) / pageSize

	response := ModuleListResponse{
		Modules:    moduleResponses,
		Total:      int(total),
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}

	respondJSON(w, http.StatusOK, response)
}

// handleGetModule gets a single module by ID
// GET /admin/api/modules/{id}
func (s *Server) handleGetModule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse module ID from URL
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid_id", "Invalid module ID")
		return
	}

	// Get module from database
	m, err := s.moduleRepo.GetByID(ctx, id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "database_error", "Failed to get module")
		return
	}

	if m == nil {
		respondError(w, http.StatusNotFound, "not_found", "Module not found")
		return
	}

	respondJSON(w, http.StatusOK, moduleToResponse(m))
}

// handleUpdateModule updates a module's metadata
// PUT /admin/api/modules/{id}
func (s *Server) handleUpdateModule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse module ID from URL
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid_id", "Invalid module ID")
		return
	}

	// Get existing module
	m, err := s.moduleRepo.GetByID(ctx, id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "database_error", "Failed to get module")
		return
	}

	if m == nil {
		respondError(w, http.StatusNotFound, "not_found", "Module not found")
		return
	}

	// Parse update request
	var updateReq struct {
		Deprecated *bool `json:"deprecated"`
		Blocked    *bool `json:"blocked"`
	}
	if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON body")
		return
	}

	// Apply updates
	if updateReq.Deprecated != nil {
		m.Deprecated = *updateReq.Deprecated
	}
	if updateReq.Blocked != nil {
		m.Blocked = *updateReq.Blocked
	}

	// Save updates
	if err := s.moduleRepo.Update(ctx, m); err != nil {
		respondError(w, http.StatusInternalServerError, "database_error", "Failed to update module")
		return
	}

	// Log audit event
	s.logAuditEvent(r, "update_module", "module", idStr, true, "", map[string]interface{}{
		"deprecated": m.Deprecated,
		"blocked":    m.Blocked,
	})

	respondJSON(w, http.StatusOK, moduleToResponse(m))
}

// handleDeleteModule deletes a module
// DELETE /admin/api/modules/{id}
func (s *Server) handleDeleteModule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse module ID from URL
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid_id", "Invalid module ID")
		return
	}

	// Get module first to get S3 key
	m, err := s.moduleRepo.GetByID(ctx, id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "database_error", "Failed to get module")
		return
	}

	if m == nil {
		respondError(w, http.StatusNotFound, "not_found", "Module not found")
		return
	}

	// Delete from storage first
	if err := s.storage.Delete(ctx, m.S3Key); err != nil {
		s.logger.Printf("Warning: Failed to delete module from storage: %v", err)
		// Continue anyway - database record is more important to delete
	}

	// Delete from database
	if err := s.moduleRepo.Delete(ctx, id); err != nil {
		respondError(w, http.StatusInternalServerError, "database_error", "Failed to delete module")
		return
	}

	// Log audit event
	s.logAuditEvent(r, "delete_module", "module", idStr, true, "", map[string]interface{}{
		"namespace": m.Namespace,
		"name":      m.Name,
		"system":    m.System,
		"version":   m.Version,
	})

	w.WriteHeader(http.StatusNoContent)
}

// moduleToResponse converts a database Module to a ModuleResponse
func moduleToResponse(m *database.Module) ModuleResponse {
	resp := ModuleResponse{
		ID:         m.ID,
		Namespace:  m.Namespace,
		Name:       m.Name,
		System:     m.System,
		Version:    m.Version,
		S3Key:      m.S3Key,
		Filename:   m.Filename,
		SizeBytes:  m.SizeBytes,
		Deprecated: m.Deprecated,
		Blocked:    m.Blocked,
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
	}
	if m.OriginalSourceURL.Valid {
		resp.OriginalSourceURL = m.OriginalSourceURL.String
	}
	return resp
}
