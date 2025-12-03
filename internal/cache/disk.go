package cache

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// DiskCache implements a disk-based cache with TTL support
type DiskCache struct {
	mu sync.RWMutex

	// basePath is the directory for cache storage
	basePath string

	// maxSize is the maximum cache size in bytes
	maxSize int64

	// currentSize is the current cache size in bytes
	currentSize int64

	// defaultTTL is the default TTL for items
	defaultTTL time.Duration

	// stats tracks cache statistics
	stats CacheStats

	// index keeps track of cached items and their metadata
	index map[string]*diskCacheEntry

	// cleanupTicker for periodic cleanup
	cleanupTicker *time.Ticker
	cleanupDone   chan struct{}
}

// diskCacheEntry represents metadata for a cached item on disk
type diskCacheEntry struct {
	Key          string    `json:"key"`
	Filename     string    `json:"filename"`
	ContentType  string    `json:"content_type"`
	Size         int64     `json:"size"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	LastAccessed time.Time `json:"last_accessed"`
	AccessCount  int64     `json:"access_count"`
}

// DiskCacheConfig contains configuration for the disk cache
type DiskCacheConfig struct {
	// BasePath is the directory for cache storage
	BasePath string

	// MaxSizeGB is the maximum cache size in gigabytes
	MaxSizeGB int

	// DefaultTTL is the default TTL for cached items
	DefaultTTL time.Duration

	// CleanupInterval is how often to run cleanup
	CleanupInterval time.Duration
}

// NewDiskCache creates a new disk-based cache
func NewDiskCache(cfg DiskCacheConfig) (*DiskCache, error) {
	if cfg.BasePath == "" {
		return nil, fmt.Errorf("base path is required")
	}

	if cfg.MaxSizeGB <= 0 {
		cfg.MaxSizeGB = 10 // Default to 10GB
	}

	if cfg.DefaultTTL <= 0 {
		cfg.DefaultTTL = 24 * time.Hour
	}

	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = 10 * time.Minute
	}

	// Create cache directories
	dataPath := filepath.Join(cfg.BasePath, "data")
	if err := os.MkdirAll(dataPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	maxSize := int64(cfg.MaxSizeGB) * 1024 * 1024 * 1024

	dc := &DiskCache{
		basePath:   cfg.BasePath,
		maxSize:    maxSize,
		defaultTTL: cfg.DefaultTTL,
		index:      make(map[string]*diskCacheEntry),
		stats: CacheStats{
			MaxSize: maxSize,
		},
		cleanupDone: make(chan struct{}),
	}

	// Load existing index
	if err := dc.loadIndex(); err != nil {
		// Log but don't fail - we can rebuild
		fmt.Printf("Warning: failed to load cache index: %v\n", err)
	}

	// Calculate current size
	dc.calculateSize()

	// Start cleanup goroutine
	dc.cleanupTicker = time.NewTicker(cfg.CleanupInterval)
	go dc.cleanupLoop()

	return dc, nil
}

// cleanupLoop periodically removes expired items
func (dc *DiskCache) cleanupLoop() {
	for {
		select {
		case <-dc.cleanupTicker.C:
			dc.removeExpired()
		case <-dc.cleanupDone:
			return
		}
	}
}

// removeExpired removes all expired items from the cache
func (dc *DiskCache) removeExpired() {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	now := time.Now()
	var expired []string

	for key, entry := range dc.index {
		if !entry.ExpiresAt.IsZero() && now.After(entry.ExpiresAt) {
			expired = append(expired, key)
		}
	}

	for _, key := range expired {
		dc.removeItemLocked(key)
		atomic.AddInt64(&dc.stats.Expirations, 1)
	}

	// Save index after cleanup
	dc.saveIndexLocked()
}

// Get retrieves an item from the cache
func (dc *DiskCache) Get(ctx context.Context, key string) (io.ReadCloser, string, bool) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	entry, exists := dc.index[key]
	if !exists {
		atomic.AddInt64(&dc.stats.Misses, 1)
		return nil, "", false
	}

	// Check if expired
	if !entry.ExpiresAt.IsZero() && time.Now().After(entry.ExpiresAt) {
		dc.removeItemLocked(key)
		atomic.AddInt64(&dc.stats.Expirations, 1)
		atomic.AddInt64(&dc.stats.Misses, 1)
		return nil, "", false
	}

	// Open the file
	filePath := dc.dataFilePath(entry.Filename)
	file, err := os.Open(filePath)
	if err != nil {
		// File doesn't exist, remove from index
		dc.removeItemLocked(key)
		atomic.AddInt64(&dc.stats.Misses, 1)
		return nil, "", false
	}

	// Update access info
	entry.LastAccessed = time.Now()
	entry.AccessCount++

	atomic.AddInt64(&dc.stats.Hits, 1)

	return file, entry.ContentType, true
}

// Set stores an item in the cache
func (dc *DiskCache) Set(ctx context.Context, key string, data io.Reader, contentType string, size int64, ttl time.Duration) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	// Generate filename from key hash
	filename := dc.hashKey(key)

	// Read data to temporary buffer to get actual size
	dataBytes, err := io.ReadAll(data)
	if err != nil {
		return fmt.Errorf("failed to read data: %w", err)
	}

	actualSize := int64(len(dataBytes))

	// Check if item is too large for the cache
	if actualSize > dc.maxSize {
		return fmt.Errorf("item size (%d bytes) exceeds cache size (%d bytes)", actualSize, dc.maxSize)
	}

	if ttl <= 0 {
		ttl = dc.defaultTTL
	}

	dc.mu.Lock()
	defer dc.mu.Unlock()

	// If item already exists, remove it first
	if existing, exists := dc.index[key]; exists {
		dc.currentSize -= existing.Size
		dc.removeFileLocked(existing.Filename)
	}

	// Make room if necessary
	for dc.currentSize+actualSize > dc.maxSize && len(dc.index) > 0 {
		dc.evictLRU()
	}

	// Write data to file (ensuring subdirectory exists)
	if err := dc.writeDataFile(filename, dataBytes); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	now := time.Now()
	entry := &diskCacheEntry{
		Key:          key,
		Filename:     filename,
		ContentType:  contentType,
		Size:         actualSize,
		CreatedAt:    now,
		ExpiresAt:    now.Add(ttl),
		LastAccessed: now,
		AccessCount:  0,
	}

	dc.index[key] = entry
	dc.currentSize += actualSize

	dc.stats.Size = dc.currentSize
	dc.stats.ItemCount = int64(len(dc.index))

	// Save index
	dc.saveIndexLocked()

	return nil
}

// Delete removes an item from the cache
func (dc *DiskCache) Delete(ctx context.Context, key string) error {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	if _, exists := dc.index[key]; !exists {
		return nil // Not an error if item doesn't exist
	}

	dc.removeItemLocked(key)
	dc.saveIndexLocked()

	return nil
}

// Exists checks if an item exists in the cache
func (dc *DiskCache) Exists(ctx context.Context, key string) bool {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	entry, exists := dc.index[key]
	if !exists {
		return false
	}

	// Check if expired
	if !entry.ExpiresAt.IsZero() && time.Now().After(entry.ExpiresAt) {
		return false
	}

	// Verify file exists
	filePath := dc.dataFilePath(entry.Filename)
	_, err := os.Stat(filePath)
	return err == nil
}

// Clear removes all items from the cache
func (dc *DiskCache) Clear(ctx context.Context) error {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	// Remove all data files
	dataPath := filepath.Join(dc.basePath, "data")
	if err := os.RemoveAll(dataPath); err != nil {
		return fmt.Errorf("failed to clear cache: %w", err)
	}

	// Recreate data directory
	if err := os.MkdirAll(dataPath, 0755); err != nil {
		return fmt.Errorf("failed to recreate cache directory: %w", err)
	}

	// Clear index
	dc.index = make(map[string]*diskCacheEntry)
	dc.currentSize = 0
	dc.stats.Size = 0
	dc.stats.ItemCount = 0

	// Save empty index
	dc.saveIndexLocked()

	return nil
}

// Stats returns cache statistics
func (dc *DiskCache) Stats() CacheStats {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	return CacheStats{
		Hits:        atomic.LoadInt64(&dc.stats.Hits),
		Misses:      atomic.LoadInt64(&dc.stats.Misses),
		Size:        dc.currentSize,
		MaxSize:     dc.maxSize,
		ItemCount:   int64(len(dc.index)),
		Evictions:   atomic.LoadInt64(&dc.stats.Evictions),
		Expirations: atomic.LoadInt64(&dc.stats.Expirations),
	}
}

// Close performs cleanup
func (dc *DiskCache) Close() error {
	dc.cleanupTicker.Stop()
	close(dc.cleanupDone)

	// Save final index state
	dc.mu.Lock()
	defer dc.mu.Unlock()
	return dc.saveIndexLocked()
}

// removeItemLocked removes an item from the cache (must hold lock)
func (dc *DiskCache) removeItemLocked(key string) {
	entry, exists := dc.index[key]
	if !exists {
		return
	}

	dc.currentSize -= entry.Size
	dc.removeFileLocked(entry.Filename)
	delete(dc.index, key)

	dc.stats.Size = dc.currentSize
	dc.stats.ItemCount = int64(len(dc.index))
}

// removeFileLocked removes a file from disk (must hold lock)
func (dc *DiskCache) removeFileLocked(filename string) {
	filePath := dc.dataFilePath(filename)
	os.Remove(filePath) // Ignore errors
}

// evictLRU evicts the least recently used item (must hold lock)
func (dc *DiskCache) evictLRU() {
	if len(dc.index) == 0 {
		return
	}

	// Find the LRU item
	var lruKey string
	var lruTime time.Time
	first := true

	for key, entry := range dc.index {
		if first || entry.LastAccessed.Before(lruTime) {
			lruKey = key
			lruTime = entry.LastAccessed
			first = false
		}
	}

	if lruKey != "" {
		dc.removeItemLocked(lruKey)
		atomic.AddInt64(&dc.stats.Evictions, 1)
	}
}

// hashKey generates a filename from a cache key
func (dc *DiskCache) hashKey(key string) string {
	h := sha256.New()
	h.Write([]byte(key))
	return hex.EncodeToString(h.Sum(nil))
}

// dataFilePath returns the full path for a data file
func (dc *DiskCache) dataFilePath(filename string) string {
	// Use subdirectories based on first 2 characters for better filesystem performance
	subdir := filename[:2]
	return filepath.Join(dc.basePath, "data", subdir, filename)
}

// indexFilePath returns the path to the index file
func (dc *DiskCache) indexFilePath() string {
	return filepath.Join(dc.basePath, "index.json")
}

// loadIndex loads the cache index from disk
func (dc *DiskCache) loadIndex() error {
	data, err := os.ReadFile(dc.indexFilePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No index yet
		}
		return err
	}

	var entries []*diskCacheEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	// Rebuild index map
	dc.index = make(map[string]*diskCacheEntry)
	for _, entry := range entries {
		dc.index[entry.Key] = entry
	}

	return nil
}

// saveIndexLocked saves the cache index to disk (must hold lock)
func (dc *DiskCache) saveIndexLocked() error {
	entries := make([]*diskCacheEntry, 0, len(dc.index))
	for _, entry := range dc.index {
		entries = append(entries, entry)
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(dc.indexFilePath(), data, 0644)
}

// calculateSize calculates the current cache size from disk
func (dc *DiskCache) calculateSize() {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	var totalSize int64
	dataPath := filepath.Join(dc.basePath, "data")

	filepath.Walk(dataPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	dc.currentSize = totalSize
	dc.stats.Size = totalSize
}

// GetEntry returns the cache entry for a key (for internal use)
func (dc *DiskCache) GetEntry(key string) (*diskCacheEntry, bool) {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	entry, exists := dc.index[key]
	if !exists {
		return nil, false
	}

	// Check if expired
	if !entry.ExpiresAt.IsZero() && time.Now().After(entry.ExpiresAt) {
		return nil, false
	}

	// Return a copy
	entryCopy := *entry
	return &entryCopy, true
}

// SetFromBytes is a convenience method for setting data from bytes
func (dc *DiskCache) SetFromBytes(ctx context.Context, key string, data []byte, contentType string, ttl time.Duration) error {
	return dc.Set(ctx, key, bytes.NewReader(data), contentType, int64(len(data)), ttl)
}

// Ensure subdirectory exists before writing
func (dc *DiskCache) ensureSubdir(filename string) error {
	subdir := filename[:2]
	dirPath := filepath.Join(dc.basePath, "data", subdir)
	return os.MkdirAll(dirPath, 0755)
}

// dataFilePath needs to ensure subdirectory exists when writing
func (dc *DiskCache) writeDataFile(filename string, data []byte) error {
	if err := dc.ensureSubdir(filename); err != nil {
		return err
	}
	return os.WriteFile(dc.dataFilePath(filename), data, 0644)
}

// ListKeys returns all non-expired cache keys
func (dc *DiskCache) ListKeys() []string {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	now := time.Now()
	keys := make([]string, 0, len(dc.index))

	for key, entry := range dc.index {
		if entry.ExpiresAt.IsZero() || now.Before(entry.ExpiresAt) {
			keys = append(keys, key)
		}
	}

	return keys
}

// HasPrefix returns all keys with the given prefix
func (dc *DiskCache) HasPrefix(prefix string) []string {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	now := time.Now()
	var keys []string

	for key, entry := range dc.index {
		if strings.HasPrefix(key, prefix) {
			if entry.ExpiresAt.IsZero() || now.Before(entry.ExpiresAt) {
				keys = append(keys, key)
			}
		}
	}

	return keys
}
