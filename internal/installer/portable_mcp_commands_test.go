package installer

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSCN223_SerializesManagedMCPExecutablesAsBareCommands(t *testing.T) {
	// REQ-015 → SCN-223 → TestSCN223_SerializesManagedMCPExecutablesAsBareCommands
	// Scenario: Serialize a managed MCP executable as a bare command
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", bin+":/bin")

	writeExecutable(t, filepath.Join(bin, "ancora"), `#!/bin/sh
case "$2" in
  claude-code) mkdir -p "$HOME/.claude/mcp"; printf '{"command":"/opt/homebrew/Cellar/ancora/1.2.3/bin/ancora","args":["mcp"]}' > "$HOME/.claude/mcp/ancora.json" ;;
  opencode) mkdir -p "$HOME/.config/opencode"; printf '{"mcp":{"ancora":{"command":"/opt/homebrew/Cellar/ancora/1.2.3/bin/ancora","args":["mcp"]}}}' > "$HOME/.config/opencode/opencode.jsonc" ;;
esac
`)
	writeExecutable(t, filepath.Join(bin, "vela"), `#!/bin/sh
agent=""
while [ "$#" -gt 0 ]; do
  case "$1" in --agent) shift; agent="$1" ;; esac
  shift
done
case "$agent" in
  claude) mkdir -p "$HOME/.claude"; printf '{"command":"/home/linuxbrew/.linuxbrew/Cellar/vela/4.5.6/bin/vela","args":["mcp"]}' > "$HOME/.claude/vela-mcp.json" ;;
  opencode) mkdir -p "$HOME/.config/opencode"; printf '{"mcp":{"vela":{"command":"/home/linuxbrew/.linuxbrew/Cellar/vela/4.5.6/bin/vela","args":["mcp"]}}}' > "$HOME/.config/opencode/opencode.json" ;;
esac
`)

	if _, err := SetupAncora(Options{Target: "both"}, home); err != nil {
		t.Fatalf("setup Ancora: %v", err)
	}
	project := t.TempDir()
	if _, err := SetupVela(Options{Target: "both"}, home, project); err != nil {
		t.Fatalf("setup Vela: %v", err)
	}
	context7 := Context7ServerConfig()
	if err := writeOpenCodeContext7MCP(filepath.Join(home, ".config", "opencode", "context7.json"), context7); err != nil {
		t.Fatalf("write Context7 OpenCode MCP: %v", err)
	}
	if err := writeClaudeContext7MCP(filepath.Join(home, ".claude", "mcp", "context7.json"), context7); err != nil {
		t.Fatalf("write Context7 Claude MCP: %v", err)
	}

	for path, want := range map[string]struct{ command, resolved, server string }{
		filepath.Join(home, ".claude", "mcp", "ancora.json"):         {"ancora", "/opt/homebrew/Cellar/ancora/1.2.3/bin/ancora", ""},
		filepath.Join(home, ".config", "opencode", "opencode.jsonc"): {"ancora", "/opt/homebrew/Cellar/ancora/1.2.3/bin/ancora", "ancora"},
		filepath.Join(home, ".claude", "vela-mcp.json"):              {"vela", "/home/linuxbrew/.linuxbrew/Cellar/vela/4.5.6/bin/vela", ""},
		filepath.Join(home, ".config", "opencode", "opencode.json"):  {"vela", "/home/linuxbrew/.linuxbrew/Cellar/vela/4.5.6/bin/vela", "vela"},
		filepath.Join(home, ".config", "opencode", "context7.json"):  {"npx", "/home/user/.local/bin/npx", "context7"},
		filepath.Join(home, ".claude", "mcp", "context7.json"):       {"npx", "/home/user/.local/bin/npx", ""},
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if got := serializedMCPCommand(t, data, want.server); got != want.command {
			t.Errorf("expected %s to serialize bare %q command, got %q", path, want.command, got)
		}
		if strings.Contains(string(data), want.resolved) {
			t.Errorf("expected %s not to serialize an executable path, got %s", path, data)
		}
	}
}

