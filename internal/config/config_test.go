package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "s3", cfg.Storage.Type)
	assert.Equal(t, "terraform-mirror", cfg.Storage.Bucket)
	assert.Equal(t, "/data/terraform-mirror.db", cfg.Database.Path)
	assert.Equal(t, 256, cfg.Cache.MemorySizeMB)
	assert.Equal(t, 8, cfg.Auth.JWTExpirationHours)
	assert.Equal(t, 12, cfg.Auth.BCryptCost)
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.True(t, cfg.Providers.GPGVerificationEnabled)
	assert.False(t, cfg.Features.AutoDownloadProviders)
}

func TestLoadFromFile(t *testing.T) {
	// Create a temporary HCL config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.hcl")

	configContent := `
server {
  port = 9090
  tls_enabled = false
}

storage {
  type = "s3"
  bucket = "test-bucket"
  region = "us-west-2"
}

database {
  path = "/tmp/test.db"
}

cache {
  memory_size_mb = 512
  disk_size_gb = 20
  ttl_seconds = 7200
}

features {
  auto_download_providers = false
  auto_download_modules = false
}

auth {
  jwt_expiration_hours = 24
  bcrypt_cost = 10
}

logging {
  level = "debug"
  format = "json"
}

telemetry {
  enabled = false
}

processor {
  polling_interval_seconds = 10
  max_concurrent_jobs = 3
  retry_attempts = 3
  retry_delay_seconds = 5
  worker_shutdown_seconds = 30
}

providers {
  gpg_verification_enabled = true
  gpg_key_url = "https://www.hashicorp.com/.well-known/pgp-key.txt"
}

modules {
  download_timeout_seconds = 600
  download_retry_attempts = 3
  download_retry_initial_delay_ms = 5000
}

quota {
  enabled = false
}

auto_download {
  enabled = false
  rate_limit_per_minute = 10
  max_concurrent_downloads = 3
  allowed_namespaces = ["hashicorp"]
  timeout_seconds = 300
}
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Load the config
	cfg, err := Load(configPath)
	require.NoError(t, err)

	// Verify values from file
	assert.Equal(t, 9090, cfg.Server.Port)
	assert.False(t, cfg.Server.TLSEnabled)
	assert.Equal(t, "test-bucket", cfg.Storage.Bucket)
	assert.Equal(t, "us-west-2", cfg.Storage.Region)
	assert.Equal(t, "/tmp/test.db", cfg.Database.Path)
	assert.Equal(t, 512, cfg.Cache.MemorySizeMB)
	assert.Equal(t, 20, cfg.Cache.DiskSizeGB)
	assert.Equal(t, 24, cfg.Auth.JWTExpirationHours)
	assert.Equal(t, 10, cfg.Auth.BCryptCost)
	assert.Equal(t, "debug", cfg.Logging.Level)
	assert.Equal(t, "json", cfg.Logging.Format)
}

func TestLoadNonExistentFile(t *testing.T) {
	_, err := Load("/nonexistent/config.hcl")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config file not found")
}

func TestEnvOverrides(t *testing.T) {
	// Set environment variables
	os.Setenv("TFM_SERVER_PORT", "3000")
	os.Setenv("TFM_STORAGE_BUCKET", "env-bucket")
	os.Setenv("TFM_CACHE_MEMORY_SIZE_MB", "1024")
	os.Setenv("TFM_AUTH_JWT_EXPIRATION_HOURS", "12")
	os.Setenv("TFM_LOGGING_LEVEL", "error")
	os.Setenv("TFM_FEATURES_AUTO_DOWNLOAD_PROVIDERS", "true")

	defer func() {
		os.Unsetenv("TFM_SERVER_PORT")
		os.Unsetenv("TFM_STORAGE_BUCKET")
		os.Unsetenv("TFM_CACHE_MEMORY_SIZE_MB")
		os.Unsetenv("TFM_AUTH_JWT_EXPIRATION_HOURS")
		os.Unsetenv("TFM_LOGGING_LEVEL")
		os.Unsetenv("TFM_FEATURES_AUTO_DOWNLOAD_PROVIDERS")
	}()

	// Load config with no file (uses defaults + env overrides)
	cfg, err := Load("")
	require.NoError(t, err)

	// Verify environment overrides
	assert.Equal(t, 3000, cfg.Server.Port)
	assert.Equal(t, "env-bucket", cfg.Storage.Bucket)
	assert.Equal(t, 1024, cfg.Cache.MemorySizeMB)
	assert.Equal(t, 12, cfg.Auth.JWTExpirationHours)
	assert.Equal(t, "error", cfg.Logging.Level)
	assert.True(t, cfg.Features.AutoDownloadProviders)
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"true", true},
		{"True", true},
		{"TRUE", true},
		{"yes", true},
		{"Yes", true},
		{"1", true},
		{"false", false},
		{"False", false},
		{"no", false},
		{"0", false},
		{"", false},
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseBool(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateServer(t *testing.T) {
	tests := []struct {
		name        string
		config      ServerConfig
		shouldError bool
		errorMsg    string
	}{
		{
			name:        "valid config",
			config:      ServerConfig{Port: 8080},
			shouldError: false,
		},
		{
			name:        "invalid port - too low",
			config:      ServerConfig{Port: 0},
			shouldError: true,
			errorMsg:    "port must be between",
		},
		{
			name:        "invalid port - too high",
			config:      ServerConfig{Port: 99999},
			shouldError: true,
			errorMsg:    "port must be between",
		},
		{
			name: "TLS enabled without cert",
			config: ServerConfig{
				Port:       8080,
				TLSEnabled: true,
			},
			shouldError: true,
			errorMsg:    "tls_cert_path is required",
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

func TestValidateStorage(t *testing.T) {
	tests := []struct {
		name        string
		config      StorageConfig
		shouldError bool
		errorMsg    string
	}{
		{
			name: "valid S3 config",
			config: StorageConfig{
				Type:   "s3",
				Bucket: "test-bucket",
				Region: "us-east-1",
			},
			shouldError: false,
		},
		{
			name: "invalid storage type",
			config: StorageConfig{
				Type:   "invalid",
				Bucket: "test-bucket",
			},
			shouldError: true,
			errorMsg:    "storage type must be one of",
		},
		{
			name: "missing bucket",
			config: StorageConfig{
				Type:   "s3",
				Region: "us-east-1",
			},
			shouldError: true,
			errorMsg:    "bucket name is required",
		},
		{
			name: "S3 missing region and endpoint",
			config: StorageConfig{
				Type:   "s3",
				Bucket: "test-bucket",
			},
			shouldError: true,
			errorMsg:    "either region or endpoint must be specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateStorage(&tt.config)
			if tt.shouldError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateAuth(t *testing.T) {
	tests := []struct {
		name        string
		config      AuthConfig
		shouldError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: AuthConfig{
				JWTExpirationHours: 8,
				BCryptCost:         12,
			},
			shouldError: false,
		},
		{
			name: "invalid JWT expiration",
			config: AuthConfig{
				JWTExpirationHours: 0,
				BCryptCost:         12,
			},
			shouldError: true,
			errorMsg:    "jwt_expiration_hours must be at least 1",
		},
		{
			name: "bcrypt cost too low",
			config: AuthConfig{
				JWTExpirationHours: 8,
				BCryptCost:         3,
			},
			shouldError: true,
			errorMsg:    "bcrypt_cost must be between 4 and 31",
		},
		{
			name: "bcrypt cost too high",
			config: AuthConfig{
				JWTExpirationHours: 8,
				BCryptCost:         32,
			},
			shouldError: true,
			errorMsg:    "bcrypt_cost must be between 4 and 31",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAuth(&tt.config)
			if tt.shouldError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateLogging(t *testing.T) {
	tests := []struct {
		name        string
		config      LoggingConfig
		shouldError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: LoggingConfig{
				Level:  "info",
				Format: "text",
				Output: "stdout",
			},
			shouldError: false,
		},
		{
			name: "invalid level",
			config: LoggingConfig{
				Level:  "invalid",
				Format: "text",
				Output: "stdout",
			},
			shouldError: true,
			errorMsg:    "logging level must be one of",
		},
		{
			name: "invalid format",
			config: LoggingConfig{
				Level:  "info",
				Format: "invalid",
				Output: "stdout",
			},
			shouldError: true,
			errorMsg:    "logging format must be one of",
		},
		{
			name: "file output without path",
			config: LoggingConfig{
				Level:  "info",
				Format: "text",
				Output: "file",
			},
			shouldError: true,
			errorMsg:    "file_path is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateLogging(&tt.config)
			if tt.shouldError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfigHelperMethods(t *testing.T) {
	cfg := DefaultConfig()

	// Test JWT expiration
	jwtExp := cfg.Auth.GetJWTExpiration()
	assert.Equal(t, 8*time.Hour, jwtExp)

	// Test download retry delay
	retryDelay := cfg.Providers.GetDownloadRetryDelay()
	assert.Equal(t, 1000*time.Millisecond, retryDelay)

	// Test download timeout
	timeout := cfg.Providers.GetDownloadTimeout()
	assert.Equal(t, 60*time.Second, timeout)

	// Test backup interval
	backupInterval := cfg.Database.GetBackupInterval()
	assert.Equal(t, 24*time.Hour, backupInterval)

	// Test cache TTL
	cacheTTL := cfg.Cache.GetCacheTTL()
	assert.Equal(t, 3600*time.Second, cacheTTL)
}

func TestFullValidation(t *testing.T) {
	cfg := DefaultConfig()

	// Default config should be valid
	err := Validate(cfg)
	assert.NoError(t, err)

	// Test invalid nested configs
	cfg.Server.Port = -1
	err = Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "server config")

	// Reset and test storage
	cfg = DefaultConfig()
	cfg.Storage.Bucket = ""
	err = Validate(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "storage config")
}
