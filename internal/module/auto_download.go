package module

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ned1313/terraform-mirror/internal/config"
	"github.com/ned1313/terraform-mirror/internal/database"
	"github.com/ned1313/terraform-mirror/internal/storage"
	"golang.org/x/time/rate"
)

// AutoDownloadService handles on-demand module downloads with rate limiting
type AutoDownloadService struct {
	config     *config.AutoDownloadModulesConfig
	moduleCfg  *config.ModulesConfig
	registry   RegistryDownloader
	rewriter   *Rewriter
	storage    storage.Storage
	moduleRepo *database.ModuleRepository
	logger     *log.Logger

	// Rate limiting
	rateLimiter *rate.Limiter

	// Concurrency control
	semaphore chan struct{}

	// In-flight download tracking to prevent duplicate downloads
	inFlight   map[string]chan *moduleDownloadResult
	inFlightMu sync.Mutex

	// Negative cache for "not found" responses
	negativeCache   map[string]time.Time
	negativeCacheMu sync.RWMutex

	// Metrics
	stats     ModuleAutoDownloadStats
	statsMu   sync.RWMutex
	startTime time.Time
}

// ModuleAutoDownloadStats contains statistics about module auto-download operations
type ModuleAutoDownloadStats struct {
	TotalRequests       int64
	SuccessfulDownloads int64
	FailedDownloads     int64
	CacheHits           int64
	NegativeCacheHits   int64
	RateLimitedCount    int64
	NamespaceBlocked    int64
	InFlightCoalesced   int64
	BytesDownloaded     int64
}

// moduleDownloadResult is used for coalescing in-flight requests
type moduleDownloadResult struct {
	module *database.Module
	err    error
}

// NewAutoDownloadService creates a new module auto-download service
func NewAutoDownloadService(
	cfg *config.AutoDownloadModulesConfig,
	moduleCfg *config.ModulesConfig,
	storage storage.Storage,
	db *database.DB,
) *AutoDownloadService {
	// Create rate limiter: allows RateLimitPerMinute requests per minute
	// with a burst of MaxConcurrentDL
	ratePerSecond := float64(cfg.RateLimitPerMinute) / 60.0
	limiter := rate.NewLimiter(rate.Limit(ratePerSecond), cfg.MaxConcurrentDL)

	return &AutoDownloadService{
		config:        cfg,
		moduleCfg:     moduleCfg,
		registry:      NewRegistryClient(moduleCfg.GetUpstreamRegistry()),
		rewriter:      NewRewriter(moduleCfg.MirrorHostname),
		storage:       storage,
		moduleRepo:    database.NewModuleRepository(db),
		logger:        log.Default(),
		rateLimiter:   limiter,
		semaphore:     make(chan struct{}, cfg.MaxConcurrentDL),
		inFlight:      make(map[string]chan *moduleDownloadResult),
		negativeCache: make(map[string]time.Time),
		startTime:     time.Now(),
	}
}

// SetRegistry allows setting a custom registry client (for testing)
func (s *AutoDownloadService) SetRegistry(r RegistryDownloader) {
	s.registry = r
}

// GetStats returns current statistics
func (s *AutoDownloadService) GetStats() ModuleAutoDownloadStats {
	s.statsMu.RLock()
	defer s.statsMu.RUnlock()
	return s.stats
}

// IsEnabled returns whether auto-download is enabled
func (s *AutoDownloadService) IsEnabled() bool {
	return s.config.Enabled
}

// GetAvailableVersions queries the upstream registry for available versions
func (s *AutoDownloadService) GetAvailableVersions(ctx context.Context, namespace, name, system string) ([]string, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("module auto-download is disabled")
	}

	// Check if namespace is allowed
	if !s.config.IsNamespaceAllowed(namespace) {
		return nil, fmt.Errorf("namespace %s is not allowed for module auto-download", namespace)
	}

	return s.registry.GetAvailableVersions(ctx, namespace, name, system)
}

