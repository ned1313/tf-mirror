package module

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"path"

	"github.com/ned1313/terraform-mirror/internal/database"
	"github.com/ned1313/terraform-mirror/internal/storage"
)

// Service orchestrates module operations (parse, download, rewrite, upload, store)
type Service struct {
	registry RegistryDownloader
	rewriter *Rewriter
	storage  storage.Storage
	db       *database.DB
}

// NewService creates a new module service
func NewService(storage storage.Storage, db *database.DB, mirrorHostname string) *Service {
	return &Service{
		registry: NewRegistryClient(""),
		rewriter: NewRewriter(mirrorHostname),
		storage:  storage,
		db:       db,
	}
}

// NewServiceWithRegistry creates a new module service with a custom upstream registry
func NewServiceWithRegistry(storage storage.Storage, db *database.DB, upstreamRegistry, mirrorHostname string) *Service {
	return &Service{
		registry: NewRegistryClient(upstreamRegistry),
		rewriter: NewRewriter(mirrorHostname),
		storage:  storage,
		db:       db,
	}
}

// SetRegistry allows injection of a mock registry client for testing
func (s *Service) SetRegistry(registry RegistryDownloader) {
	s.registry = registry
}

// LoadResult represents the result of loading a single module version
type LoadResult struct {
	Namespace string
	Name      string
	System    string
	Version   string
	Success   bool
	Error     error
	Skipped   bool // Already exists
}

// ProgressCallback is called after each module is processed
type ProgressCallback func(result *LoadResult)

// LoadFromDefinitions loads all modules specified in the definitions
func (s *Service) LoadFromDefinitions(ctx context.Context, defs *ModuleDefinitions) ([]*LoadResult, error) {
	return s.LoadFromDefinitionsWithProgress(ctx, defs, nil)
}

// LoadFromDefinitionsWithProgress loads all modules with a progress callback
func (s *Service) LoadFromDefinitionsWithProgress(ctx context.Context, defs *ModuleDefinitions, onProgress ProgressCallback) ([]*LoadResult, error) {
	results := make([]*LoadResult, 0, defs.CountItems())

	moduleRepo := database.NewModuleRepository(s.db)

	// Process each module definition
	for _, def := range defs.Modules {
		// Process each version
		for _, version := range def.Versions {
			// Check context cancellation
			if ctx.Err() != nil {
				return results, ctx.Err()
			}

			result := s.loadModuleVersion(ctx, moduleRepo, def, version)
			results = append(results, result)

			// Call progress callback if provided
			if onProgress != nil {
				onProgress(result)
			}
		}
	}

	return results, nil
}

// loadModuleVersion loads a single module version
func (s *Service) loadModuleVersion(ctx context.Context, moduleRepo *database.ModuleRepository, def *ModuleDefinition, version string) *LoadResult {
	result := &LoadResult{
		Namespace: def.Namespace,
		Name:      def.Name,
		System:    def.System,
		Version:   version,
	}

	// Check if already exists
	existing, err := moduleRepo.GetByIdentity(ctx, def.Namespace, def.Name, def.System, version)
	if err != nil {
		result.Error = fmt.Errorf("database check failed: %w", err)
		return result
	}

	if existing != nil {
		// Already exists, skip
		result.Success = true
		result.Skipped = true
		return result
	}

	// Download from registry
	downloadResult := s.registry.DownloadModuleComplete(ctx, def.Namespace, def.Name, def.System, version)
	if downloadResult.Error != nil {
		result.Error = fmt.Errorf("download failed: %w", downloadResult.Error)
		return result
	}

	// Rewrite module sources (if mirror hostname is configured)
	moduleData, err := s.rewriter.RewriteModule(downloadResult.Data)
	if err != nil {
		result.Error = fmt.Errorf("source rewriting failed: %w", err)
		return result
	}

	// Build S3 key
	filename := fmt.Sprintf("%s-%s-%s-%s.tar.gz", def.Namespace, def.Name, def.System, version)
	s3Key := s.buildS3Key(def.Namespace, def.Name, def.System, version, filename)

	// Upload to S3
	reader := bytes.NewReader(moduleData)
	if err := s.storage.Upload(ctx, s3Key, reader, "application/gzip", nil); err != nil {
		result.Error = fmt.Errorf("storage upload failed: %w", err)
		return result
	}

	// Save to database
	module := &database.Module{
		Namespace: def.Namespace,
		Name:      def.Name,
		System:    def.System,
		Version:   version,
		S3Key:     s3Key,
		Filename:  filename,
		SizeBytes: int64(len(moduleData)),
		OriginalSourceURL: sql.NullString{
			String: downloadResult.Info.DownloadURL,
			Valid:  downloadResult.Info.DownloadURL != "",
		},
	}

	if err := moduleRepo.Create(ctx, module); err != nil {
		// Try to clean up S3 upload
		_ = s.storage.Delete(ctx, s3Key)
		result.Error = fmt.Errorf("database save failed: %w", err)
		return result
	}

	// Success!
	result.Success = true
	return result
}