func TestSCN232_RollsBackOnlyFailingOpenCodeAgentInstallation(t *testing.T) {
	// REQ-020 → SCN-232 → TestSCN232_RollsBackOnlyFailingOpenCodeAgentInstallation
	// Scenario: Roll back only the failing coding agent installation
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", bin+":/bin")

	claudeConfig := filepath.Join(home, ".claude", "mcp", "ancora.json")
	openCodeConfig := filepath.Join(home, ".config", "opencode", "opencode.jsonc")
	openCodeHostConfig := filepath.Join(home, ".config", "opencode", "opencode.json")
	openCodeBefore := []byte(`{"mcp":{"user-server":{"command":"keep"}},"theme":"keep"}`)
	writeTestFile(t, openCodeConfig, openCodeBefore)
	openCodeHostBefore := []byte(`{"theme":"keep"}`)
	writeTestFile(t, openCodeHostConfig, openCodeHostBefore)
	writeExecutable(t, filepath.Join(bin, "ancora"), `#!/bin/sh
case "$2" in
  claude-code) mkdir -p "$HOME/.claude/mcp"; printf '{"command":"ancora","args":["mcp"]}' > "$HOME/.claude/mcp/ancora.json" ;;
  opencode) printf '{"mcp":{"ancora":{"command":"ancora","args":["mcp"]}}}' > "$HOME/.config/opencode/opencode.jsonc"; exit 23 ;;
esac
`)

	_, err := Install(Options{Target: "both", ProjectPath: filepath.Join(home, "project"), InstallSpec: true, SetupAncora: true})
	if err == nil {
		t.Fatal("expected OpenCode setup failure")
	}
	if got := mustReadFile(t, claudeConfig); string(got) != `{"command":"ancora","args":["mcp"]}` {
		t.Fatalf("expected completed Claude Code installation to remain intact, got %s", got)
	}
	if got := mustReadFile(t, openCodeConfig); string(got) != string(openCodeBefore) {
		t.Fatalf("expected complete OpenCode pre-installation configuration restored, got %s", got)
	}
	if got := mustReadFile(t, openCodeHostConfig); string(got) != string(openCodeHostBefore) {
		t.Fatalf("expected every OpenCode configuration file restored, got %s", got)
	}
	if !strings.Contains(err.Error(), "OpenCode") || !strings.Contains(err.Error(), "rerun") {
		t.Fatalf("expected OpenCode remediation, got %v", err)
	}
}

// REQ-020 → SCN-233 → TestSCN233_RollsBackEveryPartialAgentConfigurationChange
func TestSCN233_RollsBackEveryPartialAgentConfigurationChange(t *testing.T) {
	// Scenario: Roll back partial configuration changes within one coding agent
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", bin+":/bin")

	configPath := filepath.Join(home, ".config", "opencode", "opencode.jsonc")
	instructionsPath := filepath.Join(home, ".config", "opencode", "instructions.md")
	pluginPath := filepath.Join(home, ".config", "opencode", "plugin", "rotta-vela-freshness-guard.js")
	before := []byte(`{"mcp":{"user-server":{"command":"keep"}},"theme":"keep"}`)
	writeTestFile(t, configPath, before)
	writeExecutable(t, filepath.Join(bin, "ancora"), `#!/bin/sh
mkdir -p "$HOME/.config/opencode/plugin"
printf '{"mcp":{"ancora":{"command":"ancora","args":["mcp"]}}}' > "$HOME/.config/opencode/opencode.jsonc"
printf 'partial Rotta instructions\n' > "$HOME/.config/opencode/instructions.md"
printf 'partial Rotta plugin\n' > "$HOME/.config/opencode/plugin/rotta-vela-freshness-guard.js"
exit 23
`)

	result, err := Install(Options{Target: "opencode", ProjectPath: filepath.Join(home, "project"), SetupAncora: true})
	if err == nil {
		t.Fatal("expected OpenCode setup failure")
	}
	if got := mustReadFile(t, configPath); string(got) != string(before) {
		t.Fatalf("expected preexisting OpenCode configuration restored, got %s", got)
	}
	for _, path := range []string{instructionsPath, pluginPath} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("expected newly created configuration %s removed, stat error: %v", path, statErr)
		}
	}
	if result == nil || result.Hosts["opencode"].Status != HostInstallStatusFailed {
		t.Fatalf("expected failed OpenCode installation to be reported, got %#v", result)
	}
}

