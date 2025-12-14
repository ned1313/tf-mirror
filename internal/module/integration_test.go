//go:build integration
// +build integration

package module

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ned1313/terraform-mirror/internal/database"
	"github.com/ned1313/terraform-mirror/internal/storage"
)

// Integration tests for the module service with real storage
// Run with: go test -tags=integration ./internal/module/...

func setupModuleIntegrationTest(t *testing.T) (*database.DB, storage.Storage, string, func()) {
	// Create temporary directory for storage
	tempDir, err := os.MkdirTemp("", "module-integration-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create local storage
	store, err := storage.NewLocalStorage(storage.LocalConfig{
		BasePath: tempDir,
	})
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create local storage: %v", err)
	}

	// Create in-memory database
	db, err := database.New(":memory:")
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create database: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.RemoveAll(tempDir)
	}

	return db, store, tempDir, cleanup
}

// MockModuleRegistryClient is a mock implementation of the module registry client
type MockModuleRegistryClient struct {
	versions    map[string][]string // key: namespace/name/system -> versions
	downloadURL map[string]string   // key: namespace/name/system/version -> url
	moduleData  map[string][]byte   // key: url -> data
}

func newMockModuleRegistryClient() *MockModuleRegistryClient {
	return &MockModuleRegistryClient{
		versions:    make(map[string][]string),
		downloadURL: make(map[string]string),
		moduleData:  make(map[string][]byte),
	}
}

func (m *MockModuleRegistryClient) AddModule(namespace, name, system, version string, data []byte) {
	key := namespace + "/" + name + "/" + system
	versionKey := key + "/" + version
	url := "https://mock-registry.example.com/modules/" + versionKey + ".tar.gz"

	if m.versions[key] == nil {
		m.versions[key] = []string{}
	}
	m.versions[key] = append(m.versions[key], version)
	m.downloadURL[versionKey] = url
	m.moduleData[url] = data
}

func (m *MockModuleRegistryClient) GetAvailableVersions(ctx context.Context, namespace, name, system string) ([]string, error) {
	key := namespace + "/" + name + "/" + system
	versions, ok := m.versions[key]
	if !ok {
		return nil, nil
	}
	return versions, nil
}

func (m *MockModuleRegistryClient) GetDownloadURL(ctx context.Context, namespace, name, system, version string) (string, error) {
	key := namespace + "/" + name + "/" + system + "/" + version
	url, ok := m.downloadURL[key]
	if !ok {
		return "", nil
	}
	return url, nil
}

func (m *MockModuleRegistryClient) DownloadModule(ctx context.Context, downloadURL string) ([]byte, error) {
	data, ok := m.moduleData[downloadURL]
	if !ok {
		return nil, nil
	}
	return data, nil
}

func (m *MockModuleRegistryClient) DownloadModuleComplete(ctx context.Context, namespace, name, system, version string) *DownloadResult {
	key := namespace + "/" + name + "/" + system + "/" + version
	url, ok := m.downloadURL[key]
	if !ok {
		return &DownloadResult{
			Error: nil,
		}
	}

	data, ok := m.moduleData[url]
	if !ok {
		return &DownloadResult{
			Error: nil,
		}
	}

	return &DownloadResult{
		Data: data,
		Info: &ModuleDownloadInfo{
			Namespace:   namespace,
			Name:        name,
			System:      system,
			Version:     version,
			DownloadURL: url,
			Filename:    namespace + "-" + name + "-" + system + "-" + version + ".tar.gz",
		},
		Error: nil,
	}
}

