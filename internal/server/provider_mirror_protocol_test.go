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

// TestMirrorProtocol_IndexJSON tests the /{hostname}/{namespace}/{type}/index.json endpoint
func TestMirrorProtocol_IndexJSON(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()
	repo := database.NewProviderRepository(srv.db)

	// Insert multiple versions of the same provider
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
			Platform:  "darwin_arm64",
			Filename:  "terraform-provider-aws_5.2.0_darwin_arm64.zip",
			Shasum:    "ghi789",
			S3Key:     "providers/hashicorp/aws/5.2.0/darwin_arm64.zip",
		},
	}

	for _, p := range providers {
		err := repo.Create(ctx, &p)
		require.NoError(t, err)
	}

	tests := []struct {
		name           string
		hostname       string
		namespace      string
		providerType   string
		expectedStatus int
		expectedCount  int
	}{
		{
			name:           "valid provider with registry.terraform.io",
			hostname:       "registry.terraform.io",
			namespace:      "hashicorp",
			providerType:   "aws",
			expectedStatus: http.StatusOK,
			expectedCount:  3,
		},
		{
			name:           "valid provider with custom hostname",
			hostname:       "example.com",
			namespace:      "hashicorp",
			providerType:   "aws",
			expectedStatus: http.StatusOK,
			expectedCount:  3,
		},
		{
			name:           "non-existent provider",
			hostname:       "registry.terraform.io",
			namespace:      "hashicorp",
			providerType:   "nonexistent",
			expectedStatus: http.StatusNotFound,
			expectedCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/" + tt.hostname + "/" + tt.namespace + "/" + tt.providerType + "/index.json"
			req := httptest.NewRequest(http.MethodGet, url, nil)
			w := httptest.NewRecorder()

			srv.Router().ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				err := json.NewDecoder(w.Body).Decode(&response)
				require.NoError(t, err)

				versions, ok := response["versions"].(map[string]interface{})
				require.True(t, ok, "response should have versions map")
				assert.Len(t, versions, tt.expectedCount, "should have correct number of versions")

				// Verify versions are present with empty objects
				for version := range versions {
					assert.NotEmpty(t, version, "version should not be empty")
				}
			}
		})
	}
}

// TestMirrorProtocol_VersionJSON tests the /{hostname}/{namespace}/{type}/{version}.json endpoint
func TestMirrorProtocol_VersionJSON(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()
	repo := database.NewProviderRepository(srv.db)

	// Insert providers with multiple platforms
	providers := []database.Provider{
		{
			Namespace: "hashicorp",
			Type:      "random",
			Version:   "3.5.0",
			Platform:  "linux_amd64",
			Filename:  "terraform-provider-random_3.5.0_linux_amd64.zip",
			Shasum:    "abc123def456",
			S3Key:     "providers/hashicorp/random/3.5.0/linux_amd64.zip",
		},
		{
			Namespace: "hashicorp",
			Type:      "random",
			Version:   "3.5.0",
			Platform:  "darwin_arm64",
			Filename:  "terraform-provider-random_3.5.0_darwin_arm64.zip",
			Shasum:    "def456abc123",
			S3Key:     "providers/hashicorp/random/3.5.0/darwin_arm64.zip",
		},
		{
			Namespace: "hashicorp",
			Type:      "random",
			Version:   "3.5.0",
			Platform:  "windows_amd64",
			Filename:  "terraform-provider-random_3.5.0_windows_amd64.zip",
			Shasum:    "789abc123def",
			S3Key:     "providers/hashicorp/random/3.5.0/windows_amd64.zip",
		},
	}

	for _, p := range providers {
		err := repo.Create(ctx, &p)
		require.NoError(t, err)
	}

	tests := []struct {
		name              string
		hostname          string
		namespace         string
		providerType      string
		version           string
		expectedStatus    int
		expectedPlatforms int
	}{
		{
			name:              "valid version with multiple platforms",
			hostname:          "registry.terraform.io",
			namespace:         "hashicorp",
			providerType:      "random",
			version:           "3.5.0",
			expectedStatus:    http.StatusOK,
			expectedPlatforms: 3,
		},
		{
			name:              "valid version with custom hostname",
			hostname:          "mirror.company.com",
			namespace:         "hashicorp",
			providerType:      "random",
			version:           "3.5.0",
			expectedStatus:    http.StatusOK,
			expectedPlatforms: 3,
		},
		{
			name:              "non-existent version",
			hostname:          "registry.terraform.io",
			namespace:         "hashicorp",
			providerType:      "random",
			version:           "99.99.99",
			expectedStatus:    http.StatusNotFound,
			expectedPlatforms: 0,
		},
		{
			name:              "non-existent provider",
			hostname:          "registry.terraform.io",
			namespace:         "hashicorp",
			providerType:      "nonexistent",
			version:           "1.0.0",
			expectedStatus:    http.StatusNotFound,
			expectedPlatforms: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/" + tt.hostname + "/" + tt.namespace + "/" + tt.providerType + "/" + tt.version + ".json"
			req := httptest.NewRequest(http.MethodGet, url, nil)
			w := httptest.NewRecorder()

			srv.Router().ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				err := json.NewDecoder(w.Body).Decode(&response)
				require.NoError(t, err)

				archives, ok := response["archives"].(map[string]interface{})
				require.True(t, ok, "response should have archives map")
				assert.Len(t, archives, tt.expectedPlatforms, "should have correct number of platforms")

				// Verify each archive has required fields
				for platform, archiveData := range archives {
					assert.NotEmpty(t, platform, "platform should not be empty")

					archive, ok := archiveData.(map[string]interface{})
					require.True(t, ok, "archive should be a map")

					url, ok := archive["url"].(string)
					require.True(t, ok, "archive should have url string")
					assert.NotEmpty(t, url, "URL should be generated")

					hashes, ok := archive["hashes"].([]interface{})
					require.True(t, ok, "archive should have hashes array")
					assert.NotEmpty(t, hashes, "hashes should be present")

					// Verify hash format (zh:hex)
					for _, hashInterface := range hashes {
						hash, ok := hashInterface.(string)
						require.True(t, ok, "hash should be string")
						assert.Contains(t, hash, "zh:", "hash should have zh: prefix")
					}
				}
			}
		})
	}
}

