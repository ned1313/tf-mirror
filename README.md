# Terraform Mirror

A caching proxy server for Terraform providers and modules, designed for air-gapped and low-bandwidth environments.

## Features

- **Provider Network Mirror**: Implements HashiCorp's Provider Network Mirror Protocol
- **Module Registry Mirror**: Implements Terraform Module Registry Protocol (Phase 2+)
- **S3-Compatible Storage**: Works with AWS S3, MinIO, and other S3-compatible object stores
- **GPG Verification**: Automatic provider signature verification
- **Two-Tier Caching**: Memory and disk caching for optimal performance
- **Web UI**: Vue.js-based admin interface and consumer browse interface
- **Admin Controls**: Bulk import, version management, audit logging
- **Container-Ready**: Docker, Kubernetes, and Helm deployments supported

## Quick Start

### Using Docker Compose

1. Clone the repository:
```bash
git clone https://github.com/yourusername/terraform-mirror
cd terraform-mirror
```

2. Start the services:
```bash
docker-compose -f deployments/docker-compose/docker-compose.yml up -d
```

3. Access the web UI at `http://localhost:8080`

4. Default admin credentials (change immediately):
   - Username: Set via `TFM_ADMIN_USERNAME` environment variable
   - Password: Set via `TFM_ADMIN_PASSWORD` environment variable

### Using Kubernetes

```bash
# Using kubectl
kubectl apply -f deployments/kubernetes/

# Using Helm
helm install terraform-mirror deployments/helm/terraform-mirror
```

## Configuration

Terraform Mirror can be configured via HCL file or environment variables.

### Configuration File

Create `/etc/tf-mirror/config.hcl`:

```hcl
server {
  port = 8080
  tls_enabled = false
}

storage {
  type = "s3"
  bucket = "terraform-mirror"
  region = "us-east-1"
  endpoint = "http://minio:9000"  # For MinIO
}

cache {
  memory_size_mb = 256
  disk_path = "/var/cache/tf-mirror"
  disk_size_gb = 10
}
```

### Environment Variables

- `TFM_SERVER_PORT` - Server port (default: 8080)
- `TFM_STORAGE_BUCKET` - S3 bucket name
- `TFM_STORAGE_ACCESS_KEY` - S3 access key
- `TFM_STORAGE_SECRET_KEY` - S3 secret key
- `TFM_ADMIN_USERNAME` - Initial admin username
- `TFM_ADMIN_PASSWORD` - Initial admin password

See [docs/configuration.md](docs/configuration.md) for full configuration reference.

## Usage

### Configuring Terraform Client

Add to your `~/.terraformrc`:

```hcl
provider_installation {
  network_mirror {
    url = "http://your-mirror-host:8080/v1/providers/"
  }
}
```

### Loading Providers

Create a provider definition file `providers.hcl`:

```hcl
provider "hashicorp/aws" {
  versions = ["5.31.0", "5.30.0"]
  architectures = ["linux_amd64", "darwin_arm64"]
}

provider "hashicorp/azurerm" {
  versions = ["3.84.0"]
  architectures = ["linux_amd64"]
}
```

Upload via the web UI or API:

```bash
curl -X POST http://localhost:8080/admin/api/providers/upload \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -F "file=@providers.hcl"
```

### Monitoring Jobs

Check job progress:

```bash
curl http://localhost:8080/admin/api/jobs/1 \
  -H "Authorization: Bearer YOUR_TOKEN"
```

## Development

### Prerequisites

- Go 1.23+
- Node.js 18+
- Docker (for local S3 via MinIO)

### Setup

```bash
# Install dependencies
go mod download
cd web && npm install

# Start MinIO for local development
docker-compose -f deployments/docker-compose/minio.yml up -d

# Run in development mode
make dev
```

### Testing

```bash
# Run all tests
make test

# Run unit tests only
make test-unit

# Run integration tests
make test-integration
```

## Architecture

```
┌─────────────────────────────────────────────────┐
│         Terraform Mirror Container              │
│                                                 │
│  ┌──────────┐         ┌─────────────┐         │
│  │ Vue 3 UI │────────▶│ Go Backend  │         │
│  └──────────┘         └──────┬──────┘         │
│                              │                  │
│                   ┌──────────┴──────────┐      │
│                   │                     │      │
│            ┌──────▼──────┐      ┌──────▼────┐ │
│            │   SQLite    │      │   Cache   │ │
│            └─────────────┘      └──────┬────┘ │
│                                        │      │
└────────────────────────────────────────┼──────┘
                                         │
                                  ┌──────▼─────┐
                                  │ S3 Storage │
                                  └────────────┘
```

See [technical-design.md](technical-design.md) for detailed architecture documentation.

## Documentation

- [Installation Guide](docs/installation.md)
- [Configuration Reference](docs/configuration.md)
- [API Documentation](docs/api.md)
- [User Guide](docs/user-guide.md)

## Roadmap

### Phase 1 (MVP) - ✅ Current
- Provider network mirror (manual loading)
- Admin authentication and UI
- Basic web interface

### Phase 2
- Auto-download providers on-demand
- Module registry mirror
- Enhanced search and filtering

### Phase 3
- Auto-download modules
- Advanced caching strategies
- Performance optimizations

### Phase 4
- SSO integration
- Multi-region support
- High availability

## Contributing

Contributions are welcome! Please read our contributing guidelines before submitting PRs.

## License

[Your chosen license]

## Support

For issues and questions, please use the GitHub issue tracker.
