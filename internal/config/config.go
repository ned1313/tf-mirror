package config

import (
	"time"
)

// Config represents the complete application configuration
type Config struct {
	Server       ServerConfig        `hcl:"server,block"`
	Storage      StorageConfig       `hcl:"storage,block"`
	Database     DatabaseConfig      `hcl:"database,block"`
	Cache        CacheConfig         `hcl:"cache,block"`
	Features     FeaturesConfig      `hcl:"features,block"`
	Auth         AuthConfig          `hcl:"auth,block"`
	Processor    ProcessorConfig     `hcl:"processor,block"`
	Logging      LoggingConfig       `hcl:"logging,block"`
	Telemetry    TelemetryConfig     `hcl:"telemetry,block"`
	Providers    ProvidersConfig     `hcl:"providers,block"`
	Quota        QuotaConfig         `hcl:"quota,block"`
	AutoDownload *AutoDownloadConfig `hcl:"auto_download,block"`
}

// ServerConfig contains HTTP server settings
type ServerConfig struct {
	Port           int      `hcl:"port,optional"`
	TLSEnabled     bool     `hcl:"tls_enabled,optional"`
	TLSCertPath    string   `hcl:"tls_cert_path,optional"`
	TLSKeyPath     string   `hcl:"tls_key_path,optional"`
	BehindProxy    bool     `hcl:"behind_proxy,optional"`
	TrustedProxies []string `hcl:"trusted_proxies,optional"`
}

// StorageConfig contains object storage settings
type StorageConfig struct {
	Type           string `hcl:"type,optional"`
	Bucket         string `hcl:"bucket,optional"`
	Region         string `hcl:"region,optional"`
	Endpoint       string `hcl:"endpoint,optional"`
	AccessKey      string `hcl:"access_key,optional"`
	SecretKey      string `hcl:"secret_key,optional"`
	ForcePathStyle bool   `hcl:"force_path_style,optional"`
}

// DatabaseConfig contains database settings
type DatabaseConfig struct {
	Path                string `hcl:"path,optional"`
	BackupEnabled       bool   `hcl:"backup_enabled,optional"`
	BackupIntervalHours int    `hcl:"backup_interval_hours,optional"`
	BackupToS3          bool   `hcl:"backup_to_s3,optional"`
	BackupS3Prefix      string `hcl:"backup_s3_prefix,optional"`
}

// CacheConfig contains caching settings
type CacheConfig struct {
	MemorySizeMB int    `hcl:"memory_size_mb,optional"`
	DiskPath     string `hcl:"disk_path,optional"`
	DiskSizeGB   int    `hcl:"disk_size_gb,optional"`
	TTLSeconds   int    `hcl:"ttl_seconds,optional"`
}

// FeaturesConfig contains feature flags
type FeaturesConfig struct {
	AutoDownloadProviders bool `hcl:"auto_download_providers,optional"`
	AutoDownloadModules   bool `hcl:"auto_download_modules,optional"`
	MaxDownloadSizeMB     int  `hcl:"max_download_size_mb,optional"`
}

// AutoDownloadConfig contains auto-download specific settings
type AutoDownloadConfig struct {
	Enabled              bool     `hcl:"enabled,optional"`
	AllowedNamespaces    []string `hcl:"allowed_namespaces,optional"`    // Empty = all allowed
	BlockedNamespaces    []string `hcl:"blocked_namespaces,optional"`    // Takes precedence over allowed
	Platforms            []string `hcl:"platforms,optional"`             // Platforms to download (e.g., linux_amd64)
	RateLimitPerMinute   int      `hcl:"rate_limit_per_minute,optional"` // Max downloads per minute
	MaxConcurrentDL      int      `hcl:"max_concurrent_downloads,optional"`
	QueueSize            int      `hcl:"queue_size,optional"`      // Max pending downloads
	TimeoutSeconds       int      `hcl:"timeout_seconds,optional"` // Per-download timeout
	RetryOnFailure       bool     `hcl:"retry_on_failure,optional"`
	CacheNegativeResults bool     `hcl:"cache_negative_results,optional"` // Cache "not found" responses
	NegativeCacheTTL     int      `hcl:"negative_cache_ttl_seconds,optional"`
}