// TestMirrorProtocol_PathParsing tests the catchall handler's path parsing logic
func TestMirrorProtocol_PathParsing(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()
	repo := database.NewProviderRepository(srv.db)

	// Insert a test provider
	provider := &database.Provider{
		Namespace: "hashicorp",
		Type:      "aws",
		Version:   "5.0.0",
		Platform:  "linux_amd64",
		Filename:  "terraform-provider-aws_5.0.0_linux_amd64.zip",
		Shasum:    "abc123",
		S3Key:     "providers/hashicorp/aws/5.0.0/linux_amd64.zip",
	}

	err := repo.Create(ctx, provider)
	require.NoError(t, err)

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		description    string
	}{
		{
			name:           "index.json path",
			path:           "/registry.terraform.io/hashicorp/aws/index.json",
			expectedStatus: http.StatusOK,
			description:    "should route to versions handler",
		},
		{
			name:           "version.json path",
			path:           "/registry.terraform.io/hashicorp/aws/5.0.0.json",
			expectedStatus: http.StatusOK,
			description:    "should route to packages handler",
		},
		{
			name:           "malformed path - too short",
			path:           "/registry.terraform.io/hashicorp",
			expectedStatus: http.StatusNotFound,
			description:    "should reject incomplete paths",
		},
		{
			name:           "path without .json suffix",
			path:           "/registry.terraform.io/hashicorp/aws/5.0.0",
			expectedStatus: http.StatusNotFound,
			description:    "should only match .json paths",
		},
		{
			name:           "empty namespace",
			path:           "//namespace/type/index.json",
			expectedStatus: http.StatusNotFound,
			description:    "should reject empty hostname",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()

			srv.Router().ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code, tt.description)
		})
	}
}

// TestMirrorProtocol_MultipleHostnames tests that different hostnames work correctly
func TestMirrorProtocol_MultipleHostnames(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()
	repo := database.NewProviderRepository(srv.db)

	provider := &database.Provider{
		Namespace: "hashicorp",
		Type:      "aws",
		Version:   "5.0.0",
		Platform:  "linux_amd64",
		Filename:  "terraform-provider-aws_5.0.0_linux_amd64.zip",
		Shasum:    "abc123",
		S3Key:     "providers/hashicorp/aws/5.0.0/linux_amd64.zip",
	}

	err := repo.Create(ctx, provider)
	require.NoError(t, err)

	hostnames := []string{
		"registry.terraform.io",
		"mirror.company.com",
		"terraform-mirror.local",
		"example.com",
	}

	for _, hostname := range hostnames {
		t.Run("hostname_"+hostname, func(t *testing.T) {
			// Test index.json
			indexURL := "/" + hostname + "/hashicorp/aws/index.json"
			req := httptest.NewRequest(http.MethodGet, indexURL, nil)
			w := httptest.NewRecorder()

			srv.Router().ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code, "index.json should work with any hostname")

			// Test version.json
			versionURL := "/" + hostname + "/hashicorp/aws/5.0.0.json"
			req = httptest.NewRequest(http.MethodGet, versionURL, nil)
			w = httptest.NewRecorder()

			srv.Router().ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code, "version.json should work with any hostname")
		})
	}
}

// TestMirrorProtocol_EmptyResults tests handling of providers with no versions
func TestMirrorProtocol_EmptyResults(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	// Don't insert any providers - test empty database response

	req := httptest.NewRequest(http.MethodGet, "/registry.terraform.io/hashicorp/nonexistent/index.json", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code, "should return 404 for non-existent provider")
}

// TestMirrorProtocol_HashFormat tests that hashes are in correct zh:hex format
func TestMirrorProtocol_HashFormat(t *testing.T) {
	srv, cleanup := setupTestServer(t)
	defer cleanup()

	ctx := context.Background()
	repo := database.NewProviderRepository(srv.db)

	provider := &database.Provider{
		Namespace: "hashicorp",
		Type:      "random",
		Version:   "3.5.0",
		Platform:  "linux_amd64",
		Filename:  "terraform-provider-random_3.5.0_linux_amd64.zip",
		Shasum:    "abcdef1234567890",
		S3Key:     "providers/hashicorp/random/3.5.0/linux_amd64.zip",
	}

	err := repo.Create(ctx, provider)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/registry.terraform.io/hashicorp/random/3.5.0.json", nil)
	w := httptest.NewRecorder()

	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)

	archives, ok := response["archives"].(map[string]interface{})
	require.True(t, ok, "response should have archives map")

	archive, exists := archives["linux_amd64"]
	require.True(t, exists, "linux_amd64 platform should exist")

	archiveMap, ok := archive.(map[string]interface{})
	require.True(t, ok, "archive should be a map")

	hashesInterface, ok := archiveMap["hashes"].([]interface{})
	require.True(t, ok, "archive should have hashes array")
	require.NotEmpty(t, hashesInterface, "hashes should not be empty")

	// Verify hash format
	hash, ok := hashesInterface[0].(string)
	require.True(t, ok, "hash should be string")
	assert.Contains(t, hash, "zh:", "hash should have zh: prefix")
	assert.Equal(t, "zh:abcdef1234567890", hash, "hash should be in zh:hex format")
}
