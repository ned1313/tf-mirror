# Terraform Mirror - Technical Design Document

## 1. Executive Summary

Terraform Mirror is a caching proxy server that provides network mirror capabilities for Terraform providers and modules. It is designed for air-gapped and low-bandwidth environments, implementing both the Provider Network Mirror Protocol and Module Registry Protocol as specified by HashiCorp.

**Phase 1 MVP Goals:**
- Single container deployment with Go backend
- S3-compatible object storage (AWS S3 or MinIO)
- Provider network mirror protocol (read-only with manual loading)
- Admin authentication and basic web UI
- SQLite metadata database

## 2. System Architecture

### 2.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Terraform Mirror Container              │
│                                                             │
│  ┌──────────────┐         ┌─────────────────────┐         │
│  │   Vue 3 UI   │────────▶│   Go HTTP Server    │         │
│  │  (Vite/TS)   │         │    (chi router)     │         │
│  └──────────────┘         └──────────┬──────────┘         │
│                                      │                      │
│                           ┌──────────┴──────────┐          │
│                           │                     │          │
│                    ┌──────▼──────┐      ┌──────▼──────┐   │
│                    │   SQLite    │      │   Cache     │   │
│                    │   Metadata  │      │ (Memory+Disk)│   │
│                    └─────────────┘      └──────┬──────┘   │
│                                                 │          │
└─────────────────────────────────────────────────┼──────────┘
                                                  │
                                          ┌───────▼────────┐
                                          │  S3 Storage    │
                                          │ (AWS/MinIO)    │
                                          └────────────────┘
