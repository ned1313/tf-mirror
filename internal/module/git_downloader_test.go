package module

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGitURL(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantClone   string
		wantRef     string
		wantSubdir  string
		wantErr     bool
		errContains string
	}{
		{
			name:       "simple git URL",
			input:      "git::https://github.com/hashicorp/example",
			wantClone:  "https://github.com/hashicorp/example",
			wantRef:    "",
			wantSubdir: "",
		},
		{
			name:       "git URL with ref",
			input:      "git::https://github.com/hashicorp/example?ref=v1.0.0",
			wantClone:  "https://github.com/hashicorp/example",
			wantRef:    "v1.0.0",
			wantSubdir: "",
		},
		{
			name:       "git URL with subdir",
			input:      "git::https://github.com/hashicorp/example//modules/consul",
			wantClone:  "https://github.com/hashicorp/example",
			wantRef:    "",
			wantSubdir: "modules/consul",
		},
		{
			name:       "git URL with subdir and ref",
			input:      "git::https://github.com/hashicorp/example//modules/consul?ref=v2.0.0",
			wantClone:  "https://github.com/hashicorp/example",
			wantRef:    "v2.0.0",
			wantSubdir: "modules/consul",
		},
		{
			name:       "git URL with commit hash ref",
			input:      "git::https://github.com/hashicorp/example?ref=abc123def456",
			wantClone:  "https://github.com/hashicorp/example",
			wantRef:    "abc123def456",
			wantSubdir: "",
		},
		{
			name:       "git URL with branch ref",
			input:      "git::https://github.com/hashicorp/example?ref=main",
			wantClone:  "https://github.com/hashicorp/example",
			wantRef:    "main",
			wantSubdir: "",
		},
		{
			name:       "real terraform registry URL",
			input:      "git::https://github.com/hashicorp/terraform-aws-consul?ref=v0.1.0",
			wantClone:  "https://github.com/hashicorp/terraform-aws-consul",
			wantRef:    "v0.1.0",
			wantSubdir: "",
		},
		{
			name:        "not a git URL",
			input:       "https://github.com/hashicorp/example",
			wantErr:     true,
			errContains: "not a git URL",
		},
		{
			name:        "empty string",
			input:       "",
			wantErr:     true,
			errContains: "not a git URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := ParseGitURL(tt.input)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, info)

			assert.Equal(t, tt.input, info.OriginalURL)
			assert.Equal(t, tt.wantClone, info.CloneURL)
			assert.Equal(t, tt.wantRef, info.Ref)
			assert.Equal(t, tt.wantSubdir, info.Subdir)
		})
	}
}

func TestIsGitURL(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"git::https://github.com/hashicorp/example", true},
		{"git::https://github.com/owner/repo?ref=v1.0.0", true},
		{"git::ssh://git@github.com/owner/repo", true},
		{"https://github.com/hashicorp/example", false},
		{"http://example.com/module.tar.gz", false},
		{"", false},
		{"git:", false},
		{"git::", true}, // technically valid prefix, will fail on parse
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsGitURL(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGitDownloader_DownloadFromGit(t *testing.T) {
	// Skip in short mode since these tests hit the network
	if testing.Short() {
		t.Skip("Skipping network tests in short mode")
	}

	downloader := NewGitDownloader()
	ctx := context.Background()

	t.Run("download real module", func(t *testing.T) {
		// Use hashicorp/terraform-aws-consul which is the actual module
		// that caused our original issue
		gitURL := "git::https://github.com/hashicorp/terraform-aws-consul?ref=v0.11.0"

		data, err := downloader.DownloadFromGit(ctx, gitURL)
		require.NoError(t, err)
		require.NotEmpty(t, data)

		// Verify it's a valid gzip file (starts with magic bytes)
		assert.Equal(t, byte(0x1f), data[0])
		assert.Equal(t, byte(0x8b), data[1])
	})

	t.Run("nonexistent repo", func(t *testing.T) {
		gitURL := "git::https://github.com/this-org-does-not-exist-12345/fake-repo"

		_, err := downloader.DownloadFromGit(ctx, gitURL)
		require.Error(t, err)
	})

	t.Run("invalid ref", func(t *testing.T) {
		gitURL := "git::https://github.com/hashicorp/terraform-aws-consul?ref=nonexistent-ref-12345"

		_, err := downloader.DownloadFromGit(ctx, gitURL)
		require.Error(t, err)
	})
}

func TestCreateTarGz(t *testing.T) {
	// Create a temp directory with some files
	tempDir := t.TempDir()

	// Create test files
	testFiles := map[string]string{
		"main.tf":      "resource \"null_resource\" \"test\" {}",
		"variables.tf": "variable \"name\" { type = string }",
		"outputs.tf":   "output \"id\" { value = null_resource.test.id }",
	}

	for path, content := range testFiles {
		fullPath := tempDir + "/" + path
		require.NoError(t, os.WriteFile(fullPath, []byte(content), 0644))
	}

	// Create subdirectory with file
	require.NoError(t, os.Mkdir(tempDir+"/subdir", 0755))
	require.NoError(t, os.WriteFile(tempDir+"/subdir/child.tf", []byte("# Child module"), 0644))

	// Create tarball
	data, err := createTarGz(tempDir)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	// Verify it's gzip compressed (magic bytes)
	assert.Equal(t, byte(0x1f), data[0])
	assert.Equal(t, byte(0x8b), data[1])
}
