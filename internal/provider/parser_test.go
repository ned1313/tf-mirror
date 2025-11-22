package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseHCL_Success(t *testing.T) {
	hcl := []byte(`
provider "hashicorp/aws" {
  versions = ["5.0.0", "5.1.0"]
  platforms = ["linux_amd64", "darwin_amd64"]
}

provider "hashicorp/azurerm" {
  versions = ["3.0.0"]
  platforms = ["linux_amd64", "linux_arm64", "windows_amd64"]
}
`)

	defs, err := ParseHCL(hcl)
	require.NoError(t, err)
	require.NotNil(t, defs)

	assert.Len(t, defs.Providers, 2)

	// Check first provider
	p1 := defs.Providers[0]
	assert.Equal(t, "hashicorp/aws", p1.Source)
	assert.Equal(t, "hashicorp", p1.Namespace)
	assert.Equal(t, "aws", p1.Type)
	assert.Equal(t, []string{"5.0.0", "5.1.0"}, p1.Versions)
	assert.Equal(t, []string{"linux_amd64", "darwin_amd64"}, p1.Platforms)

	// Check second provider
	p2 := defs.Providers[1]
	assert.Equal(t, "hashicorp/azurerm", p2.Source)
	assert.Equal(t, "hashicorp", p2.Namespace)
	assert.Equal(t, "azurerm", p2.Type)
	assert.Equal(t, []string{"3.0.0"}, p2.Versions)
	assert.Equal(t, []string{"linux_amd64", "linux_arm64", "windows_amd64"}, p2.Platforms)
}

func TestParseHCL_CountItems(t *testing.T) {
	hcl := []byte(`
provider "hashicorp/aws" {
  versions = ["5.0.0", "5.1.0"]  # 2 versions
  platforms = ["linux_amd64", "darwin_amd64"]  # 2 platforms
}

provider "hashicorp/random" {
  versions = ["3.5.0"]  # 1 version
  platforms = ["linux_amd64", "linux_arm64", "darwin_amd64"]  # 3 platforms
}
`)

	defs, err := ParseHCL(hcl)
	require.NoError(t, err)

	// (2 versions × 2 platforms) + (1 version × 3 platforms) = 4 + 3 = 7
	assert.Equal(t, 7, defs.CountItems())
}

func TestParseHCL_InvalidSyntax(t *testing.T) {
	hcl := []byte(`
provider "hashicorp/aws" {
  versions = ["5.0.0"
  # Missing closing bracket
}
`)

	_, err := ParseHCL(hcl)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse HCL")
}

func TestParseHCL_InvalidSource(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"missing namespace", `provider "aws" { versions = ["1.0.0"] platforms = ["linux_amd64"] }`},
		{"too many parts", `provider "hash/corp/aws" { versions = ["1.0.0"] platforms = ["linux_amd64"] }`},
		{"empty namespace", `provider "/aws" { versions = ["1.0.0"] platforms = ["linux_amd64"] }`},
		{"empty type", `provider "hashicorp/" { versions = ["1.0.0"] platforms = ["linux_amd64"] }`},
		{"invalid characters", `provider "hash@corp/aws!" { versions = ["1.0.0"] platforms = ["linux_amd64"] }`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseHCL([]byte(tt.source))
			assert.Error(t, err)
			// HCL parse error or validation error both acceptable
		})
	}
}

func TestParseHCL_MissingVersions(t *testing.T) {
	hcl := []byte(`
provider "hashicorp/aws" {
  platforms = ["linux_amd64"]
}
`)

	_, err := ParseHCL(hcl)
	assert.Error(t, err)
	// HCL will report missing required argument
}

func TestParseHCL_EmptyVersions(t *testing.T) {
	hcl := []byte(`
provider "hashicorp/aws" {
  versions = []
  platforms = ["linux_amd64"]
}
`)

	_, err := ParseHCL(hcl)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one version is required")
}

