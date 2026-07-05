package installer

import (
	"path/filepath"
	"testing"
)

func TestSCN201_InstallRottaIntoSingleSupportedCodexHost(t *testing.T) {
	// REQ-001, REQ-002 → SCN-201 → TestSCN201_InstallRottaIntoSingleSupportedCodexHost
	// Scenario: Install Rotta into a single supported host
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	t.Setenv("HOME", home)

	result, err := Install(Options{
		Target:        "codex",
		ProjectPath:   projectPath,
		InstallSpec:   true,
		InstallImpl:   true,
		InstallReview: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	codexInstructions := filepath.Join(home, ".codex", "AGENTS.md")
	assertPathExists(t, codexInstructions)
	assertFileContains(t, codexInstructions, "Rotta")
	assertStringListContains(t, result.Files, codexInstructions)
	if result.Hosts["codex"].Status != HostInstallStatusInstalled {
		t.Fatalf("expected Codex host summary to report installed, got %#v", result.Hosts)
	}

	assertPathMissing(t, filepath.Join(home, ".claude", "settings.json"))
	assertPathMissing(t, filepath.Join(home, ".claude", "skills", "rotta"))
	assertPathMissing(t, filepath.Join(home, ".config", "opencode", "opencode.json"))
	assertPathMissing(t, filepath.Join(home, ".config", "opencode", "skills", "rotta-orchestrator"))
}
