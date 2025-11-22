# Example Provider Definition File for Terraform Mirror
#
# This file defines which providers should be loaded into the mirror.
# Upload this file to the admin API endpoint: POST /admin/api/providers/load
#
# Format: HCL (HashiCorp Configuration Language)
#
# Each provider block specifies:
# - source: namespace/type (e.g., hashicorp/aws)
# - versions: list of semantic versions to cache
# - platforms: list of os_arch combinations (e.g., linux_amd64)

# AWS Provider - Multiple versions and platforms
provider "hashicorp/aws" {
  versions = [
    "5.0.0",
    "5.1.0",
    "5.2.0"
  ]
  platforms = [
    "linux_amd64",
    "linux_arm64",
    "darwin_amd64",
    "darwin_arm64",
    "windows_amd64"
  ]
}

# Random Provider - Latest version
provider "hashicorp/random" {
  versions = ["3.5.0"]
  platforms = [
    "linux_amd64",
    "darwin_amd64",
    "windows_amd64"
  ]
}

# Null Provider - For testing
provider "hashicorp/null" {
  versions = ["3.2.0"]
  platforms = ["linux_amd64"]
}

# Example of a third-party provider
provider "integrations/github" {
  versions = ["5.40.0"]
  platforms = [
    "linux_amd64",
    "darwin_amd64"
  ]
}

# Notes:
# - Versions must be valid semantic versions (X.Y.Z)
# - Platforms must be in os_arch format
# - Valid OS: linux, darwin, windows, freebsd, openbsd, solaris
# - Valid architectures: amd64, arm64, 386, arm
# - The system will:
#   1. Download each provider/version/platform combination
#   2. Verify SHA256 checksums
#   3. Upload to S3 storage
#   4. Store metadata in database
#   5. Skip providers that already exist
