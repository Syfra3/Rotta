package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSCN221_GeneratedHostRulesDescribeContext7Degradation(t *testing.T) {
	// REQ-013, REQ-014 → SCN-221 → TestSCN221_GeneratedHostRulesDescribeContext7Degradation
	// Scenario: Continue without inventing library details when Context7 fails
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	binDir := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	writeContext7StrictFakeNPX(t, filepath.Join(binDir, "npx"), true, []string{"resolve-library-id", "query-docs"})

	_, err := Install(Options{
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
			"Context7 Degradation Fallback",
			"missing or unavailable Context7 tools", "times out", "permission is denied",
			"command, initialization, or documentation-query failure",
			"visible Context7-degraded state", "continues without a documentation lookup",
			"does not present unverified library or API details as fact",
			"assumptions and verification needs", "project or user-provided evidence",
			"phase order", "approval", "TDD", "review", "quality-gate", "source-of-truth",
		})
	}
}

func TestSCN222_ReportSelectedMCPConfigurationSeparatelyFromRuntimeFallback(t *testing.T) {
	// REQ-014, REQ-011, REQ-012, REQ-013 → SCN-222 → TestSCN222_ReportSelectedMCPConfigurationSeparatelyFromRuntimeFallback
	// Scenario: Expose selected MCP configuration and runtime fallback states
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	binDir := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	writeHostCompatibilityFakeAncora(t, filepath.Join(binDir, "ancora"))
	writeHostCompatibilityFakeVela(t, filepath.Join(binDir, "vela"))
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

	assertSelectedMCPStatuses(t, result)

	t.Run("health failure is reported per selected MCP", func(t *testing.T) {
		failureHome := t.TempDir()
		failureProjectPath := filepath.Join(failureHome, "project")
		failureBinDir := filepath.Join(failureHome, "bin")
		t.Setenv("HOME", failureHome)
		t.Setenv("PATH", failureBinDir+string(os.PathListSeparator)+os.Getenv("PATH"))
		writeHostCompatibilityFakeAncora(t, filepath.Join(failureBinDir, "ancora"))
		writeHostCompatibilityFakeVela(t, filepath.Join(failureBinDir, "vela"))
		writeExecutable(t, filepath.Join(failureBinDir, "npx"), "#!/bin/sh\nexit 2\n")

		failed, installErr := Install(Options{
			Target:        "all",
			ProjectPath:   failureProjectPath,
			InstallSpec:   true,
			InstallImpl:   true,
			InstallReview: true,
			SetupAncora:   true,
			SetupVela:     true,
			SetupContext7: true,
		})
		if installErr == nil {
			t.Fatal("expected Context7 health failure")
		}
		assertContext7HealthFailures(t, failed)
	})
}

func assertSelectedMCPStatuses(t *testing.T, result *Result) {
	t.Helper()
	for _, host := range []string{"claude-code", "opencode", "codex"} {
		for _, mcp := range []string{"ancora", "vela", "context7"} {
			assertSelectedMCPStatus(t, result, host, mcp)
		}
	}
	if status := result.MCPStatuses["codex"]["context7"]; status.Status != MCPStatusDegraded {
		t.Fatalf("expected detected Codex Context7 health limitation to be degraded, not healthy, got %#v", status)
	}
}

func assertSelectedMCPStatus(t *testing.T, result *Result, host, mcp string) {
	t.Helper()
	status, ok := result.MCPStatuses[host][mcp]
	if !ok {
		t.Fatalf("expected %s MCP status for %s, got %#v", mcp, host, result.MCPStatuses)
	}
	if status.Reason == "" || status.Remediation == "" {
		t.Fatalf("expected %s MCP status for %s to include reason/remediation, got %#v", mcp, host, status)
	}
	if status.RuntimeFallback.State != MCPRuntimeFallbackNotObserved {
		t.Fatalf("expected installer to distinguish later runtime fallback from install status, got %#v", status)
	}
}

func assertContext7HealthFailures(t *testing.T, result *Result) {
	t.Helper()
	for _, host := range []string{"claude-code", "opencode", "codex"} {
		if got := result.MCPStatuses[host]["context7"].Status; got != MCPStatusFailed {
			t.Fatalf("expected Context7 health failure for %s to be reported as failed, got %#v", host, result.MCPStatuses)
		}
		if got := result.MCPStatuses[host]["ancora"].Status; got != MCPStatusConfigured {
			t.Fatalf("expected unrelated configured Ancora status for %s to remain configured, got %#v", host, result.MCPStatuses)
		}
	}
}

func TestSCN222_FailedHostCannotReportConfiguredSelectedMCP(t *testing.T) {
	// REQ-014, REQ-011, REQ-012, REQ-013 → SCN-222 → TestSCN222_FailedHostCannotReportConfiguredSelectedMCP
	// Scenario: Expose selected MCP configuration and runtime fallback states
	status := mcpStatusResult(HostInstallResult{
		Host:   "opencode",
		Status: HostInstallStatusFailed,
	}, "mcp:ancora")

	if status.Status == MCPStatusConfigured {
		t.Fatalf("expected failed selected host to prevent configured/healthy MCP status, got %#v", status)
	}
}