// AuthConfig contains authentication settings
type AuthConfig struct {
	JWTSecret          string `hcl:"jwt_secret,optional"`
	JWTExpirationHours int    `hcl:"jwt_expiration_hours,optional"`
	BCryptCost         int    `hcl:"bcrypt_cost,optional"`
}

// ProcessorConfig contains background job processor settings
type ProcessorConfig struct {
	PollingIntervalSeconds int `hcl:"polling_interval_seconds,optional"`
	MaxConcurrentJobs      int `hcl:"max_concurrent_jobs,optional"`
	RetryAttempts          int `hcl:"retry_attempts,optional"`
	RetryDelaySeconds      int `hcl:"retry_delay_seconds,optional"`
	WorkerShutdownSeconds  int `hcl:"worker_shutdown_seconds,optional"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level    string `hcl:"level,optional"`
	Format   string `hcl:"format,optional"`
	Output   string `hcl:"output,optional"`
	FilePath string `hcl:"file_path,optional"`
}

// TelemetryConfig contains observability settings
type TelemetryConfig struct {
	Enabled       bool   `hcl:"enabled,optional"`
	OtelEnabled   bool   `hcl:"otel_enabled,optional"`
	OtelEndpoint  string `hcl:"otel_endpoint,optional"`
	OtelProtocol  string `hcl:"otel_protocol,optional"`
	ExportTraces  bool   `hcl:"export_traces,optional"`
	ExportMetrics bool   `hcl:"export_metrics,optional"`
}

// ProvidersConfig contains provider-specific settings
type ProvidersConfig struct {
	GPGVerificationEnabled      bool   `hcl:"gpg_verification_enabled,optional"`
	GPGKeyURL                   string `hcl:"gpg_key_url,optional"`
	DownloadRetryAttempts       int    `hcl:"download_retry_attempts,optional"`
	DownloadRetryInitialDelayMs int    `hcl:"download_retry_initial_delay_ms,optional"`
	DownloadTimeoutSeconds      int    `hcl:"download_timeout_seconds,optional"`
}

// QuotaConfig contains storage quota settings
type QuotaConfig struct {
	Enabled                 bool `hcl:"enabled,optional"`
	MaxStorageGB            int  `hcl:"max_storage_gb,optional"`
	WarningThresholdPercent int  `hcl:"warning_threshold_percent,optional"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:           8080,
			TLSEnabled:     false,
			TLSCertPath:    "",
			TLSKeyPath:     "",
			BehindProxy:    false,
			TrustedProxies: []string{},
		},
		Storage: StorageConfig{
			Type:           "s3",
			Bucket:         "terraform-mirror",
			Region:         "us-east-1",
			Endpoint:       "",
			AccessKey:      "",
			SecretKey:      "",
			ForcePathStyle: false,
		},
		Database: DatabaseConfig{
			Path:                "/data/terraform-mirror.db",
			BackupEnabled:       false,
			BackupIntervalHours: 24,
			BackupToS3:          false,
			BackupS3Prefix:      "backups/",
		},
		Cache: CacheConfig{
			MemorySizeMB: 256,
			DiskPath:     "/var/cache/tf-mirror",
			DiskSizeGB:   10,
			TTLSeconds:   3600,
		},
		Features: FeaturesConfig{
			AutoDownloadProviders: false,
			AutoDownloadModules:   false,
			MaxDownloadSizeMB:     500,
		},
		Auth: AuthConfig{
			JWTExpirationHours: 8,
			BCryptCost:         12,
			JWTSecret:          "", // Must be set via environment variable or config file
		},
		Processor: ProcessorConfig{
			PollingIntervalSeconds: 10,
			MaxConcurrentJobs:      3,
			RetryAttempts:          3,
			RetryDelaySeconds:      5,
			WorkerShutdownSeconds:  30,
		},
		Logging: LoggingConfig{
			Level:    "info",
			Format:   "text",
			Output:   "stdout",
			FilePath: "",
		},
		Telemetry: TelemetryConfig{
			Enabled:       false,
			OtelEnabled:   false,
			OtelEndpoint:  "",
			OtelProtocol:  "grpc",
			ExportTraces:  false,
			ExportMetrics: false,
		},
		Providers: ProvidersConfig{
			GPGVerificationEnabled:      true,
			GPGKeyURL:                   "https://www.hashicorp.com/.well-known/pgp-key.txt",
			DownloadRetryAttempts:       5,
			DownloadRetryInitialDelayMs: 1000,
			DownloadTimeoutSeconds:      60,
		},
		Quota: QuotaConfig{
			Enabled:                 false,
			MaxStorageGB:            0,
			WarningThresholdPercent: 80,
		},
		AutoDownload: &AutoDownloadConfig{
			Enabled:              false, // Disabled by default for security
			AllowedNamespaces:    []string{},
			BlockedNamespaces:    []string{},
			Platforms:            []string{"linux_amd64", "windows_amd64"}, // Default platforms
			RateLimitPerMinute:   10,
			MaxConcurrentDL:      3,
			QueueSize:            100,
			TimeoutSeconds:       300, // 5 minutes per download
			RetryOnFailure:       true,
			CacheNegativeResults: true,
			NegativeCacheTTL:     300, // 5 minutes
		},
	}
}

