package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
)

// Load reads configuration from a file and applies environment variable overrides
func Load(configPath string) (*Config, error) {
	// Start with defaults
	cfg := DefaultConfig()

	// Load from HCL file if provided
	if configPath != "" {
		if err := loadFromFile(configPath, cfg); err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
	}

	// Apply environment variable overrides
	applyEnvOverrides(cfg)

	// Validate the configuration
	if err := Validate(cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// loadFromFile parses an HCL configuration file
func loadFromFile(path string, cfg *Config) error {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("config file not found: %s", path)
	}

	// Parse HCL file
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCLFile(path)
	if diags.HasErrors() {
		return fmt.Errorf("failed to parse HCL file: %s", diags.Error())
	}

	// Decode into config struct
	diags = gohcl.DecodeBody(file.Body, nil, cfg)
	if diags.HasErrors() {
		return fmt.Errorf("failed to decode HCL: %s", diags.Error())
	}

	return nil
}

// applyEnvOverrides applies environment variable overrides with TFM_ prefix
func applyEnvOverrides(cfg *Config) {
	// Server configuration
	if val := os.Getenv("TFM_SERVER_PORT"); val != "" {
		if port, err := strconv.Atoi(val); err == nil {
			cfg.Server.Port = port
		}
	}
	if val := os.Getenv("TFM_SERVER_TLS_ENABLED"); val != "" {
		cfg.Server.TLSEnabled = parseBool(val)
	}
	if val := os.Getenv("TFM_SERVER_TLS_CERT_PATH"); val != "" {
		cfg.Server.TLSCertPath = val
	}
	if val := os.Getenv("TFM_SERVER_TLS_KEY_PATH"); val != "" {
		cfg.Server.TLSKeyPath = val
	}
	if val := os.Getenv("TFM_SERVER_BEHIND_PROXY"); val != "" {
		cfg.Server.BehindProxy = parseBool(val)
	}

	// Storage configuration
	if val := os.Getenv("TFM_STORAGE_TYPE"); val != "" {
		cfg.Storage.Type = val
	}
	if val := os.Getenv("TFM_STORAGE_BUCKET"); val != "" {
		cfg.Storage.Bucket = val
	}
	if val := os.Getenv("TFM_STORAGE_REGION"); val != "" {
		cfg.Storage.Region = val
	}
	if val := os.Getenv("TFM_STORAGE_ENDPOINT"); val != "" {
		cfg.Storage.Endpoint = val
	}
	if val := os.Getenv("TFM_STORAGE_ACCESS_KEY"); val != "" {
		cfg.Storage.AccessKey = val
	}
	if val := os.Getenv("TFM_STORAGE_SECRET_KEY"); val != "" {
		cfg.Storage.SecretKey = val
	}
	if val := os.Getenv("TFM_STORAGE_FORCE_PATH_STYLE"); val != "" {
		cfg.Storage.ForcePathStyle = parseBool(val)
	}

	// Database configuration
	if val := os.Getenv("TFM_DATABASE_PATH"); val != "" {
		cfg.Database.Path = val
	}
	if val := os.Getenv("TFM_DATABASE_BACKUP_ENABLED"); val != "" {
		cfg.Database.BackupEnabled = parseBool(val)
	}

	// Cache configuration
	if val := os.Getenv("TFM_CACHE_MEMORY_SIZE_MB"); val != "" {
		if size, err := strconv.Atoi(val); err == nil {
			cfg.Cache.MemorySizeMB = size
		}
	}
	if val := os.Getenv("TFM_CACHE_DISK_PATH"); val != "" {
		cfg.Cache.DiskPath = val
	}
	if val := os.Getenv("TFM_CACHE_DISK_SIZE_GB"); val != "" {
		if size, err := strconv.Atoi(val); err == nil {
			cfg.Cache.DiskSizeGB = size
		}
	}
	if val := os.Getenv("TFM_CACHE_TTL_SECONDS"); val != "" {
		if ttl, err := strconv.Atoi(val); err == nil {
			cfg.Cache.TTLSeconds = ttl
		}
	}

	// Features configuration
	if val := os.Getenv("TFM_FEATURES_AUTO_DOWNLOAD_PROVIDERS"); val != "" {
		cfg.Features.AutoDownloadProviders = parseBool(val)
	}
	if val := os.Getenv("TFM_FEATURES_AUTO_DOWNLOAD_MODULES"); val != "" {
		cfg.Features.AutoDownloadModules = parseBool(val)
	}

	// Auth configuration
	if val := os.Getenv("TFM_AUTH_JWT_EXPIRATION_HOURS"); val != "" {
		if hours, err := strconv.Atoi(val); err == nil {
			cfg.Auth.JWTExpirationHours = hours
		}
	}
	if val := os.Getenv("TFM_AUTH_BCRYPT_COST"); val != "" {
		if cost, err := strconv.Atoi(val); err == nil {
			cfg.Auth.BCryptCost = cost
		}
	}

	// Logging configuration
	if val := os.Getenv("TFM_LOGGING_LEVEL"); val != "" {
		cfg.Logging.Level = val
	}
	if val := os.Getenv("TFM_LOGGING_FORMAT"); val != "" {
		cfg.Logging.Format = val
	}
	if val := os.Getenv("TFM_LOGGING_OUTPUT"); val != "" {
		cfg.Logging.Output = val
	}
	if val := os.Getenv("TFM_LOGGING_FILE_PATH"); val != "" {
		cfg.Logging.FilePath = val
	}

	// Telemetry configuration
	if val := os.Getenv("TFM_TELEMETRY_ENABLED"); val != "" {
		cfg.Telemetry.Enabled = parseBool(val)
	}
	if val := os.Getenv("TFM_TELEMETRY_OTEL_ENABLED"); val != "" {
		cfg.Telemetry.OtelEnabled = parseBool(val)
	}
	if val := os.Getenv("TFM_TELEMETRY_OTEL_ENDPOINT"); val != "" {
		cfg.Telemetry.OtelEndpoint = val
	}

	// Providers configuration
	if val := os.Getenv("TFM_PROVIDERS_GPG_VERIFICATION_ENABLED"); val != "" {
		cfg.Providers.GPGVerificationEnabled = parseBool(val)
	}
	if val := os.Getenv("TFM_PROVIDERS_GPG_KEY_URL"); val != "" {
		cfg.Providers.GPGKeyURL = val
	}

	// Quota configuration
	if val := os.Getenv("TFM_QUOTA_ENABLED"); val != "" {
		cfg.Quota.Enabled = parseBool(val)
	}
	if val := os.Getenv("TFM_QUOTA_MAX_STORAGE_GB"); val != "" {
		if max, err := strconv.Atoi(val); err == nil {
			cfg.Quota.MaxStorageGB = max
		}
	}
}

// parseBool parses a boolean value from string (supports: true/false, yes/no, 1/0)
func parseBool(val string) bool {
	val = strings.ToLower(strings.TrimSpace(val))
	return val == "true" || val == "yes" || val == "1"
}