func TestParseHCL_InvalidVersionFormat(t *testing.T) {
	tests := []struct {
		name    string
		version string
	}{
		{"missing patch", "5.0"},
		{"missing minor and patch", "5"},
		{"not a number", "abc"},
		{"invalid characters", "5.0.0@beta"},
		{"leading v", "v5.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hcl := []byte(`
provider "hashicorp/aws" {
  versions = ["` + tt.version + `"]
  platforms = ["linux_amd64"]
}
`)
			_, err := ParseHCL(hcl)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid version format")
		})
	}
}

func TestParseHCL_ValidVersionFormats(t *testing.T) {
	tests := []struct {
		name    string
		version string
	}{
		{"simple version", "5.0.0"},
		{"pre-release", "5.0.0-beta.1"},
		{"build metadata", "5.0.0+build.123"},
		{"both", "5.0.0-beta.1+build.123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hcl := []byte(`
provider "hashicorp/aws" {
  versions = ["` + tt.version + `"]
  platforms = ["linux_amd64"]
}
`)
			defs, err := ParseHCL(hcl)
			require.NoError(t, err)
			assert.Equal(t, tt.version, defs.Providers[0].Versions[0])
		})
	}
}

func TestParseHCL_MissingPlatforms(t *testing.T) {
	hcl := []byte(`
provider "hashicorp/aws" {
  versions = ["5.0.0"]
}
`)

	_, err := ParseHCL(hcl)
	assert.Error(t, err)
	// HCL will report missing required argument
}

func TestParseHCL_EmptyPlatforms(t *testing.T) {
	hcl := []byte(`
provider "hashicorp/aws" {
  versions = ["5.0.0"]
  platforms = []
}
`)

	_, err := ParseHCL(hcl)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one platform is required")
}

func TestParseHCL_InvalidPlatformFormat(t *testing.T) {
	tests := []struct {
		name     string
		platform string
	}{
		{"missing arch", "linux"},
		{"missing os", "amd64"},
		{"wrong separator", "linux-amd64"},
		{"invalid os", "macos_amd64"},
		{"invalid arch", "linux_x86"},
		{"too many parts", "linux_amd64_extra"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hcl := []byte(`
provider "hashicorp/aws" {
  versions = ["5.0.0"]
  platforms = ["` + tt.platform + `"]
}
`)
			_, err := ParseHCL(hcl)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid platform format")
		})
	}
}

func TestParseHCL_ValidPlatforms(t *testing.T) {
	platforms := []string{
		"linux_amd64",
		"linux_arm64",
		"linux_386",
		"linux_arm",
		"darwin_amd64",
		"darwin_arm64",
		"windows_amd64",
		"windows_386",
		"freebsd_amd64",
	}

	for _, platform := range platforms {
		t.Run(platform, func(t *testing.T) {
			hcl := []byte(`
provider "hashicorp/aws" {
  versions = ["5.0.0"]
  platforms = ["` + platform + `"]
}
`)
			defs, err := ParseHCL(hcl)
			require.NoError(t, err)
			assert.Contains(t, defs.Providers[0].Platforms, platform)
		})
	}
}

func TestParseHCL_DuplicateProviders(t *testing.T) {
	hcl := []byte(`
provider "hashicorp/aws" {
  versions = ["5.0.0"]
  platforms = ["linux_amd64"]
}

provider "hashicorp/aws" {
  versions = ["5.1.0"]
  platforms = ["darwin_amd64"]
}
`)

	_, err := ParseHCL(hcl)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate provider definition")
}

func TestParseHCL_NoProviders(t *testing.T) {
	hcl := []byte(`
# Just a comment, no providers
`)

	_, err := ParseHCL(hcl)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no provider definitions found")
}

func TestParseHCL_ComplexExample(t *testing.T) {
	hcl := []byte(`
# Terraform Provider Definitions
# This file specifies which providers to pre-load

provider "hashicorp/aws" {
  versions = [
    "5.0.0",
    "5.1.0",
    "5.2.0"
  ]
  platforms = [
    "linux_amd64",
    "linux_arm64",
    "darwin_amd64",
    "darwin_arm64",
    "windows_amd64"
  ]
}

provider "hashicorp/azurerm" {
  versions = ["3.0.0", "3.1.0", "3.2.0"]
  platforms = ["linux_amd64", "windows_amd64"]
}

provider "hashicorp/google" {
  versions = ["4.50.0"]
  platforms = ["linux_amd64"]
}

provider "terraform-aws-modules/vpc" {
  versions = ["5.0.0"]
  platforms = ["linux_amd64", "darwin_amd64"]
}
`)

	defs, err := ParseHCL(hcl)
	require.NoError(t, err)
	require.NotNil(t, defs)

	assert.Len(t, defs.Providers, 4)

	// Verify item count
	// aws: 3 versions × 5 platforms = 15
	// azurerm: 3 versions × 2 platforms = 6
	// google: 1 version × 1 platform = 1
	// vpc: 1 version × 2 platforms = 2
	// Total: 15 + 6 + 1 + 2 = 24
	assert.Equal(t, 24, defs.CountItems())

	// Verify different namespace works
	vpc := defs.Providers[3]
	assert.Equal(t, "terraform-aws-modules", vpc.Namespace)
	assert.Equal(t, "vpc", vpc.Type)
}
