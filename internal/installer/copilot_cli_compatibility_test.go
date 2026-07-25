package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// REQ-100 → SCN-402 → TestSCN402_InstallsOnlyCopilotCLI
func TestSCN402_InstallsOnlyCopilotCLI(t *testing.T) {
	// Scenario: Select GitHub Copilot CLI as the only installation target
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	t.Setenv("HOME", home)

	unchanged := map[string]string{
		filepath.Join(home, ".claude", "settings.json"):             "claude settings",
		filepath.Join(home, ".config", "opencode", "opencode.json"): "opencode settings",
		filepath.Join(home, ".codex", "config.toml"):                "codex settings",
	}
	for path, content := range unchanged {
		writeTestFile(t, path, []byte(content))
	}

	result, err := Install(Options{Target: "copilot-cli", ProjectPath: projectPath})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Hosts) != 1 || result.Hosts["copilot-cli"].Status != HostInstallStatusInstalled {
		t.Fatalf("expected only Copilot CLI to be installed and reported, got %#v", result.Hosts)
	}
	for path, want := range unchanged {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read unchanged host configuration %s: %v", path, err)
		}
		if string(got) != want {
			t.Fatalf("expected %s to remain unchanged, got %q", path, got)
		}
	}
}

// REQ-100 → REQ-104 → SCN-403 → TestSCN403_InstallsAndReportsEverySupportedHost
func TestSCN403_InstallsAndReportsEverySupportedHost(t *testing.T) {
	// Scenario: Select every supported host through an explicit aggregate label
	home := t.TempDir()
	t.Setenv("HOME", home)

	result, err := Install(Options{Target: "all", ProjectPath: filepath.Join(home, "project")})
	if err != nil {
		t.Fatal(err)
	}

	wantHosts := []string{"claude-code", "opencode", "codex", "copilot-cli"}
	if len(result.Hosts) != len(wantHosts) {
		t.Fatalf("expected one result for each supported host, got %#v", result.Hosts)
	}
	for _, host := range wantHosts {
		if result.Hosts[host].Status != HostInstallStatusInstalled {
			t.Fatalf("expected %s to be attempted and installed once, got %#v", host, result.Hosts[host])
		}
	}
}

// REQ-100 → REQ-105 → SCN-404 → TestSCN404_PreservesLegacyBothWithoutCopilotSideEffects
func TestSCN404_PreservesLegacyBothWithoutCopilotSideEffects(t *testing.T) {
	// Scenario: Preserve the legacy two-host target string
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	copilotRoot := filepath.Join(home, "copilot-global")
	copilotArtifact := filepath.Join(copilotRoot, "existing.md")
	t.Setenv("HOME", home)
	t.Setenv("COPILOT_HOME", copilotRoot)
	writeTestFile(t, copilotArtifact, []byte("existing Copilot configuration"))

	result, err := Install(Options{Target: "both", ProjectPath: projectPath})
	if err != nil {
		t.Fatal(err)
	}

	wantHosts := []string{"claude-code", "opencode"}
	if len(result.Hosts) != len(wantHosts) {
		t.Fatalf("expected legacy both target to report only Claude Code and OpenCode, got %#v", result.Hosts)
	}
	for _, host := range wantHosts {
		if result.Hosts[host].Status != HostInstallStatusInstalled {
			t.Fatalf("expected legacy both target to install %s, got %#v", host, result.Hosts[host])
		}
	}
	if _, reported := result.Hosts["copilot-cli"]; reported {
		t.Fatalf("legacy both target must not report Copilot CLI, got %#v", result.Hosts)
	}

	data, err := os.ReadFile(copilotArtifact)
	if err != nil {
		t.Fatalf("read existing Copilot artifact: %v", err)
	}
	if string(data) != "existing Copilot configuration" {
		t.Fatalf("legacy both target must not clean Copilot artifacts, got %q", data)
	}
	assertPathMissing(t, filepath.Join(result.BackupDir, "files", "home", "copilot-global"))
	for _, file := range result.Files {
		if file == copilotRoot || strings.HasPrefix(file, copilotRoot+string(os.PathSeparator)) {
			t.Fatalf("legacy both target must not report a Copilot artifact, got %#v", result.Files)
		}
	}

	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatalf("read user-facing documentation: %v", err)
	}
	if !strings.Contains(string(readme), "`both` (legacy compatibility input)") || !strings.Contains(string(readme), "Claude Code and OpenCode") {
		t.Fatalf("expected user-facing documentation to identify both as the legacy Claude Code and OpenCode compatibility input")
	}
}

