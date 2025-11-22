package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Provider Mirror Protocol Response Types
// Based on: https://developer.hashicorp.com/terraform/internals/provider-network-mirror-protocol

// ProviderVersionsResponse represents the response for listing provider versions
type ProviderVersionsResponse struct {
	Versions map[string]VersionInfo `json:"versions"`
}

// VersionInfo contains metadata about a specific provider version
type VersionInfo struct {
	// Empty for now - Terraform doesn't require additional fields
}

// ProviderDownloadResponse represents the response for downloading a provider
type ProviderDownloadResponse struct {
	Protocols           []string        `json:"protocols"`
	OS                  string          `json:"os"`
	Arch                string          `json:"arch"`
	Filename            string          `json:"filename"`
	DownloadURL         string          `json:"download_url"`
	SHA256SumsURL       string          `json:"shasum_url"`
	SHA256SumsSignature string          `json:"shasum_signature_url"`
	SHA256Sum           string          `json:"shasum"`
	SigningKeys         SigningKeysInfo `json:"signing_keys"`
}

// SigningKeysInfo contains GPG signing key information
type SigningKeysInfo struct {
	GPGPublicKeys []GPGPublicKey `json:"gpg_public_keys,omitempty"`
}

// GPGPublicKey represents a GPG public key
type GPGPublicKey struct {
	KeyID          string `json:"key_id"`
	ASCIIArmor     string `json:"ascii_armor"`
	TrustSignature string `json:"trust_signature,omitempty"`
	Source         string `json:"source,omitempty"`
	SourceURL      string `json:"source_url,omitempty"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// respondJSON writes a JSON response
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondError writes an error response
func respondError(w http.ResponseWriter, status int, err string, message string) {
	respondJSON(w, status, ErrorResponse{
		Error:   err,
		Message: message,
	})
}

// handleProviderVersions lists available versions for a provider
// GET /v1/providers/{namespace}/{type}/versions
func (s *Server) handleProviderVersions(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	providerType := chi.URLParam(r, "type")

	if namespace == "" || providerType == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "namespace and type are required")
		return
	}

	// Query database for all versions of this provider
	providers, err := s.providerRepo.ListVersions(r.Context(), namespace, providerType)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "database_error", "failed to query provider versions")
		return
	}

	if len(providers) == 0 {
		respondError(w, http.StatusNotFound, "not_found", fmt.Sprintf("provider %s/%s not found", namespace, providerType))
		return
	}

	// Build response per Terraform protocol spec
	// We need to deduplicate versions since there can be multiple platforms per version
	versionMap := make(map[string]VersionInfo)
	for _, p := range providers {
		versionMap[p.Version] = VersionInfo{}
	}

	response := ProviderVersionsResponse{
		Versions: versionMap,
	}

	respondJSON(w, http.StatusOK, response)
}

// handleProviderDownload handles provider download requests
// GET /v1/providers/{namespace}/{type}/{version}/download/{os}/{arch}
func (s *Server) handleProviderDownload(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	providerType := chi.URLParam(r, "type")
	version := chi.URLParam(r, "version")
	os := chi.URLParam(r, "os")
	arch := chi.URLParam(r, "arch")

	if namespace == "" || providerType == "" || version == "" || os == "" || arch == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "all path parameters are required")
		return
	}

	// Platform format is "os_arch"
	platform := os + "_" + arch

	// Query database for this specific provider version
	provider, err := s.providerRepo.GetByIdentity(r.Context(), namespace, providerType, version, platform)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "database_error", "failed to query provider")
		return
	}

	if provider == nil {
		respondError(w, http.StatusNotFound, "not_found",
			fmt.Sprintf("provider %s/%s version %s for %s/%s not found", namespace, providerType, version, os, arch))
		return
	}

	// Generate presigned URL for the provider binary
	downloadURL, err := s.storage.GetPresignedURL(r.Context(), provider.S3Key, 3600) // 1 hour expiry
	if err != nil {
		respondError(w, http.StatusInternalServerError, "storage_error", "failed to generate download URL")
		return
	}

	// Generate presigned URL for SHA256SUMS file
	shasumsPath := provider.S3Key + "_SHA256SUMS"
	shasumsURL, err := s.storage.GetPresignedURL(r.Context(), shasumsPath, 3600)
	if err != nil {
		// SHA256SUMS file might not exist yet, log but continue
		shasumsURL = ""
	}

	// Generate presigned URL for SHA256SUMS.sig file
	sigPath := provider.S3Key + "_SHA256SUMS.sig"
	sigURL, err := s.storage.GetPresignedURL(r.Context(), sigPath, 3600)
	if err != nil {
		// Signature file might not exist yet, log but continue
		sigURL = ""
	}

	// Build response per Terraform protocol spec
	response := ProviderDownloadResponse{
		Protocols:           []string{"5.0"}, // Terraform protocol version
		OS:                  os,
		Arch:                arch,
		Filename:            provider.Filename,
		DownloadURL:         downloadURL,
		SHA256SumsURL:       shasumsURL,
		SHA256SumsSignature: sigURL,
		SHA256Sum:           provider.Shasum,
		SigningKeys: SigningKeysInfo{
			GPGPublicKeys: []GPGPublicKey{},
		},
	}

	respondJSON(w, http.StatusOK, response)
}
