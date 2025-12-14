package processor

import (
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/ned1313/terraform-mirror/internal/database"
	"github.com/ned1313/terraform-mirror/internal/provider"
	"github.com/ned1313/terraform-mirror/internal/storage"
)

// mockStorage implements storage.Storage for testing
type mockStorage struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		objects: make(map[string][]byte),
	}
}

func (m *mockStorage) Upload(ctx context.Context, key string, reader io.Reader, contentType string, metadata map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, _ := io.ReadAll(reader)
	m.objects[key] = data
	return nil
}

func (m *mockStorage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	return nil, nil
}

func (m *mockStorage) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, key)
	return nil
}

func (m *mockStorage) Exists(ctx context.Context, key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, exists := m.objects[key]
	return exists, nil
}

// mockRegistryClient implements provider.RegistryDownloader for testing
type mockRegistryClient struct{}

func (m *mockRegistryClient) DownloadProviderComplete(ctx context.Context, namespace, providerType, version, os, arch string) *provider.DownloadResult {
	// Simulate a successful download
	return &provider.DownloadResult{
		Info: &provider.ProviderDownloadInfo{
			Namespace:   namespace,
			Type:        providerType,
			Version:     version,
			OS:          os,
			Arch:        arch,
			Platform:    fmt.Sprintf("%s_%s", os, arch),
			Filename:    fmt.Sprintf("terraform-provider-%s_%s_%s_%s.zip", providerType, version, os, arch),
			DownloadURL: fmt.Sprintf("https://releases.hashicorp.com/terraform-provider-%s/%s/terraform-provider-%s_%s_%s_%s.zip", providerType, version, providerType, version, os, arch),
			Shasum:      "abc123def456",
		},
		Data:     []byte("mock provider binary data"),
		Error:    nil,
		Duration: 100 * time.Millisecond,
	}
}

func (m *mockRegistryClient) GetAvailableVersions(ctx context.Context, namespace, providerType string) ([]string, error) {
	// Return some mock versions
	return []string{"1.0.0", "1.1.0", "2.0.0"}, nil
}

func (m *mockStorage) GetPresignedURL(ctx context.Context, key string, expiration time.Duration) (string, error) {
	return "https://mock-presigned-url/" + key, nil
}

func (m *mockStorage) GetMetadata(ctx context.Context, key string) (map[string]string, error) {
	return nil, nil
}

func (m *mockStorage) ListObjects(ctx context.Context, prefix string) ([]string, error) {
	return nil, nil
}

func (m *mockStorage) GetObjectSize(ctx context.Context, key string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if data, exists := m.objects[key]; exists {
		return int64(len(data)), nil
	}
	return 0, nil
}

func (m *mockStorage) Close() error {
	return nil
}

// Verify mockStorage implements storage.Storage
var _ storage.Storage = (*mockStorage)(nil)

func setupTestDB(t *testing.T) *database.DB {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	return db
}

func setupTestService(t *testing.T, db *database.DB) (*Service, *mockStorage) {
	store := newMockStorage()
	config := Config{
		PollingInterval:    100 * time.Millisecond,
		MaxConcurrentJobs:  2,
		RetryAttempts:      3,
		RetryDelay:         1 * time.Second,
		WorkerShutdownTime: 5 * time.Second,
	}
	service := NewService(config, db, store, "registry.terraform.io")
	// Inject mock registry to avoid real network calls
	service.SetRegistry(&mockRegistryClient{})
	return service, store
}

func TestService_StartStop(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	service, _ := setupTestService(t, db)

	// Test start
	err := service.Start(context.Background())
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Verify service is running
	status := service.GetStatus()
	if !status["running"].(bool) {
		t.Error("Service should be running")
	}

	// Test double start
	err = service.Start(context.Background())
	if err == nil {
		t.Error("Starting already running service should return error")
	}

	// Test stop
	err = service.Stop()
	if err != nil {
		t.Fatalf("Failed to stop service: %v", err)
	}

	// Verify service is stopped
	status = service.GetStatus()
	if status["running"].(bool) {
		t.Error("Service should not be running")
	}

	// Test double stop
	err = service.Stop()
	if err == nil {
		t.Error("Stopping already stopped service should return error")
	}
}

