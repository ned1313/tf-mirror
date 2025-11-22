# Development Quick Start for terraform-mirror with MinIO

## Prerequisites
- Docker and Docker Compose installed
- Go 1.24+ installed

## Start MinIO

```powershell
# Start MinIO container
docker-compose -f docker-compose.dev.yml up -d

# Wait for MinIO to be ready
Start-Sleep -Seconds 10

# Verify MinIO is running
docker ps | Select-String "tf-mirror-minio"

# Check bucket was created
docker logs tf-mirror-minio-create-bucket-1
```

MinIO Console: http://localhost:9001
- Username: `minioadmin`
- Password: `minioadmin`

## Build and Run terraform-mirror

```powershell
# Set MinIO credentials
$env:AWS_ACCESS_KEY_ID="minioadmin"
$env:AWS_SECRET_ACCESS_KEY="minioadmin"

# Build
go build -o terraform-mirror.exe ./cmd/terraform-mirror

# Run with dev config
Start-Process -FilePath ".\terraform-mirror.exe" -ArgumentList "-config", "config.dev.hcl" -WindowStyle Normal
```

## Test the Provider Loading Endpoint

```powershell
# Test health
curl http://localhost:8080/health

# Load providers
curl -X POST http://localhost:8080/admin/api/providers/load `
  -F "file=@examples/providers-test.hcl"
```

## Verify in MinIO

1. Open http://localhost:9001
2. Login with minioadmin/minioadmin
3. Browse to `terraform-mirror` bucket
4. Should see uploaded provider files

## Stop MinIO

```powershell
docker-compose -f docker-compose.dev.yml down

# Remove data (clean slate)
docker-compose -f docker-compose.dev.yml down -v
```

## Troubleshooting

### MinIO not accessible
```powershell
# Check logs
docker logs tf-mirror-minio

# Restart
docker-compose -f docker-compose.dev.yml restart minio
```

### Bucket not created
```powershell
# Check bucket creation logs
docker logs tf-mirror-minio-create-bucket-1

# Manually create bucket
docker exec -it tf-mirror-minio sh
mc alias set local http://localhost:9000 minioadmin minioadmin
mc mb local/terraform-mirror
exit
```

### Connection refused from terraform-mirror
- Ensure `s3_endpoint = "http://localhost:9000"` in config
- Ensure `s3_force_path_style = true` (required for MinIO)
- Verify environment variables are set:
  ```powershell
  $env:AWS_ACCESS_KEY_ID="minioadmin"
  $env:AWS_SECRET_ACCESS_KEY="minioadmin"
  ```

### Provider downloads but S3 upload fails
- Check MinIO is running: `docker ps`
- Check bucket exists: Open http://localhost:9001
- Check credentials in environment variables
- Check server logs for detailed error
