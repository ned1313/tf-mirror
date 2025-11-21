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
  # In-memory cache size in MB
  memory_size_mb = 256
  
  # Disk cache location and size
  disk_path = "/var/cache/tf-mirror"
  disk_size_gb = 10
  
  # Cache TTL in seconds
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
