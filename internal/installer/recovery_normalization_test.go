package installer

import (
	"path/filepath"
	"testing"
)

// REQ-029 → SCN-232 → TestSCN232_NormalizesRecoveredStaleManagedCommandIdempotently
func TestSCN232_NormalizesRecoveredStaleManagedCommandIdempotently(t *testing.T) {
	// Scenario: Normalize a recovered stale managed executable during reinstall
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	project := filepath.Join(home, "project")
	configPath := filepath.Join(home, ".claude", "vela-mcp.json")
	t.Setenv("HOME", home)
	t.Setenv("PATH", bin+":/bin")
	writeExecutable(t, filepath.Join(bin, "vela"), "#!/bin/sh\nexit 0\n")
	writeTestFile(t, configPath, []byte(`{"command":"/home/linuxbrew/.linuxbrew/Cellar/vela/4.5.6/bin/vela","args":["mcp","--transport","stdio"]}`))

	first, err := SetupVela(Options{Target: "claude-code"}, home, project)
	if err != nil {
		t.Fatalf("first Vela reinstall: %v", err)
	}
	if got := first.NormalizedMCPEntries; len(got) != 1 || got[0] != configPath {
		t.Fatalf("expected normalized recovered entry %q, got %v", configPath, got)
	}
	if got := serializedMCPCommand(t, mustReadFile(t, configPath), ""); got != "vela" {
		t.Fatalf("expected normalized Vela command vela, got %q", got)
	}

	second, err := SetupVela(Options{Target: "claude-code"}, home, project)
	if err != nil {
		t.Fatalf("second Vela reinstall: %v", err)
	}
	if got := second.NormalizedMCPEntries; len(got) != 0 {
		t.Fatalf("expected idempotent reinstall to report no command-field change, got %v", got)
	}
}
