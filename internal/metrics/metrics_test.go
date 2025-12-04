package metrics

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// newTestMetrics creates metrics with a fresh registry for testing
func newTestMetrics() *Metrics {
	reg := prometheus.NewRegistry()
	return NewWithRegistry(reg)
}

func TestNewWithRegistry(t *testing.T) {
	m := newTestMetrics()

	if m == nil {
		t.Fatal("NewWithRegistry() returned nil")
	}

	// Verify all metrics are initialized
	if m.HTTPRequestsTotal == nil {
		t.Error("HTTPRequestsTotal is nil")
	}
	if m.HTTPRequestDuration == nil {
		t.Error("HTTPRequestDuration is nil")
	}
	if m.HTTPRequestsInFlight == nil {
		t.Error("HTTPRequestsInFlight is nil")
	}
	if m.ProvidersTotal == nil {
		t.Error("ProvidersTotal is nil")
	}
	if m.ProviderVersions == nil {
		t.Error("ProviderVersions is nil")
	}
	if m.ProviderDownloads == nil {
		t.Error("ProviderDownloads is nil")
	}
	if m.JobsTotal == nil {
		t.Error("JobsTotal is nil")
	}
	if m.JobsProcessed == nil {
		t.Error("JobsProcessed is nil")
	}
	if m.ActiveJobs == nil {
		t.Error("ActiveJobs is nil")
	}
	if m.ProcessorStatus == nil {
		t.Error("ProcessorStatus is nil")
	}
	if m.StorageBytesUsed == nil {
		t.Error("StorageBytesUsed is nil")
	}
	if m.CacheHits == nil {
		t.Error("CacheHits is nil")
	}
	if m.CacheMisses == nil {
		t.Error("CacheMisses is nil")
	}
	if m.AuthAttempts == nil {
		t.Error("AuthAttempts is nil")
	}
}

func TestRecordHTTPRequest(t *testing.T) {
	m := newTestMetrics()

	m.RecordHTTPRequest("GET", "/health", "200", 0.1)
	m.RecordHTTPRequest("GET", "/health", "200", 0.2)
	m.RecordHTTPRequest("POST", "/admin/api/login", "200", 0.05)

	// Verify the counter was incremented
	count := testutil.ToFloat64(m.HTTPRequestsTotal.WithLabelValues("GET", "/health", "200"))
	if count != 2 {
		t.Errorf("Expected 2 requests, got %v", count)
	}
}

func TestRecordProviderDownload(t *testing.T) {
	m := newTestMetrics()

	m.RecordProviderDownload("hashicorp", "aws", "5.0.0", "linux", "amd64", 50*1024*1024)
	m.RecordProviderDownload("hashicorp", "aws", "5.0.0", "darwin", "arm64", 48*1024*1024)

	// Verify download counter
	count := testutil.ToFloat64(m.ProviderDownloads.WithLabelValues("hashicorp", "aws", "5.0.0", "linux", "amd64"))
	if count != 1 {
		t.Errorf("Expected 1 download, got %v", count)
	}

	// Verify bytes counter
	bytes := testutil.ToFloat64(m.ProviderDownloadSize.WithLabelValues("hashicorp", "aws"))
	expectedBytes := float64((50 + 48) * 1024 * 1024)
	if bytes != expectedBytes {
		t.Errorf("Expected %v bytes, got %v", expectedBytes, bytes)
	}
}

func TestRecordJobProcessed(t *testing.T) {
	m := newTestMetrics()

	m.RecordJobProcessed("completed", 30.5, "provider_load")
	m.RecordJobProcessed("failed", 10.2, "provider_load")
	m.RecordJobProcessed("completed", 25.0, "provider_load")

	completedCount := testutil.ToFloat64(m.JobsProcessed.WithLabelValues("completed"))
	if completedCount != 2 {
		t.Errorf("Expected 2 completed jobs, got %v", completedCount)
	}

	failedCount := testutil.ToFloat64(m.JobsProcessed.WithLabelValues("failed"))
	if failedCount != 1 {
		t.Errorf("Expected 1 failed job, got %v", failedCount)
	}
}

