package cache

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
	"time"
)

func TestTieredCache_BasicOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tiered-cache-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cache, err := NewTieredCache(TieredCacheConfig{
		MemorySizeMB:          1,
		DiskPath:              tempDir,
		DiskSizeGB:            1,
		DefaultTTL:            time.Hour,
		MemoryCleanupInterval: time.Hour,
		DiskCleanupInterval:   time.Hour,
		PromoteOnHit:          true,
		WriteThrough:          false,
	})
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Test Set and Get
	key := "test-key"
	data := []byte("test data for tiered cache")
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

func TestTieredCache_MemoryFirst(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tiered-cache-memfirst-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cache, err := NewTieredCache(TieredCacheConfig{
		MemorySizeMB:          1,
		DiskPath:              tempDir,
		DiskSizeGB:            1,
		DefaultTTL:            time.Hour,
		MemoryCleanupInterval: time.Hour,
		DiskCleanupInterval:   time.Hour,
		PromoteOnHit:          true,
		WriteThrough:          false,
	})
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Set data (goes to memory)
	key := "memory-key"
	data := []byte("memory data")
	cache.Set(ctx, key, bytes.NewReader(data), "text/plain", int64(len(data)), 0)

	// Get should hit memory
	cache.Get(ctx, key)
	cache.Get(ctx, key)

	stats := cache.DetailedStats()
	if stats.MemoryHits != 2 {
		t.Errorf("expected 2 memory hits, got %d", stats.MemoryHits)
	}
	if stats.DiskHits != 0 {
		t.Errorf("expected 0 disk hits, got %d", stats.DiskHits)
	}
}

func TestTieredCache_DiskFallback(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tiered-cache-diskfallback-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cache, err := NewTieredCache(TieredCacheConfig{
		MemorySizeMB:          1,
		DiskPath:              tempDir,
		DiskSizeGB:            1,
		DefaultTTL:            time.Hour,
		MemoryCleanupInterval: time.Hour,
		DiskCleanupInterval:   time.Hour,
		PromoteOnHit:          true,
		WriteThrough:          false,
	})
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Set directly to disk
	key := "disk-key"
	data := []byte("disk data")
	cache.SetToDisk(ctx, key, bytes.NewReader(data), "text/plain", int64(len(data)), 0)

	// Verify not in memory
	if cache.memory.Exists(ctx, key) {
		t.Error("key should not be in memory")
	}

	// Get should fall back to disk and promote to memory
	reader, _, found := cache.Get(ctx, key)
	if !found {
		t.Fatal("key not found")
	}
	reader.Close()

	stats := cache.DetailedStats()
	if stats.DiskHits != 1 {
		t.Errorf("expected 1 disk hit, got %d", stats.DiskHits)
	}
	if stats.Promotions != 1 {
		t.Errorf("expected 1 promotion, got %d", stats.Promotions)
	}

	// Now should be in memory
	if !cache.memory.Exists(ctx, key) {
		t.Error("key should be promoted to memory")
	}
}

func TestTieredCache_WriteThrough(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tiered-cache-writethrough-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cache, err := NewTieredCache(TieredCacheConfig{
		MemorySizeMB:          1,
		DiskPath:              tempDir,
		DiskSizeGB:            1,
		DefaultTTL:            time.Hour,
		MemoryCleanupInterval: time.Hour,
		DiskCleanupInterval:   time.Hour,
		PromoteOnHit:          true,
		WriteThrough:          true, // Enable write-through
	})
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	key := "writethrough-key"
	data := []byte("writethrough data")
	cache.Set(ctx, key, bytes.NewReader(data), "text/plain", int64(len(data)), 0)

	// Should be in both caches
	if !cache.memory.Exists(ctx, key) {
		t.Error("key should be in memory")
	}
	if !cache.disk.Exists(ctx, key) {
		t.Error("key should be in disk")
	}
}