// GetJWTExpiration returns the JWT expiration as a duration
func (c *AuthConfig) GetJWTExpiration() time.Duration {
	return time.Duration(c.JWTExpirationHours) * time.Hour
}

// GetDownloadRetryDelay returns the initial retry delay as a duration
func (c *ProvidersConfig) GetDownloadRetryDelay() time.Duration {
	return time.Duration(c.DownloadRetryInitialDelayMs) * time.Millisecond
}

// GetDownloadTimeout returns the download timeout as a duration
func (c *ProvidersConfig) GetDownloadTimeout() time.Duration {
	return time.Duration(c.DownloadTimeoutSeconds) * time.Second
}

// GetBackupInterval returns the backup interval as a duration
func (c *DatabaseConfig) GetBackupInterval() time.Duration {
	return time.Duration(c.BackupIntervalHours) * time.Hour
}

// GetCacheTTL returns the cache TTL as a duration
func (c *CacheConfig) GetCacheTTL() time.Duration {
	return time.Duration(c.TTLSeconds) * time.Second
}

// GetTimeout returns the auto-download timeout as a duration
func (c *AutoDownloadConfig) GetTimeout() time.Duration {
	return time.Duration(c.TimeoutSeconds) * time.Second
}

// GetNegativeCacheTTL returns the negative cache TTL as a duration
func (c *AutoDownloadConfig) GetNegativeCacheTTL() time.Duration {
	return time.Duration(c.NegativeCacheTTL) * time.Second
}

// GetPlatforms returns the configured platforms, with defaults if empty
func (c *AutoDownloadConfig) GetPlatforms() []string {
	if len(c.Platforms) == 0 {
		return []string{"linux_amd64", "windows_amd64"}
	}
	return c.Platforms
}

// IsNamespaceAllowed checks if a namespace is allowed for auto-download
func (c *AutoDownloadConfig) IsNamespaceAllowed(namespace string) bool {
	// Check blocked list first (takes precedence)
	for _, blocked := range c.BlockedNamespaces {
		if blocked == namespace {
			return false
		}
	}

	// If allowed list is empty, all non-blocked namespaces are allowed
	if len(c.AllowedNamespaces) == 0 {
		return true
	}

	// Check allowed list
	for _, allowed := range c.AllowedNamespaces {
		if allowed == namespace {
			return true
		}
	}

	return false
}
