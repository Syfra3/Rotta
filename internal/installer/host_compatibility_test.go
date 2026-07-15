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

// REQ-015 → SCN-301 → TestSCN301_InstallGlobalClaudeOrchestratorAndHiddenPhaseAgents
func TestSCN301_InstallGlobalClaudeOrchestratorAndHiddenPhaseAgents(t *testing.T) {
	// Scenario: Install the global Claude orchestration surface and phase agents
	home := t.TempDir()
	t.Setenv("HOME", home)

	_, err := Install(Options{
		Target:        "claude-code",
		ProjectPath:   filepath.Join(home, "project"),
		InstallSpec:   true,
		InstallImpl:   true,
		InstallReview: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	agentsDir := filepath.Join(home, ".claude", "agents")
	assertFileContains(t, filepath.Join(agentsDir, "rotta-orchestrator.md"), "name: rotta-orchestrator")
	for _, name := range []string{"rotta-spec", "rotta-impl", "rotta-review"} {
		path := filepath.Join(agentsDir, name+".md")
		assertFileContains(t, path, "name: "+name)
		assertFileContains(t, path, "user-invocable: false")
		assertFileContains(t, path, "model: inherit")
		assertFileContains(t, path, "You are a sub-agent invoked by the Rotta-Orchestrator.")
	}
}

// REQ-008 → SCN-353 → TestSCN353_ClaudeArtifactInstallationRequiresNoLocalExecutable
func TestSCN353_ClaudeArtifactInstallationRequiresNoLocalExecutable(t *testing.T) {
	// Scenario: Installing Claude artifacts does not require a local Claude executable
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())

	result, err := Install(Options{
		Target:        "claude-code",
		ProjectPath:   filepath.Join(home, "project"),
		InstallSpec:   true,
		InstallImpl:   true,
		InstallReview: true,
	})
	if err != nil {
		t.Fatalf("install Claude artifacts without claude executable: %v", err)
	}
	if result.Hosts["claude-code"].Status != HostInstallStatusInstalled {
		t.Fatalf("expected Claude artifacts installed without a local executable, got %#v", result.Hosts["claude-code"])
	}
	assertFileContains(t, filepath.Join(home, ".claude", "agents", "rotta-orchestrator.md"), "Artifact installation does not require a local Claude executable and makes no runtime compatibility verification claim.")
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
	home, options := setupCanonicalWorkflowInstall(t)
	result, err := Install(options)
	if err != nil {
		t.Fatal(err)
	}

	assertCanonicalWorkflowInstructions(t, result, home)
}

// REQ-050, REQ-051 → SCN-323 → TestSCN323_GeneratesLifecycleAuthorityRulesForEverySupportedHost
func TestSCN323_GeneratesLifecycleAuthorityRulesForEverySupportedHost(t *testing.T) {
	// Scenario: Generate the same lifecycle authority rules for every supported host
	home, options := setupCanonicalWorkflowInstall(t)
	result, err := Install(options)
	if err != nil {
		t.Fatal(err)
	}

	for host, path := range map[string]string{
		"claude-code": filepath.Join(home, ".claude", "agents", "rotta-orchestrator.md"),
		"opencode":    filepath.Join(home, ".config", "opencode", "skills", "rotta-orchestrator", "SKILL.md"),
		"codex":       filepath.Join(home, ".codex", "AGENTS.md"),
	} {
		if result.Hosts[host].Status != HostInstallStatusInstalled {
			t.Fatalf("%s was not installed: %#v", host, result.Hosts[host])
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s instructions: %v", host, err)
		}
		assertContainsAll(t, string(data), []string{
			"recorded pre-spec feature-worktree",
			"feature-scoped approval record as authoritative instead of `specs/.approved`",
			"autonomous scenario checkpoint",
			"archive and eligible explicit cleanup lifecycle",
			"does not create a second worktree after approval",
		})
	}

	for host, path := range map[string]string{
		"claude-code": filepath.Join(home, ".claude", "skills", "rotta", "implementation-mode", "SKILL.md"),
		"opencode":    filepath.Join(home, ".config", "opencode", "skills", "rotta-impl", "SKILL.md"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s implementation instructions: %v", host, err)
		}
		assertFileContains(t, path, "matching feature-scoped approval record and committed baseline")
		assertNotContains(t, string(data), "`specs/.approved` exists and contains the scenario ID")
	}

	for _, path := range []string{
		filepath.Join(home, ".claude", "skills", "rotta", "spec-mode", "SKILL.md"),
		filepath.Join(home, ".claude", "skills", "rotta", "review-mode", "SKILL.md"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read Claude lifecycle instructions: %v", err)
		}
		assertNotContains(t, string(data), "write `specs/.approved`")
		assertNotContains(t, string(data), "in `specs/.approved`")
	}
}

func TestGeneratedHostInstructionsApplyProportionalWorkflowPolicy(t *testing.T) {
	home, options := setupCanonicalWorkflowInstall(t)
	result, err := Install(options)
	if err != nil {
		t.Fatal(err)
	}

	for host, path := range map[string]string{
		"claude-code": filepath.Join(home, ".claude", "skills", "rotta", "implementation-mode", "SKILL.md"),
		"opencode":    filepath.Join(home, ".config", "opencode", "skills", "rotta-orchestrator", "SKILL.md"),
		"codex":       filepath.Join(home, ".codex", "AGENTS.md"),
	} {
		if result.Hosts[host].Status != HostInstallStatusInstalled {
			t.Fatalf("%s was not installed: %#v", host, result.Hosts[host])
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s instructions: %v", host, err)
		}
		assertContainsAll(t, string(data), []string{
			"Workflow Selection (MANDATORY)",
			"direct, narrowly verified path",
			"focused impact assessment and appropriate focused verification",
			"A Makefile change alone is not automatically low-risk",
			"Use the full workflow for ambiguous, multi-component, destructive, security, auth, payments, infrastructure, secrets, migrations, public-contract, data-loss, or behaviorally significant changes.",
			"When uncertain, use the full workflow.",
			"Phase 2 — Spec + Gherkin",
			"Do NOT advance without explicit human approval",
			"Phase 3 — TDD",
			"strict Red/Green/Refactor TDD",
			"Phase 4 — Review",
		})
	}
}

// REQ-042, REQ-043 → SCN-248 → TestSCN248_GeneratedHostInstructionsPreserveManualPRHandoffPolicy
func TestSCN248_GeneratedHostInstructionsPreserveManualPRHandoffPolicy(t *testing.T) {
	// Scenario: Present resolved manual GitHub PR handoff after Phase 4 passes
	home, options := setupCanonicalWorkflowInstall(t)
	result, err := Install(options)
	if err != nil {
		t.Fatal(err)
	}

	for host, path := range map[string]string{
		"claude-code": filepath.Join(home, ".claude", "skills", "rotta", "implementation-mode", "SKILL.md"),
		"opencode":    filepath.Join(home, ".config", "opencode", "skills", "rotta-orchestrator", "SKILL.md"),
		"codex":       filepath.Join(home, ".codex", "AGENTS.md"),
	} {
		if result.Hosts[host].Status != HostInstallStatusInstalled {
			t.Fatalf("%s was not installed: %#v", host, result.Hosts[host])
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s instructions: %v", host, err)
		}
		assertContainsAll(t, string(data), []string{
			"After Phase 4 passes, provide a testable manual GitHub PR handoff",
			"exactly one GitHub-capable push remote",
			"Do not push, create a pull request, merge, or directly modify the base branch",
		})
	}
}

func setupCanonicalWorkflowInstall(t *testing.T) (string, Options) {
	t.Helper()
	home := t.TempDir()
	binDir := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	writeHostCompatibilityFakeAncora(t, filepath.Join(binDir, "ancora"))
	writeHostCompatibilityFakeVela(t, filepath.Join(binDir, "vela"))
	return home, Options{Target: "all", ProjectPath: filepath.Join(home, "project"), InstallSpec: true, InstallImpl: true, InstallReview: true, SetupAncora: true, SetupVela: true}
}

func assertCanonicalWorkflowInstructions(t *testing.T, result *Result, home string) {
	t.Helper()
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
			"Every TDD scenario task starts clean",
			"checkpoint or clean the task diff before starting another scenario",
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
	home, opencodeConfig, options := setupSelectedMCPHostInstall(t)
	result, err := Install(options)
	if err != nil {
		t.Fatal(err)
	}

	assertSelectedMCPHostArtifacts(t, result, home, opencodeConfig, options.ProjectPath)
}

func setupSelectedMCPHostInstall(t *testing.T) (string, string, Options) {
	t.Helper()
	home := t.TempDir()
	binDir := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	opencodeConfig := filepath.Join(home, ".config", "opencode", "opencode.json")
	writeTestFile(t, opencodeConfig, []byte(`{"mcp":{"user-server":{"command":"keep"}},"theme":"keep"}`))
	writeTestFile(t, filepath.Join(home, ".claude", "mcp", "user.json"), []byte(`{"command":"keep"}`))
	writeTestFile(t, filepath.Join(home, ".codex", "config.toml"), []byte("model = \"gpt-5\"\n"))
	writeLoggingAncora(t, filepath.Join(binDir, "ancora"))
	writeLoggingVela(t, filepath.Join(binDir, "vela"))
	writeContext7StrictFakeNPX(t, filepath.Join(binDir, "npx"), true, []string{"resolve-library-id", "query-docs"})
	return home, opencodeConfig, Options{Target: "all", ProjectPath: filepath.Join(home, "project"), InstallSpec: true, InstallImpl: true, InstallReview: true, SetupAncora: true, SetupVela: true, SetupContext7: true}
}

func assertSelectedMCPHostArtifacts(t *testing.T, result *Result, home, opencodeConfig, projectPath string) {
	t.Helper()
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
	binDir := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	writeHostCompatibilityFakeAncora(t, filepath.Join(binDir, "ancora"))
	writeContext7StrictFakeNPX(t, filepath.Join(binDir, "npx"), true, []string{"resolve-library-id", "query-docs"})

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

// REQ-005 → SCN-341 → TestSCN341_AdaptedHostPhaseRequestsRouteThroughOrchestrator
func TestSCN341_AdaptedHostPhaseRequestsRouteThroughOrchestrator(t *testing.T) {
	// Scenario: Host capability differences do not permit direct phase execution
	home := t.TempDir()
	t.Setenv("HOME", home)

	_, err := Install(Options{
		Target:        "codex",
		ProjectPath:   filepath.Join(home, "project"),
		InstallSpec:   true,
		InstallImpl:   true,
		InstallReview: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(home, ".codex", "AGENTS.md"))
	if err != nil {
		t.Fatalf("read Codex instructions: %v", err)
	}

	assertContainsAll(t, string(data), []string{
		"every user request for specification, implementation, or review MUST first route to the Rotta-Orchestrator decision point",
		"Natural-language command adaptation never permits direct phase execution",
	})
}