// REQ-016 → SCN-224 → TestSCN224_ReinstallNormalizesManagedVelaCellarCommand
func TestSCN224_ReinstallNormalizesManagedVelaCellarCommand(t *testing.T) {
	// Scenario: Normalize a stale managed Homebrew MCP executable during reinstall
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	project := filepath.Join(home, "project")
	t.Setenv("HOME", home)
	t.Setenv("PATH", bin+":/bin")
	writeExecutable(t, filepath.Join(bin, "vela"), "#!/bin/sh\nexit 0\n")

	configPath := filepath.Join(home, ".claude", "vela-mcp.json")
	writeTestFile(t, configPath, []byte(`{"command":"/home/linuxbrew/.linuxbrew/Cellar/vela/4.5.6/bin/vela","args":["mcp"]}`))

	first, err := SetupVela(Options{Target: "claude-code"}, home, project)
	if err != nil {
		t.Fatalf("first Vela reinstall: %v", err)
	}
	if got := first.NormalizedMCPEntries; len(got) != 1 || got[0] != configPath {
		t.Fatalf("expected normalized Vela entry %q, got %v", configPath, got)
	}
	if got := serializedMCPCommand(t, mustReadFile(t, configPath), ""); got != "vela" {
		t.Fatalf("expected normalized Vela command vela, got %q", got)
	}

	second, err := SetupVela(Options{Target: "claude-code"}, home, project)
	if err != nil {
		t.Fatalf("second Vela reinstall: %v", err)
	}
	if len(second.NormalizedMCPEntries) != 0 {
		t.Fatalf("expected idempotent reinstall to report no command-field change, got %v", second.NormalizedMCPEntries)
	}
}

// REQ-016 → SCN-225 → TestSCN225_ReinstallPreservesAbsoluteHookScriptReference
func TestSCN225_ReinstallPreservesAbsoluteHookScriptReference(t *testing.T) {
	// Scenario: Preserve non-executable absolute references during MCP normalization
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	project := filepath.Join(home, "project")
	hookPath := filepath.Join(home, ".config", "opencode", "plugin", "rotta-vela-freshness-guard.js")
	t.Setenv("HOME", home)
	t.Setenv("PATH", bin+":/bin")
	writeExecutable(t, filepath.Join(bin, "vela"), `#!/bin/sh
while [ "$#" -gt 0 ]; do
  case "$1" in --opencode-dir) shift; opencode_dir="$1" ;; esac
  shift
done
mkdir -p "$opencode_dir"
printf '{"plugin":["file://%s"],"mcp":{"vela":{"command":"/home/linuxbrew/.linuxbrew/Cellar/vela/4.5.6/bin/vela","args":["mcp"]}}}' "$HOME/.config/opencode/plugin/rotta-vela-freshness-guard.js" > "$opencode_dir/opencode.json"
`)

	if _, err := SetupVela(Options{Target: "opencode"}, home, project); err != nil {
		t.Fatalf("reinstall Vela: %v", err)
	}

	configPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	data := mustReadFile(t, configPath)
	if got := serializedMCPCommand(t, data, "vela"); got != "vela" {
		t.Fatalf("expected normalized Vela command vela, got %q", got)
	}
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("decode OpenCode config: %v", err)
	}
	plugins, _ := config["plugin"].([]interface{})
	if len(plugins) != 1 || plugins[0] != "file://"+hookPath {
		t.Fatalf("expected absolute generated hook reference to be preserved, got %#v", plugins)
	}
}

// REQ-016, REQ-019 → SCN-226 → TestSCN226_ReinstallPreservesAmbiguousMCPEntry
func TestSCN226_ReinstallPreservesAmbiguousMCPEntry(t *testing.T) {
	// Scenario: Preserve an ambiguous MCP entry rather than rewriting user configuration
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	project := filepath.Join(home, "project")
	configPath := filepath.Join(home, ".claude", "vela-mcp.json")
	t.Setenv("HOME", home)
	t.Setenv("PATH", bin+":/bin")
	writeExecutable(t, filepath.Join(bin, "vela"), `#!/bin/sh
mkdir -p "$HOME/.claude"
printf '{"command":"/usr/local/bin/vela","args":["serve","--private"]}' > "$HOME/.claude/vela-mcp.json"
`)

	result, err := SetupVela(Options{Target: "claude-code"}, home, project)
	if err != nil {
		t.Fatalf("reinstall Vela: %v", err)
	}
	if got := serializedMCPCommand(t, mustReadFile(t, configPath), ""); got != "/usr/local/bin/vela" {
		t.Fatalf("expected ambiguous command to remain unchanged, got %q", got)
	}
	skipped := result.SkippedAmbiguousMCPEntries
	if len(skipped) != 1 || skipped[0] != configPath {
		t.Fatalf("expected ambiguous entry %q to be reported as skipped, got %#v", configPath, result)
	}
}

