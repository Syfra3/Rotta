package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSCN222_PrivateArtifactPathsRejectMissingAndInvalidParents(t *testing.T) {
	// REQ-014 → SCN-222 → TestSCN222_PrivateArtifactPathsRejectMissingAndInvalidParents
	// Scenario: Expose selected MCP configuration and runtime fallback states
	home := t.TempDir()
	assertMissingPrivateParentRejected(t, home)
	assertPrivateArtifactExistence(t, home)
}

func assertMissingPrivateParentRejected(t *testing.T, home string) {
	t.Helper()
	missing := filepath.Join(home, "missing", "artifact.json")
	exists, err := fileExistsWithinParent(missing)
	if err == nil || exists {
		t.Fatalf("expected missing parent to be rejected, exists=%t err=%v", exists, err)
	}
}

func assertPrivateArtifactExistence(t *testing.T, home string) {
	t.Helper()
	path := filepath.Join(home, "private", "artifact.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := writePrivateFile(path, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	exists, err := fileExistsWithinParent(path)
	if err != nil || !exists {
		t.Fatalf("expected private artifact to be found, exists=%t err=%v", exists, err)
	}
	exists, err = fileExistsWithinParent(filepath.Join(home, "private", "missing.json"))
	if err != nil || exists {
		t.Fatalf("expected absent private artifact to be reported absent, exists=%t err=%v", exists, err)
	}
	if _, err := readPrivateFile(filepath.Join(home, "private", "missing.json")); err == nil {
		t.Fatal("expected missing private artifact read to fail")
	}
	if err := writePrivateFile(filepath.Join(home, "missing", "artifact.json"), []byte("private"), 0o600); err == nil {
		t.Fatal("expected private artifact write with missing parent to fail")
	}
}

func TestSCN222_InstallerDispatchRejectsUnsupportedHost(t *testing.T) {
	// REQ-014 → SCN-222 → TestSCN222_InstallerDispatchRejectsUnsupportedHost
	// Scenario: Expose selected MCP configuration and runtime fallback states
	if _, err := installHost(Options{}, "unsupported", t.TempDir()); err == nil {
		t.Fatal("expected unsupported host dispatch to fail")
	}
}

func TestSCN222_OptionalSetupReportsUnavailableBinaries(t *testing.T) {
	// REQ-014 → SCN-222 → TestSCN222_OptionalSetupReportsUnavailableBinaries
	// Scenario: Expose selected MCP configuration and runtime fallback states
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", filepath.Join(home, "empty-bin"))
	assertOptionalSetupUnavailable(t, home)
}

func TestSCN222_BackupReportsBlockedRootsAndScopedPaths(t *testing.T) {
	// REQ-014 → SCN-222 → TestSCN222_BackupReportsBlockedRootsAndScopedPaths
	// Scenario: Expose selected MCP configuration and runtime fallback states
	assertBackupBlockedRootFails(t)
	assertBackupBlockedScopedPathFails(t)
}

func TestSCN222_BackupReportsCopyDestinationFailures(t *testing.T) {
	// REQ-014 → SCN-222 → TestSCN222_BackupReportsCopyDestinationFailures
	// Scenario: Expose selected MCP configuration and runtime fallback states
	home := t.TempDir()
	assertBackupDirectoryCopyFailure(t, home)
	assertBackupFileCopyFailure(t, home)
}

func TestSCN222_CleanupReportsMalformedSelectedVelaSettings(t *testing.T) {
	// REQ-014 → SCN-222 → TestSCN222_CleanupReportsMalformedSelectedVelaSettings
	// Scenario: Expose selected MCP configuration and runtime fallback states
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, ".claude", "settings.json"), []byte("not json"))
	if err := cleanVelaArtifacts("claude-code", home, filepath.Join(home, "project")); err == nil {
		t.Fatal("expected malformed selected Vela settings to be reported")
	}
}

