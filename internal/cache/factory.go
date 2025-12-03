package cache

import (
	"fmt"
	"time"

	"github.com/ned1313/terraform-mirror/internal/config"
)

// NewFromConfig creates a cache from application configuration
func NewFromConfig(cfg config.CacheConfig) (Cache, error) {
	// If memory and disk are both configured, use tiered cache
	if cfg.MemorySizeMB > 0 && cfg.DiskPath != "" && cfg.DiskSizeGB > 0 {
		return NewTieredCache(TieredCacheConfig{
			MemorySizeMB:          cfg.MemorySizeMB,
			DiskPath:              cfg.DiskPath,
			DiskSizeGB:            cfg.DiskSizeGB,
			DefaultTTL:            cfg.GetCacheTTL(),
			MemoryCleanupInterval: 5 * time.Minute,
			DiskCleanupInterval:   10 * time.Minute,
			PromoteOnHit:          true,
			WriteThrough:          false,
		})
	}

	// Memory-only cache
	if cfg.MemorySizeMB > 0 && cfg.DiskPath == "" {
		return NewMemoryCache(MemoryCacheConfig{
			MaxSizeMB:       cfg.MemorySizeMB,
			DefaultTTL:      cfg.GetCacheTTL(),
			CleanupInterval: 5 * time.Minute,
		})
	}

	// Disk-only cache
	if cfg.DiskPath != "" && cfg.DiskSizeGB > 0 {
		return NewDiskCache(DiskCacheConfig{
			BasePath:        cfg.DiskPath,
			MaxSizeGB:       cfg.DiskSizeGB,
			DefaultTTL:      cfg.GetCacheTTL(),
			CleanupInterval: 10 * time.Minute,
		})
	}

	return nil, fmt.Errorf("invalid cache configuration: must specify memory size or disk path")
}

// NewNoOpCache creates a cache that does nothing (for when caching is disabled)
func NewNoOpCache() Cache {
	return &noOpCache{}
}
