package provider

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/ned1313/terraform-mirror/internal/database"
	"github.com/ned1313/terraform-mirror/internal/storage"
)

// Service orchestrates provider operations (parse, download, upload, store)
type Service struct {
	registry *RegistryClient
	storage  storage.Storage
	db       *database.DB
}

// NewService creates a new provider service
func NewService(storage storage.Storage, db *database.DB) *Service {
	return &Service{
		registry: NewRegistryClient(),
		storage:  storage,
		db:       db,
	}
}

// LoadResult represents the result of loading a single provider
type LoadResult struct {
	Namespace string
	Type      string
	Version   string
	Platform  string
	Success   bool
	Error     error
	Skipped   bool // Already exists
}

// LoadFromDefinitions loads all providers specified in the definitions
func (s *Service) LoadFromDefinitions(ctx context.Context, defs *ProviderDefinitions) ([]*LoadResult, error) {
	results := make([]*LoadResult, 0, defs.CountItems())

	providerRepo := database.NewProviderRepository(s.db)

	// Process each provider definition
	for _, def := range defs.Providers {
		// Process each version
		for _, version := range def.Versions {
			// Process each platform
			for _, platform := range def.Platforms {
				// Check context cancellation
				if ctx.Err() != nil {
					return results, ctx.Err()
				}

				// Split platform into OS and arch
				parts := strings.SplitN(platform, "_", 2)
				if len(parts) != 2 {
					results = append(results, &LoadResult{
						Namespace: def.Namespace,
						Type:      def.Type,
						Version:   version,
						Platform:  platform,
						Success:   false,
						Error:     fmt.Errorf("invalid platform format: %s", platform),
					})
					continue
				}
				os, arch := parts[0], parts[1]

				// Check if already exists
				existing, err := providerRepo.GetByIdentity(ctx, def.Namespace, def.Type, version, platform)
				if err != nil {
					results = append(results, &LoadResult{
						Namespace: def.Namespace,
						Type:      def.Type,
						Version:   version,
						Platform:  platform,
						Success:   false,
						Error:     fmt.Errorf("database check failed: %w", err),
					})
					continue
				}

				if existing != nil {
					// Already exists, skip
					results = append(results, &LoadResult{
						Namespace: def.Namespace,
						Type:      def.Type,
						Version:   version,
						Platform:  platform,
						Success:   true,
						Skipped:   true,
					})
					continue
				}

				// Download from registry
				downloadResult := s.registry.DownloadProviderComplete(ctx, def.Namespace, def.Type, version, os, arch)
				if downloadResult.Error != nil {
					results = append(results, &LoadResult{
						Namespace: def.Namespace,
						Type:      def.Type,
						Version:   version,
						Platform:  platform,
						Success:   false,
						Error:     fmt.Errorf("download failed: %w", downloadResult.Error),
					})
					continue
				}

				// Build S3 key
				s3Key := s.buildS3Key(def.Namespace, def.Type, version, platform, downloadResult.Info.Filename)

				// Upload to S3
				reader := bytes.NewReader(downloadResult.Data)
				if err := s.storage.Upload(ctx, s3Key, reader, "application/zip", nil); err != nil {
					results = append(results, &LoadResult{
						Namespace: def.Namespace,
						Type:      def.Type,
						Version:   version,
						Platform:  platform,
						Success:   false,
						Error:     fmt.Errorf("storage upload failed: %w", err),
					})
					continue
				}

				// Save to database
				provider := &database.Provider{
					Namespace: def.Namespace,
					Type:      def.Type,
					Version:   version,
					Platform:  platform,
					Filename:  downloadResult.Info.Filename,
					Shasum:    downloadResult.Info.Shasum,
					S3Key:     s3Key,
					SizeBytes: int64(len(downloadResult.Data)),
				}

				if err := providerRepo.Create(ctx, provider); err != nil {
					// Try to clean up S3 upload
					_ = s.storage.Delete(ctx, s3Key)

					results = append(results, &LoadResult{
						Namespace: def.Namespace,
						Type:      def.Type,
						Version:   version,
						Platform:  platform,
						Success:   false,
						Error:     fmt.Errorf("database save failed: %w", err),
					})
					continue
				}

				// Success!
				results = append(results, &LoadResult{
					Namespace: def.Namespace,
					Type:      def.Type,
					Version:   version,
					Platform:  platform,
					Success:   true,
				})
			}
		}
	}

	return results, nil
}

// buildS3Key constructs the S3 storage key for a provider
// Format: providers/{namespace}/{type}/{version}/{platform}/{filename}
func (s *Service) buildS3Key(namespace, providerType, version, platform, filename string) string {
	return path.Join("providers", namespace, providerType, version, platform, filename)
}

// LoadStats returns statistics about the load operation
type LoadStats struct {
	Total   int
	Success int
	Failed  int
	Skipped int
	Errors  []string
}

// CalculateStats calculates statistics from load results
func CalculateStats(results []*LoadResult) *LoadStats {
	stats := &LoadStats{
		Total:  len(results),
		Errors: make([]string, 0),
	}

	for _, r := range results {
		if r.Skipped {
			stats.Skipped++
			stats.Success++ // Skipped counts as success
		} else if r.Success {
			stats.Success++
		} else {
			stats.Failed++
			if r.Error != nil {
				errMsg := fmt.Sprintf("%s/%s %s %s: %s",
					r.Namespace, r.Type, r.Version, r.Platform, r.Error.Error())
				stats.Errors = append(stats.Errors, errMsg)
			}
		}
	}

	return stats
}
