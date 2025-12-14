package module

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// DefaultUpstreamRegistry is the default upstream module registry
	DefaultUpstreamRegistry = "registry.terraform.io"

	// DownloadTimeout is the timeout for downloading module files
	DownloadTimeout = 5 * time.Minute

	// MaxRetries is the maximum number of retry attempts
	MaxRetries = 3

	// RetryDelay is the initial delay between retries
	RetryDelay = 1 * time.Second
)

// RegistryDownloader is an interface for downloading modules from a registry
type RegistryDownloader interface {
	// GetAvailableVersions retrieves available versions from the registry
	GetAvailableVersions(ctx context.Context, namespace, name, system string) ([]string, error)
	// GetDownloadURL retrieves the download URL for a specific module version
	GetDownloadURL(ctx context.Context, namespace, name, system, version string) (string, error)
	// DownloadModule downloads a module from a URL
	DownloadModule(ctx context.Context, downloadURL string) ([]byte, error)
	// DownloadModuleComplete performs the complete download workflow
	DownloadModuleComplete(ctx context.Context, namespace, name, system, version string) *DownloadResult
}

// RegistryClient handles communication with the Terraform Module Registry API
type RegistryClient struct {
	httpClient       *http.Client
	upstreamRegistry string
	gitDownloader    *GitDownloader
}

// NewRegistryClient creates a new Module Registry API client
func NewRegistryClient(upstreamRegistry string) *RegistryClient {
	if upstreamRegistry == "" {
		upstreamRegistry = DefaultUpstreamRegistry
	}
	return &RegistryClient{
		httpClient: &http.Client{
			Timeout: DownloadTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Don't follow redirects for the download endpoint
				// We need to capture the X-Terraform-Get header
				return http.ErrUseLastResponse
			},
		},
		upstreamRegistry: upstreamRegistry,
		gitDownloader:    NewGitDownloader(),
	}
}

// ModuleDownloadInfo contains information about a module download
type ModuleDownloadInfo struct {
	Namespace   string
	Name        string
	System      string
	Version     string
	DownloadURL string
	Filename    string
}

// registryVersionsResponse represents the API response from the versions endpoint
type registryVersionsResponse struct {
	Modules []struct {
		Versions []struct {
			Version string `json:"version"`
		} `json:"versions"`
	} `json:"modules"`
}

// GetAvailableVersions retrieves available versions from the Terraform Module Registry
func (c *RegistryClient) GetAvailableVersions(ctx context.Context, namespace, name, system string) ([]string, error) {
	// Construct URL: /v1/modules/{namespace}/{name}/{system}/versions
	url := fmt.Sprintf("https://%s/v1/modules/%s/%s/%s/versions",
		c.upstreamRegistry, namespace, name, system)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query versions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("module not found: %s/%s/%s", namespace, name, system)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("registry returned status %d: %s", resp.StatusCode, string(body))
	}

	var data registryVersionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(data.Modules) == 0 || len(data.Modules[0].Versions) == 0 {
		return nil, fmt.Errorf("no versions found for module: %s/%s/%s", namespace, name, system)
	}

	versions := make([]string, 0, len(data.Modules[0].Versions))
	for _, v := range data.Modules[0].Versions {
		versions = append(versions, v.Version)
	}

	return versions, nil
}

// GetDownloadURL retrieves the download URL for a specific module version
// The module registry returns a 204 No Content with X-Terraform-Get header
func (c *RegistryClient) GetDownloadURL(ctx context.Context, namespace, name, system, version string) (string, error) {
	// Construct URL: /v1/modules/{namespace}/{name}/{system}/{version}/download
	url := fmt.Sprintf("https://%s/v1/modules/%s/%s/%s/%s/download",
		c.upstreamRegistry, namespace, name, system, version)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
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
				return "", ctx.Err()
			}
		}

		resp, lastErr = c.httpClient.Do(req)
		if lastErr == nil {
			break
		}
	}

	if lastErr != nil {
		return "", fmt.Errorf("failed after %d attempts: %w", MaxRetries, lastErr)
	}
	defer resp.Body.Close()

	// Check status - expect 204 No Content or 200 OK
	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("module version not found: %s/%s/%s v%s", namespace, name, system, version)
	}

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("registry returned status %d: %s", resp.StatusCode, string(body))
	}

	// Get download URL from X-Terraform-Get header
	downloadURL := resp.Header.Get("X-Terraform-Get")
	if downloadURL == "" {
		return "", fmt.Errorf("no X-Terraform-Get header in response")
	}

	return downloadURL, nil
}

// DownloadModule downloads a module from the given URL
// Supports both HTTP/HTTPS URLs and Git URLs (git::https://...)
func (c *RegistryClient) DownloadModule(ctx context.Context, downloadURL string) ([]byte, error) {
	// Check if this is a Git URL
	if IsGitURL(downloadURL) {
		return c.gitDownloader.DownloadFromGit(ctx, downloadURL)
	}

	// Handle HTTP/HTTPS download
	// Create a new client without redirect restriction for actual download
	downloadClient := &http.Client{
		Timeout: DownloadTimeout,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
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

		resp, lastErr = downloadClient.Do(req)
		if lastErr == nil {
			break
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("download failed after %d attempts: %w", MaxRetries, lastErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("download returned status %d: %s", resp.StatusCode, string(body))
	}

	// Read the entire file
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read download: %w", err)
	}

	return data, nil
}

// DownloadResult represents the result of downloading a module
type DownloadResult struct {
	Info     *ModuleDownloadInfo
	Data     []byte
	Error    error
	Duration time.Duration
}

// DownloadModuleComplete performs the complete download workflow:
// 1. Get download URL from registry
// 2. Download the module tarball
func (c *RegistryClient) DownloadModuleComplete(ctx context.Context, namespace, name, system, version string) *DownloadResult {
	start := time.Now()
	result := &DownloadResult{
		Info: &ModuleDownloadInfo{
			Namespace: namespace,
			Name:      name,
			System:    system,
			Version:   version,
		},
	}

	// Get download URL
	downloadURL, err := c.GetDownloadURL(ctx, namespace, name, system, version)
	if err != nil {
		result.Error = fmt.Errorf("failed to get download URL: %w", err)
		result.Duration = time.Since(start)
		return result
	}
	result.Info.DownloadURL = downloadURL
	result.Info.Filename = fmt.Sprintf("%s-%s-%s-%s.tar.gz", namespace, name, system, version)

	// Download module
	data, err := c.DownloadModule(ctx, downloadURL)
	if err != nil {
		result.Error = fmt.Errorf("failed to download module: %w", err)
		result.Duration = time.Since(start)
		return result
	}
	result.Data = data

	result.Duration = time.Since(start)
	return result
}
