package provider

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ned1313/terraform-mirror/internal/config"
	"github.com/ned1313/terraform-mirror/internal/database"
	"github.com/ned1313/terraform-mirror/internal/storage"
	"golang.org/x/time/rate"
)

// AutoDownloadService handles on-demand provider downloads with rate limiting
type AutoDownloadService struct {
	config       *config.AutoDownloadConfig
	providerCfg  *config.ProvidersConfig
	registry     RegistryDownloader
	storage      storage.Storage
	providerRepo *database.ProviderRepository
	logger       *log.Logger

	// Rate limiting
	rateLimiter *rate.Limiter

	// Concurrency control
	semaphore chan struct{}

	// In-flight download tracking to prevent duplicate downloads
	inFlight   map[string]chan *downloadResult
	inFlightMu sync.Mutex

	// Negative cache for "not found" responses
	negativeCache   map[string]time.Time
	negativeCacheMu sync.RWMutex

	// Metrics
	stats     AutoDownloadStats
	statsMu   sync.RWMutex
	startTime time.Time
}

// AutoDownloadStats contains statistics about auto-download operations
type AutoDownloadStats struct {
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

// downloadResult is used for coalescing in-flight requests
type downloadResult struct {
	provider *database.Provider
	err      error
}

// NewAutoDownloadService creates a new auto-download service
func NewAutoDownloadService(
	cfg *config.AutoDownloadConfig,
	providerCfg *config.ProvidersConfig,
	storage storage.Storage,
	db *database.DB,
) *AutoDownloadService {
	// Create rate limiter: allows RateLimitPerMinute requests per minute
	// with a burst of MaxConcurrentDL
	ratePerSecond := float64(cfg.RateLimitPerMinute) / 60.0
	limiter := rate.NewLimiter(rate.Limit(ratePerSecond), cfg.MaxConcurrentDL)

	return &AutoDownloadService{
		config:        cfg,
		providerCfg:   providerCfg,
		registry:      NewRegistryClient(),
		storage:       storage,
		providerRepo:  database.NewProviderRepository(db),
		logger:        log.Default(),
		rateLimiter:   limiter,
		semaphore:     make(chan struct{}, cfg.MaxConcurrentDL),
		inFlight:      make(map[string]chan *downloadResult),
		negativeCache: make(map[string]time.Time),
		startTime:     time.Now(),
	}
}

// SetRegistry allows setting a custom registry client (for testing)
func (s *AutoDownloadService) SetRegistry(r RegistryDownloader) {
	s.registry = r
}

// GetStats returns current statistics
func (s *AutoDownloadService) GetStats() AutoDownloadStats {
	s.statsMu.RLock()
	defer s.statsMu.RUnlock()
	return s.stats
}

// IsEnabled returns whether auto-download is enabled
func (s *AutoDownloadService) IsEnabled() bool {
	return s.config.Enabled
}

// GetAvailableVersions queries the upstream registry for available versions
func (s *AutoDownloadService) GetAvailableVersions(ctx context.Context, namespace, providerType string) ([]string, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("auto-download is disabled")
	}

	// Check if namespace is allowed
	if !s.config.IsNamespaceAllowed(namespace) {
		return nil, fmt.Errorf("namespace %s is not allowed for auto-download", namespace)
	}

	return s.registry.GetAvailableVersions(ctx, namespace, providerType)
}

