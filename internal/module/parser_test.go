package module

import (
	"testing"
)

func TestParseModuleHCL(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantErr   bool
		errMsg    string
		wantCount int
	}{
		{
			name: "valid single module",
			content: `
module "hashicorp/consul/aws" {
  versions = ["0.1.0", "0.2.0"]
}`,
			wantCount: 1,
		},
		{
			name: "valid multiple modules",
			content: `
module "hashicorp/consul/aws" {
  versions = ["0.1.0"]
}

module "hashicorp/vault/aws" {
  versions = ["1.0.0", "1.1.0"]
}`,
			wantCount: 2,
		},
		{
			name: "invalid source format - missing system",
			content: `
module "hashicorp/consul" {
  versions = ["0.1.0"]
}`,
			wantErr: true,
			errMsg:  "invalid source format",
		},
		{
			name: "duplicate module",
			content: `
module "hashicorp/consul/aws" {
  versions = ["0.1.0"]
}

module "hashicorp/consul/aws" {
  versions = ["0.2.0"]
}`,
			wantErr: true,
			errMsg:  "duplicate module definition",
		},
		{
			name: "no versions",
			content: `
module "hashicorp/consul/aws" {
  versions = []
}`,
			wantErr: true,
			errMsg:  "at least one version is required",
		},
		{
			name: "invalid version format",
			content: `
module "hashicorp/consul/aws" {
  versions = ["latest"]
}`,
			wantErr: true,
			errMsg:  "invalid version format",
		},
		{
			name:    "no modules",
			content: `# empty file`,
			wantErr: true,
			errMsg:  "no module definitions found",
		},
		{
			name: "valid prerelease version",
			content: `
module "hashicorp/consul/aws" {
  versions = ["0.1.0-beta.1"]
}`,
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defs, err := ParseModuleHCL([]byte(tt.content))

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(defs.Modules) != tt.wantCount {
				t.Errorf("expected %d modules, got %d", tt.wantCount, len(defs.Modules))
			}
		})
	}
}

func TestModuleDefinitionParsing(t *testing.T) {
	content := `
module "hashicorp/consul/aws" {
  versions = ["0.1.0", "0.2.0"]
}`

	defs, err := ParseModuleHCL([]byte(content))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(defs.Modules) != 1 {
		t.Fatalf("expected 1 module, got %d", len(defs.Modules))
	}

	m := defs.Modules[0]

	if m.Source != "hashicorp/consul/aws" {
		t.Errorf("expected source 'hashicorp/consul/aws', got %q", m.Source)
	}

	if m.Namespace != "hashicorp" {
		t.Errorf("expected namespace 'hashicorp', got %q", m.Namespace)
	}

	if m.Name != "consul" {
		t.Errorf("expected name 'consul', got %q", m.Name)
	}

	if m.System != "aws" {
		t.Errorf("expected system 'aws', got %q", m.System)
	}

	if len(m.Versions) != 2 {
		t.Errorf("expected 2 versions, got %d", len(m.Versions))
	}
}

func TestModuleDefinitionsCountItems(t *testing.T) {
	content := `
module "hashicorp/consul/aws" {
  versions = ["0.1.0", "0.2.0"]
}

module "hashicorp/vault/aws" {
  versions = ["1.0.0"]
}`

	defs, err := ParseModuleHCL([]byte(content))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 2 versions + 1 version = 3 items
	if count := defs.CountItems(); count != 3 {
		t.Errorf("expected 3 items, got %d", count)
	}
}

func TestModuleDefinitionGetKeys(t *testing.T) {
	def := &ModuleDefinition{
		Namespace: "hashicorp",
		Name:      "consul",
		System:    "aws",
		Versions:  []string{"0.1.0"},
	}

	if key := def.GetModuleKey(); key != "hashicorp/consul/aws" {
		t.Errorf("expected key 'hashicorp/consul/aws', got %q", key)
	}

	if key := def.GetFullKey("0.1.0"); key != "hashicorp/consul/aws/0.1.0" {
		t.Errorf("expected key 'hashicorp/consul/aws/0.1.0', got %q", key)
	}
}

func TestModuleSourceValidation(t *testing.T) {
	validSources := []string{
		"hashicorp/consul/aws",
		"company/my-module/azure",
		"org123/module_name/gcp",
	}

	for _, source := range validSources {
		if !moduleSourceRegex.MatchString(source) {
			t.Errorf("expected source %q to be valid", source)
		}
	}

	invalidSources := []string{
		"hashicorp/consul",      // missing system
		"consul/aws",            // missing namespace
		"-invalid/consul/aws",   // starts with dash
		"hashicorp//aws",        // empty name
		"hashicorp/consul//aws", // extra slash
	}

	for _, source := range invalidSources {
		if moduleSourceRegex.MatchString(source) {
			t.Errorf("expected source %q to be invalid", source)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