// REQ-017 → SCN-227 → TestSCN227_SkipsNewMCPConfigurationWhenCommandUnavailable
func TestSCN227_SkipsNewMCPConfigurationWhenCommandUnavailable(t *testing.T) {
	// Scenario: Skip a new MCP configuration when command installation fails
	home := t.TempDir()
	project := filepath.Join(home, "project")
	configPath := filepath.Join(home, ".claude", "vela-mcp.json")
	t.Setenv("HOME", home)
	t.Setenv("PATH", filepath.Join(home, "empty-bin"))

	result, err := SetupVela(Options{Target: "claude-code"}, home, project)
	if err != nil {
		t.Fatalf("install with unavailable Vela command: %v", err)
	}
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("expected no new Vela MCP configuration, stat error: %v", err)
	}
	availability := result.MCPAvailability["claude-code"]["vela"]
	if availability.Status != MCPStatusSkipped || availability.Reason != "command availability" || availability.Remediation == "" {
		t.Fatalf("expected skipped command availability with remediation, got %#v", availability)
	}
}

// REQ-017 → SCN-227 → TestSCN227_UnavailableCommandDoesNotTreatExistingConfigAsNew
func TestSCN227_UnavailableCommandDoesNotTreatExistingConfigAsNew(t *testing.T) {
	// Scenario: Skip a new MCP configuration when command installation fails
	home := t.TempDir()
	configPath := filepath.Join(home, ".claude", "vela-mcp.json")
	writeTestFile(t, configPath, []byte(`{"command":"vela","args":["mcp"]}`))

	result := unavailableVelaMCPResult(Options{Target: "claude-code"}, home)
	status := result.MCPAvailability["claude-code"]["vela"]
	if status.Status != MCPStatusDegraded {
		t.Fatalf("expected existing configuration to be degraded rather than skipped, got %#v", status)
	}
	if status.Reason != "previous configuration preserved but not newly validated" {
		t.Fatalf("expected existing configuration preservation reason, got %#v", status)
	}
}

// REQ-017, REQ-020 → SCN-231 → TestSCN231_PreservesExistingMCPConfigurationWhenCommandUnavailable
func TestSCN231_PreservesExistingMCPConfigurationWhenCommandUnavailable(t *testing.T) {
	// Scenario: Preserve an existing MCP configuration when command installation fails
	home := t.TempDir()
	project := filepath.Join(home, "project")
	configPath := filepath.Join(home, ".claude", "vela-mcp.json")
	previous := []byte(`{"type":"stdio","command":"vela","args":["mcp"]}`)
	t.Setenv("HOME", home)
	t.Setenv("PATH", filepath.Join(home, "empty-bin"))
	writeTestFile(t, configPath, previous)

	result, err := Install(Options{Target: "claude-code", ProjectPath: project, SetupVela: true})
	if err != nil {
		t.Fatalf("reinstall with unavailable Vela command: %v", err)
	}
	if got := mustReadFile(t, configPath); string(got) != string(previous) {
		t.Fatalf("expected previous MCP configuration unchanged, got %s", got)
	} else if strings.Contains(string(got), "/") {
		t.Fatalf("expected no absolute fallback executable path, got %s", got)
	}
	availability := result.MCPStatuses["claude-code"]["vela"]
	if availability.Status != MCPStatusDegraded || availability.Reason != "previous configuration preserved but not newly validated" || availability.Remediation == "" {
		t.Fatalf("expected preserved-but-unvalidated status with remediation, got %#v", availability)
	}
}

// REQ-017, REQ-020 → SCN-231 → TestSCN231_PreservationKeepsRuntimeFallbackUnobserved
func TestSCN231_PreservationKeepsRuntimeFallbackUnobserved(t *testing.T) {
	// Scenario: Preserve an existing MCP configuration when command installation fails
	status := preservedVelaMCPStatus()
	if status.Status != MCPStatusDegraded || status.RuntimeFallback.State != MCPRuntimeFallbackNotObserved {
		t.Fatalf("expected preservation degradation without invented runtime fallback, got %#v", status)
	}
	if !strings.Contains(status.Remediation, "PATH") {
		t.Fatalf("expected command availability remediation, got %#v", status)
	}
}