// LoadSingleModule loads a specific module version
func (s *Service) LoadSingleModule(ctx context.Context, namespace, name, system, version string) *LoadResult {
	def := &ModuleDefinition{
		Namespace: namespace,
		Name:      name,
		System:    system,
		Versions:  []string{version},
	}

	moduleRepo := database.NewModuleRepository(s.db)
	return s.loadModuleVersion(ctx, moduleRepo, def, version)
}

// buildS3Key constructs the S3 storage key for a module
// Format: modules/{namespace}/{name}/{system}/{version}/{filename}
func (s *Service) buildS3Key(namespace, name, system, version, filename string) string {
	return path.Join("modules", namespace, name, system, version, filename)
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
				errMsg := fmt.Sprintf("%s/%s/%s %s: %s",
					r.Namespace, r.Name, r.System, r.Version, r.Error.Error())
				stats.Errors = append(stats.Errors, errMsg)
			}
		}
	}

	return stats
}

// GetModule retrieves a module from storage and database
func (s *Service) GetModule(ctx context.Context, namespace, name, system, version string) (*database.Module, []byte, error) {
	moduleRepo := database.NewModuleRepository(s.db)

	// Get from database
	module, err := moduleRepo.GetByIdentity(ctx, namespace, name, system, version)
	if err != nil {
		return nil, nil, fmt.Errorf("database lookup failed: %w", err)
	}
	if module == nil {
		return nil, nil, nil // Not found
	}

	// Download from storage
	reader, err := s.storage.Download(ctx, module.S3Key)
	if err != nil {
		return nil, nil, fmt.Errorf("storage download failed: %w", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read module data: %w", err)
	}

	return module, data, nil
}

// ListVersions lists all available versions for a module
func (s *Service) ListVersions(ctx context.Context, namespace, name, system string) ([]string, error) {
	moduleRepo := database.NewModuleRepository(s.db)
	modules, err := moduleRepo.ListVersions(ctx, namespace, name, system)
	if err != nil {
		return nil, err
	}

	versions := make([]string, len(modules))
	for i, m := range modules {
		versions[i] = m.Version
	}
	return versions, nil
}

// GetDownloadURL returns a presigned URL for downloading a module
func (s *Service) GetDownloadURL(ctx context.Context, namespace, name, system, version string) (string, error) {
	moduleRepo := database.NewModuleRepository(s.db)

	// Get from database
	module, err := moduleRepo.GetByIdentity(ctx, namespace, name, system, version)
	if err != nil {
		return "", fmt.Errorf("database lookup failed: %w", err)
	}
	if module == nil {
		return "", fmt.Errorf("module not found: %s/%s/%s/%s", namespace, name, system, version)
	}

	// Get presigned URL from storage
	url, err := s.storage.GetPresignedURL(ctx, module.S3Key, 3600) // 1 hour expiry
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return url, nil
}

// DeleteModule removes a module from storage and database
func (s *Service) DeleteModule(ctx context.Context, moduleID int64) error {
	moduleRepo := database.NewModuleRepository(s.db)

	// Get module details first
	module, err := moduleRepo.GetByID(ctx, moduleID)
	if err != nil {
		return fmt.Errorf("database lookup failed: %w", err)
	}
	if module == nil {
		return fmt.Errorf("module not found: %d", moduleID)
	}

	// Delete from storage
	if err := s.storage.Delete(ctx, module.S3Key); err != nil {
		return fmt.Errorf("storage delete failed: %w", err)
	}

	// Delete from database
	if err := moduleRepo.Delete(ctx, moduleID); err != nil {
		return fmt.Errorf("database delete failed: %w", err)
	}

	return nil
}
