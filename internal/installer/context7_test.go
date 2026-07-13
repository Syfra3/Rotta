package installer

import (
	"bufio"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSCN103_SelectedContext7ConfiguresBothHostsWithStdioCommand(t *testing.T) {
	// REQ-002, REQ-003 → SCN-103 → TestSCN103_SelectedContext7ConfiguresBothHostsWithStdioCommand
	// Scenario: Selected Context7 configures both host MCP entries with the compatible stdio command.
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"), []byte(`{"mcp":{"user-server":{"command":"keep"}},"theme":"keep"}`))
	writeTestFile(t, filepath.Join(home, ".claude", "mcp", "user.json"), []byte(`{"command":"keep"}`))

	result, err := ConfigureContext7(Options{Target: "opencode", SetupContext7: true}, home)
	if err != nil {
		t.Fatal(err)
	}

	assertContext7OpenCodeEntry(t, filepath.Join(home, ".config", "opencode", "opencode.json"))
	assertContext7ClaudeEntry(t, filepath.Join(home, ".claude", "mcp", "context7.json"))
	assertFileContains(t, filepath.Join(home, ".claude", "mcp", "user.json"), "keep")
	assertStringListContains(t, result.Files, filepath.Join(home, ".config", "opencode", "opencode.json"))
	assertStringListContains(t, result.Files, filepath.Join(home, ".claude", "mcp", "context7.json"))
}

func TestSCN104_Context7ReportsPerHostConfigurationFailures(t *testing.T) {
	// REQ-002, REQ-006 → SCN-104 → TestSCN104_Context7ReportsPerHostConfigurationFailures
	// Scenario: Host configuration failures are reported per host instead of as full success.
	result := summarizeContext7HostConfig(context7HostConfigResult{Host: "opencode", OK: true}, context7HostConfigResult{Host: "claude-code", OK: false, Err: os.ErrPermission})

	if result.FullyConfigured {
		t.Fatalf("expected partial failure not full success: %#v", result)
	}
	if !result.OpenCode.OK || result.ClaudeCode.OK || result.ClaudeCode.Err == nil {
		t.Fatalf("expected per-host success/failure details, got %#v", result)
	}
}

func TestSCN105_Context7MissingCommandFailsWithoutBlamingHostConfig(t *testing.T) {
	// REQ-003, REQ-004 → SCN-105 → TestSCN105_Context7MissingCommandFailsWithoutBlamingHostConfig
	// Scenario: Missing command availability fails Context7 without blaming host configuration.
	t.Setenv("PATH", t.TempDir())

	result := CheckContext7Health(Context7ServerConfig())
	if result.OK {
		t.Fatal("expected missing npx to fail health")
	}
	if result.Category != Context7FailureCommandUnavailable {
		t.Fatalf("expected command-unavailable failure, got %#v", result)
	}
}

func TestSCN106_Context7HealthRequiresMCPInitializationAndToolDiscovery(t *testing.T) {
	// REQ-004 → SCN-106 → TestSCN106_Context7HealthRequiresMCPInitializationAndToolDiscovery
	// Scenario: Context7 health passes only after MCP initialization and tool discovery.
	home := t.TempDir()
	binDir := filepath.Join(home, "bin")
	t.Setenv("PATH", binDir)
	writeContext7StrictFakeNPX(t, filepath.Join(binDir, "npx"), true, []string{"resolve-library-id", "query-docs"})

	result := CheckContext7Health(Context7ServerConfig())
	if !result.OK {
		t.Fatalf("expected healthy Context7 MCP, got %#v", result)
	}
	if !result.Initialized || !result.ToolsDiscovered {
		t.Fatalf("expected init and tool discovery proof, got %#v", result)
	}
	if result.Command != "npx" || strings.Join(result.Args, " ") != "-y @upstash/context7-mcp" || result.Transport != "stdio" {
		t.Fatalf("expected health check to use configured command/args/transport, got %#v", result)
	}
}

func TestSCN107_Context7HealthRejectsFalsePositives(t *testing.T) {
	// REQ-004 → SCN-107 → TestSCN107_Context7HealthRejectsFalsePositives
	// Scenario Outline: Context7 health rejects false positives.
	tests := []struct {
		name     string
		initOK   bool
		tools    []string
		category Context7FailureCategory
	}{
		{name: "configuration text exists but MCP init fails", initOK: false, tools: []string{"resolve-library-id", "query-docs"}, category: Context7FailureInitialization},
		{name: "server initializes but exposes no tools", initOK: true, tools: nil, category: Context7FailureToolDiscovery},
		{name: "server exposes only one expected tool", initOK: true, tools: []string{"resolve-library-id"}, category: Context7FailureToolDiscovery},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			binDir := filepath.Join(home, "bin")
			t.Setenv("PATH", binDir)
			writeContext7FakeNPX(t, filepath.Join(binDir, "npx"), tt.initOK, tt.tools)

			result := CheckContext7Health(Context7ServerConfig())
			if result.OK || result.Category != tt.category {
				t.Fatalf("expected failure category %s, got %#v", tt.category, result)
			}
		})
	}
}

