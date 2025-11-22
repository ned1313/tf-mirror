# Admin API: Provider Loading

## Endpoint

**POST** `/admin/api/providers/load`

Load providers into the mirror from an HCL definition file.

## Authentication

🚧 **Note**: Authentication is not yet implemented. In production, this endpoint will require admin authentication via JWT token.

## Request

### Content-Type
`multipart/form-data`

### Form Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `file` | File | Yes | HCL file containing provider definitions |

### HCL File Format

See [examples/providers.hcl](../examples/providers.hcl) for a complete example.

```hcl
provider "namespace/type" {
  versions  = ["X.Y.Z", ...]
  platforms = ["os_arch", ...]
}
```

**Example:**
```hcl
provider "hashicorp/aws" {
  versions = ["5.0.0", "5.1.0"]
  platforms = ["linux_amd64", "darwin_amd64"]
}
```

### Validation Rules

1. **Provider Source**: Must be in `namespace/type` format
   - Namespace: lowercase, alphanumeric, hyphens allowed
   - Type: lowercase, alphanumeric, hyphens allowed

2. **Versions**: Must be valid semantic versions
   - Format: `MAJOR.MINOR.PATCH[-PRERELEASE][+BUILD]`
   - Examples: `1.0.0`, `2.1.3-beta.1`, `3.0.0+20130313144700`

3. **Platforms**: Must be in `os_arch` format
   - Valid OS: `linux`, `darwin`, `windows`, `freebsd`, `openbsd`, `solaris`
   - Valid Arch: `amd64`, `arm64`, `386`, `arm`
   - Examples: `linux_amd64`, `darwin_arm64`, `windows_386`

4. **File Size**: Maximum 1MB

## Response

### Success Response (200 OK)

```json
{
  "message": "Provider loading completed: 4 total, 3 successful, 1 failed, 0 skipped",
  "stats": {
    "total": 4,
    "success": 3,
    "failed": 1,
    "skipped": 0,
    "errors": [
      "hashicorp/nonexistent 1.0.0 linux_amd64: download failed: provider not found"
    ]
  },
  "results": [
    {
      "namespace": "hashicorp",
      "type": "aws",
      "version": "5.0.0",
      "platform": "linux_amd64",
      "success": true,
      "skipped": false,
      "error": null
    },
    // ... more results
  ]
}
```

### Error Responses

#### 400 Bad Request - Invalid Form Data
```json
{
  "error": "invalid_form",
  "message": "Failed to parse form data: ..."
}
```

#### 400 Bad Request - Missing File
```json
{
  "error": "missing_file",
  "message": "No file uploaded: ..."
}
```

#### 400 Bad Request - File Too Large
```json
{
  "error": "file_too_large",
  "message": "File too large (max 1MB, got 2048576 bytes)"
}
```

#### 400 Bad Request - Parse Error
```json
{
  "error": "parse_error",
  "message": "Failed to parse HCL: ..."
}
```

#### 400 Bad Request - No Providers
```json
{
  "error": "no_providers",
  "message": "No providers defined in file"
}
```

#### 500 Internal Server Error - Load Error
```json
{
  "error": "load_error",
  "message": "Failed to load providers: ..."
}
```

## Behavior

1. **Parsing**: The HCL file is parsed and validated
2. **Downloading**: For each provider/version/platform:
   - Check if already exists in database (skip if found)
   - Download from `registry.terraform.io`
   - Verify SHA256 checksum
   - Retry up to 3 times with exponential backoff
3. **Storage**: Upload to S3 storage with key: `providers/{namespace}/{type}/{version}/{platform}/{filename}`
4. **Database**: Store metadata (namespace, type, version, platform, shasum, S3 key)
5. **Cleanup**: On failure, delete uploaded S3 objects
6. **Statistics**: Return counts of total, successful, failed, and skipped providers

## Example Usage

### Using curl

```bash
curl -X POST http://localhost:8080/admin/api/providers/load \
  -F "file=@examples/providers.hcl"
```

### Using httpie

```bash
http -f POST http://localhost:8080/admin/api/providers/load \
  file@examples/providers.hcl
```

### Using JavaScript (fetch)

```javascript
const formData = new FormData();
formData.append('file', fileInput.files[0]);

const response = await fetch('http://localhost:8080/admin/api/providers/load', {
  method: 'POST',
  body: formData
});

const result = await response.json();
console.log(result);
```

## Notes

- **Synchronous**: The endpoint waits for all providers to download before responding
- **Existing Providers**: Already cached providers are skipped (not re-downloaded)
- **Partial Success**: If some providers fail, the endpoint still returns 200 OK with error details in the stats
- **GPG Verification**: Not implemented in Phase 1 (deferred to Phase 2)
- **Job System**: Phase 1 uses synchronous processing; async job processing will be added later

## Future Enhancements (Phase 2+)

- Asynchronous job processing
- Job status tracking
- Progress updates via WebSocket
- GPG signature verification
- Rate limiting
- Authentication/authorization
