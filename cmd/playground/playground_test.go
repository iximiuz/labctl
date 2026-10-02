package playground

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/iximiuz/labctl/api"
)

func TestReadManifestFile_NewFormat(t *testing.T) {
	// Create a temporary manifest file with the new kernel format (struct)
	content := `kind: playground
name: test-playground
title: Test Playground
description: A test playground
playground:
  machines:
    - name: node1
      kernel:
        source: ubuntu-22.04
        snapshot:
          id: snap-123
          leaseId: lease-456
      users:
        - name: root
          default: true
  networks:
    - name: net1
      subnet: 10.0.0.0/24
`

	tmpFile := createTempManifest(t, content)
	defer os.Remove(tmpFile)

	manifest, err := readManifestFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read manifest: %v", err)
	}

	// Verify basic fields
	if manifest.Kind != "playground" {
		t.Errorf("Expected kind 'playground', got '%s'", manifest.Kind)
	}
	if manifest.Name != "test-playground" {
		t.Errorf("Expected name 'test-playground', got '%s'", manifest.Name)
	}

	// Verify machine kernel
	if len(manifest.Playground.Machines) != 1 {
		t.Fatalf("Expected 1 machine, got %d", len(manifest.Playground.Machines))
	}
	machine := manifest.Playground.Machines[0]
	if machine.Name != "node1" {
		t.Errorf("Expected machine name 'node1', got '%s'", machine.Name)
	}
	if machine.Kernel == nil {
		t.Fatal("Expected kernel to be non-nil")
	}
	if machine.Kernel.Source != "ubuntu-22.04" {
		t.Errorf("Expected kernel source 'ubuntu-22.04', got '%s'", machine.Kernel.Source)
	}
	if machine.Kernel.Snapshot == nil {
		t.Fatal("Expected kernel snapshot to be non-nil")
	}
	if machine.Kernel.Snapshot.ID != "snap-123" {
		t.Errorf("Expected snapshot ID 'snap-123', got '%s'", machine.Kernel.Snapshot.ID)
	}
}

func TestReadManifestFile_LegacyFormat(t *testing.T) {
	// Create a temporary manifest file with the old kernel format (string)
	content := `kind: playground
name: legacy-playground
title: Legacy Test Playground
description: A test playground with legacy kernel format
playground:
  machines:
    - name: node1
      kernel: ubuntu-22.04
      users:
        - name: root
          default: true
  networks:
    - name: net1
      subnet: 10.0.0.0/24
`

	tmpFile := createTempManifest(t, content)
	defer os.Remove(tmpFile)

	manifest, err := readManifestFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read manifest: %v", err)
	}

	// Verify basic fields
	if manifest.Kind != "playground" {
		t.Errorf("Expected kind 'playground', got '%s'", manifest.Kind)
	}
	if manifest.Name != "legacy-playground" {
		t.Errorf("Expected name 'legacy-playground', got '%s'", manifest.Name)
	}

	// Verify machine kernel - should be converted to new format
	if len(manifest.Playground.Machines) != 1 {
		t.Fatalf("Expected 1 machine, got %d", len(manifest.Playground.Machines))
	}
	machine := manifest.Playground.Machines[0]
	if machine.Name != "node1" {
		t.Errorf("Expected machine name 'node1', got '%s'", machine.Name)
	}
	if machine.Kernel == nil {
		t.Fatal("Expected kernel to be non-nil (converted from legacy format)")
	}
	if machine.Kernel.Source != "ubuntu-22.04" {
		t.Errorf("Expected kernel source 'ubuntu-22.04' (converted from legacy string), got '%s'", machine.Kernel.Source)
	}
	if machine.Kernel.Snapshot != nil {
		t.Error("Expected kernel snapshot to be nil for legacy format")
	}
}

func TestReadManifestFile_LegacyFormatEmptyKernel(t *testing.T) {
	// Test that an empty kernel string in legacy format doesn't create a kernel object
	content := `kind: playground
name: no-kernel-playground
playground:
  machines:
    - name: node1
      users:
        - name: root
`

	tmpFile := createTempManifest(t, content)
	defer os.Remove(tmpFile)

	manifest, err := readManifestFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read manifest: %v", err)
	}

	if len(manifest.Playground.Machines) != 1 {
		t.Fatalf("Expected 1 machine, got %d", len(manifest.Playground.Machines))
	}
	machine := manifest.Playground.Machines[0]
	if machine.Kernel != nil {
		t.Errorf("Expected kernel to be nil when not specified, got %+v", machine.Kernel)
	}
}

