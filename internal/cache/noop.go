package cache

import (
	"context"
	"io"
	"time"
)

// noOpCache is a cache implementation that does nothing
// Used when caching is disabled
type noOpCache struct{}

// Get always returns not found
func (n *noOpCache) Get(ctx context.Context, key string) (io.ReadCloser, string, bool) {
	return nil, "", false
}

// Set does nothing
func (n *noOpCache) Set(ctx context.Context, key string, data io.Reader, contentType string, size int64, ttl time.Duration) error {
	// Drain the reader to avoid blocking
	io.Copy(io.Discard, data)
	return nil
}

// Delete does nothing
func (n *noOpCache) Delete(ctx context.Context, key string) error {
	return nil
}

// Exists always returns false
func (n *noOpCache) Exists(ctx context.Context, key string) bool {
	return false
}

// Clear does nothing
func (n *noOpCache) Clear(ctx context.Context) error {
	return nil
}

// Stats returns empty statistics
func (n *noOpCache) Stats() CacheStats {
	return CacheStats{}
}

// Close does nothing
func (n *noOpCache) Close() error {
	return nil
}
