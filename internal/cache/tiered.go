package cache

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync/atomic"
	"time"
)

// TieredCache implements a two-tier cache with memory (L1) and disk (L2)
// Items are promoted to memory on access and demoted to disk when evicted
type TieredCache struct {
	// memory is the L1 (fast) cache
	memory *MemoryCache

	// disk is the L2 (large) cache
	disk *DiskCache

	// config holds the tiered cache configuration
	config TieredCacheConfig

	// stats tracks combined statistics
	stats TieredCacheStats
}

// TieredCacheConfig contains configuration for the tiered cache
type TieredCacheConfig struct {
	// MemorySizeMB is the maximum memory cache size in megabytes
	MemorySizeMB int

	// DiskPath is the directory for disk cache storage
	DiskPath string

	// DiskSizeGB is the maximum disk cache size in gigabytes
	DiskSizeGB int

	// DefaultTTL is the default TTL for cached items
	DefaultTTL time.Duration

	// MemoryCleanupInterval is how often to clean the memory cache
	MemoryCleanupInterval time.Duration

	// DiskCleanupInterval is how often to clean the disk cache
	DiskCleanupInterval time.Duration

	// PromoteOnHit promotes items from disk to memory on access
	PromoteOnHit bool

	// WriteThrough writes to both caches on Set (vs write to memory only)
	WriteThrough bool
}

// TieredCacheStats contains combined statistics for both cache tiers
type TieredCacheStats struct {
	// Memory cache stats
	MemoryHits        int64 `json:"memory_hits"`
	MemoryMisses      int64 `json:"memory_misses"`
	MemorySize        int64 `json:"memory_size"`
	MemoryMaxSize     int64 `json:"memory_max_size"`
	MemoryItemCount   int64 `json:"memory_item_count"`
	MemoryEvictions   int64 `json:"memory_evictions"`
	MemoryExpirations int64 `json:"memory_expirations"`

	// Disk cache stats
	DiskHits        int64 `json:"disk_hits"`
	DiskMisses      int64 `json:"disk_misses"`
	DiskSize        int64 `json:"disk_size"`
	DiskMaxSize     int64 `json:"disk_max_size"`
	DiskItemCount   int64 `json:"disk_item_count"`
	DiskEvictions   int64 `json:"disk_evictions"`
	DiskExpirations int64 `json:"disk_expirations"`

	// Combined stats
	TotalHits   int64 `json:"total_hits"`
	TotalMisses int64 `json:"total_misses"`
	Promotions  int64 `json:"promotions"`
}

// DefaultTieredConfig returns a default tiered cache configuration
func DefaultTieredConfig() TieredCacheConfig {
	return TieredCacheConfig{
		MemorySizeMB:          256,
		DiskPath:              "/var/cache/tf-mirror",
		DiskSizeGB:            10,
		DefaultTTL:            time.Hour,
		MemoryCleanupInterval: 5 * time.Minute,
		DiskCleanupInterval:   10 * time.Minute,
		PromoteOnHit:          true,
		WriteThrough:          false,
	}
}

