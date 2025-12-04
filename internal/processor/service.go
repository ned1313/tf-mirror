package processor

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/ned1313/terraform-mirror/internal/database"
	"github.com/ned1313/terraform-mirror/internal/provider"
	"github.com/ned1313/terraform-mirror/internal/storage"
)

// Config holds the processor configuration
type Config struct {
	PollingInterval    time.Duration // How often to check for new jobs
	MaxConcurrentJobs  int           // Maximum number of jobs to process concurrently
	RetryAttempts      int           // Number of retry attempts for failed downloads
	RetryDelay         time.Duration // Delay between retry attempts
	WorkerShutdownTime time.Duration // Time to wait for workers to finish during shutdown
}

// Service manages background job processing
type Service struct {
	config       Config
	db           *database.DB
	jobRepo      *database.JobRepository
	providerRepo *database.ProviderRepository
	storage      storage.Storage
	registry     provider.RegistryDownloader
	hostname     string // Hostname for storage keys (e.g., "registry.terraform.io")

	mu       sync.Mutex
	running  bool
	stopCh   chan struct{}
	doneCh   chan struct{}
	workerWg sync.WaitGroup

	// Track active jobs
	activeJobs map[int64]context.CancelFunc
}

// NewService creates a new processor service
func NewService(config Config, db *database.DB, store storage.Storage, hostname string) *Service {
	return &Service{
		config:       config,
		db:           db,
		jobRepo:      database.NewJobRepository(db),
		providerRepo: database.NewProviderRepository(db),
		storage:      store,
		registry:     provider.NewRegistryClient(),
		hostname:     hostname,
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
		activeJobs:   make(map[int64]context.CancelFunc),
	}
}

// SetRegistry allows injection of a mock registry client for testing
func (s *Service) SetRegistry(registry provider.RegistryDownloader) {
	s.registry = registry
}

// Start begins processing jobs
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("processor already running")
	}
	s.running = true
	s.mu.Unlock()

	log.Println("Starting job processor service")

	// Start the polling loop in a goroutine
	go s.pollLoop(ctx)

	return nil
}

// Stop stops the processor and waits for active jobs to complete
func (s *Service) Stop() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return fmt.Errorf("processor not running")
	}
	s.running = false
	s.mu.Unlock()

	log.Println("Stopping job processor service...")

	// Signal stop
	close(s.stopCh)

	// Cancel all active jobs
	s.mu.Lock()
	for jobID, cancel := range s.activeJobs {
		log.Printf("Cancelling active job %d", jobID)
		cancel()
	}
	s.mu.Unlock()

	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		s.workerWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("All workers stopped gracefully")
	case <-time.After(s.config.WorkerShutdownTime):
		log.Println("Worker shutdown timeout reached, forcing stop")
	}

	// Wait for polling loop to finish
	<-s.doneCh

	log.Println("Job processor service stopped")
	return nil
}

// CancelJob cancels a running job by its ID
// Returns true if the job was actively running and cancelled, false if not found
func (s *Service) CancelJob(jobID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cancel, exists := s.activeJobs[jobID]; exists {
		log.Printf("Cancelling job %d", jobID)
		cancel()
		return true
	}
	return false
}

// IsJobActive returns true if the job is currently being processed
func (s *Service) IsJobActive(jobID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, exists := s.activeJobs[jobID]
	return exists
}

// pollLoop continuously checks for pending jobs
func (s *Service) pollLoop(ctx context.Context) {
	defer close(s.doneCh)

	ticker := time.NewTicker(s.config.PollingInterval)
	defer ticker.Stop()

	// Process immediately on start
	s.processPendingJobs(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Println("Poll loop stopped: context cancelled")
			return
		case <-s.stopCh:
			log.Println("Poll loop stopped: stop signal received")
			return
		case <-ticker.C:
			s.processPendingJobs(ctx)
		}
	}
}

// processPendingJobs fetches and processes pending jobs
func (s *Service) processPendingJobs(ctx context.Context) {
	s.mu.Lock()
	activeCount := len(s.activeJobs)
	s.mu.Unlock()

	// Check if we've reached the max concurrent jobs limit
	if activeCount >= s.config.MaxConcurrentJobs {
		log.Printf("Max concurrent jobs reached (%d/%d), skipping poll",
			activeCount, s.config.MaxConcurrentJobs)
		return
	}

	// Calculate how many jobs we can start
	availableSlots := s.config.MaxConcurrentJobs - activeCount

	// Fetch pending jobs directly
	pendingJobs, err := s.jobRepo.ListPending(ctx, availableSlots)
	if err != nil {
		log.Printf("Error fetching pending jobs: %v", err)
		return
	}

	if len(pendingJobs) == 0 {
		return
	}

	log.Printf("Found %d pending jobs, starting processing...", len(pendingJobs))

	// Start a worker for each pending job
	for _, job := range pendingJobs {
		s.startJobWorker(ctx, job)
	}
}

