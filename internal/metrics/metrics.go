// Package metrics provides Prometheus metrics for the Terraform Mirror service.
package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace = "terraform_mirror"
)

var (
	// globalMetrics holds the singleton metrics instance
	globalMetrics *Metrics
	once          sync.Once
)

// Metrics holds all Prometheus metrics for the application
type Metrics struct {
	registry *prometheus.Registry

	// HTTP metrics
	HTTPRequestsTotal    *prometheus.CounterVec
	HTTPRequestDuration  *prometheus.HistogramVec
	HTTPRequestsInFlight prometheus.Gauge

	// Provider metrics
	ProvidersTotal       prometheus.Gauge
	ProviderVersions     prometheus.Gauge
	ProviderDownloads    *prometheus.CounterVec
	ProviderDownloadSize *prometheus.CounterVec

	// Job metrics
	JobsTotal       *prometheus.GaugeVec
	JobsProcessed   *prometheus.CounterVec
	JobDuration     *prometheus.HistogramVec
	JobItemsTotal   *prometheus.CounterVec
	ActiveJobs      prometheus.Gauge
	ProcessorStatus prometheus.Gauge

	// Storage metrics
	StorageBytesUsed   prometheus.Gauge
	StorageObjectCount prometheus.Gauge
	StorageOperations  *prometheus.CounterVec

	// Cache metrics
	CacheHits      prometheus.Counter
	CacheMisses    prometheus.Counter
	CacheSize      prometheus.Gauge
	CacheEvictions prometheus.Counter

	// Database metrics
	DBConnections   prometheus.Gauge
	DBQueryDuration *prometheus.HistogramVec
	DBQueryTotal    *prometheus.CounterVec

	// Authentication metrics
	AuthAttempts   *prometheus.CounterVec
	ActiveSessions prometheus.Gauge
}

// New creates and registers all Prometheus metrics (singleton)
// Returns the same instance on subsequent calls
func New() *Metrics {
	once.Do(func() {
		globalMetrics = newMetrics(prometheus.DefaultRegisterer)
	})
	return globalMetrics
}

// NewWithRegistry creates metrics with a custom registry (for testing)
func NewWithRegistry(reg prometheus.Registerer) *Metrics {
	return newMetrics(reg)
}

// newMetrics creates and registers all Prometheus metrics
func newMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{}

	// HTTP metrics
	m.HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	m.HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	m.HTTPRequestsInFlight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "http_requests_in_flight",
			Help:      "Number of HTTP requests currently being processed",
		},
	)

	// Provider metrics
	m.ProvidersTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "providers_total",
			Help:      "Total number of providers in the mirror",
		},
	)

	m.ProviderVersions = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "provider_versions_total",
			Help:      "Total number of provider versions in the mirror",
		},
	)

	m.ProviderDownloads = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "provider_downloads_total",
			Help:      "Total number of provider downloads",
		},
		[]string{"namespace", "type", "version", "os", "arch"},
	)

	m.ProviderDownloadSize = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "provider_download_bytes_total",
			Help:      "Total bytes downloaded for providers",
		},
		[]string{"namespace", "type"},
	)

	// Job metrics
	m.JobsTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "jobs_total",
			Help:      "Total number of jobs by status",
		},
		[]string{"status"},
	)

	m.JobsProcessed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "jobs_processed_total",
			Help:      "Total number of jobs processed",
		},
		[]string{"status"},
	)

	m.JobDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "job_duration_seconds",
			Help:      "Job processing duration in seconds",
			Buckets:   []float64{1, 5, 10, 30, 60, 120, 300, 600},
		},
		[]string{"type"},
	)

	m.JobItemsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "job_items_total",
			Help:      "Total number of job items processed",
		},
		[]string{"status"},
	)

	m.ActiveJobs = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "active_jobs",
			Help:      "Number of currently active jobs",
		},
	)

	m.ProcessorStatus = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "processor_running",
			Help:      "Whether the job processor is running (1) or stopped (0)",
		},
	)

	// Storage metrics
	m.StorageBytesUsed = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "storage_bytes_used",
			Help:      "Total bytes used in storage",
		},
	)

	m.StorageObjectCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "storage_objects_total",
			Help:      "Total number of objects in storage",
		},
	)

	m.StorageOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "storage_operations_total",
			Help:      "Total number of storage operations",
		},
		[]string{"operation", "status"},
	)

	// Cache metrics
	m.CacheHits = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "cache_hits_total",
			Help:      "Total number of cache hits",
		},
	)

	m.CacheMisses = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "cache_misses_total",
			Help:      "Total number of cache misses",
		},
	)

	m.CacheSize = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "cache_size_bytes",
			Help:      "Current size of cache in bytes",
		},
	)

	m.CacheEvictions = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "cache_evictions_total",
			Help:      "Total number of cache evictions",
		},
	)

	// Database metrics
	m.DBConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "db_connections",
			Help:      "Number of active database connections",
		},
	)

	m.DBQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "db_query_duration_seconds",
			Help:      "Database query duration in seconds",
			Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
		[]string{"query"},
	)

	m.DBQueryTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "db_queries_total",
			Help:      "Total number of database queries",
		},
		[]string{"query", "status"},
	)

	// Authentication metrics
	m.AuthAttempts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "auth_attempts_total",
			Help:      "Total number of authentication attempts",
		},
		[]string{"result"},
	)

	m.ActiveSessions = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "active_sessions",
			Help:      "Number of active user sessions",
		},
	)

	// Register all metrics
	reg.MustRegister(
		m.HTTPRequestsTotal,
		m.HTTPRequestDuration,
		m.HTTPRequestsInFlight,
		m.ProvidersTotal,
		m.ProviderVersions,
		m.ProviderDownloads,
		m.ProviderDownloadSize,
		m.JobsTotal,
		m.JobsProcessed,
		m.JobDuration,
		m.JobItemsTotal,
		m.ActiveJobs,
		m.ProcessorStatus,
		m.StorageBytesUsed,
		m.StorageObjectCount,
		m.StorageOperations,
		m.CacheHits,
		m.CacheMisses,
		m.CacheSize,
		m.CacheEvictions,
		m.DBConnections,
		m.DBQueryDuration,
		m.DBQueryTotal,
		m.AuthAttempts,
		m.ActiveSessions,
	)

	return m
}

