package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ned1313/terraform-mirror/internal/cache"
)

// CacheStatsResponse represents cache statistics
type CacheStatsResponse struct {
	Enabled      bool              `json:"enabled"`
	Hits         int64             `json:"hits"`
	Misses       int64             `json:"misses"`
	HitRate      float64           `json:"hit_rate"`
	HitRateStr   string            `json:"hit_rate_str"`
	Size         int64             `json:"size"`
	SizeHuman    string            `json:"size_human"`
	MaxSize      int64             `json:"max_size"`
	MaxSizeHuman string            `json:"max_size_human"`
	UsagePercent float64           `json:"usage_percent"`
	ItemCount    int64             `json:"item_count"`
	Evictions    int64             `json:"evictions"`
	Expirations  int64             `json:"expirations"`
	Efficiency   *CacheEfficiency  `json:"efficiency"`
	Config       *CacheConfigInfo  `json:"config"`
	Tiered       *TieredCacheStats `json:"tiered,omitempty"`
}

// CacheEfficiency contains efficiency metrics
type CacheEfficiency struct {
	// TotalRequests is the total number of cache requests
	TotalRequests int64 `json:"total_requests"`
	// BytesSaved is estimated bytes saved by cache hits (hits * avg item size)
	BytesSaved int64 `json:"bytes_saved"`
	// BytesSavedHuman is human-readable bytes saved
	BytesSavedHuman string `json:"bytes_saved_human"`
	// EvictionRate is the percentage of items evicted vs total items added
	EvictionRate float64 `json:"eviction_rate"`
	// EvictionRateStr is the eviction rate as a string
	EvictionRateStr string `json:"eviction_rate_str"`
	// AverageItemSize is the average size of cached items
	AverageItemSize int64 `json:"average_item_size"`
	// AverageItemSizeHuman is human-readable average item size
	AverageItemSizeHuman string `json:"average_item_size_human"`
}

// CacheConfigInfo contains cache configuration information
type CacheConfigInfo struct {
	MemorySizeMB int    `json:"memory_size_mb"`
	DiskSizeGB   int    `json:"disk_size_gb"`
	DiskPath     string `json:"disk_path"`
	TTLSeconds   int    `json:"ttl_seconds"`
}

// TieredCacheStats contains detailed tiered cache statistics
type TieredCacheStats struct {
	// Memory cache stats
	MemoryHits         int64   `json:"memory_hits"`
	MemoryMisses       int64   `json:"memory_misses"`
	MemoryHitRate      float64 `json:"memory_hit_rate"`
	MemoryHitRateStr   string  `json:"memory_hit_rate_str"`
	MemorySize         int64   `json:"memory_size"`
	MemorySizeHuman    string  `json:"memory_size_human"`
	MemoryMaxSize      int64   `json:"memory_max_size"`
	MemoryMaxSizeHuman string  `json:"memory_max_size_human"`
	MemoryUsagePercent float64 `json:"memory_usage_percent"`
	MemoryItemCount    int64   `json:"memory_item_count"`
	MemoryEvictions    int64   `json:"memory_evictions"`
	MemoryExpirations  int64   `json:"memory_expirations"`

	// Disk cache stats
	DiskHits         int64   `json:"disk_hits"`
	DiskMisses       int64   `json:"disk_misses"`
	DiskHitRate      float64 `json:"disk_hit_rate"`
	DiskHitRateStr   string  `json:"disk_hit_rate_str"`
	DiskSize         int64   `json:"disk_size"`
	DiskSizeHuman    string  `json:"disk_size_human"`
	DiskMaxSize      int64   `json:"disk_max_size"`
	DiskMaxSizeHuman string  `json:"disk_max_size_human"`
	DiskUsagePercent float64 `json:"disk_usage_percent"`
	DiskItemCount    int64   `json:"disk_item_count"`
	DiskEvictions    int64   `json:"disk_evictions"`
	DiskExpirations  int64   `json:"disk_expirations"`

	// Combined stats
	TotalHits   int64 `json:"total_hits"`
	TotalMisses int64 `json:"total_misses"`
	Promotions  int64 `json:"promotions"`
}

