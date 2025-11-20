package playground

import (
	"os"
	"path/filepath"
	"testing"
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
