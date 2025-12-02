# Terraform Mirror - Implementation Questions & Suggestions

## General Architecture Questions

### Container Architecture

1. **Single vs Multi-Container Approach**
   - Should the Go backend and TypeScript frontend run in separate containers or as a single container?
   - **Suggestion**: Start with a single container for simplicity, but design with clear separation of concerns to allow easy splitting later if needed for scaling.
   - If multi-container: How will you handle inter-service communication? (REST API, gRPC?) REST API

We will go with a single container to start with.

2. **Base Image Selection**
   - Alpine is mentioned, but have you considered distroless images for even smaller attack surface?
   - **Suggestion**: Use multi-stage builds - one stage for Go compilation, another for TypeScript build, final stage for runtime.

I am fine with distroless images. Please provide examples for me to select from.

**CLARIFICATION**: Distroless image options:
   - `gcr.io/distroless/static-debian12` - For static Go binaries (recommended if no CGO dependencies)
   - `gcr.io/distroless/base-debian12` - For Go binaries with minimal runtime dependencies
   - Both are ~20MB vs Alpine's ~5MB, but have smaller attack surface (no shell, package manager, etc.)
   - **Recommendation**: Use `static-debian12` for the final stage since Go can compile to a static binary

### Storage Layer

3. **S3 Client Library**
   - Which S3 SDK will you use for Go? (AWS SDK v2 recommended)
   - How will you handle credentials for S3/MinIO? (IAM roles, access keys, environment variables?)
   - **Suggestion**: Support both AWS IAM roles (for AWS deployments) and access key authentication (for MinIO/other S3-compatible stores).

Let's go with the recommended SDK. We should support both auth methods.

4. **Storage Structure**
   - What will the object key structure look like in S3?
     - Providers: `providers/{hostname}/{namespace}/{type}/{version}/{os}_{arch}/{filename}`?
     - Modules: `modules/{namespace}/{name}/{provider}/{version}/{files}`?
   - How will you handle metadata? (S3 object metadata, separate index files, database?)

Use the links in the planning document to match the storage to the expected responses from the provider network mirror and module registry protocols. Metadata can go in a SQLLite database or key-value store. I'm open to suggestions.

**CLARIFICATION**: For metadata storage:

   - **SQLite** (recommended for single-container deployment):
     - Pros: ACID compliance, SQL queries, no additional dependencies, file-based
     - Cons: Single writer limitation (but likely fine for this use case)
     - Store: provider/module versions, checksums, deprecation flags, admin actions log
   - **bbolt** (embedded key-value store):
     - Pros: Pure Go, no CGO, very lightweight
     - Cons: No SQL, more manual indexing needed
   - **Question**: Should the SQLite DB also be backed up to S3 periodically for disaster recovery?

5. **Caching Strategy**
   - Will you implement a local disk cache in front of S3 for frequently accessed providers/modules?
   - What's the cache invalidation strategy?
   - **Suggestion**: Implement a two-tier cache (memory + disk) with configurable TTLs.

Two-tier cache sounds good. The TTL can be configured by the admin.

## Provider Network Mirror Specifics

6. **Provider Download Logic**
   - When auto-downloading providers, will you verify signatures using HashiCorp's GPG keys?
   - **Suggestion**: Always verify provider signatures to prevent supply chain attacks.
   - How will you handle failed downloads? Retry logic? Error reporting to clients?

Yes, GPG keys should be verified for each provider file.

Since the server will run in environments that may be network constrained, there are two possible behaviors.

* When bulk loading providers, use standard retry logic and fail the download after 60s. The error should be reported to the admin.
* When attempting to download a provider on-demand from a client, report back to Terraform using a 500 HTTP error. Check with the network mirror protocol to make sure Terraform will handle this gracefully.

**CLARIFICATION**: 
   - For on-demand downloads, should the server queue the download and return 404 (not yet available) while downloading, then have the client retry? Or block the request until download completes (with timeout)?
   - Should there be a max file size limit for auto-downloads to prevent storage exhaustion?
   - Per the protocol spec, Terraform expects 404 for missing providers. Should we return 404 immediately and download in background, making it available on next retry?

7. **Predefined Provider File Format**
   - What format for the provider definition file? (JSON, YAML, HCL?)
   - **Suggestion**: Use YAML for readability:
     ```yaml
     providers:
       - source: hashicorp/aws
         versions: ["5.31.0", "5.30.0"]
         architectures: ["linux_amd64", "darwin_arm64"]
       - source: hashicorp/azurerm
         versions: ["3.84.0"]
         architectures: ["linux_amd64"]
     ```