// createTestModuleTarball creates a valid tar.gz file with module content
func createTestModuleTarball(t *testing.T, mainTF string) []byte {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	// Add main.tf
	content := []byte(mainTF)
	header := &tar.Header{
		Name:    "main.tf",
		Mode:    0644,
		Size:    int64(len(content)),
		ModTime: time.Now(),
	}

	if err := tw.WriteHeader(header); err != nil {
		t.Fatalf("Failed to write tar header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("Failed to write tar content: %v", err)
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("Failed to close tar writer: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("Failed to close gzip writer: %v", err)
	}

	return buf.Bytes()
}

// TestIntegration_ModuleServiceLoadSingle tests loading a single module
func TestIntegration_ModuleServiceLoadSingle(t *testing.T) {
	db, store, tempDir, cleanup := setupModuleIntegrationTest(t)
	defer cleanup()

	// Create mock registry client
	mockRegistry := newMockModuleRegistryClient()

	// Create a test module tarball
	moduleContent := `
variable "vpc_cidr" {
  description = "The CIDR block for the VPC"
  type        = string
  default     = "10.0.0.0/16"
}

resource "aws_vpc" "main" {
  cidr_block = var.vpc_cidr
}
`
	tarball := createTestModuleTarball(t, moduleContent)
	mockRegistry.AddModule("terraform-aws-modules", "vpc", "aws", "5.0.0", tarball)

	// Create service with mock registry
	service := NewService(store, db, "mirror.example.com")
	service.SetRegistry(mockRegistry)

	// Load the module
	ctx := context.Background()
	result := service.LoadSingleModule(ctx, "terraform-aws-modules", "vpc", "aws", "5.0.0")

	if !result.Success {
		t.Fatalf("Failed to load module: %v", result.Error)
	}
	if result.Skipped {
		t.Error("Module was unexpectedly skipped")
	}

	// Verify module was stored in database
	moduleRepo := database.NewModuleRepository(db)
	storedModule, err := moduleRepo.GetByIdentity(ctx, "terraform-aws-modules", "vpc", "aws", "5.0.0")
	if err != nil {
		t.Fatalf("Failed to get module from database: %v", err)
	}
	if storedModule == nil {
		t.Fatal("Module not found in database")
	}

	// Verify S3 key format
	expectedKeyPrefix := "modules/terraform-aws-modules/vpc/aws/5.0.0/"
	if len(storedModule.S3Key) < len(expectedKeyPrefix) || storedModule.S3Key[:len(expectedKeyPrefix)] != expectedKeyPrefix {
		t.Errorf("Expected S3 key to start with '%s', got '%s'", expectedKeyPrefix, storedModule.S3Key)
	}

	// Verify file was uploaded to storage
	exists, err := store.Exists(ctx, storedModule.S3Key)
	if err != nil {
		t.Fatalf("Failed to check storage: %v", err)
	}
	if !exists {
		t.Errorf("Module file not found in storage at key: %s", storedModule.S3Key)
	}

	// Verify file content exists
	expectedPath := filepath.Join(tempDir, storedModule.S3Key)
	data, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("Failed to read module file: %v", err)
	}
	if len(data) == 0 {
		t.Error("Module file is empty")
	}

	t.Logf("Successfully loaded module: %s/%s/%s %s",
		storedModule.Namespace, storedModule.Name, storedModule.System, storedModule.Version)
	t.Logf("Storage path: %s", storedModule.S3Key)
	t.Logf("File size: %d bytes", len(data))
}

// TestIntegration_ModuleServiceSkipsExisting tests that existing modules are skipped
func TestIntegration_ModuleServiceSkipsExisting(t *testing.T) {
	db, store, _, cleanup := setupModuleIntegrationTest(t)
	defer cleanup()

	// Pre-create a module in the database
	moduleRepo := database.NewModuleRepository(db)
	existingModule := &database.Module{
		Namespace: "terraform-aws-modules",
		Name:      "vpc",
		System:    "aws",
		Version:   "5.0.0",
		S3Key:     "modules/terraform-aws-modules/vpc/aws/5.0.0/terraform-aws-modules-vpc-aws-5.0.0.tar.gz",
		Filename:  "terraform-aws-modules-vpc-aws-5.0.0.tar.gz",
		SizeBytes: 1234,
	}
	err := moduleRepo.Create(context.Background(), existingModule)
	if err != nil {
		t.Fatalf("Failed to create existing module: %v", err)
	}

	// Create mock registry client (should NOT be called)
	mockRegistry := newMockModuleRegistryClient()

	// Create service
	service := NewService(store, db, "mirror.example.com")
	service.SetRegistry(mockRegistry)

	// Try to load the same module
	ctx := context.Background()
	result := service.LoadSingleModule(ctx, "terraform-aws-modules", "vpc", "aws", "5.0.0")

	if !result.Success {
		t.Errorf("Expected success, got error: %v", result.Error)
	}
	if !result.Skipped {
		t.Error("Expected module to be skipped since it already exists")
	}

	t.Log("Successfully skipped existing module")
}

// TestIntegration_ModuleServiceLoadFromDefinitions tests loading multiple modules
func TestIntegration_ModuleServiceLoadFromDefinitions(t *testing.T) {
	db, store, _, cleanup := setupModuleIntegrationTest(t)
	defer cleanup()

	// Create mock registry client with multiple modules
	mockRegistry := newMockModuleRegistryClient()

	// Add test modules
	vpcModule := createTestModuleTarball(t, `resource "aws_vpc" "main" {}`)
	ec2Module := createTestModuleTarball(t, `resource "aws_instance" "main" {}`)

	mockRegistry.AddModule("terraform-aws-modules", "vpc", "aws", "5.0.0", vpcModule)
	mockRegistry.AddModule("terraform-aws-modules", "vpc", "aws", "5.1.0", vpcModule)
	mockRegistry.AddModule("terraform-aws-modules", "ec2-instance", "aws", "5.0.0", ec2Module)

	// Create service
	service := NewService(store, db, "mirror.example.com")
	service.SetRegistry(mockRegistry)

	// Create module definitions
	defs := &ModuleDefinitions{
		Modules: []*ModuleDefinition{
			{
				Namespace: "terraform-aws-modules",
				Name:      "vpc",
				System:    "aws",
				Versions:  []string{"5.0.0", "5.1.0"},
			},
			{
				Namespace: "terraform-aws-modules",
				Name:      "ec2-instance",
				System:    "aws",
				Versions:  []string{"5.0.0"},
			},
		},
	}

	// Load all modules
	ctx := context.Background()
	results, err := service.LoadFromDefinitions(ctx, defs)
	if err != nil {
		t.Fatalf("Failed to load modules: %v", err)
	}

	// Verify results
	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	successCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		} else if result.Error != nil {
			t.Errorf("Module %s/%s/%s %s failed: %v",
				result.Namespace, result.Name, result.System, result.Version, result.Error)
		}
	}

	if successCount != 3 {
		t.Errorf("Expected 3 successful loads, got %d", successCount)
	}

	// Verify all modules in database
	moduleRepo := database.NewModuleRepository(db)
	modules, err := moduleRepo.List(context.Background(), 100, 0)
	if err != nil {
		t.Fatalf("Failed to list modules: %v", err)
	}

	if len(modules) != 3 {
		t.Errorf("Expected 3 modules in database, got %d", len(modules))
	}

	t.Logf("Successfully loaded %d modules", len(modules))
}

