package installer

import (
	"encoding/json"
	"errors"
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

// REQ-102 → REQ-103 → SCN-412 → TestSCN412_RerunPreservesUnrelatedCopilotConfiguration
func TestSCN412_RerunPreservesUnrelatedCopilotConfiguration(t *testing.T) {
	// Scenario: Rerun Copilot installation without changing unrelated configuration
	home := t.TempDir()
	root := filepath.Join(home, "active-copilot-root")
	mcpPath := filepath.Join(root, "mcp-config.json")
	t.Setenv("HOME", home)
	t.Setenv("COPILOT_HOME", root)
	t.Setenv("COPILOT_MCP_CONFIG", mcpPath)
	writeTestFile(t, mcpPath, []byte(`{"unrelatedLargeSetting":9007199254740993,"mcpServers":{"user-server":{"command":"user-mcp","args":["serve"]}}}`))
	writeTestFile(t, filepath.Join(root, "agents", "user.agent.md"), []byte("user agent"))
	writeTestFile(t, filepath.Join(root, "instructions", "user.instructions.md"), []byte("user instructions"))

	opts := Options{
		Target:        "copilot-cli",
		ProjectPath:   filepath.Join(home, "project"),
		InstallSpec:   true,
		InstallImpl:   true,
		InstallReview: true,
		SetupAncora:   true,
		SetupVela:     true,
		SetupContext7: true,
	}
	if _, err := Install(opts); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(opts); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("read rerun Copilot MCP configuration: %v", err)
	}
	var config map[string]json.RawMessage
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("parse rerun Copilot MCP configuration: %v", err)
	}
	if got := string(config["unrelatedLargeSetting"]); got != "9007199254740993" {
		t.Fatalf("expected unrelated root setting to be preserved, got %s", got)
	}
	var servers map[string]copilotMCPServer
	if err := json.Unmarshal(config["mcpServers"], &servers); err != nil {
		t.Fatalf("parse rerun Copilot MCP registrations: %v", err)
	}
	if len(servers) != 4 || servers["user-server"].Command != "user-mcp" {
		t.Fatalf("expected selected and unrelated MCP registrations to be preserved once, got %#v", servers)
	}
	for _, name := range []string{"ancora", "vela", "context7"} {
		if _, ok := servers[name]; !ok {
			t.Fatalf("expected one managed %s MCP registration, got %#v", name, servers)
		}
	}
	for _, name := range []string{"rotta-orchestrator", "rotta-spec", "rotta-impl", "rotta-review", "user"} {
		if _, err := os.Stat(filepath.Join(root, "agents", name+".agent.md")); err != nil {
			t.Fatalf("expected agent artifact %s to be preserved once: %v", name, err)
		}
	}
	for _, name := range []string{"rotta.instructions.md", "user.instructions.md"} {
		if _, err := os.Stat(filepath.Join(root, "instructions", name)); err != nil {
			t.Fatalf("expected instruction artifact %s to be preserved: %v", name, err)
		}
	}
}

