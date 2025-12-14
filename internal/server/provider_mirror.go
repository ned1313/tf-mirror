package server

import (
	"encoding/json"
	"net/http"
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
