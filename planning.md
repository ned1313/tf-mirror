# Terraform Mirror

This project is intended to provide two useful functions to Terraform users in air-gapped or low-bandwidth environments.

## Provider Network Mirror

The first benefit is to provide a network mirror for terraform providers. The mirror will be enabled through the client settings for the Terraform client as detailed here: https://developer.hashicorp.com/terraform/cli/config/config-file#provider-installation

The network mirror protocol should be implemented as detailed here: https://developer.hashicorp.com/terraform/internals/provider-network-mirror-protocol

In addition to implementing the protocol, the provider mirror should also have two additional features:

1. Automatically download new providers on demand when requested by a terraform client. The server will download the requested version only. This feature can be disabled by the administrator.
1. Load a set of predefined providers from a file. The file will include the provider source addresses, architectures, and versions to include.

## Module Mirror

The second benefit is to provide a mirror for Terraform modules from the public registry. The Terraform client doesn't currently support a mirror setting for modules, so we need to take a different approach.

To use the module mirror, the source property for a module will be updated to include the hostname of the module mirror: e.g. if the current module source is "terraform-aws-modules/iam/aws", the new source will be "mirror.hostname.local/terraform-aws-modules/iam/aws"

The module will then be served by the local cache on the Terraform Mirror server. To serve the modules properly, the server will need to implement the Terraform Registry protocol as detailed here: https://developer.hashicorp.com/terraform/internals/module-registry-protocol

In addition to implementing the protocol, the module mirror should also have two additional features:

1. Automatically download new modules on demand when requested by a terraform client. The server will download the requested version only. This feature can be disabled by the administrator.
1. Load a set of predefined modules from a file. The file will include the module source addresses and versions to include.

## Architecture

The backend logic should be developed using Go. The frontend should use Typescript for the Web UI and to handle requests. There should be two endpoints: `providers` and `modules` to handle the two distinct functionalities supported by the server.

For the server, it should be available to run as a container. I'd like to start with a minimal container, maybe alpine, and layer in only the necessary components. TBD whether the backend and frontend run as separate containers? The persistent storage for the modules and providers should be S3 compliant object store. That can be actual AWS S3 or MinIO.

For the initial implementation, there should be two personas. Admins are able to configure the server options, add and remove modules and providers, and pre-load modules and providers. Consumers will have read-only access to the web UI for discovery and read-only access to download modules and providers. Consumers will not require authentication initially.

Admins will require authentication. We'll use a simple username/password login. Later on, I'd like to introduce SSO. Admin credentials should be created during the setup process and should be changeable by the admin. Credentials should be stored hashed and salted.

## Implementation Progress

### Completed

#### Backend (Go)
- ✅ Core server with Chi router
- ✅ SQLite database with providers, admin users, sessions, jobs, audit logs
- ✅ S3-compatible storage (AWS S3 + MinIO)
- ✅ Provider Network Mirror Protocol (https://developer.hashicorp.com/terraform/internals/provider-network-mirror-protocol)
- ✅ JWT authentication for admin sessions
- ✅ Admin API endpoints:
  - `POST /admin/api/login` - Admin authentication
  - `POST /admin/api/logout` - Session termination
  - `GET /admin/api/me` - Current user info
  - `GET /admin/api/providers` - List providers
  - `GET /admin/api/providers/{id}` - Get provider details
  - `PUT /admin/api/providers/{id}` - Update provider (deprecate/block)
  - `DELETE /admin/api/providers/{id}` - Delete provider
  - `POST /admin/api/providers/load` - Load providers from HCL file
  - `GET /admin/api/stats/storage` - Storage statistics
  - `GET /admin/api/stats/audit` - Audit logs
  - `POST /admin/api/stats/recalculate` - Recalculate storage sizes
  - `GET /admin/api/config` - Server configuration (sanitized)
  - `POST /admin/api/backup` - Trigger database backup
  - `GET /admin/api/jobs` - List download jobs
  - `GET /admin/api/jobs/{id}` - Get job details
  - `POST /admin/api/jobs/{id}/retry` - Retry failed job
  - `POST /admin/api/jobs/{id}/cancel` - Cancel running/pending job
  - `GET /admin/api/processor/status` - Background processor status
  - `POST /admin/api/processor/start` - Start processor
  - `POST /admin/api/processor/stop` - Stop processor
- ✅ Background job processor with concurrent downloads
- ✅ Audit logging with IP tracking
- ✅ Database backup to local filesystem and S3
- ✅ Configuration via environment variables

#### Frontend (Vue 3 + TypeScript + Vite)
- ✅ Project setup with Tailwind CSS
- ✅ TypeScript types matching Go API responses
- ✅ API client with Axios (interceptors for auth)
- ✅ Pinia stores (auth, providers, jobs, stats)
- ✅ Vue Router with authentication guards
- ✅ Components:
  - AppHeader - Top navigation bar
  - AppSidebar - Left navigation with quick stats
  - AdminLayout - Wrapper layout component
- ✅ Views:
  - Login - Admin authentication
  - Admin (Dashboard) - Overview with stats, recent activity, active jobs
  - Providers - List, filter, update, delete providers
  - Modules - List, filter, upload, delete modules
  - Jobs - View job history, retry failed jobs, auto-refresh
  - AuditLogs - Search and filter audit entries
  - Settings - View configuration, trigger backups
  - BrowseProviders - Public provider browsing (no auth required)
  - BrowseModules - Public module browsing (no auth required)
- ✅ Build passes with TypeScript checking

### Pending

#### High Priority

- [x] Module Registry Protocol implementation ✅
- [x] Auto-download providers on demand ✅
- [x] Auto-download modules on demand ✅
- [x] Docker containerization ✅
- [x] Frontend updates for modules ✅
- [x] Public browse pages for unauthenticated users ✅
- [x] Async job processing with progress tracking ✅
- [ ] Production deployment configuration

#### Medium Priority

- [x] Module storage and management ✅
- [ ] Rate limiting (per-consumer)
- [x] Caching layer (memory + disk) ✅
- [ ] Telemetry/observability enhancements
- [ ] Improve test coverage to >80%

#### Low Priority

- [ ] SSO integration
- [ ] Multiple admin users
- [ ] Advanced search/filtering
- [ ] GPG signature verification for providers
