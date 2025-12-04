package cache

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"
)

func TestMemoryCache_BasicOperations(t *testing.T) {
	cache, err := NewMemoryCache(MemoryCacheConfig{
		MaxSizeMB:       1,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Hour, // Don't run cleanup during tests
	})
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Test Set and Get
	key := "test-key"
	data := []byte("test data")
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

	_, _, found = cache.Get(ctx, key)
	if found {
		t.Error("Get found item after delete")
	}
}

func TestMemoryCache_LRUEviction(t *testing.T) {
	// Create a small cache (1KB)
	cache, err := NewMemoryCache(MemoryCacheConfig{
		MaxSizeMB:       1,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Add items until we exceed capacity
	// Each item is ~500KB
	data1 := make([]byte, 500*1024)
	data2 := make([]byte, 500*1024)
	data3 := make([]byte, 500*1024)

	cache.Set(ctx, "item1", bytes.NewReader(data1), "application/octet-stream", int64(len(data1)), 0)
	cache.Set(ctx, "item2", bytes.NewReader(data2), "application/octet-stream", int64(len(data2)), 0)

	// Access item1 to make it more recently used
	cache.Get(ctx, "item1")

	// Add item3 which should evict item2 (LRU)
	cache.Set(ctx, "item3", bytes.NewReader(data3), "application/octet-stream", int64(len(data3)), 0)

	// item1 should still exist (was accessed)
	if !cache.Exists(ctx, "item1") {
		t.Error("item1 should not have been evicted (was recently accessed)")
	}

	// item2 should have been evicted (LRU)
	if cache.Exists(ctx, "item2") {
		t.Error("item2 should have been evicted (LRU)")
	}

	// item3 should exist
	if !cache.Exists(ctx, "item3") {
		t.Error("item3 should exist")
	}

	// Check eviction stats
	stats := cache.Stats()
	if stats.Evictions == 0 {
		t.Error("expected evictions to be tracked")
	}
}

func TestMemoryCache_TTLExpiration(t *testing.T) {
	cache, err := NewMemoryCache(MemoryCacheConfig{
		MaxSizeMB:       1,
		DefaultTTL:      50 * time.Millisecond,
		CleanupInterval: time.Hour, // Don't use cleanup for this test
	})
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	key := "expiring-key"
	data := []byte("expiring data")

	err = cache.Set(ctx, key, bytes.NewReader(data), "text/plain", int64(len(data)), 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Should exist immediately
	if !cache.Exists(ctx, key) {
		t.Error("item should exist immediately after set")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired now
	if cache.Exists(ctx, key) {
		t.Error("item should have expired")
	}

	_, _, found := cache.Get(ctx, key)
	if found {
		t.Error("Get should return not found for expired item")
	}
}

func TestMemoryCache_Clear(t *testing.T) {
	cache, err := NewMemoryCache(MemoryCacheConfig{
		MaxSizeMB:       1,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Add some items
	for i := 0; i < 10; i++ {
		key := "key-" + string(rune('0'+i))
		cache.Set(ctx, key, bytes.NewReader([]byte("data")), "text/plain", 4, 0)
	}

	stats := cache.Stats()
	if stats.ItemCount != 10 {
		t.Errorf("expected 10 items, got %d", stats.ItemCount)
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
	if stats.Size != 0 {
		t.Errorf("expected 0 size after clear, got %d", stats.Size)
	}
}

func TestMemoryCache_Stats(t *testing.T) {
	cache, err := NewMemoryCache(MemoryCacheConfig{
		MaxSizeMB:       1,
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
	if stats.ItemCount != 1 {
		t.Errorf("expected 1 item, got %d", stats.ItemCount)
	}
	if stats.Size != int64(len(data)) {
		t.Errorf("expected size %d, got %d", len(data), stats.Size)
	}
}

func TestMemoryCache_EmptyKey(t *testing.T) {
	cache, err := NewMemoryCache(MemoryCacheConfig{
		MaxSizeMB:       1,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	err = cache.Set(ctx, "", bytes.NewReader([]byte("data")), "text/plain", 4, 0)
	if err == nil {
		t.Error("expected error for empty key")
	}
}

func TestMemoryCache_ItemTooLarge(t *testing.T) {
	// Create a tiny cache (1MB)
	cache, err := NewMemoryCache(MemoryCacheConfig{
		MaxSizeMB:       1,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Try to store 2MB item
	largeData := make([]byte, 2*1024*1024)
	err = cache.Set(ctx, "large", bytes.NewReader(largeData), "application/octet-stream", int64(len(largeData)), 0)
	if err == nil {
		t.Error("expected error for item larger than cache")
	}
}

func TestMemoryCache_OverwriteExisting(t *testing.T) {
	cache, err := NewMemoryCache(MemoryCacheConfig{
		MaxSizeMB:       1,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Hour,
	})
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	key := "test-key"
	data1 := []byte("original data")
	data2 := []byte("updated data")

	// Set original
	cache.Set(ctx, key, bytes.NewReader(data1), "text/plain", int64(len(data1)), 0)

	// Overwrite
	cache.Set(ctx, key, bytes.NewReader(data2), "text/plain", int64(len(data2)), 0)

	// Verify updated
	reader, _, found := cache.Get(ctx, key)
	if !found {
		t.Fatal("key not found")
	}
	retrieved, _ := io.ReadAll(reader)
	reader.Close()

	if !bytes.Equal(retrieved, data2) {
		t.Errorf("expected updated data, got %s", retrieved)
	}

	// Should still be 1 item
	stats := cache.Stats()
	if stats.ItemCount != 1 {
		t.Errorf("expected 1 item, got %d", stats.ItemCount)
	}
}