// NewTieredCache creates a new two-tier cache
func NewTieredCache(cfg TieredCacheConfig) (*TieredCache, error) {
	// Create memory cache
	memoryCache, err := NewMemoryCache(MemoryCacheConfig{
		MaxSizeMB:       cfg.MemorySizeMB,
		DefaultTTL:      cfg.DefaultTTL,
		CleanupInterval: cfg.MemoryCleanupInterval,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create memory cache: %w", err)
	}

	// Create disk cache
	diskCache, err := NewDiskCache(DiskCacheConfig{
		BasePath:        cfg.DiskPath,
		MaxSizeGB:       cfg.DiskSizeGB,
		DefaultTTL:      cfg.DefaultTTL,
		CleanupInterval: cfg.DiskCleanupInterval,
	})
	if err != nil {
		memoryCache.Close()
		return nil, fmt.Errorf("failed to create disk cache: %w", err)
	}

	return &TieredCache{
		memory: memoryCache,
		disk:   diskCache,
		config: cfg,
	}, nil
}

// Get retrieves an item from the cache, checking memory first, then disk
func (tc *TieredCache) Get(ctx context.Context, key string) (io.ReadCloser, string, bool) {
	// Try memory cache first (L1)
	if reader, contentType, found := tc.memory.Get(ctx, key); found {
		atomic.AddInt64(&tc.stats.MemoryHits, 1)
		atomic.AddInt64(&tc.stats.TotalHits, 1)
		return reader, contentType, true
	}
	atomic.AddInt64(&tc.stats.MemoryMisses, 1)

	// Try disk cache (L2)
	reader, contentType, found := tc.disk.Get(ctx, key)
	if !found {
		atomic.AddInt64(&tc.stats.DiskMisses, 1)
		atomic.AddInt64(&tc.stats.TotalMisses, 1)
		return nil, "", false
	}
	atomic.AddInt64(&tc.stats.DiskHits, 1)
	atomic.AddInt64(&tc.stats.TotalHits, 1)

	// Promote to memory cache if configured
	if tc.config.PromoteOnHit {
		tc.promoteToMemory(ctx, key, reader, contentType)
		atomic.AddInt64(&tc.stats.Promotions, 1)

		// Return fresh reader from memory
		if memReader, memContentType, found := tc.memory.Get(ctx, key); found {
			return memReader, memContentType, true
		}
	}

	return reader, contentType, true
}

// promoteToMemory copies an item from disk to memory
func (tc *TieredCache) promoteToMemory(ctx context.Context, key string, reader io.ReadCloser, contentType string) {
	// Read all data
	data, err := io.ReadAll(reader)
	reader.Close()
	if err != nil {
		return
	}

	// Get TTL from disk entry if available
	ttl := tc.config.DefaultTTL
	if entry, found := tc.disk.GetEntry(key); found {
		remaining := time.Until(entry.ExpiresAt)
		if remaining > 0 {
			ttl = remaining
		}
	}

	// Store in memory
	tc.memory.Set(ctx, key, bytes.NewReader(data), contentType, int64(len(data)), ttl)
}

// Set stores an item in the cache
func (tc *TieredCache) Set(ctx context.Context, key string, data io.Reader, contentType string, size int64, ttl time.Duration) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	// Read all data since we may need to write to multiple caches
	dataBytes, err := io.ReadAll(data)
	if err != nil {
		return fmt.Errorf("failed to read data: %w", err)
	}

	// Always write to memory cache (L1)
	if err := tc.memory.Set(ctx, key, bytes.NewReader(dataBytes), contentType, int64(len(dataBytes)), ttl); err != nil {
		// If memory is full, write to disk instead
		return tc.disk.Set(ctx, key, bytes.NewReader(dataBytes), contentType, int64(len(dataBytes)), ttl)
	}

	// Write-through: also write to disk for durability
	if tc.config.WriteThrough {
		tc.disk.Set(ctx, key, bytes.NewReader(dataBytes), contentType, int64(len(dataBytes)), ttl)
	}

	return nil
}

// Delete removes an item from both caches
func (tc *TieredCache) Delete(ctx context.Context, key string) error {
	// Delete from both caches
	memErr := tc.memory.Delete(ctx, key)
	diskErr := tc.disk.Delete(ctx, key)

	if memErr != nil {
		return memErr
	}
	return diskErr
}

// Exists checks if an item exists in either cache
func (tc *TieredCache) Exists(ctx context.Context, key string) bool {
	if tc.memory.Exists(ctx, key) {
		return true
	}
	return tc.disk.Exists(ctx, key)
}

// Clear removes all items from both caches
func (tc *TieredCache) Clear(ctx context.Context) error {
	if err := tc.memory.Clear(ctx); err != nil {
		return err
	}
	return tc.disk.Clear(ctx)
}

