package provider

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/hcl/v2/hclsimple"
)

// ProviderDefinition represents a parsed provider definition from HCL
type ProviderDefinition struct {
	Source    string   // e.g., "hashicorp/aws"
	Namespace string   // e.g., "hashicorp"
	Type      string   // e.g., "aws"
	Versions  []string // e.g., ["5.0.0", "5.1.0"]
	Platforms []string // e.g., ["linux_amd64", "darwin_arm64"]
}

// ProviderDefinitions is a collection of provider definitions
type ProviderDefinitions struct {
	Providers []*ProviderDefinition
}

// hclProviderConfig represents the HCL file structure
type hclProviderConfig struct {
	Providers []hclProvider `hcl:"provider,block"`
}

// hclProvider represents a single provider block in HCL
type hclProvider struct {
	Source    string   `hcl:"source,label"`
	Versions  []string `hcl:"versions"`
	Platforms []string `hcl:"platforms"`
}

var (
	// providerSourceRegex validates provider source format (namespace/type)
	providerSourceRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+/[a-zA-Z0-9_-]+$`)

	// semanticVersionRegex validates semantic version format
	semanticVersionRegex = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?(\+[a-zA-Z0-9.]+)?$`)

	// platformRegex validates platform format (os_arch)
	platformRegex = regexp.MustCompile(`^(linux|darwin|windows|freebsd)_(amd64|arm64|386|arm)$`)
)

// ParseHCL parses a provider definition HCL file
func ParseHCL(content []byte) (*ProviderDefinitions, error) {
	var hclConfig hclProviderConfig

	// Parse HCL
	if err := hclsimple.Decode("providers.hcl", content, nil, &hclConfig); err != nil {
		return nil, fmt.Errorf("failed to parse HCL: %w", err)
	}

	// Convert to ProviderDefinitions
	defs := &ProviderDefinitions{
		Providers: make([]*ProviderDefinition, 0, len(hclConfig.Providers)),
	}

	// Track seen providers to detect duplicates
	seen := make(map[string]bool)

	for _, p := range hclConfig.Providers {
		// Validate and parse
		def, err := parseProvider(&p)
		if err != nil {
			return nil, fmt.Errorf("provider %q: %w", p.Source, err)
		}

		// Check for duplicates
		if seen[def.Source] {
			return nil, fmt.Errorf("duplicate provider definition: %q", def.Source)
		}
		seen[def.Source] = true

		defs.Providers = append(defs.Providers, def)
	}

	if len(defs.Providers) == 0 {
		return nil, fmt.Errorf("no provider definitions found")
	}

	return defs, nil
}

// parseProvider validates and converts an HCL provider block
func parseProvider(p *hclProvider) (*ProviderDefinition, error) {
	// Validate source format
	if !providerSourceRegex.MatchString(p.Source) {
		return nil, fmt.Errorf("invalid source format, expected 'namespace/type'")
	}

	// Split source into namespace and type
	parts := strings.SplitN(p.Source, "/", 2)
	namespace := parts[0]
	providerType := parts[1]

	// Validate versions
	if len(p.Versions) == 0 {
		return nil, fmt.Errorf("at least one version is required")
	}

	for _, v := range p.Versions {
		if !semanticVersionRegex.MatchString(v) {
			return nil, fmt.Errorf("invalid version format %q, expected semantic version (e.g., 1.2.3)", v)
		}
	}

	// Validate platforms
	if len(p.Platforms) == 0 {
		return nil, fmt.Errorf("at least one platform is required")
	}

	for _, platform := range p.Platforms {
		if !platformRegex.MatchString(platform) {
			return nil, fmt.Errorf("invalid platform format %q, expected 'os_arch' (e.g., linux_amd64)", platform)
		}
	}

	return &ProviderDefinition{
		Source:    p.Source,
		Namespace: namespace,
		Type:      providerType,
		Versions:  p.Versions,
		Platforms: p.Platforms,
	}, nil
}

// CountItems returns the total number of download items
// (providers × versions × platforms)
func (d *ProviderDefinitions) CountItems() int {
	count := 0
	for _, p := range d.Providers {
		count += len(p.Versions) * len(p.Platforms)
	}
	return count
}
