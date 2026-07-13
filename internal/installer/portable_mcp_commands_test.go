package installer

import (
	"encoding/json"
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

// REQ-017 → SCN-227 → TestSCN227_ReinstallReportsUnavailableManagedCommand
func TestSCN227_ReinstallReportsUnavailableManagedCommand(t *testing.T) {
	// Scenario: Report an unavailable installer command without serializing a fallback path
	home := t.TempDir()
	project := filepath.Join(home, "project")
	configPath := filepath.Join(home, ".claude", "vela-mcp.json")
	t.Setenv("HOME", home)
	t.Setenv("PATH", filepath.Join(home, "empty-bin"))
	writeTestFile(t, configPath, []byte(`{"command":"/home/linuxbrew/.linuxbrew/Cellar/vela/4.5.6/bin/vela","args":["mcp"]}`))

	result, err := SetupVela(Options{Target: "claude-code"}, home, project)
	if err != nil {
		t.Fatalf("reinstall with unavailable Vela command: %v", err)
	}
	if got := serializedMCPCommand(t, mustReadFile(t, configPath), ""); got != "vela" {
		t.Fatalf("expected stale command to normalize to bare vela, got %q", got)
	}
	if strings.Contains(string(mustReadFile(t, configPath)), "/home/linuxbrew/.linuxbrew/") {
		t.Fatal("expected no absolute fallback executable path")
	}
	availability := result.MCPAvailability["claude-code"]["vela"]
	if availability.Status != MCPStatusDegraded || availability.Reason != "command availability" || availability.Remediation == "" {
		t.Fatalf("expected degraded command availability with remediation, got %#v", availability)
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
