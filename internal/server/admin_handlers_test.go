package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ned1313/terraform-mirror/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleGetProvider(t *testing.T) {
	server, cleanup := setupAdminTest(t)
	defer cleanup()

	// Create a test provider
	providerRepo := database.NewProviderRepository(server.db)
	provider := &database.Provider{
		Namespace:   "hashicorp",
		Type:        "aws",
		Version:     "5.0.0",
		Platform:    "linux_amd64",
		Filename:    "terraform-provider-aws_5.0.0_linux_amd64.zip",
		DownloadURL: "https://example.com/provider.zip",
		Shasum:      "abc123",
		S3Key:       "providers/hashicorp/aws/5.0.0/linux_amd64/provider.zip",
		SizeBytes:   1024,
	}
	err := providerRepo.Create(context.Background(), provider)
	require.NoError(t, err)

	// Get auth token
	token := getAuthToken(t, server)

	t.Run("get existing provider", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/api/providers/1", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var result database.Provider
		err := json.NewDecoder(w.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, "hashicorp", result.Namespace)
		assert.Equal(t, "aws", result.Type)
		assert.Equal(t, "5.0.0", result.Version)
	})

	t.Run("get non-existent provider", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/api/providers/999", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("invalid provider ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/api/providers/invalid", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandleUpdateProvider(t *testing.T) {
	server, cleanup := setupAdminTest(t)
	defer cleanup()

	// Create a test provider
	providerRepo := database.NewProviderRepository(server.db)
	provider := &database.Provider{
		Namespace:   "hashicorp",
		Type:        "aws",
		Version:     "5.0.0",
		Platform:    "linux_amd64",
		Filename:    "terraform-provider-aws_5.0.0_linux_amd64.zip",
		DownloadURL: "https://example.com/provider.zip",
		Shasum:      "abc123",
		S3Key:       "providers/hashicorp/aws/5.0.0/linux_amd64/provider.zip",
		SizeBytes:   1024,
	}
	err := providerRepo.Create(context.Background(), provider)
	require.NoError(t, err)

	// Get auth token
	token := getAuthToken(t, server)

	t.Run("deprecate provider", func(t *testing.T) {
		body := `{"deprecated": true}`
		req := httptest.NewRequest(http.MethodPut, "/admin/api/providers/1", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var result database.Provider
		err := json.NewDecoder(w.Body).Decode(&result)
		require.NoError(t, err)

		assert.True(t, result.Deprecated)
		assert.False(t, result.Blocked)
	})

	t.Run("block provider", func(t *testing.T) {
		body := `{"blocked": true}`
		req := httptest.NewRequest(http.MethodPut, "/admin/api/providers/1", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var result database.Provider
		err := json.NewDecoder(w.Body).Decode(&result)
		require.NoError(t, err)

		assert.True(t, result.Blocked)
	})

	t.Run("update non-existent provider", func(t *testing.T) {
		body := `{"deprecated": true}`
		req := httptest.NewRequest(http.MethodPut, "/admin/api/providers/999", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandleDeleteProvider(t *testing.T) {
	server, cleanup := setupAdminTest(t)
	defer cleanup()

	// Create a test provider
	providerRepo := database.NewProviderRepository(server.db)
	provider := &database.Provider{
		Namespace:   "hashicorp",
		Type:        "aws",
		Version:     "5.0.0",
		Platform:    "linux_amd64",
		Filename:    "terraform-provider-aws_5.0.0_linux_amd64.zip",
		DownloadURL: "https://example.com/provider.zip",
		Shasum:      "abc123",
		S3Key:       "providers/hashicorp/aws/5.0.0/linux_amd64/provider.zip",
		SizeBytes:   1024,
	}
	err := providerRepo.Create(context.Background(), provider)
	require.NoError(t, err)

	// Get auth token
	token := getAuthToken(t, server)

	t.Run("delete existing provider", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/admin/api/providers/1", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Verify provider was deleted
		deleted, err := providerRepo.GetByID(context.Background(), 1)
		require.NoError(t, err)
		assert.Nil(t, deleted)
	})

	t.Run("delete non-existent provider", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/admin/api/providers/999", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandleRetryJob(t *testing.T) {
	server, cleanup := setupAdminTest(t)
	defer cleanup()

	// Get auth token
	token := getAuthToken(t, server)

	t.Run("retry job with failed items", func(t *testing.T) {
		// Create a failed job with failed items
		jobRepo := database.NewJobRepository(server.db)
		job := &database.DownloadJob{
			SourceType:     "hcl",
			SourceData:     "test",
			Status:         "failed",
			TotalItems:     2,
			CompletedItems: 0,
			FailedItems:    2,
		}
		err := jobRepo.Create(context.Background(), job)
		require.NoError(t, err)

		// Create failed items
		item1 := &database.DownloadJobItem{
			JobID:     job.ID,
			Namespace: "hashicorp",
			Type:      "aws",
			Version:   "5.0.0",
			Platform:  "linux_amd64",
			Status:    "failed",
		}
		err = jobRepo.CreateItem(context.Background(), item1)
		require.NoError(t, err)

		item2 := &database.DownloadJobItem{
			JobID:     job.ID,
			Namespace: "hashicorp",
			Type:      "aws",
			Version:   "5.0.0",
			Platform:  "darwin_amd64",
			Status:    "failed",
		}
		err = jobRepo.CreateItem(context.Background(), item2)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/admin/api/jobs/1/retry", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var result map[string]interface{}
		err = json.NewDecoder(w.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, float64(2), result["reset_count"])
	})

	t.Run("retry running job fails", func(t *testing.T) {
		// Create a running job
		jobRepo := database.NewJobRepository(server.db)
		job := &database.DownloadJob{
			SourceType: "hcl",
			SourceData: "test",
			Status:     "running",
			TotalItems: 1,
		}
		err := jobRepo.Create(context.Background(), job)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/admin/api/jobs/2/retry", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("retry non-existent job", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/api/jobs/999/retry", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestHandleCancelJob(t *testing.T) {
	server, cleanup := setupAdminTest(t)
	defer cleanup()

	// Get auth token
	token := getAuthToken(t, server)

	t.Run("cancel pending job", func(t *testing.T) {
		// Create a pending job
		jobRepo := database.NewJobRepository(server.db)
		job := &database.DownloadJob{
			SourceType:     "hcl",
			SourceData:     "test",
			Status:         "pending",
			TotalItems:     2,
			CompletedItems: 0,
			FailedItems:    0,
		}
		err := jobRepo.Create(context.Background(), job)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/api/jobs/%d/cancel", job.ID), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var result map[string]interface{}
		err = json.NewDecoder(w.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, "Job cancelled", result["message"])

		// Verify job status is cancelled
		updatedJob, err := jobRepo.GetByID(context.Background(), job.ID)
		require.NoError(t, err)
		assert.Equal(t, "cancelled", updatedJob.Status)
	})

	t.Run("cancel running job", func(t *testing.T) {
		// Create a running job
		jobRepo := database.NewJobRepository(server.db)
		job := &database.DownloadJob{
			SourceType:     "hcl",
			SourceData:     "test",
			Status:         "running",
			TotalItems:     2,
			CompletedItems: 0,
			FailedItems:    0,
		}
		err := jobRepo.Create(context.Background(), job)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/api/jobs/%d/cancel", job.ID), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var result map[string]interface{}
		err = json.NewDecoder(w.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, "Job cancelled", result["message"])

		// Verify job status is cancelled
		updatedJob, err := jobRepo.GetByID(context.Background(), job.ID)
		require.NoError(t, err)
		assert.Equal(t, "cancelled", updatedJob.Status)
	})

	t.Run("cancel completed job fails", func(t *testing.T) {
		// Create a completed job
		jobRepo := database.NewJobRepository(server.db)
		job := &database.DownloadJob{
			SourceType:     "hcl",
			SourceData:     "test",
			Status:         "completed",
			TotalItems:     1,
			CompletedItems: 1,
		}
		err := jobRepo.Create(context.Background(), job)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/api/jobs/%d/cancel", job.ID), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var result map[string]interface{}
		err = json.NewDecoder(w.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, "invalid_status", result["error"])
	})

	t.Run("cancel failed job fails", func(t *testing.T) {
		// Create a failed job
		jobRepo := database.NewJobRepository(server.db)
		job := &database.DownloadJob{
			SourceType:  "hcl",
			SourceData:  "test",
			Status:      "failed",
			TotalItems:  1,
			FailedItems: 1,
		}
		err := jobRepo.Create(context.Background(), job)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/api/jobs/%d/cancel", job.ID), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var result map[string]interface{}
		err = json.NewDecoder(w.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, "invalid_status", result["error"])
	})

	t.Run("cancel non-existent job", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/api/jobs/999/cancel", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("cancel invalid job id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/admin/api/jobs/invalid/cancel", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandleStorageStats(t *testing.T) {
	server, cleanup := setupAdminTest(t)
	defer cleanup()

	// Create some test providers
	providerRepo := database.NewProviderRepository(server.db)

	providers := []*database.Provider{
		{
			Namespace:  "hashicorp",
			Type:       "aws",
			Version:    "5.0.0",
			Platform:   "linux_amd64",
			SizeBytes:  1024 * 1024,
			Deprecated: true,
		},
		{
			Namespace: "hashicorp",
			Type:      "aws",
			Version:   "5.0.0",
			Platform:  "darwin_amd64",
			SizeBytes: 1024 * 1024 * 2,
			Blocked:   true,
		},
		{
			Namespace: "hashicorp",
			Type:      "random",
			Version:   "3.5.0",
			Platform:  "linux_amd64",
			SizeBytes: 512 * 1024,
		},
	}

	for _, p := range providers {
		err := providerRepo.Create(context.Background(), p)
		require.NoError(t, err)
	}

	// Get auth token
	token := getAuthToken(t, server)

	req := httptest.NewRequest(http.MethodGet, "/admin/api/stats/storage", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result StorageStatsResponse
	err := json.NewDecoder(w.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, int64(3), result.TotalProviders)
	assert.Equal(t, int64(1024*1024+1024*1024*2+512*1024), result.TotalSizeBytes)
	assert.Equal(t, int64(1), result.UniqueNamespaces)
	assert.Equal(t, int64(2), result.UniqueTypes)
	assert.Equal(t, int64(1), result.DeprecatedCount)
	assert.Equal(t, int64(1), result.BlockedCount)
	assert.NotEmpty(t, result.TotalSizeHuman)
}

func TestHandleAuditLogs(t *testing.T) {
	server, cleanup := setupAdminTest(t)
	defer cleanup()

	// Create some audit log entries
	auditRepo := database.NewAuditRepository(server.db)
	entries := []*database.AdminAction{
		{
			Action:       "login",
			ResourceType: "session",
			Success:      true,
		},
		{
			Action:       "provider_create",
			ResourceType: "provider",
			Success:      true,
		},
		{
			Action:       "login",
			ResourceType: "session",
			Success:      false,
		},
	}

	for _, e := range entries {
		err := auditRepo.Log(context.Background(), e)
		require.NoError(t, err)
	}

	// Get auth token
	token := getAuthToken(t, server)

	t.Run("list all audit logs", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/api/stats/audit", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var result AuditLogResponse
		err := json.NewDecoder(w.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, 3, result.Total)
		assert.Len(t, result.Logs, 3)
	})

	t.Run("filter by action", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/api/stats/audit?action=login", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var result AuditLogResponse
		err := json.NewDecoder(w.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, 2, result.Total)
		for _, log := range result.Logs {
			assert.Equal(t, "login", log.Action)
		}
	})
}

func TestHandleGetConfig(t *testing.T) {
	server, cleanup := setupAdminTest(t)
	defer cleanup()

	// Get auth token
	token := getAuthToken(t, server)

	req := httptest.NewRequest(http.MethodGet, "/admin/api/config", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	server.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result SanitizedConfig
	err := json.NewDecoder(w.Body).Decode(&result)
	require.NoError(t, err)

	// Check that config is returned
	assert.Equal(t, 8080, result.Server.Port)
	assert.False(t, result.Server.BehindProxy)

	// Note: Secrets should NOT be in the response
	// JWT secret, access keys, etc. are not exposed
}

func TestHandleTriggerBackup(t *testing.T) {
	server, cleanup := setupAdminTest(t)
	defer cleanup()

	// Get auth token
	token := getAuthToken(t, server)

	t.Run("backup disabled", func(t *testing.T) {
		// Default config has backup disabled
		req := httptest.NewRequest(http.MethodPost, "/admin/api/backup", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		server.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var result map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&result)
		require.NoError(t, err)

		assert.Equal(t, "backup_disabled", result["error"])
	})
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{1024 * 1024, "1.00 MB"},
		{1024 * 1024 * 1024, "1.00 GB"},
		{1024 * 1024 * 1024 * 2, "2.00 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatBytes(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}
