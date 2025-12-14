# Phase 3: Module Registry Mirror - Implementation Plan

This document outlines the implementation plan for Phase 3 based on the answers provided in `phase3-questions.md`.

## Summary of Decisions

| Question | Decision |
|----------|----------|
| Nested module handling | Dynamically rewrite remote module sources; local paths unchanged |
| Upstream registry config | Yes, default to `registry.terraform.io` |
| S3 key structure | `modules/{namespace}/{name}/{system}/{version}/{filename}` |
| Module storage format | Extract and repackage (to rewrite nested sources) |
| Status flags | Yes (`deprecated`, `blocked`) |
| Track original source | Yes |
| Auto-download config | Separate flag: `auto_download_modules` |
| Size limits | Shared with providers (`max_download_size_mb`) |
| HCL format | Exact versions only (no constraints) |
| Upstream verification | Trust upstream registry |
| Version list caching | Fetch fresh |
| UI approach | Combined "Registry" view with tabs |
| Dashboard stats | Include module statistics |
| Job system | Same system, `job_type = 'module'` |
| Job items table | Separate `module_job_items` table |
| Admin API | Follow provider pattern |
| Auto-download | Include from start (Phase 2 complete) |

---

## Implementation Tasks

### Week 1-2: Database & Core Models

#### 1.1 Database Schema Updates

Add new tables for modules:

```sql
-- Modules table (parallel to providers)
CREATE TABLE modules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    namespace TEXT NOT NULL,           -- e.g., "terraform-aws-modules"
    name TEXT NOT NULL,                 -- e.g., "vpc"
    system TEXT NOT NULL,               -- e.g., "aws"
    version TEXT NOT NULL,              -- e.g., "5.0.0"
    
    -- Storage information
    s3_key TEXT NOT NULL,
    filename TEXT NOT NULL,
    size_bytes INTEGER NOT NULL DEFAULT 0,
    
    -- Original source tracking
    original_source_url TEXT,           -- e.g., GitHub tarball URL
    
    -- Status flags
    deprecated BOOLEAN NOT NULL DEFAULT 0,
    blocked BOOLEAN NOT NULL DEFAULT 0,
    
    -- Timestamps
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(namespace, name, system, version)
);

CREATE INDEX idx_modules_lookup ON modules(namespace, name, system);
CREATE INDEX idx_modules_deprecated ON modules(deprecated);
CREATE INDEX idx_modules_blocked ON modules(blocked);

-- Module job items table (separate from provider job items)
CREATE TABLE module_job_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id INTEGER NOT NULL,
    
    -- Module identity
    namespace TEXT NOT NULL,
    name TEXT NOT NULL,
    system TEXT NOT NULL,
    version TEXT NOT NULL,
    
    -- Item status
    status TEXT NOT NULL,               -- pending, downloading, completed, failed
    
    -- Results
    module_id INTEGER,
    error_message TEXT,
    
    -- Retry tracking
    retry_count INTEGER NOT NULL DEFAULT 0,
    
    -- Timestamps
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME,
    
    FOREIGN KEY (job_id) REFERENCES download_jobs(id),
    FOREIGN KEY (module_id) REFERENCES modules(id)
);

CREATE INDEX idx_module_job_items_job ON module_job_items(job_id);
CREATE INDEX idx_module_job_items_status ON module_job_items(status);
```

**Files to create/modify:**

- [ ] `internal/database/models.go` - Add `Module` and `ModuleJobItem` structs
- [ ] `internal/database/database.go` - Add module tables to migrations
- [ ] `internal/database/module_repository.go` - New file for module CRUD operations
- [ ] `internal/database/module_job_repository.go` - New file for module job items

#### 1.2 Module Repository Implementation

Create `internal/database/module_repository.go` with methods:

- `Create(ctx, module) (int64, error)`
- `GetByID(ctx, id) (*Module, error)`
- `GetByIdentity(ctx, namespace, name, system, version) (*Module, error)`
- `ListVersions(ctx, namespace, name, system) ([]*Module, error)`
- `List(ctx, opts ListOptions) ([]*Module, int, error)`
- `Update(ctx, module) error`
- `Delete(ctx, id) error`
- `CountByStatus(ctx) (total, deprecated, blocked int, error)`