// REQ-102 → REQ-103 → SCN-413 → TestSCN413_RefusesUnsafeCopilotMCPConfigurationMutation
func TestSCN413_RefusesUnsafeCopilotMCPConfigurationMutation(t *testing.T) {
	// Scenario: Refuse unsafe active-root MCP configuration mutation
	for _, test := range []struct {
		name     string
		config   string
		blocking string
	}{
		{name: "malformed JSON", config: `{"mcpServers":`, blocking: "malformed JSON"},
		{name: "non-object mcpServers", config: `{"mcpServers":[]}`, blocking: "mcpServers must be an object"},
		{name: "incompatible shape", config: `[]`, blocking: "incompatible configuration shape"},
		{name: "unproven managed entry", config: `{"mcpServers":{"ancora":{"command":"other-mcp","args":["serve"]}}}`, blocking: "not proven Rotta-managed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			root := filepath.Join(home, "active-copilot-root")
			mcpPath := filepath.Join(root, "mcp-config.json")
			t.Setenv("HOME", home)
			t.Setenv("COPILOT_HOME", root)
			t.Setenv("COPILOT_MCP_CONFIG", mcpPath)
			writeTestFile(t, mcpPath, []byte(test.config))

			result, err := Install(Options{
				Target:        "copilot-cli",
				ProjectPath:   filepath.Join(home, "project"),
				SetupAncora:   true,
				SetupVela:     true,
				SetupContext7: true,
			})
			if err == nil {
				t.Fatal("expected unsafe Copilot MCP configuration to be refused")
			}
			if result == nil {
				t.Fatal("expected the refusal result to report the resolved Copilot configuration path")
			}
			if result.CopilotGlobalConfigRoot != root || result.CopilotMCPConfigPath != mcpPath {
				t.Fatalf("expected resolved Copilot root/path %q and %q, got %#v", root, mcpPath, result)
			}
			if !strings.Contains(result.Error, test.blocking) {
				t.Fatalf("expected blocking condition %q, got %q", test.blocking, result.Error)
			}
			if host := result.Hosts["copilot-cli"]; host.Status != HostInstallStatusFailed || host.Capabilities["mcp"].Status == HostCapabilityStatusExact {
				t.Fatalf("expected no successful Copilot MCP configuration, got %#v", host)
			}
			data, readErr := os.ReadFile(mcpPath)
			if readErr != nil {
				t.Fatalf("read unchanged Copilot MCP configuration: %v", readErr)
			}
			if string(data) != test.config {
				t.Fatalf("expected unsafe Copilot MCP configuration to remain unchanged, got %q", data)
			}
		})
	}
}

// REQ-103 → SCN-414 → TestSCN414_RecoversSafelyAfterPartialCopilotGlobalWrite
func TestSCN414_RecoversSafelyAfterPartialCopilotGlobalWrite(t *testing.T) {
	// Scenario: Recover safely after a partial Copilot global write
	home := t.TempDir()
	root := filepath.Join(home, "active-copilot-root")
	completedArtifact := filepath.Join(root, "agents", "rotta-orchestrator.agent.md")
	failingArtifact := filepath.Join(root, "instructions", "rotta.instructions.md")
	t.Setenv("HOME", home)
	t.Setenv("COPILOT_HOME", root)
	writeTestFile(t, completedArtifact, []byte("prior valid agent\n"))
	writeTestFile(t, failingArtifact, []byte("prior valid instructions\n"))

	result, err := Install(Options{
		Target:      "copilot-cli",
		ProjectPath: filepath.Join(home, "project"),
		CopilotManagedFileWriter: func(path string, data []byte, perm os.FileMode) error {
			if path == failingArtifact {
				return errors.New("injected managed-file write failure")
			}
			return writePrivateFile(path, data, perm)
		},
	})
	if err == nil {
		t.Fatal("expected injected Copilot managed-file write to fail")
	}
	if result == nil {
		t.Fatal("expected failed Copilot installation result")
	}
	if got, readErr := os.ReadFile(failingArtifact); readErr != nil || string(got) != "prior valid instructions\n" {
		t.Fatalf("expected failing artifact to retain prior valid content, got %q (read error: %v)", got, readErr)
	}
	if !containsString(result.Files, completedArtifact) {
		t.Fatalf("expected result to identify completed Copilot work %q, got %#v", completedArtifact, result.Files)
	}
	if result.CopilotGlobalConfigRoot != root || !strings.Contains(result.Error, failingArtifact) || result.BackupDir == "" {
		t.Fatalf("expected result to identify resolved Copilot artifact, failure, and backup location, got %#v", result)
	}
	if result.Hosts["copilot-cli"].Status != HostInstallStatusFailed {
		t.Fatalf("expected failed Copilot host rather than success, got %#v", result.Hosts["copilot-cli"])
	}

	manifest, manifestErr := loadBackupManifest(filepath.Join(result.BackupDir, "manifest.json"))
	if manifestErr != nil {
		t.Fatalf("read Copilot installation backup: %v", manifestErr)
	}
	for _, path := range []string{completedArtifact, failingArtifact} {
		if !containsString(manifest.BackedUpPaths, path) {
			t.Fatalf("expected eligible Copilot artifact %q to be backed up, got %#v", path, manifest.BackedUpPaths)
		}
	}
}