// REQ-100 → SCN-405 → TestSCN405_RejectsUnsupportedTargetsBeforeConfigurationChanges
func TestSCN405_RejectsUnsupportedTargetsBeforeConfigurationChanges(t *testing.T) {
	// Scenario: Reject an unknown target before changing user configuration
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	t.Setenv("HOME", home)

	unchanged := map[string]string{
		filepath.Join(home, ".claude", "settings.json"):               "claude settings",
		filepath.Join(home, ".config", "opencode", "instructions.md"): "opencode instructions",
		filepath.Join(home, ".codex", "AGENTS.md"):                    "codex guidance",
	}
	for path, content := range unchanged {
		writeTestFile(t, path, []byte(content))
	}

	for _, target := range []string{"copilot-vscode", "unsupported-target"} {
		t.Run(target, func(t *testing.T) {
			result, err := Install(Options{Target: target, ProjectPath: projectPath})
			if err == nil {
				t.Fatal("expected unsupported target to be rejected")
			}
			if result != nil {
				t.Fatalf("expected no installation result for unsupported target, got %#v", result)
			}
			for path, want := range unchanged {
				got, readErr := os.ReadFile(path)
				if readErr != nil {
					t.Fatalf("read unchanged host configuration %s: %v", path, readErr)
				}
				if string(got) != want {
					t.Fatalf("expected %s to remain unchanged, got %q", path, got)
				}
			}
			assertPathMissing(t, filepath.Join(home, ".rotta", "backups"))
			assertPathMissing(t, filepath.Join(projectPath, ".rotta"))
			for _, host := range []string{"Claude Code", "OpenCode", "Codex", "Copilot CLI"} {
				if !strings.Contains(err.Error(), host) {
					t.Fatalf("expected unsupported-target error to list %q, got %q", host, err)
				}
			}
		})
	}
}

// REQ-101 → SCN-406 → TestSCN406_GeneratesGlobalCopilotRoleDefinitionsForSelectedModes
func TestSCN406_GeneratesGlobalCopilotRoleDefinitionsForSelectedModes(t *testing.T) {
	// Scenario: Generate global Copilot Markdown role definitions for selected Rotta modes
	for _, test := range []struct {
		name          string
		installSpec   bool
		installImpl   bool
		installReview bool
		roles         []string
	}{
		{
			name:          "all phases",
			installSpec:   true,
			installImpl:   true,
			installReview: true,
			roles:         []string{"rotta-orchestrator", "rotta-spec", "rotta-impl", "rotta-review"},
		},
		{
			name:        "review omitted",
			installSpec: true,
			installImpl: true,
			roles:       []string{"rotta-orchestrator", "rotta-spec", "rotta-impl"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			root := filepath.Join(home, "active-copilot-root")
			t.Setenv("HOME", home)
			t.Setenv("COPILOT_HOME", root)

			result, err := Install(Options{
				Target:        "copilot-cli",
				ProjectPath:   filepath.Join(home, "project"),
				InstallSpec:   test.installSpec,
				InstallImpl:   test.installImpl,
				InstallReview: test.installReview,
			})
			if err != nil {
				t.Fatal(err)
			}

			for _, role := range test.roles {
				assertCopilotAgentFixture(t, filepath.Join(root, "agents", role+".agent.md"), role)
			}
			if !test.installReview {
				assertPathMissing(t, filepath.Join(root, "agents", "rotta-review.agent.md"))
			}
			assertFileContains(t, filepath.Join(root, "instructions", "rotta.instructions.md"), "Rotta")

			capability := result.Hosts["copilot-cli"].Capabilities["agents"]
			if capability.Status != HostCapabilityStatusDegraded || capability.Reason == "" || capability.Remediation == "" {
				t.Fatalf("expected unavailable Copilot CLI role acceptance proof to be degraded with remediation, got %#v", capability)
			}
		})
	}
}

