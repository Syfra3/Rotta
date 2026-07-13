package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSCN214_RecoverSafelyFromPartialMultiHostInstallFailure(t *testing.T) {
	// REQ-007, REQ-009 → SCN-214 → TestSCN214_RecoverSafelyFromPartialMultiHostInstallFailure
	// Scenario: Recover safely from a partial multi-host install failure
	home, options := setupPartialMultiHostFailure(t)
	result, err := Install(options)
	if err == nil {
		t.Fatal("expected Codex configuration failure to report partial install failure")
	}
	if result == nil {
		t.Fatal("expected partial result with completed host configuration and recovery guidance")
	}

	assertPartialMultiHostResult(t, result, home, err)
}

func setupPartialMultiHostFailure(t *testing.T) (string, Options) {
	t.Helper()
	home := t.TempDir()
	binDir := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	writePartialFailureAncora(t, filepath.Join(binDir, "ancora"))
	writeTestFile(t, filepath.Join(home, ".codex", "config.toml", "blocked"), []byte("not a file\n"))
	return home, Options{Target: "all", ProjectPath: filepath.Join(home, "project"), InstallSpec: true, InstallImpl: true, InstallReview: true, SetupAncora: true}
}

func writePartialFailureAncora(t *testing.T, path string) {
	t.Helper()
	writeExecutable(t, path, "#!/bin/sh\nif [ \"$1\" = setup ] && [ \"$2\" = opencode ]; then mkdir -p \"$HOME/.config/opencode\"; printf '{\"mcp\":{\"ancora\":{\"type\":\"stdio\",\"command\":\"ancora\",\"args\":[\"mcp\"]}}}' > \"$HOME/.config/opencode/opencode.jsonc\"; fi\n")
}

func assertPartialMultiHostResult(t *testing.T, result *Result, home string, installErr error) {
	t.Helper()
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
	if !strings.Contains(installErr.Error(), "codex") || !strings.Contains(installErr.Error(), "MCP config") {
		t.Fatalf("expected error to identify Codex and failed artifact type, got %v", installErr)
	}
}

func TestSCN215_RefuseToOverwriteMalformedHostConfigurationSilently(t *testing.T) {
	// REQ-007, REQ-009 → SCN-215 → TestSCN215_RefuseToOverwriteMalformedHostConfigurationSilently
	// Scenario: Refuse to overwrite malformed host configuration silently
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	t.Setenv("HOME", home)

	opencodeConfig := filepath.Join(home, ".config", "opencode", "opencode.json")
	malformedConfig := []byte("not json")
	writeTestFile(t, opencodeConfig, malformedConfig)

	result, err := Install(Options{
		Target:        "opencode",
		ProjectPath:   projectPath,
		InstallSpec:   true,
		InstallImpl:   true,
		InstallReview: true,
	})
	if err == nil {
		t.Fatal("expected malformed host configuration to fail before mutation")
	}
	if result == nil {
		t.Fatal("expected failed host result with backup and recovery details")
	}
	if !strings.Contains(err.Error(), "opencode") || !strings.Contains(err.Error(), opencodeConfig) {
		t.Fatalf("expected error to report host and malformed file path, got %v", err)
	}
	if result.Hosts["opencode"].Status != HostInstallStatusFailed {
		t.Fatalf("expected OpenCode host not to claim successful installation, got %#v", result.Hosts["opencode"])
	}
	data, readErr := os.ReadFile(opencodeConfig)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != string(malformedConfig) {
		t.Fatalf("expected malformed configuration to remain untouched, got %q", string(data))
	}
	backupCopy := backupDestination(result.BackupDir, home, opencodeConfig)
	assertFileContains(t, backupCopy, string(malformedConfig))
}

