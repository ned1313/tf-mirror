package cache

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

// MemoryCache implements an in-memory LRU cache with TTL support
type MemoryCache struct {
	mu sync.RWMutex

	// items stores the cached items
	items map[string]*CacheItem

	// lruOrder tracks access order for LRU eviction
	lruOrder []string

	// maxSize is the maximum cache size in bytes
	maxSize int64

	// currentSize is the current cache size in bytes
	currentSize int64

	// defaultTTL is the default TTL for items
	defaultTTL time.Duration

	// stats tracks cache statistics
	stats CacheStats

	// cleanupTicker for periodic cleanup
	cleanupTicker *time.Ticker
	cleanupDone   chan struct{}
}

// MemoryCacheConfig contains configuration for the memory cache
type MemoryCacheConfig struct {
	// MaxSizeMB is the maximum cache size in megabytes
	MaxSizeMB int

	// DefaultTTL is the default TTL for cached items
	DefaultTTL time.Duration

	// CleanupInterval is how often to run cleanup
	CleanupInterval time.Duration
}

// NewMemoryCache creates a new in-memory LRU cache
func NewMemoryCache(cfg MemoryCacheConfig) (*MemoryCache, error) {
	if cfg.MaxSizeMB <= 0 {
		return nil, fmt.Errorf("max size must be positive")
	}

	if cfg.DefaultTTL <= 0 {
		cfg.DefaultTTL = time.Hour
	}

	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = 5 * time.Minute
	}

	maxSize := int64(cfg.MaxSizeMB) * 1024 * 1024

	mc := &MemoryCache{
		items:      make(map[string]*CacheItem),
		lruOrder:   make([]string, 0),
		maxSize:    maxSize,
		defaultTTL: cfg.DefaultTTL,
		stats: CacheStats{
			MaxSize: maxSize,
		},
		cleanupDone: make(chan struct{}),
	}

	// Start cleanup goroutine
	mc.cleanupTicker = time.NewTicker(cfg.CleanupInterval)
	go mc.cleanupLoop()

	return mc, nil
}

// cleanupLoop periodically removes expired items
func (mc *MemoryCache) cleanupLoop() {
	for {
		select {
		case <-mc.cleanupTicker.C:
			mc.removeExpired()
		case <-mc.cleanupDone:
			return
		}
	}
}

// removeExpired removes all expired items from the cache
func (mc *MemoryCache) removeExpired() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	now := time.Now()
	var expired []string

	for key, item := range mc.items {
		if !item.ExpiresAt.IsZero() && now.After(item.ExpiresAt) {
			expired = append(expired, key)
		}
	}

	for _, key := range expired {
		mc.removeItemLocked(key)
		atomic.AddInt64(&mc.stats.Expirations, 1)
	}
}

// Get retrieves an item from the cache
func (mc *MemoryCache) Get(ctx context.Context, key string) (io.ReadCloser, string, bool) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	item, exists := mc.items[key]
	if !exists {
		atomic.AddInt64(&mc.stats.Misses, 1)
		return nil, "", false
	}

	// Check if expired
	if item.IsExpired() {
		mc.removeItemLocked(key)
		atomic.AddInt64(&mc.stats.Expirations, 1)
		atomic.AddInt64(&mc.stats.Misses, 1)
		return nil, "", false
	}

	// Update access info
	item.LastAccessed = time.Now()
	item.AccessCount++
	mc.promoteToFront(key)

	atomic.AddInt64(&mc.stats.Hits, 1)

	// Return a copy of the data
	return io.NopCloser(bytes.NewReader(item.Data)), item.ContentType, true
}