// RecordHTTPRequest records metrics for an HTTP request
func (m *Metrics) RecordHTTPRequest(method, path, status string, duration float64) {
	m.HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
	m.HTTPRequestDuration.WithLabelValues(method, path).Observe(duration)
}

// RecordProviderDownload records a provider download
func (m *Metrics) RecordProviderDownload(namespace, providerType, version, os, arch string, sizeBytes int64) {
	m.ProviderDownloads.WithLabelValues(namespace, providerType, version, os, arch).Inc()
	m.ProviderDownloadSize.WithLabelValues(namespace, providerType).Add(float64(sizeBytes))
}

// RecordJobProcessed records a completed job
func (m *Metrics) RecordJobProcessed(status string, durationSeconds float64, jobType string) {
	m.JobsProcessed.WithLabelValues(status).Inc()
	m.JobDuration.WithLabelValues(jobType).Observe(durationSeconds)
}

// RecordStorageOperation records a storage operation
func (m *Metrics) RecordStorageOperation(operation, status string) {
	m.StorageOperations.WithLabelValues(operation, status).Inc()
}

// RecordAuthAttempt records an authentication attempt
func (m *Metrics) RecordAuthAttempt(result string) {
	m.AuthAttempts.WithLabelValues(result).Inc()
}

// UpdateProviderCounts updates the provider gauge metrics
func (m *Metrics) UpdateProviderCounts(total, versions int) {
	m.ProvidersTotal.Set(float64(total))
	m.ProviderVersions.Set(float64(versions))
}

// UpdateJobCounts updates the job gauge metrics by status
func (m *Metrics) UpdateJobCounts(pending, running, completed, failed int) {
	m.JobsTotal.WithLabelValues("pending").Set(float64(pending))
	m.JobsTotal.WithLabelValues("running").Set(float64(running))
	m.JobsTotal.WithLabelValues("completed").Set(float64(completed))
	m.JobsTotal.WithLabelValues("failed").Set(float64(failed))
	m.ActiveJobs.Set(float64(running))
}

// UpdateStorageStats updates storage gauge metrics
func (m *Metrics) UpdateStorageStats(bytesUsed int64, objectCount int) {
	m.StorageBytesUsed.Set(float64(bytesUsed))
	m.StorageObjectCount.Set(float64(objectCount))
}

// UpdateCacheStats updates cache gauge metrics
func (m *Metrics) UpdateCacheStats(sizeBytes int64) {
	m.CacheSize.Set(float64(sizeBytes))
}

// RecordCacheHit records a cache hit
func (m *Metrics) RecordCacheHit() {
	m.CacheHits.Inc()
}

// RecordCacheMiss records a cache miss
func (m *Metrics) RecordCacheMiss() {
	m.CacheMisses.Inc()
}

// RecordCacheEviction records a cache eviction
func (m *Metrics) RecordCacheEviction() {
	m.CacheEvictions.Inc()
}

// SetProcessorStatus sets whether the processor is running
func (m *Metrics) SetProcessorStatus(running bool) {
	if running {
		m.ProcessorStatus.Set(1)
	} else {
		m.ProcessorStatus.Set(0)
	}
}

// SetActiveSessions sets the number of active sessions
func (m *Metrics) SetActiveSessions(count int) {
	m.ActiveSessions.Set(float64(count))
}