// REQ-101 → REQ-104 → SCN-407 → TestSCN407_RoutesCopilotPhaseRequestsThroughAdaptedOrchestration
func TestSCN407_RoutesCopilotPhaseRequestsThroughAdaptedOrchestration(t *testing.T) {
	// Scenario: Route Copilot phase requests through adapted orchestration
	home := t.TempDir()
	root := filepath.Join(home, "active-copilot-root")
	t.Setenv("HOME", home)
	t.Setenv("COPILOT_HOME", root)

	if _, err := Install(Options{Target: "copilot-cli", ProjectPath: filepath.Join(home, "project")}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(root, "instructions", "rotta.instructions.md"))
	if err != nil {
		t.Fatalf("read Copilot instructions: %v", err)
	}
	assertContainsAll(t, string(data), []string{
		"/agent rotta-orchestrator",
		"copilot --agent rotta-orchestrator",
		"Rotta-Orchestrator decision point",
		"specs/", "features/", ".rotta/",
		"Phase 1 — Draft", "explicit human approval", "strict Red/Green/Refactor TDD", "Phase 4 — Review", "final_human_review",
		"Copilot role-agent and command support is adapted",
		"not host-native hidden subagent delegation",
	})
}

// REQ-101 → REQ-105 → SCN-408 → TestSCN408_KeepsCopilotIntegrationGlobalAndOutOfRepositories
func TestSCN408_KeepsCopilotIntegrationGlobalAndOutOfRepositories(t *testing.T) {
	// Scenario: Keep Copilot integration global and out of repositories
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	root := filepath.Join(home, "active-copilot-root")
	t.Setenv("HOME", home)
	t.Setenv("COPILOT_HOME", root)
	writeTestFile(t, filepath.Join(projectPath, "go.mod"), []byte("module project\n"))

	if _, err := Install(Options{Target: "copilot-cli", ProjectPath: projectPath}); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{
		filepath.Join(projectPath, ".github"),
		filepath.Join(projectPath, ".mcp.json"),
		filepath.Join(projectPath, "AGENTS.md"),
		filepath.Join(projectPath, "CLAUDE.md"),
	} {
		assertPathMissing(t, path)
	}

	instructions, err := os.ReadFile(filepath.Join(root, "instructions", "rotta.instructions.md"))
	if err != nil {
		t.Fatalf("read global Copilot instructions: %v", err)
	}
	if !strings.Contains(string(instructions), "Copilot integration is global-only") {
		t.Fatal("expected global Copilot instructions to identify the integration as global-only")
	}
}

