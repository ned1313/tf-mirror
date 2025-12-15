# Terraform Mirror - Testing Guide

This document provides comprehensive instructions for QA and functionality testing of Terraform Mirror. It covers setup, manual testing procedures, and automated test execution.

## Table of Contents

1. [Test Environment Setup](#test-environment-setup)
2. [Quick Smoke Test](#quick-smoke-test)
3. [Authentication Tests](#authentication-tests)
4. [Provider Management Tests](#provider-management-tests)
5. [Module Management Tests](#module-management-tests)
6. [Public Browse Tests](#public-browse-tests)
7. [Job Processing Tests](#job-processing-tests)
8. [Terraform Client Integration Tests](#terraform-client-integration-tests)
9. [API Tests](#api-tests)
10. [UI/UX Tests](#uiux-tests)
11. [Performance Tests](#performance-tests)
12. [Error Handling Tests](#error-handling-tests)
13. [Cleanup Procedures](#cleanup-procedures)

---

## Test Environment Setup

### Prerequisites

- Docker and Docker Compose installed
- Go 1.21+ (for running automated tests)
- Node.js 20+ (for frontend development)
- Terraform CLI 1.5+ (for integration tests)
- curl or Postman (for API testing)
- A web browser (Chrome, Firefox, or Edge recommended)

### Starting the Test Environment

#### Option 1: Docker Compose (Recommended)

```bash
# Start the full stack with MinIO
docker-compose -f deployments/docker-compose/docker-compose.local.yml up -d

# Verify services are running
docker-compose -f deployments/docker-compose/docker-compose.local.yml ps
```

Expected output:
- `terraform-mirror` - Running on https://localhost:8443
- `minio` - Running on http://localhost:9000 (console: http://localhost:9001)

#### Option 2: Local Development

```bash
# Terminal 1: Start the backend
make run-local

# Terminal 2: Start the frontend dev server
cd web && npm run dev
```

### Creating the Admin User

```bash
# Using Docker
docker-compose -f deployments/docker-compose/docker-compose.local.yml exec terraform-mirror \
  /app/create-admin -username admin -password "YourSecurePassword123!"

# Using local development
go run cmd/create-admin/main.go -username admin -password "YourSecurePassword123!"
```

### Verifying the Setup

1. Open https://localhost:8443 in your browser
2. Accept the self-signed certificate warning
3. You should see the Terraform Mirror home page with "Browse Providers" and "Browse Modules" cards

---

## Quick Smoke Test

Perform these tests to verify basic functionality:

| # | Test | Steps | Expected Result |
|---|------|-------|-----------------|
| 1 | Home page loads | Navigate to https://localhost:8443 | Home page displays with welcome message and browse cards |
| 2 | Health endpoint | `curl -k https://localhost:8443/health` | Returns `{"status":"healthy","database":"ok"}` |
| 3 | Service discovery | `curl -k https://localhost:8443/.well-known/terraform.json` | Returns JSON with `providers.v1` endpoint |
| 4 | Admin login page | Click "Admin Login" or navigate to /admin/login | Login form displays |
| 5 | Metrics endpoint | `curl -k https://localhost:8443/metrics` | Prometheus metrics output |

---

## Authentication Tests

### Test A1: Successful Login

**Preconditions:** Admin user created

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Navigate to /admin/login | Login form displays |
| 2 | Enter valid username | Username field accepts input |
| 3 | Enter valid password | Password field accepts input (masked) |
| 4 | Click "Sign In" | Redirected to /admin dashboard |
| 5 | Check sidebar | Shows username and logout option |

### Test A2: Failed Login - Invalid Credentials

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Navigate to /admin/login | Login form displays |
| 2 | Enter invalid username/password | Fields accept input |
| 3 | Click "Sign In" | Error message: "Invalid credentials" |
| 4 | URL remains | Still on /admin/login |

### Test A3: Failed Login - Empty Fields

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Navigate to /admin/login | Login form displays |
| 2 | Leave fields empty | - |
| 3 | Click "Sign In" | Validation error shown |

### Test A4: Session Persistence

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Login successfully | Redirected to dashboard |
| 2 | Close browser tab | - |
| 3 | Open new tab, navigate to /admin | Still logged in (session persists) |

### Test A5: Logout

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Login successfully | On dashboard |
| 2 | Click "Logout" in sidebar | - |
| 3 | Verify redirect | Redirected to /admin/login |
| 4 | Navigate to /admin | Redirected back to login |

### Test A6: Protected Route Access

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Clear browser storage/cookies | Logged out state |
| 2 | Navigate to /admin/providers | Redirected to /admin/login |
| 3 | Navigate to /admin/modules | Redirected to /admin/login |
| 4 | Navigate to /admin/jobs | Redirected to /admin/login |

---

## Provider Management Tests

### Test P1: Upload Provider HCL File

**Preconditions:** Logged in as admin

Create a test file `test-providers.hcl`:
```hcl
provider "registry.terraform.io/hashicorp/random" {
  versions = ["3.5.0", "3.5.1"]
  platforms = ["linux_amd64", "darwin_arm64", "windows_amd64"]
}
```

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Navigate to /admin/providers | Providers list displays |
| 2 | Click "Upload Providers" | Modal opens with file input |
| 3 | Select test-providers.hcl | File name shown |
| 4 | Click "Upload" | Modal closes, success toast shown |
| 5 | Check Jobs page | New job created with "running" or "pending" status |
| 6 | Wait for job completion | Job status changes to "completed" |
| 7 | Return to Providers | hashicorp/random providers listed |

### Test P2: View Provider Details

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Navigate to /admin/providers | Providers list displays |
| 2 | Click on a provider row | Provider detail modal opens |
| 3 | Verify details shown | Namespace, name, version, platform, size, dates |
| 4 | Click close/outside modal | Modal closes |

### Test P3: Filter Providers

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Navigate to /admin/providers | All providers listed |
| 2 | Enter "random" in search | Only random providers shown |
| 3 | Clear search | All providers shown again |
| 4 | Select namespace filter | Filtered by namespace |

### Test P4: Delete Provider

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Navigate to /admin/providers | Providers listed |
| 2 | Click delete icon on a provider | Confirmation dialog appears |
| 3 | Confirm deletion | Provider removed from list |
| 4 | Refresh page | Provider still gone |

### Test P5: Deprecate Provider

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Click on a provider | Detail modal opens |
| 2 | Toggle "Deprecated" | Provider marked as deprecated |
| 3 | Verify in list | Deprecated badge shown |

### Test P6: Invalid HCL Upload

Create `invalid.hcl`:
```hcl
this is not valid HCL {{{
```

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Click "Upload Providers" | Modal opens |
| 2 | Select invalid.hcl | File selected |
| 3 | Click "Upload" | Error message about parse failure |

### Test P7: Empty Provider File

Create `empty.hcl`:
```hcl
# No providers defined
```

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Upload empty.hcl | Error: "No providers found" |

---

## Module Management Tests

### Test M1: Upload Module HCL File

Create `test-modules.hcl`:
```hcl
module "terraform-aws-modules/vpc/aws" {
  versions = ["5.0.0", "5.1.0"]
}

module "terraform-aws-modules/s3-bucket/aws" {
  versions = ["4.0.0"]
}
```

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Navigate to /admin/modules | Modules list displays |
| 2 | Click "Upload Modules" | Modal opens |
| 3 | Select test-modules.hcl | File shown |
| 4 | Click "Upload" | Modal closes, job created |
| 5 | Wait for completion | Job completes |
| 6 | Check Modules list | vpc and s3-bucket modules listed |

### Test M2: View Module Details

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Click on a module row | Detail modal opens |
| 2 | Verify Terraform usage shown | Example source URL displayed |
| 3 | Verify versions listed | All versions shown |

### Test M3: Filter Modules

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Enter "vpc" in search | Only VPC modules shown |
| 2 | Filter by namespace | Namespace-filtered results |

### Test M4: Delete Module

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Click delete on a module | Confirmation appears |
| 2 | Confirm | Module removed |

---

## Public Browse Tests

### Test B1: Browse Providers (Unauthenticated)

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Open incognito/private window | Fresh session |
| 2 | Navigate to /providers | Public providers page loads |
| 3 | Verify no login required | Page displays without redirect |
| 4 | See provider list | Aggregated by namespace/name |
| 5 | Click on a provider | Shows versions and platforms |

### Test B2: Browse Modules (Unauthenticated)

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Open incognito/private window | Fresh session |
| 2 | Navigate to /modules | Public modules page loads |
| 3 | Verify module list | Aggregated by namespace/name/system |
| 4 | Filter by namespace | Filters work |

### Test B3: Home Page Navigation

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Navigate to / | Home page displays |
| 2 | Click "Browse Providers" card | Goes to /providers |
| 3 | Go back, click "Browse Modules" | Goes to /modules |
| 4 | Click "Admin Login" | Goes to /admin/login |

### Test B4: Public API Endpoints

```bash
# Test public providers API
curl -k https://localhost:8443/api/public/providers

# Test public modules API
curl -k https://localhost:8443/api/public/modules

# Test with namespace filter
curl -k "https://localhost:8443/api/public/providers?namespace=hashicorp"
```

| Endpoint | Expected Result |
|----------|-----------------|
| /api/public/providers | JSON array with aggregated providers |
| /api/public/modules | JSON array with aggregated modules |

---

## Job Processing Tests

### Test J1: View Jobs List

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Navigate to /admin/jobs | Jobs list displays |
| 2 | Verify columns | ID, Type, Status, Progress, Created, Started, Completed |
| 3 | Check pagination | Page controls work if >10 jobs |

### Test J2: Job Auto-Refresh

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Navigate to /admin/jobs | Jobs page loads |
| 2 | Upload a provider file (new tab) | Creates a new job |
| 3 | Return to jobs tab | New job appears automatically |
| 4 | Watch job progress | Status updates without manual refresh |

### Test J3: View Job Details

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Click on a job row | Job detail modal opens |
| 2 | Verify info shown | Items list, progress, error messages |
| 3 | Check item statuses | Individual item success/failure shown |

### Test J4: Cancel Pending/Running Job

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Start a large provider upload | Job in pending/running |
| 2 | Click "Cancel" on the job | Confirmation appears |
| 3 | Confirm cancellation | Job status → "cancelled" |

### Test J5: Retry Failed Job

**Preconditions:** Have a failed job (e.g., network error during download)

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Find a failed job | Status shows "failed" |
| 2 | Click "Retry" | Confirmation appears |
| 3 | Confirm | New job created or existing retried |
| 4 | Check new job status | Running/completed |

### Test J6: Job Progress Tracking

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Upload multi-version provider | Job created |
| 2 | Watch progress bar | Updates as items complete |
| 3 | Verify counts | Completed/Failed/Total accurate |
| 4 | Check "Started" time | Shows actual start time, not "Not Started" |

---

## Terraform Client Integration Tests

### Test T1: Provider Discovery

```bash
# Create Terraform CLI config
cat > ~/.terraformrc << 'EOF'
provider_installation {
  network_mirror {
    url = "https://localhost:8443/"
  }
}
EOF

# Create test configuration
mkdir -p tf-test && cd tf-test
cat > main.tf << 'EOF'
terraform {
  required_providers {
    random = {
      source  = "hashicorp/random"
      version = "3.5.0"
    }
  }
}

resource "random_id" "test" {
  byte_length = 8
}
EOF
```

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Run `terraform init` | Provider downloaded from mirror |
| 2 | Check output | Shows mirror URL being used |
| 3 | Verify provider cached | `.terraform/providers/` has files |

### Test T2: Module Discovery

```bash
cat > main.tf << 'EOF'
module "vpc" {
  source  = "localhost:8443/terraform-aws-modules/vpc/aws"
  version = "5.0.0"
}
EOF
```

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Run `terraform init` | Module downloaded from mirror |
| 2 | Check `.terraform/modules/` | Module files present |

### Test T3: Auto-Download Provider (if enabled)

**Preconditions:** Auto-download enabled in config

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Request a provider not in cache | - |
| 2 | Run `terraform init` | Provider auto-downloaded |
| 3 | Check admin UI | New provider appears in list |
| 4 | Subsequent requests | Served from cache |

### Test T4: Auto-Download Module (if enabled)

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Request uncached module | - |
| 2 | Run `terraform init` | Module auto-downloaded |
| 3 | Check admin UI | Module appears in list |

---

## API Tests

Use curl or Postman for these tests.

### Test API1: Login API

```bash
curl -k -X POST https://localhost:8443/admin/api/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"YourSecurePassword123!"}'
```

| Test | Expected |
|------|----------|
| Valid credentials | 200 OK with `{"token":"...","expires_at":"..."}` |
| Invalid credentials | 401 Unauthorized |
| Missing fields | 400 Bad Request |

### Test API2: Protected Endpoints

```bash
# Get token first
TOKEN=$(curl -sk -X POST https://localhost:8443/admin/api/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"YourSecurePassword123!"}' | jq -r .token)

# Use token for authenticated requests
curl -k https://localhost:8443/admin/api/providers \
  -H "Authorization: Bearer $TOKEN"
```

| Endpoint | Without Token | With Token |
|----------|---------------|------------|
| GET /admin/api/providers | 401 | 200 + provider list |
| GET /admin/api/modules | 401 | 200 + module list |
| GET /admin/api/jobs | 401 | 200 + job list |
| GET /admin/api/stats/storage | 401 | 200 + storage stats |
| GET /admin/api/config | 401 | 200 + config (sanitized) |

### Test API3: Provider CRUD

```bash
# List providers
curl -k -H "Authorization: Bearer $TOKEN" \
  https://localhost:8443/admin/api/providers

# Get single provider
curl -k -H "Authorization: Bearer $TOKEN" \
  https://localhost:8443/admin/api/providers/1

# Update provider (deprecate)
curl -k -X PUT -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"deprecated":true}' \
  https://localhost:8443/admin/api/providers/1

# Delete provider
curl -k -X DELETE -H "Authorization: Bearer $TOKEN" \
  https://localhost:8443/admin/api/providers/1
```

### Test API4: Storage Statistics

```bash
curl -k -H "Authorization: Bearer $TOKEN" \
  https://localhost:8443/admin/api/stats/storage
```

Expected response:
```json
{
  "total_size": 12345678,
  "total_size_formatted": "11.77 MB",
  "provider_count": 5,
  "module_count": 3
}
```

---

## UI/UX Tests

### Test U1: Responsive Design

| Screen Size | Test | Expected Result |
|-------------|------|-----------------|
| Desktop (1920x1080) | Navigate all pages | Full layout, sidebar visible |
| Tablet (768x1024) | Navigate all pages | Sidebar may collapse, tables readable |
| Mobile (375x667) | Navigate all pages | Mobile-friendly layout, stacked elements |

### Test U2: Loading States

| Action | Expected |
|--------|----------|
| Navigate to Providers | Loading spinner while fetching |
| Upload file | Progress indicator |
| Submit form | Button disabled during submission |

### Test U3: Error States

| Scenario | Expected |
|----------|----------|
| Network error | Error message displayed |
| 404 page | Friendly "not found" message |
| Server error | Generic error with retry option |

### Test U4: Toast Notifications

| Action | Expected |
|--------|----------|
| Successful upload | Green success toast |
| Delete provider | Success confirmation toast |
| Error occurs | Red error toast |

### Test U5: Modal Behavior

| Test | Expected |
|------|----------|
| Click outside modal | Modal closes |
| Press Escape | Modal closes |
| Click X button | Modal closes |
| Submit form in modal | Modal closes on success |

### Test U6: Navigation

| Test | Expected |
|------|----------|
| Sidebar links | Navigate to correct pages |
| Browser back/forward | History works correctly |
| Direct URL access | Pages load correctly |

---

## Performance Tests

### Test PF1: Page Load Times

| Page | Target | How to Measure |
|------|--------|----------------|
| Home | < 2s | Browser DevTools Network tab |
| Providers list | < 3s | Time from navigation to data displayed |
| Jobs list | < 2s | Including auto-refresh setup |

### Test PF2: Large Data Sets

| Test | Steps | Expected |
|------|-------|----------|
| 100+ providers | Upload multiple HCL files | Pagination works, no UI freeze |
| 50+ jobs | Create many jobs | List renders smoothly |
| Large file upload | Upload 10MB HCL | Progress shown, no timeout |

### Test PF3: Concurrent Users

| Test | Steps | Expected |
|------|-------|----------|
| 5 simultaneous logins | Use different browsers/incognito | All sessions work |
| Concurrent downloads | Multiple terraform init | All succeed |

---

## Error Handling Tests

### Test E1: Network Disconnection

| Step | Action | Expected |
|------|--------|----------|
| 1 | Login successfully | Dashboard shows |
| 2 | Disconnect network | - |
| 3 | Try to navigate | Error message shown |
| 4 | Reconnect | Retry works |

### Test E2: Invalid File Types

| Test | Expected |
|------|----------|
| Upload .txt instead of .hcl | Error: Invalid file type |
| Upload binary file | Error: Cannot parse |

### Test E3: Expired Session

| Step | Action | Expected |
|------|--------|----------|
| 1 | Login | Session created |
| 2 | Wait for expiration (or invalidate via API) | - |
| 3 | Try to access protected page | Redirected to login |

### Test E4: Database Errors

| Test | Steps | Expected |
|------|-------|----------|
| Database locked | Simulate heavy concurrent writes | Operations queue, don't fail |

---

## Cleanup Procedures

### Reset Test Environment

```bash
# Stop all containers
docker-compose -f deployments/docker-compose/docker-compose.local.yml down -v

# Remove database file (local dev)
rm -f terraform-mirror-dev.db

# Clear MinIO data
docker volume rm docker-compose_minio-data

# Restart fresh
docker-compose -f deployments/docker-compose/docker-compose.local.yml up -d
```

### Clear Browser Data

1. Open browser DevTools (F12)
2. Application tab → Storage → Clear site data
3. Or use incognito/private window for clean tests

---

## Test Execution Checklist

Use this checklist to track test execution:

### Authentication
- [ ] A1: Successful Login
- [ ] A2: Failed Login - Invalid Credentials
- [ ] A3: Failed Login - Empty Fields
- [ ] A4: Session Persistence
- [ ] A5: Logout
- [ ] A6: Protected Route Access

### Provider Management
- [ ] P1: Upload Provider HCL File
- [ ] P2: View Provider Details
- [ ] P3: Filter Providers
- [ ] P4: Delete Provider
- [ ] P5: Deprecate Provider
- [ ] P6: Invalid HCL Upload
- [ ] P7: Empty Provider File

### Module Management
- [ ] M1: Upload Module HCL File
- [ ] M2: View Module Details
- [ ] M3: Filter Modules
- [ ] M4: Delete Module

### Public Browse
- [ ] B1: Browse Providers (Unauthenticated)
- [ ] B2: Browse Modules (Unauthenticated)
- [ ] B3: Home Page Navigation
- [ ] B4: Public API Endpoints

### Job Processing
- [ ] J1: View Jobs List
- [ ] J2: Job Auto-Refresh
- [ ] J3: View Job Details
- [ ] J4: Cancel Pending/Running Job
- [ ] J5: Retry Failed Job
- [ ] J6: Job Progress Tracking

### Terraform Integration
- [ ] T1: Provider Discovery
- [ ] T2: Module Discovery
- [ ] T3: Auto-Download Provider
- [ ] T4: Auto-Download Module

### API Tests
- [ ] API1: Login API
- [ ] API2: Protected Endpoints
- [ ] API3: Provider CRUD
- [ ] API4: Storage Statistics

### UI/UX
- [ ] U1: Responsive Design
- [ ] U2: Loading States
- [ ] U3: Error States
- [ ] U4: Toast Notifications
- [ ] U5: Modal Behavior
- [ ] U6: Navigation

### Performance
- [ ] PF1: Page Load Times
- [ ] PF2: Large Data Sets
- [ ] PF3: Concurrent Users

### Error Handling
- [ ] E1: Network Disconnection
- [ ] E2: Invalid File Types
- [ ] E3: Expired Session
- [ ] E4: Database Errors

---

## Reporting Issues

When reporting issues found during testing:

1. **Test ID**: Reference the test number (e.g., P3, J2)
2. **Environment**: Docker/Local, browser, OS
3. **Steps to Reproduce**: Exact steps taken
4. **Expected Result**: What should happen
5. **Actual Result**: What actually happened
6. **Screenshots/Logs**: Include relevant evidence
7. **Severity**: Critical/High/Medium/Low

Example:
```
Test ID: J6
Environment: Docker Compose, Chrome 120, macOS
Steps: 1. Upload multi-version provider, 2. Watch progress
Expected: Progress bar updates in real-time
Actual: Progress bar stuck at 0% until completion
Severity: Medium
```