func TestReadManifestFile_MultipleMachinesMixedFormats(t *testing.T) {
	// Test a manifest with multiple machines using different kernel formats
	content := `kind: playground
name: mixed-playground
playground:
  machines:
    - name: legacy-node
      kernel: debian-11
    - name: new-node
      kernel:
        source: ubuntu-22.04
    - name: no-kernel-node
      users:
        - name: user1
`

	tmpFile := createTempManifest(t, content)
	defer os.Remove(tmpFile)

	manifest, err := readManifestFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read manifest: %v", err)
	}

	if len(manifest.Playground.Machines) != 3 {
		t.Fatalf("Expected 3 machines, got %d", len(manifest.Playground.Machines))
	}

	// Check legacy format machine
	legacyMachine := manifest.Playground.Machines[0]
	if legacyMachine.Name != "legacy-node" {
		t.Errorf("Expected first machine name 'legacy-node', got '%s'", legacyMachine.Name)
	}
	if legacyMachine.Kernel == nil {
		t.Fatal("Expected legacy machine kernel to be non-nil")
	}
	if legacyMachine.Kernel.Source != "debian-11" {
		t.Errorf("Expected legacy kernel source 'debian-11', got '%s'", legacyMachine.Kernel.Source)
	}

	// Check new format machine
	newMachine := manifest.Playground.Machines[1]
	if newMachine.Name != "new-node" {
		t.Errorf("Expected second machine name 'new-node', got '%s'", newMachine.Name)
	}
	if newMachine.Kernel == nil {
		t.Fatal("Expected new machine kernel to be non-nil")
	}
	if newMachine.Kernel.Source != "ubuntu-22.04" {
		t.Errorf("Expected new kernel source 'ubuntu-22.04', got '%s'", newMachine.Kernel.Source)
	}

	// Check machine without kernel
	noKernelMachine := manifest.Playground.Machines[2]
	if noKernelMachine.Name != "no-kernel-node" {
		t.Errorf("Expected third machine name 'no-kernel-node', got '%s'", noKernelMachine.Name)
	}
	if noKernelMachine.Kernel != nil {
		t.Errorf("Expected no-kernel machine kernel to be nil, got %+v", noKernelMachine.Kernel)
	}
}

func TestReadManifestFile_StartupFiles(t *testing.T) {
	// Top-level startupFiles entry (new form, using source + machines) plus a
	// legacy per-machine entry - both must parse.
	content := `kind: playground
name: startup-files-playground
playground:
  startupFiles:
    - path: /opt/app
      source: __static__/app/
      owner: laborant
      mode: "644"
      machines: [node-01]
  machines:
    - name: node-01
      startupFiles:
        - path: /home/laborant/.bashrc
          append: true
          content: |
            alias k=kubectl
`

	tmpFile := createTempManifest(t, content)
	defer os.Remove(tmpFile)

	manifest, err := readManifestFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read manifest: %v", err)
	}

	if len(manifest.Playground.StartupFiles) != 1 {
		t.Fatalf("Expected 1 top-level startup file, got %d", len(manifest.Playground.StartupFiles))
	}
	topLevel := manifest.Playground.StartupFiles[0]
	if topLevel.Path != "/opt/app" {
		t.Errorf("Expected path '/opt/app', got '%s'", topLevel.Path)
	}
	if topLevel.Source != "__static__/app/" {
		t.Errorf("Expected source '__static__/app/', got '%s'", topLevel.Source)
	}
	if len(topLevel.Machines) != 1 || topLevel.Machines[0] != "node-01" {
		t.Errorf("Expected machines ['node-01'], got %v", topLevel.Machines)
	}

	if len(manifest.Playground.Machines) != 1 {
		t.Fatalf("Expected 1 machine, got %d", len(manifest.Playground.Machines))
	}
	legacy := manifest.Playground.Machines[0].StartupFiles
	if len(legacy) != 1 {
		t.Fatalf("Expected 1 legacy per-machine startup file, got %d", len(legacy))
	}
	if legacy[0].Path != "/home/laborant/.bashrc" || !legacy[0].Append {
		t.Errorf("Unexpected legacy startup file: %+v", legacy[0])
	}
}

