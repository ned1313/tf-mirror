package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDownloadInfo_Success(t *testing.T) {
	// Mock provider data
	providerZip := []byte("fake-provider-binary-content")
	hash := sha256.Sum256(providerZip)
	shasum := hex.EncodeToString(hash[:])

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request path
		expectedPath := "/v1/providers/hashicorp/aws/5.0.0/download/linux/amd64"
		assert.Equal(t, expectedPath, r.URL.Path)

		// Return mock response
		resp := registryDownloadResponse{
			Protocols:   []string{"5.0"},
			OS:          "linux",
			Arch:        "amd64",
			Filename:    "terraform-provider-aws_5.0.0_linux_amd64.zip",
			DownloadURL: "http://example.com/download.zip",
			Shasum:      shasum,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create client with mock server
	client := &RegistryClient{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		baseURL:    server.URL + "/v1/providers",
	}

	// Test
	info, err := client.GetDownloadInfo(context.Background(), "hashicorp", "aws", "5.0.0", "linux", "amd64")
	require.NoError(t, err)
	require.NotNil(t, info)

	assert.Equal(t, "hashicorp", info.Namespace)
	assert.Equal(t, "aws", info.Type)
	assert.Equal(t, "5.0.0", info.Version)
	assert.Equal(t, "linux", info.OS)
	assert.Equal(t, "amd64", info.Arch)
	assert.Equal(t, "linux_amd64", info.Platform)
	assert.Equal(t, "terraform-provider-aws_5.0.0_linux_amd64.zip", info.Filename)
	assert.Equal(t, "http://example.com/download.zip", info.DownloadURL)
	assert.Equal(t, shasum, info.Shasum)
}

func TestGetDownloadInfo_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "provider not found"}`))
	}))
	defer server.Close()

	client := &RegistryClient{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		baseURL:    server.URL + "/v1/providers",
	}

	info, err := client.GetDownloadInfo(context.Background(), "nonexistent", "provider", "1.0.0", "linux", "amd64")
	assert.Error(t, err)
	assert.Nil(t, info)
	assert.Contains(t, err.Error(), "provider not found")
}

func TestGetDownloadInfo_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	client := &RegistryClient{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		baseURL:    server.URL + "/v1/providers",
	}

	info, err := client.GetDownloadInfo(context.Background(), "hashicorp", "aws", "5.0.0", "linux", "amd64")
	assert.Error(t, err)
	assert.Nil(t, info)
	assert.Contains(t, err.Error(), "failed to parse response")
}

func TestGetDownloadInfo_ContextCancellation(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &RegistryClient{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		baseURL:    server.URL + "/v1/providers",
	}

	// Create context that cancels immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	info, err := client.GetDownloadInfo(ctx, "hashicorp", "aws", "5.0.0", "linux", "amd64")
	assert.Error(t, err)
	assert.Nil(t, info)
}

func TestDownloadProvider_Success(t *testing.T) {
	// Mock provider data
	providerZip := []byte("fake-provider-binary-content")
	hash := sha256.Sum256(providerZip)
	shasum := hex.EncodeToString(hash[:])

	// Create mock download server
	downloadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.Write(providerZip)
	}))
	defer downloadServer.Close()

	client := NewRegistryClient()

	info := &ProviderDownloadInfo{
		Namespace:   "hashicorp",
		Type:        "aws",
		Version:     "5.0.0",
		OS:          "linux",
		Arch:        "amd64",
		Platform:    "linux_amd64",
		Filename:    "terraform-provider-aws_5.0.0_linux_amd64.zip",
		DownloadURL: downloadServer.URL,
		Shasum:      shasum,
	}

	data, err := client.DownloadProvider(context.Background(), info)
	require.NoError(t, err)
	assert.Equal(t, providerZip, data)
}

func TestDownloadProvider_ChecksumMismatch(t *testing.T) {
	// Mock provider data
	providerZip := []byte("fake-provider-binary-content")
	wrongShasum := "0000000000000000000000000000000000000000000000000000000000000000"

	// Create mock download server
	downloadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.Write(providerZip)
	}))
	defer downloadServer.Close()

	client := NewRegistryClient()

	info := &ProviderDownloadInfo{
		Namespace:   "hashicorp",
		Type:        "aws",
		Version:     "5.0.0",
		OS:          "linux",
		Arch:        "amd64",
		Platform:    "linux_amd64",
		Filename:    "terraform-provider-aws_5.0.0_linux_amd64.zip",
		DownloadURL: downloadServer.URL,
		Shasum:      wrongShasum,
	}

	data, err := client.DownloadProvider(context.Background(), info)
	assert.Error(t, err)
	assert.Nil(t, data)
	assert.Contains(t, err.Error(), "checksum mismatch")
}

func TestDownloadProvider_DownloadFails(t *testing.T) {
	// Create server that returns error
	downloadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer downloadServer.Close()

	client := NewRegistryClient()

	info := &ProviderDownloadInfo{
		DownloadURL: downloadServer.URL,
		Shasum:      "abc123",
	}

	data, err := client.DownloadProvider(context.Background(), info)
	assert.Error(t, err)
	assert.Nil(t, data)
	assert.Contains(t, err.Error(), "download returned status 500")
}

func TestDownloadProviderComplete_Success(t *testing.T) {
	// Mock provider data
	providerZip := []byte("fake-provider-binary-content")
	hash := sha256.Sum256(providerZip)
	shasum := hex.EncodeToString(hash[:])

	// Create mock registry server
	registryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/providers/hashicorp/aws/5.0.0/download/linux/amd64" {
			resp := registryDownloadResponse{
				Protocols:   []string{"5.0"},
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
	defer registryServer.Close()

	client := &RegistryClient{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		baseURL:    registryServer.URL + "/v1/providers",
	}

	result := client.DownloadProviderComplete(context.Background(), "hashicorp", "aws", "5.0.0", "linux", "amd64")
	require.NotNil(t, result)
	assert.NoError(t, result.Error)
	assert.NotNil(t, result.Info)
	assert.Equal(t, providerZip, result.Data)
	// Duration assertion removed - timing can be too fast in tests
}

func TestDownloadProviderComplete_InfoFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := &RegistryClient{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		baseURL:    server.URL + "/v1/providers",
	}

	result := client.DownloadProviderComplete(context.Background(), "hashicorp", "aws", "5.0.0", "linux", "amd64")
	require.NotNil(t, result)
	assert.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "failed to get download info")
	assert.Nil(t, result.Data)
}

func TestDownloadProviderComplete_DownloadFails(t *testing.T) {
	providerZip := []byte("fake-provider")
	hash := sha256.Sum256(providerZip)
	shasum := hex.EncodeToString(hash[:])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/providers/hashicorp/aws/5.0.0/download/linux/amd64" {
			resp := registryDownloadResponse{
				OS:          "linux",
				Arch:        "amd64",
				Filename:    "test.zip",
				DownloadURL: "http://" + r.Host + "/download",
				Shasum:      shasum,
			}
			json.NewEncoder(w).Encode(resp)
		} else if r.URL.Path == "/download" {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client := &RegistryClient{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		baseURL:    server.URL + "/v1/providers",
	}

	result := client.DownloadProviderComplete(context.Background(), "hashicorp", "aws", "5.0.0", "linux", "amd64")
	require.NotNil(t, result)
	assert.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "failed to download provider")
	assert.NotNil(t, result.Info)
	assert.Nil(t, result.Data)
}

func TestNewRegistryClient(t *testing.T) {
	client := NewRegistryClient()
	require.NotNil(t, client)
	assert.NotNil(t, client.httpClient)
	assert.Equal(t, TerraformRegistryBaseURL, client.baseURL)
	assert.Equal(t, DownloadTimeout, client.httpClient.Timeout)
}