func TestRecordStorageOperation(t *testing.T) {
	m := newTestMetrics()

	m.RecordStorageOperation("upload", "success")
	m.RecordStorageOperation("upload", "success")
	m.RecordStorageOperation("upload", "error")
	m.RecordStorageOperation("download", "success")

	uploadSuccess := testutil.ToFloat64(m.StorageOperations.WithLabelValues("upload", "success"))
	if uploadSuccess != 2 {
		t.Errorf("Expected 2 successful uploads, got %v", uploadSuccess)
	}
}

func TestRecordAuthAttempt(t *testing.T) {
	m := newTestMetrics()

	m.RecordAuthAttempt("success")
	m.RecordAuthAttempt("success")
	m.RecordAuthAttempt("invalid_password")
	m.RecordAuthAttempt("user_not_found")

	successCount := testutil.ToFloat64(m.AuthAttempts.WithLabelValues("success"))
	if successCount != 2 {
		t.Errorf("Expected 2 successful auth attempts, got %v", successCount)
	}
}

func TestUpdateProviderCounts(t *testing.T) {
	m := newTestMetrics()

	m.UpdateProviderCounts(10, 50)

	providersTotal := testutil.ToFloat64(m.ProvidersTotal)
	if providersTotal != 10 {
		t.Errorf("Expected 10 providers, got %v", providersTotal)
	}

	versionsTotal := testutil.ToFloat64(m.ProviderVersions)
	if versionsTotal != 50 {
		t.Errorf("Expected 50 versions, got %v", versionsTotal)
	}
}

func TestUpdateJobCounts(t *testing.T) {
	m := newTestMetrics()

	m.UpdateJobCounts(5, 2, 100, 3)

	pending := testutil.ToFloat64(m.JobsTotal.WithLabelValues("pending"))
	if pending != 5 {
		t.Errorf("Expected 5 pending jobs, got %v", pending)
	}

	running := testutil.ToFloat64(m.JobsTotal.WithLabelValues("running"))
	if running != 2 {
		t.Errorf("Expected 2 running jobs, got %v", running)
	}

	activeJobs := testutil.ToFloat64(m.ActiveJobs)
	if activeJobs != 2 {
		t.Errorf("Expected 2 active jobs, got %v", activeJobs)
	}
}

func TestUpdateStorageStats(t *testing.T) {
	m := newTestMetrics()

	m.UpdateStorageStats(1024*1024*1024, 500) // 1GB, 500 objects

	bytesUsed := testutil.ToFloat64(m.StorageBytesUsed)
	if bytesUsed != 1024*1024*1024 {
		t.Errorf("Expected 1GB, got %v", bytesUsed)
	}

	objectCount := testutil.ToFloat64(m.StorageObjectCount)
	if objectCount != 500 {
		t.Errorf("Expected 500 objects, got %v", objectCount)
	}
}

func TestCacheMetrics(t *testing.T) {
	m := newTestMetrics()

	m.RecordCacheHit()
	m.RecordCacheHit()
	m.RecordCacheMiss()
	m.RecordCacheEviction()
	m.UpdateCacheStats(256 * 1024 * 1024) // 256MB

	hits := testutil.ToFloat64(m.CacheHits)
	if hits != 2 {
		t.Errorf("Expected 2 cache hits, got %v", hits)
	}

	misses := testutil.ToFloat64(m.CacheMisses)
	if misses != 1 {
		t.Errorf("Expected 1 cache miss, got %v", misses)
	}

	evictions := testutil.ToFloat64(m.CacheEvictions)
	if evictions != 1 {
		t.Errorf("Expected 1 cache eviction, got %v", evictions)
	}

	size := testutil.ToFloat64(m.CacheSize)
	if size != 256*1024*1024 {
		t.Errorf("Expected 256MB cache size, got %v", size)
	}
}

