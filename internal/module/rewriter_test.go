package module

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"testing"
)

func TestNewRewriter(t *testing.T) {
	r := NewRewriter("mirror.example.com")
	if r.mirrorHostname != "mirror.example.com" {
		t.Errorf("expected mirror hostname to be mirror.example.com, got %s", r.mirrorHostname)
	}
}

func TestRewriteSource(t *testing.T) {
	r := NewRewriter("mirror.example.com")

	tests := []struct {
		name           string
		source         string
		expectedSource string
		shouldRewrite  bool
	}{
		{
			name:           "public registry module",
			source:         "hashicorp/consul/aws",
			expectedSource: "mirror.example.com/hashicorp/consul/aws",
			shouldRewrite:  true,
		},
		{
			name:           "private registry module",
			source:         "app.terraform.io/company/consul/aws",
			expectedSource: "mirror.example.com/company/consul/aws",
			shouldRewrite:  true,
		},
		{
			name:          "already using mirror",
			source:        "mirror.example.com/hashicorp/consul/aws",
			shouldRewrite: false,
		},
		{
			name:          "local path relative",
			source:        "./modules/vpc",
			shouldRewrite: false,
		},
		{
			name:          "local path parent",
			source:        "../modules/vpc",
			shouldRewrite: false,
		},
		{
			name:          "git source",
			source:        "git@github.com:hashicorp/example.git",
			shouldRewrite: false,
		},
		{
			name:          "https source",
			source:        "https://example.com/module.zip",
			shouldRewrite: false,
		},
		{
			name:          "s3 source",
			source:        "s3://bucket/module.zip",
			shouldRewrite: false,
		},
		{
			name:          "gcs source",
			source:        "gcs://bucket/module.zip",
			shouldRewrite: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newSource, shouldRewrite := r.rewriteSource(tt.source)
			if shouldRewrite != tt.shouldRewrite {
				t.Errorf("expected shouldRewrite=%v, got %v", tt.shouldRewrite, shouldRewrite)
			}
			if shouldRewrite && newSource != tt.expectedSource {
				t.Errorf("expected source %q, got %q", tt.expectedSource, newSource)
			}
		})
	}
}

func TestRewriteModuleSources(t *testing.T) {
	r := NewRewriter("mirror.example.com")

	tests := []struct {
		name            string
		content         string
		expectChanged   bool
		expectedContent string
	}{
		{
			name: "rewrite public registry module",
			content: `module "consul" {
  source  = "hashicorp/consul/aws"
  version = "0.1.0"
}`,
			expectChanged:   true,
			expectedContent: `mirror.example.com/hashicorp/consul/aws`, // Just check the rewritten source
		},
		{
			name: "keep local module unchanged",
			content: `module "vpc" {
  source = "./modules/vpc"
}`,
			expectChanged: false,
		},
		{
			name: "mixed modules - only rewrite registry",
			content: `module "consul" {
  source  = "hashicorp/consul/aws"
  version = "0.1.0"
}

module "vpc" {
  source = "./modules/vpc"
}`,
			expectChanged: true,
		},
		{
			name: "no modules",
			content: `variable "name" {
  type = string
}`,
			expectChanged: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, changed, err := r.rewriteModuleSources([]byte(tt.content), "test.tf")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if changed != tt.expectChanged {
				t.Errorf("expected changed=%v, got %v", tt.expectChanged, changed)
			}
			if tt.expectChanged && tt.expectedContent != "" {
				if !bytes.Contains(result, []byte(tt.expectedContent)) {
					t.Errorf("expected result to contain %q, got:\n%s", tt.expectedContent, string(result))
				}
			}
		})
	}
}

func TestRewriteWithEmptyMirrorHostname(t *testing.T) {
	r := NewRewriter("")

	// When mirror hostname is empty, modules should not be rewritten
	tarball := createTestTarball(t, map[string]string{
		"main.tf": `module "consul" {
  source  = "hashicorp/consul/aws"
  version = "0.1.0"
}`,
	})

	result, err := r.RewriteModule(tarball)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return original tarball unchanged
	if !bytes.Equal(result, tarball) {
		t.Error("expected original tarball to be returned when mirror hostname is empty")
	}
}

func TestRewriteModule(t *testing.T) {
	r := NewRewriter("mirror.example.com")

	// Create a test tarball with a module that needs rewriting
	files := map[string]string{
		"main.tf": `module "consul" {
  source  = "hashicorp/consul/aws"
  version = "0.1.0"
}

module "local" {
  source = "./modules/local"
}`,
		"modules/local/main.tf": `variable "name" {
  type = string
}`,
	}

	tarball := createTestTarball(t, files)
	result, err := r.RewriteModule(tarball)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Extract and verify the rewritten tarball
	extractedFiles := extractTestTarball(t, result)

	mainTf, ok := extractedFiles["main.tf"]
	if !ok {
		t.Fatal("main.tf not found in result")
	}

	if !bytes.Contains(mainTf, []byte("mirror.example.com/hashicorp/consul/aws")) {
		t.Error("expected module source to be rewritten")
	}

	// Local module should be unchanged
	if !bytes.Contains(mainTf, []byte(`"./modules/local"`)) {
		t.Error("expected local module source to be unchanged")
	}
}

// createTestTarball creates a gzipped tarball for testing
func createTestTarball(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	for name, content := range files {
		header := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(header); err != nil {
			t.Fatalf("failed to write tar header: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write tar content: %v", err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("failed to close tar writer: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("failed to close gzip writer: %v", err)
	}

	return buf.Bytes()
}

// extractTestTarball extracts a gzipped tarball for testing
func extractTestTarball(t *testing.T, data []byte) map[string][]byte {
	t.Helper()

	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	files := make(map[string][]byte)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("failed to read tar entry: %v", err)
		}

		if header.Typeflag == tar.TypeReg {
			content, err := io.ReadAll(tr)
			if err != nil {
				t.Fatalf("failed to read file content: %v", err)
			}
			files[header.Name] = content
		}
	}

	return files
}

func TestGetAttributeStringValue(t *testing.T) {
	r := NewRewriter("mirror.example.com")

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "simple string",
			content:  `source = "hashicorp/consul/aws"`,
			expected: "hashicorp/consul/aws",
		},
		{
			name:     "string with hostname",
			content:  `source = "app.terraform.io/company/module/aws"`,
			expected: "app.terraform.io/company/module/aws",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Wrap in a terraform block for valid HCL
			hcl := `module "test" {
  ` + tt.content + `
}`
			_, changed, err := r.rewriteModuleSources([]byte(hcl), "test.tf")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// Just verify it parses without error
			_ = changed
		})
	}
}
