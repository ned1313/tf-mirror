package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateDatabase(t *testing.T) {
	tests := []struct {
		name        string
		config      DatabaseConfig
		shouldError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: DatabaseConfig{
				Path: "/data/test.db",
			},
			shouldError: false,
		},
		{
			name: "missing path",
			config: DatabaseConfig{
				Path: "",
			},
			shouldError: true,
			errorMsg:    "database path is required",
		},
		{
			name: "backup enabled with valid interval",
			config: DatabaseConfig{
				Path:                "/data/test.db",
				BackupEnabled:       true,
				BackupIntervalHours: 24,
			},
			shouldError: false,
		},
		{
			name: "backup enabled with invalid interval",
			config: DatabaseConfig{
				Path:                "/data/test.db",
				BackupEnabled:       true,
				BackupIntervalHours: 0,
			},
			shouldError: true,
			errorMsg:    "backup_interval_hours must be at least 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDatabase(&tt.config)
			if tt.shouldError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateCache(t *testing.T) {
	tests := []struct {
		name        string
		config      CacheConfig
		shouldError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: CacheConfig{
				MemorySizeMB: 256,
				DiskSizeGB:   10,
				TTLSeconds:   3600,
				DiskPath:     "/cache",
			},
			shouldError: false,
		},
		{
			name: "negative memory size",
			config: CacheConfig{
				MemorySizeMB: -1,
			},
			shouldError: true,
			errorMsg:    "memory_size_mb cannot be negative",
		},
		{
			name: "negative disk size",
			config: CacheConfig{
				MemorySizeMB: 0,
				DiskSizeGB:   -1,
			},
			shouldError: true,
			errorMsg:    "disk_size_gb cannot be negative",
		},
		{
			name: "negative TTL",
			config: CacheConfig{
				MemorySizeMB: 0,
				DiskSizeGB:   0,
				TTLSeconds:   -1,
			},
			shouldError: true,
			errorMsg:    "ttl_seconds cannot be negative",
		},
		{
			name: "disk size without path",
			config: CacheConfig{
				MemorySizeMB: 0,
				DiskSizeGB:   10,
				TTLSeconds:   0,
				DiskPath:     "",
			},
			shouldError: true,
			errorMsg:    "disk_path is required when disk_size_gb > 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCache(&tt.config)
			if tt.shouldError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateTelemetry(t *testing.T) {
	tests := []struct {
		name        string
		config      TelemetryConfig
		shouldError bool
		errorMsg    string
	}{
		{
			name: "disabled telemetry",
			config: TelemetryConfig{
				Enabled:     true,
				OtelEnabled: false,
			},
			shouldError: false,
		},
		{
			name: "otel enabled with valid config",
			config: TelemetryConfig{
				Enabled:      true,
				OtelEnabled:  true,
				OtelEndpoint: "localhost:4317",
				OtelProtocol: "grpc",
			},
			shouldError: false,
		},
		{
			name: "otel enabled without endpoint",
			config: TelemetryConfig{
				Enabled:     true,
				OtelEnabled: true,
			},
			shouldError: true,
			errorMsg:    "otel_endpoint is required",
		},
		{
			name: "otel enabled with invalid protocol",
			config: TelemetryConfig{
				Enabled:      true,
				OtelEnabled:  true,
				OtelEndpoint: "localhost:4317",
				OtelProtocol: "invalid",
			},
			shouldError: true,
			errorMsg:    "otel_protocol must be one of",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTelemetry(&tt.config)
			if tt.shouldError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateProviders(t *testing.T) {
	tests := []struct {
		name        string
		config      ProvidersConfig
		shouldError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: ProvidersConfig{
				GPGVerificationEnabled:      true,
				GPGKeyURL:                   "https://example.com/key.txt",
				DownloadRetryAttempts:       3,
				DownloadRetryInitialDelayMs: 1000,
				DownloadTimeoutSeconds:      60,
			},
			shouldError: false,
		},
		{
			name: "gpg enabled without key url",
			config: ProvidersConfig{
				GPGVerificationEnabled:      true,
				GPGKeyURL:                   "",
				DownloadRetryAttempts:       3,
				DownloadRetryInitialDelayMs: 1000,
				DownloadTimeoutSeconds:      60,
			},
			shouldError: true,
			errorMsg:    "gpg_key_url is required",
		},
		{
			name: "negative retry attempts",
			config: ProvidersConfig{
				GPGVerificationEnabled:      false,
				DownloadRetryAttempts:       -1,
				DownloadRetryInitialDelayMs: 1000,
				DownloadTimeoutSeconds:      60,
			},
			shouldError: true,
			errorMsg:    "download_retry_attempts cannot be negative",
		},
		{
			name: "negative retry delay",
			config: ProvidersConfig{
				GPGVerificationEnabled:      false,
				DownloadRetryAttempts:       3,
				DownloadRetryInitialDelayMs: -1,
				DownloadTimeoutSeconds:      60,
			},
			shouldError: true,
			errorMsg:    "download_retry_initial_delay_ms cannot be negative",
		},
		{
			name: "invalid timeout",
			config: ProvidersConfig{
				GPGVerificationEnabled:      false,
				DownloadRetryAttempts:       3,
				DownloadRetryInitialDelayMs: 1000,
				DownloadTimeoutSeconds:      0,
			},
			shouldError: true,
			errorMsg:    "download_timeout_seconds must be at least 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProviders(&tt.config)
			if tt.shouldError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateQuota(t *testing.T) {
	tests := []struct {
		name        string
		config      QuotaConfig
		shouldError bool
		errorMsg    string
	}{
		{
			name: "disabled quota",
			config: QuotaConfig{
				Enabled: false,
			},
			shouldError: false,
		},
		{
			name: "valid enabled quota",
			config: QuotaConfig{
				Enabled:                 true,
				MaxStorageGB:            100,
				WarningThresholdPercent: 80,
			},
			shouldError: false,
		},
		{
			name: "enabled with invalid storage",
			config: QuotaConfig{
				Enabled:                 true,
				MaxStorageGB:            0,
				WarningThresholdPercent: 80,
			},
			shouldError: true,
			errorMsg:    "max_storage_gb must be at least 1",
		},
		{
			name: "enabled with invalid threshold low",
			config: QuotaConfig{
				Enabled:                 true,
				MaxStorageGB:            100,
				WarningThresholdPercent: 0,
			},
			shouldError: true,
			errorMsg:    "warning_threshold_percent must be between 1 and 100",
		},
		{
			name: "enabled with invalid threshold high",
			config: QuotaConfig{
				Enabled:                 true,
				MaxStorageGB:            100,
				WarningThresholdPercent: 101,
			},
			shouldError: true,
			errorMsg:    "warning_threshold_percent must be between 1 and 100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateQuota(&tt.config)
			if tt.shouldError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateServerTLS(t *testing.T) {
	// Create temp cert/key files
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")

	os.WriteFile(certPath, []byte("fake cert"), 0644)
	os.WriteFile(keyPath, []byte("fake key"), 0644)

	tests := []struct {
		name        string
		config      ServerConfig
		shouldError bool
		errorMsg    string
	}{
		{
			name: "TLS enabled with valid paths",
			config: ServerConfig{
				Port:        8443,
				TLSEnabled:  true,
				TLSCertPath: certPath,
				TLSKeyPath:  keyPath,
			},
			shouldError: false,
		},
		{
			name: "TLS enabled with missing cert",
			config: ServerConfig{
				Port:        8443,
				TLSEnabled:  true,
				TLSCertPath: "",
				TLSKeyPath:  keyPath,
			},
			shouldError: true,
			errorMsg:    "tls_cert_path is required",
		},
		{
			name: "TLS enabled with missing key",
			config: ServerConfig{
				Port:        8443,
				TLSEnabled:  true,
				TLSCertPath: certPath,
				TLSKeyPath:  "",
			},
			shouldError: true,
			errorMsg:    "tls_key_path is required",
		},
		{
			name: "TLS enabled with non-existent cert",
			config: ServerConfig{
				Port:        8443,
				TLSEnabled:  true,
				TLSCertPath: "/nonexistent/cert.pem",
				TLSKeyPath:  keyPath,
			},
			shouldError: true,
			errorMsg:    "tls_cert_path file not found",
		},
		{
			name: "TLS enabled with non-existent key",
			config: ServerConfig{
				Port:        8443,
				TLSEnabled:  true,
				TLSCertPath: certPath,
				TLSKeyPath:  "/nonexistent/key.pem",
			},
			shouldError: true,
			errorMsg:    "tls_key_path file not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateServer(&tt.config)
			if tt.shouldError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestContainsHelper(t *testing.T) {
	slice := []string{"a", "b", "c"}

	assert.True(t, contains(slice, "a"))
	assert.True(t, contains(slice, "A")) // case insensitive
	assert.True(t, contains(slice, "B"))
	assert.False(t, contains(slice, "d"))
	assert.False(t, contains(slice, ""))
}

func TestFullValidation_AllPaths(t *testing.T) {
	// Test that all validation paths are covered
	cfg := DefaultConfig()

	// Database error
	cfg.Database.Path = ""
	err := Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database config")

	// Cache error
	cfg = DefaultConfig()
	cfg.Cache.MemorySizeMB = -1
	err = Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cache config")

	// Auth error
	cfg = DefaultConfig()
	cfg.Auth.JWTExpirationHours = 0
	err = Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "auth config")

	// Logging error
	cfg = DefaultConfig()
	cfg.Logging.Level = "invalid"
	err = Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "logging config")

	// Telemetry error
	cfg = DefaultConfig()
	cfg.Telemetry.OtelEnabled = true
	cfg.Telemetry.OtelEndpoint = ""
	err = Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "telemetry config")

	// Providers error
	cfg = DefaultConfig()
	cfg.Providers.DownloadTimeoutSeconds = 0
	err = Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "providers config")

	// Quota error
	cfg = DefaultConfig()
	cfg.Quota.Enabled = true
	cfg.Quota.MaxStorageGB = 0
	err = Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "quota config")
}

func TestValidateLogging_OutputBoth(t *testing.T) {
	// Test 'both' output type requires file_path
	config := LoggingConfig{
		Level:  "info",
		Format: "text",
		Output: "both",
	}

	err := validateLogging(&config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file_path is required")

	// With file_path should be valid
	config.FilePath = "/var/log/app.log"
	err = validateLogging(&config)
	assert.NoError(t, err)
}

func TestValidateLogging_InvalidOutput(t *testing.T) {
	config := LoggingConfig{
		Level:  "info",
		Format: "text",
		Output: "invalid",
	}

	err := validateLogging(&config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "logging output must be one of")
}
