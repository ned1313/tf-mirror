# User Guide

This guide covers day-to-day usage of Terraform Mirror for both administrators and Terraform users.

## Table of Contents

- [Overview](#overview)
- [For Terraform Users](#for-terraform-users)
  - [Configuring Terraform](#configuring-terraform)
  - [Using the Mirror](#using-the-mirror)
  - [Troubleshooting](#troubleshooting)
- [For Administrators](#for-administrators)
  - [Web UI Overview](#web-ui-overview)
  - [Loading Providers](#loading-providers)
  - [Managing Providers](#managing-providers)
  - [Monitoring Jobs](#monitoring-jobs)
  - [Viewing Statistics](#viewing-statistics)
  - [Audit Logs](#audit-logs)
  - [Backup and Maintenance](#backup-and-maintenance)

---

## Overview

Terraform Mirror acts as a caching proxy between your Terraform clients and the public Terraform Registry. It provides:

- **Offline Access**: Use Terraform in air-gapped environments
- **Faster Downloads**: Cached providers download quickly from local storage
- **Bandwidth Savings**: Multiple teams share a single cached copy
- **Version Control**: Explicitly manage which provider versions are available
- **Audit Trail**: Track all provider downloads and administrative actions

---

## For Terraform Users

### Configuring Terraform

To use Terraform Mirror, you need to configure Terraform to use it as a network mirror.

#### Global Configuration

Create or edit `~/.terraformrc` (Linux/macOS) or `%APPDATA%\terraform.rc` (Windows):

```hcl
provider_installation {
  network_mirror {
    url = "http://your-mirror-host:8080/"
  }
}
```

#### Per-Project Configuration

For project-specific configuration, create a file (e.g., `.terraformrc`) in your project:

```hcl
provider_installation {
  network_mirror {
    url = "http://your-mirror-host:8080/"
  }
}
```

Then set the environment variable before running Terraform:

```bash
export TF_CLI_CONFIG_FILE=.terraformrc
terraform init
```

#### Mixed Mode (Mirror + Direct)

To fall back to the public registry if a provider isn't in the mirror:

```hcl
provider_installation {
  network_mirror {
    url = "http://your-mirror-host:8080/"
  }
  
  direct {
    # Fallback to public registry
  }
}
```

#### Air-Gapped Mode (Mirror Only)

For fully air-gapped environments, remove the `direct` block:

```hcl
provider_installation {
  network_mirror {
    url = "http://your-mirror-host:8080/"
  }
  # No direct block = no fallback
}
```

### Using the Mirror

Once configured, use Terraform normally:

```bash
# Initialize - providers download from mirror
terraform init

# Plan and apply work as normal
terraform plan
terraform apply
```

#### Verify Mirror Usage

To confirm Terraform is using the mirror:

```bash
# Enable debug logging
export TF_LOG=DEBUG
terraform init

# Look for lines like:
# Finding hashicorp/aws versions matching "~> 5.0"...
# provider.address=hashicorp/aws url=http://your-mirror:8080/v1/providers/hashicorp/aws/versions
```

#### Check Available Providers

Query the mirror directly to see available providers:

```bash
# List versions of AWS provider
curl http://your-mirror-host:8080/v1/providers/hashicorp/aws/versions

# Response:
# {"versions": {"5.31.0": {}, "5.30.0": {}}}
```

### Troubleshooting

#### Provider Not Found

**Error:** `Error: Failed to query available provider packages`

**Cause:** The provider/version isn't loaded in the mirror.

**Solution:**
1. Contact your administrator to load the required provider
2. Provide the exact provider source and version needed
3. If using mixed mode, ensure the `direct` block is present

#### Connection Refused

**Error:** `Error: Failed to install provider`

**Cause:** Cannot reach the mirror server.

**Solution:**
1. Verify the mirror URL is correct
2. Check network connectivity: `curl http://your-mirror-host:8080/health`
3. Check firewall rules

#### Certificate Errors

**Error:** `x509: certificate signed by unknown authority`

**Cause:** HTTPS with self-signed certificate.

**Solution:**
1. Add the CA certificate to your system trust store
2. Or use HTTP (not recommended for production)
3. Or set `TF_SKIP_PROVIDER_VERIFY=true` (not recommended)

#### Cache Issues

If you're getting stale results, Terraform may be caching provider information:

```bash
# Clear Terraform plugin cache
rm -rf ~/.terraform.d/plugins
rm -rf .terraform

# Re-initialize
terraform init -upgrade
```

---

## For Administrators

### Web UI Overview

Access the admin UI at `http://your-mirror-host:8080/admin/`

#### Dashboard

The dashboard provides an overview of:
- Total providers and storage usage
- Recent jobs and their status
- Cache statistics and hit rates
- System health

#### Navigation

- **Providers**: View, search, and manage loaded providers
- **Jobs**: Monitor download jobs and their progress
- **Audit Logs**: View all administrative actions
- **Settings**: View configuration (read-only)

### Loading Providers

Providers must be explicitly loaded before they're available to Terraform clients.

#### Using the Web UI

1. Navigate to **Providers** → **Load Providers**
2. Create an HCL file defining the providers you need:

```hcl
provider "hashicorp/aws" {
  versions  = ["5.31.0", "5.30.0", "5.29.0"]
  platforms = ["linux_amd64", "darwin_arm64", "windows_amd64"]
}

provider "hashicorp/azurerm" {
  versions  = ["3.84.0", "3.83.0"]
  platforms = ["linux_amd64", "darwin_arm64"]
}

provider "hashicorp/google" {
  versions  = ["5.10.0"]
  platforms = ["linux_amd64"]
}
```

3. Upload the file
4. Monitor the job progress

#### Using the API

```bash
# Login
TOKEN=$(curl -s -X POST http://localhost:8080/admin/api/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "password"}' | jq -r '.token')

# Upload providers
curl -X POST http://localhost:8080/admin/api/providers/load \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@providers.hcl"
```

#### Provider Definition Format

```hcl
provider "<namespace>/<type>" {
  versions  = ["<version1>", "<version2>", ...]
  platforms = ["<os>_<arch>", ...]
}
```

**Common Platforms:**
- `linux_amd64` - Linux 64-bit (most common for CI/CD)
- `linux_arm64` - Linux ARM64 (AWS Graviton, etc.)
- `darwin_amd64` - macOS Intel
- `darwin_arm64` - macOS Apple Silicon
- `windows_amd64` - Windows 64-bit

**Tips:**
- Only load platforms your team actually uses
- Start with recent versions and add older ones as needed
- Group related providers in a single file

### Managing Providers

#### Viewing Providers

The Providers page shows all loaded providers with:
- Namespace, type, version, platform
- File size
- Status (active, deprecated, blocked)
- Load date

Use filters to find specific providers:
- Filter by namespace (e.g., `hashicorp`)
- Filter by type (e.g., `aws`)
- Search by version

#### Deprecating Providers

Mark old versions as deprecated to discourage use:

1. Find the provider in the list
2. Click to open details
3. Toggle "Deprecated" status
4. Save changes

Deprecated providers still work but may show warnings in the UI.

#### Blocking Providers

Block problematic versions to prevent download:

1. Find the provider
2. Toggle "Blocked" status
3. Save changes

Blocked providers return a 404 to Terraform clients.

#### Deleting Providers

Remove providers to free storage:

1. Find the provider
2. Click Delete
3. Confirm deletion

**Warning:** This removes the provider from storage. Terraform clients will no longer be able to download it.

### Monitoring Jobs

#### Job States

| State | Description |
|-------|-------------|
| **Pending** | Job created, waiting to start |
| **Running** | Actively downloading providers |
| **Completed** | All items processed successfully |
| **Failed** | One or more items failed |
| **Cancelled** | Job was cancelled by user |

#### Viewing Job Details

Click on a job to see:
- Overall progress percentage
- List of all items (provider/version/platform)
- Status of each item
- Error messages for failed items

#### Retrying Failed Jobs

If some items failed (e.g., network timeout):

1. Open the job details
2. Click "Retry Failed"
3. Failed items are reset to pending and reprocessed

#### Cancelling Jobs

To stop a running job:

1. Open the job details
2. Click "Cancel"
3. Job stops processing new items

Already-downloaded providers remain available.

### Viewing Statistics

#### Storage Statistics

View storage usage:
- Total providers count
- Total storage size
- Breakdown by namespace
- Deprecated/blocked counts

#### Cache Statistics

Monitor cache performance:
- Hit rate (higher is better, aim for >80%)
- Memory usage vs. limit
- Disk usage vs. limit
- Items in cache
- Evictions and expirations

**Improving Cache Performance:**
- Increase memory cache for frequently accessed providers
- Increase disk cache for large provider sets
- Monitor eviction rate - high evictions indicate cache too small

#### Clearing the Cache

To force fresh downloads:

1. Navigate to Statistics → Cache
2. Click "Clear Cache"
3. Confirm

**Note:** This doesn't delete providers from storage, only the response cache.

### Audit Logs

All administrative actions are logged:

- Login/logout events
- Provider loading
- Provider updates and deletions
- Job retries and cancellations
- Backup triggers

#### Viewing Logs

Navigate to **Audit Logs** to see:
- Timestamp
- User who performed action
- Action type
- Target resource
- Success/failure status
- IP address

#### Filtering Logs

Filter by:
- Action type (login, load_providers, etc.)
- Date range
- User
- Success/failure

### Backup and Maintenance

#### Database Backups

The SQLite database contains all metadata. Regular backups are important.

**Automatic Backups:**

Enable in configuration:

```hcl
database {
  backup_enabled        = true
  backup_interval_hours = 6
  backup_to_s3          = true
  backup_s3_prefix      = "backups/"
}
```

**Manual Backups:**

Via API:
```bash
curl -X POST http://localhost:8080/admin/api/backup \
  -H "Authorization: Bearer $TOKEN"
```

Via Web UI:
1. Navigate to Settings
2. Click "Trigger Backup"

#### Storage Maintenance

**Recalculate Statistics:**

If storage stats seem incorrect:

```bash
curl -X POST http://localhost:8080/admin/api/stats/recalculate \
  -H "Authorization: Bearer $TOKEN"
```

**Cleaning Up Old Providers:**

1. Identify deprecated/unused versions
2. Delete via UI or API
3. Storage is automatically reclaimed

#### Cache Maintenance

**Cache Warmup:**

After restart, the cache is cold. Pre-warm by accessing common providers:

```bash
# List your most-used providers
curl http://localhost:8080/v1/providers/hashicorp/aws/versions
curl http://localhost:8080/v1/providers/hashicorp/azurerm/versions
```

**Cache Clear:**

If responses seem stale or corrupted:

```bash
curl -X POST http://localhost:8080/admin/api/stats/cache/clear \
  -H "Authorization: Bearer $TOKEN"
```

---

## Best Practices

### For Organizations

1. **Centralize Provider Management**
   - Designate who can load/manage providers
   - Document required providers and versions
   - Review and clean up periodically

2. **Plan Storage Capacity**
   - Each provider version is 20-100+ MB
   - Plan for all platforms you support
   - AWS provider alone can be 500+ MB per version

3. **Monitor Cache Performance**
   - Target >80% hit rate
   - Size cache appropriately for your usage
   - Clear cache if performance degrades

4. **Implement Access Controls**
   - Change default admin password immediately
   - Use strong passwords
   - Review audit logs regularly

### For Terraform Users

1. **Be Specific with Versions**
   - Use exact versions when possible
   - Avoid broad version constraints
   - Coordinate with admins on needed versions

2. **Handle Mirror Outages**
   - Consider fallback configuration
   - Cache providers locally for critical workflows
   - Have offline procedures documented

3. **Report Issues**
   - Tell admins if providers are missing
   - Report slow downloads (may indicate cache issues)
   - Note any error messages for troubleshooting