// REQ-020 → SCN-232 → TestSCN232_RecordsOnlyAgentSetupFailures
func TestSCN232_RecordsOnlyAgentSetupFailures(t *testing.T) {
	// Scenario: Roll back only the failing coding agent installation
	result := &Result{Hosts: map[string]HostInstallResult{
		"claude-code": {Host: "claude-code", Status: HostInstallStatusInstalled},
	}}
	recordAgentSetupFailure(result, errors.New("unrelated failure"))
	if result.Hosts["claude-code"].Status != HostInstallStatusInstalled || result.Error != "" {
		t.Fatalf("expected unrelated errors not to alter completed agent status, got %#v", result)
	}
	setupErr := &agentSetupError{host: "opencode", err: errors.New("setup failed")}
	if !errors.Is(setupErr, setupErr.err) {
		t.Fatalf("expected setup error to preserve its cause")
	}
	if codingAgentName("claude-code") != "Claude Code" || codingAgentName("other") != "other" {
		t.Fatal("expected agent names to preserve known labels and unknown hosts")
	}
}

// REQ-020 → SCN-232 → TestSCN232_ReportsSerializationAndRollbackBoundaries
func TestSCN232_ReportsSerializationAndRollbackBoundaries(t *testing.T) {
	// Scenario: Roll back only the failing coding agent installation
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", bin+":/bin")
	writeExecutable(t, filepath.Join(bin, "ancora"), `#!/bin/sh
mkdir -p "$HOME/.claude/mcp"
printf 'not json' > "$HOME/.claude/mcp/ancora.json"
`)

	backup, err := createAgentBackup(Options{Target: "claude-code"}, "claude-code", home)
	if err != nil {
		t.Fatalf("create agent backup: %v", err)
	}
	if err := configureAncoraHosts(Options{Target: "claude-code"}, "ancora", home, map[string]string{"claude-code": backup}); err == nil {
		t.Fatal("expected invalid managed configuration serialization to fail")
	}
	if err := restoreFailedAncoraAgentSetup("claude-code", filepath.Join(home, "missing-backup"), errors.New("setup failed")); err == nil || !strings.Contains(err.Error(), "rollback failed") {
		t.Fatalf("expected rollback failure to remain observable, got %v", err)
	}

	blockedHome := t.TempDir()
	writeTestFile(t, filepath.Join(blockedHome, ".rotta"), []byte("not a directory"))
	if err := configureAncoraHosts(Options{Target: "claude-code"}, "ancora", blockedHome, nil); err == nil || !strings.Contains(err.Error(), "backup") {
		t.Fatalf("expected backup failure before setup, got %v", err)
	}
}

// REQ-020 → SCN-233 → TestSCN233_ReportsAgentBackupBoundaryFailures
func TestSCN233_ReportsAgentBackupBoundaryFailures(t *testing.T) {
	// Scenario: Roll back partial configuration changes within one coding agent
	home := t.TempDir()
	opts := Options{Target: "claude-code", ProjectPath: filepath.Join(home, "project")}

	t.Run("transaction backup path is a file", func(t *testing.T) {
		backupRoot := filepath.Join(home, "backup-file")
		writeTestFile(t, backupRoot, []byte("not a directory"))
		if _, err := createAgentBackups(opts, home, backupRoot); err == nil {
			t.Fatal("expected transaction backup path failure")
		}
	})

	t.Run("agent files path is a file", func(t *testing.T) {
		backupDir := filepath.Join(home, "agent-backup")
		writeTestFile(t, filepath.Join(backupDir, "files"), []byte("not a directory"))
		if _, err := createAgentBackupAt(opts, "claude-code", home, backupDir); err == nil {
			t.Fatal("expected agent backup files path failure")
		}
	})

	t.Run("selected source path is inaccessible", func(t *testing.T) {
		blockedHome := filepath.Join(home, "blocked-home")
		writeTestFile(t, filepath.Join(blockedHome, ".claude"), []byte("not a directory"))
		if _, err := createAgentBackupAt(opts, "claude-code", blockedHome, filepath.Join(home, "inaccessible-agent-backup")); err == nil {
			t.Fatal("expected inaccessible selected source path failure")
		}
	})

	t.Run("manifest destination is a directory", func(t *testing.T) {
		backupDir := filepath.Join(home, "manifest-directory")
		if err := os.MkdirAll(filepath.Join(backupDir, "manifest.json"), 0o750); err != nil {
			t.Fatal(err)
		}
		if _, err := createAgentBackupAt(opts, "claude-code", home, backupDir); err == nil {
			t.Fatal("expected manifest write failure")
		}
	})
}