```

### 2.2 Component Breakdown

#### 2.2.1 HTTP Server (Go)
- **Router**: chi v5 for lightweight, fast routing
- **Endpoints**:
  - `/.well-known/terraform.json` - Service discovery
  - `/v1/providers/{namespace}/{type}/{version}/download/{os}/{arch}` - Provider downloads
  - `/v1/providers/{namespace}/{type}/versions` - Provider version listing
  - `/admin/api/*` - Admin API endpoints (authenticated)
  - `/health` - Health check endpoint
  - `/metrics` - OpenTelemetry metrics endpoint
- **Middleware**:
  - Request logging with request IDs
  - Authentication (JWT for admin routes)
  - CORS handling
  - Panic recovery

#### 2.2.2 Storage Layer
- **Primary**: S3-compatible object storage
  - Provider path: `providers/{hostname}/{namespace}/{type}/{version}/{os}_{arch}/{filename}`
  - Example: `providers/registry.terraform.io/hashicorp/aws/5.31.0/linux_amd64/terraform-provider-aws_v5.31.0_linux_amd64.zip`
- **Metadata**: SQLite database (`/data/terraform-mirror.db`)
- **Cache**: Two-tier cache
  - L1: In-memory LRU cache (configurable size, default 256MB)
  - L2: Local disk cache (`/var/cache/tf-mirror`, configurable size, default 10GB)

#### 2.2.3 Database Schema (SQLite)

```sql
-- Providers table
CREATE TABLE providers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    hostname TEXT NOT NULL,
    namespace TEXT NOT NULL,
    type TEXT NOT NULL,
    version TEXT NOT NULL,
    architecture TEXT NOT NULL,
    os TEXT NOT NULL,
    s3_key TEXT NOT NULL,
    filename TEXT NOT NULL,
    checksum TEXT NOT NULL,
    checksum_type TEXT NOT NULL DEFAULT 'sha256',
    gpg_verified BOOLEAN NOT NULL DEFAULT 0,
    deprecated BOOLEAN NOT NULL DEFAULT 0,
    blocked BOOLEAN NOT NULL DEFAULT 0,
    file_size INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(hostname, namespace, type, version, architecture, os)
);

CREATE INDEX idx_providers_lookup ON providers(hostname, namespace, type, version);
CREATE INDEX idx_providers_deprecated ON providers(deprecated);
CREATE INDEX idx_providers_blocked ON providers(blocked);

-- Admin users table
CREATE TABLE admin_users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Admin sessions table (for audit and revocation)
CREATE TABLE admin_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    token_jti TEXT NOT NULL UNIQUE,
    ip_address TEXT,
    user_agent TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL,
    revoked BOOLEAN NOT NULL DEFAULT 0,
    FOREIGN KEY (user_id) REFERENCES admin_users(id)
);

CREATE INDEX idx_sessions_token ON admin_sessions(token_jti);
CREATE INDEX idx_sessions_user ON admin_sessions(user_id);

-- Admin actions audit log
CREATE TABLE admin_actions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    action TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT,
    details TEXT,
    ip_address TEXT,
    timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES admin_users(id)
);

CREATE INDEX idx_actions_user ON admin_actions(user_id);
CREATE INDEX idx_actions_timestamp ON admin_actions(timestamp);
CREATE INDEX idx_actions_resource ON admin_actions(resource_type, resource_id);

-- Download jobs table
CREATE TABLE download_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_type TEXT NOT NULL, -- 'provider' or 'module'
    status TEXT NOT NULL, -- 'pending', 'in_progress', 'completed', 'failed'
    total_items INTEGER NOT NULL DEFAULT 0,
    completed_items INTEGER NOT NULL DEFAULT 0,
    failed_items INTEGER NOT NULL DEFAULT 0,
    error_message TEXT,
    created_by INTEGER,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at DATETIME,
    completed_at DATETIME,
    FOREIGN KEY (created_by) REFERENCES admin_users(id)
);

CREATE INDEX idx_jobs_status ON download_jobs(status);
CREATE INDEX idx_jobs_created ON download_jobs(created_at);

-- Download job items table
CREATE TABLE download_job_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id INTEGER NOT NULL,
    item_type TEXT NOT NULL,
    identifier TEXT NOT NULL, -- e.g., "hashicorp/aws:5.31.0:linux_amd64"
    status TEXT NOT NULL, -- 'pending', 'downloading', 'completed', 'failed'
    retry_count INTEGER NOT NULL DEFAULT 0,
    error_message TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME,
    FOREIGN KEY (job_id) REFERENCES download_jobs(id)
);

CREATE INDEX idx_job_items_job ON download_job_items(job_id);
CREATE INDEX idx_job_items_status ON download_job_items(status);
```

#### 2.2.4 Configuration (HCL)

Default configuration file: `/etc/tf-mirror/config.hcl`

```hcl
server {
  port = 8080
  tls_enabled = false
  tls_cert_path = "/etc/tf-mirror/cert.pem"
  tls_key_path = "/etc/tf-mirror/key.pem"
  
  # Support for reverse proxy
  behind_proxy = false
  trusted_proxies = ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"]
}

storage {
  type = "s3"
  bucket = "terraform-mirror"
  region = "us-east-1"
  endpoint = "" # For MinIO or custom S3 endpoint
  
  # Leave empty to use IAM roles
  access_key = ""
  secret_key = ""
  
  # Force path style for MinIO compatibility
  force_path_style = false
}

database {
  path = "/data/terraform-mirror.db"
  
  # Backup configuration
  backup_enabled = true
  backup_interval_hours = 24
  backup_to_s3 = true
  backup_s3_prefix = "backups/"
}

cache {
  memory_size_mb = 256
  disk_path = "/var/cache/tf-mirror"
  disk_size_gb = 10
  ttl_seconds = 3600
}

features {
  auto_download_providers = false # Phase 2
  auto_download_modules = false # Phase 3
  max_download_size_mb = 500
}

auth {
  jwt_expiration_hours = 8
  bcrypt_cost = 12
}

logging {
  level = "info" # debug, info, warn, error
  format = "text" # text or json
  output = "stdout" # stdout, stderr, file, both
  file_path = "/var/log/tf-mirror/app.log"
}

telemetry {
  enabled = true
  otel_enabled = false
  otel_endpoint = "localhost:4317"
  otel_protocol = "grpc" # grpc or http
  export_traces = true
  export_metrics = true
}

providers {
  gpg_verification_enabled = true
  gpg_key_url = "https://www.hashicorp.com/.well-known/pgp-key.txt"
  
  # Retry configuration
  download_retry_attempts = 5
  download_retry_initial_delay_ms = 1000
  download_timeout_seconds = 60
}

# Storage quota management
quota {
  enabled = false
  max_storage_gb = 0 # 0 = unlimited
  warning_threshold_percent = 80
}
```

Environment variable overrides (take precedence):
- `TFM_SERVER_PORT`
- `TFM_STORAGE_BUCKET`
- `TFM_STORAGE_ACCESS_KEY`
- `TFM_STORAGE_SECRET_KEY`
- `TFM_ADMIN_USERNAME` (initial setup only)
- `TFM_ADMIN_PASSWORD` (initial setup only, hashed on first run)

### 2.3 Provider Network Mirror Protocol Implementation

#### 2.3.1 Service Discovery Endpoint

**GET** `/.well-known/terraform.json`

Response:
```json
{
  "providers.v1": "/v1/providers/"
}
```

#### 2.3.2 List Available Versions

**GET** `/v1/providers/{namespace}/{type}/versions`

Response:
```json
{
  "versions": [
    {
      "version": "5.31.0",
      "protocols": ["5.0"],
      "platforms": [
        {
          "os": "linux",
          "arch": "amd64"
        },
        {
          "os": "darwin",
          "arch": "arm64"
        }
      ]
    }
  ]
}
```

#### 2.3.3 Download Provider Package

**GET** `/v1/providers/{namespace}/{type}/{version}/download/{os}/{arch}`

Response Headers:
- `X-Terraform-Get: {download_url}`
- `Content-Type: application/json`

Response Body:
```json
{
  "protocols": ["5.0"],
  "os": "linux",
  "arch": "amd64",
  "filename": "terraform-provider-aws_v5.31.0_linux_amd64.zip",
  "download_url": "{presigned_s3_url}",
  "shasums_url": "{presigned_s3_url_for_shasums}",
  "shasums_signature_url": "{presigned_s3_url_for_sig}",
  "shasum": "abc123...",
  "signing_keys": {
    "gpg_public_keys": [
      {
        "key_id": "...",
        "ascii_armor": "..."
      }
    ]
  }
}
```

### 2.4 Admin API Endpoints

All admin endpoints require JWT authentication via `Authorization: Bearer {token}` header.

#### 2.4.1 Authentication

**POST** `/admin/api/login`
```json
{
  "username": "admin",
  "password": "securepassword"
}
```

Response:
```json
{
  "token": "eyJhbGc...",
  "expires_at": "2025-11-21T08:00:00Z"
}
```

**POST** `/admin/api/logout`
Revokes current session token.

#### 2.4.2 Provider Management

**GET** `/admin/api/providers`
List all cached providers with filtering and pagination.

**POST** `/admin/api/providers/upload`
Upload provider definition HCL file to bulk load providers.

**DELETE** `/admin/api/providers/{id}`
Remove a specific provider version.

**PATCH** `/admin/api/providers/{id}`
Mark provider as deprecated or blocked.

#### 2.4.3 Job Management

**GET** `/admin/api/jobs`
List download jobs.

**GET** `/admin/api/jobs/{id}`
Get job details and progress.

**POST** `/admin/api/jobs/{id}/retry`
Retry failed job items.

#### 2.4.4 System Management

**GET** `/admin/api/storage`
Get storage usage statistics.

**GET** `/admin/api/audit`
Get audit log of admin actions.

**POST** `/admin/api/backup`
Trigger database backup.

**GET** `/admin/api/config`
Get current configuration (sensitive values masked).

**PUT** `/admin/api/config`
Update runtime configuration.

## 3. Project Structure

```
terraform-mirror/
├── cmd/
│   └── terraform-mirror/
│       └── main.go                 # Application entry point
├── internal/
│   ├── api/
│   │   ├── handlers/
│   │   │   ├── admin.go           # Admin API handlers
│   │   │   ├── auth.go            # Authentication handlers
│   │   │   ├── health.go          # Health check handler
│   │   │   ├── providers.go       # Provider mirror handlers
│   │   │   └── metrics.go         # Metrics/telemetry handlers
│   │   ├── middleware/
│   │   │   ├── auth.go            # JWT authentication middleware
│   │   │   ├── logging.go         # Request logging middleware
│   │   │   ├── recovery.go        # Panic recovery middleware
│   │   │   └── cors.go            # CORS middleware
│   │   └── router.go              # Main router setup
│   ├── cache/
│   │   ├── cache.go               # Two-tier cache interface
│   │   ├── memory.go              # In-memory LRU cache
│   │   └── disk.go                # Disk-based cache
│   ├── config/
│   │   ├── config.go              # Configuration structures
│   │   ├── loader.go              # HCL config loader
│   │   └── validator.go           # Config validation
│   ├── database/
│   │   ├── db.go                  # Database connection and setup
│   │   ├── migrations/
│   │   │   └── 001_initial.sql   # Initial schema
│   │   ├── models/
│   │   │   ├── provider.go
│   │   │   ├── user.go
│   │   │   ├── session.go
│   │   │   ├── action.go
│   │   │   └── job.go
│   │   └── repository/
│   │       ├── provider.go        # Provider data access
│   │       ├── user.go            # User data access
│   │       ├── session.go         # Session data access
│   │       ├── action.go          # Audit log data access
│   │       └── job.go             # Job data access
│   ├── storage/
│   │   ├── storage.go             # Storage interface
│   │   ├── s3.go                  # S3 implementation
│   │   └── local.go               # Local filesystem (for testing)
│   ├── provider/
│   │   ├── downloader.go          # Provider download logic
│   │   ├── verifier.go            # GPG signature verification
│   │   ├── parser.go              # HCL file parser
│   │   └── job.go                 # Job processing
│   ├── auth/
│   │   ├── jwt.go                 # JWT token generation/validation
│   │   ├── password.go            # Password hashing (bcrypt)
│   │   └── session.go             # Session management
│   ├── telemetry/
│   │   ├── logger.go              # Structured logging
│   │   ├── metrics.go             # Metrics collection
│   │   └── tracer.go              # OpenTelemetry tracing
│   └── version/
│       └── version.go             # Version information
├── web/
│   ├── public/
│   │   └── favicon.ico
│   ├── src/
│   │   ├── assets/
│   │   ├── components/
│   │   │   ├── common/
│   │   │   │   ├── Header.vue
│   │   │   │   ├── Sidebar.vue
│   │   │   │   └── Footer.vue
│   │   │   ├── providers/
│   │   │   │   ├── ProviderList.vue
│   │   │   │   ├── ProviderDetail.vue
│   │   │   │   └── ProviderSearch.vue
│   │   │   └── admin/
│   │   │       ├── Dashboard.vue
│   │   │       ├── JobList.vue
│   │   │       ├── JobDetail.vue
│   │   │       ├── StorageView.vue
│   │   │       ├── AuditLog.vue
│   │   │       └── ProviderUpload.vue
│   │   ├── router/
│   │   │   └── index.ts
│   │   ├── stores/
│   │   │   ├── auth.ts
│   │   │   ├── providers.ts
│   │   │   └── admin.ts
│   │   ├── types/
│   │   │   ├── provider.ts
│   │   │   ├── job.ts
│   │   │   └── api.ts
│   │   ├── utils/
│   │   │   ├── api.ts
│   │   │   └── format.ts
│   │   ├── views/
│   │   │   ├── Home.vue
│   │   │   ├── Login.vue
│   │   │   ├── Providers.vue
│   │   │   └── Admin.vue
│   │   ├── App.vue
│   │   └── main.ts
│   ├── index.html
│   ├── package.json
│   ├── tsconfig.json
│   ├── vite.config.ts
│   └── tailwind.config.js
├── deployments/
│   ├── docker/
│   │   └── Dockerfile
│   ├── kubernetes/
│   │   ├── deployment.yaml
│   │   ├── service.yaml
│   │   ├── configmap.yaml
│   │   ├── secret.yaml
│   │   └── pvc.yaml
│   ├── helm/
│   │   └── terraform-mirror/
│   │       ├── Chart.yaml
│   │       ├── values.yaml
│   │       └── templates/
│   └── docker-compose/
│       ├── docker-compose.yml
│       └── minio.yml
├── scripts/
│   ├── build.sh
│   ├── test.sh
│   └── dev.sh
├── test/
│   ├── integration/
│   │   └── provider_test.go
│   ├── e2e/
│   │   └── playwright/
│   └── fixtures/
│       ├── providers/
│       └── config/
├── docs/
│   ├── installation.md
│   ├── configuration.md
│   ├── api.md
│   └── user-guide.md
├── .gitignore
├── .dockerignore
├── go.mod
├── go.sum
├── Makefile
├── README.md
└── LICENSE
```

## 4. Data Flow

### 4.1 Provider Download Flow (Phase 1 - Manual Load)

```
1. Admin uploads provider definition HCL file
   └─> POST /admin/api/providers/upload
       └─> Parse HCL file
           └─> Create download job
               └─> For each provider/version/arch:
                   ├─> Download from registry.terraform.io
                   ├─> Verify GPG signature
                   ├─> Calculate checksums
                   ├─> Upload to S3
                   └─> Insert metadata into SQLite

2. Terraform client requests provider
   └─> GET /v1/providers/{namespace}/{type}/versions
       └─> Query SQLite for available versions
           └─> Return version list

3. Terraform client downloads provider
   └─> GET /v1/providers/{namespace}/{type}/{version}/download/{os}/{arch}
       ├─> Check L1 cache (memory)
       ├─> Check L2 cache (disk)
       ├─> Generate presigned S3 URL (if not cached)
       └─> Return download URL
```

### 4.2 Admin Authentication Flow

```
1. Admin login
   └─> POST /admin/api/login
       └─> Validate credentials (bcrypt)
           └─> Generate JWT token (8hr expiration)
               └─> Store session in database
                   └─> Return token

2. Admin API request
   └─> Include Authorization: Bearer {token}
       └─> Validate JWT signature
           └─> Check session not revoked
               └─> Extract user context
                   └─> Process request
                       └─> Log admin action
```

### 4.3 Job Processing Flow

```
1. Job creation
   └─> Parse provider definition file
       └─> Create job record (status: pending)
           └─> Create job items for each provider/version/arch
               └─> Return job ID

2. Job processing (background goroutine)
   └─> Poll for pending jobs
       └─> For each job item:
           ├─> Update status: downloading
           ├─> Download provider package
           ├─> On failure: retry with exponential backoff (max 5)
           ├─> On success: upload to S3, update metadata
           └─> Update job progress

3. Job status polling
   └─> GET /admin/api/jobs/{id}
       └─> Return job progress (completed/total)
```

## 5. Security Considerations

### 5.1 Authentication & Authorization
- JWT tokens with 8-hour expiration (configurable)
- Bcrypt password hashing (cost factor 12)
- Session tracking for audit and revocation
- Password change invalidates all sessions
- No authentication required for provider downloads (consumer access)

### 5.2 Provider Verification
- GPG signature verification for all providers
- Checksum validation on download
- Checksum re-validation on serve
- Block providers with failed verification

### 5.3 Network Security
- Optional TLS support with configurable certificates
- Support for reverse proxy deployment
- Trusted proxy configuration for X-Forwarded headers
- Presigned S3 URLs with short expiration (15 minutes)

### 5.4 Input Validation
- HCL file parsing with size limits
- Max file size for provider downloads (500MB default)
- Request size limits on all endpoints
- SQL injection prevention (prepared statements)

## 6. Performance Considerations

### 6.1 Caching Strategy
- Two-tier cache reduces S3 API calls
- LRU eviction policy for memory cache
- Configurable TTLs
- Cache warming for frequently accessed providers

### 6.2 Concurrent Downloads
- Mutex-based download deduplication
- First request initiates download, others wait
- 60-second timeout for waiting requests
- Parallel downloads for different providers

### 6.3 Database Performance
- Indexes on frequently queried columns
- Connection pooling
- Prepared statement caching
- Read-only connections for provider queries

## 7. Monitoring & Observability

### 7.1 Health Checks
**GET** `/health`

Response:
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "checks": {
    "database": "ok",
    "s3": "ok",
    "cache": "ok"
  },
  "timestamp": "2025-11-20T12:00:00Z"
}
```

### 7.2 Metrics
- Request count by endpoint
- Request latency (p50, p95, p99)
- Cache hit/miss ratio
- Active download count
- Storage usage
- Database connection pool stats

### 7.3 Logging
- Structured logging with request IDs
- Log levels: debug, info, warn, error
- Configurable output (stdout, file)
- Admin action audit trail

## 8. Error Handling

### 8.1 Provider Download Errors
- Network errors: Retry with exponential backoff (5 attempts)
- GPG verification failure: Block provider, notify admin
- Checksum mismatch: Block provider, notify admin
- S3 upload failure: Retry, fail job item after 5 attempts

### 8.2 Client-Facing Errors
- 404: Provider not found (expected by Terraform)
- 500: Server error (logged with details)
- 503: Service unavailable (during startup/shutdown)

### 8.3 Admin API Errors
- 401: Unauthorized (invalid/expired token)
- 403: Forbidden (valid token, insufficient permissions)
- 400: Bad request (validation errors)
- 422: Unprocessable entity (business logic errors)

## 9. Deployment

### 9.1 Container Build

Multi-stage Dockerfile:
1. Stage 1: Node.js - Build Vue frontend
2. Stage 2: Go builder - Compile Go binary
3. Stage 3: Distroless static - Final runtime image

### 9.2 Required Volumes
- `/data` - SQLite database
- `/var/cache/tf-mirror` - Disk cache
- `/etc/tf-mirror` - Configuration files (optional, can use env vars)

### 9.3 Environment Variables
See Configuration section for full list.

### 9.4 Initial Setup
1. Set `TFM_ADMIN_USERNAME` and `TFM_ADMIN_PASSWORD` env vars
2. Start container
3. Application creates initial admin user (password hashed)
4. Admin logs in and changes password
5. Remove env vars, restart container

## 10. Testing Strategy

### 10.1 Unit Tests
- All business logic in `internal/` packages
- Mock interfaces for external dependencies
- Target: >80% code coverage

### 10.2 Integration Tests
- Use testcontainers-go for MinIO
- Test full provider download flow
- Test database operations
- Test cache behavior

### 10.3 E2E Tests
- Use Playwright for UI testing
- Test actual Terraform client integration
- Use hashicorp/null and hashicorp/random providers (small, stable)

## 11. Phase 1 Implementation Checklist

### Core Infrastructure
- [ ] Project structure setup
- [ ] Go module initialization
- [ ] Configuration loader (HCL + env vars)
- [ ] Database schema and migrations
- [ ] S3 storage client
- [ ] Two-tier cache implementation

### Provider Mirror
- [ ] Service discovery endpoint
- [ ] Version listing endpoint
- [ ] Provider download endpoint
- [ ] HCL file parser for provider definitions
- [ ] Provider downloader with GPG verification
- [ ] Job processing system

### Authentication & Admin API
- [ ] JWT authentication
- [ ] Password hashing (bcrypt)
- [ ] Session management
- [ ] Admin login/logout
- [ ] Provider upload endpoint
- [ ] Job status endpoint
- [ ] Storage stats endpoint
- [ ] Audit log endpoint

### Frontend
- [ ] Vue 3 + Vite setup
- [ ] TailwindCSS configuration
- [ ] Router setup
- [ ] Pinia stores
- [ ] Login page
- [ ] Provider list/search
- [ ] Admin dashboard
- [ ] Job status view

### Deployment
- [ ] Multi-stage Dockerfile
- [ ] Docker Compose example
- [ ] Kubernetes manifests
- [ ] Helm chart
- [ ] Documentation

### Testing
- [ ] Unit tests for core logic
- [ ] Integration tests with MinIO
- [ ] E2E tests with Terraform client
- [ ] CI/CD pipeline

## 12. Success Criteria

Phase 1 is considered complete when:
1. Admin can upload a provider definition HCL file
2. System downloads providers from registry.terraform.io
3. Providers are verified with GPG signatures
4. Providers are stored in S3-compatible storage
5. Terraform client can discover and download cached providers
6. Admin can view download job progress
7. Admin can view storage usage and audit logs
8. All tests pass
9. Container can be deployed via Docker Compose and Kubernetes
10. Documentation is complete
