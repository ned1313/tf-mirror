package config

import (
	"fmt"
	"os"
	"strings"
)

// Validate checks if the configuration is valid
func Validate(cfg *Config) error {
	if err := validateServer(&cfg.Server); err != nil {
		return fmt.Errorf("server config: %w", err)
	}

	if err := validateStorage(&cfg.Storage); err != nil {
		return fmt.Errorf("storage config: %w", err)
	}

	if err := validateDatabase(&cfg.Database); err != nil {
		return fmt.Errorf("database config: %w", err)
	}

	if err := validateCache(&cfg.Cache); err != nil {
		return fmt.Errorf("cache config: %w", err)
	}

	if err := validateAuth(&cfg.Auth); err != nil {
		return fmt.Errorf("auth config: %w", err)
	}

	if err := validateLogging(&cfg.Logging); err != nil {
		return fmt.Errorf("logging config: %w", err)
	}

	if err := validateTelemetry(&cfg.Telemetry); err != nil {
		return fmt.Errorf("telemetry config: %w", err)
	}

	if err := validateProviders(&cfg.Providers); err != nil {
		return fmt.Errorf("providers config: %w", err)
	}

	if err := validateQuota(&cfg.Quota); err != nil {
		return fmt.Errorf("quota config: %w", err)
	}

	return nil
}

func validateServer(cfg *ServerConfig) error {
	if cfg.Port < 1 || cfg.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", cfg.Port)
	}

	if cfg.TLSEnabled {
		if cfg.TLSCertPath == "" {
			return fmt.Errorf("tls_cert_path is required when TLS is enabled")
		}
		if cfg.TLSKeyPath == "" {
			return fmt.Errorf("tls_key_path is required when TLS is enabled")
		}
		if _, err := os.Stat(cfg.TLSCertPath); os.IsNotExist(err) {
			return fmt.Errorf("tls_cert_path file not found: %s", cfg.TLSCertPath)
		}
		if _, err := os.Stat(cfg.TLSKeyPath); os.IsNotExist(err) {
			return fmt.Errorf("tls_key_path file not found: %s", cfg.TLSKeyPath)
		}
	}

	return nil
}

func validateStorage(cfg *StorageConfig) error {
	validTypes := []string{"s3", "local"}
	if !contains(validTypes, cfg.Type) {
		return fmt.Errorf("storage type must be one of %v, got %s", validTypes, cfg.Type)
	}

	if cfg.Bucket == "" {
		return fmt.Errorf("bucket name is required")
	}

	if cfg.Type == "s3" {
		if cfg.Region == "" && cfg.Endpoint == "" {
			return fmt.Errorf("either region or endpoint must be specified for S3 storage")
		}
	}

	return nil
}

func validateDatabase(cfg *DatabaseConfig) error {
	if cfg.Path == "" {
		return fmt.Errorf("database path is required")
	}

	if cfg.BackupEnabled {
		if cfg.BackupIntervalHours < 1 {
			return fmt.Errorf("backup_interval_hours must be at least 1")
		}
	}

	return nil
}

func validateCache(cfg *CacheConfig) error {
	if cfg.MemorySizeMB < 0 {
		return fmt.Errorf("memory_size_mb cannot be negative")
	}

	if cfg.DiskSizeGB < 0 {
		return fmt.Errorf("disk_size_gb cannot be negative")
	}

	if cfg.TTLSeconds < 0 {
		return fmt.Errorf("ttl_seconds cannot be negative")
	}

	if cfg.DiskPath == "" && cfg.DiskSizeGB > 0 {
		return fmt.Errorf("disk_path is required when disk_size_gb > 0")
	}

	return nil
}

func validateAuth(cfg *AuthConfig) error {
	if cfg.JWTExpirationHours < 1 {
		return fmt.Errorf("jwt_expiration_hours must be at least 1")
	}

	if cfg.BCryptCost < 4 || cfg.BCryptCost > 31 {
		return fmt.Errorf("bcrypt_cost must be between 4 and 31, got %d", cfg.BCryptCost)
	}

	return nil
}

func validateLogging(cfg *LoggingConfig) error {
	validLevels := []string{"debug", "info", "warn", "error"}
	if !contains(validLevels, cfg.Level) {
		return fmt.Errorf("logging level must be one of %v, got %s", validLevels, cfg.Level)
	}

	validFormats := []string{"text", "json"}
	if !contains(validFormats, cfg.Format) {
		return fmt.Errorf("logging format must be one of %v, got %s", validFormats, cfg.Format)
	}

	validOutputs := []string{"stdout", "stderr", "file", "both"}
	if !contains(validOutputs, cfg.Output) {
		return fmt.Errorf("logging output must be one of %v, got %s", validOutputs, cfg.Output)
	}

	if (cfg.Output == "file" || cfg.Output == "both") && cfg.FilePath == "" {
		return fmt.Errorf("file_path is required when output is 'file' or 'both'")
	}

	return nil
}

func validateTelemetry(cfg *TelemetryConfig) error {
	if cfg.OtelEnabled {
		if cfg.OtelEndpoint == "" {
			return fmt.Errorf("otel_endpoint is required when OpenTelemetry is enabled")
		}

		validProtocols := []string{"grpc", "http"}
		if !contains(validProtocols, cfg.OtelProtocol) {
			return fmt.Errorf("otel_protocol must be one of %v, got %s", validProtocols, cfg.OtelProtocol)
		}
	}

	return nil
}

func validateProviders(cfg *ProvidersConfig) error {
	if cfg.GPGVerificationEnabled && cfg.GPGKeyURL == "" {
		return fmt.Errorf("gpg_key_url is required when GPG verification is enabled")
	}

	if cfg.DownloadRetryAttempts < 0 {
		return fmt.Errorf("download_retry_attempts cannot be negative")
	}

	if cfg.DownloadRetryInitialDelayMs < 0 {
		return fmt.Errorf("download_retry_initial_delay_ms cannot be negative")
	}

	if cfg.DownloadTimeoutSeconds < 1 {
		return fmt.Errorf("download_timeout_seconds must be at least 1")
	}

	return nil
}

func validateQuota(cfg *QuotaConfig) error {
	if cfg.Enabled {
		if cfg.MaxStorageGB < 1 {
			return fmt.Errorf("max_storage_gb must be at least 1 when quota is enabled")
		}

		if cfg.WarningThresholdPercent < 1 || cfg.WarningThresholdPercent > 100 {
			return fmt.Errorf("warning_threshold_percent must be between 1 and 100")
		}
	}

	return nil
}

// contains checks if a string slice contains a value
func contains(slice []string, val string) bool {
	val = strings.ToLower(val)
	for _, item := range slice {
		if strings.ToLower(item) == val {
			return true
		}
	}
	return false
}