func TestService_ProcessJob(t *testing.T) {
	db := setupTestDB(t)

	service, _ := setupTestService(t, db)
	jobRepo := database.NewJobRepository(db)

	// Create a test job
	job := &database.DownloadJob{
		SourceType:     "api",
		SourceData:     `{"namespace":"hashicorp","type":"aws"}`,
		Status:         "pending",
		TotalItems:     2,
		CompletedItems: 0,
		FailedItems:    0,
	}

	err := jobRepo.Create(context.Background(), job)
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	// Create job items
	item1 := &database.DownloadJobItem{
		JobID:     job.ID,
		Namespace: "hashicorp",
		Type:      "aws",
		Version:   "5.0.0",
		Platform:  "linux_amd64",
		Status:    "pending",
	}
	item2 := &database.DownloadJobItem{
		JobID:     job.ID,
		Namespace: "hashicorp",
		Type:      "aws",
		Version:   "5.0.0",
		Platform:  "darwin_amd64",
		Status:    "pending",
	}

	err = jobRepo.CreateItem(context.Background(), item1)
	if err != nil {
		t.Fatalf("Failed to create item 1: %v", err)
	}

	err = jobRepo.CreateItem(context.Background(), item2)
	if err != nil {
		t.Fatalf("Failed to create item 2: %v", err)
	}

	// Start the service
	err = service.Start(context.Background())
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Wait for job to be processed
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	processed := false
	for !processed {
		select {
		case <-timeout:
			service.Stop()
			db.Close()
			t.Fatal("Timeout waiting for job to be processed")
		case <-ticker.C:
			updatedJob, err := jobRepo.GetByID(context.Background(), job.ID)
			if err != nil {
				service.Stop()
				db.Close()
				t.Fatalf("Failed to get updated job: %v", err)
			}

			if updatedJob.Status == "completed" || updatedJob.Status == "failed" {
				processed = true

				// Verify job status
				if updatedJob.Status != "completed" {
					t.Errorf("Expected job status 'completed', got '%s'", updatedJob.Status)
				}

				// Verify job progress
				if updatedJob.Progress != 100 {
					t.Errorf("Expected progress 100, got %d", updatedJob.Progress)
				}

				// Verify completed items
				if updatedJob.CompletedItems != 2 {
					t.Errorf("Expected 2 completed items, got %d", updatedJob.CompletedItems)
				}

				// Verify failed items
				if updatedJob.FailedItems != 0 {
					t.Errorf("Expected 0 failed items, got %d", updatedJob.FailedItems)
				}

				// Verify items are completed
				items, err := jobRepo.GetItems(context.Background(), job.ID)
				if err != nil {
					t.Fatalf("Failed to get job items: %v", err)
				}

				for _, item := range items {
					if item.Status != "completed" {
						t.Errorf("Expected item status 'completed', got '%s'", item.Status)
					}
				}
			}
		}
	}

	// Stop service and close DB
	service.Stop()
	db.Close()
}

func TestService_GetStatus(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	service, _ := setupTestService(t, db)

	// Get status before start
	status := service.GetStatus()
	if status["running"].(bool) {
		t.Error("Service should not be running")
	}
	if status["active_jobs"].(int) != 0 {
		t.Errorf("Expected 0 active jobs, got %d", status["active_jobs"])
	}

	// Start service
	err := service.Start(context.Background())
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}
	defer service.Stop()

	// Get status after start
	status = service.GetStatus()
	if !status["running"].(bool) {
		t.Error("Service should be running")
	}
}