func TestSCN222_CleanupIgnoresUnknownHostWithoutDeletingFiles(t *testing.T) {
	// REQ-014 → SCN-222 → TestSCN222_CleanupIgnoresUnknownHostWithoutDeletingFiles
	// Scenario: Expose selected MCP configuration and runtime fallback states
	home := t.TempDir()
	if err := cleanHostInstallation("unknown", home); err != nil {
		t.Fatalf("expected unknown host cleanup to be a no-op, got %v", err)
	}
}

func assertBackupDirectoryCopyFailure(t *testing.T, home string) {
	t.Helper()
	source := filepath.Join(home, "source-dir")
	if err := os.Mkdir(source, 0o750); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(home, "backup-dir")
	writeTestFile(t, filepath.Join(backup, "files"), []byte("blocked"))
	if err := backupInstallPaths(&backupManifest{}, backup, home, []string{source}); err == nil {
		t.Fatal("expected directory backup copy failure to be reported")
	}
}

func assertBackupFileCopyFailure(t *testing.T, home string) {
	t.Helper()
	source := filepath.Join(home, "source-file")
	writeTestFile(t, source, []byte("source"))
	backup := filepath.Join(home, "backup-file")
	writeTestFile(t, filepath.Join(backup, "files"), []byte("blocked"))
	if err := backupInstallPaths(&backupManifest{}, backup, home, []string{source}); err == nil {
		t.Fatal("expected file backup copy failure to be reported")
	}
}

func assertBackupBlockedRootFails(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, ".rotta"), []byte("blocked"))
	if _, err := createInstallBackup(Options{}, home, filepath.Join(home, "project")); err == nil {
		t.Fatal("expected blocked backup root to be reported")
	}
}

func assertBackupBlockedScopedPathFails(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	project := filepath.Join(home, "project")
	writeTestFile(t, filepath.Join(project, ".rotta"), []byte("blocked"))
	if _, err := createInstallBackup(Options{}, home, project); err == nil {
		t.Fatal("expected blocked scoped backup path to be reported")
	}
}

func assertOptionalSetupUnavailable(t *testing.T, home string) {
	t.Helper()
	if _, err := SetupAncora(Options{Target: "opencode"}, home); err == nil || !strings.Contains(err.Error(), "ancora setup opencode") {
		t.Fatalf("expected unavailable Ancora to be reported, got %v", err)
	}
	if _, err := SetupVela(Options{Target: "opencode"}, home, filepath.Join(home, "project")); err == nil || !strings.Contains(err.Error(), "vela install opencode") {
		t.Fatalf("expected unavailable Vela to be reported, got %v", err)
	}
}

func TestSCN222_Context7SetupReportsPartialConfiguration(t *testing.T) {
	// REQ-014 → SCN-222 → TestSCN222_Context7SetupReportsPartialConfiguration
	// Scenario: Expose selected MCP configuration and runtime fallback states
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".config"), []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	result := &Result{}
	if _, err := setupContext7(Options{Target: "opencode", SetupContext7: true}, result, home, filepath.Join(home, "project")); err != nil {
		t.Fatalf("expected an observable partial configuration result, got %v", err)
	}
	if result.Context7.OpenCode.OK || result.Context7.FullyConfigured {
		t.Fatalf("expected blocked OpenCode configuration to be reported as partial, got %#v", result.Context7)
	}
}

func TestSCN222_OptionalMCPSetupReportsCommandFailures(t *testing.T) {
	// REQ-014 → SCN-222 → TestSCN222_OptionalMCPSetupReportsCommandFailures
	// Scenario: Expose selected MCP configuration and runtime fallback states
	home := t.TempDir()
	binDir := filepath.Join(home, "bin")
	t.Setenv("PATH", binDir)
	writeExecutable(t, filepath.Join(binDir, "ancora"), "#!/bin/sh\nexit 19\n")
	writeExecutable(t, filepath.Join(binDir, "vela"), "#!/bin/sh\nexit 23\n")

	if _, err := SetupAncora(Options{Target: "opencode"}, home); err == nil || !strings.Contains(err.Error(), "ancora setup opencode") {
		t.Fatalf("expected selected Ancora setup failure, got %v", err)
	}
	if _, err := SetupVela(Options{Target: "claude-code"}, home, filepath.Join(home, "project")); err == nil || !strings.Contains(err.Error(), "vela install claude") {
		t.Fatalf("expected selected Vela setup failure, got %v", err)
	}
	if err := runCommand(Options{}, filepath.Join(binDir, "unsupported"), "install"); err == nil {
		t.Fatal("expected unsupported command dispatcher executable to fail")
	}
}