func TestSetProcessorStatus(t *testing.T) {
	m := newTestMetrics()

	m.SetProcessorStatus(true)
	status := testutil.ToFloat64(m.ProcessorStatus)
	if status != 1 {
		t.Errorf("Expected processor status 1 (running), got %v", status)
	}

	m.SetProcessorStatus(false)
	status = testutil.ToFloat64(m.ProcessorStatus)
	if status != 0 {
		t.Errorf("Expected processor status 0 (stopped), got %v", status)
	}
}

func TestSetActiveSessions(t *testing.T) {
	m := newTestMetrics()

	m.SetActiveSessions(5)
	sessions := testutil.ToFloat64(m.ActiveSessions)
	if sessions != 5 {
		t.Errorf("Expected 5 active sessions, got %v", sessions)
	}
}

func TestMetricsNamespace(t *testing.T) {
	m := newTestMetrics()

	// Get metric description and verify namespace
	desc := make(chan *prometheus.Desc, 1)
	m.ProvidersTotal.Describe(desc)
	d := <-desc

	// The string representation should contain our namespace
	descStr := d.String()
	if !strings.Contains(descStr, "terraform_mirror") {
		t.Errorf("Expected metric namespace 'terraform_mirror', got: %s", descStr)
	}
}

func TestUpdateCacheStats(t *testing.T) {
	m := newTestMetrics()

	m.UpdateCacheStats(256 * 1024 * 1024) // 256MB

	cacheSize := testutil.ToFloat64(m.CacheSize)
	if cacheSize != 256*1024*1024 {
		t.Errorf("Expected 256MB cache size, got %v", cacheSize)
	}

	// Update to new value
	m.UpdateCacheStats(512 * 1024 * 1024) // 512MB
	cacheSize = testutil.ToFloat64(m.CacheSize)
	if cacheSize != 512*1024*1024 {
		t.Errorf("Expected 512MB cache size, got %v", cacheSize)
	}
}

func TestRecordCacheHit(t *testing.T) {
	m := newTestMetrics()

	m.RecordCacheHit()
	m.RecordCacheHit()
	m.RecordCacheHit()

	hits := testutil.ToFloat64(m.CacheHits)
	if hits != 3 {
		t.Errorf("Expected 3 cache hits, got %v", hits)
	}
}

func TestRecordCacheMiss(t *testing.T) {
	m := newTestMetrics()

	m.RecordCacheMiss()
	m.RecordCacheMiss()

	misses := testutil.ToFloat64(m.CacheMisses)
	if misses != 2 {
		t.Errorf("Expected 2 cache misses, got %v", misses)
	}
}

func TestRecordCacheEviction(t *testing.T) {
	m := newTestMetrics()

	m.RecordCacheEviction()
	m.RecordCacheEviction()
	m.RecordCacheEviction()
	m.RecordCacheEviction()

	evictions := testutil.ToFloat64(m.CacheEvictions)
	if evictions != 4 {
		t.Errorf("Expected 4 cache evictions, got %v", evictions)
	}
}

func TestSetActiveSessions_Multiple(t *testing.T) {
	m := newTestMetrics()

	// Set initial value
	m.SetActiveSessions(10)
	sessions := testutil.ToFloat64(m.ActiveSessions)
	if sessions != 10 {
		t.Errorf("Expected 10 active sessions, got %v", sessions)
	}

	// Update to new value
	m.SetActiveSessions(5)
	sessions = testutil.ToFloat64(m.ActiveSessions)
	if sessions != 5 {
		t.Errorf("Expected 5 active sessions, got %v", sessions)
	}

	// Set to zero
	m.SetActiveSessions(0)
	sessions = testutil.ToFloat64(m.ActiveSessions)
	if sessions != 0 {
		t.Errorf("Expected 0 active sessions, got %v", sessions)
	}
}
