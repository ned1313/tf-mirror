package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/ned1313/terraform-mirror/internal/provider"
)

// LoadProvidersResponse represents the response after loading providers
type LoadProvidersResponse struct {
	Message string                 `json:"message"`
	Stats   *provider.LoadStats    `json:"stats"`
	Results []*provider.LoadResult `json:"results,omitempty"`
}

// handleLoadProviders handles the provider definition upload and loading
// POST /admin/api/providers/load
// Accepts multipart/form-data with "file" field containing HCL content
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

	// Create provider service
	providerSvc := provider.NewService(s.storage, s.db)

	// Load providers
	results, err := providerSvc.LoadFromDefinitions(r.Context(), defs)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "load_error", fmt.Sprintf("Failed to load providers: %v", err))
		return
	}

	// Calculate statistics
	stats := provider.CalculateStats(results)

	// Prepare response
	response := LoadProvidersResponse{
		Message: fmt.Sprintf("Provider loading completed: %d total, %d successful, %d failed, %d skipped",
			stats.Total, stats.Success, stats.Failed, stats.Skipped),
		Stats:   stats,
		Results: results,
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
