package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/ned1313/terraform-mirror/internal/config"
	"github.com/ned1313/terraform-mirror/internal/database"
	"github.com/ned1313/terraform-mirror/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestServer creates a test server with temporary database and storage
func setupTestServer(t *testing.T) (*Server, func()) {
	// Create temporary directory for test database and storage
	tempDir, err := os.MkdirTemp("", "server_test_*")
	require.NoError(t, err)

	// Create test configuration
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:           8080,
			TLSEnabled:     false,
			BehindProxy:    true,
			TrustedProxies: []string{"127.0.0.1"},
		},
		Database: config.DatabaseConfig{
			Path:          filepath.Join(tempDir, "test.db"),
			BackupEnabled: false,
		},
		Storage: config.StorageConfig{
			Type:   "s3",
			Bucket: "test-bucket",
			Region: "us-east-1",
		},
		Auth: config.AuthConfig{
			JWTExpirationHours: 24,
			BcryptCost:         10,
		},
		Telemetry: config.TelemetryConfig{
			Enabled: true,
		},
	}

	// Initialize database
	db, err := database.New(cfg.Database.Path)
	require.NoError(t, err)

	// Initialize storage
	store, err := storage.NewFromConfig(context.Background(), cfg.Storage)
	require.NoError(t, err)

	// Create server
	srv := New(cfg, db, store)

	// Cleanup function
	cleanup := func() {
		srv.Shutdown(context.Background())
		db.Close()
		store.Close()
		os.RemoveAll(tempDir)
	}

	return srv, cleanup
}

func TestNew(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	assert.NotNil(t, srv)
	assert.NotNil(t, srv.config)
	assert.NotNil(t, srv.db)
	assert.NotNil(t, srv.storage)
}

func TestRouter(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	router := srv.Router()
	assert.NotNil(t, router)
}

func TestHandleHealth(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response HealthResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, "healthy", response.Status)
	assert.Equal(t, "0.1.0", response.Version)
}

func TestHandleServiceDiscovery(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/.well-known/terraform.json", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response ServiceDiscoveryResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, "/v1/providers/", response.ProvidersV1)
}

func TestHandleProviderVersions_NotImplemented(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/providers/hashicorp/aws/versions", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotImplemented, w.Code)
}

func TestHandleProviderDownload_NotImplemented(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/providers/hashicorp/aws/5.0.0/download/linux/amd64", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotImplemented, w.Code)
}

func TestHandleLogin_NotImplemented(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/admin/api/login", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotImplemented, w.Code)
}

func TestHandleListProviders_NotImplemented(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/admin/api/providers", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotImplemented, w.Code)
}

func TestHandleMetrics_NotImplemented(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotImplemented, w.Code)
	assert.Equal(t, "text/plain", w.Header().Get("Content-Type"))
}

func TestCORSMiddleware(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	tests := []struct {
		name           string
		origin         string
		expectHeaders  bool
		expectedOrigin string
	}{
		{
			name:           "trusted proxy",
			origin:         "http://127.0.0.1:3000",
			expectHeaders:  true,
			expectedOrigin: "http://127.0.0.1:3000",
		},
		{
			name:          "untrusted origin",
			origin:        "http://evil.com",
			expectHeaders: false,
		},
		{
			name:          "no origin",
			origin:        "",
			expectHeaders: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			w := httptest.NewRecorder()

			srv.Router().ServeHTTP(w, req)

			if tt.expectHeaders {
				assert.Equal(t, tt.expectedOrigin, w.Header().Get("Access-Control-Allow-Origin"))
				assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Methods"))
				assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Headers"))
			} else {
				assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
			}
		})
	}
}

func TestCORSMiddleware_Preflight(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodOptions, "/health", nil)
	req.Header.Set("Origin", "http://127.0.0.1:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "http://127.0.0.1:3000", w.Header().Get("Access-Control-Allow-Origin"))
	assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Methods"))
	assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Headers"))
}
