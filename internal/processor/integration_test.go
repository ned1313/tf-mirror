//go:build integration
// +build integration

package processor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ned1313/terraform-mirror/internal/database"
	"github.com/ned1313/terraform-mirror/internal/storage"
)

// Integration tests for the processor service with real storage
// Run with: go test -tags=integration ./internal/processor/...

func setupIntegrationTest(t *testing.T) (*database.DB, storage.Storage, string, func()) {
	// Create temporary directory for storage
	tempDir, err := os.MkdirTemp("", "processor-integration-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create local storage
	store, err := storage.NewLocalStorage(storage.LocalConfig{
		BasePath: tempDir,
	})
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create local storage: %v", err)
	}

	// Create in-memory database
	db, err := database.New(":memory:")
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create database: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.RemoveAll(tempDir)
	}

	return db, store, tempDir, cleanup
}

// TestIntegration_ProcessorWithLocalStorage tests the processor with real local storage
func TestIntegration_ProcessorWithLocalStorage(t *testing.T) {
	db, store, tempDir, cleanup := setupIntegrationTest(t)
	defer cleanup()

	config := Config{
		PollingInterval:    100 * time.Millisecond,
		MaxConcurrentJobs:  2,
		RetryAttempts:      3,
		RetryDelay:         1 * time.Second,
		WorkerShutdownTime: 5 * time.Second,
	}

	service := NewService(config, db, store, "registry.terraform.io")
	// Use mock registry client to avoid hitting real registry
	service.SetRegistry(&mockRegistryClient{})

	jobRepo := database.NewJobRepository(db)
	providerRepo := database.NewProviderRepository(db)

	// Create a test job
	job := &database.DownloadJob{
		SourceType:     "api",
		SourceData:     `{"namespace":"hashicorp","type":"random"}`,
		Status:         "pending",
		TotalItems:     1,
		CompletedItems: 0,
		FailedItems:    0,
	}

	err := jobRepo.Create(context.Background(), job)
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	// Create a job item
	item := &database.DownloadJobItem{
		JobID:     job.ID,
		Namespace: "hashicorp",
		Type:      "random",
		Version:   "3.5.1",
		Platform:  "linux_amd64",
		Status:    "pending",
	}

	err = jobRepo.CreateItem(context.Background(), item)
	if err != nil {
		t.Fatalf("Failed to create job item: %v", err)
	}

	// Start the service
	err = service.Start(context.Background())
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Wait for job to complete
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var completedJob *database.DownloadJob
	for {
		select {
		case <-timeout:
			service.Stop()
			t.Fatal("Timeout waiting for job to complete")
		case <-ticker.C:
			completedJob, err = jobRepo.GetByID(context.Background(), job.ID)
			if err != nil {
				service.Stop()
				t.Fatalf("Failed to get job: %v", err)
			}
			if completedJob.Status == "completed" || completedJob.Status == "failed" {
				goto done
			}
		}
	}
done:
	service.Stop()

	// Verify job completed successfully
	if completedJob.Status != "completed" {
		t.Errorf("Expected job status 'completed', got '%s'", completedJob.Status)
	}

	// Verify provider was stored in database
	storedProvider, err := providerRepo.GetByIdentity(context.Background(), "hashicorp", "random", "3.5.1", "linux_amd64")
	if err != nil {
		t.Fatalf("Failed to get provider from database: %v", err)
	}
	if storedProvider == nil {
		t.Fatal("Provider not found in database")
	}

	// Verify provider file was uploaded to storage
	expectedKey := "providers/registry.terraform.io/hashicorp/random/3.5.1/linux_amd64/terraform-provider-random_3.5.1_linux_amd64.zip"
	exists, err := store.Exists(context.Background(), expectedKey)
	if err != nil {
		t.Fatalf("Failed to check storage: %v", err)
	}
	if !exists {
		t.Errorf("Provider file not found in storage at key: %s", expectedKey)
	}

	// Verify file content
	expectedPath := filepath.Join(tempDir, expectedKey)
	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("Failed to read provider file: %v", err)
	}
	if len(data) == 0 {
		t.Error("Provider file is empty")
	}

	t.Logf("Successfully processed provider: %s/%s %s (%s)",
		storedProvider.Namespace, storedProvider.Type, storedProvider.Version, storedProvider.Platform)
	t.Logf("Storage path: %s", expectedPath)
	t.Logf("File size: %d bytes", len(data))
}

