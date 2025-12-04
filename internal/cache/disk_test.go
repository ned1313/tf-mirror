package cache

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDiskCache_BasicOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "disk-cache-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cache, err := NewDiskCache(DiskCacheConfig{
		BasePath:        tempDir,
		MaxSizeGB:       1,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Test Set and Get
	key := "test-key"
	data := []byte("test data for disk cache")
	contentType := "text/plain"

	err = cache.Set(ctx, key, bytes.NewReader(data), contentType, int64(len(data)), 0)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	reader, ct, found := cache.Get(ctx, key)
	if !found {
		t.Fatal("Get returned not found for existing key")
	}
	if ct != contentType {
		t.Errorf("content type mismatch: got %s, want %s", ct, contentType)
	}

	retrieved, err := io.ReadAll(reader)
	reader.Close()
	if err != nil {
		t.Fatalf("failed to read data: %v", err)
	}
	if !bytes.Equal(retrieved, data) {
		t.Errorf("data mismatch: got %s, want %s", retrieved, data)
	}

	// Test Exists
	if !cache.Exists(ctx, key) {
		t.Error("Exists returned false for existing key")
	}

	// Test Delete
	err = cache.Delete(ctx, key)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if cache.Exists(ctx, key) {
		t.Error("Exists returned true after delete")
	}
}

func TestDiskCache_Persistence(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "disk-cache-persist-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	ctx := context.Background()

	// Create cache and add data
	cache1, err := NewDiskCache(DiskCacheConfig{
		BasePath:        tempDir,
		MaxSizeGB:       1,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	key := "persistent-key"
	data := []byte("persistent data")
	cache1.Set(ctx, key, bytes.NewReader(data), "text/plain", int64(len(data)), 0)
	cache1.Close()

	// Reopen cache and verify data persists
	cache2, err := NewDiskCache(DiskCacheConfig{
		BasePath:        tempDir,
		MaxSizeGB:       1,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("failed to reopen cache: %v", err)
	}
	defer cache2.Close()

	reader, _, found := cache2.Get(ctx, key)
	if !found {
		t.Fatal("data not found after cache reopen")
	}
	retrieved, _ := io.ReadAll(reader)
	reader.Close()

	if !bytes.Equal(retrieved, data) {
		t.Error("data changed after cache reopen")
	}
}

func TestDiskCache_TTLExpiration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "disk-cache-ttl-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cache, err := NewDiskCache(DiskCacheConfig{
		BasePath:        tempDir,
		MaxSizeGB:       1,
		DefaultTTL:      50 * time.Millisecond,
		CleanupInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	key := "expiring-key"
	data := []byte("expiring data")

	cache.Set(ctx, key, bytes.NewReader(data), "text/plain", int64(len(data)), 50*time.Millisecond)

	// Should exist immediately
	if !cache.Exists(ctx, key) {
		t.Error("item should exist immediately")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired
	if cache.Exists(ctx, key) {
		t.Error("item should have expired")
	}
}

func TestDiskCache_Clear(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "disk-cache-clear-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cache, err := NewDiskCache(DiskCacheConfig{
		BasePath:        tempDir,
		MaxSizeGB:       1,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Add items
	for i := 0; i < 5; i++ {
		key := "key-" + string(rune('0'+i))
		cache.Set(ctx, key, bytes.NewReader([]byte("data")), "text/plain", 4, 0)
	}

	stats := cache.Stats()
	if stats.ItemCount != 5 {
		t.Errorf("expected 5 items, got %d", stats.ItemCount)
	}

	// Clear
	err = cache.Clear(ctx)
	if err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	stats = cache.Stats()
	if stats.ItemCount != 0 {
		t.Errorf("expected 0 items after clear, got %d", stats.ItemCount)
	}
}

func TestDiskCache_LRUEviction(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "disk-cache-lru-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a very small cache (1MB)
	cache, err := NewDiskCache(DiskCacheConfig{
		BasePath:        tempDir,
		MaxSizeGB:       1, // 1GB but we'll fill it
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Add items and verify stats tracking
	data := []byte("test data")
	cache.Set(ctx, "item1", bytes.NewReader(data), "text/plain", int64(len(data)), 0)
	cache.Set(ctx, "item2", bytes.NewReader(data), "text/plain", int64(len(data)), 0)

	stats := cache.Stats()
	if stats.ItemCount != 2 {
		t.Errorf("expected 2 items, got %d", stats.ItemCount)
	}
}

func TestDiskCache_Stats(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "disk-cache-stats-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cache, err := NewDiskCache(DiskCacheConfig{
		BasePath:        tempDir,
		MaxSizeGB:       1,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Add an item
	data := []byte("test data")
	cache.Set(ctx, "key1", bytes.NewReader(data), "text/plain", int64(len(data)), 0)

	// Hit
	cache.Get(ctx, "key1")
	cache.Get(ctx, "key1")

	// Miss
	cache.Get(ctx, "nonexistent")

	stats := cache.Stats()
	if stats.Hits != 2 {
		t.Errorf("expected 2 hits, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", stats.Misses)
	}
}

func TestDiskCache_ListKeys(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "disk-cache-list-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cache, err := NewDiskCache(DiskCacheConfig{
		BasePath:        tempDir,
		MaxSizeGB:       1,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Add items
	cache.Set(ctx, "prefix/item1", bytes.NewReader([]byte("data")), "text/plain", 4, 0)
	cache.Set(ctx, "prefix/item2", bytes.NewReader([]byte("data")), "text/plain", 4, 0)
	cache.Set(ctx, "other/item3", bytes.NewReader([]byte("data")), "text/plain", 4, 0)

	// List all keys
	keys := cache.ListKeys()
	if len(keys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(keys))
	}

	// List with prefix
	prefixKeys := cache.HasPrefix("prefix/")
	if len(prefixKeys) != 2 {
		t.Errorf("expected 2 prefix keys, got %d", len(prefixKeys))
	}
}

func TestDiskCache_FileStructure(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "disk-cache-structure-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cache, err := NewDiskCache(DiskCacheConfig{
		BasePath:        tempDir,
		MaxSizeGB:       1,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	ctx := context.Background()
	cache.Set(ctx, "test-key", bytes.NewReader([]byte("data")), "text/plain", 4, 0)
	cache.Close()

	// Verify index file exists
	indexPath := filepath.Join(tempDir, "index.json")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		t.Error("index.json should exist")
	}

	// Verify data directory exists
	dataPath := filepath.Join(tempDir, "data")
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		t.Error("data directory should exist")
	}
}

func TestDiskCache_InvalidBasePath(t *testing.T) {
	_, err := NewDiskCache(DiskCacheConfig{
		BasePath:        "",
		MaxSizeGB:       1,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Hour,
	})
	if err == nil {
		t.Error("expected error for empty base path")
	}
}