func TestStartupFile_MarshalOmitsUnsetFields(t *testing.T) {
	file := api.StartupFile{
		Path:    "/home/laborant/.bashrc",
		Content: "alias k=kubectl\n",
		Append:  true,
	}

	out, err := yaml.Marshal(file)
	if err != nil {
		t.Fatalf("Failed to marshal StartupFile: %v", err)
	}

	for _, key := range []string{"mode:", "owner:", "source:", "machines:", "extract:"} {
		if strings.Contains(string(out), key) {
			t.Errorf("Expected marshaled YAML to omit %q, got:\n%s", key, out)
		}
	}

	fileWithExtract := api.StartupFile{
		Path:    "/home/laborant/app.tar.gz",
		Source:  "__static__/app.tar.gz",
		Extract: true,
	}

	outWithExtract, err := yaml.Marshal(fileWithExtract)
	if err != nil {
		t.Fatalf("Failed to marshal StartupFile: %v", err)
	}

	if !strings.Contains(string(outWithExtract), "extract: true") {
		t.Errorf("Expected marshaled YAML to contain 'extract: true', got:\n%s", outWithExtract)
	}
}

func TestReadManifestFile_InvalidKind(t *testing.T) {
	content := `kind: invalid
name: test
playground:
  machines: []
`

	tmpFile := createTempManifest(t, content)
	defer os.Remove(tmpFile)

	_, err := readManifestFile(tmpFile)
	if err == nil {
		t.Fatal("Expected error for invalid kind, got nil")
	}
	if err.Error() != "invalid manifest kind: invalid (expected 'playground')" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestReadManifestFile_InvalidYAML(t *testing.T) {
	content := `kind: playground
name: test
invalid yaml content [[[
`

	tmpFile := createTempManifest(t, content)
	defer os.Remove(tmpFile)

	_, err := readManifestFile(tmpFile)
	if err == nil {
		t.Fatal("Expected error for invalid YAML, got nil")
	}
}

func TestReadManifestFile_NonExistentFile(t *testing.T) {
	_, err := readManifestFile("/nonexistent/path/to/manifest.yaml")
	if err == nil {
		t.Fatal("Expected error for non-existent file, got nil")
	}
}

func TestReadManifestFile_Stdin(t *testing.T) {
	// Save original stdin
	oldStdin := os.Stdin
	defer func() { os.Stdin = oldStdin }()

	// Create a temporary file to simulate stdin
	content := `kind: playground
name: stdin-playground
playground:
  machines:
    - name: node1
      kernel: ubuntu-22.04
`
	tmpFile := createTempManifest(t, content)
	defer os.Remove(tmpFile)

	// Redirect stdin
	file, err := os.Open(tmpFile)
	if err != nil {
		t.Fatalf("Failed to open temp file: %v", err)
	}
	defer file.Close()
	os.Stdin = file

	// Test reading from stdin using "-" as file path
	manifest, err := readManifestFile("-")
	if err != nil {
		t.Fatalf("Failed to read manifest from stdin: %v", err)
	}

	if manifest.Name != "stdin-playground" {
		t.Errorf("Expected name 'stdin-playground', got '%s'", manifest.Name)
	}
}

// Helper function to create a temporary manifest file
func createTempManifest(t *testing.T, content string) string {
	t.Helper()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "manifest.yaml")

	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	return tmpFile
}

func TestFindPlayByTitle(t *testing.T) {
	plays := []*api.Play{
		{ID: "id-1", Title: "web-app"},
		{ID: "id-2", Title: "web-app-staging"},
		{ID: "id-3", Title: "database"},
		{ID: "id-4", Title: ""},
	}

	tests := []struct {
		name    string
		title   string
		wantID  string
		wantErr string
	}{
		{name: "exact unique match", title: "database", wantID: "id-3"},
		{name: "unique prefix", title: "data", wantID: "id-3"},
		{name: "ambiguous prefix", title: "web", wantErr: "ambiguous title"},
		{name: "exact match shadowed by longer title", title: "web-app", wantErr: "ambiguous title"},
		{name: "longer prefix disambiguates", title: "web-app-", wantID: "id-2"},
		{name: "no match", title: "nope", wantErr: "could not find a play"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			play, err := findPlayByTitle(plays, tt.title)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if play.ID != tt.wantID {
				t.Errorf("expected play %q, got %q", tt.wantID, play.ID)
			}
		})
	}
}

func TestFindPlayByTitle_SkipsUntitledPlays(t *testing.T) {
	plays := []*api.Play{
		{ID: "untitled", Title: ""},
		{ID: "titled", Title: "only-one"},
	}

	// An empty prefix matches every title, but untitled plays must not count.
	play, err := findPlayByTitle(plays, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if play.ID != "titled" {
		t.Errorf("expected play %q, got %q", "titled", play.ID)
	}
}
