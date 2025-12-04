# Installation Guide

This guide covers installing and running Terraform Mirror in various environments.

## Table of Contents

- [Requirements](#requirements)
- [Quick Start](#quick-start)
- [Docker Compose (Recommended)](#docker-compose-recommended)
- [Docker Standalone](#docker-standalone)
- [Binary Installation](#binary-installation)
- [Building from Source](#building-from-source)
- [Initial Setup](#initial-setup)
- [Configuring Terraform Clients](#configuring-terraform-clients)
- [Verification](#verification)

## Requirements

### Minimum Requirements

- **CPU**: 1 core
- **Memory**: 512 MB RAM
- **Storage**: 
  - 1 GB for application and database
  - Additional storage for cached providers (varies by usage)

### Recommended Production Requirements

- **CPU**: 2+ cores
- **Memory**: 2 GB RAM
- **Storage**:
  - SSD-backed storage for database
  - S3-compatible object storage for providers

### Software Dependencies

For Docker deployment:
- Docker 20.10+
- Docker Compose v2.0+ (optional, for Docker Compose deployment)

For binary installation:
- Linux (amd64, arm64), macOS (amd64, arm64), or Windows (amd64)

For building from source:
- Go 1.23+
- Node.js 18+ (for frontend)
- Make (optional)

## Quick Start

The fastest way to get started is with Docker Compose:

```bash
# Clone the repository
git clone https://github.com/ned1313/terraform-mirror.git
cd terraform-mirror

# Start with Docker Compose (includes MinIO for S3 storage)
docker-compose -f deployments/docker-compose/docker-compose.yml up -d

# Access the web UI
open http://localhost:8080
```

Default credentials (change immediately in production):
- **Username**: admin
- **Password**: changeme123

## Docker Compose (Recommended)

Docker Compose is the recommended deployment method as it includes all dependencies.

### Standard Deployment (with MinIO)

This setup includes Terraform Mirror and MinIO for S3-compatible storage:

```bash
cd terraform-mirror
docker-compose -f deployments/docker-compose/docker-compose.yml up -d
```

Services started:
- `terraform-mirror`: Main application on port 8080
- `minio`: S3-compatible storage on ports 9000 (API) and 9001 (Console)
- `minio-init`: Initializes the storage bucket

### Development Deployment

For development with additional debugging features:

```bash
docker-compose -f deployments/docker-compose/docker-compose.dev.yml up -d
```

### Local Filesystem Storage

For testing without S3 storage:

```bash
docker-compose -f deployments/docker-compose/docker-compose.local.yml up -d
```

### Custom Configuration

Create a `.env` file to customize the deployment:

```bash
# .env file
TFM_ADMIN_USERNAME=myadmin
TFM_ADMIN_PASSWORD=mysecurepassword
TFM_STORAGE_BUCKET=my-terraform-providers
TFM_CACHE_MEMORY_SIZE_MB=512
TFM_CACHE_DISK_SIZE_GB=20
```

Then run:

```bash
docker-compose -f deployments/docker-compose/docker-compose.yml --env-file .env up -d
```

### Connecting to External S3 (AWS)

To use AWS S3 instead of MinIO, create a custom `docker-compose.override.yml`:

```yaml
# docker-compose.override.yml
services:
  terraform-mirror:
    environment:
      - TFM_STORAGE_TYPE=s3
      - TFM_STORAGE_BUCKET=your-bucket-name
      - TFM_STORAGE_REGION=us-west-2
      - TFM_STORAGE_ACCESS_KEY=${AWS_ACCESS_KEY_ID}
      - TFM_STORAGE_SECRET_KEY=${AWS_SECRET_ACCESS_KEY}
      # Remove endpoint for real AWS S3
      - TFM_STORAGE_ENDPOINT=
    depends_on: []  # Remove MinIO dependency

  # Disable MinIO services
  minio:
    profiles: ["disabled"]
  minio-init:
    profiles: ["disabled"]
```

Run with:

```bash
docker-compose -f deployments/docker-compose/docker-compose.yml \
  -f docker-compose.override.yml up -d
```

## Docker Standalone

Run Terraform Mirror as a standalone container:

### With Local Storage

```bash
# Create data directories
mkdir -p ./data ./cache

# Run container
docker run -d \
  --name terraform-mirror \
  -p 8080:8080 \
  -v $(pwd)/data:/data \
  -v $(pwd)/cache:/var/cache/tf-mirror \
  -e TFM_STORAGE_TYPE=local \
  -e TFM_STORAGE_ENDPOINT=/data/storage \
  -e TFM_DATABASE_PATH=/data/terraform-mirror.db \
  -e TFM_ADMIN_USERNAME=admin \
  -e TFM_ADMIN_PASSWORD=changeme123 \
  terraform-mirror:latest
```

### With External S3

```bash
docker run -d \
  --name terraform-mirror \
  -p 8080:8080 \
  -v $(pwd)/data:/data \
  -e TFM_STORAGE_TYPE=s3 \
  -e TFM_STORAGE_BUCKET=my-terraform-bucket \
  -e TFM_STORAGE_REGION=us-west-2 \
  -e TFM_STORAGE_ACCESS_KEY=AKIAIOSFODNN7EXAMPLE \
  -e TFM_STORAGE_SECRET_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY \
  -e TFM_DATABASE_PATH=/data/terraform-mirror.db \
  -e TFM_ADMIN_USERNAME=admin \
  -e TFM_ADMIN_PASSWORD=changeme123 \
  terraform-mirror:latest
```

### With MinIO

```bash
# Start MinIO first
docker run -d \
  --name minio \
  -p 9000:9000 \
  -p 9001:9001 \
  -v minio-data:/data \
  -e MINIO_ROOT_USER=minioadmin \
  -e MINIO_ROOT_PASSWORD=minioadmin \
  minio/minio server /data --console-address ":9001"

# Create bucket
docker run --rm --link minio:minio minio/mc \
  alias set myminio http://minio:9000 minioadmin minioadmin && \
  mc mb myminio/terraform-mirror --ignore-existing

# Start Terraform Mirror
docker run -d \
  --name terraform-mirror \
  --link minio:minio \
  -p 8080:8080 \
  -v tfm-data:/data \
  -e TFM_STORAGE_TYPE=s3 \
  -e TFM_STORAGE_BUCKET=terraform-mirror \
  -e TFM_STORAGE_ENDPOINT=http://minio:9000 \
  -e TFM_STORAGE_ACCESS_KEY=minioadmin \
  -e TFM_STORAGE_SECRET_KEY=minioadmin \
  -e TFM_STORAGE_FORCE_PATH_STYLE=true \
  -e TFM_DATABASE_PATH=/data/terraform-mirror.db \
  -e TFM_ADMIN_USERNAME=admin \
  -e TFM_ADMIN_PASSWORD=changeme123 \
  terraform-mirror:latest
```

## Binary Installation

### Download Pre-built Binary

Download the latest release from the [releases page](https://github.com/ned1313/terraform-mirror/releases):

```bash
# Linux (amd64)
curl -LO https://github.com/ned1313/terraform-mirror/releases/latest/download/terraform-mirror-linux-amd64.tar.gz
tar xzf terraform-mirror-linux-amd64.tar.gz
chmod +x terraform-mirror
sudo mv terraform-mirror /usr/local/bin/

# macOS (arm64/Apple Silicon)
curl -LO https://github.com/ned1313/terraform-mirror/releases/latest/download/terraform-mirror-darwin-arm64.tar.gz
tar xzf terraform-mirror-darwin-arm64.tar.gz
chmod +x terraform-mirror
sudo mv terraform-mirror /usr/local/bin/
```

### Running the Binary

1. Create a configuration file:

```bash
mkdir -p /etc/tf-mirror
cat > /etc/tf-mirror/config.hcl << 'EOF'
server {
  port = 8080
}

storage {
  type = "local"
  endpoint = "/var/lib/tf-mirror/storage"
}

database {
  path = "/var/lib/tf-mirror/terraform-mirror.db"
}

cache {
  memory_size_mb = 256
  disk_path = "/var/cache/tf-mirror"
  disk_size_gb = 10
  ttl_seconds = 3600
}
EOF
```

2. Create data directories:

```bash
sudo mkdir -p /var/lib/tf-mirror/storage
sudo mkdir -p /var/cache/tf-mirror
sudo chown -R $USER:$USER /var/lib/tf-mirror /var/cache/tf-mirror
```

3. Run the server:

```bash
# Set admin credentials via environment
export TFM_ADMIN_USERNAME=admin
export TFM_ADMIN_PASSWORD=changeme123

# Start server
terraform-mirror -config /etc/tf-mirror/config.hcl
```

### Running as a Systemd Service

Create `/etc/systemd/system/terraform-mirror.service`:

```ini
[Unit]
Description=Terraform Mirror Server
After=network.target

[Service]
Type=simple
User=terraform-mirror
Group=terraform-mirror
ExecStart=/usr/local/bin/terraform-mirror -config /etc/tf-mirror/config.hcl
Restart=on-failure
RestartSec=5
Environment=TFM_ADMIN_USERNAME=admin
Environment=TFM_ADMIN_PASSWORD=changeme123

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/tf-mirror /var/cache/tf-mirror

[Install]
WantedBy=multi-user.target
```

Enable and start the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable terraform-mirror
sudo systemctl start terraform-mirror
sudo systemctl status terraform-mirror
```

## Building from Source

### Prerequisites

```bash
# Install Go 1.23+
# See https://go.dev/doc/install

# Install Node.js 18+
# See https://nodejs.org/

# Verify installations
go version
node --version
```

### Build

```bash
# Clone repository
git clone https://github.com/ned1313/terraform-mirror.git
cd terraform-mirror

# Build using Make (recommended)
make build

# Or build manually:

# Build frontend
cd web
npm ci
npm run build
cd ..

# Build backend
go build -o terraform-mirror ./cmd/terraform-mirror
```

### Build Docker Image

```bash
# Build image
docker build -t terraform-mirror:local -f deployments/docker/Dockerfile .

# Or using Make
make docker-build
```

## Initial Setup

After installation, complete these initial setup steps:

### 1. Access the Web UI

Open your browser to `http://localhost:8080` (or your configured host/port).

### 2. Log In

Use the admin credentials configured via environment variables:
- Default username: `admin`
- Default password: Set via `TFM_ADMIN_PASSWORD`

### 3. Change Default Password

Navigate to Settings and change the default password immediately.

### 4. Load Your First Providers

Create a provider definition file `providers.hcl`:

```hcl
provider "hashicorp/aws" {
  versions = ["5.31.0"]
  platforms = ["linux_amd64", "darwin_arm64", "windows_amd64"]
}

provider "hashicorp/azurerm" {
  versions = ["3.84.0"]
  platforms = ["linux_amd64"]
}
```

Upload via the web UI:
1. Navigate to **Providers** → **Load Providers**
2. Select your `providers.hcl` file
3. Click **Upload**

Or via API:

```bash
# Get auth token
TOKEN=$(curl -s -X POST http://localhost:8080/admin/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"changeme123"}' | jq -r '.token')

# Upload providers file
curl -X POST http://localhost:8080/admin/api/providers/load \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@providers.hcl"
```

### 5. Monitor Job Progress

Check the Jobs page in the UI or query the API:

```bash
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/admin/api/jobs
```

## Configuring Terraform Clients

Once Terraform Mirror is running and has providers loaded, configure your Terraform clients to use it.

### CLI Configuration (~/.terraformrc)

Create or edit `~/.terraformrc` (Linux/macOS) or `%APPDATA%\terraform.rc` (Windows):

```hcl
provider_installation {
  network_mirror {
    url = "http://your-mirror-host:8080/"
  }
}
```

### Per-Project Configuration

Create a `.terraformrc` file in your project directory:

```hcl
provider_installation {
  network_mirror {
    url = "http://your-mirror-host:8080/"
  }
}
```

Then set the environment variable:

```bash
export TF_CLI_CONFIG_FILE=.terraformrc
terraform init
```

### Air-Gapped / Offline Mode

For fully air-gapped environments, use the `direct` fallback to disable:

```hcl
provider_installation {
  network_mirror {
    url = "http://your-mirror-host:8080/"
  }
  
  # Remove or comment out to disable fallback to public registry
  # direct {}
}
```

## Verification

### Check Service Health

```bash
curl http://localhost:8080/health
# Expected: {"status":"healthy"}
```

### Test Service Discovery

```bash
curl http://localhost:8080/.well-known/terraform.json
# Expected: {"providers.v1":"/v1/providers/"}
```

### List Available Providers

```bash
curl http://localhost:8080/v1/providers/hashicorp/aws/versions
```

### Run Terraform Init

```bash
# In a directory with Terraform configuration using a mirrored provider
terraform init

# You should see:
# Initializing provider plugins...
# - Finding hashicorp/aws versions matching "~> 5.0"...
# - Installing hashicorp/aws v5.31.0...
```

## Troubleshooting

### Container Won't Start

Check logs:
```bash
docker logs terraform-mirror
```

Common issues:
- Storage not accessible (check S3 credentials)
- Database directory not writable
- Port already in use

### Terraform Can't Find Providers

1. Verify the provider is loaded:
   ```bash
   curl http://localhost:8080/v1/providers/hashicorp/aws/versions
   ```

2. Check Terraform configuration:
   ```bash
   cat ~/.terraformrc
   ```

3. Ensure the mirror URL is correct and accessible

### Slow Provider Downloads

Enable caching with adequate memory and disk:
```bash
TFM_CACHE_MEMORY_SIZE_MB=512
TFM_CACHE_DISK_SIZE_GB=20
```

### Connection Refused to MinIO

Ensure MinIO is healthy before starting Terraform Mirror:
```bash
docker-compose ps
curl http://localhost:9000/minio/health/live
```

## Next Steps

- [Configuration Reference](configuration.md) - Full configuration options
- [API Documentation](api.md) - REST API reference
- [User Guide](user-guide.md) - Detailed usage instructions