// DownloadProvider attempts to download a provider on-demand
// Returns the provider record if successful, or an error
func (s *AutoDownloadService) DownloadProvider(
	ctx context.Context,
	namespace, providerType, version, os, arch string,
) (*database.Provider, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("auto-download is disabled")
	}

	s.statsMu.Lock()
	s.stats.TotalRequests++
	s.statsMu.Unlock()

	// Check if namespace is allowed
	if !s.config.IsNamespaceAllowed(namespace) {
		s.statsMu.Lock()
		s.stats.NamespaceBlocked++
		s.statsMu.Unlock()
		return nil, fmt.Errorf("namespace %s is not allowed for auto-download", namespace)
	}

	platform := os + "_" + arch
	cacheKey := fmt.Sprintf("%s/%s/%s/%s", namespace, providerType, version, platform)

	// Check negative cache
	if s.config.CacheNegativeResults {
		s.negativeCacheMu.RLock()
		if expiry, found := s.negativeCache[cacheKey]; found {
			if time.Now().Before(expiry) {
				s.negativeCacheMu.RUnlock()
				s.statsMu.Lock()
				s.stats.NegativeCacheHits++
				s.statsMu.Unlock()
				return nil, fmt.Errorf("provider %s not found (cached)", cacheKey)
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
			return result.provider, result.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Create channel for this download
	resultCh := make(chan *downloadResult, 1)
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
		result := &downloadResult{err: fmt.Errorf("rate limited: %w", err)}
		resultCh <- result
		return nil, result.err
	}

	// Acquire semaphore for concurrent download limit
	select {
	case s.semaphore <- struct{}{}:
		defer func() { <-s.semaphore }()
	case <-ctx.Done():
		result := &downloadResult{err: ctx.Err()}
		resultCh <- result
		return nil, result.err
	}

	// Perform the download
	provider, err := s.performDownload(ctx, namespace, providerType, version, os, arch)

	result := &downloadResult{provider: provider, err: err}
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

	return provider, nil
}

// performDownload does the actual download work
func (s *AutoDownloadService) performDownload(
	ctx context.Context,
	namespace, providerType, version, os, arch string,
) (*database.Provider, error) {
	// Create timeout context
	downloadCtx, cancel := context.WithTimeout(ctx, s.config.GetTimeout())
	defer cancel()

	s.logger.Printf("Auto-downloading provider: %s/%s %s (%s_%s)",
		namespace, providerType, version, os, arch)

	// Download from registry
	result := s.registry.DownloadProviderComplete(downloadCtx, namespace, providerType, version, os, arch)
	if result.Error != nil {
		return nil, fmt.Errorf("download failed: %w", result.Error)
	}

	// Build storage key
	platform := os + "_" + arch
	storageKey := fmt.Sprintf("providers/registry.terraform.io/%s/%s/%s/%s/%s",
		namespace, providerType, version, platform, result.Info.Filename)

	// Upload to storage
	reader := bytes.NewReader(result.Data)
	err := s.storage.Upload(downloadCtx, storageKey, reader, "application/zip", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to upload to storage: %w", err)
	}

	// Create database record
	provider := &database.Provider{
		Namespace:   namespace,
		Type:        providerType,
		Version:     version,
		Platform:    platform,
		Filename:    result.Info.Filename,
		DownloadURL: result.Info.DownloadURL,
		Shasum:      result.Info.Shasum,
		S3Key:       storageKey,
		SizeBytes:   int64(len(result.Data)),
	}

	err = s.providerRepo.Create(downloadCtx, provider)
	if err != nil {
		// If provider already exists (race condition), fetch and return it
		existing, getErr := s.providerRepo.GetByIdentity(downloadCtx, namespace, providerType, version, platform)
		if getErr == nil && existing != nil {
			return existing, nil
		}
		return nil, fmt.Errorf("failed to store provider record: %w", err)
	}

	s.statsMu.Lock()
	s.stats.BytesDownloaded += int64(len(result.Data))
	s.statsMu.Unlock()

	s.logger.Printf("Auto-download complete: %s/%s %s (%s) - %d bytes in %v",
		namespace, providerType, version, platform, len(result.Data), result.Duration)

	return provider, nil
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

// PlatformDownloadResult represents the result of downloading a single platform
type PlatformDownloadResult struct {
	Platform string
	Provider *database.Provider
	Error    error
}

// DownloadProviderAllPlatforms downloads a provider for all configured platforms
// It downloads the requested platform first, then queues downloads for other platforms
// Returns the provider for the requested platform and starts background downloads for others
func (s *AutoDownloadService) DownloadProviderAllPlatforms(
	ctx context.Context,
	namespace, providerType, version, requestedOS, requestedArch string,
) (*database.Provider, error) {
	// First, download the requested platform (blocking)
	requestedPlatform := requestedOS + "_" + requestedArch
	provider, err := s.DownloadProvider(ctx, namespace, providerType, version, requestedOS, requestedArch)
	if err != nil {
		return nil, err
	}

	// Get configured platforms
	platforms := s.config.GetPlatforms()

	// Start background downloads for other configured platforms
	for _, platform := range platforms {
		if platform == requestedPlatform {
			continue // Already downloaded
		}

		// Parse platform into os and arch
		platformOS, platformArch := parsePlatform(platform)
		if platformOS == "" || platformArch == "" {
			s.logger.Printf("Invalid platform format: %s (expected os_arch)", platform)
			continue
		}

		// Launch background download
		go func(os, arch string) {
			bgCtx, cancel := context.WithTimeout(context.Background(), s.config.GetTimeout())
			defer cancel()

			_, bgErr := s.DownloadProvider(bgCtx, namespace, providerType, version, os, arch)
			if bgErr != nil {
				s.logger.Printf("Background download failed for %s/%s %s (%s_%s): %v",
					namespace, providerType, version, os, arch, bgErr)
			} else {
				s.logger.Printf("Background download complete for %s/%s %s (%s_%s)",
					namespace, providerType, version, os, arch)
			}
		}(platformOS, platformArch)
	}

	return provider, nil
}

// parsePlatform splits a platform string (e.g., "linux_amd64") into os and arch
func parsePlatform(platform string) (os, arch string) {
	for i := len(platform) - 1; i >= 0; i-- {
		if platform[i] == '_' {
			return platform[:i], platform[i+1:]
		}
	}
	return "", ""
}
