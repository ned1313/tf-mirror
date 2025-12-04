package storage

import (
	"context"
	"fmt"

	"github.com/ned1313/terraform-mirror/internal/config"
)

// NewFromConfig creates a storage instance from application configuration
func NewFromConfig(ctx context.Context, cfg config.StorageConfig) (Storage, error) {
	switch cfg.Type {
	case "s3":
		return NewS3Storage(ctx, S3Config{
			Region:         cfg.Region,
			Bucket:         cfg.Bucket,
			Endpoint:       cfg.Endpoint,
			AccessKey:      cfg.AccessKey,
			SecretKey:      cfg.SecretKey,
			ForcePathStyle: cfg.ForcePathStyle,
		})
	case "local":
		// Use endpoint as base path for local storage
		basePath := cfg.Endpoint
		if basePath == "" {
			basePath = "/var/lib/tf-mirror/storage"
		}
		return NewLocalStorage(LocalConfig{
			BasePath: basePath,
		})
	default:
		return nil, fmt.Errorf("unsupported storage type: %s (supported: s3, local)", cfg.Type)
	}
}

// BuildProviderKey generates the S3 key for a provider binary
// Format: providers/{hostname}/{namespace}/{type}/{version}/{os}_{arch}/{filename}
func BuildProviderKey(hostname, namespace, providerType, version, os, arch, filename string) string {
	return fmt.Sprintf("providers/%s/%s/%s/%s/%s_%s/%s",
		hostname, namespace, providerType, version, os, arch, filename)
}

// BuildModuleKey generates the S3 key for a module archive
// Format: modules/{hostname}/{namespace}/{name}/{provider}/{version}/{filename}
func BuildModuleKey(hostname, namespace, name, provider, version, filename string) string {
	return fmt.Sprintf("modules/%s/%s/%s/%s/%s/%s",
		hostname, namespace, name, provider, version, filename)
}

// BuildBackupKey generates the S3 key for a database backup
// Format: backups/{timestamp}.db
func BuildBackupKey(timestamp string) string {
	return fmt.Sprintf("backups/%s.db", timestamp)
}