// handleCacheStats returns cache statistics
// GET /admin/api/stats/cache
func (s *Server) handleCacheStats(w http.ResponseWriter, r *http.Request) {
	stats := s.cache.Stats()

	hitRate := stats.HitRate()
	response := CacheStatsResponse{
		Enabled:      true,
		Hits:         stats.Hits,
		Misses:       stats.Misses,
		HitRate:      hitRate,
		HitRateStr:   formatPercent(hitRate),
		Size:         stats.Size,
		SizeHuman:    formatBytes(stats.Size),
		MaxSize:      stats.MaxSize,
		MaxSizeHuman: formatBytes(stats.MaxSize),
		UsagePercent: stats.UsagePercent(),
		ItemCount:    stats.ItemCount,
		Evictions:    stats.Evictions,
		Expirations:  stats.Expirations,
	}

	// Check if it's a NoOp cache (disabled)
	if stats.MaxSize == 0 && stats.Hits == 0 && stats.Misses == 0 {
		response.Enabled = false
	}

	// Add cache configuration info
	response.Config = &CacheConfigInfo{
		MemorySizeMB: s.config.Cache.MemorySizeMB,
		DiskSizeGB:   s.config.Cache.DiskSizeGB,
		DiskPath:     s.config.Cache.DiskPath,
		TTLSeconds:   s.config.Cache.TTLSeconds,
	}

	// Calculate efficiency metrics
	totalRequests := stats.Hits + stats.Misses
	var avgItemSize int64
	if stats.ItemCount > 0 {
		avgItemSize = stats.Size / stats.ItemCount
	}

	var evictionRate float64
	totalItemsProcessed := stats.ItemCount + stats.Evictions + stats.Expirations
	if totalItemsProcessed > 0 {
		evictionRate = float64(stats.Evictions) / float64(totalItemsProcessed) * 100
	}

	// Estimate bytes saved (cache hits * average item size)
	bytesSaved := stats.Hits * avgItemSize

	response.Efficiency = &CacheEfficiency{
		TotalRequests:        totalRequests,
		BytesSaved:           bytesSaved,
		BytesSavedHuman:      formatBytes(bytesSaved),
		EvictionRate:         evictionRate,
		EvictionRateStr:      formatPercent(evictionRate),
		AverageItemSize:      avgItemSize,
		AverageItemSizeHuman: formatBytes(avgItemSize),
	}

	// If it's a tiered cache, get detailed stats
	if tieredCache, ok := s.cache.(*cache.TieredCache); ok {
		detailed := tieredCache.DetailedStats()

		// Calculate memory hit rate
		memoryTotal := detailed.MemoryHits + detailed.MemoryMisses
		var memoryHitRate float64
		if memoryTotal > 0 {
			memoryHitRate = float64(detailed.MemoryHits) / float64(memoryTotal) * 100
		}

		// Calculate disk hit rate
		diskTotal := detailed.DiskHits + detailed.DiskMisses
		var diskHitRate float64
		if diskTotal > 0 {
			diskHitRate = float64(detailed.DiskHits) / float64(diskTotal) * 100
		}

		// Calculate usage percentages
		var memoryUsagePercent, diskUsagePercent float64
		if detailed.MemoryMaxSize > 0 {
			memoryUsagePercent = float64(detailed.MemorySize) / float64(detailed.MemoryMaxSize) * 100
		}
		if detailed.DiskMaxSize > 0 {
			diskUsagePercent = float64(detailed.DiskSize) / float64(detailed.DiskMaxSize) * 100
		}

		response.Tiered = &TieredCacheStats{
			MemoryHits:         detailed.MemoryHits,
			MemoryMisses:       detailed.MemoryMisses,
			MemoryHitRate:      memoryHitRate,
			MemoryHitRateStr:   formatPercent(memoryHitRate),
			MemorySize:         detailed.MemorySize,
			MemorySizeHuman:    formatBytes(detailed.MemorySize),
			MemoryMaxSize:      detailed.MemoryMaxSize,
			MemoryMaxSizeHuman: formatBytes(detailed.MemoryMaxSize),
			MemoryUsagePercent: memoryUsagePercent,
			MemoryItemCount:    detailed.MemoryItemCount,
			MemoryEvictions:    detailed.MemoryEvictions,
			MemoryExpirations:  detailed.MemoryExpirations,

			DiskHits:         detailed.DiskHits,
			DiskMisses:       detailed.DiskMisses,
			DiskHitRate:      diskHitRate,
			DiskHitRateStr:   formatPercent(diskHitRate),
			DiskSize:         detailed.DiskSize,
			DiskSizeHuman:    formatBytes(detailed.DiskSize),
			DiskMaxSize:      detailed.DiskMaxSize,
			DiskMaxSizeHuman: formatBytes(detailed.DiskMaxSize),
			DiskUsagePercent: diskUsagePercent,
			DiskItemCount:    detailed.DiskItemCount,
			DiskEvictions:    detailed.DiskEvictions,
			DiskExpirations:  detailed.DiskExpirations,

			TotalHits:   detailed.TotalHits,
			TotalMisses: detailed.TotalMisses,
			Promotions:  detailed.Promotions,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// formatPercent formats a percentage value as a string
func formatPercent(percent float64) string {
	return fmt.Sprintf("%.2f%%", percent)
}

// ClearCacheResponse represents the response from clearing the cache
type ClearCacheResponse struct {
	Message      string `json:"message"`
	ItemsCleared int64  `json:"items_cleared"`
}

// handleClearCache clears all items from the cache
// POST /admin/api/stats/cache/clear
func (s *Server) handleClearCache(w http.ResponseWriter, r *http.Request) {
	// Get current item count before clearing
	stats := s.cache.Stats()
	itemsCleared := stats.ItemCount

	// Clear the cache
	if err := s.cache.Clear(r.Context()); err != nil {
		respondError(w, http.StatusInternalServerError, "cache_error", "Failed to clear cache: "+err.Error())
		return
	}

	// Log the action
	s.logAuditEvent(r, "clear_cache", "cache", "", true, "", map[string]interface{}{
		"items_cleared": itemsCleared,
	})

	response := ClearCacheResponse{
		Message:      "Cache cleared successfully",
		ItemsCleared: itemsCleared,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
