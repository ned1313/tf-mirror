package server

import (
	"net/http"
	"sort"

	"github.com/ned1313/terraform-mirror/internal/database"
)

// AggregatedProviderResponse represents a provider grouped by namespace/type with all versions
type AggregatedProviderResponse struct {
	Namespace string   `json:"namespace"`
	Type      string   `json:"type"`
	Versions  []string `json:"versions"`
	Platforms []string `json:"platforms"`
}

// AggregatedModuleResponse represents a module grouped by namespace/name/system with all versions
type AggregatedModuleResponse struct {
	Namespace string   `json:"namespace"`
	Name      string   `json:"name"`
	System    string   `json:"system"`
	Versions  []string `json:"versions"`
}

// handlePublicListProviders lists all providers for public browsing, aggregated by namespace/type
// GET /api/public/providers
func (s *Server) handlePublicListProviders(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get query parameters for filtering
	namespace := r.URL.Query().Get("namespace")
	providerType := r.URL.Query().Get("type")

	// Get all providers from database (we need all to aggregate)
	providers, err := s.providerRepo.List(ctx, 10000, 0)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "database_error", "Failed to list providers")
		return
	}

	// Ensure providers is never nil
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

	// Aggregate by namespace/type
	aggregated := make(map[string]*AggregatedProviderResponse)
	for _, p := range filtered {
		key := p.Namespace + "/" + p.Type
		if agg, exists := aggregated[key]; exists {
			// Add version if not already present
			found := false
			for _, v := range agg.Versions {
				if v == p.Version {
					found = true
					break
				}
			}
			if !found {
				agg.Versions = append(agg.Versions, p.Version)
			}
			// Add platform if not already present
			found = false
			for _, pl := range agg.Platforms {
				if pl == p.Platform {
					found = true
					break
				}
			}
			if !found {
				agg.Platforms = append(agg.Platforms, p.Platform)
			}
		} else {
			aggregated[key] = &AggregatedProviderResponse{
				Namespace: p.Namespace,
				Type:      p.Type,
				Versions:  []string{p.Version},
				Platforms: []string{p.Platform},
			}
		}
	}

	// Convert map to slice
	responses := make([]AggregatedProviderResponse, 0, len(aggregated))
	for _, agg := range aggregated {
		// Sort versions in descending order (newest first)
		sort.Sort(sort.Reverse(sort.StringSlice(agg.Versions)))
		sort.Strings(agg.Platforms)
		responses = append(responses, *agg)
	}

	// Sort by namespace/type
	sort.Slice(responses, func(i, j int) bool {
		if responses[i].Namespace != responses[j].Namespace {
			return responses[i].Namespace < responses[j].Namespace
		}
		return responses[i].Type < responses[j].Type
	})

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"providers": responses,
		"count":     len(responses),
	})
}

// handlePublicListModules lists all modules for public browsing, aggregated by namespace/name/system
// GET /api/public/modules
func (s *Server) handlePublicListModules(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get query parameters for filtering
	namespace := r.URL.Query().Get("namespace")
	name := r.URL.Query().Get("name")
	system := r.URL.Query().Get("system")

	// Get all modules from database (we need all to aggregate)
	modules, err := s.moduleRepo.List(ctx, 10000, 0)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "database_error", "Failed to list modules")
		return
	}

	// Ensure modules is never nil
	if modules == nil {
		modules = make([]*database.Module, 0)
	}

	// Filter if parameters provided
	filtered := modules
	if namespace != "" || name != "" || system != "" {
		filtered = make([]*database.Module, 0)
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
	}

	// Aggregate by namespace/name/system
	aggregated := make(map[string]*AggregatedModuleResponse)
	for _, m := range filtered {
		key := m.Namespace + "/" + m.Name + "/" + m.System
		if agg, exists := aggregated[key]; exists {
			// Add version if not already present
			found := false
			for _, v := range agg.Versions {
				if v == m.Version {
					found = true
					break
				}
			}
			if !found {
				agg.Versions = append(agg.Versions, m.Version)
			}
		} else {
			aggregated[key] = &AggregatedModuleResponse{
				Namespace: m.Namespace,
				Name:      m.Name,
				System:    m.System,
				Versions:  []string{m.Version},
			}
		}
	}

	// Convert map to slice
	responses := make([]AggregatedModuleResponse, 0, len(aggregated))
	for _, agg := range aggregated {
		// Sort versions in descending order (newest first)
		sort.Sort(sort.Reverse(sort.StringSlice(agg.Versions)))
		responses = append(responses, *agg)
	}

	// Sort by namespace/name/system
	sort.Slice(responses, func(i, j int) bool {
		if responses[i].Namespace != responses[j].Namespace {
			return responses[i].Namespace < responses[j].Namespace
		}
		if responses[i].Name != responses[j].Name {
			return responses[i].Name < responses[j].Name
		}
		return responses[i].System < responses[j].System
	})

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"modules": responses,
		"count":   len(responses),
	})
}
