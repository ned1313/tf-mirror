package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ned1313/terraform-mirror/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockStorage implements storage.Storage interface for testing
type mockStorage struct {
	data map[string][]byte
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		data: make(map[string][]byte),
	}
}

func (m *mockStorage) Upload(ctx context.Context, key string, reader io.Reader, contentType string, metadata map[string]string) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	m.data[key] = data
	return nil
}

func (m *mockStorage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	return nil, nil
}

func (m *mockStorage) Delete(ctx context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func (m *mockStorage) Exists(ctx context.Context, key string) (bool, error) {
	_, ok := m.data[key]
	return ok, nil
}

func (m *mockStorage) GetPresignedURL(ctx context.Context, key string, expiration time.Duration) (string, error) {
	return "http://example.com/presigned/" + key, nil
}

func (m *mockStorage) GetMetadata(ctx context.Context, key string) (map[string]string, error) {
	return nil, nil
}

func (m *mockStorage) ListObjects(ctx context.Context, prefix string) ([]string, error) {
	return nil, nil
}

func (m *mockStorage) GetObjectSize(ctx context.Context, key string) (int64, error) {
	if data, ok := m.data[key]; ok {
		return int64(len(data)), nil
	}
	return 0, nil
}

func (m *mockStorage) Close() error {
	return nil
}

func setupServiceTest(t *testing.T) (*Service, *database.DB, func()) {
	// Create test database
	db, err := database.New(":memory:")
	require.NoError(t, err)

	// Create mock storage
	mockStore := newMockStorage()

	// Create service
	svc := NewService(mockStore, db)

	cleanup := func() {
		db.Close()
	}

	return svc, db, cleanup
}

func TestNewService(t *testing.T) {
	svc, _, cleanup := setupServiceTest(t)
	defer cleanup()

	require.NotNil(t, svc)
	assert.NotNil(t, svc.registry)
	assert.NotNil(t, svc.storage)
	assert.NotNil(t, svc.db)
}

func TestLoadFromDefinitions_Success(t *testing.T) {
	// Create mock provider data
	providerZip := []byte("fake-provider-binary")
	hash := sha256.Sum256(providerZip)
	shasum := hex.EncodeToString(hash[:])

	// Create mock registry server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/providers/hashicorp/aws/5.0.0/download/linux/amd64" {
			resp := registryDownloadResponse{
				OS:          "linux",
				Arch:        "amd64",
				Filename:    "terraform-provider-aws_5.0.0_linux_amd64.zip",
				DownloadURL: "http://" + r.Host + "/download",
				Shasum:      shasum,
			}
			json.NewEncoder(w).Encode(resp)
		} else if r.URL.Path == "/download" {
			w.Write(providerZip)
		}
	}))
	defer server.Close()

	// Setup service with custom registry client
	svc, db, cleanup := setupServiceTest(t)
	defer cleanup()

	svc.registry = &RegistryClient{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		baseURL:    server.URL + "/v1/providers",
	}

	// Create definitions
	defs := &ProviderDefinitions{
		Providers: []*ProviderDefinition{
			{
				Namespace: "hashicorp",
				Type:      "aws",
				Versions:  []string{"5.0.0"},
				Platforms: []string{"linux_amd64"},
			},
		},
	}

	// Load providers
	results, err := svc.LoadFromDefinitions(context.Background(), defs)
	require.NoError(t, err)
	require.Len(t, results, 1)

	// Check result
	result := results[0]
	assert.True(t, result.Success)
	assert.False(t, result.Skipped)
	assert.NoError(t, result.Error)
	assert.Equal(t, "hashicorp", result.Namespace)
	assert.Equal(t, "aws", result.Type)
	assert.Equal(t, "5.0.0", result.Version)
	assert.Equal(t, "linux_amd64", result.Platform)

	// Verify database entry
	repo := database.NewProviderRepository(db)
	provider, err := repo.GetByIdentity(context.Background(), "hashicorp", "aws", "5.0.0", "linux_amd64")
	require.NoError(t, err)
	require.NotNil(t, provider)
	assert.Equal(t, "hashicorp", provider.Namespace)
	assert.Equal(t, "aws", provider.Type)
	assert.Equal(t, "5.0.0", provider.Version)
	assert.Equal(t, "linux_amd64", provider.Platform)
	assert.Equal(t, shasum, provider.Shasum)
	assert.Contains(t, provider.S3Key, "providers/hashicorp/aws/5.0.0/linux_amd64")

	// Verify storage
	mockStore := svc.storage.(*mockStorage)
	assert.Contains(t, mockStore.data, provider.S3Key)
	assert.Equal(t, providerZip, mockStore.data[provider.S3Key])
}

