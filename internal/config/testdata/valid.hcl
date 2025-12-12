server {
  port = 9090
  tls_enabled = false
}

storage {
  type = "s3"
  bucket = "test-terraform-mirror"
  region = "us-west-2"
  endpoint = ""
  force_path_style = false
}

database {
  path = "/tmp/test-terraform-mirror.db"
  backup_enabled = false
}

cache {
  memory_size_mb = 128
  disk_path = "/tmp/cache"
  disk_size_gb = 5
  ttl_seconds = 1800
}

features {
  auto_download_providers = false
  auto_download_modules = false
  max_download_size_mb = 250
}

auth {
  jwt_expiration_hours = 12
  bcrypt_cost = 10
}

logging {
  level = "debug"
  format = "json"
  output = "stdout"
}

telemetry {
  enabled = false
  otel_enabled = false
}

providers {
  gpg_verification_enabled = true
  gpg_key_url = "https://www.hashicorp.com/.well-known/pgp-key.txt"
  download_retry_attempts = 3
  download_retry_initial_delay_ms = 500
  download_timeout_seconds = 30
}

quota {
  enabled = false
  max_storage_gb = 0
  warning_threshold_percent = 80
}

auto_download {
  enabled = false
  allowed_namespaces = []
  blocked_namespaces = []
  platforms = ["linux_amd64", "windows_amd64"]
  rate_limit_per_minute = 10
  max_concurrent_downloads = 3
  queue_size = 100
  timeout_seconds = 300
  retry_on_failure = true
  cache_negative_results = true
  negative_cache_ttl_seconds = 300
}