// TestIntegration_ModuleServiceWithSourceRewriting tests that nested module sources are rewritten
func TestIntegration_ModuleServiceWithSourceRewriting(t *testing.T) {
	db, store, tempDir, cleanup := setupModuleIntegrationTest(t)
	defer cleanup()

	// Create a module that references another registry module
	moduleContent := `
module "nested_vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "5.0.0"
}

resource "aws_instance" "main" {
  vpc_id = module.nested_vpc.vpc_id
}
`
	tarball := createTestModuleTarball(t, moduleContent)

	// Create mock registry client
	mockRegistry := newMockModuleRegistryClient()
	mockRegistry.AddModule("myorg", "wrapper", "aws", "1.0.0", tarball)

	// Create service with mirror hostname
	mirrorHostname := "mirror.example.com"
	service := NewService(store, db, mirrorHostname)
	service.SetRegistry(mockRegistry)

	// Load the module
	ctx := context.Background()
	result := service.LoadSingleModule(ctx, "myorg", "wrapper", "aws", "1.0.0")

	if !result.Success {
		t.Fatalf("Failed to load module: %v", result.Error)
	}

	// Get the stored module
	moduleRepo := database.NewModuleRepository(db)
	storedModule, err := moduleRepo.GetByIdentity(ctx, "myorg", "wrapper", "aws", "1.0.0")
	if err != nil {
		t.Fatalf("Failed to get module: %v", err)
	}

	// Read the stored tarball and verify the source was rewritten
	filePath := filepath.Join(tempDir, storedModule.S3Key)
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read stored module: %v", err)
	}

	// Extract and check the main.tf content
	mainTF := extractMainTFFromTarball(t, data)

	// The source should now point to the mirror
	expectedSource := mirrorHostname + "/terraform-aws-modules/vpc/aws"
	if !bytes.Contains([]byte(mainTF), []byte(expectedSource)) {
		t.Errorf("Expected source to be rewritten to contain '%s', got:\n%s", expectedSource, mainTF)
	}

	// The original public registry source should not be present
	originalSource := `source  = "terraform-aws-modules/vpc/aws"`
	if bytes.Contains([]byte(mainTF), []byte(originalSource)) {
		t.Errorf("Original source should have been rewritten, but found: %s", originalSource)
	}

	t.Logf("Successfully verified source rewriting to: %s", expectedSource)
}

// extractMainTFFromTarball extracts the main.tf content from a tarball
func extractMainTFFromTarball(t *testing.T, data []byte) string {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Failed to create gzip reader: %v", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err != nil {
			break
		}

		if header.Name == "main.tf" {
			content, err := bytes.NewBuffer(nil), error(nil)
			buf := make([]byte, 1024)
			for {
				n, readErr := tr.Read(buf)
				if n > 0 {
					content.Write(buf[:n])
				}
				if readErr != nil {
					break
				}
			}
			if err != nil {
				t.Fatalf("Failed to read main.tf: %v", err)
			}
			return content.String()
		}
	}

	t.Fatal("main.tf not found in tarball")
	return ""
}