// REQ-017, REQ-020 → SCN-231 → TestSCN231_ReportsUnavailableRestoreAndManifestBoundaries
func TestSCN231_ReportsUnavailableRestoreAndManifestBoundaries(t *testing.T) {
	// Scenario: Preserve an existing MCP configuration when command installation fails
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", filepath.Join(home, "empty-bin"))
	writeTestFile(t, filepath.Join(home, ".claude", "vela-mcp.json"), []byte(`{"command":"vela"}`))

	result := &Result{BackupDir: filepath.Join(home, "missing-backup")}
	if err := setupVela(Options{Target: "claude-code", SetupVela: true}, result, home, filepath.Join(home, "project")); err == nil || !strings.Contains(err.Error(), "restore previous Vela configuration") {
		t.Fatalf("expected failed preservation restore to be reported, got %v", err)
	}

	availability := &VelaResult{MCPAvailability: map[string]map[string]MCPStatusResult{
		"claude-code": {"vela": unavailableVelaMCPStatus(MCPStatusSkipped)},
	}}
	markBackedUpVelaConfigurations(availability, filepath.Join(home, "missing-manifest"), home)
	if velaConfigurationNeedsRestore(availability) {
		t.Fatalf("expected skipped availability not to request restore, got %#v", availability)
	}
}

// REQ-018 → SCN-228 → TestSCN228_OpenCodeReportsUnverifiedHostCommandResolution
func TestSCN228_OpenCodeReportsUnverifiedHostCommandResolution(t *testing.T) {
	// Scenario: Distinguish OpenCode PATH uncertainty from installer command availability
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", bin)
	writeContext7StrictFakeNPX(t, filepath.Join(bin, "npx"), true, []string{"resolve-library-id", "query-docs"})

	result, err := Install(Options{Target: "opencode", ProjectPath: filepath.Join(home, "project"), SetupContext7: true})
	if err != nil {
		t.Fatalf("install Context7 for OpenCode: %v", err)
	}

	configPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	if got := serializedMCPCommand(t, mustReadFile(t, configPath), "context7"); got != "npx" {
		t.Fatalf("expected OpenCode to serialize bare npx, got %q", got)
	}
	status := result.MCPStatuses["opencode"]["context7"]
	if status.Status != MCPStatusDegraded || status.Reason != "portable-but-host-resolution-unverified" || !strings.Contains(status.Remediation, "OpenCode") || !strings.Contains(status.Remediation, "npx") || !strings.Contains(status.Remediation, "PATH") {
		t.Fatalf("expected unverified OpenCode host resolution with PATH remediation, got %#v", status)
	}
}

// REQ-018, REQ-019 → SCN-229 → TestSCN229_ReportsHostCommandLookupFailure
func TestSCN229_ReportsHostCommandLookupFailure(t *testing.T) {
	// Scenario: Report a host-side command lookup failure without masking it
	home := t.TempDir()
	configPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	writeTestFile(t, configPath, []byte(`{"mcp":{"context7":{"type":"local","command":["npx","-y","@upstash/context7-mcp"],"enabled":true}}}`))

	result := &Result{Hosts: map[string]HostInstallResult{
		"opencode": {Host: "opencode", Status: HostInstallStatusInstalled},
	}}
	recordMCPHealthFailure(result, Options{Target: "opencode", SetupContext7: true}, "mcp:context7", Context7HealthResult{
		Category: Context7FailureCommandUnavailable,
		Message:  "host process could not find npx on PATH",
	})
	recordMCPStatuses(result, Options{Target: "opencode", SetupContext7: true})

	capability := result.Hosts["opencode"].Capabilities["mcp:context7"]
	if capability.Status != HostCapabilityStatusFailed || !strings.Contains(capability.Reason, "host command availability") || !strings.Contains(capability.Remediation, "PATH") {
		t.Fatalf("expected failed host command availability with PATH remediation, got %#v", capability)
	}
	if status := result.MCPStatuses["opencode"]["context7"]; status.Status != MCPStatusFailed {
		t.Fatalf("expected host command lookup failure never to be healthy, got %#v", status)
	}
	if got := serializedMCPCommand(t, mustReadFile(t, configPath), "context7"); got != "npx" {
		t.Fatalf("expected portable npx command to remain serialized, got %q", got)
	}
}

