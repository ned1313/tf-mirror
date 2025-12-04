package cache

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/ned1313/terraform-mirror/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNoOpCache(t *testing.T) {
	cache := NewNoOpCache()
	require.NotNil(t, cache)
}

func TestNoOpCache_Get(t *testing.T) {
	cache := NewNoOpCache()
	ctx := context.Background()

	reader, contentType, found := cache.Get(ctx, "any-key")
	assert.Nil(t, reader)
	assert.Empty(t, contentType)
	assert.False(t, found)
}

func TestNoOpCache_Set(t *testing.T) {
	cache := NewNoOpCache()
	ctx := context.Background()

	data := []byte("test data")
	err := cache.Set(ctx, "key", bytes.NewReader(data), "text/plain", int64(len(data)), time.Hour)
	require.NoError(t, err)

	// Verify data is not stored (it's a no-op)
	_, _, found := cache.Get(ctx, "key")
	assert.False(t, found)
}

func TestNoOpCache_Delete(t *testing.T) {
	cache := NewNoOpCache()
	ctx := context.Background()

	err := cache.Delete(ctx, "any-key")
	require.NoError(t, err)
}

func TestNoOpCache_Exists(t *testing.T) {
	cache := NewNoOpCache()
	ctx := context.Background()

	exists := cache.Exists(ctx, "any-key")
	assert.False(t, exists)
}

func TestNoOpCache_Clear(t *testing.T) {
	cache := NewNoOpCache()
	ctx := context.Background()

	err := cache.Clear(ctx)
	require.NoError(t, err)
}

func TestNoOpCache_Stats(t *testing.T) {
	cache := NewNoOpCache()

	stats := cache.Stats()
	assert.Equal(t, CacheStats{}, stats)
	assert.Equal(t, int64(0), stats.ItemCount)
	assert.Equal(t, int64(0), stats.Size)
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(0), stats.Misses)
}

func TestNoOpCache_Close(t *testing.T) {
	cache := NewNoOpCache()

	err := cache.Close()
	require.NoError(t, err)
}

func TestNoOpCache_ImplementsCacheInterface(t *testing.T) {
	var _ Cache = NewNoOpCache()
}

func TestNoOpCache_SetDrainsReader(t *testing.T) {
	cache := NewNoOpCache()
	ctx := context.Background()

	// Create a reader that we can track
	data := []byte("test data that should be drained")
	reader := bytes.NewReader(data)

	err := cache.Set(ctx, "key", reader, "text/plain", int64(len(data)), time.Hour)
	require.NoError(t, err)

	// After Set, the reader should be fully consumed
	remaining := reader.Len()
	assert.Equal(t, 0, remaining, "reader should be fully drained")
}

func TestNoOpCache_MultipleOperations(t *testing.T) {
	cache := NewNoOpCache()
	ctx := context.Background()

	// Multiple sets should not error
	for i := 0; i < 10; i++ {
		data := []byte("test data")
		err := cache.Set(ctx, "key", bytes.NewReader(data), "text/plain", int64(len(data)), time.Hour)
		require.NoError(t, err)
	}

	// Multiple gets should all return not found
	for i := 0; i < 10; i++ {
		_, _, found := cache.Get(ctx, "key")
		assert.False(t, found)
	}

	// Multiple deletes should not error
	for i := 0; i < 10; i++ {
		err := cache.Delete(ctx, "key")
		require.NoError(t, err)
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Greater(t, config.MemorySizeMB, 0)
	assert.Greater(t, config.DiskSizeGB, 0)
	assert.NotEmpty(t, config.DiskPath)
	assert.Greater(t, config.DefaultTTL, time.Duration(0))
}

func TestCacheStats_HitRate(t *testing.T) {
	stats := CacheStats{
		Hits:   80,
		Misses: 20,
	}

	rate := stats.HitRate()
	assert.InDelta(t, 80.0, rate, 0.001)
}

func TestCacheStats_HitRate_ZeroTotal(t *testing.T) {
	stats := CacheStats{
		Hits:   0,
		Misses: 0,
	}

	rate := stats.HitRate()
	assert.Equal(t, float64(0), rate)
}

func TestCacheStats_UsagePercent(t *testing.T) {
	stats := CacheStats{
		Size:    500,
		MaxSize: 1000,
	}

	usage := stats.UsagePercent()
	assert.InDelta(t, 50.0, usage, 0.001)
}

func TestCacheStats_UsagePercent_ZeroMax(t *testing.T) {
	stats := CacheStats{
		Size:    100,
		MaxSize: 0,
	}

	usage := stats.UsagePercent()
	assert.Equal(t, float64(0), usage)
}

func TestNewFromConfig_TieredCache(t *testing.T) {
	tempDir := t.TempDir()
	cfg := config.CacheConfig{
		MemorySizeMB: 64,
		DiskPath:     tempDir,
		DiskSizeGB:   1,
		TTLSeconds:   3600,
	}

	cache, err := NewFromConfig(cfg)
	require.NoError(t, err)
	require.NotNil(t, cache)
	defer cache.Close()

	// Verify it's a tiered cache by type assertion
	_, isTiered := cache.(*TieredCache)
	assert.True(t, isTiered)
}

func TestNewFromConfig_MemoryOnlyCache(t *testing.T) {
	cfg := config.CacheConfig{
		MemorySizeMB: 64,
		DiskPath:     "",
		DiskSizeGB:   0,
		TTLSeconds:   3600,
	}

	cache, err := NewFromConfig(cfg)
	require.NoError(t, err)
	require.NotNil(t, cache)
	defer cache.Close()

	// Verify it's a memory cache
	_, isMemory := cache.(*MemoryCache)
	assert.True(t, isMemory)
}

func TestNewFromConfig_DiskOnlyCache(t *testing.T) {
	tempDir := t.TempDir()
	cfg := config.CacheConfig{
		MemorySizeMB: 0,
		DiskPath:     tempDir,
		DiskSizeGB:   1,
		TTLSeconds:   3600,
	}

	cache, err := NewFromConfig(cfg)
	require.NoError(t, err)
	require.NotNil(t, cache)
	defer cache.Close()

	// Verify it's a disk cache
	_, isDisk := cache.(*DiskCache)
	assert.True(t, isDisk)
}

func TestNewFromConfig_InvalidConfig(t *testing.T) {
	cfg := config.CacheConfig{
		MemorySizeMB: 0,
		DiskPath:     "",
		DiskSizeGB:   0,
		TTLSeconds:   3600,
	}

	_, err := NewFromConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid cache configuration")
}