#### 1.3 Module Job Item Repository

Create `internal/database/module_job_repository.go` with methods:

- `CreateItem(ctx, item) (int64, error)`
- `GetItem(ctx, id) (*ModuleJobItem, error)`
- `ListItemsByJob(ctx, jobID) ([]*ModuleJobItem, error)`
- `UpdateItem(ctx, item) error`
- `CountItemsByStatus(ctx, jobID) (pending, completed, failed int, error)`

---

### Week 2-3: Module Package & Storage

#### 2.1 Configuration Updates

Update `internal/config/config.go`:

```go
// Add to FeaturesConfig
AutoDownloadModules bool `hcl:"auto_download_modules,optional"`

// Add new config block
type ModulesConfig struct {
    UpstreamRegistry string `hcl:"upstream_registry,optional"` // default: registry.terraform.io
}

// Add AutoDownloadModulesConfig (similar to AutoDownloadConfig for providers)
type AutoDownloadModulesConfig struct {
    Enabled              bool     `hcl:"enabled,optional"`
    AllowedNamespaces    []string `hcl:"allowed_namespaces,optional"`
    BlockedNamespaces    []string `hcl:"blocked_namespaces,optional"`
    RateLimitPerMinute   int      `hcl:"rate_limit_per_minute,optional"`
    MaxConcurrentDL      int      `hcl:"max_concurrent_downloads,optional"`
    TimeoutSeconds       int      `hcl:"timeout_seconds,optional"`
    CacheNegativeResults bool     `hcl:"cache_negative_results,optional"`
    NegativeCacheTTL     int      `hcl:"negative_cache_ttl_seconds,optional"`
}
```

**Files to modify:**

- [ ] `internal/config/config.go` - Add module configuration structs
- [ ] `internal/config/loader.go` - Add defaults for module config
- [ ] `internal/config/validation.go` - Add module config validation

#### 2.2 Module Downloader

Create `internal/module/downloader.go`:

```go
// RegistryClient interface for module registry operations
type RegistryClient interface {
    GetAvailableVersions(ctx context.Context, namespace, name, system string) ([]string, error)
    GetDownloadURL(ctx context.Context, namespace, name, system, version string) (string, error)
    DownloadModule(ctx context.Context, url string) ([]byte, error)
}

// Downloader handles module downloads
type Downloader struct {
    httpClient *http.Client
    registry   string // upstream registry hostname
}
```

**Key methods:**

- `GetAvailableVersions(ctx, namespace, name, system) ([]string, error)`
- `GetDownloadURL(ctx, namespace, name, system, version) (string, error)`
- `DownloadModule(ctx, url string) ([]byte, error)`

**Files to create:**

- [ ] `internal/module/downloader.go` - Registry client and download logic
- [ ] `internal/module/downloader_test.go` - Unit tests

#### 2.3 Module Source Rewriter

Create `internal/module/rewriter.go` to handle nested module source rewriting:

```go
// Rewriter handles rewriting module sources in downloaded modules
type Rewriter struct {
    mirrorHostname string // the hostname clients will use
}

// RewriteModule extracts a tarball, rewrites remote sources, and repacks
func (r *Rewriter) RewriteModule(tarball []byte) ([]byte, error)

// rewriteTerraformFiles finds and rewrites .tf files with remote module sources
func (r *Rewriter) rewriteTerraformFiles(dir string) error
```

**Logic:**

1. Extract tarball to temp directory
2. Find all `.tf` files recursively
3. Parse each file for `module` blocks
4. For each module with a remote `source` (not starting with `./` or `../`):
   - Rewrite to include mirror hostname prefix
5. Repack as tarball
6. Return rewritten tarball

**Files to create:**

- [ ] `internal/module/rewriter.go` - Source rewriting logic
- [ ] `internal/module/rewriter_test.go` - Unit tests with sample modules

#### 2.4 Module Parser (HCL Definitions)