func TestService_MaxConcurrentJobs(t *testing.T) {
	db := setupTestDB(t)

	service, _ := setupTestService(t, db)
	// Override max concurrent jobs to 1
	service.config.MaxConcurrentJobs = 1
	jobRepo := database.NewJobRepository(db)

	// Create multiple test jobs
	for i := 0; i < 3; i++ {
		job := &database.DownloadJob{
			SourceType:     "api",
			SourceData:     `{"test":"data"}`,
			Status:         "pending",
			TotalItems:     1,
			CompletedItems: 0,
			FailedItems:    0,
		}
		err := jobRepo.Create(context.Background(), job)
		if err != nil {
			t.Fatalf("Failed to create job %d: %v", i, err)
		}

		// Create a job item for each job
		item := &database.DownloadJobItem{
			JobID:     job.ID,
			Namespace: "test",
			Type:      "provider",
			Version:   "1.0.0",
			Platform:  "linux_amd64",
			Status:    "pending",
		}
		err = jobRepo.CreateItem(context.Background(), item)
		if err != nil {
			t.Fatalf("Failed to create item for job %d: %v", i, err)
		}
	}

	// Start the service
	err := service.Start(context.Background())
	if err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Wait for all jobs to complete
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			service.Stop()
			db.Close()
			t.Fatal("Timeout waiting for all jobs to complete")
		case <-ticker.C:
			// Count completed jobs
			completedCount := 0
			allJobs, err := jobRepo.List(context.Background(), 10, 0)
			if err != nil {
				// Database might be closed, just break
				service.Stop()
				db.Close()
				t.Fatalf("Failed to list jobs: %v", err)
			}
			for _, job := range allJobs {
				if job.Status == "completed" {
					completedCount++
				}
			}

			if completedCount >= 3 {
				// All jobs completed - stop service and close DB
				service.Stop()
				db.Close()
				return
			}
		}
	}
}

func TestService_CancelJob(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	service, _ := setupTestService(t, db)
	jobRepo := database.NewJobRepository(db)

	t.Run("cancel job not in activeJobs returns false", func(t *testing.T) {
		// Create a pending job (not in activeJobs)
		job := &database.DownloadJob{
			SourceType:     "api",
			SourceData:     `{"namespace":"hashicorp","type":"aws"}`,
			Status:         "pending",
			TotalItems:     1,
			CompletedItems: 0,
			FailedItems:    0,
		}
		err := jobRepo.Create(context.Background(), job)
		if err != nil {
			t.Fatalf("Failed to create job: %v", err)
		}

		// CancelJob returns false when job is not in activeJobs
		cancelled := service.CancelJob(job.ID)
		if cancelled {
			t.Error("Expected CancelJob to return false for job not in activeJobs")
		}
	})

	t.Run("cancel running job with active context", func(t *testing.T) {
		// Create a running job
		job := &database.DownloadJob{
			SourceType:     "api",
			SourceData:     `{"namespace":"hashicorp","type":"random"}`,
			Status:         "running",
			TotalItems:     1,
			CompletedItems: 0,
			FailedItems:    0,
		}
		err := jobRepo.Create(context.Background(), job)
		if err != nil {
			t.Fatalf("Failed to create job: %v", err)
		}

		// Simulate an active job by adding to activeJobs map
		ctx, cancel := context.WithCancel(context.Background())
		service.mu.Lock()
		service.activeJobs[job.ID] = cancel
		service.mu.Unlock()

		// Cancel the job
		cancelled := service.CancelJob(job.ID)
		if !cancelled {
			t.Error("Expected CancelJob to return true for active job")
		}

		// Verify the context was cancelled
		select {
		case <-ctx.Done():
			// Good, context was cancelled
		default:
			t.Error("Expected context to be cancelled")
		}
	})

	t.Run("cancel non-existent job returns false", func(t *testing.T) {
		// Try to cancel a job that doesn't exist in activeJobs
		cancelled := service.CancelJob(99999)
		if cancelled {
			t.Error("Expected CancelJob to return false for non-existent job")
		}
	})
}
