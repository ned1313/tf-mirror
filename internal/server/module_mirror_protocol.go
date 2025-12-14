package server

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

// Module Registry Protocol Response Types
// Based on: https://developer.hashicorp.com/terraform/internals/module-registry-protocol

// ModuleVersionsResponse represents the response for listing module versions
type ModuleVersionsResponse struct {
	Modules []ModuleVersionsModule `json:"modules"`
}

// ModuleVersionsModule represents a single module in the versions response
type ModuleVersionsModule struct {
	Versions []ModuleVersion `json:"versions"`
}

// ModuleVersion represents a single version of a module
type ModuleVersion struct {
	Version string `json:"version"`
}

// handleModuleVersions handles GET /v1/modules/{namespace}/{name}/{system}/versions
// Returns all available versions of a module
func (s *Server) handleModuleVersions(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	system := chi.URLParam(r, "system")

	ctx := r.Context()

	// Query database for all versions of this module
	modules, err := s.moduleRepo.ListVersions(ctx, namespace, name, system)
	if err != nil {
		s.logger.Printf("Failed to list module versions for %s/%s/%s: %v", namespace, name, system, err)
		respondError(w, http.StatusInternalServerError, "database_error", "failed to query module versions")
		return
	}

	// If no local modules, try to get versions from upstream registry
	if len(modules) == 0 && s.moduleAutoDownloadService != nil && s.moduleAutoDownloadService.IsEnabled() {
		s.logger.Printf("Module %s/%s/%s not found locally, querying upstream registry for versions",
			namespace, name, system)

		upstreamVersions, err := s.moduleAutoDownloadService.GetAvailableVersions(ctx, namespace, name, system)
		if err != nil {
			s.logger.Printf("Failed to get upstream versions for %s/%s/%s: %v", namespace, name, system, err)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Build response with upstream versions
		versions := make([]ModuleVersion, len(upstreamVersions))
		for i, v := range upstreamVersions {
			versions[i] = ModuleVersion{Version: v}
		}

		response := ModuleVersionsResponse{
			Modules: []ModuleVersionsModule{
				{Versions: versions},
			},
		}

		respondJSON(w, http.StatusOK, response)
		return
	}

	if len(modules) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Build versions list
	versions := make([]ModuleVersion, len(modules))
	for i, m := range modules {
		versions[i] = ModuleVersion{Version: m.Version}
	}

	response := ModuleVersionsResponse{
		Modules: []ModuleVersionsModule{
			{Versions: versions},
		},
	}

	respondJSON(w, http.StatusOK, response)
}

// handleModuleDownload handles GET /v1/modules/{namespace}/{name}/{system}/{version}/download
// Returns a redirect to the module download URL (X-Terraform-Get header)
func (s *Server) handleModuleDownload(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	name := chi.URLParam(r, "name")
	system := chi.URLParam(r, "system")
	version := chi.URLParam(r, "version")

	ctx := r.Context()

	// Check if module exists in database
	module, err := s.moduleRepo.GetByIdentity(ctx, namespace, name, system, version)
	if err != nil {
		s.logger.Printf("Failed to get module %s/%s/%s/%s: %v", namespace, name, system, version, err)
		respondError(w, http.StatusInternalServerError, "database_error", "failed to query module")
		return
	}

	// If module not found, try auto-download
	if module == nil {
		if s.moduleAutoDownloadService != nil && s.moduleAutoDownloadService.IsEnabled() {
			s.logger.Printf("Module %s/%s/%s %s not found in mirror, attempting auto-download",
				namespace, name, system, version)

			downloadedModule, downloadErr := s.moduleAutoDownloadService.DownloadModule(
				ctx, namespace, name, system, version,
			)
			if downloadErr != nil {
				s.logger.Printf("Auto-download failed for %s/%s/%s %s: %v",
					namespace, name, system, version, downloadErr)
				w.WriteHeader(http.StatusNotFound)
				return
			}
			module = downloadedModule
		} else {
			w.WriteHeader(http.StatusNotFound)
			return
		}
	}

	// Get presigned URL for the module
	downloadURL, err := s.storage.GetPresignedURL(ctx, module.S3Key, 1*time.Hour)
	if err != nil {
		s.logger.Printf("Failed to get presigned URL for module %s: %v", module.S3Key, err)
		respondError(w, http.StatusInternalServerError, "storage_error", "failed to generate download URL")
		return
	}

	// Return the download URL via X-Terraform-Get header
	// This is how the module registry protocol works - it returns a 204 with the header
	w.Header().Set("X-Terraform-Get", downloadURL)
	w.WriteHeader(http.StatusNoContent)
}