HCL is preferred. This is for Terraform after all.

**CLARIFICATION**: Example HCL structure needed:
   ```hcl
   provider "hashicorp/aws" {
     versions = ["5.31.0", "5.30.0", "~> 5.0"]
     architectures = ["linux_amd64", "darwin_arm64", "windows_amd64"]
   }
   
   provider "hashicorp/azurerm" {
     versions = ["3.84.0"]
     architectures = ["linux_amd64"]
   }
   ```
   - Should we support version constraints in the HCL file (e.g., `~> 5.0` to fetch latest 5.x)?
   - Should architectures be optional and default to a set (e.g., linux_amd64, linux_arm64, darwin_amd64, darwin_arm64)?

8. **Version Management**
   - How will you handle version constraints? (exact versions only, or support ranges?)
   - Should the system support automatic updates to latest patch versions?
   - How will you handle provider deprecation/removal?

When the version is a range, serve up the latest version that supports the constraint. If no version is available, attempt to download the newest supported version from upstream.

Administrators should be able to run a regular update to grab the latest version for all providers for each major version being included.

Providers that have been manually removed from the mirror by admins should be marked as deprecated and not downloaded again.

**CLARIFICATION**:
   - Should "regular update" be a scheduled task (cron-like) or manual admin action via UI/API?
   - When updating "latest version for each major version", should it update all major versions found, or only majors explicitly configured?
   - For blocked providers: should the block apply to the entire provider or specific versions?
   - Should there be a "force re-download" option in case a cached provider is corrupted?

## Module Registry Mirror Specifics

9. **Module Source Rewriting**
   - Will you provide tooling to help users rewrite their module sources?
   - **Suggestion**: Create a CLI tool or script that can scan .tf files and update module sources automatically.

This is an eventual goal. Not a day-1 requirement.

10. **Module Download & Verification**
    - Modules from the public registry - how will you verify integrity?
    - Will you support private modules from other registries (GitHub, GitLab)?
    - **Suggestion**: Store checksums and verify on each serve.

How does the module registry protocol ensure integrity today? We should use that. Checksums sounds good to me.

For today, we are only going to support modules from the public registry at registry.terraform.io.

**CLARIFICATION**:
   - The Terraform registry uses Git commit SHAs and version tags for integrity. For modules downloaded from registry.terraform.io, we should:
     - Store the source URL and commit SHA returned by the registry API
     - Verify archive checksums on download
     - Re-verify checksums when serving to clients
   - Should we cache the module source code or just proxy requests to the upstream registry?
   - If caching: do we download the .tar.gz archive or clone the Git repository?

11. **Module Versioning**
    - The registry protocol supports version discovery - will you cache all versions or only requested ones?
    - How will you handle semantic version constraints in module requests?

For modules, we will cache all versions.

For version constraints, the server will return the latest version that meets the constraint.

If the client requests a newer version than what is available locally, the server will attempt to download a newer version if it exists.

**CLARIFICATION**:
   - "Cache all versions" - does this mean automatically fetch all available versions from registry.terraform.io on first request, or fetch on-demand as clients request different versions?
   - Should there be a configurable limit on number of versions cached per module to prevent storage bloat?
   - For the version constraint resolution, should this happen at module discovery time or when Terraform requests a specific version?

## Frontend/Web UI

12. **UI Framework**
    - Which TypeScript framework? (React, Vue, Svelte, vanilla?)
    - **Suggestion**: Consider a lightweight option like Svelte or Vue for faster load times.

Vue is good.

13. **UI Features**
    - What discovery features do consumers need?
      - Search by provider/module name?
      - Browse by namespace?
      - Version comparison?
      - Download statistics?
    - Admin UI features needed?
      - Dashboard showing storage usage?
      - Logs/audit trail?
      - Bulk import/export?

Consumers should be able to search the currently cached providers and modules by name or browse by name.

They should be able to select the version of a provider or module to view using a drop-down menu.

Download statistics are not included.

Admins should be able to manage what versions are available and remove them if needed. They should be able to see storage usage. A log should be included of when administrative actions have been taken. They should be able to import providers and modules in bulk using a file upload. They should also be able to mark provider and module versions as deprecated, and block certain providers and modules entirely.

**CLARIFICATION**:
   - For bulk import via file upload: should this be the HCL file format mentioned earlier, or a different format?
   - Should the import be synchronous (wait for all downloads) or asynchronous (queued background job with progress tracking)?
   - For storage usage display: break down by provider vs module? Show top consumers? Include cache storage?
   - For admin action logging: what level of detail? (user, timestamp, action, affected resources, IP address?)
   - When marking as deprecated: should they still be available for download with a warning, or blocked entirely?