// Set stores an item in the cache
func (mc *MemoryCache) Set(ctx context.Context, key string, data io.Reader, contentType string, size int64, ttl time.Duration) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	// Read all data into memory
	dataBytes, err := io.ReadAll(data)
	if err != nil {
		return fmt.Errorf("failed to read data: %w", err)
	}

	actualSize := int64(len(dataBytes))

	// Check if item is too large for the cache
	if actualSize > mc.maxSize {
		return fmt.Errorf("item size (%d bytes) exceeds cache size (%d bytes)", actualSize, mc.maxSize)
	}

	if ttl <= 0 {
		ttl = mc.defaultTTL
	}

	now := time.Now()
	item := &CacheItem{
		Key:          key,
		Data:         dataBytes,
		ContentType:  contentType,
		Size:         actualSize,
		CreatedAt:    now,
		ExpiresAt:    now.Add(ttl),
		LastAccessed: now,
		AccessCount:  0,
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	// If item already exists, remove it first
	if existing, exists := mc.items[key]; exists {
		mc.currentSize -= existing.Size
		mc.removeFromLRU(key)
	}

	// Make room if necessary
	for mc.currentSize+actualSize > mc.maxSize && len(mc.lruOrder) > 0 {
		mc.evictLRU()
	}

	// Add the new item
	mc.items[key] = item
	mc.lruOrder = append([]string{key}, mc.lruOrder...)
	mc.currentSize += actualSize

	mc.stats.Size = mc.currentSize
	mc.stats.ItemCount = int64(len(mc.items))

	return nil
}

// Delete removes an item from the cache
func (mc *MemoryCache) Delete(ctx context.Context, key string) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if _, exists := mc.items[key]; !exists {
		return nil // Not an error if item doesn't exist
	}

	mc.removeItemLocked(key)
	return nil
}

// Exists checks if an item exists in the cache
func (mc *MemoryCache) Exists(ctx context.Context, key string) bool {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	item, exists := mc.items[key]
	if !exists {
		return false
	}

	// Check if expired
	return !item.IsExpired()
}

// Clear removes all items from the cache
func (mc *MemoryCache) Clear(ctx context.Context) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.items = make(map[string]*CacheItem)
	mc.lruOrder = make([]string, 0)
	mc.currentSize = 0
	mc.stats.Size = 0
	mc.stats.ItemCount = 0

	return nil
}

// Stats returns cache statistics
func (mc *MemoryCache) Stats() CacheStats {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	return CacheStats{
		Hits:        atomic.LoadInt64(&mc.stats.Hits),
		Misses:      atomic.LoadInt64(&mc.stats.Misses),
		Size:        mc.currentSize,
		MaxSize:     mc.maxSize,
		ItemCount:   int64(len(mc.items)),
		Evictions:   atomic.LoadInt64(&mc.stats.Evictions),
		Expirations: atomic.LoadInt64(&mc.stats.Expirations),
	}
}

// Close performs cleanup
func (mc *MemoryCache) Close() error {
	mc.cleanupTicker.Stop()
	close(mc.cleanupDone)
	return nil
}

// removeItemLocked removes an item from the cache (must hold lock)
func (mc *MemoryCache) removeItemLocked(key string) {
	item, exists := mc.items[key]
	if !exists {
		return
	}

	mc.currentSize -= item.Size
	delete(mc.items, key)
	mc.removeFromLRU(key)

	mc.stats.Size = mc.currentSize
	mc.stats.ItemCount = int64(len(mc.items))
}

// evictLRU evicts the least recently used item (must hold lock)
func (mc *MemoryCache) evictLRU() {
	if len(mc.lruOrder) == 0 {
		return
	}

	// Get the LRU item (last in the list)
	lruKey := mc.lruOrder[len(mc.lruOrder)-1]
	mc.removeItemLocked(lruKey)
	atomic.AddInt64(&mc.stats.Evictions, 1)
}

// promoteToFront moves a key to the front of the LRU list (must hold lock)
func (mc *MemoryCache) promoteToFront(key string) {
	mc.removeFromLRU(key)
	mc.lruOrder = append([]string{key}, mc.lruOrder...)
}

// removeFromLRU removes a key from the LRU list (must hold lock)
func (mc *MemoryCache) removeFromLRU(key string) {
	for i, k := range mc.lruOrder {
		if k == key {
			mc.lruOrder = append(mc.lruOrder[:i], mc.lruOrder[i+1:]...)
			return
		}
	}
}

// GetItem returns the full cache item (for internal use)
func (mc *MemoryCache) GetItem(key string) (*CacheItem, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	item, exists := mc.items[key]
	if !exists || item.IsExpired() {
		return nil, false
	}

	// Return a copy
	itemCopy := *item
	return &itemCopy, true
}
