// Package cache provides caching functionality for terraform-mirror.
// It includes in-memory LRU cache, disk-based cache, and a two-tier
// cache coordinator that manages both layers.
package cache

import (
	"context"
	"io"
	"time"
)

// Cache defines the interface for cache implementations
type Cache interface {
	// Get retrieves an item from the cache
	// Returns the data reader, content type, and whether it was found
	Get(ctx context.Context, key string) (io.ReadCloser, string, bool)

	// Set stores an item in the cache with optional TTL
	// If ttl is 0, the default TTL is used
	Set(ctx context.Context, key string, data io.Reader, contentType string, size int64, ttl time.Duration) error

	// Delete removes an item from the cache
	Delete(ctx context.Context, key string) error

	// Exists checks if an item exists in the cache
	Exists(ctx context.Context, key string) bool

	// Clear removes all items from the cache
	Clear(ctx context.Context) error

	// Stats returns cache statistics
	Stats() CacheStats

	// Close performs cleanup
	Close() error
}

// CacheStats contains statistics about cache usage
type CacheStats struct {
	// Hits is the number of successful cache retrievals
	Hits int64 `json:"hits"`

	// Misses is the number of cache misses
	Misses int64 `json:"misses"`

	// Size is the current cache size in bytes
	Size int64 `json:"size"`

	// MaxSize is the maximum cache size in bytes
	MaxSize int64 `json:"max_size"`

	// ItemCount is the number of items in the cache
	ItemCount int64 `json:"item_count"`

	// Evictions is the number of items evicted from the cache
	Evictions int64 `json:"evictions"`

	// Expirations is the number of items that expired
	Expirations int64 `json:"expirations"`
}

// CacheItem represents a cached item with metadata
type CacheItem struct {
	// Key is the cache key
	Key string

	// Data is the cached content
	Data []byte

	// ContentType is the MIME type of the content
	ContentType string

	// Size is the size of the data in bytes
	Size int64

	// CreatedAt is when the item was cached
	CreatedAt time.Time

	// ExpiresAt is when the item expires
	ExpiresAt time.Time

	// LastAccessed is when the item was last accessed (for LRU)
	LastAccessed time.Time

	// AccessCount is the number of times this item was accessed
	AccessCount int64
}

// IsExpired returns true if the cache item has expired
func (c *CacheItem) IsExpired() bool {
	if c.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(c.ExpiresAt)
}

// Config contains cache configuration
type Config struct {
	// MemorySizeMB is the maximum memory cache size in megabytes
	MemorySizeMB int

	// DiskPath is the directory for disk cache storage
	DiskPath string

	// DiskSizeGB is the maximum disk cache size in gigabytes
	DiskSizeGB int

	// DefaultTTL is the default TTL for cached items
	DefaultTTL time.Duration

	// CleanupInterval is how often to run cache cleanup
	CleanupInterval time.Duration

	// Enabled indicates if caching is enabled
	Enabled bool
}

// DefaultConfig returns a default cache configuration
func DefaultConfig() Config {
	return Config{
		MemorySizeMB:    256,
		DiskPath:        "/var/cache/tf-mirror",
		DiskSizeGB:      10,
		DefaultTTL:      time.Hour,
		CleanupInterval: 5 * time.Minute,
		Enabled:         true,
	}
}

// HitRate calculates the cache hit rate as a percentage
func (s *CacheStats) HitRate() float64 {
	total := s.Hits + s.Misses
	if total == 0 {
		return 0
	}
	return float64(s.Hits) / float64(total) * 100
}

// UsagePercent calculates the cache usage as a percentage
func (s *CacheStats) UsagePercent() float64 {
	if s.MaxSize == 0 {
		return 0
	}
	return float64(s.Size) / float64(s.MaxSize) * 100
}