// REQ-103 → SCN-415 → TestSCN415_RestoresCompletedCopilotBackupAfterRootChanges
func TestSCN415_RestoresCompletedCopilotBackupAfterRootChanges(t *testing.T) {
	// Scenario: Restore Copilot configuration from a completed backup
	home := t.TempDir()
	previousRoot := filepath.Join(home, "previous-copilot-root")
	currentRoot := filepath.Join(home, "current-copilot-root")
	selectedBackupDir := filepath.Join(home, ".rotta", "backups", "20260725T234000Z")
	orchestratorPath := filepath.Join(previousRoot, "agents", "rotta-orchestrator.agent.md")
	instructionsPath := filepath.Join(previousRoot, "instructions", "rotta.instructions.md")
	missingSpecPath := filepath.Join(previousRoot, "agents", "rotta-spec.agent.md")
	mcpPath := filepath.Join(previousRoot, "mcp-config.json")
	t.Setenv("HOME", home)
	t.Setenv("COPILOT_HOME", currentRoot)
	t.Setenv("COPILOT_MCP_CONFIG", mcpPath)

	writeTestFile(t, backupDestination(selectedBackupDir, home, orchestratorPath), []byte("backup orchestrator\n"))
	writeTestFile(t, backupDestination(selectedBackupDir, home, instructionsPath), []byte("backup instructions\n"))
	writeTestFile(t, backupDestination(selectedBackupDir, home, mcpPath), []byte(`{"mcpServers":{"ancora":{"command":"ancora","args":["mcp"]}}}`))
	writeTestFile(t, filepath.Join(selectedBackupDir, "manifest.json"), []byte(`{"version":1,"timestamp":"20260725T234000Z","project_path":"`+filepath.Join(home, "project")+`","target":"copilot-cli","selected_modes":{"spec":true,"implementation":false,"review":false},"optional_integrations":{"ancora":true,"vela":false,"context7":false},"backed_up_paths":["`+orchestratorPath+`","`+instructionsPath+`","`+mcpPath+`"],"missing_paths":["`+missingSpecPath+`"],"status":"complete"}`))

	writeTestFile(t, orchestratorPath, []byte("current orchestrator\n"))
	writeTestFile(t, instructionsPath, []byte("current instructions\n"))
	writeTestFile(t, mcpPath, []byte(`{"mcpServers":{"ancora":{"command":"changed","args":[]}}}`))
	writeTestFile(t, missingSpecPath, []byte("remove only because backup recorded it absent\n"))
	writeTestFile(t, filepath.Join(previousRoot, "agents", "user.agent.md"), []byte("preserve user agent\n"))

	result, err := RestoreBackup(selectedBackupDir)
	if err != nil {
		t.Fatal(err)
	}
	assertFileContains(t, orchestratorPath, "backup orchestrator")
	assertFileContains(t, instructionsPath, "backup instructions")
	assertFileContains(t, mcpPath, `"command":"ancora"`)
	assertPathMissing(t, missingSpecPath)
	assertFileContains(t, filepath.Join(previousRoot, "agents", "user.agent.md"), "preserve user agent")
	assertFileContains(t, backupDestination(result.PreRestoreBackupDir, home, orchestratorPath), "current orchestrator")
	assertFileContains(t, backupDestination(result.PreRestoreBackupDir, home, instructionsPath), "current instructions")

	writeTestFile(t, orchestratorPath, []byte("pre-failed-restore orchestrator\n"))
	failedResult, restoreErr := restoreBackupWithHooks(selectedBackupDir, restoreHooks{
		afterRestorePath: func(path string) error {
			if path == orchestratorPath {
				return errors.New("injected restore failure")
			}
			return nil
		},
	})
	if restoreErr == nil || failedResult == nil || failedResult.PreRestoreBackupDir == "" {
		t.Fatalf("expected selected restore failure with a pre-restore recovery backup, got result %#v and error %v", failedResult, restoreErr)
	}
	for _, expected := range []string{selectedBackupDir, "rollback to pre-restore state succeeded"} {
		if !strings.Contains(restoreErr.Error(), expected) {
			t.Fatalf("expected restore failure to report %q, got %v", expected, restoreErr)
		}
	}
	assertFileContains(t, orchestratorPath, "pre-failed-restore orchestrator")
}

