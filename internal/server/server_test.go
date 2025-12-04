package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ned1313/terraform-mirror/internal/config"
	"github.com/ned1313/terraform-mirror/internal/database"
	"github.com/ned1313/terraform-mirror/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestServer creates a test server with temporary database and mock storage
func setupTestServer(t *testing.T) (*Server, func()) {
	// Create temporary directory for test database
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
			Type:     "local",
			Endpoint: filepath.Join(tempDir, "storage"),
			Bucket:   "test-bucket",
			Region:   "us-east-1",
		},
		Auth: config.AuthConfig{
			JWTExpirationHours: 24,
			BCryptCost:         10,
			JWTSecret:          "test-secret-key-for-testing",
		},
		Processor: config.ProcessorConfig{
			PollingIntervalSeconds: 10,
			MaxConcurrentJobs:      3,
			RetryAttempts:          3,
			RetryDelaySeconds:      5,
			WorkerShutdownSeconds:  30,
		},
		Telemetry: config.TelemetryConfig{
			Enabled: true,
		},
	}

	// Initialize database
	db, err := database.New(cfg.Database.Path)
	require.NoError(t, err)

	// Use mock storage for testing
	store := storage.NewMockStorage()

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

// createTestToken creates a test admin user and returns a valid JWT token
func createTestToken(t *testing.T, srv *Server) string {
	// Create admin user
	userRepo := database.NewUserRepository(srv.db)
	hashedPassword, err := srv.authService.HashPassword("testpass")
	require.NoError(t, err)

	user := &database.AdminUser{
		Username:     "testadmin",
		PasswordHash: hashedPassword,
		Active:       true,
	}
	err = userRepo.Create(context.Background(), user)
	require.NoError(t, err)

	// Generate token
	token, jti, expiresAt, err := srv.authService.GenerateToken(user.ID, user.Username)
	require.NoError(t, err)

	// Create session record
	sessionRepo := database.NewSessionRepository(srv.db)
	session := &database.AdminSession{
		UserID:    user.ID,
		TokenJTI:  jti,
		ExpiresAt: expiresAt,
		Revoked:   false,
		CreatedAt: time.Now(),
	}
	err = sessionRepo.Create(context.Background(), session)
	require.NoError(t, err)

	return token
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

func TestHandleLogin_MissingBody(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/admin/api/login", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	// Should return bad request when no body is provided
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleListProviders_Success(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	token := createTestToken(t, srv)

	req := httptest.NewRequest(http.MethodGet, "/admin/api/providers", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)

	// Empty database should return empty array with count of 0
	providers, ok := response["providers"].([]interface{})
	require.True(t, ok, "providers should be an array")
	assert.Empty(t, providers, "providers array should be empty for empty database")

	count, ok := response["count"].(float64) // JSON numbers are float64
	require.True(t, ok, "count should be a number")
	assert.Equal(t, float64(0), count, "count should be 0 for empty database")
}

func TestHandleMetrics(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/plain")

	// Verify Prometheus metrics are present in the response
	body := w.Body.String()
	assert.Contains(t, body, "terraform_mirror_")
	assert.Contains(t, body, "providers_total")
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