14. **API Design**
    - Will the admin functions use a separate API endpoint (`/admin/api`) or share the same backend?
    - **Suggestion**: Use separate `/admin/api` with authentication middleware.

Use separate `/admin/api` endpoint as suggested.

## Security & Authentication

15. **Password Storage**
    - Which hashing algorithm? (bcrypt recommended, or argon2?)
    - **Suggestion**: Use bcrypt with cost factor 12-14, or argon2id.

Use the suggestion.

16. **Session Management**
    - JWT tokens or server-side sessions?
    - Session timeout duration?
    - **Suggestion**: Use HTTP-only cookies with JWT tokens for admin sessions.

Use the suggestion.

17. **Future SSO Integration**
    - Which SSO protocols to support? (OIDC, SAML?)
    - **Suggestion**: Design auth layer as pluggable to make SSO integration easier later.

Use suggestion.

18. **Rate Limiting**
    - Should you implement rate limiting to prevent abuse?
    - Different limits for consumers vs authenticated admins?
    - **Suggestion**: Yes, implement rate limiting per IP with higher limits for authenticated users.

Rate limiting can be handled through a reverse proxy or api gateway.

## Configuration & Deployment

19. **Configuration Management**
    - How will configuration be provided? (environment variables, config file, both?)
    - **Suggestion**: Support both, with environment variables taking precedence.
    - Example config:
      ```yaml
      server:
        port: 8080
        tls_enabled: true
      storage:
        type: s3
        bucket: terraform-mirror
        endpoint: https://minio.example.com
      features:
        auto_download_providers: true
        auto_download_modules: true
      ```

Agree with the suggestion. I would prefer HCL to yaml.

**CLARIFICATION**: Example HCL configuration structure:
   ```hcl
   server {
     port = 8080
     tls_enabled = true
     tls_cert_path = "/etc/tf-mirror/cert.pem"
     tls_key_path = "/etc/tf-mirror/key.pem"
   }
   
   storage {
     type = "s3"
     bucket = "terraform-mirror"
     region = "us-east-1"
     endpoint = "https://minio.example.com"  # optional for MinIO
     
     # Auth method 1: IAM role (leave access_key empty)
     # Auth method 2: Access keys
     access_key = "AKIAIOSFODNN7EXAMPLE"      # optional
     secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"  # optional
   }
   
   cache {
     memory_size_mb = 256
     disk_path = "/var/cache/tf-mirror"
     disk_size_gb = 10
     ttl_seconds = 3600
   }
   
   features {
     auto_download_providers = true
     auto_download_modules = true
     max_download_size_mb = 500
   }
   ```
   - Should sensitive values like secret_key support environment variable references (e.g., `${env.S3_SECRET_KEY}`)?

20. **Initial Setup**
    - How will the initial admin account be created?
    - **Suggestion**: Use environment variables for initial admin creation, or interactive setup on first run.

Agree with suggestion. The setup should not generate or store the admin password in plaintext.

**CLARIFICATION**:
   - For interactive setup: should this be a CLI wizard or web-based initial setup page?
   - For environment variables: use `ADMIN_USERNAME` and `ADMIN_PASSWORD` (password hashed on first run)?
   - Should there be a way to reset admin password if locked out? (e.g., special environment variable or CLI command?)
   - Support multiple admin accounts from the start, or single admin initially?

21. **Health Checks & Monitoring**
    - Health check endpoints for container orchestration?
    - Metrics export (Prometheus format)?
    - **Suggestion**: Implement `/health` and `/metrics` endpoints.

Agree with suggestion. We should support OpenTelemetry format.

**CLARIFICATION**:
   - `/health` endpoint should return what information? (HTTP 200 + JSON with S3 connectivity, DB status, cache status?)
   - For OpenTelemetry: should we export traces, metrics, or both?
   - OTLP export destination: gRPC, HTTP, or both?
   - Should OTLP be optional/configurable, or always enabled?
   - Common metrics to track: request latency, cache hit/miss ratio, download counts, storage usage, active downloads?

## Network & Protocol Implementation

22. **HTTPS/TLS**
    - Will TLS termination happen at the container level or expect reverse proxy?
    - **Suggestion**: Support both - built-in TLS for simple deployments, but work behind reverse proxy too.

Support both.

