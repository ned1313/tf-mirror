# Configuration Reference

Terraform Mirror can be configured using HCL configuration files, environment variables, or a combination of both. Environment variables take precedence over configuration file values.

## Table of Contents

- [Configuration Methods](#configuration-methods)
- [Server Configuration](#server-configuration)
- [Storage Configuration](#storage-configuration)
- [Database Configuration](#database-configuration)
- [Cache Configuration](#cache-configuration)
- [Authentication Configuration](#authentication-configuration)
- [Processor Configuration](#processor-configuration)
- [Logging Configuration](#logging-configuration)
- [Telemetry Configuration](#telemetry-configuration)
- [Provider Configuration](#provider-configuration)
- [Module Configuration](#module-configuration)
- [Quota Configuration](#quota-configuration)
- [Feature Flags](#feature-flags)
- [Complete Example](#complete-example)

## Configuration Methods

### HCL Configuration File

Create a configuration file (e.g., `/etc/tf-mirror/config.hcl`) and pass it to the application:

```bash
terraform-mirror -config /etc/tf-mirror/config.hcl
```

### Environment Variables

All configuration options can be set via environment variables with the `TFM_` prefix. Environment variables use uppercase with underscores separating words.

**Naming Convention:**
- HCL: `server { port = 8080 }` → Environment: `TFM_SERVER_PORT=8080`
- Nested values use underscores: `cache { memory_size_mb = 256 }` → `TFM_CACHE_MEMORY_SIZE_MB=256`

### Configuration Precedence

1. Environment variables (highest priority)
2. Configuration file values
3. Default values (lowest priority)

---

## Server Configuration

HTTP server settings.

### HCL Block

```hcl
server {
  port           = 8080
  tls_enabled    = false
  tls_cert_path  = "/etc/tf-mirror/cert.pem"
  tls_key_path   = "/etc/tf-mirror/key.pem"
  behind_proxy   = false
  trusted_proxies = ["10.0.0.0/8", "172.16.0.0/12"]
}
```

### Options

| Option | Environment Variable | Type | Default | Description |
|--------|---------------------|------|---------|-------------|
| `port` | `TFM_SERVER_PORT` | int | `8080` | HTTP server port |
| `tls_enabled` | `TFM_SERVER_TLS_ENABLED` | bool | `false` | Enable HTTPS |
| `tls_cert_path` | `TFM_SERVER_TLS_CERT_PATH` | string | `""` | Path to TLS certificate file |
| `tls_key_path` | `TFM_SERVER_TLS_KEY_PATH` | string | `""` | Path to TLS private key file |
| `behind_proxy` | `TFM_SERVER_BEHIND_PROXY` | bool | `false` | Enable when running behind a reverse proxy |
| `trusted_proxies` | - | list | `[]` | CIDR ranges of trusted proxy IPs |

### Examples

**Basic HTTP server:**
```bash
export TFM_SERVER_PORT=8080
```

**HTTPS with certificates:**
```bash
export TFM_SERVER_TLS_ENABLED=true
export TFM_SERVER_TLS_CERT_PATH=/etc/tf-mirror/server.crt
export TFM_SERVER_TLS_KEY_PATH=/etc/tf-mirror/server.key
```

**Behind reverse proxy (nginx, traefik, etc.):**
```bash
export TFM_SERVER_BEHIND_PROXY=true
```

---

## Storage Configuration

Object storage settings for provider binaries.

### HCL Block

```hcl
storage {
  type             = "s3"
  bucket           = "terraform-mirror"
  region           = "us-east-1"
  endpoint         = ""
  access_key       = ""
  secret_key       = ""
  force_path_style = false
}
```

### Options

| Option | Environment Variable | Type | Default | Description |
|--------|---------------------|------|---------|-------------|
| `type` | `TFM_STORAGE_TYPE` | string | `"s3"` | Storage type: `s3` or `local` |
| `bucket` | `TFM_STORAGE_BUCKET` | string | `"terraform-mirror"` | S3 bucket name |
| `region` | `TFM_STORAGE_REGION` | string | `"us-east-1"` | AWS region |
| `endpoint` | `TFM_STORAGE_ENDPOINT` | string | `""` | Custom S3 endpoint (for MinIO, etc.) |
| `access_key` | `TFM_STORAGE_ACCESS_KEY` | string | `""` | S3 access key ID |
| `secret_key` | `TFM_STORAGE_SECRET_KEY` | string | `""` | S3 secret access key |
| `force_path_style` | `TFM_STORAGE_FORCE_PATH_STYLE` | bool | `false` | Use path-style URLs (required for MinIO) |

### Storage Types

#### AWS S3

```bash
export TFM_STORAGE_TYPE=s3
export TFM_STORAGE_BUCKET=my-terraform-providers
export TFM_STORAGE_REGION=us-west-2
export TFM_STORAGE_ACCESS_KEY=AKIAIOSFODNN7EXAMPLE
export TFM_STORAGE_SECRET_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

#### AWS S3 with IAM Role (EC2, ECS, EKS)

When running on AWS infrastructure with IAM roles, omit credentials:

```bash
export TFM_STORAGE_TYPE=s3
export TFM_STORAGE_BUCKET=my-terraform-providers
export TFM_STORAGE_REGION=us-west-2
# Access key and secret key are not needed - SDK uses instance metadata
```

#### MinIO

```bash
export TFM_STORAGE_TYPE=s3
export TFM_STORAGE_BUCKET=terraform-mirror
export TFM_STORAGE_ENDPOINT=http://minio:9000
export TFM_STORAGE_ACCESS_KEY=minioadmin
export TFM_STORAGE_SECRET_KEY=minioadmin
export TFM_STORAGE_FORCE_PATH_STYLE=true
```

#### Local Filesystem

```bash
export TFM_STORAGE_TYPE=local
export TFM_STORAGE_ENDPOINT=/var/lib/tf-mirror/storage
```

---

## Database Configuration

SQLite database settings for metadata storage.

### HCL Block

```hcl
database {
  path                  = "/data/terraform-mirror.db"
  backup_enabled        = true
  backup_interval_hours = 24
  backup_to_s3          = true
  backup_s3_prefix      = "backups/"
}
```

### Options

| Option | Environment Variable | Type | Default | Description |
|--------|---------------------|------|---------|-------------|
| `path` | `TFM_DATABASE_PATH` | string | `"/data/terraform-mirror.db"` | Path to SQLite database file |
| `backup_enabled` | `TFM_DATABASE_BACKUP_ENABLED` | bool | `false` | Enable automatic backups |
| `backup_interval_hours` | - | int | `24` | Hours between backups |
| `backup_to_s3` | - | bool | `false` | Store backups in S3 |
| `backup_s3_prefix` | - | string | `"backups/"` | S3 prefix for backup files |

### Examples

```bash
export TFM_DATABASE_PATH=/data/terraform-mirror.db
export TFM_DATABASE_BACKUP_ENABLED=true
```

---

## Cache Configuration

Two-tier caching settings (memory + disk).

### HCL Block

```hcl
cache {
  memory_size_mb = 256
  disk_path      = "/var/cache/tf-mirror"
  disk_size_gb   = 10
  ttl_seconds    = 3600
}
```

### Options

| Option | Environment Variable | Type | Default | Description |
|--------|---------------------|------|---------|-------------|
| `memory_size_mb` | `TFM_CACHE_MEMORY_SIZE_MB` | int | `256` | In-memory LRU cache size (MB) |
| `disk_path` | `TFM_CACHE_DISK_PATH` | string | `"/var/cache/tf-mirror"` | Disk cache directory |
| `disk_size_gb` | `TFM_CACHE_DISK_SIZE_GB` | int | `10` | Maximum disk cache size (GB) |
| `ttl_seconds` | `TFM_CACHE_TTL_SECONDS` | int | `3600` | Cache entry time-to-live (seconds) |

### Cache Behavior

- **Memory Cache (L1)**: Fast, limited size, LRU eviction
- **Disk Cache (L2)**: Larger capacity, persistent across restarts
- **Tiered Operation**: Items are promoted from disk to memory on access

### Disabling Cache

To disable caching entirely, set both sizes to 0:

```bash
export TFM_CACHE_MEMORY_SIZE_MB=0
export TFM_CACHE_DISK_SIZE_GB=0
```

### Cache Sizing Guidelines

| Environment | Memory | Disk | Notes |
|-------------|--------|------|-------|
| Development | 64 MB | 1 GB | Minimal resources |
| Small team | 256 MB | 10 GB | Default settings |
| Enterprise | 1024 MB | 100 GB | High traffic, many providers |

---

## Authentication Configuration

JWT and password hashing settings.

### HCL Block

```hcl
auth {
  jwt_secret           = ""
  jwt_expiration_hours = 8
  bcrypt_cost          = 12
}
```

### Options

| Option | Environment Variable | Type | Default | Description |
|--------|---------------------|------|---------|-------------|
| `jwt_secret` | `TFM_AUTH_JWT_SECRET` | string | `""` | Secret key for JWT signing (auto-generated if empty) |
| `jwt_expiration_hours` | `TFM_AUTH_JWT_EXPIRATION_HOURS` | int | `8` | JWT token validity period |
| `bcrypt_cost` | `TFM_AUTH_BCRYPT_COST` | int | `12` | BCrypt hashing cost (10-14 recommended) |

### Initial Admin User

Set the initial admin credentials via environment variables:

| Environment Variable | Description |
|---------------------|-------------|
| `TFM_ADMIN_USERNAME` | Initial admin username |
| `TFM_ADMIN_PASSWORD` | Initial admin password |

**Important:** These credentials are only used to create the initial admin user on first startup. Change the password after first login.

### Examples

```bash
export TFM_ADMIN_USERNAME=admin
export TFM_ADMIN_PASSWORD=mysecurepassword
export TFM_AUTH_JWT_EXPIRATION_HOURS=24
```

---

## Processor Configuration

Background job processor settings.

### HCL Block

```hcl
processor {
  polling_interval_seconds = 10
  max_concurrent_jobs      = 3
  retry_attempts           = 3
  retry_delay_seconds      = 5
  worker_shutdown_seconds  = 30
}
```

### Options

| Option | Environment Variable | Type | Default | Description |
|--------|---------------------|------|---------|-------------|
| `polling_interval_seconds` | - | int | `10` | Interval between job queue checks |
| `max_concurrent_jobs` | - | int | `3` | Maximum parallel job execution |
| `retry_attempts` | - | int | `3` | Number of retry attempts for failed jobs |
| `retry_delay_seconds` | - | int | `5` | Delay between retries |
| `worker_shutdown_seconds` | - | int | `30` | Grace period for worker shutdown |

### Tuning Guidelines

- **High throughput**: Increase `max_concurrent_jobs` (consider network bandwidth)
- **Unreliable network**: Increase `retry_attempts` and `retry_delay_seconds`
- **Slow shutdown**: Decrease `worker_shutdown_seconds`

---

## Logging Configuration

Application logging settings.

### HCL Block

```hcl
logging {
  level     = "info"
  format    = "text"
  output    = "stdout"
  file_path = "/var/log/tf-mirror/app.log"
}
```

### Options

| Option | Environment Variable | Type | Default | Description |
|--------|---------------------|------|---------|-------------|
| `level` | `TFM_LOGGING_LEVEL` | string | `"info"` | Log level: `debug`, `info`, `warn`, `error` |
| `format` | `TFM_LOGGING_FORMAT` | string | `"text"` | Output format: `text` or `json` |
| `output` | `TFM_LOGGING_OUTPUT` | string | `"stdout"` | Output destination: `stdout`, `stderr`, `file`, `both` |
| `file_path` | `TFM_LOGGING_FILE_PATH` | string | `""` | Log file path (required if output includes `file`) |

### Examples

**Development (verbose):**
```bash
export TFM_LOGGING_LEVEL=debug
export TFM_LOGGING_FORMAT=text
```

**Production (JSON for log aggregation):**
```bash
export TFM_LOGGING_LEVEL=info
export TFM_LOGGING_FORMAT=json
export TFM_LOGGING_OUTPUT=stdout
```

---

## Telemetry Configuration

Observability and metrics settings.

### HCL Block

```hcl
telemetry {
  enabled        = true
  otel_enabled   = false
  otel_endpoint  = "localhost:4317"
  otel_protocol  = "grpc"
  export_traces  = true
  export_metrics = true
}
```

### Options

| Option | Environment Variable | Type | Default | Description |
|--------|---------------------|------|---------|-------------|
| `enabled` | `TFM_TELEMETRY_ENABLED` | bool | `false` | Enable telemetry endpoints |
| `otel_enabled` | `TFM_TELEMETRY_OTEL_ENABLED` | bool | `false` | Enable OpenTelemetry export |
| `otel_endpoint` | `TFM_TELEMETRY_OTEL_ENDPOINT` | string | `""` | OpenTelemetry collector endpoint |
| `otel_protocol` | - | string | `"grpc"` | Protocol: `grpc` or `http` |
| `export_traces` | - | bool | `false` | Export distributed traces |
| `export_metrics` | - | bool | `false` | Export metrics |

---

## Provider Configuration

Provider download and verification settings.

### HCL Block

```hcl
providers {
  gpg_verification_enabled       = true
  gpg_key_url                    = "https://www.hashicorp.com/.well-known/pgp-key.txt"
  download_retry_attempts        = 5
  download_retry_initial_delay_ms = 1000
  download_timeout_seconds       = 60
}
```

### Options

| Option | Environment Variable | Type | Default | Description |
|--------|---------------------|------|---------|-------------|
| `gpg_verification_enabled` | `TFM_PROVIDERS_GPG_VERIFICATION_ENABLED` | bool | `true` | Verify provider GPG signatures |
| `gpg_key_url` | `TFM_PROVIDERS_GPG_KEY_URL` | string | HashiCorp URL | URL to fetch GPG public key |
| `download_retry_attempts` | - | int | `5` | Maximum download retry attempts |
| `download_retry_initial_delay_ms` | - | int | `1000` | Initial retry delay (exponential backoff) |
| `download_timeout_seconds` | - | int | `60` | Download timeout per attempt |

---

## Module Configuration

Module download settings for the module registry mirror.

### HCL Block

```hcl
modules {
  upstream_registry            = "registry.terraform.io"
  download_retry_attempts      = 3
  download_timeout_seconds     = 300
}
```

### Options

| Option | Environment Variable | Type | Default | Description |
|--------|---------------------|------|---------|-------------|
| `upstream_registry` | `TFM_MODULES_UPSTREAM_REGISTRY` | string | `registry.terraform.io` | Upstream module registry |
| `download_retry_attempts` | - | int | `3` | Maximum download retry attempts |
| `download_timeout_seconds` | - | int | `300` | Download timeout (modules can be large) |

### Module Sources

The module mirror supports downloading modules from:

- **Git URLs** (`git::https://...`) - Most common for public modules
- **HTTP/HTTPS URLs** - Direct tarball downloads
- **Terraform Registry** - Proxies to upstream registry

### Example Module Definition

```hcl
# modules.hcl - Upload via Admin API
module "hashicorp/consul/aws" {
  versions = ["0.11.0", "0.10.0"]
}

module "hashicorp/vpc/aws" {
  versions = ["5.0.0", "4.0.0", "3.0.0"]
}
```

---

## Quota Configuration

Storage quota and limits.

### HCL Block

```hcl
quota {
  enabled                   = false
  max_storage_gb            = 0
  warning_threshold_percent = 80
}
```

### Options

| Option | Environment Variable | Type | Default | Description |
|--------|---------------------|------|---------|-------------|
| `enabled` | `TFM_QUOTA_ENABLED` | bool | `false` | Enable storage quotas |
| `max_storage_gb` | `TFM_QUOTA_MAX_STORAGE_GB` | int | `0` | Maximum storage (0 = unlimited) |
| `warning_threshold_percent` | - | int | `80` | Storage warning threshold percentage |

---

## Feature Flags

Enable/disable optional features.

### HCL Block

```hcl
features {
  auto_download_providers = false
  auto_download_modules   = false
  max_download_size_mb    = 500
}
```

### Options

| Option | Environment Variable | Type | Default | Description |
|--------|---------------------|------|---------|-------------|
| `auto_download_providers` | `TFM_FEATURES_AUTO_DOWNLOAD_PROVIDERS` | bool | `false` | Auto-download providers on first request |
| `auto_download_modules` | `TFM_FEATURES_AUTO_DOWNLOAD_MODULES` | bool | `false` | Auto-download modules on first request |
| `max_download_size_mb` | - | int | `500` | Maximum single file download size |

### Auto-Download Behavior

When enabled, auto-download features will:

1. **Providers**: When a Terraform client requests a provider version that isn't cached, the mirror will fetch it from the upstream registry (registry.terraform.io) and cache it before responding.

2. **Modules**: When a Terraform client requests a module version that isn't cached, the mirror will:
   - Query the upstream registry for the download URL
   - Clone Git repositories or download HTTP tarballs
   - Cache the module tarball before responding

**Note:** Auto-download adds latency to the first request for uncached resources. For air-gapped environments, pre-load all required providers and modules using the Admin API.

---

## Complete Example

### HCL Configuration File

```hcl
# /etc/tf-mirror/config.hcl

server {
  port           = 8080
  tls_enabled    = false
  behind_proxy   = true
  trusted_proxies = ["10.0.0.0/8"]
}

storage {
  type             = "s3"
  bucket           = "terraform-mirror-prod"
  region           = "us-west-2"
  # Credentials via environment variables or IAM role
}

database {
  path                  = "/data/terraform-mirror.db"
  backup_enabled        = true
  backup_interval_hours = 6
  backup_to_s3          = true
  backup_s3_prefix      = "backups/db/"
}

cache {
  memory_size_mb = 512
  disk_path      = "/var/cache/tf-mirror"
  disk_size_gb   = 50
  ttl_seconds    = 7200
}

auth {
  jwt_expiration_hours = 12
  bcrypt_cost          = 12
}

processor {
  max_concurrent_jobs = 5
  retry_attempts      = 5
}

logging {
  level  = "info"
  format = "json"
  output = "stdout"
}

telemetry {
  enabled      = true
  otel_enabled = true
  otel_endpoint = "otel-collector:4317"
}

providers {
  gpg_verification_enabled = true
  download_retry_attempts  = 5
  download_timeout_seconds = 120
}

modules {
  upstream_registry        = "registry.terraform.io"
  download_retry_attempts  = 3
  download_timeout_seconds = 300
}

quota {
  enabled                   = true
  max_storage_gb            = 500
  warning_threshold_percent = 80
}

features {
  auto_download_providers = false
  auto_download_modules   = false
}
```

### Environment Variables Only

```bash
# Server
export TFM_SERVER_PORT=8080
export TFM_SERVER_BEHIND_PROXY=true

# Storage (AWS S3)
export TFM_STORAGE_TYPE=s3
export TFM_STORAGE_BUCKET=terraform-mirror-prod
export TFM_STORAGE_REGION=us-west-2
export TFM_STORAGE_ACCESS_KEY=AKIAIOSFODNN7EXAMPLE
export TFM_STORAGE_SECRET_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY

# Database
export TFM_DATABASE_PATH=/data/terraform-mirror.db
export TFM_DATABASE_BACKUP_ENABLED=true

# Cache
export TFM_CACHE_MEMORY_SIZE_MB=512
export TFM_CACHE_DISK_PATH=/var/cache/tf-mirror
export TFM_CACHE_DISK_SIZE_GB=50
export TFM_CACHE_TTL_SECONDS=7200

# Auth
export TFM_ADMIN_USERNAME=admin
export TFM_ADMIN_PASSWORD=securepassword123

# Logging
export TFM_LOGGING_LEVEL=info
export TFM_LOGGING_FORMAT=json
```

### Docker Compose Environment

```yaml
services:
  terraform-mirror:
    environment:
      - TFM_SERVER_PORT=8080
      - TFM_STORAGE_TYPE=s3
      - TFM_STORAGE_BUCKET=terraform-mirror
      - TFM_STORAGE_ENDPOINT=http://minio:9000
      - TFM_STORAGE_ACCESS_KEY=minioadmin
      - TFM_STORAGE_SECRET_KEY=minioadmin
      - TFM_STORAGE_FORCE_PATH_STYLE=true
      - TFM_DATABASE_PATH=/data/terraform-mirror.db
      - TFM_CACHE_MEMORY_SIZE_MB=256
      - TFM_CACHE_DISK_SIZE_GB=10
      - TFM_CACHE_DISK_PATH=/data/cache
      - TFM_CACHE_TTL_SECONDS=3600
      - TFM_ADMIN_USERNAME=admin
      - TFM_ADMIN_PASSWORD=changeme123
```

---

## Environment Variable Reference

Quick reference for all environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| **Server** | | |
| `TFM_SERVER_PORT` | `8080` | HTTP port |
| `TFM_SERVER_TLS_ENABLED` | `false` | Enable HTTPS |
| `TFM_SERVER_TLS_CERT_PATH` | - | TLS certificate path |
| `TFM_SERVER_TLS_KEY_PATH` | - | TLS key path |
| `TFM_SERVER_BEHIND_PROXY` | `false` | Behind reverse proxy |
| **Storage** | | |
| `TFM_STORAGE_TYPE` | `s3` | Storage type: `s3`, `local` |
| `TFM_STORAGE_BUCKET` | `terraform-mirror` | S3 bucket name |
| `TFM_STORAGE_REGION` | `us-east-1` | AWS region |
| `TFM_STORAGE_ENDPOINT` | - | Custom S3 endpoint |
| `TFM_STORAGE_ACCESS_KEY` | - | S3 access key |
| `TFM_STORAGE_SECRET_KEY` | - | S3 secret key |
| `TFM_STORAGE_FORCE_PATH_STYLE` | `false` | Path-style URLs |
| **Database** | | |
| `TFM_DATABASE_PATH` | `/data/terraform-mirror.db` | Database file path |
| `TFM_DATABASE_BACKUP_ENABLED` | `false` | Enable backups |
| **Cache** | | |
| `TFM_CACHE_MEMORY_SIZE_MB` | `256` | Memory cache size |
| `TFM_CACHE_DISK_PATH` | `/var/cache/tf-mirror` | Disk cache path |
| `TFM_CACHE_DISK_SIZE_GB` | `10` | Disk cache size |
| `TFM_CACHE_TTL_SECONDS` | `3600` | Cache TTL |
| **Auth** | | |
| `TFM_ADMIN_USERNAME` | - | Initial admin user |
| `TFM_ADMIN_PASSWORD` | - | Initial admin password |
| `TFM_AUTH_JWT_EXPIRATION_HOURS` | `8` | JWT validity |
| `TFM_AUTH_BCRYPT_COST` | `12` | Password hash cost |
| **Logging** | | |
| `TFM_LOGGING_LEVEL` | `info` | Log level |
| `TFM_LOGGING_FORMAT` | `text` | Log format |
| `TFM_LOGGING_OUTPUT` | `stdout` | Log output |
| `TFM_LOGGING_FILE_PATH` | - | Log file path |
| **Telemetry** | | |
| `TFM_TELEMETRY_ENABLED` | `false` | Enable telemetry |
| `TFM_TELEMETRY_OTEL_ENABLED` | `false` | Enable OpenTelemetry |
| `TFM_TELEMETRY_OTEL_ENDPOINT` | - | OTEL endpoint |
| **Providers** | | |
| `TFM_PROVIDERS_GPG_VERIFICATION_ENABLED` | `true` | GPG verification |
| `TFM_PROVIDERS_GPG_KEY_URL` | HashiCorp URL | GPG key URL |
| **Quota** | | |
| `TFM_QUOTA_ENABLED` | `false` | Enable quotas |
| `TFM_QUOTA_MAX_STORAGE_GB` | `0` | Max storage |
| **Features** | | |
| `TFM_FEATURES_AUTO_DOWNLOAD_PROVIDERS` | `false` | Auto-download providers |
| `TFM_FEATURES_AUTO_DOWNLOAD_MODULES` | `false` | Auto-download modules |
