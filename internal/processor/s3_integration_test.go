//go:build integration
// +build integration

package processor

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ned1313/terraform-mirror/internal/database"
	"github.com/ned1313/terraform-mirror/internal/storage"
)

// TestIntegration_ProcessorWithS3Storage tests the processor with MinIO/S3 storage
// Run with: go test -tags=integration -v ./internal/processor/... -run S3
// Requires MinIO running: docker-compose -f deployments/docker-compose/docker-compose.test.yml up -d
func TestIntegration_ProcessorWithS3Storage(t *testing.T) {
	// Skip if not running with S3 environment
	endpoint := os.Getenv("S3_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:9000"
	}

	// Try to connect to MinIO - skip if not available
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	store, err := storage.NewS3Storage(ctx, storage.S3Config{
		Region:         "us-east-1",
		Bucket:         "terraform-mirror",
		Endpoint:       endpoint,
		AccessKey:      "minioadmin",
		SecretKey:      "minioadmin",
		ForcePathStyle: true,
	})
	if err != nil {
		t.Skipf("MinIO not available at %s: %v", endpoint, err)
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
	providerRepo := database.NewProviderRepository(db)

	// Create a test job
	job := &database.DownloadJob{
		SourceType:     "api",
		SourceData:     `{"namespace":"hashicorp","type":"null"}`,
		Status:         "pending",
		TotalItems:     1,
		CompletedItems: 0,
		FailedItems:    0,
	}

	err = jobRepo.Create(ctx, job)
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	// Create a job item for null provider (small, fast to download)
	item := &database.DownloadJobItem{
		JobID:     job.ID,
		Namespace: "hashicorp",
		Type:      "null",
		Version:   "3.2.2",
		Platform:  "linux_amd64",
		Status:    "pending",
	}

	err = jobRepo.CreateItem(ctx, item)
	if err != nil {
		t.Fatalf("Failed to create job item: %v", err)
	}

	// Start the service
	err = service.Start(ctx)
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
			completedJob, err = jobRepo.GetByID(ctx, job.ID)
			if err != nil {
				service.Stop()
				t.Fatalf("Failed to get job: %v", err)
			}
			if completedJob.Status == "completed" || completedJob.Status == "failed" {
				goto done
			}
			t.Logf("Job status: %s, completed: %d, failed: %d",
				completedJob.Status, completedJob.CompletedItems, completedJob.FailedItems)
		}
	}
done:
	service.Stop()

	// Verify job completed successfully
	if completedJob.Status != "completed" {
		// Get job items for debugging
		items, _ := jobRepo.GetItems(ctx, job.ID)
		for _, it := range items {
			t.Logf("Item %s/%s %s (%s): %s - %s",
				it.Namespace, it.Type, it.Version, it.Platform, it.Status, it.ErrorMessage.String)
		}
		t.Errorf("Expected job status 'completed', got '%s'", completedJob.Status)
	}

	// Verify provider was stored in database
	storedProvider, err := providerRepo.GetByIdentity(ctx, "hashicorp", "null", "3.2.2", "linux_amd64")
	if err != nil {
		t.Fatalf("Failed to get provider from database: %v", err)
	}
	if storedProvider == nil {
		t.Fatal("Provider not found in database")
	}

	// Verify provider file was uploaded to S3
	expectedKey := "providers/registry.terraform.io/hashicorp/null/3.2.2/linux_amd64/terraform-provider-null_3.2.2_linux_amd64.zip"
	exists, err := store.Exists(ctx, expectedKey)
	if err != nil {
		t.Fatalf("Failed to check S3 storage: %v", err)
	}
	if !exists {
		t.Errorf("Provider file not found in S3 at key: %s", expectedKey)
	}

	t.Logf("Successfully processed provider with S3 storage: %s/%s %s (%s)",
		storedProvider.Namespace, storedProvider.Type, storedProvider.Version, storedProvider.Platform)
	t.Logf("S3 key: %s", expectedKey)
}

// TestIntegration_FullProviderLoadFlow tests the complete flow from HCL parsing to S3 storage
func TestIntegration_FullProviderLoadFlow(t *testing.T) {
	endpoint := os.Getenv("S3_ENDPOINT")
	if endpoint == "" {
		endpoint = "http://localhost:9000"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	store, err := storage.NewS3Storage(ctx, storage.S3Config{
		Region:         "us-east-1",
		Bucket:         "terraform-mirror",
		Endpoint:       endpoint,
		AccessKey:      "minioadmin",
		SecretKey:      "minioadmin",
		ForcePathStyle: true,
	})
	if err != nil {
		t.Skipf("MinIO not available at %s: %v", endpoint, err)
	}
	defer store.Close()

	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	config := Config{
		PollingInterval:    100 * time.Millisecond,
		MaxConcurrentJobs:  1,
		RetryAttempts:      2,
		RetryDelay:         500 * time.Millisecond,
		WorkerShutdownTime: 5 * time.Second,
	}

	service := NewService(config, db, store, "registry.terraform.io")
	service.SetRegistry(&mockRegistryClient{})

	jobRepo := database.NewJobRepository(db)

	// Create a job with multiple providers (simulating HCL file upload)
	job := &database.DownloadJob{
		SourceType:     "hcl",
		SourceData:     `provider "hashicorp/null" { versions = ["3.2.2"] platforms = ["linux_amd64"] }`,
		Status:         "pending",
		TotalItems:     2,
		CompletedItems: 0,
		FailedItems:    0,
	}

	err = jobRepo.Create(context.Background(), job)
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	// Create job items
	items := []struct {
		namespace string
		typ       string
		version   string
		platform  string
	}{
		{"hashicorp", "null", "3.2.2", "linux_amd64"},
		{"hashicorp", "random", "3.5.1", "linux_amd64"},
	}

	for _, it := range items {
		item := &database.DownloadJobItem{
			JobID:     job.ID,
			Namespace: it.namespace,
			Type:      it.typ,
			Version:   it.version,
			Platform:  it.platform,
			Status:    "pending",
		}
		err = jobRepo.CreateItem(context.Background(), item)
		if err != nil {
			t.Fatalf("Failed to create job item: %v", err)
		}
	}

	// Start the service
	err = service.Start(context.Background())
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Wait for job to complete
	timeout := time.After(60 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			service.Stop()
			t.Fatal("Timeout waiting for job to complete")
		case <-ticker.C:
			completedJob, err := jobRepo.GetByID(context.Background(), job.ID)
			if err != nil {
				service.Stop()
				t.Fatalf("Failed to get job: %v", err)
			}
			t.Logf("Job progress: %d/%d completed, %d failed",
				completedJob.CompletedItems, completedJob.TotalItems, completedJob.FailedItems)
			if completedJob.Status == "completed" || completedJob.Status == "failed" {
				service.Stop()

				if completedJob.CompletedItems != completedJob.TotalItems {
					jobItems, _ := jobRepo.GetItems(context.Background(), job.ID)
					for _, it := range jobItems {
						t.Logf("  %s/%s %s (%s): %s - %s",
							it.Namespace, it.Type, it.Version, it.Platform, it.Status, it.ErrorMessage.String)
					}
				}

				if completedJob.Status != "completed" {
					t.Errorf("Expected job status 'completed', got '%s'", completedJob.Status)
				}
				return
			}
		}
	}
}
