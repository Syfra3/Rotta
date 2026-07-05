package installer

import (
	"os"
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

func TestSCN202_InstallRottaIntoAllSupportedHostsWithIndependentResults(t *testing.T) {
	// REQ-001, REQ-002 → SCN-202 → TestSCN202_InstallRottaIntoAllSupportedHostsWithIndependentResults
	// Scenario: Install Rotta into all supported hosts with independent results
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	t.Setenv("HOME", home)

	openCodeDir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(openCodeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(openCodeDir, "opencode.json"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Install(Options{
		Target:        "all",
		ProjectPath:   projectPath,
		InstallSpec:   true,
		InstallImpl:   true,
		InstallReview: true,
	})
	if err == nil {
		t.Fatal("expected an install error for the blocked OpenCode host")
	}
	if result == nil {
		t.Fatal("expected partial result when one selected host fails")
	}

	if len(result.Hosts) != 3 {
		t.Fatalf("expected exactly three host results, got %#v", result.Hosts)
	}
	if result.Hosts["claude-code"].Status != HostInstallStatusInstalled {
		t.Fatalf("expected Claude Code installed independently, got %#v", result.Hosts["claude-code"])
	}
	if result.Hosts["opencode"].Status != HostInstallStatusFailed {
		t.Fatalf("expected OpenCode failed independently, got %#v", result.Hosts["opencode"])
	}
	if result.Hosts["codex"].Status != HostInstallStatusInstalled {
		t.Fatalf("expected Codex installed independently, got %#v", result.Hosts["codex"])
	}

	assertPathExists(t, filepath.Join(home, ".claude", "skills", "rotta"))
	assertPathExists(t, filepath.Join(home, ".codex", "AGENTS.md"))
}