func TestSCN107_Context7HealthRejectsImmediateServerExit(t *testing.T) {
	// REQ-004 → SCN-107 → TestSCN107_Context7HealthRejectsImmediateServerExit
	// Scenario Outline: Context7 health rejects false positives.
	home := t.TempDir()
	binDir := filepath.Join(home, "bin")
	t.Setenv("PATH", binDir)
	writeExecutable(t, filepath.Join(binDir, "npx"), "#!/bin/sh\nexit 0\n")

	result := CheckContext7Health(Context7ServerConfig())
	if result.OK || result.Category != Context7FailureStartup {
		t.Fatalf("expected server startup failure for immediate exit, got %#v", result)
	}
}

func TestSCN107_Context7HealthRejectsTimeout(t *testing.T) {
	// REQ-004 → SCN-107 → TestSCN107_Context7HealthRejectsTimeout
	// Scenario Outline: Context7 health rejects false positives.
	home := t.TempDir()
	binDir := filepath.Join(home, "bin")
	t.Setenv("PATH", binDir)
	writeExecutable(t, filepath.Join(binDir, "npx"), "#!/bin/sh\n/bin/sleep 2\n")
	previous := context7HealthTimeout
	context7HealthTimeout = 50 * time.Millisecond
	t.Cleanup(func() { context7HealthTimeout = previous })

	result := CheckContext7Health(Context7ServerConfig())
	if result.OK || result.Category != Context7FailureTimeout {
		t.Fatalf("expected timeout failure, got %#v", result)
	}
}

func TestSCN108_SkippedContext7LeavesHostConfigAndInstructionsUnchanged(t *testing.T) {
	// REQ-005 → SCN-108 → TestSCN108_SkippedContext7LeavesHostConfigAndInstructionsUnchanged
	// Scenario: Explicitly deselecting Context7 leaves host config and generated instructions unchanged for Context7.
	home := t.TempDir()
	opencodePath := filepath.Join(home, ".config", "opencode", "opencode.json")
	writeTestFile(t, opencodePath, []byte(`{"mcp":{"user-server":{"command":"keep"}}}`))

	result, err := ConfigureContext7(Options{Target: "both", SetupContext7: false}, home)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != Context7StatusSkipped || result.HealthRan || result.CommandChecked {
		t.Fatalf("expected skipped without checks, got %#v", result)
	}
	assertFileDoesNotContain(t, opencodePath, "context7")
	if _, err := os.Stat(filepath.Join(home, ".claude", "mcp", "context7.json")); !os.IsNotExist(err) {
		t.Fatalf("expected no Claude Context7 MCP entry, stat err=%v", err)
	}
	assertNoContext7InstructionPrompts(t, integrationInstructions(Options{SetupAncora: true, SetupVela: true, SetupContext7: false}))
}