// TestIntegration_ModuleParser tests parsing module definition files
func TestIntegration_ModuleParser(t *testing.T) {
	// Create a temporary HCL file
	tmpDir := t.TempDir()
	hclPath := filepath.Join(tmpDir, "modules.hcl")

	hclContent := `
module "terraform-aws-modules/vpc/aws" {
  versions = ["5.0.0", "5.1.0", "5.2.0"]
}

module "terraform-aws-modules/ec2-instance/aws" {
  versions = ["5.0.0"]
}

module "terraform-google-modules/network/google" {
  versions = ["7.0.0", "7.1.0"]
}
`
	err := os.WriteFile(hclPath, []byte(hclContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write HCL file: %v", err)
	}

	// Read and parse
	data, err := os.ReadFile(hclPath)
	if err != nil {
		t.Fatalf("Failed to read HCL file: %v", err)
	}

	defs, err := ParseModuleHCL(data)
	if err != nil {
		t.Fatalf("Failed to parse HCL: %v", err)
	}

	// Verify parsed content
	if len(defs.Modules) != 3 {
		t.Errorf("Expected 3 module definitions, got %d", len(defs.Modules))
	}

	totalItems := defs.CountItems()
	if totalItems != 6 {
		t.Errorf("Expected 6 total items (versions), got %d", totalItems)
	}

	// Verify first module
	vpcDef := defs.Modules[0]
	if vpcDef.Namespace != "terraform-aws-modules" {
		t.Errorf("Expected namespace 'terraform-aws-modules', got '%s'", vpcDef.Namespace)
	}
	if vpcDef.Name != "vpc" {
		t.Errorf("Expected name 'vpc', got '%s'", vpcDef.Name)
	}
	if vpcDef.System != "aws" {
		t.Errorf("Expected system 'aws', got '%s'", vpcDef.System)
	}
	if len(vpcDef.Versions) != 3 {
		t.Errorf("Expected 3 versions, got %d", len(vpcDef.Versions))
	}

	t.Logf("Successfully parsed %d modules with %d total versions", len(defs.Modules), totalItems)
}

// TestIntegration_ModuleRewriterWithLocalModules tests that local modules are not rewritten
func TestIntegration_ModuleRewriterWithLocalModules(t *testing.T) {
	db, store, tempDir, cleanup := setupModuleIntegrationTest(t)
	defer cleanup()

	// Create a module that references a local module (should NOT be rewritten)
	moduleContent := `
module "local_module" {
  source = "./modules/submodule"
}

module "parent_module" {
  source = "../shared"
}

# This one should be rewritten
module "remote_module" {
  source  = "hashicorp/consul/aws"
  version = "1.0.0"
}
`
	tarball := createTestModuleTarball(t, moduleContent)

	// Create mock registry client
	mockRegistry := newMockModuleRegistryClient()
	mockRegistry.AddModule("myorg", "mixed", "aws", "1.0.0", tarball)

	// Create service with mirror hostname
	mirrorHostname := "mirror.example.com"
	service := NewService(store, db, mirrorHostname)
	service.SetRegistry(mockRegistry)

	// Load the module
	ctx := context.Background()
	result := service.LoadSingleModule(ctx, "myorg", "mixed", "aws", "1.0.0")

	if !result.Success {
		t.Fatalf("Failed to load module: %v", result.Error)
	}

	// Get the stored module
	moduleRepo := database.NewModuleRepository(db)
	storedModule, err := moduleRepo.GetByIdentity(ctx, "myorg", "mixed", "aws", "1.0.0")
	if err != nil {
		t.Fatalf("Failed to get module: %v", err)
	}

	// Read the stored tarball
	filePath := filepath.Join(tempDir, storedModule.S3Key)
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read stored module: %v", err)
	}

	// Extract and check the main.tf content
	mainTF := extractMainTFFromTarball(t, data)

	// Local modules should NOT be rewritten
	if !bytes.Contains([]byte(mainTF), []byte(`"./modules/submodule"`)) {
		t.Error("Local module source './modules/submodule' should not have been rewritten")
	}
	if !bytes.Contains([]byte(mainTF), []byte(`"../shared"`)) {
		t.Error("Local module source '../shared' should not have been rewritten")
	}

	// Remote module SHOULD be rewritten
	if bytes.Contains([]byte(mainTF), []byte(`"hashicorp/consul/aws"`)) {
		t.Error("Remote module source 'hashicorp/consul/aws' should have been rewritten")
	}

	expectedRewrittenSource := mirrorHostname + "/hashicorp/consul/aws"
	if !bytes.Contains([]byte(mainTF), []byte(expectedRewrittenSource)) {
		t.Errorf("Expected rewritten source '%s' not found in:\n%s", expectedRewrittenSource, mainTF)
	}

	t.Log("Successfully verified local modules preserved and remote modules rewritten")
}