// startJobWorker starts processing a job in a new goroutine
func (s *Service) startJobWorker(ctx context.Context, job *database.DownloadJob) {
	// Create a cancellable context for this job
	jobCtx, cancel := context.WithCancel(ctx)

	s.mu.Lock()
	s.activeJobs[job.ID] = cancel
	s.mu.Unlock()

	s.workerWg.Add(1)
	go func() {
		defer s.workerWg.Done()
		defer func() {
			s.mu.Lock()
			delete(s.activeJobs, job.ID)
			s.mu.Unlock()
		}()

		log.Printf("Starting job %d", job.ID)
		err := s.processJob(jobCtx, job)
		if err != nil {
			log.Printf("Job %d failed: %v", job.ID, err)
		} else {
			log.Printf("Job %d completed successfully", job.ID)
		}
	}()
}

// processJob processes a single download job
func (s *Service) processJob(ctx context.Context, job *database.DownloadJob) error {
	// Update job status to running
	job.Status = "running"
	now := time.Now()
	job.StartedAt.Time = now
	job.StartedAt.Valid = true
	if err := s.jobRepo.Update(ctx, job); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Get job items
	items, err := s.jobRepo.GetItems(ctx, job.ID)
	if err != nil {
		return s.failJob(ctx, job, fmt.Errorf("failed to get job items: %w", err))
	}

	if len(items) == 0 {
		return s.failJob(ctx, job, fmt.Errorf("job has no items to process"))
	}

	// Process each item
	for _, item := range items {
		if item.Status == "completed" {
			continue // Skip already completed items
		}

		// Check if context was cancelled
		select {
		case <-ctx.Done():
			return s.failJob(ctx, job, fmt.Errorf("job cancelled"))
		default:
		}

		// Process the item
		if err := s.processJobItem(ctx, job, item); err != nil {
			log.Printf("Job %d item %d failed: %v", job.ID, item.ID, err)
			job.FailedItems++
		} else {
			job.CompletedItems++
		}

		// Update job progress
		job.Progress = (job.CompletedItems * 100) / job.TotalItems
		if err := s.jobRepo.Update(ctx, job); err != nil {
			log.Printf("Failed to update job progress: %v", err)
		}
	}

	// Mark job as completed or failed
	if job.FailedItems == 0 {
		job.Status = "completed"
	} else if job.CompletedItems == 0 {
		job.Status = "failed"
	} else {
		job.Status = "completed" // Partial success
	}

	job.CompletedAt.Time = time.Now()
	job.CompletedAt.Valid = true

	if err := s.jobRepo.Update(ctx, job); err != nil {
		return fmt.Errorf("failed to update job final status: %w", err)
	}

	return nil
}