**CLARIFICATION**:
   - For built-in TLS: auto-generate self-signed cert on first run, or require admin to provide cert/key?
   - Support Let's Encrypt ACME protocol for automatic cert management?
   - When behind reverse proxy: should we detect `X-Forwarded-Proto` and other proxy headers?
   - Redirect HTTP to HTTPS when TLS is enabled?

23. **Protocol Compliance**
    - Have you reviewed the full protocol specs for edge cases?
    - How will you handle protocol version negotiation if specs evolve?
    - **Suggestion**: Implement strict protocol compliance with version headers.

Agree with suggestion.

## Testing & Quality

24. **Testing Strategy**
    - Unit tests for both Go and TypeScript?
    - Integration tests against real S3/MinIO?
    - End-to-end tests with actual Terraform client?
    - **Suggestion**: All three levels, with ability to run integration tests against MinIO in CI/CD.

Agree with suggestion.

25. **Test Fixtures**
    - How will you test without downloading real providers repeatedly?
    - **Suggestion**: Create mock provider packages for testing, intercept actual HashiCorp registry calls in tests.

Unsure of this one. We can download actual providers and clean up the storage afterwards. Mock provider packages could be tricky.

**CLARIFICATION**:
   - Using real providers for tests: should we commit a few small test providers to the repo for offline testing?
   - Alternative: use Go's `httptest` to mock the HashiCorp registry responses during unit tests
   - For integration tests: use a dedicated MinIO container with test data that can be reset between test runs
   - Should we use Terraform's own test providers (e.g., `hashicorp/null`, `hashicorp/random`) as they're small and stable?

## Documentation

26. **Documentation Needs**
    - Installation guide
    - Configuration reference
    - API documentation
    - User guide for both personas
    - **Suggestion**: Use OpenAPI/Swagger for API docs, keep user docs in markdown.

Agree with suggestion.

## Implementation Phases

### Suggested Phase 1 (MVP)

- Basic container with Go backend
- S3 storage implementation
- Provider network mirror protocol (read-only, manual loading from file)
- Simple admin authentication
- Basic web UI for browsing

### Suggested Phase 2

- Auto-download functionality for providers
- Module registry mirror protocol
- Enhanced web UI with search
- Admin configuration UI

### Suggested Phase 3

- Auto-download for modules
- Advanced caching
- Metrics and monitoring
- Performance optimizations

### Suggested Phase 4

- SSO integration
- Advanced admin features (bulk operations, audit logs)
- Multi-region support
- High availability considerations

## Open Design Decisions

1. **Namespace handling**: How to handle namespace mapping between public and private mirror? Not sure what is meant here. The network mirror for providers will use public namespaces and the client configuration file to establish the mirror. The module mirror will rely on updating the source argument for modules to point to the mirror instead of the public namespace. 

   **CLARIFICATION**: This is clear - no custom namespace mapping needed since:
   - Providers use Terraform's built-in mirror configuration
   - Modules require source rewriting to point to mirror hostname
   - Question: Should the UI show both the original source and mirror source for clarity? 
2. **Concurrent downloads**: If multiple clients request same uncached provider simultaneously, download once or multiple times? Download once if it's the same version requested.

   **CLARIFICATION**: Implementation approach:
   - Use a sync.Map or mutex to track in-progress downloads by provider+version+arch key
   - First request initiates download, subsequent requests wait for completion
   - Question: What's the max wait time before timing out? Same 60s as bulk downloads?
   - Should waiting requests get progress updates, or just block until complete/failed?
3. **Storage quota**: Should there be storage limits? Automatic cleanup of old versions? Cleanup of older versions will be handled by the admin.

   **CLARIFICATION**:
   - Should the system have a configurable max storage size that prevents new downloads when reached?
   - Warning threshold (e.g., alert admin at 80% capacity)?
   - When at capacity: fail new downloads or allow admin to configure priority (auto-delete oldest unused versions)?
4. **Backup/restore**: How to backup the mirror? Just S3 backup or need metadata export too? We will need to backup the metadata and S3 mirror.

   **CLARIFICATION**:
   - For S3 backup: rely on S3 versioning/replication, or provide export functionality?
   - For metadata backup: automated periodic SQLite backup to S3?
   - Should there be a `/admin/api/backup` endpoint to trigger on-demand backups?
   - Restore process: manual file replacement or automated restore from backup?
   - Include configuration file in backup bundle?
5. **Migration path**: How to migrate from one mirror instance to another? I'm not worried about this just yet. For migration I think we could have an export feature that exports the config and metadata, and then it can be imported by a new instance.

   **CLARIFICATION**:
   - Export format: single archive file (.tar.gz) containing SQLite DB + config?
   - S3 data migration: assume both instances can access same S3 bucket, or need S3-to-S3 copy feature?
   - Import: API endpoint or CLI command?
   - Handle version conflicts: if importing into instance with existing data, merge or replace?

