# Terraform Provider Network Mirror Protocol Tests

Based on: https://developer.hashicorp.com/terraform/internals/provider-network-mirror-protocol

## Protocol Overview

The Provider Network Mirror Protocol requires 2 main endpoints:

1. **List Available Versions** - Returns all versions for a provider
2. **List Available Installation Packages** - Returns download info for a specific version

Unlike the Provider Registry Protocol, there is NO service discovery endpoint (`.well-known/terraform.json`).
However, our implementation includes service discovery since we also implement registry protocol features.

## Endpoint Specifications

### 1. Service Discovery (Optional - Not in Mirror Protocol)

**Endpoint:** `GET /.well-known/terraform.json`  
**Content-Type:** `application/json`  
**Status:** Should we implement this? It's not part of the mirror protocol.

**Current Implementation:**
```bash
curl http://localhost:8080/.well-known/terraform.json
```

**Expected Response:**
```json
{
  "providers.v1": "/v1/providers/"
}
```

**Notes:** 
- This is from the Provider Registry Protocol, not the Mirror Protocol
- Mirror protocol doesn't use service discovery
- Our implementation has both registry and mirror features

---

### 2. List Available Versions

**Endpoint:** `GET /:hostname/:namespace/:type/index.json`  
**Content-Type:** `application/json`

**Parameters:**
- `hostname` - Provider hostname (e.g., `registry.terraform.io`)
- `namespace` - Provider namespace (e.g., `hashicorp`)
- `type` - Provider type (e.g., `random`)

**Example Request:**
```bash
curl http://localhost:8080/registry.terraform.io/hashicorp/random/index.json
```

**Expected Response (Success):**
```json
{
  "versions": {
    "3.5.0": {},
    "3.4.3": {},
    "3.4.2": {}
  }
}
```

**Expected Response (Not Found):**
```json
HTTP 404 Not Found
```

**Notes:**
- Version property values should be empty objects `{}`
- This allows forward compatibility
- Must return 404 if provider not found

---

### 3. List Available Installation Packages

**Endpoint:** `GET /:hostname/:namespace/:type/:version.json`  
**Content-Type:** `application/json`

**Parameters:**
- `hostname` - Provider hostname (e.g., `registry.terraform.io`)
- `namespace` - Provider namespace (e.g., `hashicorp`)
- `type` - Provider type (e.g., `random`)
- `version` - Exact version string from index.json (e.g., `3.5.0`)

**Example Request:**
```bash
curl http://localhost:8080/registry.terraform.io/hashicorp/random/3.5.0.json
```

**Expected Response:**
```json
{
  "archives": {
    "darwin_amd64": {
      "url": "terraform-provider-random_3.5.0_darwin_amd64.zip",
      "hashes": [
        "h1:4A07+ZFc2wgJwo8YNlQpr1rVlgUDlxXHhPJciaPY5gs="
      ]
    },
    "linux_amd64": {
      "url": "terraform-provider-random_3.5.0_linux_amd64.zip",
      "hashes": [
        "h1:lCJCxf/LIowc2IGS9TPjWDyXY4nOmdGdfcwwDQCOURQ="
      ]
    },
    "windows_amd64": {
      "url": "terraform-provider-random_3.5.0_windows_amd64.zip",
      "hashes": [
        "h1:..."
      ]
    }
  }
}
```

**Response Properties:**
- `archives` - Object with platform keys (e.g., `linux_amd64`)
- `url` - Download URL (can be relative or absolute)
- `hashes` - Optional array of hash strings using Terraform's format

**Notes:**
- URL can be relative (resolved against current JSON URL)
- Hashes are optional but recommended for verification
- Platform format: `{os}_{arch}` (e.g., `linux_amd64`, `darwin_arm64`)
- Terraform will verify against strongest hash algorithm if provided

---

## Our Current Implementation Status

Based on server routes, we implement:

### Mirror Protocol Routes (Should Implement)
- ❓ `GET /:hostname/:namespace/:type/index.json` - List versions
- ❓ `GET /:hostname/:namespace/:type/:version.json` - Installation packages

### Registry Protocol Routes (Currently Implemented)
- ✅ `GET /.well-known/terraform.json` - Service discovery
- ✅ `GET /v1/providers/{namespace}/{type}/versions` - List versions
- ✅ `GET /v1/providers/{namespace}/{type}/{version}/download/{os}/{arch}` - Download

### Admin API Routes (Currently Implemented)
- ✅ `POST /admin/api/providers/load` - Load from HCL
- ✅ `GET /admin/api/providers` - List providers
- ❓ Other admin endpoints

## Test Plan

### Test 1: Service Discovery
```bash
curl http://localhost:8080/.well-known/terraform.json
```
**Expected:** JSON with `providers.v1` endpoint

---

### Test 2: List Versions (Registry Protocol)
```bash
curl http://localhost:8080/v1/providers/hashicorp/random/versions
```
**Expected:** JSON with versions array

---

### Test 3: Download Package (Registry Protocol)
```bash
curl http://localhost:8080/v1/providers/hashicorp/random/3.5.0/download/linux/amd64
```
**Expected:** JSON with download URL and metadata

---

### Test 4: List Versions (Mirror Protocol) - NOT IMPLEMENTED YET
```bash
curl http://localhost:8080/registry.terraform.io/hashicorp/random/index.json
```
**Expected:** 404 (not implemented) or JSON with versions object

---

### Test 5: Installation Packages (Mirror Protocol) - NOT IMPLEMENTED YET
```bash
curl http://localhost:8080/registry.terraform.io/hashicorp/random/3.5.0.json
```
**Expected:** 404 (not implemented) or JSON with archives object

---

## Key Differences: Mirror vs Registry Protocol

| Feature | Mirror Protocol | Registry Protocol | Our Implementation |
|---------|----------------|-------------------|-------------------|
| Service Discovery | ❌ Not used | ✅ Required | ✅ Implemented |
| URL Pattern | `/:hostname/:namespace/:type/...` | `/v1/providers/:namespace/:type/...` | Registry only |
| List Versions | `index.json` format | `versions` array | Registry format |
| Download Info | `{version}.json` with archives | `download` endpoint with redirect | Registry format |
| Authentication | Optional on base URL | Optional on base URL | Not implemented |
| Hostname in URL | ✅ Required in path | ❌ Not in path | Not in registry |

## Recommendations

1. **Decide on Protocol Support:**
   - Are we implementing Mirror Protocol or Registry Protocol?
   - Current implementation is Registry Protocol only
   - Mirror protocol would need different routes

2. **If Supporting Mirror Protocol:**
   - Add route: `GET /:hostname/:namespace/:type/index.json`
   - Add route: `GET /:hostname/:namespace/:type/:version.json`
   - Respond with correct JSON format

3. **If Supporting Only Registry Protocol:**
   - Remove service discovery endpoint (not needed for mirror)
   - OR keep it for flexibility
   - Document which protocol we implement

4. **Current State:**
   - We implement Registry Protocol ✅
   - We do NOT implement Mirror Protocol ❌
   - Our routes use `/v1/providers/` pattern (registry)
   - Not compatible with mirror protocol clients
