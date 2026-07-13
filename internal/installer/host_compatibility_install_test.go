package installer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSCN211_PreserveCleanWorktreeExpectationsDuringHostInstallation(t *testing.T) {
	// REQ-006 → SCN-211 → TestSCN211_PreserveCleanWorktreeExpectationsDuringHostInstallation
	// Scenario: Preserve clean worktree expectations during host installation
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

	assertStringListContains(t, result.ChangedFiles[FileChangeCategoryHostConfig], filepath.Join(home, ".codex", "AGENTS.md"))
	assertStringListContains(t, result.ChangedFiles[FileChangeCategoryLifecycle], filepath.Join(projectPath, ".rotta", "state-machine.yaml"))
	assertStringListContains(t, result.ChangedFiles[FileChangeCategoryLifecycle], filepath.Join(projectPath, ".rotta", "quality-gates.yaml"))
	if len(result.ChangedFiles[FileChangeCategoryWorkspaceHostConfig]) != 0 {
		t.Fatalf("expected no workspace host config changes for Codex-only install, got %#v", result.ChangedFiles[FileChangeCategoryWorkspaceHostConfig])
	}
	if result.LifecycleArtifactsRequireCommit {
		t.Fatal("expected generated Rotta lifecycle artifacts not to require commits by default")
	}
}

func TestSCN212_StoreMemoryStateAsCompactPointersOnly(t *testing.T) {
	// REQ-006 → SCN-212 → TestSCN212_StoreMemoryStateAsCompactPointersOnly
	// Scenario: Store memory state as compact pointers only
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	binDir := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	writeHostCompatibilityFakeAncora(t, filepath.Join(binDir, "ancora"))

	_, err := Install(Options{
		Target:        "codex",
		ProjectPath:   projectPath,
		InstallSpec:   true,
		InstallImpl:   true,
		InstallReview: true,
		SetupAncora:   true,
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
		"Workspace files remain the source of truth for specs, Gherkin features, TDD logs, reports, and workflow state.",
		"State Index per Cycle (not the full log)",
		"log_file: .rotta/tdd-log.md",
		"completed_scenarios:",
		"last_scenario:",
		"last_test:",
		"status: green",
		"files_changed:",
		"Do not store full hard specs, feature files, TDD logs, or review reports in Ancora",
	})
	assertNotContains(t, got, "paste the full hard spec")
	assertNotContains(t, got, "copy the full feature file")
}

func TestSCN213_RerunInstallationWithoutDuplicatingRottaManagedArtifacts(t *testing.T) {
	// REQ-007, REQ-010 → SCN-213 → TestSCN213_RerunInstallationWithoutDuplicatingRottaManagedArtifacts
	// Scenario: Re-run installation without duplicating Rotta-managed artifacts
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	binDir := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	opencodeConfig := filepath.Join(home, ".config", "opencode", "opencode.json")
	codexConfig := filepath.Join(home, ".codex", "config.toml")
	writeTestFile(t, opencodeConfig, []byte(`{"mcp":{"user-server":{"command":"keep"}},"theme":"keep"}`))
	writeTestFile(t, codexConfig, []byte("model = \"gpt-5\"\n"))
	writeExecutable(t, filepath.Join(binDir, "ancora"), `#!/bin/sh
exit 0
`)
	writeExecutable(t, filepath.Join(binDir, "vela"), `#!/bin/sh
project=""
agent=""
claude_dir=""
opencode_dir=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --project) shift; project="$1" ;;
    --agent) shift; agent="$1" ;;
    --claude-dir) shift; claude_dir="$1" ;;
    --opencode-dir) shift; opencode_dir="$1" ;;
  esac
  shift
done
mkdir -p "$project/.vela"
printf 'fresh graph' > "$project/.vela/graph.db"
if [ "$agent" = claude ]; then
  mkdir -p "$claude_dir"
  printf '{"type":"stdio","command":"vela","args":["mcp"]}' > "$claude_dir/vela-mcp.json"
fi
if [ "$agent" = opencode ]; then
  mkdir -p "$opencode_dir"
  printf '{"mcp":{"vela":{"type":"stdio","command":"vela","args":["mcp"]}}}' > "$opencode_dir/opencode-vela.json"
fi
`)
	writeContext7StrictFakeNPX(t, filepath.Join(binDir, "npx"), true, []string{"resolve-library-id", "query-docs"})

	options := Options{
		Target:        "all",
		ProjectPath:   projectPath,
		InstallSpec:   true,
		InstallImpl:   true,
		InstallReview: true,
		SetupAncora:   true,
		SetupVela:     true,
		SetupContext7: true,
	}
	if _, err := Install(options); err != nil {
		t.Fatal(err)
	}

	result, err := Install(options)
	if err != nil {
		t.Fatal(err)
	}

	assertNoDuplicateStrings(t, result.Files)
	assertNoDuplicateStrings(t, result.Hosts["claude-code"].Files)
	assertNoDuplicateStrings(t, result.Hosts["opencode"].Files)
	assertNoDuplicateStrings(t, result.Hosts["codex"].Files)
	assertFileContains(t, opencodeConfig, "user-server")
	assertFileContains(t, opencodeConfig, "theme")
	assertFileContains(t, codexConfig, "model = \"gpt-5\"")
	assertFileContainsCount(t, opencodeConfig, `"context7"`, 1)
	assertFileContainsCount(t, codexConfig, "[mcp_servers.ancora]", 1)
	assertFileContainsCount(t, codexConfig, "[mcp_servers.vela]", 1)
	assertFileContainsCount(t, codexConfig, "[mcp_servers.context7]", 1)
}
