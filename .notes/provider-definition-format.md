# Provider Definition File Format

## Overview

Provider definition files allow administrators to specify which Terraform providers should be pre-loaded into the mirror. The files use HCL (HashiCorp Configuration Language) format.

## File Format

```hcl
# Provider definition for hashicorp/aws
provider "hashicorp/aws" {
  versions = ["5.0.0", "5.1.0", "5.2.0"]
  platforms = [
    "linux_amd64",
    "linux_arm64",
    "darwin_amd64",
    "darwin_arm64",
    "windows_amd64"
  ]
}

# Provider definition for hashicorp/azurerm
provider "hashicorp/azurerm" {
  versions = ["3.0.0", "3.1.0"]
  platforms = ["linux_amd64", "darwin_amd64"]
}

# Support for version ranges (future enhancement)
provider "hashicorp/random" {
  version_constraint = "~> 3.5"  # Will download latest 3.5.x
  platforms = ["linux_amd64"]
}
```

## Schema

### `provider` block

Each `provider` block defines a single provider to download.

**Block Label**: The provider source address in the format `{namespace}/{type}`
- Example: `"hashicorp/aws"`, `"terraform-aws-modules/vpc"`

**Arguments**:

- `versions` (list of strings, required for Phase 1)
  - Explicit list of version numbers to download
  - Each version must be a valid semantic version
  - Example: `["5.0.0", "5.1.0"]`

- `platforms` (list of strings, required)
  - List of OS/architecture combinations to download
  - Format: `"{os}_{arch}"`
  - Valid OS values: `linux`, `darwin`, `windows`, `freebsd`
  - Valid arch values: `amd64`, `arm64`, `386`, `arm`
  - Example: `["linux_amd64", "darwin_arm64"]`

- `version_constraint` (string, future enhancement)
  - Semantic version constraint
  - Not implemented in Phase 1
  - Examples: `"~> 3.5"`, `">= 2.0, < 3.0"`

## Usage

### Admin API Endpoint

```http
POST /admin/api/providers/load
Content-Type: multipart/form-data

{
  "file": <HCL file content>,
  "overwrite": false  // Optional: overwrite existing providers
}
```

### CLI Tool (future)

```bash
terraform-mirror provider load -file providers.hcl
```

## Implementation Notes

### Phase 1 (Current)
- Parse HCL provider definitions
- Create download jobs for each provider/version/platform combination
- Download from registry.terraform.io
- Calculate SHA256 checksums
- Store in S3 and database
- **NO GPG verification** (deferred to Phase 2)

### Phase 2 (Future)
- Add GPG signature verification
- Support version constraints
- Auto-download on first request

### Example Job Processing

For this definition:
```hcl
provider "hashicorp/aws" {
  versions = ["5.0.0", "5.1.0"]
  platforms = ["linux_amd64", "darwin_amd64"]
}
```

The system creates **1 job** with **4 items**:
1. hashicorp/aws 5.0.0 linux_amd64
2. hashicorp/aws 5.0.0 darwin_amd64
3. hashicorp/aws 5.1.0 linux_amd64
4. hashicorp/aws 5.1.0 darwin_amd64

Each item is downloaded, verified, and stored independently.

## Validation Rules

1. Provider source must be in `{namespace}/{type}` format
2. At least one version must be specified
3. At least one platform must be specified
4. Versions must be valid semantic versions (e.g., `1.2.3`)
5. Platforms must be in `{os}_{arch}` format
6. No duplicate provider blocks (same namespace/type)

## Error Handling

- Invalid HCL syntax → 400 Bad Request with parse error
- Missing required fields → 400 Bad Request with validation error
- Duplicate providers → 400 Bad Request
- Download failures → Job item marked as failed, job continues
- Network errors → Retry with exponential backoff (up to 3 attempts)
