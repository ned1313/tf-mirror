# API Documentation

Terraform Mirror provides two categories of APIs:

1. **Terraform Provider Network Mirror Protocol** - Public endpoints for Terraform clients
2. **Admin REST API** - Authenticated endpoints for administration

## Table of Contents

- [Authentication](#authentication)
- [Error Handling](#error-handling)
- [Provider Mirror Protocol](#provider-mirror-protocol)
- [Admin API](#admin-api)
  - [Authentication Endpoints](#authentication-endpoints)
  - [Provider Management](#provider-management)
  - [Job Management](#job-management)
  - [Statistics & Monitoring](#statistics--monitoring)
  - [System Administration](#system-administration)

---

## Authentication

### Admin API Authentication

The Admin API uses JWT (JSON Web Tokens) for authentication.

**Obtaining a Token:**

```bash
curl -X POST http://localhost:8080/admin/api/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "your-password"}'
```

**Using the Token:**

Include the token in the `Authorization` header for all admin API requests:

```bash
curl http://localhost:8080/admin/api/providers \
  -H "Authorization: Bearer <your-jwt-token>"
```

**Token Expiration:**

Tokens expire after the configured period (default: 8 hours). After expiration, obtain a new token via the login endpoint.

### Provider Mirror Protocol Authentication

The Terraform Provider Network Mirror Protocol endpoints are **public** and do not require authentication. This allows Terraform clients to access providers without additional configuration.

---

## Error Handling

All API errors return a consistent JSON format:

```json
{
  "error": "error_code",
  "message": "Human-readable error description"
}
```

### Common HTTP Status Codes

| Status | Description |
|--------|-------------|
| `200` | Success |
| `400` | Bad Request - Invalid input |
| `401` | Unauthorized - Invalid or missing token |
| `404` | Not Found - Resource doesn't exist |
| `500` | Internal Server Error |

### Common Error Codes

| Code | Description |
|------|-------------|
| `invalid_request` | Malformed request body |
| `missing_credentials` | Username or password missing |
| `invalid_credentials` | Wrong username or password |
| `invalid_token` | JWT token is invalid or expired |
| `session_revoked` | Session has been logged out |
| `not_found` | Requested resource not found |
| `database_error` | Database operation failed |

---

## Provider Mirror Protocol

These endpoints implement the [Terraform Provider Network Mirror Protocol](https://developer.hashicorp.com/terraform/internals/provider-network-mirror-protocol).

### Service Discovery

Terraform clients first query this endpoint to discover available services.

**Endpoint:** `GET /.well-known/terraform.json`

**Response:**

```json
{
  "providers.v1": "/v1/providers/"
}
```

**Example:**

```bash
curl http://localhost:8080/.well-known/terraform.json
```

---

### List Provider Versions

Returns available versions for a provider.

**Endpoint:** `GET /v1/providers/{namespace}/{type}/versions`

**Path Parameters:**

| Parameter | Description | Example |
|-----------|-------------|---------|
| `namespace` | Provider namespace | `hashicorp` |
| `type` | Provider type | `aws` |

**Response:**

```json
{
  "versions": {
    "5.31.0": {},
    "5.30.0": {},
    "5.29.0": {}
  }
}
```

**Response Headers:**

| Header | Description |
|--------|-------------|
| `X-Cache` | `HIT` if served from cache, `MISS` otherwise |

**Example:**

```bash
curl http://localhost:8080/v1/providers/hashicorp/aws/versions
```

---

### Get Provider Download Info

Returns download information for a specific provider version and platform.

**Endpoint:** `GET /v1/providers/{namespace}/{type}/{version}/download/{os}/{arch}`

**Path Parameters:**

| Parameter | Description | Example |
|-----------|-------------|---------|
| `namespace` | Provider namespace | `hashicorp` |
| `type` | Provider type | `aws` |
| `version` | Provider version | `5.31.0` |
| `os` | Operating system | `linux`, `darwin`, `windows` |
| `arch` | Architecture | `amd64`, `arm64` |

**Response:**

```json
{
  "protocols": ["5.0"],
  "os": "linux",
  "arch": "amd64",
  "filename": "terraform-provider-aws_5.31.0_linux_amd64.zip",
  "download_url": "https://storage.example.com/providers/...",
  "shasum_url": "https://storage.example.com/providers/..._SHA256SUMS",
  "shasum_signature_url": "https://storage.example.com/providers/..._SHA256SUMS.sig",
  "shasum": "abc123...",
  "signing_keys": {
    "gpg_public_keys": []
  }
}
```

**Example:**

```bash
curl http://localhost:8080/v1/providers/hashicorp/aws/5.31.0/download/linux/amd64
```

---

### Health Check

Returns server health status.

**Endpoint:** `GET /health`

**Response:**

```json
{
  "status": "healthy",
  "version": "0.1.0"
}
```

---

## Admin API

All Admin API endpoints require authentication (except login/logout).

**Base URL:** `/admin/api`

---

## Authentication Endpoints

### Login

Authenticate and obtain a JWT token.

**Endpoint:** `POST /admin/api/login`

**Request Body:**

```json
{
  "username": "admin",
  "password": "your-password"
}
```

**Response:**

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_at": "2025-12-04T08:00:00Z",
  "user": {
    "id": 1,
    "username": "admin"
  }
}
```

**Example:**

```bash
curl -X POST http://localhost:8080/admin/api/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "changeme123"}'
```

---

### Logout

Revoke the current session.

**Endpoint:** `POST /admin/api/logout`

**Headers:** `Authorization: Bearer <token>`

**Response:**

```json
{
  "message": "Logged out successfully"
}
```

**Example:**

```bash
curl -X POST http://localhost:8080/admin/api/logout \
  -H "Authorization: Bearer $TOKEN"
```

---

## Provider Management

### Load Providers from HCL

Upload an HCL file to load provider definitions and trigger downloads.

**Endpoint:** `POST /admin/api/providers/load`

**Content-Type:** `multipart/form-data`

**Form Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `file` | file | HCL provider definition file |

**HCL File Format:**

```hcl
provider "hashicorp/aws" {
  versions  = ["5.31.0", "5.30.0"]
  platforms = ["linux_amd64", "darwin_arm64"]
}

provider "hashicorp/azurerm" {
  versions  = ["3.84.0"]
  platforms = ["linux_amd64"]
}
```

**Response:**

```json
{
  "job_id": 1,
  "message": "Provider loading job created and completed: 2 total providers",
  "total_providers": 2
}
```

**Example:**

```bash
curl -X POST http://localhost:8080/admin/api/providers/load \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@providers.hcl"
```

---

### List Providers

List all providers with optional filtering.

**Endpoint:** `GET /admin/api/providers`

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `namespace` | string | Filter by namespace |
| `type` | string | Filter by provider type |

**Response:**

```json
{
  "providers": [
    {
      "id": 1,
      "namespace": "hashicorp",
      "type": "aws",
      "version": "5.31.0",
      "platform": "linux_amd64",
      "protocols": ["5.0"],
      "filename": "terraform-provider-aws_5.31.0_linux_amd64.zip",
      "s3_key": "providers/registry.terraform.io/hashicorp/aws/5.31.0/terraform-provider-aws_5.31.0_linux_amd64.zip",
      "sha256sum": "abc123...",
      "size_bytes": 94371840,
      "deprecated": false,
      "blocked": false,
      "created_at": "2025-12-03T10:00:00Z",
      "updated_at": "2025-12-03T10:00:00Z"
    }
  ],
  "count": 1
}
```

**Example:**

```bash
# List all providers
curl http://localhost:8080/admin/api/providers \
  -H "Authorization: Bearer $TOKEN"

# Filter by namespace
curl "http://localhost:8080/admin/api/providers?namespace=hashicorp" \
  -H "Authorization: Bearer $TOKEN"
```

---

### Get Provider

Get a specific provider by ID.

**Endpoint:** `GET /admin/api/providers/{id}`

**Response:**

```json
{
  "id": 1,
  "namespace": "hashicorp",
  "type": "aws",
  "version": "5.31.0",
  "platform": "linux_amd64",
  "protocols": ["5.0"],
  "filename": "terraform-provider-aws_5.31.0_linux_amd64.zip",
  "s3_key": "providers/...",
  "sha256sum": "abc123...",
  "size_bytes": 94371840,
  "deprecated": false,
  "blocked": false,
  "created_at": "2025-12-03T10:00:00Z",
  "updated_at": "2025-12-03T10:00:00Z"
}
```

**Example:**

```bash
curl http://localhost:8080/admin/api/providers/1 \
  -H "Authorization: Bearer $TOKEN"
```

---

### Update Provider

Update provider metadata (deprecation/blocked status).

**Endpoint:** `PUT /admin/api/providers/{id}`

**Request Body:**

```json
{
  "deprecated": true,
  "blocked": false
}
```

**Response:** Returns the updated provider object.

**Example:**

```bash
# Mark provider as deprecated
curl -X PUT http://localhost:8080/admin/api/providers/1 \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"deprecated": true}'
```

---

### Delete Provider

Delete a provider and its storage object.

**Endpoint:** `DELETE /admin/api/providers/{id}`

**Response:**

```json
{
  "message": "Provider deleted successfully"
}
```

**Example:**

```bash
curl -X DELETE http://localhost:8080/admin/api/providers/1 \
  -H "Authorization: Bearer $TOKEN"
```

---

## Job Management

### List Jobs

List download jobs with pagination.

**Endpoint:** `GET /admin/api/jobs`

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | 10 | Items per page (max 100) |
| `offset` | int | 0 | Pagination offset |

**Response:**

```json
{
  "jobs": [
    {
      "id": 1,
      "source_type": "hcl",
      "status": "completed",
      "progress": 100,
      "total_items": 4,
      "completed_items": 4,
      "failed_items": 0,
      "created_at": "2025-12-03T10:00:00Z",
      "started_at": "2025-12-03T10:00:01Z",
      "completed_at": "2025-12-03T10:05:00Z"
    }
  ],
  "total": 1,
  "limit": 10,
  "offset": 0
}
```

**Job Statuses:**

| Status | Description |
|--------|-------------|
| `pending` | Job created, waiting to start |
| `running` | Job is actively processing |
| `completed` | Job finished successfully |
| `failed` | Job failed with errors |
| `cancelled` | Job was cancelled by user |

**Example:**

```bash
curl "http://localhost:8080/admin/api/jobs?limit=20" \
  -H "Authorization: Bearer $TOKEN"
```

---

### Get Job Details

Get detailed information about a specific job, including all items.

**Endpoint:** `GET /admin/api/jobs/{id}`

**Response:**

```json
{
  "id": 1,
  "source_type": "hcl",
  "status": "completed",
  "progress": 100,
  "total_items": 4,
  "completed_items": 3,
  "failed_items": 1,
  "created_at": "2025-12-03T10:00:00Z",
  "started_at": "2025-12-03T10:00:01Z",
  "completed_at": "2025-12-03T10:05:00Z",
  "items": [
    {
      "id": 1,
      "namespace": "hashicorp",
      "type": "aws",
      "version": "5.31.0",
      "platform": "linux_amd64",
      "status": "completed"
    },
    {
      "id": 2,
      "namespace": "hashicorp",
      "type": "aws",
      "version": "5.31.0",
      "platform": "darwin_arm64",
      "status": "failed",
      "error_message": "Download failed: connection timeout"
    }
  ]
}
```

**Example:**

```bash
curl http://localhost:8080/admin/api/jobs/1 \
  -H "Authorization: Bearer $TOKEN"
```

---

### Retry Job

Retry failed items in a completed or failed job.

**Endpoint:** `POST /admin/api/jobs/{id}/retry`

**Response:**

```json
{
  "message": "Job retry started",
  "reset_count": 2,
  "job_id": 1
}
```

**Example:**

```bash
curl -X POST http://localhost:8080/admin/api/jobs/1/retry \
  -H "Authorization: Bearer $TOKEN"
```

---

### Cancel Job

Cancel a pending or running job.

**Endpoint:** `POST /admin/api/jobs/{id}/cancel`

**Response:**

```json
{
  "message": "Job cancelled",
  "job_id": 1,
  "was_active": true
}
```

**Example:**

```bash
curl -X POST http://localhost:8080/admin/api/jobs/1/cancel \
  -H "Authorization: Bearer $TOKEN"
```

---

## Statistics & Monitoring

### Storage Statistics

Get storage usage statistics.

**Endpoint:** `GET /admin/api/stats/storage`

**Response:**

```json
{
  "total_providers": 150,
  "total_size_bytes": 15728640000,
  "total_size_human": "14.65 GB",
  "unique_namespaces": 5,
  "unique_types": 25,
  "unique_versions": 75,
  "deprecated_count": 3,
  "blocked_count": 1
}
```

**Example:**

```bash
curl http://localhost:8080/admin/api/stats/storage \
  -H "Authorization: Bearer $TOKEN"
```

---

### Cache Statistics

Get cache usage and efficiency statistics.

**Endpoint:** `GET /admin/api/stats/cache`

**Response:**

```json
{
  "enabled": true,
  "hits": 1500,
  "misses": 200,
  "hit_rate": 88.24,
  "hit_rate_str": "88.24%",
  "size": 134217728,
  "size_human": "128.00 MB",
  "max_size": 268435456,
  "max_size_human": "256.00 MB",
  "usage_percent": 50.0,
  "item_count": 45,
  "evictions": 12,
  "expirations": 5,
  "efficiency": {
    "total_requests": 1700,
    "bytes_saved": 4473495552,
    "bytes_saved_human": "4.17 GB",
    "eviction_rate": 19.35,
    "eviction_rate_str": "19.35%",
    "average_item_size": 2982764,
    "average_item_size_human": "2.84 MB"
  },
  "config": {
    "memory_size_mb": 256,
    "disk_size_gb": 10,
    "disk_path": "/var/cache/tf-mirror",
    "ttl_seconds": 3600
  },
  "tiered": {
    "memory_hits": 1200,
    "memory_misses": 500,
    "memory_hit_rate": 70.59,
    "memory_hit_rate_str": "70.59%",
    "memory_size": 67108864,
    "memory_size_human": "64.00 MB",
    "memory_max_size": 268435456,
    "memory_max_size_human": "256.00 MB",
    "memory_usage_percent": 25.0,
    "memory_item_count": 20,
    "memory_evictions": 8,
    "memory_expirations": 3,
    "disk_hits": 300,
    "disk_misses": 200,
    "disk_hit_rate": 60.0,
    "disk_hit_rate_str": "60.00%",
    "disk_size": 5368709120,
    "disk_size_human": "5.00 GB",
    "disk_max_size": 10737418240,
    "disk_max_size_human": "10.00 GB",
    "disk_usage_percent": 50.0,
    "disk_item_count": 25,
    "disk_evictions": 4,
    "disk_expirations": 2,
    "total_hits": 1500,
    "total_misses": 200,
    "promotions": 150
  }
}
```

**Example:**

```bash
curl http://localhost:8080/admin/api/stats/cache \
  -H "Authorization: Bearer $TOKEN"
```

---

### Clear Cache

Clear all items from the cache.

**Endpoint:** `POST /admin/api/stats/cache/clear`

**Response:**

```json
{
  "message": "Cache cleared successfully",
  "items_cleared": 45
}
```

**Example:**

```bash
curl -X POST http://localhost:8080/admin/api/stats/cache/clear \
  -H "Authorization: Bearer $TOKEN"
```

---

### Audit Logs

Get audit logs with optional filtering.

**Endpoint:** `GET /admin/api/stats/audit`

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | 50 | Items per page (max 500) |
| `offset` | int | 0 | Pagination offset |
| `action` | string | - | Filter by action type |
| `resource_type` | string | - | Filter by resource type |
| `resource_id` | string | - | Filter by resource ID |

**Response:**

```json
{
  "logs": [
    {
      "id": 1,
      "user_id": 1,
      "action": "login",
      "resource_type": "session",
      "resource_id": "abc-123",
      "ip_address": "192.168.1.100",
      "success": true,
      "created_at": "2025-12-03T10:00:00Z"
    },
    {
      "id": 2,
      "user_id": 1,
      "action": "load_providers",
      "resource_type": "job",
      "resource_id": "1",
      "ip_address": "192.168.1.100",
      "success": true,
      "created_at": "2025-12-03T10:05:00Z"
    }
  ],
  "total": 2,
  "limit": 50,
  "offset": 0
}
```

**Action Types:**

| Action | Description |
|--------|-------------|
| `login` | User login |
| `logout` | User logout |
| `load_providers` | Provider loading job |
| `update_provider` | Provider metadata update |
| `delete_provider` | Provider deletion |
| `retry_job` | Job retry |
| `cancel_job` | Job cancellation |
| `clear_cache` | Cache cleared |
| `trigger_backup` | Manual backup |

**Example:**

```bash
# Get recent logs
curl http://localhost:8080/admin/api/stats/audit \
  -H "Authorization: Bearer $TOKEN"

# Filter by action
curl "http://localhost:8080/admin/api/stats/audit?action=login" \
  -H "Authorization: Bearer $TOKEN"
```

---

### Recalculate Storage Statistics

Recalculate storage sizes from actual files.

**Endpoint:** `POST /admin/api/stats/recalculate`

**Response:**

```json
{
  "message": "Recalculated storage sizes for 150 providers",
  "updated": 5,
  "errors": 0,
  "new_total_bytes": 15728640000
}
```

**Example:**

```bash
curl -X POST http://localhost:8080/admin/api/stats/recalculate \
  -H "Authorization: Bearer $TOKEN"
```

---

## System Administration

### Get Configuration

Get current configuration (secrets redacted).

**Endpoint:** `GET /admin/api/config`

**Response:**

```json
{
  "server": {
    "port": 8080,
    "tls_enabled": false,
    "behind_proxy": false
  },
  "storage": {
    "type": "s3",
    "bucket": "terraform-mirror",
    "region": "us-east-1",
    "endpoint": "http://minio:9000",
    "force_path_style": true
  },
  "database": {
    "path": "/data/terraform-mirror.db",
    "backup_enabled": true,
    "backup_interval_hours": 24,
    "backup_to_s3": true
  },
  "cache": {
    "memory_size_mb": 256,
    "disk_path": "/var/cache/tf-mirror",
    "disk_size_gb": 10,
    "ttl_seconds": 3600
  },
  "features": {
    "auto_download_providers": false,
    "auto_download_modules": false,
    "max_download_size_mb": 500
  },
  "processor": {
    "polling_interval_seconds": 10,
    "max_concurrent_jobs": 3,
    "retry_attempts": 3,
    "retry_delay_seconds": 5
  },
  "logging": {
    "level": "info",
    "format": "text",
    "output": "stdout"
  },
  "telemetry": {
    "enabled": false,
    "otel_enabled": false,
    "export_traces": false,
    "export_metrics": false
  }
}
```

**Example:**

```bash
curl http://localhost:8080/admin/api/config \
  -H "Authorization: Bearer $TOKEN"
```

---

### Processor Status

Get background processor status.

**Endpoint:** `GET /admin/api/processor/status`

**Response:**

```json
{
  "running": true,
  "active_jobs": 2,
  "jobs_processed": 150,
  "jobs_failed": 5,
  "last_poll_at": "2025-12-03T10:00:00Z"
}
```

**Example:**

```bash
curl http://localhost:8080/admin/api/processor/status \
  -H "Authorization: Bearer $TOKEN"
```

---

### Trigger Backup

Manually trigger a database backup.

**Endpoint:** `POST /admin/api/backup`

**Response:**

```json
{
  "message": "Backup created and uploaded to S3 successfully",
  "backup_path": "/data/backups/terraform-mirror-backup-20251203-100000.db",
  "s3_key": "backups/terraform-mirror-backup-20251203-100000.db",
  "size_bytes": 1048576,
  "created_at": "2025-12-03T10:00:00Z"
}
```

**Example:**

```bash
curl -X POST http://localhost:8080/admin/api/backup \
  -H "Authorization: Bearer $TOKEN"
```

---

## Shell Script Examples

### Complete Workflow Example

```bash
#!/bin/bash
# Example: Load providers and monitor progress

# Configuration
MIRROR_URL="http://localhost:8080"
USERNAME="admin"
PASSWORD="changeme123"

# Login and get token
TOKEN=$(curl -s -X POST "$MIRROR_URL/admin/api/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\": \"$USERNAME\", \"password\": \"$PASSWORD\"}" \
  | jq -r '.token')

if [ "$TOKEN" == "null" ] || [ -z "$TOKEN" ]; then
  echo "Login failed"
  exit 1
fi

echo "Logged in successfully"

# Create provider definition
cat > /tmp/providers.hcl << 'EOF'
provider "hashicorp/aws" {
  versions  = ["5.31.0"]
  platforms = ["linux_amd64", "darwin_arm64"]
}
EOF

# Upload providers
echo "Loading providers..."
RESULT=$(curl -s -X POST "$MIRROR_URL/admin/api/providers/load" \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@/tmp/providers.hcl")

JOB_ID=$(echo "$RESULT" | jq -r '.job_id')
echo "Job created: $JOB_ID"

# Poll job status
while true; do
  STATUS=$(curl -s "$MIRROR_URL/admin/api/jobs/$JOB_ID" \
    -H "Authorization: Bearer $TOKEN")
  
  JOB_STATUS=$(echo "$STATUS" | jq -r '.status')
  PROGRESS=$(echo "$STATUS" | jq -r '.progress')
  
  echo "Status: $JOB_STATUS, Progress: $PROGRESS%"
  
  if [ "$JOB_STATUS" == "completed" ] || [ "$JOB_STATUS" == "failed" ]; then
    break
  fi
  
  sleep 5
done

# Show final result
echo "Final status:"
echo "$STATUS" | jq .

# Logout
curl -s -X POST "$MIRROR_URL/admin/api/logout" \
  -H "Authorization: Bearer $TOKEN"

echo "Done"
```

### Test Terraform Integration

```bash
#!/bin/bash
# Test that Terraform can use the mirror

# Create test directory
mkdir -p /tmp/tf-mirror-test
cd /tmp/tf-mirror-test

# Create Terraform configuration
cat > main.tf << 'EOF'
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "5.31.0"
    }
  }
}
EOF

# Create CLI config
cat > .terraformrc << 'EOF'
provider_installation {
  network_mirror {
    url = "http://localhost:8080/"
  }
}
EOF

# Run terraform init with custom config
export TF_CLI_CONFIG_FILE=.terraformrc
terraform init

# Check result
if [ $? -eq 0 ]; then
  echo "SUCCESS: Terraform initialized using mirror"
else
  echo "FAILED: Terraform init failed"
fi

# Cleanup
rm -rf /tmp/tf-mirror-test
```