func TestLoadFromDefinitions_MultipleProviders(t *testing.T) {
	providerZip := []byte("fake-provider")
	hash := sha256.Sum256(providerZip)
	shasum := hex.EncodeToString(hash[:])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/download/") {
			// Parse path to extract os/arch
			parts := strings.Split(r.URL.Path, "/")
			os := parts[len(parts)-2]
			arch := parts[len(parts)-1]

			resp := registryDownloadResponse{
				OS:          os,
				Arch:        arch,
				Filename:    "provider.zip",
				DownloadURL: "http://" + r.Host + "/download",
				Shasum:      shasum,
			}
			json.NewEncoder(w).Encode(resp)
		} else if r.URL.Path == "/download" {
			w.Write(providerZip)
		}
	}))
	defer server.Close()

	svc, _, cleanup := setupServiceTest(t)
	defer cleanup()

	svc.registry = &RegistryClient{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		baseURL:    server.URL + "/v1/providers",
	}

	defs := &ProviderDefinitions{
		Providers: []*ProviderDefinition{
			{
				Namespace: "hashicorp",
				Type:      "aws",
				Versions:  []string{"5.0.0", "5.1.0"},
				Platforms: []string{"linux_amd64", "darwin_amd64"},
			},
		},
	}

	results, err := svc.LoadFromDefinitions(context.Background(), defs)
	require.NoError(t, err)

	// Should have 2 versions × 2 platforms = 4 results
	assert.Len(t, results, 4)

	// All should succeed
	for _, r := range results {
		assert.True(t, r.Success, "Expected success for %s/%s %s %s", r.Namespace, r.Type, r.Version, r.Platform)
	}
}

func TestLoadFromDefinitions_SkipsExisting(t *testing.T) {
	svc, db, cleanup := setupServiceTest(t)
	defer cleanup()

	// Pre-create a provider in the database
	repo := database.NewProviderRepository(db)
	existing := &database.Provider{
		Namespace: "hashicorp",
		Type:      "aws",
		Version:   "5.0.0",
		Platform:  "linux_amd64",
		Filename:  "existing.zip",
		Shasum:    "abc123",
		S3Key:     "providers/hashicorp/aws/5.0.0/linux_amd64/existing.zip",
	}
	err := repo.Create(context.Background(), existing)
	require.NoError(t, err)

	defs := &ProviderDefinitions{
		Providers: []*ProviderDefinition{
			{
				Namespace: "hashicorp",
				Type:      "aws",
				Versions:  []string{"5.0.0"},
				Platforms: []string{"linux_amd64"},
			},
		},
	}

	results, err := svc.LoadFromDefinitions(context.Background(), defs)
	require.NoError(t, err)
	require.Len(t, results, 1)

	result := results[0]
	assert.True(t, result.Success)
	assert.True(t, result.Skipped)
	assert.NoError(t, result.Error)
}

func TestLoadFromDefinitions_DownloadFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	svc, _, cleanup := setupServiceTest(t)
	defer cleanup()

	svc.registry = &RegistryClient{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		baseURL:    server.URL + "/v1/providers",
	}

	defs := &ProviderDefinitions{
		Providers: []*ProviderDefinition{
			{
				Namespace: "hashicorp",
				Type:      "nonexistent",
				Versions:  []string{"1.0.0"},
				Platforms: []string{"linux_amd64"},
			},
		},
	}

	results, err := svc.LoadFromDefinitions(context.Background(), defs)
	require.NoError(t, err)
	require.Len(t, results, 1)

	result := results[0]
	assert.False(t, result.Success)
	assert.False(t, result.Skipped)
	assert.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "download failed")
}

func TestLoadFromDefinitions_InvalidPlatform(t *testing.T) {
	svc, _, cleanup := setupServiceTest(t)
	defer cleanup()

	defs := &ProviderDefinitions{
		Providers: []*ProviderDefinition{
			{
				Namespace: "hashicorp",
				Type:      "aws",
				Versions:  []string{"5.0.0"},
				Platforms: []string{"invalid-platform"}, // Missing underscore
			},
		},
	}

	results, err := svc.LoadFromDefinitions(context.Background(), defs)
	require.NoError(t, err)
	require.Len(t, results, 1)

	result := results[0]
	assert.False(t, result.Success)
	assert.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "invalid platform format")
}

func TestBuildS3Key(t *testing.T) {
	svc, _, cleanup := setupServiceTest(t)
	defer cleanup()

	key := svc.buildS3Key("hashicorp", "aws", "5.0.0", "linux_amd64", "terraform-provider-aws_5.0.0_linux_amd64.zip")
	assert.Equal(t, "providers/hashicorp/aws/5.0.0/linux_amd64/terraform-provider-aws_5.0.0_linux_amd64.zip", key)
}

func TestCalculateStats(t *testing.T) {
	results := []*LoadResult{
		{Success: true, Skipped: false},
		{Success: true, Skipped: true},
		{Success: false, Error: fmt.Errorf("error 1")},
		{Success: false, Error: fmt.Errorf("error 2"), Namespace: "ns", Type: "type", Version: "1.0.0", Platform: "linux_amd64"},
		{Success: true, Skipped: false},
	}

	stats := CalculateStats(results)
	assert.Equal(t, 5, stats.Total)
	assert.Equal(t, 3, stats.Success) // 2 success + 1 skipped
	assert.Equal(t, 2, stats.Failed)
	assert.Equal(t, 1, stats.Skipped)
	assert.Len(t, stats.Errors, 2)
	assert.Contains(t, stats.Errors[1], "ns/type 1.0.0 linux_amd64: error 2")
}

func TestCalculateStats_EmptyResults(t *testing.T) {
	stats := CalculateStats([]*LoadResult{})
	assert.Equal(t, 0, stats.Total)
	assert.Equal(t, 0, stats.Success)
	assert.Equal(t, 0, stats.Failed)
	assert.Equal(t, 0, stats.Skipped)
	assert.Empty(t, stats.Errors)
}