// processJobItem processes a single job item (provider download)
func (s *Service) processJobItem(ctx context.Context, job *database.DownloadJob, item *database.DownloadJobItem) error {
	// Parse platform into OS and arch
	parts := strings.Split(item.Platform, "_")
	if len(parts) != 2 {
		return s.failItem(ctx, item, fmt.Errorf("invalid platform format: %s", item.Platform))
	}
	osName, arch := parts[0], parts[1]

	// Check if provider already exists in database
	existingProvider, err := s.providerRepo.GetByIdentity(ctx, item.Namespace, item.Type, item.Version, item.Platform)
	if err != nil {
		return s.failItem(ctx, item, fmt.Errorf("failed to check existing provider: %w", err))
	}
	if existingProvider != nil {
		// Provider already exists, mark as completed and link to existing provider
		item.Status = "completed"
		item.ProviderID = sql.NullInt64{Int64: existingProvider.ID, Valid: true}
		item.CompletedAt.Time = time.Now()
		item.CompletedAt.Valid = true
		if err := s.jobRepo.UpdateItem(ctx, item); err != nil {
			return fmt.Errorf("failed to update item: %w", err)
		}
		log.Printf("Job %d item %d: Provider %s/%s %s (%s) already exists, skipping download",
			job.ID, item.ID, item.Namespace, item.Type, item.Version, item.Platform)
		return nil
	}

	// Update item status to downloading
	item.Status = "downloading"
	now := time.Now()
	item.StartedAt.Time = now
	item.StartedAt.Valid = true
	if err := s.jobRepo.UpdateItem(ctx, item); err != nil {
		return fmt.Errorf("failed to update item status: %w", err)
	}

	log.Printf("Job %d item %d: Downloading %s/%s %s (%s)",
		job.ID, item.ID, item.Namespace, item.Type, item.Version, item.Platform)

	// Download provider from registry (includes getting info and verification)
	result := s.registry.DownloadProviderComplete(ctx, item.Namespace, item.Type, item.Version, osName, arch)
	if result.Error != nil {
		return s.failItem(ctx, item, result.Error)
	}

	// Update item with download info
	item.DownloadURL = sql.NullString{String: result.Info.DownloadURL, Valid: true}
	item.SizeBytes = sql.NullInt64{Int64: int64(len(result.Data)), Valid: true}
	item.DownloadedBytes = sql.NullInt64{Int64: int64(len(result.Data)), Valid: true}
	if err := s.jobRepo.UpdateItem(ctx, item); err != nil {
		log.Printf("Warning: failed to update download info: %v", err)
	}

	// Generate S3 key
	s3Key := storage.BuildProviderKey(
		s.hostname,
		item.Namespace,
		item.Type,
		item.Version,
		osName,
		arch,
		result.Info.Filename,
	)

	// Upload to storage
	log.Printf("Job %d item %d: Uploading to storage: %s", job.ID, item.ID, s3Key)
	metadata := map[string]string{
		"namespace": item.Namespace,
		"type":      item.Type,
		"version":   item.Version,
		"platform":  item.Platform,
		"shasum":    result.Info.Shasum,
	}

	if err := s.storage.Upload(ctx, s3Key, bytes.NewReader(result.Data), "application/zip", metadata); err != nil {
		return s.failItem(ctx, item, fmt.Errorf("failed to upload to storage: %w", err))
	}

	// Create provider record in database
	providerRecord := &database.Provider{
		Namespace:   item.Namespace,
		Type:        item.Type,
		Version:     item.Version,
		Platform:    item.Platform,
		Filename:    result.Info.Filename,
		DownloadURL: result.Info.DownloadURL,
		Shasum:      result.Info.Shasum,
		S3Key:       s3Key,
		SizeBytes:   int64(len(result.Data)),
		Deprecated:  false,
		Blocked:     false,
	}

	if err := s.providerRepo.Create(ctx, providerRecord); err != nil {
		// If storage upload succeeded but DB failed, we should ideally clean up storage
		// For now, just log the error - the provider file is in storage and can be recovered
		log.Printf("Warning: provider uploaded but database insert failed: %v", err)
		return s.failItem(ctx, item, fmt.Errorf("failed to create provider record: %w", err))
	}

	// Mark item as completed
	item.Status = "completed"
	item.ProviderID = sql.NullInt64{Int64: providerRecord.ID, Valid: true}
	item.CompletedAt.Time = time.Now()
	item.CompletedAt.Valid = true

	if err := s.jobRepo.UpdateItem(ctx, item); err != nil {
		return fmt.Errorf("failed to update item completion: %w", err)
	}

	log.Printf("Job %d item %d: Successfully downloaded and stored %s/%s %s (%s) - %d bytes",
		job.ID, item.ID, item.Namespace, item.Type, item.Version, item.Platform, len(result.Data))

	return nil
}

// failItem marks an item as failed with an error message
func (s *Service) failItem(ctx context.Context, item *database.DownloadJobItem, err error) error {
	item.Status = "failed"
	item.ErrorMessage = sql.NullString{String: err.Error(), Valid: true}
	item.CompletedAt.Time = time.Now()
	item.CompletedAt.Valid = true

	if updateErr := s.jobRepo.UpdateItem(ctx, item); updateErr != nil {
		log.Printf("Failed to update item status: %v", updateErr)
	}

	return err
}

// failJob marks a job as failed and updates the error message
func (s *Service) failJob(ctx context.Context, job *database.DownloadJob, err error) error {
	job.Status = "failed"
	job.ErrorMessage.String = err.Error()
	job.ErrorMessage.Valid = true
	job.CompletedAt.Time = time.Now()
	job.CompletedAt.Valid = true

	if updateErr := s.jobRepo.Update(ctx, job); updateErr != nil {
		log.Printf("Failed to update job status: %v", updateErr)
	}

	return err
}

// GetStatus returns the current processor status
func (s *Service) GetStatus() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	return map[string]interface{}{
		"running":             s.running,
		"active_jobs":         len(s.activeJobs),
		"max_concurrent_jobs": s.config.MaxConcurrentJobs,
	}
}
