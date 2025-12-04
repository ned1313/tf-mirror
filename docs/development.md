# Development Guide

This guide covers how to set up a development environment, understand the codebase architecture, and contribute to Terraform Mirror.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Development Setup](#development-setup)
- [Project Structure](#project-structure)
- [Architecture Overview](#architecture-overview)
- [Key Components](#key-components)
- [Testing](#testing)
- [Building](#building)
- [Contributing](#contributing)

---

## Prerequisites

### Required Tools

- **Go 1.24+**: The backend is written in Go
  - Install: https://go.dev/dl/
  
- **Node.js 18+**: For the Vue.js admin frontend
  - Install: https://nodejs.org/
  
- **Docker & Docker Compose**: For containerized development
  - Install: https://docs.docker.com/get-docker/

### Optional Tools

- **Air**: For Go live reload during development
  ```bash
  go install github.com/air-verse/air@latest
  ```

- **Make**: For running common tasks (Makefile provided)

- **MinIO Client (`mc`)**: For S3 storage debugging
  ```bash
  brew install minio/stable/mc  # macOS
  choco install minio-client    # Windows
  ```

---

## Development Setup

### 1. Clone the Repository

```bash
git clone https://github.com/your-org/terraform-mirror.git
cd terraform-mirror
```

### 2. Install Dependencies

**Backend:**
```bash
go mod download
```

**Frontend:**
```bash
cd web
npm install
cd ..
```

### 3. Create Local Configuration

Create `config.local.hcl`:

```hcl
server {
  port     = "8080"
  hostname = "localhost"
}

storage {
  type = "local"
  local_path = "./data/providers"
}

database {
  path = "./data/mirror.db"
}

cache {
  enabled = true
  memory_mb = 128
  disk_enabled = true
  disk_path = "./data/cache"
  disk_max_mb = 1024
}

features {
  multi_platform = true
  enable_admin   = true
}

auth {
  token_expiry = "24h"
  jwt_secret   = "dev-secret-change-in-production"
}

processor {
  enabled     = true
  workers     = 4
  download_timeout = "10m"
}

logging {
  level  = "debug"
  format = "text"
}
```

### 4. Create Data Directories

```bash
mkdir -p data/providers data/cache
```

### 5. Install Git Hooks (Recommended)

Install the pre-commit hook to catch common issues before they reach CI:

**Linux/macOS:**
```bash
./scripts/install-hooks.sh
```

**Windows (PowerShell):**
```powershell
.\scripts\install-hooks.ps1
```

The pre-commit hook checks for:
- Go formatting issues (`gofmt`)
- Go vet errors
- Build errors
- Linting issues (`golangci-lint` if installed)
- Test failures

Install `golangci-lint` for full linting (same as CI):
```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### 6. Seed the Database

```bash
go run cmd/create-admin/main.go -config config.local.hcl -username admin -password admin123
```

### 7. Run the Application

**Backend (without live reload):**
```bash
go run cmd/terraform-mirror/main.go -config config.local.hcl
```

**Backend (with live reload):**
```bash
air -c .air.toml
```

**Frontend (development mode):**
```bash
cd web
npm run dev
```

The frontend dev server runs at `http://localhost:5173` and proxies API calls to the backend at `http://localhost:8080`.

### 8. Alternative: Docker Compose Development

```bash
docker-compose -f deployments/docker-compose/docker-compose.dev.yml up
```

This starts:
- Backend with live reload
- Frontend dev server
- MinIO for S3 storage

---

## Project Structure

```
terraform-mirror/
├── cmd/                          # Application entry points
│   ├── terraform-mirror/         # Main server binary
│   │   └── main.go
│   ├── create-admin/             # Admin user creation utility
│   ├── reset-password/           # Password reset utility
│   └── verify-password/          # Password verification utility
│
├── internal/                     # Private application code
│   ├── auth/                     # Authentication service
│   ├── cache/                    # Caching layer
│   ├── config/                   # Configuration loading
│   ├── database/                 # SQLite database & repositories
│   ├── processor/                # Background job processor
│   ├── provider/                 # Provider registry client
│   ├── server/                   # HTTP server & handlers
│   ├── storage/                  # Storage backends
│   └── version/                  # Version information
│
├── web/                          # Vue.js admin frontend
│   ├── src/
│   │   ├── components/           # Reusable Vue components
│   │   ├── views/                # Page-level components
│   │   ├── stores/               # Pinia state stores
│   │   ├── services/             # API service classes
│   │   ├── router/               # Vue Router configuration
│   │   └── types/                # TypeScript type definitions
│   └── ...
│
├── deployments/                  # Deployment configurations
│   ├── docker/                   # Dockerfile
│   └── docker-compose/           # Compose variants
│
├── docs/                         # Documentation
├── examples/                     # Example configurations
├── scripts/                      # Utility scripts
│
├── config.local.hcl              # Local development config (gitignored)
├── go.mod                        # Go module definition
├── go.sum                        # Go dependency checksums
└── Makefile                      # Build automation
```

---

## Architecture Overview

### High-Level Flow

```
┌───────────────────┐     ┌───────────────────┐
│  Terraform CLI    │     │    Admin UI       │
└─────────┬─────────┘     └─────────┬─────────┘
          │                         │
          │ HTTP                    │ HTTP
          │                         │
┌─────────▼─────────────────────────▼─────────┐
│                HTTP Server                   │
│  ┌─────────────┐  ┌─────────────────────┐   │
│  │   Mirror    │  │    Admin API        │   │
│  │  Protocol   │  │   (JWT auth)        │   │
│  └──────┬──────┘  └──────────┬──────────┘   │
└─────────┼────────────────────┼──────────────┘
          │                    │
┌─────────▼────────────────────▼──────────────┐
│                 Cache Layer                  │
│      (Memory LRU + Disk-backed cache)       │
└─────────────────────┬───────────────────────┘
                      │
┌─────────────────────▼───────────────────────┐
│              Service Layer                   │
│  ┌──────────────┐  ┌────────────────────┐   │
│  │   Provider   │  │     Processor      │   │
│  │   Service    │  │     Service        │   │
│  └──────┬───────┘  └──────────┬─────────┘   │
└─────────┼────────────────────┬┘─────────────┘
          │                    │
          │      ┌─────────────▼──────────────┐
          │      │     Public Terraform       │
          │      │        Registry            │
          │      └────────────────────────────┘
          │
┌─────────▼───────────────────────────────────┐
│              Storage Layer                   │
│  ┌──────────────┐  ┌────────────────────┐   │
│  │    Local     │  │       S3           │   │
│  │  Filesystem  │  │  (or MinIO)        │   │
│  └──────────────┘  └────────────────────┘   │
└─────────────────────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────┐
│              Database Layer                  │
│              (SQLite + WAL)                  │
└─────────────────────────────────────────────┘
```

### Request Flow: Provider Download

1. Terraform client requests provider versions
2. HTTP server checks cache for response
3. If cached, return immediately
4. If not cached, query database for available versions
5. Return versions JSON, cache the response
6. Terraform requests specific binary
7. Server retrieves binary from storage
8. Binary streamed to client

### Request Flow: Provider Upload (Admin)

1. Admin uploads provider HCL definition
2. Server parses HCL to extract providers/versions/platforms
3. Creates job record in database
4. Background processor picks up job
5. For each provider/version/platform:
   - Downloads from public registry
   - Stores in storage backend
   - Updates database with metadata
6. Job marked complete

---

## Key Components

### Config (`internal/config/`)

Handles HCL configuration parsing with environment variable overrides.

```go
// Load configuration
cfg, err := config.Load("config.local.hcl")

// Access values
port := cfg.Server.Port
storageType := cfg.Storage.Type
```

Key files:
- `config.go` - Configuration struct definitions
- `loader.go` - HCL parsing and env var processing
- `validation.go` - Configuration validation

### Database (`internal/database/`)

SQLite database with repository pattern for data access.

```go
// Create database connection
db, err := database.NewDatabase(cfg.Database)

// Use repositories
providers, err := db.Providers.ListByNamespace(ctx, "hashicorp")
job, err := db.Jobs.Create(ctx, &models.Job{...})
```

Key files:
- `database.go` - Connection and schema management
- `models.go` - Data models (Provider, Job, User, etc.)
- `provider_repository.go` - Provider CRUD operations
- `job_repository.go` - Job management
- `user_repository.go` - User authentication
- `audit_repository.go` - Audit logging
- `session_repository.go` - Session management

### Storage (`internal/storage/`)

Abstraction over local filesystem and S3-compatible storage.

```go
// Create storage from config
store, err := storage.NewStorage(cfg.Storage)

// Store a file
err = store.Put(ctx, "hashicorp/aws/5.31.0/linux_amd64/provider.zip", reader)

// Retrieve a file
reader, err := store.Get(ctx, "hashicorp/aws/5.31.0/linux_amd64/provider.zip")

// Delete a file
err = store.Delete(ctx, "hashicorp/aws/5.31.0/linux_amd64/provider.zip")
```

Key files:
- `storage.go` - Interface definition
- `local.go` - Filesystem implementation
- `s3.go` - S3/MinIO implementation
- `factory.go` - Storage construction from config

### Cache (`internal/cache/`)

Two-tier caching with memory LRU and disk-backed storage.

```go
// Create cache from config
cache, err := cache.NewCacheFromConfig(cfg.Cache)

// Cache operations
err = cache.Set(ctx, "key", data, time.Hour)
data, hit := cache.Get(ctx, "key")
cache.Delete(ctx, "key")

// Get statistics
stats := cache.Stats()
```

Key files:
- `cache.go` - Interface definition
- `memory.go` - In-memory LRU cache
- `disk.go` - Disk-backed cache
- `tiered.go` - Two-tier (memory + disk) cache
- `factory.go` - Cache construction from config

### Server (`internal/server/`)

HTTP server with Chi router, middleware, and handlers.

```go
// Create and start server
srv := server.NewServer(cfg, db, store, cache, processor)
srv.Start()
```

Key files:
- `server.go` - Server setup and routing
- `middleware.go` - Auth, logging, recovery middleware
- `provider_mirror_protocol.go` - Terraform mirror protocol handlers
- `auth_handlers.go` - Login/logout handlers
- `admin_providers.go` - Provider management API
- `cache_handlers.go` - Cache management API

### Provider (`internal/provider/`)

Client for interacting with the public Terraform Registry.

```go
// Create downloader
downloader := provider.NewDownloader(http.DefaultClient)

// Download a provider
info, err := downloader.Download(ctx, DownloadRequest{
    Namespace: "hashicorp",
    Type:      "aws",
    Version:   "5.31.0",
    OS:        "linux",
    Arch:      "amd64",
})
```

Key files:
- `service.go` - High-level provider operations
- `downloader.go` - Registry API client
- `parser.go` - HCL provider definition parser

### Processor (`internal/processor/`)

Background worker pool for processing download jobs.

```go
// Create processor
proc := processor.NewProcessor(cfg.Processor, db, store, downloader)

// Start processing (non-blocking)
proc.Start()

// Submit a job
jobID, err := proc.SubmitJob(ctx, []provider.Definition{...})

// Stop gracefully
proc.Stop()
```

Key files:
- `service.go` - Worker pool and job processing

### Auth (`internal/auth/`)

JWT-based authentication service.

```go
// Create auth service
auth := auth.NewService(cfg.Auth, db)

// Authenticate user
token, err := auth.Login(ctx, username, password)

// Validate token
claims, err := auth.ValidateToken(token)

// Logout
err = auth.Logout(ctx, sessionID)
```

Key files:
- `service.go` - Authentication logic

---

## Testing

### Running Tests

**All tests:**
```bash
go test ./...
```

**Specific package:**
```bash
go test ./internal/storage/...
```

**With verbose output:**
```bash
go test -v ./internal/cache/...
```

**With coverage:**
```bash
go test -cover ./...
```

**Generate coverage report:**
```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

### Test Structure

Tests are colocated with implementation files:
```
internal/cache/
├── cache.go
├── memory.go
├── memory_test.go    # Unit tests for memory cache
├── disk.go
├── disk_test.go      # Unit tests for disk cache
└── tiered_test.go    # Integration tests for tiered cache
```

### Writing Tests

**Unit test example:**
```go
func TestMemoryCache_SetGet(t *testing.T) {
    cache := NewMemoryCache(1024) // 1MB max
    ctx := context.Background()
    
    // Set a value
    err := cache.Set(ctx, "key1", []byte("value1"), time.Hour)
    require.NoError(t, err)
    
    // Get the value
    data, ok := cache.Get(ctx, "key1")
    require.True(t, ok)
    require.Equal(t, []byte("value1"), data)
}
```

**Table-driven test example:**
```go
func TestParser_ParseProviders(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    []Provider
        wantErr bool
    }{
        {
            name:  "single provider",
            input: `provider "hashicorp/aws" { versions = ["5.0.0"] }`,
            want:  []Provider{{Namespace: "hashicorp", Type: "aws", Versions: []string{"5.0.0"}}},
        },
        {
            name:    "invalid syntax",
            input:   `provider "invalid" {`,
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ParseProviders(tt.input)
            if tt.wantErr {
                require.Error(t, err)
                return
            }
            require.NoError(t, err)
            require.Equal(t, tt.want, got)
        })
    }
}
```

### Integration Tests

Integration tests use build tags:
```go
//go:build integration

package storage

func TestS3Storage_Integration(t *testing.T) {
    // Requires real S3/MinIO
}
```

Run integration tests:
```bash
go test -tags=integration ./...
```

### Running Integration Tests with MinIO

Integration tests require MinIO to be running. Use the provided scripts:

**PowerShell (Windows):**
```powershell
.\scripts\run-integration-tests.ps1
.\scripts\run-integration-tests.ps1 -KeepRunning          # Keep MinIO running after tests
.\scripts\run-integration-tests.ps1 -Package storage      # Test only storage package
```

**Bash (Linux/macOS):**
```bash
./scripts/run-integration-tests.sh
./scripts/run-integration-tests.sh --keep-running
./scripts/run-integration-tests.sh --package storage
```

**Manual setup:**
```bash
# Start MinIO
docker-compose -f deployments/docker-compose/docker-compose.test.yml up -d

# Run integration tests
go test -v -tags=integration ./internal/storage/... ./internal/processor/...

# Stop MinIO
docker-compose -f deployments/docker-compose/docker-compose.test.yml down -v
```

### End-to-End Tests

E2E tests start the full stack and test the complete workflow including Terraform CLI integration:

**PowerShell (Windows):**
```powershell
.\scripts\run-e2e-tests.ps1
.\scripts\run-e2e-tests.ps1 -KeepRunning           # Keep stack running after tests
.\scripts\run-e2e-tests.ps1 -SkipTerraformTests    # Skip Terraform CLI tests
.\scripts\run-e2e-tests.ps1 -SkipBuild             # Don't rebuild Docker image
```

**Bash (Linux/macOS):**
```bash
./scripts/run-e2e-tests.sh
./scripts/run-e2e-tests.sh --keep-running
./scripts/run-e2e-tests.sh --skip-terraform
./scripts/run-e2e-tests.sh --skip-build
```

E2E tests verify:
- Health and metrics endpoints
- Service discovery (`/.well-known/terraform.json`)
- Admin authentication
- Provider loading via HCL upload
- Job processing and completion
- Terraform CLI integration with the mirror

### Mocking

Use interfaces for mockable dependencies:

```go
// In production
srv := server.NewServer(cfg, realDB, realStorage, realCache)

// In tests
srv := server.NewServer(cfg, mockDB, mockStorage, mockCache)
```

---

## Building

### Development Build

```bash
go build -o terraform-mirror ./cmd/terraform-mirror
```

### Production Build

```bash
# With version info
go build -ldflags="-s -w -X main.version=v1.0.0" -o terraform-mirror ./cmd/terraform-mirror

# Cross-compile
GOOS=linux GOARCH=amd64 go build -o terraform-mirror-linux-amd64 ./cmd/terraform-mirror
```

### Docker Build

```bash
docker build -f deployments/docker/Dockerfile -t terraform-mirror:latest .
```

### Frontend Build

```bash
cd web
npm run build
# Output: web/dist/
```

The production Docker image embeds the built frontend.

### Makefile Targets

```bash
make build           # Build binary
make test            # Run tests
make docker          # Build Docker image
make docker-push     # Push to registry
make clean           # Clean build artifacts
```

---

## Contributing

### Code Style

- Follow standard Go formatting (`gofmt`)
- Use meaningful variable and function names
- Write godoc comments for exported items
- Keep functions focused and small

### Commit Messages

Follow conventional commits:
```
feat: add provider deprecation feature
fix: handle empty version list in parser
docs: update API documentation
test: add cache tiered tests
refactor: simplify storage factory
```

### Pull Request Process

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Make changes and test thoroughly
4. Ensure tests pass: `go test ./...`
5. Commit with meaningful messages
6. Push and create a Pull Request
7. Address review feedback

### Adding a New Feature

1. **Plan**: Discuss in an issue first for major features
2. **Implement**: Write code following existing patterns
3. **Test**: Add unit tests, integration tests if needed
4. **Document**: Update relevant documentation
5. **Review**: Submit PR for review

### Adding a New Storage Backend

1. Create `internal/storage/newbackend.go`
2. Implement the `Storage` interface
3. Add to factory in `internal/storage/factory.go`
4. Add configuration in `internal/config/config.go`
5. Add tests in `internal/storage/newbackend_test.go`
6. Document configuration options

### Debugging Tips

**Enable debug logging:**
```hcl
logging {
  level = "debug"
}
```

**Use delve debugger:**
```bash
dlv debug ./cmd/terraform-mirror -- -config config.local.hcl
```

**Database inspection:**
```bash
sqlite3 data/mirror.db
.tables
SELECT * FROM providers LIMIT 10;
```

**S3/MinIO inspection:**
```bash
mc alias set local http://localhost:9000 minioadmin minioadmin
mc ls local/terraform-mirror/
```

---

## Resources

- [Go Documentation](https://go.dev/doc/)
- [Chi Router](https://go-chi.io/)
- [Vue.js 3](https://vuejs.org/)
- [Terraform Provider Registry Protocol](https://developer.hashicorp.com/terraform/internals/provider-registry-protocol)
- [Terraform Network Mirror Protocol](https://developer.hashicorp.com/terraform/internals/provider-network-mirror-protocol)
