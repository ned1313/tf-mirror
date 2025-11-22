package server

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ned1313/terraform-mirror/internal/database"
)

// handleMirrorCatchAll is a debug handler to route mirror protocol requests
func (s *Server) handleMirrorCatchAll(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	fmt.Printf("Mirror catchall: %s\n", path)

	// Check if it ends with .json
	if !strings.HasSuffix(path, ".json") {
		http.NotFound(w, r)
		return
	}

	// Remove leading slash and split
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	fmt.Printf("Path parts: %v (len=%d)\n", parts, len(parts))

	if len(parts) == 4 && parts[3] == "index.json" {
		// Handle index.json
		s.handleMirrorProviderVersionsFromParts(w, r, parts[0], parts[1], parts[2])
		return
	}

	if len(parts) == 4 && strings.HasSuffix(parts[3], ".json") {
		// Handle version.json
		version := strings.TrimSuffix(parts[3], ".json")
		s.handleMirrorProviderPackagesFromParts(w, r, parts[0], parts[1], parts[2], version)
		return
	}

	http.NotFound(w, r)
}

func (s *Server) handleMirrorProviderVersionsFromParts(w http.ResponseWriter, r *http.Request, hostname, namespace, providerType string) {
	ctx := r.Context()

	// Query database for all versions of this provider
	providers, err := s.providerRepo.ListVersions(ctx, namespace, providerType)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "database_error", "failed to query provider versions")
		return
	}

	if len(providers) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Build versions map
	versions := make(map[string]interface{})
	for _, p := range providers {
		if _, exists := versions[p.Version]; !exists {
			versions[p.Version] = map[string]interface{}{}
		}
	}

	response := map[string]interface{}{
		"versions": versions,
	}

	respondJSON(w, http.StatusOK, response)
}

func (s *Server) handleMirrorProviderPackagesFromParts(w http.ResponseWriter, r *http.Request, hostname, namespace, providerType, version string) {
	ctx := r.Context()

	// Query database for all platforms of this specific version
	providers, err := s.providerRepo.ListVersions(ctx, namespace, providerType)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "database_error", "failed to query provider")
		return
	}

	// Filter to only the requested version
	var versionProviders []*database.Provider
	for _, p := range providers {
		if p.Version == version {
			versionProviders = append(versionProviders, p)
		}
	}

	if len(versionProviders) == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Build archives map
	archives := make(map[string]interface{})

	for _, p := range versionProviders {
		downloadURL, err := s.storage.GetPresignedURL(ctx, p.S3Key, 24*time.Hour)
		if err != nil {
			continue
		}

		var hashes []string
		if p.Shasum != "" {
			hashes = append(hashes, fmt.Sprintf("zh:%s", p.Shasum))
		}

		archives[p.Platform] = map[string]interface{}{
			"url":    downloadURL,
			"hashes": hashes,
		}
	}

	response := map[string]interface{}{
		"archives": archives,
	}

	respondJSON(w, http.StatusOK, response)
}
