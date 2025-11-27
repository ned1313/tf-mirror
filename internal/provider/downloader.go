package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// TerraformRegistryBaseURL is the base URL for the Terraform Registry API
	TerraformRegistryBaseURL = "https://registry.terraform.io/v1/providers"

	// DownloadTimeout is the timeout for downloading provider files
	DownloadTimeout = 5 * time.Minute

	// MaxRetries is the maximum number of retry attempts
	MaxRetries = 3

	// RetryDelay is the initial delay between retries
	RetryDelay = 1 * time.Second
)

// RegistryDownloader is an interface for downloading providers from a registry
type RegistryDownloader interface {
	// DownloadProviderComplete performs the complete download workflow
	DownloadProviderComplete(ctx context.Context, namespace, providerType, version, os, arch string) *DownloadResult
}

// RegistryClient handles communication with the Terraform Registry API
type RegistryClient struct {
	httpClient *http.Client
	baseURL    string
}

// NewRegistryClient creates a new Terraform Registry API client
func NewRegistryClient() *RegistryClient {
	return &RegistryClient{
		httpClient: &http.Client{
			Timeout: DownloadTimeout,
		},
		baseURL: TerraformRegistryBaseURL,
	}
}

// ProviderDownloadInfo contains information needed to download a provider
type ProviderDownloadInfo struct {
	Namespace   string
	Type        string
	Version     string
	OS          string
	Arch        string
	Platform    string // os_arch format
	Filename    string
	DownloadURL string
	Shasum      string
}

// registryDownloadResponse represents the API response from the download endpoint
type registryDownloadResponse struct {
	Protocols   []string `json:"protocols"`
	OS          string   `json:"os"`
	Arch        string   `json:"arch"`
	Filename    string   `json:"filename"`
	DownloadURL string   `json:"download_url"`
	Shasum      string   `json:"shasum"`
}

// GetDownloadInfo retrieves download metadata from the Terraform Registry
func (c *RegistryClient) GetDownloadInfo(ctx context.Context, namespace, providerType, version, os, arch string) (*ProviderDownloadInfo, error) {
	// Construct URL: /v1/providers/{namespace}/{type}/{version}/download/{os}/{arch}
	url := fmt.Sprintf("%s/%s/%s/%s/download/%s/%s",
		c.baseURL, namespace, providerType, version, os, arch)

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Execute with retries
	var resp *http.Response
	var lastErr error

	for attempt := 0; attempt < MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			delay := RetryDelay * time.Duration(1<<uint(attempt-1))
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		resp, lastErr = c.httpClient.Do(req)
		if lastErr == nil {
			break
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("failed after %d attempts: %w", MaxRetries, lastErr)
	}
	defer resp.Body.Close()

	// Check status
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("provider not found: %s/%s %s for %s_%s", namespace, providerType, version, os, arch)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("registry returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var data registryDownloadResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &ProviderDownloadInfo{
		Namespace:   namespace,
		Type:        providerType,
		Version:     version,
		OS:          data.OS,
		Arch:        data.Arch,
		Platform:    data.OS + "_" + data.Arch,
		Filename:    data.Filename,
		DownloadURL: data.DownloadURL,
		Shasum:      data.Shasum,
	}, nil
}

// DownloadProvider downloads a provider binary and verifies its checksum
func (c *RegistryClient) DownloadProvider(ctx context.Context, info *ProviderDownloadInfo) ([]byte, error) {
	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, info.DownloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}

	// Execute with retries
	var resp *http.Response
	var lastErr error

	for attempt := 0; attempt < MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			delay := RetryDelay * time.Duration(1<<uint(attempt-1))
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		resp, lastErr = c.httpClient.Do(req)
		if lastErr == nil {
			break
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("download failed after %d attempts: %w", MaxRetries, lastErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	// Read the entire file
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read download: %w", err)
	}

	// Calculate checksum
	hash := sha256.Sum256(data)
	actualSum := hex.EncodeToString(hash[:])

	// Verify checksum
	if !strings.EqualFold(actualSum, info.Shasum) {
		return nil, fmt.Errorf("checksum mismatch: expected %s, got %s", info.Shasum, actualSum)
	}

	return data, nil
}

// DownloadResult represents the result of downloading a provider
type DownloadResult struct {
	Info     *ProviderDownloadInfo
	Data     []byte
	Error    error
	Duration time.Duration
}

// DownloadProviderComplete performs the complete download workflow:
// 1. Get download info from registry
// 2. Download the provider binary
// 3. Verify checksum
func (c *RegistryClient) DownloadProviderComplete(ctx context.Context, namespace, providerType, version, os, arch string) *DownloadResult {
	start := time.Now()
	result := &DownloadResult{}

	// Get download info
	info, err := c.GetDownloadInfo(ctx, namespace, providerType, version, os, arch)
	if err != nil {
		result.Error = fmt.Errorf("failed to get download info: %w", err)
		result.Duration = time.Since(start)
		return result
	}
	result.Info = info

	// Download and verify
	data, err := c.DownloadProvider(ctx, info)
	if err != nil {
		result.Error = fmt.Errorf("failed to download provider: %w", err)
		result.Duration = time.Since(start)
		return result
	}
	result.Data = data

	result.Duration = time.Since(start)
	return result
}
