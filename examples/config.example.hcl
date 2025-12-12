# Example configuration file for Terraform Mirror
# Copy to /etc/tf-mirror/config.hcl and customize

server {
  port = 8080
  tls_enabled = false
  tls_cert_path = "/etc/tf-mirror/cert.pem"
  tls_key_path = "/etc/tf-mirror/key.pem"
  
  # Set to true if running behind a reverse proxy
  behind_proxy = false
  trusted_proxies = ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"]
}

storage {
  type = "s3"
  bucket = "terraform-mirror"
  region = "us-east-1"
  
  # For MinIO or other S3-compatible endpoints
  endpoint = ""
  
  # Leave empty to use IAM roles (recommended for AWS)
  # Or provide credentials for MinIO/other S3-compatible stores
  access_key = ""
  secret_key = ""
  
  # Set to true for MinIO
  force_path_style = false
}

database {
  path = "/data/terraform-mirror.db"
  
  # Automatic database backups
  backup_enabled = true
  backup_interval_hours = 24
  backup_to_s3 = true
  backup_s3_prefix = "backups/"
}

cache {
  # In-memory LRU cache size in MB
  # Memory cache is faster but limited in size
  # Set to 0 to disable memory caching
  # Environment variable: TFM_CACHE_MEMORY_SIZE_MB
  memory_size_mb = 256
  
  # Disk cache location and size
  # Disk cache provides larger capacity but slower access
  # Set disk_size_gb to 0 to disable disk caching
  # Environment variables: TFM_CACHE_DISK_PATH, TFM_CACHE_DISK_SIZE_GB
  disk_path = "/var/cache/tf-mirror"
  disk_size_gb = 10
  
  # Cache TTL in seconds (default: 1 hour)
  # Items older than this are automatically removed
  # Environment variable: TFM_CACHE_TTL_SECONDS
  ttl_seconds = 3600
}

features {
  # Phase 1: disabled, Phase 2: enable
  auto_download_providers = false
  auto_download_modules = false
  
  # Maximum download size in MB
  max_download_size_mb = 500
}

auth {
  # JWT token expiration in hours
  jwt_expiration_hours = 8
  
  # Bcrypt cost factor (10-14 recommended)
  bcrypt_cost = 12
}

logging {
  # Levels: debug, info, warn, error
  level = "info"
  
  # Format: text, json
  format = "text"
  
  # Output: stdout, stderr, file, both
  output = "stdout"
  file_path = "/var/log/tf-mirror/app.log"
}

telemetry {
  enabled = true
  
  # OpenTelemetry configuration
  otel_enabled = false
  otel_endpoint = "localhost:4317"
  otel_protocol = "grpc"  # grpc or http
  
  export_traces = true
  export_metrics = true
}

providers {
  # Enable GPG signature verification
  gpg_verification_enabled = true
  gpg_key_url = "https://www.hashicorp.com/.well-known/pgp-key.txt"
  
  # Download retry configuration
  download_retry_attempts = 5
  download_retry_initial_delay_ms = 1000
  download_timeout_seconds = 60
}

# Storage quota management
quota {
  enabled = false
  max_storage_gb = 0  # 0 = unlimited
  warning_threshold_percent = 80
}

# Auto-download configuration for on-demand provider downloads
# When enabled, providers not in the cache will be fetched from the upstream registry
auto_download {
  enabled = false
  
  # Namespaces to allow/block for auto-download
  # Empty allowed_namespaces means all namespaces are allowed
  # blocked_namespaces takes precedence over allowed_namespaces
  allowed_namespaces = []
  blocked_namespaces = []
  
  # Platforms to auto-download when a provider is requested
  # When a provider version is requested, it will be downloaded for all these platforms
  # Default: ["linux_amd64", "windows_amd64"]
  platforms = ["linux_amd64", "windows_amd64"]
  
  # Rate limiting
  rate_limit_per_minute = 10
  max_concurrent_downloads = 3
  queue_size = 100
  timeout_seconds = 300
  
  # Retry configuration
  retry_on_failure = true
  
  # Cache negative (not found) results to avoid repeated upstream requests
  cache_negative_results = true
  negative_cache_ttl_seconds = 300
}