Create `internal/module/parser.go` (parallel to `internal/provider/parser.go`):

```hcl
# Example module definition file
module "terraform-aws-modules/vpc/aws" {
  versions = ["5.0.0", "5.1.0", "5.2.0"]
}

module "terraform-aws-modules/eks/aws" {
  versions = ["19.0.0"]
}
```

**Files to create:**

- [ ] `internal/module/parser.go` - HCL parser for module definitions
- [ ] `internal/module/parser_test.go` - Unit tests

#### 2.5 Module Service

Create `internal/module/service.go` (orchestration layer):

```go
// Service orchestrates module download operations
type Service struct {
    downloader   *Downloader
    rewriter     *Rewriter
    storage      storage.Storage
    moduleRepo   *database.ModuleRepository
    mirrorHost   string
}

// LoadFromHCL parses definitions and creates download job
func (s *Service) LoadFromHCL(ctx context.Context, content []byte, userID int64) (*database.DownloadJob, error)

// ProcessModule downloads, rewrites, and stores a single module
func (s *Service) ProcessModule(ctx context.Context, namespace, name, system, version string) (*database.Module, error)
```

**Files to create:**

- [ ] `internal/module/service.go` - Service orchestration
- [ ] `internal/module/service_test.go` - Unit tests

#### 2.6 Storage Keys

Update `internal/storage/keys.go` (or create if not exists):

```go
// ModuleKey generates the S3 key for a module
func ModuleKey(namespace, name, system, version string) string {
    filename := fmt.Sprintf("%s-%s-%s-%s.tar.gz", namespace, name, system, version)
    return fmt.Sprintf("modules/%s/%s/%s/%s/%s", namespace, name, system, version, filename)
}
```

**Files to modify:**

- [ ] `internal/storage/s3.go` or create `internal/storage/keys.go`

---

### Week 3-4: Module Mirror Protocol

#### 3.1 Service Discovery Update

Update `/.well-known/terraform.json` response:

```json
{
  "providers.v1": "/v1/providers/",
  "modules.v1": "/v1/modules/"
}
```

**Files to modify:**

- [ ] `internal/server/handlers.go` - Update service discovery handler

#### 3.2 Module Version Listing Endpoint

Implement `GET /v1/modules/{namespace}/{name}/{system}/versions`:

```go
func (s *Server) handleModuleVersions(w http.ResponseWriter, r *http.Request) {
    // Response format:
    // {
    //   "modules": [
    //     {
    //       "versions": [
    //         {"version": "1.0.0"},
    //         {"version": "1.1.0"}
    //       ]
    //     }
    //   ]
    // }
}
```

**Files to create:**

- [ ] `internal/server/module_mirror_protocol.go` - Module mirror handlers
- [ ] `internal/server/module_mirror_protocol_test.go` - Protocol tests

#### 3.3 Module Download Endpoint

Implement `GET /v1/modules/{namespace}/{name}/{system}/{version}/download`:

```go
func (s *Server) handleModuleDownload(w http.ResponseWriter, r *http.Request) {
    // Response: 204 No Content with X-Terraform-Get header
    // X-Terraform-Get: {presigned S3 URL to tarball}
}
```

#### 3.4 Auto-Download Integration

Create `internal/module/auto_download.go` (parallel to provider auto-download):

```go
// AutoDownloadService handles on-demand module downloads
type AutoDownloadService struct {
    config      *config.AutoDownloadModulesConfig
    downloader  *Downloader
    rewriter    *Rewriter
    storage     storage.Storage
    moduleRepo  *database.ModuleRepository
    rateLimiter *rate.Limiter
    // ... similar to provider AutoDownloadService
}
```

**Files to create:**

- [ ] `internal/module/auto_download.go` - Auto-download service
- [ ] `internal/module/auto_download_test.go` - Unit tests

---

### Week 4-5: Admin API

#### 5.1 Module Admin Handlers

