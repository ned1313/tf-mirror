# Storage Layer Testing

This directory contains tests for the storage layer, including both unit tests and integration tests.

## Running Tests

### Unit Tests Only

Run the standard unit tests (no external dependencies):

```powershell
go test -v ./internal/storage/...
```

**Coverage:** ~58% (tests local storage and S3 constructors only)

### Integration Tests with MinIO

Run integration tests that use a real MinIO S3-compatible server:

1. **Start MinIO container:**
   ```powershell
   docker-compose -f docker-compose.test.yml up -d
   ```

2. **Run integration tests:**
   ```powershell
   go test -v -tags=integration ./internal/storage/...
   ```

3. **Check coverage:**
   ```powershell
   go test -tags=integration -coverprofile=coverage-integration.out ./internal/storage/...
   go tool cover -html=coverage-integration.out
   ```

4. **Stop MinIO when done:**
   ```powershell
   docker-compose -f docker-compose.test.yml down
   ```

**Coverage with integration tests:** ~80%

### Accessing MinIO Console

While MinIO is running, you can access the web console:

- URL: http://localhost:9001
- Username: `minioadmin`
- Password: `minioadmin`

You can browse the `test-bucket` to see files created during tests.

## Test Structure

### Unit Tests
- `factory_test.go` - Factory function and helper tests
- `local_test.go` - Local filesystem storage tests
- `s3_test.go` - S3 constructor and configuration tests

### Integration Tests
- `s3_integration_test.go` - Full S3 operations with MinIO (requires `-tags=integration`)

## Coverage Goals

- **Local Storage:** >80% (fully tested in unit tests)
- **S3 Storage:** >70% (requires integration tests)
- **Overall:** >80% (with integration tests)

## MinIO Configuration

The test MinIO instance is configured in `docker-compose.test.yml`:
- S3 API Port: 9000
- Console Port: 9001
- Root User: minioadmin
- Root Password: minioadmin

## CI/CD Considerations

For CI/CD pipelines:

1. Use the short test flag to skip integration tests:
   ```powershell
   go test -short ./internal/storage/...
   ```

2. Or run integration tests in CI with MinIO:
   ```yaml
   - docker-compose -f docker-compose.test.yml up -d
   - go test -v -tags=integration ./internal/storage/...
   - docker-compose -f docker-compose.test.yml down
   ```