// REQ-102 → SCN-409 → TestSCN409_RegistersSelectedMCPsAtResolvedFixturePath
func TestSCN409_RegistersSelectedMCPsAtResolvedFixturePath(t *testing.T) {
	// Scenario: Register selected MCPs through a validated active-root fixture
	home := t.TempDir()
	root := filepath.Join(home, "active-copilot-root")
	mcpPath := filepath.Join(root, "mcp-config.json")
	t.Setenv("HOME", home)
	t.Setenv("COPILOT_HOME", root)
	t.Setenv("COPILOT_MCP_CONFIG", mcpPath)
	writeTestFile(t, mcpPath, []byte(`{"userSetting":"preserve"}`))

	result, err := Install(Options{
		Target:        "copilot-cli",
		ProjectPath:   filepath.Join(home, "project"),
		SetupAncora:   true,
		SetupVela:     true,
		SetupContext7: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("read resolved Copilot MCP fixture: %v", err)
	}
	var config struct {
		UserSetting string `json:"userSetting"`
		MCPServers  map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("parse resolved Copilot MCP fixture: %v", err)
	}
	if config.UserSetting != "preserve" {
		t.Fatalf("expected unrelated Copilot configuration to be preserved, got %#v", config)
	}
	want := map[string]struct {
		command string
		args    []string
	}{
		"ancora":   {command: "ancora", args: []string{"mcp"}},
		"vela":     {command: "vela", args: []string{"mcp"}},
		"context7": {command: "npx", args: []string{"-y", "@upstash/context7-mcp"}},
	}
	if len(config.MCPServers) != len(want) {
		t.Fatalf("expected only selected Copilot MCP registrations, got %#v", config.MCPServers)
	}
	for name, expected := range want {
		server, ok := config.MCPServers[name]
		if !ok || server.Command != expected.command || !sameArguments(server.Args, expected.args) {
			t.Fatalf("expected %s registration %#v, got %#v", name, expected, server)
		}
	}
	reported := false
	for _, path := range result.Files {
		if path == mcpPath {
			reported = true
			break
		}
	}
	if !reported {
		t.Fatalf("expected result to report resolved Copilot MCP path %q, got %#v", mcpPath, result.Files)
	}
	if result.CopilotGlobalConfigRoot != root || result.CopilotMCPConfigPath != mcpPath {
		t.Fatalf("expected result to report resolved Copilot root/path %q and %q, got %#v", root, mcpPath, result)
	}
	capability := result.Hosts["copilot-cli"].Capabilities["mcp"]
	if capability.Status != HostCapabilityStatusDegraded || !strings.Contains(capability.Reason, "not a complete universal Copilot MCP schema") {
		t.Fatalf("expected the fixture report to avoid a universal-schema or runtime-health claim, got %#v", capability)
	}
	for _, path := range []string{
		filepath.Join(home, ".claude", "mcp", "context7.json"),
		filepath.Join(home, ".config", "opencode", "opencode.json"),
		filepath.Join(home, "project", ".vela", "graph.db"),
	} {
		assertPathMissing(t, path)
	}
}

// REQ-102 → SCN-410 → TestSCN410_ReportsExactCopilotMCPHealthFromCapturedDiagnostics
func TestSCN410_ReportsExactCopilotMCPHealthFromCapturedDiagnostics(t *testing.T) {
	// Scenario: Report exact Copilot MCP health only from documented host evidence
	home := t.TempDir()
	root := filepath.Join(home, "active-copilot-root")
	mcpPath := filepath.Join(root, "mcp-config.json")
	t.Setenv("HOME", home)
	t.Setenv("COPILOT_HOME", root)
	t.Setenv("COPILOT_MCP_CONFIG", mcpPath)

	evidence := CopilotMCPHealthEvidence{
		ConfigurationAccepted:    true,
		VersionOutput:            "copilot 1.2.3",
		MCPListOutput:            "ancora\nvela\ncontext7",
		InteractiveMCPListOutput: "ancora healthy\nvela healthy\ncontext7 healthy",
		InteractiveMCPShowOutputs: map[string]CopilotMCPServerEvidence{
			"ancora":   {Output: "ancora healthy", Healthy: true},
			"vela":     {Output: "vela healthy", Healthy: true},
			"context7": {Output: "context7 healthy", Healthy: true},
		},
	}
	result, err := Install(Options{
		Target:                   "copilot-cli",
		ProjectPath:              filepath.Join(home, "project"),
		SetupAncora:              true,
		SetupVela:                true,
		SetupContext7:            true,
		CopilotMCPHealthEvidence: evidence,
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.CopilotGlobalConfigRoot != root || result.CopilotMCPConfigPath != mcpPath {
		t.Fatalf("expected resolved Copilot root/path %q and %q, got %#v", root, mcpPath, result)
	}
	if result.CopilotMCPHealthEvidence.ConfigurationAccepted != evidence.ConfigurationAccepted ||
		result.CopilotMCPHealthEvidence.VersionOutput != evidence.VersionOutput ||
		result.CopilotMCPHealthEvidence.MCPListOutput != evidence.MCPListOutput ||
		result.CopilotMCPHealthEvidence.InteractiveMCPListOutput != evidence.InteractiveMCPListOutput {
		t.Fatalf("expected captured Copilot MCP evidence to be retained, got %#v", result.CopilotMCPHealthEvidence)
	}
	for _, name := range []string{"ancora", "vela", "context7"} {
		if result.CopilotMCPHealthEvidence.InteractiveMCPShowOutputs[name] != evidence.InteractiveMCPShowOutputs[name] {
			t.Fatalf("expected %s diagnostic evidence to be retained, got %#v", name, result.CopilotMCPHealthEvidence.InteractiveMCPShowOutputs[name])
		}
		capability := result.Hosts["copilot-cli"].Capabilities["mcp:"+name]
		if capability.Status != HostCapabilityStatusExact {
			t.Fatalf("expected documented diagnostic proof to report %s exact, got %#v", name, capability)
		}
		status := result.MCPStatuses["copilot-cli"][name]
		if status.Status != MCPStatusConfigured || status.RuntimeFallback.State != MCPRuntimeFallbackNotObserved {
			t.Fatalf("expected %s configuration to remain distinct from later runtime fallback, got %#v", name, status)
		}
	}
	if result.Hosts["copilot-cli"].Capabilities["mcp"].Status != HostCapabilityStatusExact {
		t.Fatalf("expected exact aggregate Copilot MCP health from captured diagnostics, got %#v", result.Hosts["copilot-cli"].Capabilities["mcp"])
	}
}

// REQ-102 → REQ-104 → SCN-411 → TestSCN411_DegradesOrFailsCopilotMCPWhenProofIsMissing
func TestSCN411_DegradesOrFailsCopilotMCPWhenProofIsMissing(t *testing.T) {
	// Scenario: Degrade rather than infer Copilot MCP configuration or health
	for _, test := range []struct {
		name            string
		evidence        CopilotMCPHealthEvidence
		proofFailure    CopilotMCPProofFailure
		affectedServers []string
		status          HostCapabilityStatus
		reason          string
		remediation     string
	}{
		{
			name:            "active root or MCP path cannot be resolved safely",
			evidence:        copilotHealthyMCPEvidence(),
			proofFailure:    CopilotMCPProofFailureRootOrPathUnresolved,
			affectedServers: []string{"ancora", "vela", "context7"},
			status:          HostCapabilityStatusFailed,
			reason:          "active global configuration root or MCP path",
			remediation:     "Resolve the active global configuration root and MCP path safely",
		},
		{
			name:            "fixture validation fails",
			evidence:        copilotHealthyMCPEvidence(),
			proofFailure:    CopilotMCPProofFailureFixtureValidationFailed,
			affectedServers: []string{"ancora", "vela", "context7"},
			status:          HostCapabilityStatusDegraded,
			reason:          "interoperability fixture was not accepted",
			remediation:     "Capture current Copilot CLI fixture acceptance",
		},
		{
			name:            "version proof is missing",
			evidence:        CopilotMCPHealthEvidence{ConfigurationAccepted: true},
			affectedServers: []string{"ancora", "vela", "context7"},
			status:          HostCapabilityStatusDegraded,
			reason:          "copilot --version output is missing",
			remediation:     "Capture successful copilot --version output",
		},
		{
			name: "list or show diagnostics are missing",
			evidence: CopilotMCPHealthEvidence{
				ConfigurationAccepted: true,
				VersionOutput:         "copilot 1.2.3",
			},
			affectedServers: []string{"ancora", "vela", "context7"},
			status:          HostCapabilityStatusDegraded,
			reason:          "MCP list or show diagnostics are missing",
			remediation:     "Capture copilot mcp list and interactive /mcp list and /mcp show diagnostics",
		},
		{
			name: "server is unavailable",
			evidence: CopilotMCPHealthEvidence{
				ConfigurationAccepted:    true,
				VersionOutput:            "copilot 1.2.3",
				MCPListOutput:            "ancora\nvela\ncontext7",
				InteractiveMCPListOutput: "ancora healthy\nvela unavailable\ncontext7 healthy",
				InteractiveMCPShowOutputs: map[string]CopilotMCPServerEvidence{
					"ancora":   {Output: "ancora healthy", Healthy: true},
					"vela":     {Output: "vela unavailable", Failure: CopilotMCPProofFailureServerUnavailable},
					"context7": {Output: "context7 healthy", Healthy: true},
				},
			},
			affectedServers: []string{"vela"},
			status:          HostCapabilityStatusFailed,
			reason:          "server is unavailable",
			remediation:     "Make the server available",
		},
		{
			name: "MCP command fails to start",
			evidence: CopilotMCPHealthEvidence{
				ConfigurationAccepted:    true,
				VersionOutput:            "copilot 1.2.3",
				MCPListOutput:            "ancora\nvela\ncontext7",
				InteractiveMCPListOutput: "ancora healthy\nvela command failed\ncontext7 healthy",
				InteractiveMCPShowOutputs: map[string]CopilotMCPServerEvidence{
					"ancora":   {Output: "ancora healthy", Healthy: true},
					"vela":     {Output: "vela command failed", Failure: CopilotMCPProofFailureCommandFailed},
					"context7": {Output: "context7 healthy", Healthy: true},
				},
			},
			affectedServers: []string{"vela"},
			status:          HostCapabilityStatusFailed,
			reason:          "configured MCP command could not start",
			remediation:     "Repair the configured MCP command",
		},
		{
			name: "MCP initialization times out",
			evidence: CopilotMCPHealthEvidence{
				ConfigurationAccepted:    true,
				VersionOutput:            "copilot 1.2.3",
				MCPListOutput:            "ancora\nvela\ncontext7",
				InteractiveMCPListOutput: "ancora healthy\nvela timeout\ncontext7 healthy",
				InteractiveMCPShowOutputs: map[string]CopilotMCPServerEvidence{
					"ancora":   {Output: "ancora healthy", Healthy: true},
					"vela":     {Output: "vela timeout", Failure: CopilotMCPProofFailureInitializationTimeout},
					"context7": {Output: "context7 healthy", Healthy: true},
				},
			},
			affectedServers: []string{"vela"},
			status:          HostCapabilityStatusFailed,
			reason:          "MCP initialization or tool discovery timed out",
			remediation:     "Capture completed MCP initialization and tool-discovery evidence",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			root := filepath.Join(home, "active-copilot-root")
			t.Setenv("HOME", home)
			t.Setenv("COPILOT_HOME", root)
			t.Setenv("COPILOT_MCP_CONFIG", filepath.Join(root, "mcp-config.json"))
			test.evidence.ProofFailure = test.proofFailure

			result, err := Install(Options{
				Target:                   "copilot-cli",
				ProjectPath:              filepath.Join(home, "project"),
				SetupAncora:              true,
				SetupVela:                true,
				SetupContext7:            true,
				CopilotMCPHealthEvidence: test.evidence,
			})
			if err != nil {
				t.Fatal(err)
			}

			host := result.Hosts["copilot-cli"]
			for _, server := range test.affectedServers {
				capability := host.Capabilities["mcp:"+server]
				if capability.Status != test.status || capability.Status == HostCapabilityStatusExact {
					t.Fatalf("expected %s to be %s rather than exact, got %#v", server, test.status, capability)
				}
				if !strings.Contains(capability.Reason, test.reason) || !strings.Contains(capability.Remediation, test.remediation) {
					t.Fatalf("expected specific proof gap %q and remediation %q, got %#v", test.reason, test.remediation, capability)
				}
				if status := result.MCPStatuses["copilot-cli"][server]; status.Status == MCPStatusConfigured {
					t.Fatalf("expected %s MCP status not to be configured from missing or failed proof, got %#v", server, status)
				}
			}
			if host.Capabilities["lifecycle"].Status != HostCapabilityStatusExact {
				t.Fatalf("expected canonical workflow gates to remain exact, got %#v", host.Capabilities["lifecycle"])
			}
		})
	}
}

func copilotHealthyMCPEvidence() CopilotMCPHealthEvidence {
	return CopilotMCPHealthEvidence{
		ConfigurationAccepted:    true,
		VersionOutput:            "copilot 1.2.3",
		MCPListOutput:            "ancora\nvela\ncontext7",
		InteractiveMCPListOutput: "ancora healthy\nvela healthy\ncontext7 healthy",
		InteractiveMCPShowOutputs: map[string]CopilotMCPServerEvidence{
			"ancora":   {Output: "ancora healthy", Healthy: true},
			"vela":     {Output: "vela healthy", Healthy: true},
			"context7": {Output: "context7 healthy", Healthy: true},
		},
	}
}

func assertCopilotAgentFixture(t *testing.T, path, role string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read Copilot agent %s: %v", role, err)
	}
	content := string(data)
	if !strings.HasPrefix(content, "---\n") || !strings.Contains(content, "\n---\n") {
		t.Fatalf("expected Copilot agent %s to have YAML frontmatter, got %q", role, content)
	}
	assertContainsAll(t, content, []string{"name: " + role, "Rotta"})
}