func TestSCN214_HostCompatibilityRecoveryBranchesRemainCovered(t *testing.T) {
	// REQ-007, REQ-009 → SCN-214 → TestSCN214_HostCompatibilityRecoveryBranchesRemainCovered
	// Scenario: Recover safely from a partial multi-host install failure
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")

	if got := installTargetLabel(""); got != "selected" {
		t.Fatalf("expected empty install target label to be selected, got %q", got)
	}

	result := &Result{Hosts: map[string]HostInstallResult{}}
	recordHostArtifactFailure(result, "codex", "Codex MCP config", Options{SetupAncora: true, SetupVela: true, SetupContext7: true})
	if result.Hosts["codex"].Status != HostInstallStatusFailed {
		t.Fatalf("expected missing host artifact failure to create failed host result, got %#v", result.Hosts["codex"])
	}
	for _, capabilityName := range []string{"mcp:ancora", "mcp:vela", "mcp:context7"} {
		if result.Hosts["codex"].Capabilities[capabilityName].Status != HostCapabilityStatusFailed {
			t.Fatalf("expected %s to be failed, got %#v", capabilityName, result.Hosts["codex"].Capabilities)
		}
	}

	result = &Result{Hosts: map[string]HostInstallResult{"codex": {Host: "codex", Status: HostInstallStatusInstalled}}}
	recordMCPHostCapabilities(result, Options{Target: "codex", SetupAncora: true})
	if result.Hosts["codex"].Capabilities["mcp:ancora"].Status != HostCapabilityStatusExact {
		t.Fatalf("expected nil capability map to be initialized with exact Ancora MCP, got %#v", result.Hosts["codex"])
	}

	result = &Result{Hosts: map[string]HostInstallResult{"codex": {Host: "codex", Status: HostInstallStatusInstalled}}}
	recordHostCapabilityMatrix(result, Options{Target: "all"})
	if result.Hosts["codex"].Capabilities["commands"].Status != HostCapabilityStatusAdapted {
		t.Fatalf("expected matrix to fill missing command capability, got %#v", result.Hosts["codex"].Capabilities)
	}
	if installationCapability(HostInstallStatusFailed).Status != HostCapabilityStatusFailed {
		t.Fatal("expected failed installation capability for failed host status")
	}

	result = &Result{Hosts: map[string]HostInstallResult{
		"opencode": {Host: "opencode", Status: HostInstallStatusFailed},
		"codex":    {Host: "codex", Status: HostInstallStatusInstalled},
	}}
	recordMCPHealthFailure(result, Options{Target: "all"}, "mcp:context7", Context7HealthResult{Category: Context7FailureStartup, Message: "boom"})
	if result.Hosts["codex"].Capabilities["mcp:context7"].Status != HostCapabilityStatusFailed {
		t.Fatalf("expected installed selected host to record MCP health failure, got %#v", result.Hosts["codex"])
	}
	if _, ok := result.Hosts["opencode"].Capabilities["mcp:context7"]; ok {
		t.Fatalf("expected already failed host to be skipped by MCP health failure, got %#v", result.Hosts["opencode"])
	}

	assertAllHostConfigFailure(t, home, projectPath)
}

func assertAllHostConfigFailure(t *testing.T, home, projectPath string) {
	t.Helper()
	writeTestFile(t, filepath.Join(projectPath, ".rotta"), []byte("not a directory"))
	if _, err := installAllHosts(Options{InstallSpec: true}, &Result{Hosts: map[string]HostInstallResult{}}, home, projectPath); err == nil {
		t.Fatal("expected all-host install to report project config write failure")
	}
}

func TestSCN215_HostCompatibilityWriteFailuresRemainCovered(t *testing.T) {
	// REQ-007, REQ-009 → SCN-215 → TestSCN215_HostCompatibilityWriteFailuresRemainCovered
	// Scenario: Refuse to overwrite malformed host configuration silently
	home := t.TempDir()

	writeTestFile(t, filepath.Join(home, ".codex"), []byte("not a directory"))
	if _, err := installCodex(Options{}, home); err == nil {
		t.Fatal("expected Codex instruction directory creation failure")
	}
	if _, err := configureCodexMCPServers(Options{SetupContext7: true}, home); err == nil {
		t.Fatal("expected Codex MCP directory creation failure")
	}
	if _, err := cleanAndInstallHost(Options{}, "codex", home); err == nil {
		t.Fatal("expected Codex clean/install to surface stale artifact cleanup failure")
	}
	if _, err := cleanAndInstallHost(Options{}, "unsupported", home); err == nil {
		t.Fatal("expected unsupported internal host dispatch to fail")
	}

	home = t.TempDir()
	writeTestFile(t, filepath.Join(home, ".codex", "AGENTS.md", "blocked"), []byte("not a file"))
	if _, err := installCodex(Options{}, home); err == nil {
		t.Fatal("expected Codex instruction file write failure")
	}

	home = t.TempDir()
	writeTestFile(t, filepath.Join(home, ".codex", "config.toml", "blocked"), []byte("not a file"))
	if _, err := configureCodexMCPServers(Options{SetupContext7: true}, home); err == nil {
		t.Fatal("expected Codex MCP config write failure")
	}

	home = t.TempDir()
	writeTestFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"), []byte("not json"))
	if _, err := cleanAndInstallHost(Options{}, "opencode", home); err == nil {
		t.Fatal("expected OpenCode clean/install to surface malformed config")
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

func writeHostCompatibilityFakeAncora(t *testing.T, path string) {
	t.Helper()
	writeExecutable(t, path, `#!/bin/sh
exit 0
`)
}

func writeHostCompatibilityFakeVela(t *testing.T, path string) {
	t.Helper()
	writeExecutable(t, path, `#!/bin/sh
project=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --project) shift; project="$1" ;;
  esac
  shift
done
if [ -n "$project" ]; then
  mkdir -p "$project/.vela"
  printf 'fresh graph' > "$project/.vela/graph.db"
fi
exit 0
`)
}