## Recommended Tech Stack

- **Backend**: Go 1.23+ with:
  - `chi` for routing (more lightweight than gorilla/mux, better performance)
  - `aws-sdk-go-v2` for S3
  - `golang.org/x/crypto/bcrypt` for password hashing
  - `golang-jwt/jwt/v5` for JWT tokens
  - `hashicorp/hcl/v2` for HCL parsing
  - `mattn/go-sqlite3` or `modernc.org/sqlite` (pure Go) for SQLite
  - `go.opentelemetry.io/otel` for observability
  - `hashicorp/go-getter` for downloading providers/modules (already handles Terraform protocols)
  
- **Frontend**: 
  - Vite + TypeScript + Vue 3 (Composition API)
  - TailwindCSS for styling
  - Vue Router for navigation
  - Pinia for state management
  
- **Container**:
  - Multi-stage Dockerfile
  - `gcr.io/distroless/static-debian12` as final stage
  
- **Testing**:
  - Go: `testify` for assertions, `httptest` for HTTP testing
  - TypeScript: Vitest
  - E2E: Playwright with Terraform CLI in tests
  - Integration: Testcontainers-go for MinIO

**CLARIFICATION**:
  - Should we use `modernc.org/sqlite` (pure Go, no CGO) vs `mattn/go-sqlite3` (CGO required)?
  - Pure Go = easier cross-compilation, CGO = better performance. Recommendation: pure Go for simplicity.

## Next Steps

1. Create detailed technical design document based on answers to above questions
2. Set up project structure and repository
3. Create proof-of-concept for S3 storage layer
4. Implement provider mirror protocol (read-only first)
5. Build basic container and test with actual Terraform client

## Additional Questions Needing Answers

### Database Schema

- What tables/structure for SQLite database?
  - `providers` table: id, namespace, type, version, architecture, s3_path, checksum, verified, deprecated, blocked, created_at
  - `modules` table: id, namespace, name, provider, version, s3_path, source_url, commit_sha, created_at
  - `admin_users` table: id, username, password_hash, created_at, updated_at
  - `admin_actions` table: id, user_id, action, resource_type, resource_id, timestamp, ip_address, details
  - `config` table: key-value pairs for runtime configuration?

Config should be held outside the Database. All other tables look good for the first pass.

### Download Progress Tracking

- For async bulk downloads, how should progress be communicated to admin?
  - WebSocket connection for real-time updates?
  - Polling endpoint that returns job status?
  - Store job status in database and provide `/admin/api/jobs/{id}` endpoint?

Polling endpoint sounds good. Page should refresh on a regular basis (every 3 seconds)

### Error Handling & Client Experience

- When auto-download fails (network issue, invalid provider, etc.):
  - Should failed downloads be retried automatically? Yes
  - How long should failed download info be cached to avoid repeated attempts? Exponential backoff with a total of 5 retries.
  - Should there be a "retry failed downloads" admin action? Yes, only after 5 failed attempts.

### Session Management Details

- JWT token expiration time? (e.g., 8 hours, 24 hours?) Configurable with a default or 8 hours.
- Refresh token support, or require re-login after expiration? For re-login after expiration.
- Should sessions be invalidated on password change? Yes.
- Store active sessions in database for audit/revocation? Yes.

### Multi-Architecture Support

- The mirror needs to serve multiple architectures - should the predefined provider file allow specifying "all" or "auto-detect common architectures"? All or a list of architectures. The default is all.
- Default architectures if not specified: linux_amd64, linux_arm64, darwin_amd64, darwin_arm64, windows_amd64? The default is all.

### Provider GPG Verification

- Should the HashiCorp GPG public key be embedded in the application or downloaded? Downloaded during configuration from a trusted source.
- Update mechanism for GPG keys if they change? This is an admin function.
- What to do if GPG verification fails? Block the provider or allow with warning? Block the provider and notify admin.

### Logging

- Log levels: debug, info, warn, error? Yes
- Log output: stdout/stderr, file, both? Configurable
- Structured logging (JSON) or plain text? Plain text
- Include request IDs for tracing requests through the system? Yes

### Container Orchestration

- Target deployment platforms: Docker Compose, Kubernetes, both? Both.
- Should we provide Helm chart and/or docker-compose.yml examples? Yes
- Persistent volume requirements for cache and SQLite DB? Persistent volumes should be required, it's up to the user to decide the storage class and target.