// Stats returns combined cache statistics
func (tc *TieredCache) Stats() CacheStats {
	memStats := tc.memory.Stats()
	diskStats := tc.disk.Stats()

	// Return combined stats fitting the CacheStats interface
	return CacheStats{
		Hits:        memStats.Hits + diskStats.Hits,
		Misses:      atomic.LoadInt64(&tc.stats.TotalMisses),
		Size:        memStats.Size + diskStats.Size,
		MaxSize:     memStats.MaxSize + diskStats.MaxSize,
		ItemCount:   memStats.ItemCount + diskStats.ItemCount,
		Evictions:   memStats.Evictions + diskStats.Evictions,
		Expirations: memStats.Expirations + diskStats.Expirations,
	}
}

// DetailedStats returns detailed statistics for both cache tiers
func (tc *TieredCache) DetailedStats() TieredCacheStats {
	memStats := tc.memory.Stats()
	diskStats := tc.disk.Stats()

	return TieredCacheStats{
		MemoryHits:        memStats.Hits,
		MemoryMisses:      memStats.Misses,
		MemorySize:        memStats.Size,
		MemoryMaxSize:     memStats.MaxSize,
		MemoryItemCount:   memStats.ItemCount,
		MemoryEvictions:   memStats.Evictions,
		MemoryExpirations: memStats.Expirations,

		DiskHits:        diskStats.Hits,
		DiskMisses:      diskStats.Misses,
		DiskSize:        diskStats.Size,
		DiskMaxSize:     diskStats.MaxSize,
		DiskItemCount:   diskStats.ItemCount,
		DiskEvictions:   diskStats.Evictions,
		DiskExpirations: diskStats.Expirations,

		TotalHits:   atomic.LoadInt64(&tc.stats.TotalHits),
		TotalMisses: atomic.LoadInt64(&tc.stats.TotalMisses),
		Promotions:  atomic.LoadInt64(&tc.stats.Promotions),
	}
}

// Close performs cleanup on both caches
func (tc *TieredCache) Close() error {
	memErr := tc.memory.Close()
	diskErr := tc.disk.Close()

	if memErr != nil {
		return memErr
	}
	return diskErr
}

// Memory returns the memory cache for direct access
func (tc *TieredCache) Memory() *MemoryCache {
	return tc.memory
}

// Disk returns the disk cache for direct access
func (tc *TieredCache) Disk() *DiskCache {
	return tc.disk
}

// SetToDisk stores an item directly to disk cache (bypassing memory)
func (tc *TieredCache) SetToDisk(ctx context.Context, key string, data io.Reader, contentType string, size int64, ttl time.Duration) error {
	return tc.disk.Set(ctx, key, data, contentType, size, ttl)
}

// SetToMemory stores an item directly to memory cache
func (tc *TieredCache) SetToMemory(ctx context.Context, key string, data io.Reader, contentType string, size int64, ttl time.Duration) error {
	return tc.memory.Set(ctx, key, data, contentType, size, ttl)
}

// Warmup loads items from disk cache into memory cache
// This is useful on startup to pre-populate the memory cache
func (tc *TieredCache) Warmup(ctx context.Context, maxItems int) int {
	keys := tc.disk.ListKeys()
	warmed := 0

	for _, key := range keys {
		if maxItems > 0 && warmed >= maxItems {
			break
		}

		if reader, contentType, found := tc.disk.Get(ctx, key); found {
			data, err := io.ReadAll(reader)
			reader.Close()
			if err == nil {
				entry, _ := tc.disk.GetEntry(key)
				ttl := tc.config.DefaultTTL
				if entry != nil {
					remaining := time.Until(entry.ExpiresAt)
					if remaining > 0 {
						ttl = remaining
					}
				}
				tc.memory.Set(ctx, key, bytes.NewReader(data), contentType, int64(len(data)), ttl)
				warmed++
			}
		}
	}

	return warmed
}