Create `internal/server/admin_modules.go`:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/admin/api/modules` | GET | List modules with filtering/pagination |
| `/admin/api/modules/{id}` | GET | Get module details |
| `/admin/api/modules/{id}` | PUT | Update module (deprecate/block) |
| `/admin/api/modules/{id}` | DELETE | Delete module |
| `/admin/api/modules/load` | POST | Load modules from HCL file |

**Files to create:**

- [ ] `internal/server/admin_modules.go` - Module admin handlers
- [ ] `internal/server/admin_modules_test.go` - Handler tests

#### 5.2 Update Stats Endpoints

Update `/admin/api/stats/storage` to include module statistics:

```json
{
  "providers": {
    "total": 150,
    "size_bytes": 5368709120
  },
  "modules": {
    "total": 45,
    "size_bytes": 1073741824
  },
  "total_size_bytes": 6442450944
}
```

**Files to modify:**

- [ ] `internal/server/handlers.go` - Update stats handlers

#### 5.3 Router Updates

Add module routes to the server:

**Files to modify:**

- [ ] `internal/server/server.go` - Add module routes

---

### Week 5-6: Job Processor Updates

#### 6.1 Update Job Types

Extend the job system to handle modules:

```go
const (
    JobTypeProvider = "provider"
    JobTypeModule   = "module"
)
```

**Files to modify:**

- [ ] `internal/database/job_repository.go` - Add job type constants
- [ ] `internal/processor/service.go` - Handle module job types

#### 6.2 Module Job Processing

Add module processing to the processor service:

```go
func (s *Service) processModuleJob(ctx context.Context, job *database.DownloadJob) error {
    // Get module job items
    // For each item: download, rewrite, store
    // Update job progress
}
```

**Files to modify:**

- [ ] `internal/processor/service.go` - Add module job processing

---

### Week 6-7: Frontend Updates

#### 7.1 TypeScript Types

Add module types to `web/src/types/`:

```typescript
// web/src/types/module.ts
export interface Module {
  id: number;
  namespace: string;
  name: string;
  system: string;
  version: string;
  s3_key: string;
  filename: string;
  size_bytes: number;
  original_source_url?: string;
  deprecated: boolean;
  blocked: boolean;
  created_at: string;
  updated_at: string;
}
```

**Files to create:**

- [ ] `web/src/types/module.ts` - Module type definitions

#### 7.2 Module Store

Create Pinia store for modules:

**Files to create:**

- [ ] `web/src/stores/modules.ts` - Module state management

#### 7.3 Registry View (Combined)

Create combined Registry view with tabs for Providers and Modules:

```
/admin/registry
├── Tab: Providers (existing ProviderList)
└── Tab: Modules (new ModuleList)
```

**Files to create/modify:**

- [ ] `web/src/views/Registry.vue` - New combined view
- [ ] `web/src/components/modules/ModuleList.vue` - Module list component
- [ ] `web/src/components/modules/ModuleDetail.vue` - Module detail modal
- [ ] `web/src/components/modules/ModuleUpload.vue` - HCL upload for modules
- [ ] `web/src/router/index.ts` - Update routes

#### 7.4 Dashboard Updates

Update dashboard to show module statistics:

**Files to modify:**

- [ ] `web/src/views/Admin.vue` or `Dashboard.vue` - Add module stats cards

---

### Week 7-8: Testing & Documentation

#### 8.1 Unit Tests

Target: >80% coverage for new code

- [ ] Module repository tests
- [ ] Module job repository tests  
- [ ] Module downloader tests
- [ ] Module rewriter tests
- [ ] Module parser tests
- [ ] Module service tests
- [ ] Module auto-download tests
- [ ] Module admin handler tests
- [ ] Module mirror protocol tests

#### 8.2 Integration Tests

- [ ] Full module download flow (download → rewrite → store)
- [ ] Module mirror protocol with Terraform client
- [ ] Auto-download on demand

#### 8.3 E2E Tests

- [ ] Admin UI module management
- [ ] Module upload via HCL
- [ ] Terraform client module download

#### 8.4 Documentation Updates

- [ ] `docs/api.md` - Add module API endpoints
- [ ] `docs/configuration.md` - Add module configuration
- [ ] `docs/user-guide.md` - Add module usage guide
- [ ] `README.md` - Update feature list

---

## File Summary

### New Files

| File | Description |
|------|-------------|
| `internal/module/downloader.go` | Module registry client and download |
| `internal/module/downloader_test.go` | Downloader tests |
| `internal/module/rewriter.go` | Nested module source rewriting |
| `internal/module/rewriter_test.go` | Rewriter tests |
| `internal/module/parser.go` | HCL module definition parser |
| `internal/module/parser_test.go` | Parser tests |
| `internal/module/service.go` | Module orchestration service |
| `internal/module/service_test.go` | Service tests |
| `internal/module/auto_download.go` | On-demand module downloads |
| `internal/module/auto_download_test.go` | Auto-download tests |
| `internal/database/module_repository.go` | Module CRUD operations |
| `internal/database/module_repository_test.go` | Repository tests |
| `internal/database/module_job_repository.go` | Module job items |
| `internal/server/admin_modules.go` | Admin API handlers |
| `internal/server/admin_modules_test.go` | Handler tests |
| `internal/server/module_mirror_protocol.go` | Mirror protocol handlers |
| `internal/server/module_mirror_protocol_test.go` | Protocol tests |
| `web/src/types/module.ts` | TypeScript types |
| `web/src/stores/modules.ts` | Pinia store |
| `web/src/views/Registry.vue` | Combined registry view |
| `web/src/components/modules/ModuleList.vue` | Module list |
| `web/src/components/modules/ModuleDetail.vue` | Module detail |
| `web/src/components/modules/ModuleUpload.vue` | HCL upload |

### Modified Files

| File | Changes |
|------|---------|
| `internal/config/config.go` | Add module config structs |
| `internal/config/loader.go` | Add module defaults |
| `internal/config/validation.go` | Add module validation |
| `internal/database/database.go` | Add module table migrations |
| `internal/database/models.go` | Add Module, ModuleJobItem structs |
| `internal/processor/service.go` | Handle module job types |
| `internal/server/server.go` | Add module routes |
| `internal/server/handlers.go` | Update service discovery, stats |
| `web/src/router/index.ts` | Add registry route |
| `web/src/views/Admin.vue` | Add module stats |

---

## Timeline Summary

| Week | Focus Area |
|------|------------|
| 1-2 | Database schema, repositories, models |
| 2-3 | Module downloader, rewriter, parser, storage |
| 3-4 | Module mirror protocol endpoints |
| 4-5 | Admin API for modules |
| 5-6 | Job processor updates |
| 6-7 | Frontend (Registry view, components) |
| 7-8 | Testing and documentation |

**Total: ~8 weeks**

---

## Risk Considerations

### 1. Module Source Rewriting Complexity

Rewriting Terraform module sources is non-trivial:

- Need to parse HCL properly (not just regex)
- Handle various source formats (registry, GitHub, S3, etc.)
- Preserve file formatting where possible
- Handle edge cases (commented blocks, heredocs, etc.)

**Mitigation:** Use `hclwrite` package for proper HCL manipulation.

### 2. Tarball Handling

- Various compression formats (`.tar.gz`, `.zip`)
- Potential for large modules
- Directory structure variations

**Mitigation:** Support common formats, enforce size limits.

### 3. Nested Module Recursion

When rewriting nested modules, those modules may also have nested modules.

**Mitigation:** Only rewrite at download time; when a nested module is requested, it goes through the same download/rewrite process.

---

## Success Criteria

Phase 3 is complete when:

1. [ ] Module database schema is implemented
2. [ ] Modules can be loaded via HCL definition file
3. [ ] Modules are downloaded from upstream registry
4. [ ] Nested module sources are rewritten
5. [ ] Modules are stored in S3
6. [ ] Terraform clients can discover modules via mirror
7. [ ] Terraform clients can download cached modules
8. [ ] Auto-download works for modules on demand
9. [ ] Admin UI shows modules in combined Registry view
10. [ ] Dashboard shows module statistics
11. [ ] Admin can deprecate/block modules
12. [ ] All tests pass (>80% coverage for new code)
13. [ ] Documentation is updated
