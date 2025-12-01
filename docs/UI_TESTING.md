# Admin UI Testing Guide

## Quick Start

> **Important Configuration Notes:**
> - Always use the config file (`-config config.dev.hcl`) rather than environment variables
> - Environment variables use `TFM_` prefix (e.g., `TFM_DATABASE_PATH`), NOT `TF_MIRROR_`
> - JWT secret can only be set via config file, not environment variables
> - The `create-admin` command defaults to `terraform-mirror-dev.db`, so the server must use the same database

### 1. Start the Backend Server

```powershell
# From the project root
cd c:\gh\tf-mirror

# Create an admin user first (if not already created)
# Note: This uses terraform-mirror-dev.db by default
go run ./cmd/create-admin -username admin -password admin123

# If user already exists and you need to reset the password:
go run ./cmd/reset-password -username admin -password admin123

# Start the backend server with the dev config file
go run ./cmd/terraform-mirror -config config.dev.hcl
```

The backend will start on `http://localhost:8080` by default.

### 2. Start the Frontend Dev Server

```powershell
# In a new terminal
cd c:\gh\tf-mirror\web

# Install dependencies (if not done)
npm install

# Start development server with API proxy
npm run dev
```

The frontend dev server runs on `http://localhost:5173` and will proxy API requests to the backend.

### 3. Configure Vite Proxy (if needed)

The frontend needs to proxy `/admin/api` requests to the backend. Check/update `vite.config.ts`:

```typescript
export default defineConfig({
  // ... other config
  server: {
    proxy: {
      '/admin/api': {
        target: 'http://localhost:8080',
        changeOrigin: true
      }
    }
  }
})
```

---

## Testing Checklist

### Authentication
- [ ] **Login page renders** - Navigate to `/login`
- [ ] **Invalid credentials show error** - Try wrong username/password
- [ ] **Successful login redirects** - Login with `admin`/`admin123`
- [ ] **Auth token persists** - Refresh page, should stay logged in
- [ ] **Logout works** - Click logout, redirects to login

### Dashboard (Admin page)
- [ ] **Stats cards display** - Total providers, versions, storage size, jobs
- [ ] **Recent activity shows** - Audit log entries (may be empty initially)
- [ ] **Active jobs display** - Running/pending jobs (may be empty)
- [ ] **Navigation works** - Sidebar links navigate correctly
- [ ] **Quick stats in sidebar** - Provider count and storage size

### Providers Page
- [ ] **Provider list loads** - Shows empty state or provider list
- [ ] **Upload HCL button works** - Opens upload modal
- [ ] **Search/filter works** - Try searching if providers exist
- [ ] **Status filter works** - Filter by Active/Deprecated/Blocked
- [ ] **Namespace filter works** - Filter by namespace dropdown
- [ ] **Provider actions work** - View details, deprecate, block, delete
- [ ] **Pagination works** - If many providers, pagination appears

### Jobs Page
- [ ] **Jobs list loads** - Shows empty state or job list
- [ ] **Tab filters work** - All/Running/Pending/Completed/Failed
- [ ] **Job details modal** - Click "View Details" on a job
- [ ] **Retry failed jobs** - If failed jobs exist, retry button works
- [ ] **Progress bars** - Running jobs show progress

### Audit Logs Page
- [ ] **Logs list loads** - Should have login entries at minimum
- [ ] **Search works** - Search by action or resource
- [ ] **Action filter works** - Filter by specific actions
- [ ] **Date filters work** - Filter by date range
- [ ] **Clear filters** - Reset all filters

### Settings Page
- [ ] **Configuration displays** - Server settings, storage, etc.
- [ ] **Storage stats show** - Provider count, size
- [ ] **Backup button works** - Creates database backup
- [ ] **API info displays** - Base URL and Terraform config example

---

## Test Data Setup

To have meaningful data to test with, you can:

### 1. Create a test HCL file

Create `test-providers.hcl`:

```hcl
required_providers {
  aws = {
    source  = "hashicorp/aws"
    version = "5.0.0"
  }
  random = {
    source  = "hashicorp/random"
    version = "3.5.1"
  }
}
```

### 2. Upload via API (curl)

```bash
# Login first
TOKEN=$(curl -s -X POST http://localhost:8080/admin/api/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.token')

# Upload HCL file
curl -X POST http://localhost:8080/admin/api/providers/load \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: text/plain" \
  --data-binary @test-providers.hcl
```

### 3. Or upload via the UI

1. Go to Providers page
2. Click "Upload HCL"
3. Drag and drop or select the `test-providers.hcl` file
4. Click "Upload and Sync"

---

## Known Issues / Limitations

1. **No static file serving** - Backend doesn't serve the built frontend yet (need to run dev server separately)
2. **Processor auto-start** - Background job processor starts automatically, may need to check status
3. **S3 storage** - If using S3, ensure credentials are configured
4. **CORS** - May need to configure CORS if running on different ports

---

## Browser Console Checks

Open browser DevTools (F12) and check:

1. **Network tab** - API calls returning 200/201
2. **Console tab** - No JavaScript errors
3. **Application tab** - auth_token in localStorage after login

---

## Environment Variables Reference

| Variable | Description | Default |
|----------|-------------|---------|
| `TF_MIRROR_SERVER_PORT` | Server port | `8080` |
| `TF_MIRROR_DATABASE_PATH` | SQLite database path | `./terraform-mirror.db` |
| `TF_MIRROR_STORAGE_TYPE` | `filesystem` or `s3` | `filesystem` |
| `TF_MIRROR_STORAGE_LOCAL_PATH` | Local storage directory | `./storage` |
| `TF_MIRROR_AUTH_JWT_SECRET` | JWT signing secret | *required* |
| `TF_MIRROR_AUTH_JWT_EXPIRATION_HOURS` | Token expiration | `24` |

---

## Feedback Template

When testing, please note:

### What works well?
- 

### What doesn't work?
- 

### UI/UX suggestions?
- 

### Missing features?
- 

### Performance observations?
- 

### Error messages encountered?
- 
