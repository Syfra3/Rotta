package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// REQ-001 -> SCN-001 -> TestSCN001_RetiredCommandsAndOptionsAreRejected
func TestSCN001_RetiredCommandsAndOptionsAreRejected(t *testing.T) {
	// Scenario: Retired workflow surfaces are unavailable
	for _, args := range [][]string{
		{"v2"},
		{"backup"},
		{"restore"},
		{"install", "--target", "opencode"},
		{"install", "--mode", "spec"},
		{"install", "--project", "example"},
		{"install", "--mcp", "vela"},
	} {
		err := runCLI(args, &bytes.Buffer{}, &bytes.Buffer{})
		if err == nil || !strings.Contains(err.Error(), "supported commands") {
			t.Fatalf("runCLI(%v) error = %v, want supported-command rejection", args, err)
		}
	}
}

// REQ-003 -> SCN-005 -> TestSCN005_StatusReportsWithoutMutation
func TestSCN005_StatusReportsWithoutMutation(t *testing.T) {
	// Scenario: Status reports without mutation
	home := t.TempDir()
	t.Setenv("HOME", home)
	var stdout bytes.Buffer
	if err := runCLI([]string{"status"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "not installed") {
		t.Fatalf("status output = %q, want bounded not-installed result", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "opencode")); !os.IsNotExist(err) {
		t.Fatalf("status created configuration path: %v", err)
	}
}

// REQ-004 -> SCN-006 -> TestSCN006_InstallWritesOnlyManagedOpenCodeAssets
func TestSCN006_InstallWritesOnlyManagedOpenCodeAssets(t *testing.T) {
	// Scenario: Install manages only OpenCode user configuration
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := runCLI([]string{"install"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(home, ".config", "opencode")
	for _, path := range []string{
		filepath.Join(root, "agents", "rotta-orchestrator.md"),
		filepath.Join(root, "agents", "rotta-spec.md"),
		filepath.Join(root, "agents", "rotta-impl.md"),
		filepath.Join(root, "agents", "rotta-review.md"),
		filepath.Join(root, ".rotta", "rotta-manifest.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("managed asset %s was not installed: %v", path, err)
		}
	}
	for _, name := range []string{"rotta-orchestrator.md", "rotta-spec.md", "rotta-impl.md", "rotta-review.md"} {
		data, err := os.ReadFile(filepath.Join(root, "agents", name))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), ".rotta/workflow") || strings.Contains(strings.ToLower(string(data)), "v2") {
			t.Fatalf("managed agent %s does not use only the non-versioned workflow root", name)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "skills", "rotta", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("install retained the retired managed skill: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude")); !os.IsNotExist(err) {
		t.Fatalf("install created non-OpenCode configuration: %v", err)
	}
}

// REQ-004 -> SCN-007 -> TestSCN007_UnmanagedConflictIsPreserved
func TestSCN007_UnmanagedConflictIsPreserved(t *testing.T) {
	// Scenario: Unmanaged configuration conflicts are preserved
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".config", "opencode", "agents", "rotta-orchestrator.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	const content = "user-owned configuration"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	err := runCLI([]string{"install"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "conflict") || !strings.Contains(err.Error(), "move or remove") {
		t.Fatalf("install error = %v, want bounded conflict and safe next action", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Fatalf("unmanaged file changed to %q", got)
	}
}
