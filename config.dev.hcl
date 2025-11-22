# Development configuration for terraform-mirror with MinIO

server {
  port = 8080
  tls_enabled = false
  behind_proxy = false
}

storage {
  type = "s3"
  bucket = "terraform-mirror"
  region = "us-east-1"
  endpoint = "http://localhost:9000"
  # Credentials via environment variables:
  # AWS_ACCESS_KEY_ID=minioadmin
  # AWS_SECRET_ACCESS_KEY=minioadmin
  force_path_style = true  # Required for MinIO
}

database {
  path = "terraform-mirror-dev.db"
  backup_enabled = false
}

cache {
  memory_size_mb = 256
  disk_path = "./cache"
  disk_size_gb = 10
  ttl_seconds = 86400  # 24 hours
}

features {
  auto_download_providers = false
  auto_download_modules = false
  max_download_size_mb = 500
}

auth {
  jwt_expiration_hours = 8
  bcrypt_cost = 12
  jwt_secret = "dev-secret-key-change-in-production-use-at-least-32-chars"
}

processor {
  polling_interval_seconds = 5    # Check for new jobs every 5 seconds
  max_concurrent_jobs = 3         # Process up to 3 jobs simultaneously
  retry_attempts = 3              # Retry failed downloads up to 3 times
  retry_delay_seconds = 5         # Wait 5 seconds between retries
  worker_shutdown_seconds = 30    # Allow 30 seconds for graceful shutdown
}

logging {
  level = "debug"
  format = "text"
  output = "stdout"
}

telemetry {
  enabled = false
}

providers {
  gpg_verification_enabled = true
  download_retry_attempts = 3
  download_retry_initial_delay_ms = 1000
  download_timeout_seconds = 300
}

quota {
  enabled = false
}
