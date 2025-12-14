package module

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// GitURLInfo contains parsed information from a Git URL
type GitURLInfo struct {
	// OriginalURL is the full URL with git:: prefix
	OriginalURL string
	// CloneURL is the URL to use for cloning (without git:: prefix)
	CloneURL string
	// Ref is the git reference (branch, tag, commit) to checkout
	Ref string
	// Subdir is the subdirectory within the repo (if specified with //)
	Subdir string
}

// ParseGitURL parses a Terraform-style git URL into its components
// Formats supported:
//   - git::https://github.com/owner/repo
//   - git::https://github.com/owner/repo?ref=v1.0.0
//   - git::https://github.com/owner/repo//subdir?ref=v1.0.0
//   - git::ssh://git@github.com/owner/repo
func ParseGitURL(rawURL string) (*GitURLInfo, error) {
	if !strings.HasPrefix(rawURL, "git::") {
		return nil, fmt.Errorf("not a git URL: must start with 'git::'")
	}

	// Remove git:: prefix
	urlWithoutPrefix := strings.TrimPrefix(rawURL, "git::")

	info := &GitURLInfo{
		OriginalURL: rawURL,
	}

	// Find the double-slash separator for subdir, but not the one in https://
	// We need to look for // that comes after the host portion
	// First, find where the scheme ends (after ://)
	schemeEnd := strings.Index(urlWithoutPrefix, "://")
	if schemeEnd == -1 {
		return nil, fmt.Errorf("invalid URL: missing scheme")
	}

	// Look for // after the scheme's ://
	afterScheme := urlWithoutPrefix[schemeEnd+3:]
	subdirIdx := strings.Index(afterScheme, "//")

	if subdirIdx != -1 {
		// Calculate the actual position in the full URL
		actualIdx := schemeEnd + 3 + subdirIdx
		baseURL := urlWithoutPrefix[:actualIdx]
		subdirPart := urlWithoutPrefix[actualIdx+2:]

		// Handle query params in subdir
		if qIdx := strings.Index(subdirPart, "?"); qIdx != -1 {
			info.Subdir = subdirPart[:qIdx]
			// Reconstruct URL with query
			urlWithoutPrefix = baseURL + subdirPart[qIdx:]
		} else {
			info.Subdir = subdirPart
			urlWithoutPrefix = baseURL
		}
	}

	// Parse the URL to extract ref query parameter
	parsedURL, err := url.Parse(urlWithoutPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	// Extract ref from query parameters
	queryParams := parsedURL.Query()
	info.Ref = queryParams.Get("ref")

	// Remove query parameters to get clean clone URL
	parsedURL.RawQuery = ""
	info.CloneURL = parsedURL.String()

	return info, nil
}

// IsGitURL checks if a URL is a Git URL (starts with git::)
func IsGitURL(rawURL string) bool {
	return strings.HasPrefix(rawURL, "git::")
}

// GitDownloader handles downloading modules from Git repositories
type GitDownloader struct{}

// NewGitDownloader creates a new Git downloader
func NewGitDownloader() *GitDownloader {
	return &GitDownloader{}
}

// DownloadFromGit clones a git repository and creates a tarball of its contents
func (g *GitDownloader) DownloadFromGit(ctx context.Context, gitURL string) ([]byte, error) {
	// Parse the Git URL
	info, err := ParseGitURL(gitURL)
	if err != nil {
		return nil, err
	}

	// Create a temporary directory for cloning
	tempDir, err := os.MkdirTemp("", "tf-module-git-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Clone the repository
	cloneOpts := &git.CloneOptions{
		URL:      info.CloneURL,
		Progress: nil, // No progress output
		Depth:    1,   // Shallow clone for efficiency
	}

	// If a specific ref is specified, we need to handle it
	if info.Ref != "" {
		// Try cloning with the ref as a tag or branch
		cloneOpts.ReferenceName = plumbing.NewTagReferenceName(info.Ref)
		cloneOpts.SingleBranch = true
	}

	repo, err := git.PlainCloneContext(ctx, tempDir, false, cloneOpts)
	if err != nil {
		// If tag clone fails, try as branch
		if info.Ref != "" {
			cloneOpts.ReferenceName = plumbing.NewBranchReferenceName(info.Ref)
			repo, err = git.PlainCloneContext(ctx, tempDir, false, cloneOpts)
		}
		if err != nil {
			// Try without specific reference and checkout after
			cloneOpts.ReferenceName = ""
			cloneOpts.SingleBranch = false
			cloneOpts.Depth = 0 // Need full clone to find arbitrary refs
			repo, err = git.PlainCloneContext(ctx, tempDir, false, cloneOpts)
			if err != nil {
				return nil, fmt.Errorf("failed to clone repository: %w", err)
			}

			// Try to checkout the specific ref
			if info.Ref != "" {
				worktree, wtErr := repo.Worktree()
				if wtErr != nil {
					return nil, fmt.Errorf("failed to get worktree: %w", wtErr)
				}

				// Try to resolve the ref
				hash, resolveErr := repo.ResolveRevision(plumbing.Revision(info.Ref))
				if resolveErr != nil {
					return nil, fmt.Errorf("failed to resolve ref '%s': %w", info.Ref, resolveErr)
				}

				checkoutErr := worktree.Checkout(&git.CheckoutOptions{
					Hash:  *hash,
					Force: true,
				})
				if checkoutErr != nil {
					return nil, fmt.Errorf("failed to checkout ref '%s': %w", info.Ref, checkoutErr)
				}
			}
		}
	}

	// Verify we have a valid repo
	if repo == nil {
		return nil, fmt.Errorf("failed to clone repository: unknown error")
	}

	// Determine the source directory (could be subdir)
	sourceDir := tempDir
	if info.Subdir != "" {
		sourceDir = filepath.Join(tempDir, info.Subdir)
		if _, err := os.Stat(sourceDir); os.IsNotExist(err) {
			return nil, fmt.Errorf("subdir '%s' not found in repository", info.Subdir)
		}
	}

	// Create tarball from the cloned directory
	tarball, err := createTarGz(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create tarball: %w", err)
	}

	return tarball, nil
}

// createTarGz creates a tar.gz archive from a directory
func createTarGz(sourceDir string) ([]byte, error) {
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip .git directory
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		// Create relative path
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		// Skip the root directory entry
		if relPath == "." {
			return nil
		}

		// Create tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		// Use forward slashes for tar compatibility
		header.Name = filepath.ToSlash(relPath)

		// Handle symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			header.Linkname = link
		}

		// Write header
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		// If it's a regular file, write its contents
		if info.Mode().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			if _, err := io.Copy(tarWriter, file); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Close writers in correct order
	if err := tarWriter.Close(); err != nil {
		return nil, err
	}
	if err := gzWriter.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