func TestTieredCache_Clear(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tiered-cache-clear-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cache, err := NewTieredCache(TieredCacheConfig{
		MemorySizeMB:          1,
		DiskPath:              tempDir,
		DiskSizeGB:            1,
		DefaultTTL:            time.Hour,
		MemoryCleanupInterval: time.Hour,
		DiskCleanupInterval:   time.Hour,
		PromoteOnHit:          true,
		WriteThrough:          true,
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

	// Clear
	err = cache.Clear(ctx)
	if err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	stats := cache.Stats()
	if stats.ItemCount != 0 {
		t.Errorf("expected 0 items after clear, got %d", stats.ItemCount)
	}
}

func TestTieredCache_Warmup(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tiered-cache-warmup-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cache, err := NewTieredCache(TieredCacheConfig{
		MemorySizeMB:          1,
		DiskPath:              tempDir,
		DiskSizeGB:            1,
		DefaultTTL:            time.Hour,
		MemoryCleanupInterval: time.Hour,
		DiskCleanupInterval:   time.Hour,
		PromoteOnHit:          true,
		WriteThrough:          false,
	})
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Add items directly to disk
	for i := 0; i < 5; i++ {
		key := "disk-key-" + string(rune('0'+i))
		cache.SetToDisk(ctx, key, bytes.NewReader([]byte("data")), "text/plain", 4, 0)
	}

	// Warmup
	warmed := cache.Warmup(ctx, 3)
	if warmed != 3 {
		t.Errorf("expected 3 warmed items, got %d", warmed)
	}

	memStats := cache.memory.Stats()
	if memStats.ItemCount != 3 {
		t.Errorf("expected 3 items in memory, got %d", memStats.ItemCount)
	}
}

func TestTieredCache_DetailedStats(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tiered-cache-stats-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cache, err := NewTieredCache(TieredCacheConfig{
		MemorySizeMB:          1,
		DiskPath:              tempDir,
		DiskSizeGB:            1,
		DefaultTTL:            time.Hour,
		MemoryCleanupInterval: time.Hour,
		DiskCleanupInterval:   time.Hour,
		PromoteOnHit:          true,
		WriteThrough:          false,
	})
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Add to memory
	cache.Set(ctx, "mem-key", bytes.NewReader([]byte("mem data")), "text/plain", 8, 0)

	// Add to disk
	cache.SetToDisk(ctx, "disk-key", bytes.NewReader([]byte("disk data")), "text/plain", 9, 0)

	// Memory hit
	cache.Get(ctx, "mem-key")

	// Disk hit (with promotion)
	cache.Get(ctx, "disk-key")

	// Miss
	cache.Get(ctx, "nonexistent")

	stats := cache.DetailedStats()

	if stats.MemoryHits < 1 {
		t.Errorf("expected at least 1 memory hit, got %d", stats.MemoryHits)
	}
	if stats.DiskHits != 1 {
		t.Errorf("expected 1 disk hit, got %d", stats.DiskHits)
	}
	if stats.TotalMisses != 1 {
		t.Errorf("expected 1 total miss, got %d", stats.TotalMisses)
	}
}

func TestTieredCache_CombinedStats(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tiered-cache-combined-stats-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cache, err := NewTieredCache(TieredCacheConfig{
		MemorySizeMB:          1,
		DiskPath:              tempDir,
		DiskSizeGB:            1,
		DefaultTTL:            time.Hour,
		MemoryCleanupInterval: time.Hour,
		DiskCleanupInterval:   time.Hour,
		PromoteOnHit:          true,
		WriteThrough:          true,
	})
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Add items
	for i := 0; i < 3; i++ {
		key := "key-" + string(rune('0'+i))
		cache.Set(ctx, key, bytes.NewReader([]byte("data")), "text/plain", 4, 0)
	}

	stats := cache.Stats()

	// With write-through, items are in both caches
	// Combined item count may be duplicated
	if stats.ItemCount < 3 {
		t.Errorf("expected at least 3 items, got %d", stats.ItemCount)
	}

	// MaxSize should be sum of both caches
	if stats.MaxSize == 0 {
		t.Error("MaxSize should not be 0")
	}
}
