package processor

import (
	"context"
	"testing"
	"time"

	"github.com/ned1313/terraform-mirror/internal/database"
)

func setupTestDB(t *testing.T) *database.DB {
	db, err := database.New(":memory:")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	return db
}

func TestService_StartStop(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	config := Config{
		PollingInterval:    100 * time.Millisecond,
		MaxConcurrentJobs:  2,
		RetryAttempts:      3,
		RetryDelay:         1 * time.Second,
		WorkerShutdownTime: 5 * time.Second,
	}

	service := NewService(config, db)

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
	defer db.Close()

	config := Config{
		PollingInterval:    100 * time.Millisecond,
		MaxConcurrentJobs:  2,
		RetryAttempts:      3,
		RetryDelay:         1 * time.Second,
		WorkerShutdownTime: 5 * time.Second,
	}

	service := NewService(config, db)
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
	defer service.Stop()

	// Wait for job to be processed
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	processed := false
	for !processed {
		select {
		case <-timeout:
			t.Fatal("Timeout waiting for job to be processed")
		case <-ticker.C:
			updatedJob, err := jobRepo.GetByID(context.Background(), job.ID)
			if err != nil {
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
}

func TestService_GetStatus(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	config := Config{
		PollingInterval:    100 * time.Millisecond,
		MaxConcurrentJobs:  3,
		RetryAttempts:      3,
		RetryDelay:         1 * time.Second,
		WorkerShutdownTime: 5 * time.Second,
	}

	service := NewService(config, db)

	// Get status before start
	status := service.GetStatus()
	if status["running"].(bool) {
		t.Error("Service should not be running")
	}
	if status["active_jobs"].(int) != 0 {
		t.Errorf("Expected 0 active jobs, got %d", status["active_jobs"])
	}
	if status["max_concurrent_jobs"].(int) != 3 {
		t.Errorf("Expected max_concurrent_jobs 3, got %d", status["max_concurrent_jobs"])
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
	defer db.Close()

	config := Config{
		PollingInterval:    100 * time.Millisecond,
		MaxConcurrentJobs:  1, // Only allow 1 concurrent job
		RetryAttempts:      3,
		RetryDelay:         1 * time.Second,
		WorkerShutdownTime: 5 * time.Second,
	}

	service := NewService(config, db)
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
	defer service.Stop()

	// Give it a moment to start processing
	time.Sleep(200 * time.Millisecond)

	// Check status - should have at most 1 active job
	status := service.GetStatus()
	activeJobs := status["active_jobs"].(int)
	if activeJobs > 1 {
		t.Errorf("Expected at most 1 active job, got %d", activeJobs)
	}

	// Wait for all jobs to complete
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatal("Timeout waiting for all jobs to complete")
		case <-ticker.C:
			// Count completed jobs
			completedCount := 0
			for i := int64(1); i <= 3; i++ {
				job, err := jobRepo.GetByID(context.Background(), i)
				if err != nil {
					t.Fatalf("Failed to get job %d: %v", i, err)
				}
				if job.Status == "completed" {
					completedCount++
				}
			}

			if completedCount == 3 {
				return // All jobs completed
			}
		}
	}
}