// TestIntegration_ProcessorSkipsExistingProviders tests that existing providers are skipped
func TestIntegration_ProcessorSkipsExistingProviders(t *testing.T) {
	db, store, _, cleanup := setupIntegrationTest(t)
	defer cleanup()

	config := Config{
		PollingInterval:    100 * time.Millisecond,
		MaxConcurrentJobs:  2,
		RetryAttempts:      3,
		RetryDelay:         1 * time.Second,
		WorkerShutdownTime: 5 * time.Second,
	}

	service := NewService(config, db, store, "registry.terraform.io")
	service.SetRegistry(&mockRegistryClient{})

	jobRepo := database.NewJobRepository(db)
	providerRepo := database.NewProviderRepository(db)

	// Pre-create a provider in the database
	existingProvider := &database.Provider{
		Namespace:   "hashicorp",
		Type:        "random",
		Version:     "3.5.1",
		Platform:    "linux_amd64",
		Filename:    "terraform-provider-random_3.5.1_linux_amd64.zip",
		DownloadURL: "https://example.com/provider.zip",
		Shasum:      "abc123",
		S3Key:       "providers/registry.terraform.io/hashicorp/random/3.5.1/linux_amd64/terraform-provider-random_3.5.1_linux_amd64.zip",
		SizeBytes:   1000,
	}
	err := providerRepo.Create(context.Background(), existingProvider)
	if err != nil {
		t.Fatalf("Failed to create existing provider: %v", err)
	}

	// Create a job for the same provider
	job := &database.DownloadJob{
		SourceType:     "api",
		SourceData:     `{"namespace":"hashicorp","type":"random"}`,
		Status:         "pending",
		TotalItems:     1,
		CompletedItems: 0,
		FailedItems:    0,
	}

	err = jobRepo.Create(context.Background(), job)
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	item := &database.DownloadJobItem{
		JobID:     job.ID,
		Namespace: "hashicorp",
		Type:      "random",
		Version:   "3.5.1",
		Platform:  "linux_amd64",
		Status:    "pending",
	}

	err = jobRepo.CreateItem(context.Background(), item)
	if err != nil {
		t.Fatalf("Failed to create job item: %v", err)
	}

	// Start the service
	err = service.Start(context.Background())
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Wait for job to complete
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var completedJob *database.DownloadJob
	for {
		select {
		case <-timeout:
			service.Stop()
			t.Fatal("Timeout waiting for job to complete")
		case <-ticker.C:
			completedJob, err = jobRepo.GetByID(context.Background(), job.ID)
			if err != nil {
				service.Stop()
				t.Fatalf("Failed to get job: %v", err)
			}
			if completedJob.Status == "completed" || completedJob.Status == "failed" {
				goto done
			}
		}
	}
done:
	service.Stop()

	// Verify job completed successfully (provider was skipped, not downloaded)
	if completedJob.Status != "completed" {
		t.Errorf("Expected job status 'completed', got '%s'", completedJob.Status)
	}

	// Verify the job item was linked to the existing provider
	items, err := jobRepo.GetItems(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("Failed to get job items: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("Expected 1 job item, got %d", len(items))
	}

	if !items[0].ProviderID.Valid || items[0].ProviderID.Int64 != existingProvider.ID {
		t.Errorf("Expected item to link to existing provider ID %d, got %v",
			existingProvider.ID, items[0].ProviderID)
	}

	t.Log("Successfully skipped existing provider and linked job item")
}

// TestIntegration_ProcessorMultipleItems tests processing multiple items in a job
func TestIntegration_ProcessorMultipleItems(t *testing.T) {
	db, store, _, cleanup := setupIntegrationTest(t)
	defer cleanup()

	config := Config{
		PollingInterval:    100 * time.Millisecond,
		MaxConcurrentJobs:  2,
		RetryAttempts:      3,
		RetryDelay:         1 * time.Second,
		WorkerShutdownTime: 5 * time.Second,
	}

	service := NewService(config, db, store, "registry.terraform.io")
	service.SetRegistry(&mockRegistryClient{})

	jobRepo := database.NewJobRepository(db)
	providerRepo := database.NewProviderRepository(db)

	// Create a job with multiple items
	job := &database.DownloadJob{
		SourceType:     "api",
		SourceData:     `{"namespace":"hashicorp","type":"aws"}`,
		Status:         "pending",
		TotalItems:     3,
		CompletedItems: 0,
		FailedItems:    0,
	}

	err := jobRepo.Create(context.Background(), job)
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	// Create multiple job items for different platforms
	platforms := []string{"linux_amd64", "darwin_amd64", "windows_amd64"}
	for _, platform := range platforms {
		item := &database.DownloadJobItem{
			JobID:     job.ID,
			Namespace: "hashicorp",
			Type:      "aws",
			Version:   "5.0.0",
			Platform:  platform,
			Status:    "pending",
		}
		err = jobRepo.CreateItem(context.Background(), item)
		if err != nil {
			t.Fatalf("Failed to create job item for %s: %v", platform, err)
		}
	}

	// Start the service
	err = service.Start(context.Background())
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Wait for job to complete
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var completedJob *database.DownloadJob
	for {
		select {
		case <-timeout:
			service.Stop()
			t.Fatal("Timeout waiting for job to complete")
		case <-ticker.C:
			completedJob, err = jobRepo.GetByID(context.Background(), job.ID)
			if err != nil {
				service.Stop()
				t.Fatalf("Failed to get job: %v", err)
			}
			if completedJob.Status == "completed" || completedJob.Status == "failed" {
				goto done
			}
		}
	}
done:
	service.Stop()

	// Verify job completed successfully
	if completedJob.Status != "completed" {
		t.Errorf("Expected job status 'completed', got '%s'", completedJob.Status)
	}

	// Verify all items completed
	if completedJob.CompletedItems != 3 {
		t.Errorf("Expected 3 completed items, got %d", completedJob.CompletedItems)
	}

	// Verify all providers were stored in database
	for _, platform := range platforms {
		parts := splitPlatform(platform)
		storedProvider, err := providerRepo.GetByIdentity(context.Background(), "hashicorp", "aws", "5.0.0", platform)
		if err != nil {
			t.Fatalf("Failed to get provider for %s: %v", platform, err)
		}
		if storedProvider == nil {
			t.Errorf("Provider not found in database for platform: %s", platform)
			continue
		}

		// Verify storage
		expectedKey := fmt.Sprintf("providers/registry.terraform.io/hashicorp/aws/5.0.0/%s_%s/terraform-provider-aws_5.0.0_%s_%s.zip",
			parts[0], parts[1], parts[0], parts[1])
		exists, err := store.Exists(context.Background(), expectedKey)
		if err != nil {
			t.Fatalf("Failed to check storage for %s: %v", platform, err)
		}
		if !exists {
			t.Errorf("Provider file not found in storage for platform %s at key: %s", platform, expectedKey)
		}
	}

	t.Logf("Successfully processed %d providers", completedJob.CompletedItems)
}

func splitPlatform(platform string) []string {
	for i, c := range platform {
		if c == '_' {
			return []string{platform[:i], platform[i+1:]}
		}
	}
	return []string{platform, ""}
}

// TestIntegration_ProcessorWithRealRegistry tests with the actual Terraform registry
// This test is skipped by default - enable with TF_MIRROR_REAL_REGISTRY=1
func TestIntegration_ProcessorWithRealRegistry(t *testing.T) {
	if os.Getenv("TF_MIRROR_REAL_REGISTRY") != "1" {
		t.Skip("Skipping real registry test - set TF_MIRROR_REAL_REGISTRY=1 to enable")
	}

	db, store, tempDir, cleanup := setupIntegrationTest(t)
	defer cleanup()

	config := Config{
		PollingInterval:    100 * time.Millisecond,
		MaxConcurrentJobs:  1, // Be nice to the registry
		RetryAttempts:      3,
		RetryDelay:         2 * time.Second,
		WorkerShutdownTime: 30 * time.Second,
	}

	// Use real registry client
	service := NewService(config, db, store, "registry.terraform.io")
	// Don't set mock registry - use real one

	jobRepo := database.NewJobRepository(db)
	providerRepo := database.NewProviderRepository(db)

	// Download a small provider (hashicorp/random is ~15MB)
	job := &database.DownloadJob{
		SourceType:     "api",
		SourceData:     `{"namespace":"hashicorp","type":"random"}`,
		Status:         "pending",
		TotalItems:     1,
		CompletedItems: 0,
		FailedItems:    0,
	}

	err := jobRepo.Create(context.Background(), job)
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	item := &database.DownloadJobItem{
		JobID:     job.ID,
		Namespace: "hashicorp",
		Type:      "random",
		Version:   "3.5.1",
		Platform:  "linux_amd64",
		Status:    "pending",
	}

	err = jobRepo.CreateItem(context.Background(), item)
	if err != nil {
		t.Fatalf("Failed to create job item: %v", err)
	}

	t.Log("Starting download of hashicorp/random 3.5.1 for linux_amd64...")
	start := time.Now()

	// Start the service
	err = service.Start(context.Background())
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Wait for job to complete (longer timeout for real download)
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var completedJob *database.DownloadJob
	for {
		select {
		case <-timeout:
			service.Stop()
			t.Fatal("Timeout waiting for job to complete")
		case <-ticker.C:
			completedJob, err = jobRepo.GetByID(context.Background(), job.ID)
			if err != nil {
				service.Stop()
				t.Fatalf("Failed to get job: %v", err)
			}
			if completedJob.Status == "completed" || completedJob.Status == "failed" {
				goto done
			}
			t.Logf("Job status: %s, progress: %d%%", completedJob.Status, completedJob.Progress)
		}
	}
done:
	service.Stop()
	duration := time.Since(start)

	// Verify job completed successfully
	if completedJob.Status != "completed" {
		t.Errorf("Expected job status 'completed', got '%s' (error: %s)",
			completedJob.Status, completedJob.ErrorMessage.String)
	}

	// Verify provider was stored
	storedProvider, err := providerRepo.GetByIdentity(context.Background(), "hashicorp", "random", "3.5.1", "linux_amd64")
	if err != nil {
		t.Fatalf("Failed to get provider: %v", err)
	}
	if storedProvider == nil {
		t.Fatal("Provider not found in database")
	}

	// Verify file exists and has reasonable size
	expectedKey := "providers/registry.terraform.io/hashicorp/random/3.5.1/linux_amd64/" + storedProvider.Filename
	expectedPath := filepath.Join(tempDir, expectedKey)

	info, err := os.Stat(expectedPath)
	if err != nil {
		t.Fatalf("Failed to stat provider file: %v", err)
	}

	// hashicorp/random 3.5.1 for linux_amd64 is about 15MB
	if info.Size() < 1000000 { // At least 1MB
		t.Errorf("Provider file seems too small: %d bytes", info.Size())
	}

	t.Logf("Successfully downloaded provider in %v", duration)
	t.Logf("Provider: %s/%s %s (%s)", storedProvider.Namespace, storedProvider.Type, storedProvider.Version, storedProvider.Platform)
	t.Logf("Filename: %s", storedProvider.Filename)
	t.Logf("Size: %d bytes (%.2f MB)", info.Size(), float64(info.Size())/1024/1024)
	t.Logf("SHA256: %s", storedProvider.Shasum)
	t.Logf("Storage path: %s", expectedPath)
}

// TestIntegration_ProcessorWithMinIO tests with MinIO (if available)
// This test is skipped by default - enable with TF_MIRROR_MINIO_TEST=1
func TestIntegration_ProcessorWithMinIO(t *testing.T) {
	endpoint := os.Getenv("TF_MIRROR_MINIO_ENDPOINT")
	accessKey := os.Getenv("TF_MIRROR_MINIO_ACCESS_KEY")
	secretKey := os.Getenv("TF_MIRROR_MINIO_SECRET_KEY")
	bucket := os.Getenv("TF_MIRROR_MINIO_BUCKET")

	if endpoint == "" || accessKey == "" || secretKey == "" || bucket == "" {
		t.Skip("Skipping MinIO test - set TF_MIRROR_MINIO_* environment variables to enable")
	}

	// Create S3 storage pointing to MinIO
	store, err := storage.NewS3Storage(context.Background(), storage.S3Config{
		Endpoint:       endpoint,
		AccessKey:      accessKey,
		SecretKey:      secretKey,
		Bucket:         bucket,
		Region:         "us-east-1",
		ForcePathStyle: true,
	})
	if err != nil {
		t.Fatalf("Failed to create MinIO storage: %v", err)
	}
	defer store.Close()

	// Create in-memory database
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	config := Config{
		PollingInterval:    100 * time.Millisecond,
		MaxConcurrentJobs:  2,
		RetryAttempts:      3,
		RetryDelay:         1 * time.Second,
		WorkerShutdownTime: 5 * time.Second,
	}

	service := NewService(config, db, store, "registry.terraform.io")
	service.SetRegistry(&mockRegistryClient{})

	jobRepo := database.NewJobRepository(db)

	// Create a test job
	job := &database.DownloadJob{
		SourceType:     "api",
		SourceData:     `{"namespace":"hashicorp","type":"random"}`,
		Status:         "pending",
		TotalItems:     1,
		CompletedItems: 0,
		FailedItems:    0,
	}

	err = jobRepo.Create(context.Background(), job)
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	item := &database.DownloadJobItem{
		JobID:     job.ID,
		Namespace: "hashicorp",
		Type:      "random",
		Version:   "3.5.1",
		Platform:  "linux_amd64",
		Status:    "pending",
	}

	err = jobRepo.CreateItem(context.Background(), item)
	if err != nil {
		t.Fatalf("Failed to create job item: %v", err)
	}

	// Start the service
	err = service.Start(context.Background())
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Wait for job to complete
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var completedJob *database.DownloadJob
	for {
		select {
		case <-timeout:
			service.Stop()
			t.Fatal("Timeout waiting for job to complete")
		case <-ticker.C:
			completedJob, err = jobRepo.GetByID(context.Background(), job.ID)
			if err != nil {
				service.Stop()
				t.Fatalf("Failed to get job: %v", err)
			}
			if completedJob.Status == "completed" || completedJob.Status == "failed" {
				goto done
			}
		}
	}
done:
	service.Stop()

	// Verify job completed successfully
	if completedJob.Status != "completed" {
		t.Errorf("Expected job status 'completed', got '%s'", completedJob.Status)
	}

	// Verify file exists in MinIO
	expectedKey := "providers/registry.terraform.io/hashicorp/random/3.5.1/linux_amd64/terraform-provider-random_3.5.1_linux_amd64.zip"
	exists, err := store.Exists(context.Background(), expectedKey)
	if err != nil {
		t.Fatalf("Failed to check MinIO storage: %v", err)
	}
	if !exists {
		t.Errorf("Provider file not found in MinIO at key: %s", expectedKey)
	}

	// Clean up the test file from MinIO
	if err := store.Delete(context.Background(), expectedKey); err != nil {
		t.Logf("Warning: Failed to clean up test file from MinIO: %v", err)
	}

	t.Log("Successfully uploaded provider to MinIO")
}
