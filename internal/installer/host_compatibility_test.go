package installer

import (
	"os"
	"path/filepath"
	"strings"
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

func TestSCN203_RejectUnsupportedHostBeforeMutation(t *testing.T) {
	// REQ-001, REQ-009 → SCN-203 → TestSCN203_RejectUnsupportedHostBeforeMutation
	// Scenario: Reject an unsupported host before mutation
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	t.Setenv("HOME", home)

	result, err := Install(Options{
		Target:        "cursor",
		ProjectPath:   projectPath,
		InstallSpec:   true,
		InstallImpl:   true,
		InstallReview: true,
	})
	if err == nil {
		t.Fatal("expected unsupported host to be rejected")
	}
	if result != nil {
		t.Fatalf("expected no install result after unsupported host rejection, got %#v", result)
	}
	if !strings.Contains(err.Error(), "supported hosts are exactly Claude Code, OpenCode, and Codex") {
		t.Fatalf("expected supported host explanation, got %q", err.Error())
	}

	assertPathMissing(t, filepath.Join(home, ".claude"))
	assertPathMissing(t, filepath.Join(home, ".config", "opencode"))
	assertPathMissing(t, filepath.Join(home, ".codex"))
	assertPathMissing(t, filepath.Join(projectPath, ".rotta"))
}

func TestSCN204_GenerateHostSpecificInstructionsFromCanonicalWorkflow(t *testing.T) {
	// REQ-003, REQ-008 → SCN-204 → TestSCN204_GenerateHostSpecificInstructionsFromCanonicalWorkflow
	// Scenario: Generate host-specific instructions from the canonical Rotta workflow
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	t.Setenv("HOME", home)

	result, err := Install(Options{
		Target:        "all",
		ProjectPath:   projectPath,
		InstallSpec:   true,
		InstallImpl:   true,
		InstallReview: true,
		SetupAncora:   true,
		SetupVela:     true,
	})
	if err != nil {
		t.Fatal(err)
	}

	hostInstructionFiles := map[string]string{
		"claude-code": filepath.Join(home, ".claude", "skills", "rotta", "implementation-mode", "SKILL.md"),
		"opencode":    filepath.Join(home, ".config", "opencode", "skills", "rotta-orchestrator", "SKILL.md"),
		"codex":       filepath.Join(home, ".codex", "AGENTS.md"),
	}
	for host, path := range hostInstructionFiles {
		assertStringListContains(t, result.Hosts[host].Files, path)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s instructions: %v", host, err)
		}
		got := string(data)
		assertContainsAll(t, got, []string{
			"Rotta Canonical Workflow Contract",
			"Phase 1 — Draft",
			"Phase 2 — Spec + Gherkin",
			"Phase 3 — TDD",
			"Phase 4 — Review",
			"Do NOT advance without explicit human approval",
			"strict Red/Green/Refactor TDD",
			"The Judge reviews evidence, not code",
			"no AI attribution",
			"Workspace files are the source of truth",
			"Ancora stores compact pointers/status only",
			"Capability Summary",
		})
	}
}

func TestSCN205_DiscloseAdaptedPrimitiveSupportForCodex(t *testing.T) {
	// REQ-003, REQ-008 → SCN-205 → TestSCN205_DiscloseAdaptedPrimitiveSupportForCodex
	// Scenario: Disclose when a host lacks an exact agent or skill primitive
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	t.Setenv("HOME", home)

	_, err := Install(Options{
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
	data, err := os.ReadFile(codexInstructions)
	if err != nil {
		t.Fatalf("read Codex instructions: %v", err)
	}
	got := string(data)
	assertContainsAll(t, got, []string{
		"Codex: host instructions are adapted into a Codex-consumable `AGENTS.md` instruction file",
		"Agent capability: adapted",
		"Skill capability: adapted",
	})
	assertNotContains(t, got, "Codex: host instructions are exact OpenCode agent and skill artifacts")
	assertNotContains(t, got, "Codex: exact agent and skill support")
}