// DownloadModule attempts to download a module on-demand
// Returns the module record if successful, or an error
func (s *AutoDownloadService) DownloadModule(
	ctx context.Context,
	namespace, name, system, version string,
) (*database.Module, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("module auto-download is disabled")
	}

	s.statsMu.Lock()
	s.stats.TotalRequests++
	s.statsMu.Unlock()

	// Check if namespace is allowed
	if !s.config.IsNamespaceAllowed(namespace) {
		s.statsMu.Lock()
		s.stats.NamespaceBlocked++
		s.statsMu.Unlock()
		return nil, fmt.Errorf("namespace %s is not allowed for module auto-download", namespace)
	}

	cacheKey := fmt.Sprintf("%s/%s/%s/%s", namespace, name, system, version)

	// Check negative cache
	if s.config.CacheNegativeResults {
		s.negativeCacheMu.RLock()
		if expiry, found := s.negativeCache[cacheKey]; found {
			if time.Now().Before(expiry) {
				s.negativeCacheMu.RUnlock()
				s.statsMu.Lock()
				s.stats.NegativeCacheHits++
				s.statsMu.Unlock()
				return nil, fmt.Errorf("module %s not found (cached)", cacheKey)
			}
		}
		s.negativeCacheMu.RUnlock()
	}

	// Check if already downloading (coalesce duplicate requests)
	s.inFlightMu.Lock()
	if ch, exists := s.inFlight[cacheKey]; exists {
		s.inFlightMu.Unlock()
		s.statsMu.Lock()
		s.stats.InFlightCoalesced++
		s.statsMu.Unlock()

		// Wait for the in-flight download to complete
		select {
		case result := <-ch:
			return result.module, result.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Create channel for this download
	resultCh := make(chan *moduleDownloadResult, 1)
	s.inFlight[cacheKey] = resultCh
	s.inFlightMu.Unlock()

	// Ensure we clean up and broadcast result when done
	defer func() {
		s.inFlightMu.Lock()
		delete(s.inFlight, cacheKey)
		s.inFlightMu.Unlock()
		close(resultCh)
	}()

	// Apply rate limiting
	if err := s.rateLimiter.Wait(ctx); err != nil {
		s.statsMu.Lock()
		s.stats.RateLimitedCount++
		s.statsMu.Unlock()
		result := &moduleDownloadResult{err: fmt.Errorf("rate limited: %w", err)}
		resultCh <- result
		return nil, result.err
	}

	// Acquire semaphore for concurrent download limit
	select {
	case s.semaphore <- struct{}{}:
		defer func() { <-s.semaphore }()
	case <-ctx.Done():
		result := &moduleDownloadResult{err: ctx.Err()}
		resultCh <- result
		return nil, result.err
	}

	// Perform the download
	module, err := s.performDownload(ctx, namespace, name, system, version)

	result := &moduleDownloadResult{module: module, err: err}
	resultCh <- result

	if err != nil {
		// Cache negative result
		if s.config.CacheNegativeResults {
			s.negativeCacheMu.Lock()
			s.negativeCache[cacheKey] = time.Now().Add(s.config.GetNegativeCacheTTL())
			s.negativeCacheMu.Unlock()
		}
		s.statsMu.Lock()
		s.stats.FailedDownloads++
		s.statsMu.Unlock()
		return nil, err
	}

	s.statsMu.Lock()
	s.stats.SuccessfulDownloads++
	s.statsMu.Unlock()

	return module, nil
}

// performDownload does the actual download work
func (s *AutoDownloadService) performDownload(
	ctx context.Context,
	namespace, name, system, version string,
) (*database.Module, error) {
	// Create timeout context
	downloadCtx, cancel := context.WithTimeout(ctx, s.config.GetTimeout())
	defer cancel()

	s.logger.Printf("Auto-downloading module: %s/%s/%s %s",
		namespace, name, system, version)

	// Download from registry
	result := s.registry.DownloadModuleComplete(downloadCtx, namespace, name, system, version)
	if result.Error != nil {
		return nil, fmt.Errorf("download failed: %w", result.Error)
	}

	// Rewrite module sources if configured
	moduleData, err := s.rewriter.RewriteModule(result.Data)
	if err != nil {
		return nil, fmt.Errorf("source rewriting failed: %w", err)
	}

	// Build storage key
	filename := fmt.Sprintf("%s-%s-%s-%s.tar.gz", namespace, name, system, version)
	storageKey := fmt.Sprintf("modules/%s/%s/%s/%s/%s",
		namespace, name, system, version, filename)

	// Upload to storage
	reader := bytes.NewReader(moduleData)
	err = s.storage.Upload(downloadCtx, storageKey, reader, "application/gzip", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to upload to storage: %w", err)
	}

	// Create database record
	module := &database.Module{
		Namespace: namespace,
		Name:      name,
		System:    system,
		Version:   version,
		Filename:  filename,
		S3Key:     storageKey,
		SizeBytes: int64(len(moduleData)),
		OriginalSourceURL: sql.NullString{
			String: result.Info.DownloadURL,
			Valid:  result.Info.DownloadURL != "",
		},
	}

	err = s.moduleRepo.Create(downloadCtx, module)
	if err != nil {
		// If module already exists (race condition), fetch and return it
		existing, getErr := s.moduleRepo.GetByIdentity(downloadCtx, namespace, name, system, version)
		if getErr == nil && existing != nil {
			return existing, nil
		}
		return nil, fmt.Errorf("failed to store module record: %w", err)
	}

	s.statsMu.Lock()
	s.stats.BytesDownloaded += int64(len(moduleData))
	s.statsMu.Unlock()

	s.logger.Printf("Auto-download complete: %s/%s/%s %s - %d bytes in %v",
		namespace, name, system, version, len(moduleData), result.Duration)

	return module, nil
}

// ClearNegativeCache clears the negative cache (useful for admin operations)
func (s *AutoDownloadService) ClearNegativeCache() {
	s.negativeCacheMu.Lock()
	s.negativeCache = make(map[string]time.Time)
	s.negativeCacheMu.Unlock()
}

// CleanupExpiredNegativeCache removes expired entries from negative cache
func (s *AutoDownloadService) CleanupExpiredNegativeCache() int {
	s.negativeCacheMu.Lock()
	defer s.negativeCacheMu.Unlock()

	now := time.Now()
	removed := 0
	for key, expiry := range s.negativeCache {
		if now.After(expiry) {
			delete(s.negativeCache, key)
			removed++
		}
	}
	return removed
}

// GetOrDownloadModule checks if a module exists locally, and downloads it if not
func (s *AutoDownloadService) GetOrDownloadModule(
	ctx context.Context,
	namespace, name, system, version string,
) (*database.Module, error) {
	// First check if the module already exists
	existing, err := s.moduleRepo.GetByIdentity(ctx, namespace, name, system, version)
	if err != nil {
		return nil, fmt.Errorf("database lookup failed: %w", err)
	}

	if existing != nil {
		s.statsMu.Lock()
		s.stats.CacheHits++
		s.statsMu.Unlock()
		return existing, nil
	}

	// Not found locally, attempt auto-download
	return s.DownloadModule(ctx, namespace, name, system, version)
}
