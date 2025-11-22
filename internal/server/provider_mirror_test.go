package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ned1313/terraform-mirror/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleProviderVersions_Success(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Insert test providers into database
	ctx := context.Background()
	repo := database.NewProviderRepository(srv.db)

	// Create multiple versions of the same provider
	providers := []database.Provider{
		{
			Namespace: "hashicorp",
			Type:      "aws",
			Version:   "5.0.0",
			Platform:  "linux_amd64",
			Filename:  "terraform-provider-aws_5.0.0_linux_amd64.zip",
			Shasum:    "abc123",
			S3Key:     "providers/hashicorp/aws/5.0.0/linux_amd64.zip",
		},
		{
			Namespace: "hashicorp",
			Type:      "aws",
			Version:   "5.1.0",
			Platform:  "linux_amd64",
			Filename:  "terraform-provider-aws_5.1.0_linux_amd64.zip",
			Shasum:    "def456",
			S3Key:     "providers/hashicorp/aws/5.1.0/linux_amd64.zip",
		},
		{
			Namespace: "hashicorp",
			Type:      "aws",
			Version:   "5.2.0",
			Platform:  "linux_amd64",
			Filename:  "terraform-provider-aws_5.2.0_linux_amd64.zip",
			Shasum:    "ghi789",
			S3Key:     "providers/hashicorp/aws/5.2.0/linux_amd64.zip",
		},
	}

	for _, p := range providers {
		err := repo.Create(ctx, &p)
		require.NoError(t, err)
	}

	// Make request
	req := httptest.NewRequest(http.MethodGet, "/v1/providers/hashicorp/aws/versions", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response ProviderVersionsResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)

	// Verify all versions are present
	assert.Len(t, response.Versions, 3)
	assert.Contains(t, response.Versions, "5.0.0")
	assert.Contains(t, response.Versions, "5.1.0")
	assert.Contains(t, response.Versions, "5.2.0")
}

func TestHandleProviderVersions_NotFound(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/providers/nonexistent/provider/versions", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response ErrorResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "not_found", response.Error)
}

func TestHandleProviderDownload_Success(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Insert test provider into database
	ctx := context.Background()
	repo := database.NewProviderRepository(srv.db)

	provider := &database.Provider{
		Namespace: "hashicorp",
		Type:      "aws",
		Version:   "5.0.0",
		Platform:  "linux_amd64",
		Filename:  "terraform-provider-aws_5.0.0_linux_amd64.zip",
		Shasum:    "abc123def456",
		S3Key:     "providers/hashicorp/aws/5.0.0/linux_amd64.zip",
	}

	err := repo.Create(ctx, provider)
	require.NoError(t, err)

	// Make request
	req := httptest.NewRequest(http.MethodGet, "/v1/providers/hashicorp/aws/5.0.0/download/linux/amd64", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response ProviderDownloadResponse
	err = json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)

	// Verify response fields
	assert.Equal(t, []string{"5.0"}, response.Protocols)
	assert.Equal(t, "linux", response.OS)
	assert.Equal(t, "amd64", response.Arch)
	assert.Equal(t, "terraform-provider-aws_5.0.0_linux_amd64.zip", response.Filename)
	assert.Equal(t, "abc123def456", response.SHA256Sum)
	assert.NotEmpty(t, response.DownloadURL, "Download URL should be generated")
}

func TestHandleProviderDownload_DifferentPlatforms(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()

	// Insert providers for different platforms
	platforms := []struct {
		os   string
		arch string
	}{
		{"linux", "amd64"},
		{"linux", "arm64"},
		{"darwin", "amd64"},
		{"darwin", "arm64"},
		{"windows", "amd64"},
	}

	repo := database.NewProviderRepository(srv.db)
	for _, platform := range platforms {
		provider := &database.Provider{
			Namespace: "hashicorp",
			Type:      "random",
			Version:   "3.5.0",
			Platform:  platform.os + "_" + platform.arch,
			Filename:  "terraform-provider-random_3.5.0_" + platform.os + "_" + platform.arch + ".zip",
			Shasum:    "platform-checksum",
			S3Key:     "providers/hashicorp/random/3.5.0/" + platform.os + "_" + platform.arch + ".zip",
		}
		err := repo.Create(ctx, provider)
		require.NoError(t, err)
	}

	// Test each platform
	for _, platform := range platforms {
		t.Run(platform.os+"_"+platform.arch, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet,
				"/v1/providers/hashicorp/random/3.5.0/download/"+platform.os+"/"+platform.arch, nil)
			w := httptest.NewRecorder()

			srv.Router().ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response ProviderDownloadResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			require.NoError(t, err)

			assert.Equal(t, platform.os, response.OS)
			assert.Equal(t, platform.arch, response.Arch)
		})
	}
}

func TestHandleProviderVersions_MultipleProviders(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()

	// Insert multiple providers with different namespaces
	providers := []database.Provider{
		{Namespace: "hashicorp", Type: "aws", Version: "5.0.0", Platform: "linux_amd64",
			Filename: "f1.zip", Shasum: "s1", S3Key: "p1"},
		{Namespace: "hashicorp", Type: "aws", Version: "5.1.0", Platform: "linux_amd64",
			Filename: "f2.zip", Shasum: "s2", S3Key: "p2"},
		{Namespace: "hashicorp", Type: "azurerm", Version: "3.0.0", Platform: "linux_amd64",
			Filename: "f3.zip", Shasum: "s3", S3Key: "p3"},
		{Namespace: "terraform-aws-modules", Type: "vpc", Version: "1.0.0", Platform: "linux_amd64",
			Filename: "f4.zip", Shasum: "s4", S3Key: "p4"},
	}

	repo := database.NewProviderRepository(srv.db)
	for i := range providers {
		err := repo.Create(ctx, &providers[i])
		require.NoError(t, err)
	}

	tests := []struct {
		name           string
		namespace      string
		providerType   string
		expectedCount  int
		expectedStatus int
	}{
		{
			name:           "hashicorp/aws has 2 versions",
			namespace:      "hashicorp",
			providerType:   "aws",
			expectedCount:  2,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "hashicorp/azurerm has 1 version",
			namespace:      "hashicorp",
			providerType:   "azurerm",
			expectedCount:  1,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "nonexistent provider returns 404",
			namespace:      "hashicorp",
			providerType:   "nonexistent",
			expectedCount:  0,
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet,
				"/v1/providers/"+tt.namespace+"/"+tt.providerType+"/versions", nil)
			w := httptest.NewRecorder()

			srv.Router().ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var response ProviderVersionsResponse
				err := json.NewDecoder(w.Body).Decode(&response)
				require.NoError(t, err)
				assert.Len(t, response.Versions, tt.expectedCount)
			}
		})
	}
}
