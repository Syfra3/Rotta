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

func TestSCN206_ConfigureSelectedMCPServersAcrossSelectedHosts(t *testing.T) {
	// REQ-004, REQ-010 → SCN-206 → TestSCN206_ConfigureSelectedMCPServersAcrossSelectedHosts
	// Scenario: Configure selected MCP servers across selected hosts
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	binDir := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	opencodeConfig := filepath.Join(home, ".config", "opencode", "opencode.json")
	writeTestFile(t, opencodeConfig, []byte(`{"mcp":{"user-server":{"command":"keep"}},"theme":"keep"}`))
	writeTestFile(t, filepath.Join(home, ".claude", "mcp", "user.json"), []byte(`{"command":"keep"}`))
	writeTestFile(t, filepath.Join(home, ".codex", "config.toml"), []byte("model = \"gpt-5\"\n"))
	writeExecutable(t, filepath.Join(binDir, "ancora"), `#!/bin/sh
printf 'ancora %s\n' "$*" >> "$HOME/setup.log"
if [ "$1" = setup ] && [ "$2" = claude-code ]; then
  mkdir -p "$HOME/.claude/mcp"
  printf '{"type":"stdio","command":"ancora","args":["mcp"]}' > "$HOME/.claude/mcp/ancora.json"
fi
if [ "$1" = setup ] && [ "$2" = opencode ]; then
  mkdir -p "$HOME/.config/opencode"
  printf '{"mcp":{"ancora":{"type":"stdio","command":"ancora","args":["mcp"]}}}' > "$HOME/.config/opencode/opencode.jsonc"
fi
`)
	writeExecutable(t, filepath.Join(binDir, "vela"), `#!/bin/sh
printf 'vela %s\n' "$*" >> "$HOME/setup.log"
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

	result, err := Install(Options{
		Target:        "all",
		ProjectPath:   projectPath,
		InstallSpec:   true,
		InstallImpl:   true,
		InstallReview: true,
		SetupAncora:   true,
		SetupVela:     true,
		SetupContext7: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	assertFileContains(t, filepath.Join(home, ".claude", "mcp", "ancora.json"), "ancora")
	assertFileContains(t, filepath.Join(home, ".claude", "vela-mcp.json"), "vela")
	assertContext7ClaudeEntry(t, filepath.Join(home, ".claude", "mcp", "context7.json"))
	assertFileContains(t, filepath.Join(home, ".claude", "mcp", "user.json"), "keep")
	assertFileContains(t, filepath.Join(home, ".config", "opencode", "opencode.jsonc"), "ancora")
	assertFileContains(t, filepath.Join(home, ".config", "opencode", "opencode-vela.json"), "vela")
	assertContext7OpenCodeEntry(t, opencodeConfig)
	assertFileContains(t, opencodeConfig, "user-server")
	assertFileContains(t, opencodeConfig, "theme")
	assertFileContains(t, filepath.Join(home, ".codex", "config.toml"), "model = \"gpt-5\"")
	assertFileContains(t, filepath.Join(home, ".codex", "config.toml"), "[mcp_servers.ancora]")
	assertFileContains(t, filepath.Join(home, ".codex", "config.toml"), "[mcp_servers.vela]")
	assertFileContains(t, filepath.Join(home, ".codex", "config.toml"), "[mcp_servers.context7]")
	assertFileContains(t, filepath.Join(home, "setup.log"), "ancora setup claude-code")
	assertFileContains(t, filepath.Join(home, "setup.log"), "ancora setup opencode")
	assertFileContains(t, filepath.Join(home, "setup.log"), "vela install --project "+projectPath+" --agent claude")
	assertFileContains(t, filepath.Join(home, "setup.log"), "vela install --project "+projectPath+" --agent opencode")
	assertStringListContains(t, result.Files, filepath.Join(home, ".codex", "config.toml"))
}

func TestSCN207_ReportUnsupportedMCPCapabilityWithoutPretendingParity(t *testing.T) {
	// REQ-004, REQ-008, REQ-009 → SCN-207 → TestSCN207_ReportUnsupportedMCPCapabilityWithoutPretendingParity
	// Scenario: Report unsupported MCP capability without pretending parity
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	t.Setenv("HOME", home)

	result, err := Install(Options{
		Target:        "codex",
		ProjectPath:   projectPath,
		InstallSpec:   true,
		InstallImpl:   true,
		InstallReview: true,
		SetupAncora:   true,
		SetupContext7: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	codexConfig := filepath.Join(home, ".codex", "config.toml")
	assertFileContains(t, codexConfig, "[mcp_servers.ancora]")
	assertFileContains(t, codexConfig, "[mcp_servers.context7]")

	context7Capability := result.Hosts["codex"].Capabilities["mcp:context7"]
	if context7Capability.Status != HostCapabilityStatusDegraded {
		t.Fatalf("expected Codex Context7 MCP to be reported as degraded rather than full parity, got %#v", context7Capability)
	}
	if context7Capability.Reason == "" || context7Capability.Remediation == "" {
		t.Fatalf("expected degraded capability to include reason and remediation, got %#v", context7Capability)
	}
	ancoraCapability := result.Hosts["codex"].Capabilities["mcp:ancora"]
	if ancoraCapability.Status != HostCapabilityStatusExact {
		t.Fatalf("expected unrelated supported Ancora MCP capability to continue as exact, got %#v", ancoraCapability)
	}
}

func TestSCN208_MCPHealthCheckReportsObservableStartupFailure(t *testing.T) {
	// REQ-004, REQ-009 → SCN-208 → TestSCN208_MCPHealthCheckReportsObservableStartupFailure
	// Scenario: MCP health check reports observable startup failure
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	binDir := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	writeExecutable(t, filepath.Join(binDir, "npx"), `#!/bin/sh
exit 2
`)

	result, err := Install(Options{
		Target:        "opencode",
		ProjectPath:   projectPath,
		InstallSpec:   true,
		InstallImpl:   true,
		InstallReview: true,
		SetupContext7: true,
	})
	if err == nil {
		t.Fatal("expected observable MCP health failure to fail the host installation")
	}
	if result == nil {
		t.Fatal("expected partial install result with MCP health failure details")
	}

	host := result.Hosts["opencode"]
	if host.Status != HostInstallStatusFailed {
		t.Fatalf("expected host installation not to be fully successful after MCP health failure, got %#v", host)
	}
	capability := host.Capabilities["mcp:context7"]
	if string(capability.Status) != "failed" {
		t.Fatalf("expected failed Context7 MCP capability, got %#v", capability)
	}
	if !strings.Contains(capability.Reason, string(Context7FailureStartup)) {
		t.Fatalf("expected capability reason to identify startup failure, got %#v", capability)
	}
}

func TestSCN209_ContinueRottaWorkflowFromDifferentSupportedHost(t *testing.T) {
	// REQ-005, REQ-006 → SCN-209 → TestSCN209_ContinueRottaWorkflowFromDifferentSupportedHost
	// Scenario: Continue a Rotta workflow from a different supported host
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	t.Setenv("HOME", home)

	_, err := Install(Options{
		Target:        "all",
		ProjectPath:   projectPath,
		InstallSpec:   true,
		InstallImpl:   true,
		InstallReview: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, filepath.Join(projectPath, "specs", "hard_spec.md"), []byte("# shared spec\n"))
	writeTestFile(t, filepath.Join(projectPath, "features", "workflow.feature"), []byte("@SCN-209\nScenario: shared workflow\n"))
	writeTestFile(t, filepath.Join(projectPath, ".rotta", "tdd-log.md"), []byte("# shared TDD log\n"))

	for host, path := range map[string]string{
		"claude-code": filepath.Join(home, ".claude", "skills", "rotta", "implementation-mode", "SKILL.md"),
		"codex":       filepath.Join(home, ".codex", "AGENTS.md"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s instructions: %v", host, err)
		}
		assertContainsAll(t, string(data), []string{
			"When continuing a workflow started from another supported host, read shared workspace state before acting",
			"specs/",
			"features/",
			".rotta/",
			"preserve the same phase order, command semantics, and approval gates",
			"Do not treat host-local config as the workflow source of truth",
		})
	}
}

func TestSCN210_PreserveCommandBehaviorWithAdaptedHostInvocation(t *testing.T) {
	// REQ-005, REQ-008 → SCN-210 → TestSCN210_PreserveCommandBehaviorWithAdaptedHostInvocation
	// Scenario: Preserve command behavior when a host requires aliases or adapted command exposure
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
	data, err := os.ReadFile(codexInstructions)
	if err != nil {
		t.Fatalf("read Codex instructions: %v", err)
	}
	assertContainsAll(t, string(data), []string{
		"Command invocation for hosts without slash commands",
		"Use natural-language invocations such as `Rotta init`, `Rotta new`, `Rotta continue`, `Rotta status`, `Rotta skip`, and `Rotta back`",
		"These adapted invocations map to the same canonical Rotta command behavior and state transitions as exact command surfaces.",
		"Command capability: adapted",
	})

	capability := result.Hosts["codex"].Capabilities["commands"]
	if capability.Status != HostCapabilityStatusAdapted {
		t.Fatalf("expected Codex command capability to be adapted, got %#v", capability)
	}
	if !strings.Contains(capability.Reason, "natural-language") || !strings.Contains(capability.Remediation, "same canonical Rotta commands") {
		t.Fatalf("expected adapted command capability to document invocation path and mapping, got %#v", capability)
	}
}

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
	t.Setenv("HOME", home)

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

func TestSCN214_RecoverSafelyFromPartialMultiHostInstallFailure(t *testing.T) {
	// REQ-007, REQ-009 → SCN-214 → TestSCN214_RecoverSafelyFromPartialMultiHostInstallFailure
	// Scenario: Recover safely from a partial multi-host install failure
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	binDir := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	writeExecutable(t, filepath.Join(binDir, "ancora"), `#!/bin/sh
if [ "$1" = setup ] && [ "$2" = opencode ]; then
  mkdir -p "$HOME/.config/opencode"
  printf '{"mcp":{"ancora":{"type":"stdio","command":"ancora","args":["mcp"]}}}' > "$HOME/.config/opencode/opencode.jsonc"
fi
`)
	writeTestFile(t, filepath.Join(home, ".codex", "config.toml", "blocked"), []byte("not a file\n"))

	result, err := Install(Options{
		Target:        "all",
		ProjectPath:   projectPath,
		InstallSpec:   true,
		InstallImpl:   true,
		InstallReview: true,
		SetupAncora:   true,
	})
	if err == nil {
		t.Fatal("expected Codex configuration failure to report partial install failure")
	}
	if result == nil {
		t.Fatal("expected partial result with completed host configuration and recovery guidance")
	}

	if result.Hosts["opencode"].Status != HostInstallStatusInstalled {
		t.Fatalf("expected completed OpenCode host configuration to remain installed, got %#v", result.Hosts["opencode"])
	}
	assertFileContains(t, filepath.Join(home, ".config", "opencode", "opencode.json"), "rotta-orchestrator")
	assertFileContains(t, filepath.Join(home, ".config", "opencode", "opencode.jsonc"), "ancora")

	codex := result.Hosts["codex"]
	if codex.Status != HostInstallStatusFailed {
		t.Fatalf("expected Codex host to be marked failed, got %#v", codex)
	}
	capability := codex.Capabilities["mcp:ancora"]
	if capability.Status != HostCapabilityStatusFailed {
		t.Fatalf("expected failed Codex MCP artifact capability, got %#v", capability)
	}
	if !strings.Contains(capability.Reason, "Codex MCP config") || !strings.Contains(capability.Remediation, "safe to rerun") {
		t.Fatalf("expected failed artifact type and safe recovery guidance, got %#v", capability)
	}
	if !strings.Contains(err.Error(), "codex") || !strings.Contains(err.Error(), "MCP config") {
		t.Fatalf("expected error to identify Codex and failed artifact type, got %v", err)
	}
}

func assertNoDuplicateStrings(t *testing.T, values []string) {
	t.Helper()
	seen := map[string]bool{}
	for _, value := range values {
		if seen[value] {
			t.Fatalf("expected no duplicate entries, found duplicate %q in %#v", value, values)
		}
		seen[value] = true
	}
}

func assertFileContainsCount(t *testing.T, path, want string, count int) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if got := strings.Count(string(data), want); got != count {
		t.Fatalf("expected %s to contain %q %d time(s), got %d: %s", path, want, count, got, string(data))
	}
}
