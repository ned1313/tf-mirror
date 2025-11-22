# Terraform Registry API

## Provider Download Flow

### 1. Get Provider Versions
```
GET https://registry.terraform.io/v1/providers/{namespace}/{type}/versions
```

Response:
```json
{
  "versions": [
    {
      "version": "5.0.0",
      "protocols": ["5.0"],
      "platforms": [
        {
          "os": "linux",
          "arch": "amd64"
        },
        {
          "os": "darwin",
          "arch": "amd64"
        }
      ]
    }
  ]
}
```

### 2. Get Download URL for Specific Platform
```
GET https://registry.terraform.io/v1/providers/{namespace}/{type}/{version}/download/{os}/{arch}
```

Response:
```json
{
  "protocols": ["5.0"],
  "os": "linux",
  "arch": "amd64",
  "filename": "terraform-provider-aws_5.0.0_linux_amd64.zip",
  "download_url": "https://releases.hashicorp.com/terraform-provider-aws/5.0.0/terraform-provider-aws_5.0.0_linux_amd64.zip",
  "shasums_url": "https://releases.hashicorp.com/terraform-provider-aws/5.0.0/terraform-provider-aws_5.0.0_SHA256SUMS",
  "shasums_signature_url": "https://releases.hashicorp.com/terraform-provider-aws/5.0.0/terraform-provider-aws_5.0.0_SHA256SUMS.sig",
  "shasum": "abc123...",
  "signing_keys": {
    "gpg_public_keys": [...]
  }
}
```

### 3. Download the Provider Binary
```
GET {download_url}
```

Returns: ZIP file containing the provider binary

### 4. Download SHA256SUMS (for verification)
```
GET {shasums_url}
```

Returns: Text file with checksums for all platforms

## Phase 1 Implementation (No GPG Verification)

For Phase 1, we:
1. ✅ Call versions API to verify provider exists
2. ✅ Call download API to get metadata and download URL
3. ✅ Download the provider ZIP file
4. ✅ Calculate SHA256 checksum of downloaded file
5. ✅ Verify checksum matches registry's shasum
6. ✅ Upload to S3 storage
7. ✅ Save metadata to database
8. ❌ Skip GPG signature verification (Phase 2)

## Error Handling

- 404: Provider/version/platform not found
- Network errors: Retry with exponential backoff (3 attempts)
- Checksum mismatch: Fail and report
- Storage errors: Fail and report
