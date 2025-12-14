package module

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/hcl/v2/hclsimple"
)

// ModuleDefinition represents a parsed module definition from HCL
type ModuleDefinition struct {
	Source    string   // e.g., "hashicorp/consul/aws"
	Namespace string   // e.g., "hashicorp"
	Name      string   // e.g., "consul"
	System    string   // e.g., "aws" (provider/target system)
	Versions  []string // e.g., ["0.1.0", "0.2.0"]
}

// ModuleDefinitions is a collection of module definitions
type ModuleDefinitions struct {
	Modules []*ModuleDefinition
}

// hclModuleConfig represents the HCL file structure
type hclModuleConfig struct {
	Modules []hclModule `hcl:"module,block"`
}

// hclModule represents a single module block in HCL
type hclModule struct {
	Source   string   `hcl:"source,label"`
	Versions []string `hcl:"versions"`
}

var (
	// moduleSourceRegex validates module source format (namespace/name/system)
	moduleSourceRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*/[a-zA-Z0-9][a-zA-Z0-9_-]*/[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

	// moduleVersionRegex validates semantic version format for modules
	moduleVersionRegex = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?(\+[a-zA-Z0-9.]+)?$`)
)

// ParseModuleHCL parses a module definition HCL file
func ParseModuleHCL(content []byte) (*ModuleDefinitions, error) {
	var hclConfig hclModuleConfig

	// Parse HCL
	if err := hclsimple.Decode("modules.hcl", content, nil, &hclConfig); err != nil {
		return nil, fmt.Errorf("failed to parse HCL: %w", err)
	}

	// Convert to ModuleDefinitions
	defs := &ModuleDefinitions{
		Modules: make([]*ModuleDefinition, 0, len(hclConfig.Modules)),
	}

	// Track seen modules to detect duplicates
	seen := make(map[string]bool)

	for _, m := range hclConfig.Modules {
		// Validate and parse
		def, err := parseModule(&m)
		if err != nil {
			return nil, fmt.Errorf("module %q: %w", m.Source, err)
		}

		// Check for duplicates
		if seen[def.Source] {
			return nil, fmt.Errorf("duplicate module definition: %q", def.Source)
		}
		seen[def.Source] = true

		defs.Modules = append(defs.Modules, def)
	}

	if len(defs.Modules) == 0 {
		return nil, fmt.Errorf("no module definitions found")
	}

	return defs, nil
}

// parseModule validates and converts an HCL module block
func parseModule(m *hclModule) (*ModuleDefinition, error) {
	// Validate source format
	if !moduleSourceRegex.MatchString(m.Source) {
		return nil, fmt.Errorf("invalid source format, expected 'namespace/name/system'")
	}

	// Split source into namespace, name, and system
	parts := strings.SplitN(m.Source, "/", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid source format, expected 'namespace/name/system'")
	}
	namespace := parts[0]
	name := parts[1]
	system := parts[2]

	// Validate versions
	if len(m.Versions) == 0 {
		return nil, fmt.Errorf("at least one version is required")
	}

	for _, v := range m.Versions {
		if !moduleVersionRegex.MatchString(v) {
			return nil, fmt.Errorf("invalid version format %q, expected semantic version (e.g., 1.2.3)", v)
		}
	}

	return &ModuleDefinition{
		Source:    m.Source,
		Namespace: namespace,
		Name:      name,
		System:    system,
		Versions:  m.Versions,
	}, nil
}

// CountItems returns the total number of download items (modules × versions)
func (d *ModuleDefinitions) CountItems() int {
	count := 0
	for _, m := range d.Modules {
		count += len(m.Versions)
	}
	return count
}

// GetModuleKey returns a unique key for a module (without version)
func (d *ModuleDefinition) GetModuleKey() string {
	return fmt.Sprintf("%s/%s/%s", d.Namespace, d.Name, d.System)
}

// GetFullKey returns a unique key for a specific module version
func (d *ModuleDefinition) GetFullKey(version string) string {
	return fmt.Sprintf("%s/%s/%s/%s", d.Namespace, d.Name, d.System, version)
}