func TestSCN216_PresentPerHostCapabilityMatrix(t *testing.T) {
	// REQ-008 → SCN-216 → TestSCN216_PresentPerHostCapabilityMatrix
	// Scenario: Present a per-host capability matrix
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	t.Setenv("HOME", home)

	result, err := Install(Options{
		Target:        "all",
		ProjectPath:   projectPath,
		InstallSpec:   true,
		InstallImpl:   true,
		InstallReview: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	requiredCapabilities := []string{"installation", "instructions", "commands", "mcp", "health_checks", "lifecycle"}
	allowedStatuses := map[HostCapabilityStatus]bool{
		HostCapabilityStatusExact:         true,
		HostCapabilityStatusAdapted:       true,
		HostCapabilityStatusDegraded:      true,
		HostCapabilityStatusUnsupported:   true,
		HostCapabilityStatusSkipped:       true,
		HostCapabilityStatusFailed:        true,
		HostCapabilityStatusNotApplicable: true,
	}
	for _, host := range []string{"claude-code", "opencode", "codex"} {
		hostResult := result.Hosts[host]
		for _, capabilityName := range requiredCapabilities {
			capability, ok := hostResult.Capabilities[capabilityName]
			if !ok {
				t.Fatalf("expected %s capability matrix to include %q, got %#v", host, capabilityName, hostResult.Capabilities)
			}
			if !allowedStatuses[capability.Status] {
				t.Fatalf("expected %s capability %q to use an allowed matrix status, got %#v", host, capabilityName, capability)
			}
		}
	}
}

func TestSCN216_PresentDegradedCodexMCPMatrixWhenContext7Selected(t *testing.T) {
	// REQ-008 → SCN-216 → TestSCN216_PresentDegradedCodexMCPMatrixWhenContext7Selected
	// Scenario: Present a per-host capability matrix
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	binDir := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	writeContext7StrictFakeNPX(t, filepath.Join(binDir, "npx"), true, []string{"resolve-library-id", "query-docs"})

	result, err := Install(Options{
		Target:        "all",
		ProjectPath:   projectPath,
		InstallSpec:   true,
		InstallImpl:   true,
		InstallReview: true,
		SetupContext7: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.Hosts["codex"].Capabilities["mcp"].Status != HostCapabilityStatusDegraded {
		t.Fatalf("expected Codex aggregate MCP matrix entry to disclose degraded Context7 health parity, got %#v", result.Hosts["codex"].Capabilities["mcp"])
	}
	if result.Hosts["codex"].Capabilities["mcp"].Remediation == "" {
		t.Fatalf("expected Codex aggregate MCP matrix entry to include verification remediation, got %#v", result.Hosts["codex"].Capabilities["mcp"])
	}
	if result.Hosts["opencode"].Capabilities["mcp"].Status != HostCapabilityStatusExact {
		t.Fatalf("expected OpenCode aggregate MCP matrix entry to remain exact, got %#v", result.Hosts["opencode"].Capabilities["mcp"])
	}
}

func TestSCN217_PreserveExistingContext7WhenAddingCodex(t *testing.T) {
	// REQ-010 → SCN-217 → TestSCN217_PreserveExistingContext7WhenAddingCodex
	// Scenario: Preserve existing OpenCode and Claude Code Context7 behavior when adding Codex
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	binDir := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	opencodeConfig := filepath.Join(home, ".config", "opencode", "opencode.json")
	claudeContext7 := filepath.Join(home, ".claude", "mcp", "context7.json")
	writeTestFile(t, opencodeConfig, []byte(`{"mcp":{"context7":{"type":"stdio","command":"npx","args":["-y","@upstash/context7-mcp"]},"user-server":{"command":"keep"}},"theme":"keep"}`))
	writeTestFile(t, claudeContext7, []byte(`{"type":"stdio","command":"npx","args":["-y","@upstash/context7-mcp"]}`))
	writeContext7StrictFakeNPX(t, filepath.Join(binDir, "npx"), true, []string{"resolve-library-id", "query-docs"})

	result, err := Install(Options{
		Target:        "codex",
		ProjectPath:   projectPath,
		InstallSpec:   true,
		InstallImpl:   true,
		InstallReview: true,
		SetupContext7: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	assertContext7OpenCodeEntry(t, opencodeConfig)
	assertFileContains(t, opencodeConfig, "user-server")
	assertFileContains(t, opencodeConfig, "theme")
	assertFileContainsCount(t, opencodeConfig, `"context7"`, 1)
	assertFileDoesNotContain(t, opencodeConfig, "rotta-context7")
	assertContext7ClaudeEntry(t, claudeContext7)

	if !result.Context7.OpenCode.OK || !result.Context7.ClaudeCode.OK {
		t.Fatalf("expected existing OpenCode and Claude Code Context7 entries to remain successful, got %#v", result.Context7)
	}
	if !result.Context7.Codex.OK || result.Context7.Codex.Host != "codex" {
		t.Fatalf("expected Codex Context7 result to be reported independently, got %#v", result.Context7)
	}
	capability := result.Hosts["codex"].Capabilities["mcp:context7"]
	if capability.Status != HostCapabilityStatusDegraded {
		t.Fatalf("expected Codex Context7 capability to be reported independently, got %#v", capability)
	}
}

func TestSCN218_GeneratedHostRulesDescribeAncoraArtifactFallback(t *testing.T) {
	// REQ-011, REQ-014 → SCN-218 → TestSCN218_GeneratedHostRulesDescribeAncoraArtifactFallback
	// Scenario: Continue from OpenSpec workflow artifacts when Ancora is unavailable
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	binDir := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	writeHostCompatibilityFakeAncora(t, filepath.Join(binDir, "ancora"))

	_, err := Install(Options{
		Target:        "all",
		ProjectPath:   projectPath,
		InstallSpec:   true,
		InstallImpl:   true,
		InstallReview: true,
		SetupAncora:   true,
	})
	if err != nil {
		t.Fatal(err)
	}

	for host, path := range map[string]string{
		"claude-code": filepath.Join(home, ".claude", "skills", "rotta", "implementation-mode", "SKILL.md"),
		"opencode":    filepath.Join(home, ".config", "opencode", "skills", "rotta-orchestrator", "SKILL.md"),
		"codex":       filepath.Join(home, ".codex", "AGENTS.md"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s instructions: %v", host, err)
		}
		assertContainsAll(t, string(data), []string{
			"Ancora Fallback",
			"missing or unavailable", "times out", "permission is denied",
			"cannot recover workflow state", "cannot save workflow state", "cannot otherwise be used",
			"workspace and installed-system OpenSpec workflow artifacts",
			"Do not fabricate recovered state", "do not block workflow progress",
			"failure category", "safe retry or recovery action",
		})
	}
}

func TestSCN219_GeneratedHostRulesPreserveWorkflowGatesDuringAncoraFallback(t *testing.T) {
	// REQ-011, REQ-005 → SCN-219 → TestSCN219_GeneratedHostRulesPreserveWorkflowGatesDuringAncoraFallback
	// Scenario: Preserve workflow gates while Ancora fallback is active
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	binDir := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	writeHostCompatibilityFakeAncora(t, filepath.Join(binDir, "ancora"))

	_, err := Install(Options{
		Target:        "all",
		ProjectPath:   projectPath,
		InstallSpec:   true,
		InstallImpl:   true,
		InstallReview: true,
		SetupAncora:   true,
	})
	if err != nil {
		t.Fatal(err)
	}

	for host, path := range map[string]string{
		"claude-code": filepath.Join(home, ".claude", "skills", "rotta", "implementation-mode", "SKILL.md"),
		"opencode":    filepath.Join(home, ".config", "opencode", "skills", "rotta-orchestrator", "SKILL.md"),
		"codex":       filepath.Join(home, ".codex", "AGENTS.md"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s instructions: %v", host, err)
		}
		assertContainsAll(t, string(data), []string{
			"While Ancora fallback is active",
			"canonical phase order", "explicit human approval gate", "TDD preconditions", "quality gates",
			"workspace and installed-system OpenSpec workflow artifacts",
			"do not bypass a required human approval or quality gate",
		})
	}
}

func TestSCN220_GeneratedHostRulesBoundVelaDegradedSourceExploration(t *testing.T) {
	// REQ-012, REQ-014 → SCN-220 → TestSCN220_GeneratedHostRulesBoundVelaDegradedSourceExploration
	// Scenario: Use bounded source exploration when Vela cannot provide graph evidence
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	binDir := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	writeHostCompatibilityFakeVela(t, filepath.Join(binDir, "vela"))

	_, err := Install(Options{
		Target:        "all",
		ProjectPath:   projectPath,
		InstallSpec:   true,
		InstallImpl:   true,
		InstallReview: true,
		SetupVela:     true,
	})
	if err != nil {
		t.Fatal(err)
	}

	for host, path := range map[string]string{
		"claude-code": filepath.Join(home, ".claude", "skills", "rotta", "implementation-mode", "SKILL.md"),
		"opencode":    filepath.Join(home, ".config", "opencode", "skills", "rotta-orchestrator", "SKILL.md"),
		"codex":       filepath.Join(home, ".codex", "AGENTS.md"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s instructions: %v", host, err)
		}
		got := string(data)
		assertContainsAll(t, got, []string{
			"Vela Degradation Fallback",
			"missing graph tools", "times out", "permission is denied", "stale, unusable, or failed graph data",
			"visible Vela-degraded state", "Do not invoke a replacement graph MCP",
			"no more than five focused source/code exploration actions",
			"source-derived evidence", "Vela graph proof was unavailable", "remaining gap",
			"phase order", "approval gates", "quality gates",
		})
	}
}
