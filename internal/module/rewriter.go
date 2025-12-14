package module

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

// Rewriter handles rewriting module sources in downloaded modules
type Rewriter struct {
	mirrorHostname string // The hostname clients will use to access the mirror
}

// NewRewriter creates a new module source rewriter
func NewRewriter(mirrorHostname string) *Rewriter {
	return &Rewriter{
		mirrorHostname: mirrorHostname,
	}
}

// RewriteModule extracts a tarball, rewrites remote module sources, and repacks
func (r *Rewriter) RewriteModule(tarball []byte) ([]byte, error) {
	if r.mirrorHostname == "" {
		// No mirror hostname configured, return original tarball
		return tarball, nil
	}

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "tf-module-rewrite-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Extract tarball
	if err := r.extractTarGz(tarball, tempDir); err != nil {
		return nil, fmt.Errorf("failed to extract tarball: %w", err)
	}

	// Rewrite .tf files
	if err := r.rewriteTerraformFiles(tempDir); err != nil {
		return nil, fmt.Errorf("failed to rewrite terraform files: %w", err)
	}

	// Repack as tarball
	repackedData, err := r.createTarGz(tempDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create tarball: %w", err)
	}

	return repackedData, nil
}

// extractTarGz extracts a gzipped tarball to the destination directory
func (r *Rewriter) extractTarGz(data []byte, destDir string) error {
	gzr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry: %w", err)
		}

		// Clean the path to prevent directory traversal
		cleanPath := filepath.Clean(header.Name)
		if strings.HasPrefix(cleanPath, "..") {
			continue // Skip paths that try to escape
		}

		target := filepath.Join(destDir, cleanPath)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", target, err)
			}
		case tar.TypeReg:
			// Create parent directories if needed
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}

			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file %s: %w", target, err)
			}

			// Copy with size limit (100MB per file max)
			if _, err := io.CopyN(f, tr, 100*1024*1024); err != nil && err != io.EOF {
				f.Close()
				return fmt.Errorf("failed to write file %s: %w", target, err)
			}
			f.Close()
		}
	}

	return nil
}

// createTarGz creates a gzipped tarball from the source directory
func (r *Rewriter) createTarGz(srcDir string) ([]byte, error) {
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("failed to create tar header: %w", err)
		}

		// Use relative path in archive
		header.Name = relPath

		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write tar header: %w", err)
		}

		if !info.IsDir() {
			f, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("failed to open file: %w", err)
			}
			defer f.Close()

			if _, err := io.Copy(tw, f); err != nil {
				return fmt.Errorf("failed to copy file content: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("failed to close tar writer: %w", err)
	}

	if err := gzw.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// rewriteTerraformFiles finds and rewrites .tf files with remote module sources
func (r *Rewriter) rewriteTerraformFiles(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Only process .tf files
		if info.IsDir() || !strings.HasSuffix(path, ".tf") {
			return nil
		}

		// Read file
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", path, err)
		}

		// Rewrite content
		rewritten, changed, err := r.rewriteModuleSources(content, path)
		if err != nil {
			// Log warning but continue - some files may have syntax issues
			return nil
		}

		// Write back if changed
		if changed {
			if err := os.WriteFile(path, rewritten, info.Mode()); err != nil {
				return fmt.Errorf("failed to write %s: %w", path, err)
			}
		}

		return nil
	})
}

// rewriteModuleSources rewrites module source attributes in HCL content
func (r *Rewriter) rewriteModuleSources(content []byte, filename string) ([]byte, bool, error) {
	// Parse HCL
	file, diags := hclwrite.ParseConfig(content, filename, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, false, fmt.Errorf("failed to parse HCL: %s", diags.Error())
	}

	changed := false
	body := file.Body()

	// Find all module blocks
	for _, block := range body.Blocks() {
		if block.Type() != "module" {
			continue
		}

		// Get source attribute
		sourceAttr := block.Body().GetAttribute("source")
		if sourceAttr == nil {
			continue
		}

		// Get the current source value
		sourceValue := r.getAttributeStringValue(sourceAttr)
		if sourceValue == "" {
			continue
		}

		// Check if this is a remote registry source that needs rewriting
		newSource, shouldRewrite := r.rewriteSource(sourceValue)
		if shouldRewrite {
			// Create new token for the rewritten source
			block.Body().SetAttributeValue("source", cty.StringVal(newSource))
			changed = true
		}
	}

	if changed {
		return file.Bytes(), true, nil
	}

	return content, false, nil
}

// getAttributeStringValue extracts the string value from an attribute
func (r *Rewriter) getAttributeStringValue(attr *hclwrite.Attribute) string {
	tokens := attr.Expr().BuildTokens(nil)

	// Look for quoted string
	for _, tok := range tokens {
		if tok.Type == hclsyntax.TokenQuotedLit {
			return string(tok.Bytes)
		}
	}

	// Try to extract from template expression
	var value strings.Builder
	for _, tok := range tokens {
		switch tok.Type {
		case hclsyntax.TokenOQuote, hclsyntax.TokenCQuote:
			continue
		case hclsyntax.TokenQuotedLit:
			value.Write(tok.Bytes)
		}
	}

	return value.String()
}

// rewriteSource determines if a source should be rewritten and returns the new source
func (r *Rewriter) rewriteSource(source string) (string, bool) {
	// Skip local paths
	if strings.HasPrefix(source, "./") || strings.HasPrefix(source, "../") {
		return "", false
	}

	// Skip other URL-style sources (git, http, s3, etc.)
	if strings.Contains(source, "://") || strings.HasPrefix(source, "git@") {
		return "", false
	}

	// Split the source by /
	parts := strings.Split(source, "/")

	// Registry module sources can be:
	// - 3 parts: namespace/name/system (public registry)
	// - 4 parts: hostname/namespace/name/system (private registry)
	if len(parts) < 3 || len(parts) > 4 {
		return "", false
	}

	var hostname, modulePath string

	if len(parts) == 3 {
		// Public registry format: namespace/name/system
		hostname = ""
		modulePath = source
	} else {
		// Private registry format: hostname/namespace/name/system
		hostname = parts[0]
		modulePath = strings.Join(parts[1:], "/")
	}

	// If it already points to our mirror, skip
	if hostname == r.mirrorHostname {
		return "", false
	}

	// Rewrite to use mirror hostname
	newSource := r.mirrorHostname + "/" + modulePath

	return newSource, true
}
