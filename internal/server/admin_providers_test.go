package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ned1313/terraform-mirror/internal/config"
	"github.com/ned1313/terraform-mirror/internal/database"
	"github.com/ned1313/terraform-mirror/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAdminTest(t *testing.T) (*Server, func()) {
	// Create test database
	db, err := database.New(":memory:")
	require.NoError(t, err)

	// Create test storage
	store, err := storage.NewLocalStorage(storage.LocalConfig{
		BasePath: t.TempDir(),
	})
	require.NoError(t, err)

	// Create test config
	cfg := &config.Config{
		Server: config.ServerConfig{
			Port:        8080,
			BehindProxy: false,
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
	}

	// Create server
	server := New(cfg, db, store)

	cleanup := func() {
		server.Shutdown(context.Background())
		db.Close()
		store.Close()
	}

	return server, cleanup
}

// getAuthToken creates a test user and returns a valid JWT token
func getAuthToken(t *testing.T, server *Server) string {
	userRepo := database.NewUserRepository(server.db)
	hashedPassword, err := server.authService.HashPassword("testpass")
	require.NoError(t, err)

	user := &database.AdminUser{
		Username:     "testadmin",
		PasswordHash: hashedPassword,
		Active:       true,
	}
	err = userRepo.Create(context.Background(), user)
	require.NoError(t, err)

	token, jti, expiresAt, err := server.authService.GenerateToken(user.ID, user.Username)
	require.NoError(t, err)

	// Create session record
	sessionRepo := database.NewSessionRepository(server.db)
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

func createMultipartRequest(t *testing.T, content string) (*http.Request, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Create form file
	part, err := writer.CreateFormFile("file", "providers.hcl")
	require.NoError(t, err)

	_, err = io.WriteString(part, content)
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/admin/api/providers/load", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	return req, writer.FormDataContentType()
}

// addAuthHeader adds authentication header to a request
func addAuthHeader(req *http.Request, token string) {
	req.Header.Set("Authorization", "Bearer "+token)
}

func TestHandleLoadProviders_Success(t *testing.T) {
	server, cleanup := setupAdminTest(t)
	defer cleanup()

	token := getAuthToken(t, server)

	hcl := `
provider "hashicorp/random" {
  versions = ["3.5.0"]
  platforms = ["linux_amd64"]
}
`

	req, _ := createMultipartRequest(t, hcl)
	addAuthHeader(req, token)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	// Note: This will likely fail because we can't actually download from registry in tests
	// But we can check that the endpoint is wired up correctly
	assert.Equal(t, http.StatusOK, rr.Code, "Expected status 200, got response: %s", rr.Body.String())

	var response LoadProvidersResponse
	err := json.NewDecoder(rr.Body).Decode(&response)
	require.NoError(t, err)

	assert.Greater(t, response.JobID, int64(0), "Job ID should be positive")
	assert.Greater(t, response.Total, 0, "Total providers should be > 0")
	assert.Contains(t, response.Message, "Provider loading job created")
}

func TestHandleLoadProviders_InvalidHCL(t *testing.T) {
	server, cleanup := setupAdminTest(t)
	defer cleanup()

	token := getAuthToken(t, server)

	hcl := `
invalid hcl content {{{
`

	req, _ := createMultipartRequest(t, hcl)
	addAuthHeader(req, token)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var errResp ErrorResponse
	err := json.NewDecoder(rr.Body).Decode(&errResp)
	require.NoError(t, err)

	assert.Equal(t, "parse_error", errResp.Error)
	assert.Contains(t, errResp.Message, "Failed to parse HCL")
}

func TestHandleLoadProviders_NoProviders(t *testing.T) {
	server, cleanup := setupAdminTest(t)
	defer cleanup()

	token := getAuthToken(t, server)

	hcl := `
# Empty file with no providers
`

	req, _ := createMultipartRequest(t, hcl)
	addAuthHeader(req, token)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var errResp ErrorResponse
	err := json.NewDecoder(rr.Body).Decode(&errResp)
	require.NoError(t, err)

	// Parser returns error for empty file
	assert.Equal(t, "parse_error", errResp.Error)
	assert.Contains(t, errResp.Message, "no provider definitions found")
}

func TestHandleLoadProviders_NoFile(t *testing.T) {
	server, cleanup := setupAdminTest(t)
	defer cleanup()

	token := getAuthToken(t, server)

	// Create request without file
	req := httptest.NewRequest(http.MethodPost, "/admin/api/providers/load", nil)
	req.Header.Set("Content-Type", "multipart/form-data")
	addAuthHeader(req, token)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var errResp ErrorResponse
	err := json.NewDecoder(rr.Body).Decode(&errResp)
	require.NoError(t, err)

	// Without proper form data, ParseMultipartForm fails
	assert.Equal(t, "invalid_form", errResp.Error)
}

func TestHandleLoadProviders_FileTooLarge(t *testing.T) {
	server, cleanup := setupAdminTest(t)
	defer cleanup()

	token := getAuthToken(t, server)

	// Create a large HCL file (> 1MB)
	largeHCL := make([]byte, 1<<20+1) // 1MB + 1 byte
	for i := range largeHCL {
		largeHCL[i] = 'a'
	}

	req, _ := createMultipartRequest(t, string(largeHCL))
	addAuthHeader(req, token)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var errResp ErrorResponse
	err := json.NewDecoder(rr.Body).Decode(&errResp)
	require.NoError(t, err)

	assert.Equal(t, "file_too_large", errResp.Error)
}

func TestHandleLoadProviders_InvalidProvider(t *testing.T) {
	server, cleanup := setupAdminTest(t)
	defer cleanup()

	token := getAuthToken(t, server)

	// Invalid provider source (missing namespace)
	hcl := `
provider "invalid" {
  versions = ["1.0.0"]
  platforms = ["linux_amd64"]
}
`

	req, _ := createMultipartRequest(t, hcl)
	addAuthHeader(req, token)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var errResp ErrorResponse
	err := json.NewDecoder(rr.Body).Decode(&errResp)
	require.NoError(t, err)

	assert.Equal(t, "parse_error", errResp.Error)
}

func TestHandleLoadProviders_InvalidVersion(t *testing.T) {
	server, cleanup := setupAdminTest(t)
	defer cleanup()

	token := getAuthToken(t, server)

	// Invalid version format
	hcl := `
provider "hashicorp/random" {
  versions = ["invalid"]
  platforms = ["linux_amd64"]
}
`

	req, _ := createMultipartRequest(t, hcl)
	addAuthHeader(req, token)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var errResp ErrorResponse
	err := json.NewDecoder(rr.Body).Decode(&errResp)
	require.NoError(t, err)

	assert.Equal(t, "parse_error", errResp.Error)
}

func TestHandleLoadProviders_InvalidPlatform(t *testing.T) {
	server, cleanup := setupAdminTest(t)
	defer cleanup()

	token := getAuthToken(t, server)

	// Invalid platform format (missing underscore)
	hcl := `
provider "hashicorp/random" {
  versions = ["3.5.0"]
  platforms = ["invalid-platform"]
}
`

	req, _ := createMultipartRequest(t, hcl)
	addAuthHeader(req, token)
	rr := httptest.NewRecorder()

	server.router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var errResp ErrorResponse
	err := json.NewDecoder(rr.Body).Decode(&errResp)
	require.NoError(t, err)

	assert.Equal(t, "parse_error", errResp.Error)
}
