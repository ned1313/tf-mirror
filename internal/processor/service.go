package processor

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ned1313/terraform-mirror/internal/database"
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
	config   Config
	db       *database.DB
	jobRepo  *database.JobRepository
	mu       sync.Mutex
	running  bool
	stopCh   chan struct{}
	doneCh   chan struct{}
	workerWg sync.WaitGroup

	// Track active jobs
	activeJobs map[int64]context.CancelFunc
}

// NewService creates a new processor service
func NewService(config Config, db *database.DB) *Service {
	return &Service{
		config:     config,
		db:         db,
		jobRepo:    database.NewJobRepository(db),
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
		activeJobs: make(map[int64]context.CancelFunc),
	}
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

	// Fetch pending jobs
	jobs, err := s.jobRepo.List(ctx, availableSlots, 0)
	if err != nil {
		log.Printf("Error fetching pending jobs: %v", err)
		return
	}

	// Filter for pending jobs only
	pendingJobs := make([]*database.DownloadJob, 0)
	for _, job := range jobs {
		if job.Status == "pending" {
			pendingJobs = append(pendingJobs, job)
		}
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
	// Update item status to downloading
	item.Status = "downloading"
	now := time.Now()
	item.StartedAt.Time = now
	item.StartedAt.Valid = true
	if err := s.jobRepo.UpdateItem(ctx, item); err != nil {
		return fmt.Errorf("failed to update item status: %w", err)
	}

	// TODO: Implement actual provider download logic
	// For now, we'll just mark it as completed
	// In Step 13, we'll integrate with the registry client and storage

	// Simulate download
	time.Sleep(100 * time.Millisecond)

	// Mark item as completed
	item.Status = "completed"
	item.CompletedAt.Time = time.Now()
	item.CompletedAt.Valid = true

	if err := s.jobRepo.UpdateItem(ctx, item); err != nil {
		return fmt.Errorf("failed to update item completion: %w", err)
	}

	return nil
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