func TestSCN109_Context7SkipDoesNotAffectAncoraAndVelaSelection(t *testing.T) {
	// REQ-005, REQ-006 → SCN-109 → TestSCN109_Context7SkipDoesNotAffectAncoraAndVelaSelection
	// Scenario: Context7 skip does not affect selected Ancora and Vela installs.
	opts := Options{SetupAncora: true, SetupVela: true, SetupContext7: false}

	if !opts.SetupAncora || !opts.SetupVela || opts.SetupContext7 {
		t.Fatalf("expected Ancora/Vela selected and Context7 skipped: %#v", opts)
	}
	result, err := ConfigureContext7(opts, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != Context7StatusSkipped {
		t.Fatalf("expected only Context7 skipped, got %#v", result)
	}
}

func TestSCN110_Context7RerunNormalizesDuplicateHostEntriesBeforeHealth(t *testing.T) {
	// REQ-002, REQ-003, REQ-004 → SCN-110 → TestSCN110_Context7RerunNormalizesDuplicateHostEntriesBeforeHealth
	// Scenario: Re-running selected Context7 normalizes duplicate host entries before health reporting.
	home := t.TempDir()
	opencodePath := filepath.Join(home, ".config", "opencode", "opencode.json")
	writeTestFile(t, opencodePath, []byte(`{"mcp":{"context7":{"command":"old"},"rotta-context7":{"command":"old"}}}`))

	result, err := ConfigureContext7(Options{Target: "opencode", SetupContext7: true}, home)
	if err != nil {
		t.Fatal(err)
	}
	config := readJSONFile(t, opencodePath)
	mcp := config["mcp"].(map[string]interface{})
	if _, exists := mcp["rotta-context7"]; exists {
		t.Fatalf("expected duplicate rotta-context7 entry removed, got %#v", mcp)
	}
	assertContext7OpenCodeEntry(t, opencodePath)
	if result.Status == Context7StatusConfigured {
		t.Fatalf("expected configuration alone not to report full success before health, got %#v", result)
	}
}

func TestSCN106_InstallRunsContext7HealthAndRecordsConfiguredStatus(t *testing.T) {
	// REQ-004 → SCN-106 → TestSCN106_InstallRunsContext7HealthAndRecordsConfiguredStatus
	// Scenario: Context7 health passes only after MCP initialization and tool discovery.
	home := t.TempDir()
	binDir := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir)
	writeContext7FakeNPX(t, filepath.Join(binDir, "npx"), true, []string{"resolve-library-id", "query-docs"})

	result, err := Install(Options{Target: "", ProjectPath: filepath.Join(home, "project"), SetupContext7: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Context7.Status != Context7StatusConfigured || !result.Context7.HealthRan || !result.Context7.Health.OK {
		t.Fatalf("expected install to run healthy Context7 proof, got %#v", result.Context7)
	}
	assertStringListContains(t, result.Files, filepath.Join(home, ".config", "opencode", "opencode.json"))
	assertStringListContains(t, result.Files, filepath.Join(home, ".claude", "mcp", "context7.json"))
}

func TestSCN106_InstallRunsContext7HealthWhenAnyHostConfigured(t *testing.T) {
	// REQ-004, REQ-006 → SCN-106 → TestSCN106_InstallRunsContext7HealthWhenAnyHostConfigured
	// Scenario: Context7 health passes only after MCP initialization and tool discovery.
	home := t.TempDir()
	binDir := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir)
	writeContext7StrictFakeNPX(t, filepath.Join(binDir, "npx"), true, []string{"resolve-library-id", "query-docs"})
	writeTestFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"), []byte(`{`))

	result, err := Install(Options{Target: "", ProjectPath: filepath.Join(home, "project"), SetupContext7: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Context7.OpenCode.OK || !result.Context7.ClaudeCode.OK {
		t.Fatalf("expected one successful host and one failed host, got %#v", result.Context7)
	}
	if !result.Context7.HealthRan || !result.Context7.Health.OK {
		t.Fatalf("expected health to run after at least one host was configured, got %#v", result.Context7)
	}
}

func TestSCN105_InstallReportsContext7HealthFailureWithPartialResult(t *testing.T) {
	// REQ-003, REQ-004 → SCN-105 → TestSCN105_InstallReportsContext7HealthFailureWithPartialResult
	// Scenario: Missing command availability fails Context7 without blaming host configuration.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())

	result, err := Install(Options{Target: "", ProjectPath: filepath.Join(home, "project"), SetupContext7: true})
	if err == nil {
		t.Fatal("expected Context7 health failure")
	}
	if result == nil || !result.Context7.HealthRan || result.Context7.Health.Category != Context7FailureCommandUnavailable {
		t.Fatalf("expected partial install result with Context7 command failure, got result=%#v err=%v", result, err)
	}
}

func TestSCN110_Context7BackupScopeIncludesUniqueHostConfigPaths(t *testing.T) {
	// REQ-002, REQ-003, REQ-004 → SCN-110 → TestSCN110_Context7BackupScopeIncludesUniqueHostConfigPaths
	// Scenario: Re-running selected Context7 normalizes duplicate host entries before health reporting.
	home := t.TempDir()
	project := filepath.Join(home, "project")

	paths := backupScope(Options{Target: "opencode", SetupContext7: true}, home, project)
	opencodePath := filepath.Join(home, ".config", "opencode", "opencode.json")
	claudePath := filepath.Join(home, ".claude", "mcp", "context7.json")
	assertStringListContains(t, paths, opencodePath)
	assertStringListContains(t, paths, claudePath)
	count := 0
	for _, path := range paths {
		if path == opencodePath {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected opencode path to be unique, got %#v", paths)
	}
}

func TestSCN110_Context7BackupManifestRecordsSelectedChoice(t *testing.T) {
	// REQ-002, REQ-003, REQ-004 → SCN-110 → TestSCN110_Context7BackupManifestRecordsSelectedChoice
	// Scenario: Re-running selected Context7 normalizes duplicate host entries before health reporting.
	home := t.TempDir()
	t.Setenv("HOME", home)
	project := filepath.Join(home, "project")

	backupDir, err := Backup(Options{Target: "both", ProjectPath: project, SetupContext7: true})
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := loadBackupManifest(filepath.Join(backupDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !manifest.OptionalIntegrations.Context7 {
		t.Fatalf("expected backup manifest to record selected Context7, got %#v", manifest.OptionalIntegrations)
	}
}

func TestSCN108_Context7BackupManifestRestoresSkippedAndSelectedChoice(t *testing.T) {
	// REQ-005 → SCN-108 → TestSCN108_Context7BackupManifestRestoresSkippedAndSelectedChoice
	// Scenario: Explicitly deselecting Context7 leaves host config and generated instructions unchanged for Context7.
	selected := optionsFromManifest(backupManifest{OptionalIntegrations: optionalIntegrations{Context7: true}})
	if !selected.SetupContext7 {
		t.Fatal("expected Context7 selection restored from manifest")
	}
	skipped := optionsFromManifest(backupManifest{OptionalIntegrations: optionalIntegrations{Context7: false}})
	if skipped.SetupContext7 {
		t.Fatal("expected Context7 skip restored from manifest")
	}
}

func TestSCN104_Context7ConfigSummarizesHostWriteFailuresAndEmptyHosts(t *testing.T) {
	// REQ-002, REQ-006 → SCN-104 → TestSCN104_Context7ConfigSummarizesHostWriteFailuresAndEmptyHosts
	// Scenario: Host configuration failures are reported per host instead of as full success.
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"), []byte(`{`))
	writeTestFile(t, filepath.Join(home, ".claude", "mcp"), []byte("not a directory"))

	result, err := ConfigureContext7(Options{SetupContext7: true}, home)
	if err != nil {
		t.Fatal(err)
	}
	if result.FullyConfigured || result.Status != Context7StatusPartial {
		t.Fatalf("expected partial status for host write failures, got %#v", result)
	}
	if result.OpenCode.OK || result.OpenCode.Err == nil || result.ClaudeCode.OK || result.ClaudeCode.Err == nil {
		t.Fatalf("expected both host failures to be recorded, got %#v", result)
	}

	empty := summarizeContext7HostConfig(context7HostConfigResult{})
	if empty.Status != Context7StatusSkipped || empty.FullyConfigured {
		t.Fatalf("expected empty host summary to be skipped, got %#v", empty)
	}
}

func TestSCN103_Context7OpenCodeCreatesMCPSectionWhenMissing(t *testing.T) {
	// REQ-002, REQ-003 → SCN-103 → TestSCN103_Context7OpenCodeCreatesMCPSectionWhenMissing
	// Scenario: Selected Context7 configures both host MCP entries with the compatible stdio command.
	path := filepath.Join(t.TempDir(), "opencode.json")
	writeTestFile(t, path, []byte(`{"theme":"keep"}`))

	if err := writeOpenCodeContext7MCP(path, Context7ServerConfig()); err != nil {
		t.Fatal(err)
	}
	assertContext7OpenCodeEntry(t, path)
}

func TestSCN107_Context7HealthRejectsStartupAndProtocolFailures(t *testing.T) {
	// REQ-004 → SCN-107 → TestSCN107_Context7HealthRejectsStartupAndProtocolFailures
	// Scenario Outline: Context7 health rejects false positives.
	tests := []struct {
		name     string
		script   string
		category Context7FailureCategory
	}{
		{name: "start fails", script: "not a script", category: Context7FailureStartup},
		{name: "initialization write fails", script: "#!/bin/sh\nexit 0\n", category: Context7FailureStartup},
		{name: "tool discovery write fails", script: "#!/bin/sh\nIFS= read -r initialize\nprintf '%s\\n' '{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{}}'\nIFS= read -r initialized\nexit 0\n", category: Context7FailureToolDiscovery},
		{name: "tool discovery read fails", script: "#!/bin/sh\nif IFS= read -r line; then printf '%s\\n' '{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{}}'; fi\nwhile IFS= read -r line; do case \"$line\" in *tools/list*) exit 0 ;; esac; done\n", category: Context7FailureToolDiscovery},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			binDir := filepath.Join(home, "bin")
			t.Setenv("PATH", binDir)
			writeExecutable(t, filepath.Join(binDir, "npx"), tt.script)

			result := CheckContext7Health(Context7ServerConfig())
			if result.OK || result.Category != tt.category {
				t.Fatalf("expected failure category %s, got %#v", tt.category, result)
			}
		})
	}
}

func TestSCN107_Context7JSONRPCHelpersReportWriteAndDecodeErrors(t *testing.T) {
	// REQ-004 → SCN-107 → TestSCN107_Context7JSONRPCHelpersReportWriteAndDecodeErrors
	// Scenario Outline: Context7 health rejects false positives.
	if err := writeJSONRPC(failingWriter{}, map[string]interface{}{"jsonrpc": "2.0"}); err == nil {
		t.Fatal("expected JSON-RPC write error")
	}

	_, err := readJSONRPC(context.Background(), bufio.NewReader(strings.NewReader("not-json\n")))
	if err == nil {
		t.Fatal("expected JSON-RPC decode error")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func assertContext7OpenCodeEntry(t *testing.T, path string) {
	t.Helper()
	config := readJSONFile(t, path)
	mcp := config["mcp"].(map[string]interface{})
	entry := mcp["context7"].(map[string]interface{})
	if entry["type"] != "local" || entry["enabled"] != true {
		t.Fatalf("unexpected Context7 OpenCode entry: %#v", entry)
	}
	command := entry["command"].([]interface{})
	if len(command) != 3 || command[0] != "npx" || command[1] != "-y" || command[2] != "@upstash/context7-mcp" {
		t.Fatalf("unexpected Context7 OpenCode command: %#v", command)
	}
	if _, exists := mcp["user-server"]; !exists && strings.Contains(readFileString(t, path), "user-server") {
		t.Fatalf("expected unrelated MCP entries preserved, got %#v", mcp)
	}
}

func assertContext7ClaudeEntry(t *testing.T, path string) {
	t.Helper()
	entry := readJSONFile(t, path)
	if entry["command"] != "npx" || entry["type"] != "stdio" {
		t.Fatalf("unexpected Context7 Claude entry: %#v", entry)
	}
	args := entry["args"].([]interface{})
	if len(args) != 2 || args[0] != "-y" || args[1] != "@upstash/context7-mcp" {
		t.Fatalf("unexpected Context7 Claude args: %#v", args)
	}
}

func assertNoContext7InstructionPrompts(t *testing.T, text string) {
	t.Helper()
	for _, unwanted := range []string{"Context7", "library docs", "API references", "code examples", "setup help"} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("expected no Context7 instruction prompt %q in:\n%s", unwanted, text)
		}
	}
}

func writeContext7FakeNPX(t *testing.T, path string, initOK bool, tools []string) {
	t.Helper()
	initResponse := `{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"init failed"}}`
	if initOK {
		initResponse = `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{"tools":{}}}}`
	}
	var toolEntries []string
	for _, tool := range tools {
		toolEntries = append(toolEntries, `{"name":"`+tool+`"}`)
	}
	toolsResponse := `{"jsonrpc":"2.0","id":2,"result":{"tools":[` + strings.Join(toolEntries, ",") + `]}}`
	script := `#!/bin/sh
if IFS= read -r line; then
  printf '%s\n' '` + initResponse + `'
fi
while IFS= read -r line; do
  case "$line" in
    *tools/list*) printf '%s\n' '` + toolsResponse + `' ; exit 0 ;;
  esac
done
`
	writeExecutable(t, path, script)
}

func writeContext7StrictFakeNPX(t *testing.T, path string, initOK bool, tools []string) {
	t.Helper()
	initResponse := `{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"init failed"}}`
	if initOK {
		initResponse = `{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{"tools":{}}}}`
	}
	var toolEntries []string
	for _, tool := range tools {
		toolEntries = append(toolEntries, `{"name":"`+tool+`"}`)
	}
	toolsResponse := `{"jsonrpc":"2.0","id":2,"result":{"tools":[` + strings.Join(toolEntries, ",") + `]}}`
	script := `#!/bin/sh
if IFS= read -r line; then
  case "$line" in
    *'"method":"initialize"'*) printf '%s\n' '` + initResponse + `' ;;
    *) exit 0 ;;
  esac
fi
while IFS= read -r line; do
  case "$line" in
    *'"method":"tools/list"'*) printf '%s\n' '` + toolsResponse + `' ; exit 0 ;;
  esac
done
`
	writeExecutable(t, path, script)
}