func TestSCN222_InstallerPathHelpersReportInvalidPaths(t *testing.T) {
	// REQ-014 → SCN-222 → TestSCN222_InstallerPathHelpersReportInvalidPaths
	// Scenario: Expose selected MCP configuration and runtime fallback states
	home := t.TempDir()
	if err := validateContext7HealthCommand(Context7MCPServer{Command: "other"}); err == nil {
		t.Fatal("expected unmanaged Context7 command to be rejected")
	}
	if resolveProjectPath("~/project", home) != filepath.Join(home, "project") || resolveProjectPath("~", home) != home {
		t.Fatal("expected project path shortcuts to remain rooted at the selected home")
	}
	blocked := filepath.Join(home, "blocked")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := backupInstallPaths(&backupManifest{}, filepath.Join(home, "backup"), home, []string{filepath.Join(blocked, "child")}); err == nil {
		t.Fatal("expected backup to report an inaccessible scoped path")
	}
	if err := os.MkdirAll(filepath.Join(home, ".config", "opencode"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".config", "opencode", "skills"), []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := installOpenCode(Options{InstallSpec: true}, home); err == nil {
		t.Fatal("expected blocked OpenCode skill directory to abort installation")
	}
}

func TestSCN222_InstallAndClaudeSettingsFailuresRemainObservable(t *testing.T) {
	// REQ-014 → SCN-222 → TestSCN222_InstallAndClaudeSettingsFailuresRemainObservable
	// Scenario: Expose selected MCP configuration and runtime fallback states
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	if err := os.MkdirAll(projectPath, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectPath, ".rotta"), []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	result := &Result{Hosts: map[string]HostInstallResult{}}
	if _, err := installSelectedHosts(Options{Target: "codex", InstallSpec: true}, result, home, projectPath); err == nil {
		t.Fatal("expected project artifact directory failure to be returned after host installation")
	}

	settings := filepath.Join(home, ".claude", "settings.json")
	writeTestFile(t, settings, []byte("not json"))
	if _, err := readClaudeCodeSettings(settings); err == nil {
		t.Fatal("expected malformed Claude settings to be reported")
	}
}

func TestSCN222_MCPValidationRejectsMismatchedArgumentsAndHosts(t *testing.T) {
	// REQ-014 → SCN-222 → TestSCN222_MCPValidationRejectsMismatchedArgumentsAndHosts
	// Scenario: Expose selected MCP configuration and runtime fallback states
	if sameArguments([]string{"-y"}, context7CommandArgs) || sameArguments([]string{"wrong", "@upstash/context7-mcp"}, context7CommandArgs) {
		t.Fatal("expected Context7 argument validation to reject length and value mismatches")
	}
	if err := runAncoraSetup(Options{}, "", "unsupported"); err == nil {
		t.Fatal("expected unsupported Ancora host to be rejected")
	}
	if got := ancoraSetupHosts("claude-code"); len(got) != 1 || got[0] != "claude-code" {
		t.Fatalf("expected Claude-only Ancora setup selection, got %#v", got)
	}
	if keepResult, err := setupContext7(Options{}, &Result{}, t.TempDir(), t.TempDir()); err != nil || keepResult {
		t.Fatalf("expected unselected Context7 to skip setup, keep=%t err=%v", keepResult, err)
	}
}

func TestSCN222_SelectedMCPStatusMapsEveryCapabilityOutcome(t *testing.T) {
	// REQ-014 → SCN-222 → TestSCN222_SelectedMCPStatusMapsEveryCapabilityOutcome
	// Scenario: Expose selected MCP configuration and runtime fallback states
	cases := map[HostCapabilityStatus]MCPStatus{
		HostCapabilityStatusSkipped:     MCPStatusSkipped,
		HostCapabilityStatusDegraded:    MCPStatusDegraded,
		HostCapabilityStatusUnsupported: MCPStatusDegraded,
		HostCapabilityStatusFailed:      MCPStatusFailed,
		HostCapabilityStatusExact:       MCPStatusConfigured,
	}
	for capability, want := range cases {
		if got := statusForCapability(capability); got != want {
			t.Fatalf("capability %q mapped to %q, want %q", capability, got, want)
		}
	}
}

func TestSCN222_MCPStatusPreservesCapabilityExplanation(t *testing.T) {
	// REQ-014 → SCN-222 → TestSCN222_MCPStatusPreservesCapabilityExplanation
	// Scenario: Expose selected MCP configuration and runtime fallback states
	status := mcpStatusResult(HostInstallResult{Capabilities: map[string]HostCapability{
		"mcp:vela": {Status: HostCapabilityStatusDegraded, Reason: "graph stale", Remediation: "refresh graph"},
	}}, "mcp:vela")
	if status.Status != MCPStatusDegraded || status.Reason != "graph stale" || status.Remediation != "refresh graph" {
		t.Fatalf("expected capability explanation in MCP status, got %#v", status)
	}
}

func TestSCN222_IntegrationInstructionsIncludeEveryEnabledBoundary(t *testing.T) {
	// REQ-014 → SCN-222 → TestSCN222_IntegrationInstructionsIncludeEveryEnabledBoundary
	// Scenario: Expose selected MCP configuration and runtime fallback states
	enabled := integrationInstructions(Options{SetupAncora: true, SetupVela: true, SetupContext7: true})
	disabled := integrationInstructions(Options{})
	assertIntegrationDocumentSections(t, enabled)
	assertIntegrationChoicesChangeDocument(t, enabled, disabled)
}

func assertIntegrationDocumentSections(t *testing.T, text string) {
	t.Helper()
	sections := []string{"## Rotta Canonical Workflow Contract", "## Installed Integration Choices (Authoritative)", "### Ancora Memory Enabled", "### Vela Graph Intelligence Enabled", "### Context7 Degradation Fallback", "### Exploration Enrichment For Vela"}
	previous := -1
	for _, section := range sections {
		current := strings.Index(text, section)
		if current < 0 || current <= previous {
			t.Fatalf("expected ordered generated instruction section %q, got %q", section, text)
		}
		previous = current
	}
	for _, boundary := range []string{"slash commands.\n\n## Installed Integration", "quality gate.\n\n### Vela", "quality gates.\n\n### Context7", "requirements.\n\n### Exploration"} {
		if !strings.Contains(text, boundary) {
			t.Fatalf("expected generated Markdown sections to remain separated at %q in %q", boundary, text)
		}
	}
}

func assertIntegrationChoicesChangeDocument(t *testing.T, enabled, disabled string) {
	t.Helper()
	if !strings.HasPrefix(enabled, "\n---\n\n") || !strings.HasPrefix(disabled, "\n---\n\n") {
		t.Fatal("expected generated instructions to preserve their Markdown document boundary")
	}
	if strings.Contains(disabled, "### Ancora Memory Enabled") || strings.Contains(disabled, "### Vela Graph Intelligence Enabled") || strings.Contains(disabled, "### Context7 Degradation Fallback") {
		t.Fatal("expected disabled integrations to change generated behavior")
	}
	if !strings.Contains(disabled, "### Ancora Memory Disabled") || !strings.Contains(disabled, "### Vela Graph Intelligence Disabled") || !strings.Contains(disabled, "### Exploration Output") {
		t.Fatal("expected disabled integrations to retain explicit fallback guidance")
	}
}