// REQ-104 → SCN-416 → TestSCN416_AccountsForCopilotGlobalPathsAsHostConfiguration
func TestSCN416_AccountsForCopilotGlobalPathsAsHostConfiguration(t *testing.T) {
	// Scenario: Account for Copilot changes separately from workspace lifecycle artifacts
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	root := filepath.Join(projectPath, ".copilot-global")
	mcpPath := filepath.Join(root, "mcp-config.json")
	t.Setenv("HOME", home)
	t.Setenv("COPILOT_HOME", root)
	t.Setenv("COPILOT_MCP_CONFIG", mcpPath)

	result, err := Install(Options{
		Target:        "copilot-cli",
		ProjectPath:   projectPath,
		SetupAncora:   true,
		InstallSpec:   true,
		InstallImpl:   true,
		InstallReview: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	copilotPaths := []string{
		filepath.Join(root, "agents", "rotta-orchestrator.agent.md"),
		filepath.Join(root, "agents", "rotta-spec.agent.md"),
		filepath.Join(root, "agents", "rotta-impl.agent.md"),
		filepath.Join(root, "agents", "rotta-review.agent.md"),
		filepath.Join(root, "instructions", "rotta.instructions.md"),
		mcpPath,
	}
	for _, path := range copilotPaths {
		if countString(result.Hosts["copilot-cli"].Files, path) != 1 {
			t.Fatalf("expected Copilot host result to report %q exactly once, got %#v", path, result.Hosts["copilot-cli"].Files)
		}
		if countString(result.Files, path) != 1 {
			t.Fatalf("expected result files to report %q exactly once, got %#v", path, result.Files)
		}
		if countString(result.ChangedFiles[FileChangeCategoryHostConfig], path) != 1 {
			t.Fatalf("expected %q to be host configuration exactly once, got %#v", path, result.ChangedFiles)
		}
		for _, category := range []FileChangeCategory{FileChangeCategoryWorkspaceHostConfig, FileChangeCategoryLifecycle} {
			if containsString(result.ChangedFiles[category], path) {
				t.Fatalf("Copilot global path %q must not be classified as %s, got %#v", path, category, result.ChangedFiles)
			}
		}
	}
}

// REQ-104 → SCN-417 → TestSCN417_PreservesCanonicalLifecycleAuthorityInCopilotGuidance
func TestSCN417_PreservesCanonicalLifecycleAuthorityInCopilotGuidance(t *testing.T) {
	// Scenario: Preserve canonical lifecycle authority in Copilot guidance
	home := t.TempDir()
	root := filepath.Join(home, "active-copilot-root")
	t.Setenv("HOME", home)
	t.Setenv("COPILOT_HOME", root)

	if _, err := Install(Options{
		Target:        "copilot-cli",
		ProjectPath:   filepath.Join(home, "project"),
		InstallSpec:   true,
		InstallImpl:   true,
		InstallReview: true,
		SetupAncora:   true,
	}); err != nil {
		t.Fatal(err)
	}

	instructions, err := os.ReadFile(filepath.Join(root, "instructions", "rotta.instructions.md"))
	if err != nil {
		t.Fatalf("read Copilot lifecycle guidance: %v", err)
	}
	assertContainsAll(t, string(instructions), []string{
		"workspace specs, features, and .rotta artifacts are the durable source of truth",
		"Copilot configuration, MCP state, and Ancora memory are not approval or lifecycle authority",
		"direct phase roles must not advance approval, baseline, checkpoint, review, or completion state",
	})
}

// REQ-105 → SCN-418 → TestSCN418_DescribesSupportedHostsAndCopilotBoundaries
func TestSCN418_DescribesSupportedHostsAndCopilotBoundaries(t *testing.T) {
	// Scenario: Describe all actual supported hosts and corrected Copilot boundaries
	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatalf("read user-facing documentation: %v", err)
	}

	assertContainsAll(t, string(readme), []string{
		"Claude Code", "OpenCode", "Codex", "GitHub Copilot CLI",
		"All supported hosts", "resolved active global configuration root", ".agent.md",
		"adapted orchestration", "global-only MCP scope", "offline-safe verification",
		"VS Code", "JetBrains", "repository-local", "`.github`", "`.mcp.json`",
	})
}

// REQ-105 → SCN-419 → TestSCN419_RecordsTimeBoundCopilotCompatibilityStatus
func TestSCN419_RecordsTimeBoundCopilotCompatibilityStatus(t *testing.T) {
	// Scenario: Record time-bound verification without making installation online-dependent
	home := t.TempDir()
	root := filepath.Join(home, "active-copilot-root")
	t.Setenv("HOME", home)
	t.Setenv("COPILOT_HOME", root)
	t.Setenv("COPILOT_MCP_CONFIG", filepath.Join(root, "mcp-config.json"))

	evidence := copilotHealthyMCPEvidence()
	verification := CopilotCompatibilityVerification{
		OfficialReleaseIdentity: "github/copilot-cli v1.2.3",
		OfficialReleaseSource:   "https://github.com/github/copilot-cli/releases/tag/v1.2.3",
		VerifiedAt:              "2026-07-25T00:00:00Z",
	}
	result, err := Install(Options{
		Target:                           "copilot-cli",
		ProjectPath:                      filepath.Join(home, "project"),
		SetupAncora:                      true,
		SetupVela:                        true,
		SetupContext7:                    true,
		CopilotMCPHealthEvidence:         evidence,
		CopilotCompatibilityVerification: verification,
	})
	if err != nil {
		t.Fatal(err)
	}

	status := result.CopilotCompatibilityStatus
	if status.Status != HostCapabilityStatusExact ||
		status.OfficialReleaseIdentity != verification.OfficialReleaseIdentity ||
		status.OfficialReleaseSource != verification.OfficialReleaseSource ||
		status.VerifiedAt != verification.VerifiedAt ||
		status.VersionOutput != evidence.VersionOutput ||
		status.MCPDiagnostics.InteractiveMCPShowOutputs["ancora"] != evidence.InteractiveMCPShowOutputs["ancora"] {
		t.Fatalf("expected time-bound release, version, and diagnostic verification status, got %#v", status)
	}

	for _, test := range []struct {
		name     string
		evidence CopilotMCPHealthEvidence
	}{
		{name: "missing version", evidence: CopilotMCPHealthEvidence{ConfigurationAccepted: true}},
		{name: "fixture not accepted", evidence: CopilotMCPHealthEvidence{
			VersionOutput:             evidence.VersionOutput,
			MCPListOutput:             evidence.MCPListOutput,
			InteractiveMCPListOutput:  evidence.InteractiveMCPListOutput,
			InteractiveMCPShowOutputs: evidence.InteractiveMCPShowOutputs,
		}},
		{name: "runtime proof failure", evidence: CopilotMCPHealthEvidence{
			ConfigurationAccepted:     true,
			VersionOutput:             evidence.VersionOutput,
			MCPListOutput:             evidence.MCPListOutput,
			InteractiveMCPListOutput:  evidence.InteractiveMCPListOutput,
			InteractiveMCPShowOutputs: evidence.InteractiveMCPShowOutputs,
			ProofFailure:              CopilotMCPProofFailureInitializationTimeout,
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := Install(Options{
				Target:                           "copilot-cli",
				ProjectPath:                      filepath.Join(home, "project-"+test.name),
				SetupAncora:                      true,
				SetupVela:                        true,
				SetupContext7:                    true,
				CopilotMCPHealthEvidence:         test.evidence,
				CopilotCompatibilityVerification: verification,
			})
			if err != nil {
				t.Fatal(err)
			}
			if status := result.CopilotCompatibilityStatus; status.Status == HostCapabilityStatusExact || !strings.Contains(status.Reason, "unverified") {
				t.Fatalf("expected unavailable version or runtime proof to remain unverified and degraded, got %#v", status)
			}
		})
	}

	ordinaryResult, err := Install(Options{Target: "copilot-cli", ProjectPath: filepath.Join(home, "ordinary-install")})
	if err != nil {
		t.Fatalf("expected ordinary offline installation without release resolution: %v", err)
	}
	if status := ordinaryResult.CopilotCompatibilityStatus; status.Status != HostCapabilityStatusDegraded || status.OfficialReleaseSource != "" {
		t.Fatalf("expected ordinary installation to be offline-safe and compatibility to remain unverified, got %#v", status)
	}
}

func countString(values []string, want string) int {
	count := 0
	for _, value := range values {
		if value == want {
			count++
		}
	}
	return count
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
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