// REQ-019 → SCN-230 → TestSCN230_ReinstallRetainsBareCommandsAfterExecutableUpgrades
func TestSCN230_ReinstallRetainsBareCommandsAfterExecutableUpgrades(t *testing.T) {
	// Scenario: Retain portable managed commands after an executable upgrade
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	project := filepath.Join(home, "project")
	t.Setenv("HOME", home)
	t.Setenv("PATH", bin+":/bin")

	// The upgraded executables reproduce their setup output with their new,
	// installation-specific locations.
	writeExecutable(t, filepath.Join(bin, "ancora"), `#!/bin/sh
case "$2" in
  claude-code) mkdir -p "$HOME/.claude/mcp"; printf '{"command":"/opt/homebrew/Cellar/ancora/2.0.0/bin/ancora","args":["mcp"]}' > "$HOME/.claude/mcp/ancora.json" ;;
  opencode) mkdir -p "$HOME/.config/opencode"; printf '{"mcp":{"ancora":{"command":"/opt/homebrew/Cellar/ancora/2.0.0/bin/ancora","args":["mcp"]}}}' > "$HOME/.config/opencode/opencode.jsonc" ;;
esac
`)
	writeExecutable(t, filepath.Join(bin, "vela"), `#!/bin/sh
agent=""
while [ "$#" -gt 0 ]; do
  case "$1" in --agent) shift; agent="$1" ;; esac
  shift
done
case "$agent" in
  claude) mkdir -p "$HOME/.claude"; printf '{"command":"/home/user/.local/bin/vela","args":["mcp"]}' > "$HOME/.claude/vela-mcp.json" ;;
  opencode) mkdir -p "$HOME/.config/opencode"; printf '{"mcp":{"vela":{"command":"/home/linuxbrew/.linuxbrew/Cellar/vela/9.0.0/bin/vela","args":["mcp"]}}}' > "$HOME/.config/opencode/opencode.json" ;;
esac
`)

	if _, err := SetupAncora(Options{Target: "both"}, home); err != nil {
		t.Fatalf("reinstall after Homebrew Ancora upgrade: %v", err)
	}
	if _, err := SetupVela(Options{Target: "both"}, home, project); err != nil {
		t.Fatalf("reinstall after curl/manual Vela upgrade: %v", err)
	}

	context7 := Context7ServerConfig()
	if err := writeOpenCodeContext7MCP(filepath.Join(home, ".config", "opencode", "context7.json"), context7); err != nil {
		t.Fatalf("write initial OpenCode Context7 config: %v", err)
	}
	if err := writeClaudeContext7MCP(filepath.Join(home, ".claude", "mcp", "context7.json"), context7); err != nil {
		t.Fatalf("write initial Claude Context7 config: %v", err)
	}
	if _, err := ConfigureContext7(Options{SetupContext7: true}, home); err != nil {
		t.Fatalf("reinstall after manual Context7 command upgrade: %v", err)
	}

	for path, want := range map[string]struct{ command, server string }{
		filepath.Join(home, ".claude", "mcp", "ancora.json"):         {"ancora", ""},
		filepath.Join(home, ".config", "opencode", "opencode.jsonc"): {"ancora", "ancora"},
		filepath.Join(home, ".claude", "vela-mcp.json"):              {"vela", ""},
		filepath.Join(home, ".config", "opencode", "opencode.json"):  {"vela", "vela"},
		filepath.Join(home, ".config", "opencode", "context7.json"):  {"npx", "context7"},
		filepath.Join(home, ".claude", "mcp", "context7.json"):       {"npx", ""},
	} {
		data := mustReadFile(t, path)
		got := serializedMCPCommand(t, data, want.server)
		if got != want.command {
			t.Errorf("expected %s to retain bare command %q after upgrade, got %q", path, want.command, got)
		}
		if strings.Contains(got, "/") {
			t.Errorf("expected %s not to retain an absolute binary location, got command %q", path, got)
		}
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func serializedMCPCommand(t *testing.T, data []byte, server string) string {
	t.Helper()
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("decode MCP config: %v", err)
	}
	if server != "" {
		mcp, _ := config["mcp"].(map[string]interface{})
		config, _ = mcp[server].(map[string]interface{})
	}
	if command, ok := config["command"].(string); ok {
		return command
	}
	command, _ := config["command"].([]interface{})
	if len(command) == 0 {
		return ""
	}
	value, _ := command[0].(string)
	return value
}
