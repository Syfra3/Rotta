package installer

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSCN001_InstallCreatesTimestampedBackupBeforeMutation(t *testing.T) {
	// REQ-001 → SCN-001 → TestSCN001_InstallCreatesTimestampedBackupBeforeMutation
	// Scenario: Install creates a timestamped backup before any mutation
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	t.Setenv("HOME", home)

	preInstallOpenCodeConfig := []byte(`{"agent":{"user-agent":{"description":"keep"}}}`)
	writeTestFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"), preInstallOpenCodeConfig)
	writeTestFile(t, filepath.Join(home, ".claude", "settings.json"), []byte(`{"permissions":{"allow":["user-permission"]}}`))
	writeTestFile(t, filepath.Join(projectPath, ".rotta", "state-machine.yaml"), []byte("previous: state\n"))

	result, err := Install(Options{
		Target:        "both",
		ProjectPath:   projectPath,
		InstallSpec:   true,
		InstallImpl:   true,
		InstallReview: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.BackupDir == "" {
		t.Fatal("expected install result to include backup directory")
	}
	if filepath.Dir(result.BackupDir) != filepath.Join(home, ".rotta", "backups") {
		t.Fatalf("expected backup under ~/.rotta/backups, got %s", result.BackupDir)
	}

	manifest := readBackupManifest(t, filepath.Join(result.BackupDir, "manifest.json"))
	if manifest.ProjectPath != projectPath {
		t.Fatalf("expected project path %q, got %q", projectPath, manifest.ProjectPath)
	}
	if manifest.Target != "both" {
		t.Fatalf("expected target both, got %q", manifest.Target)
	}
	if !manifest.SelectedModes.Spec || !manifest.SelectedModes.Implementation || !manifest.SelectedModes.Review {
		t.Fatalf("expected selected modes in manifest, got %#v", manifest.SelectedModes)
	}
	if manifest.OptionalIntegrations.Ancora || manifest.OptionalIntegrations.Vela {
		t.Fatalf("expected disabled optional integrations in manifest, got %#v", manifest.OptionalIntegrations)
	}
	assertManifestContainsPath(t, manifest.BackedUpPaths, filepath.Join(home, ".config", "opencode", "opencode.json"))
	assertManifestContainsPath(t, manifest.BackedUpPaths, filepath.Join(home, ".claude", "settings.json"))
	assertManifestContainsPath(t, manifest.MissingPaths, filepath.Join(projectPath, ".vela", "graph.db"))

	backupFile := filepath.Join(result.BackupDir, "files", "home", ".config", "opencode", "opencode.json")
	backupContent, err := os.ReadFile(backupFile)
	if err != nil {
		t.Fatalf("read backed-up opencode config: %v", err)
	}
	if string(backupContent) != string(preInstallOpenCodeConfig) {
		t.Fatalf("backup should capture pre-install content before mutation, got %s", backupContent)
	}
}

func TestSCN002_SuccessfulInstallCleansPreviousSettingsBeforeFreshInstall(t *testing.T) {
	// REQ-004 → SCN-002 → TestSCN002_SuccessfulInstallCleansPreviousSettingsBeforeFreshInstall
	// Scenario: Successful install cleans previous rotta settings before fresh install
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	t.Setenv("HOME", home)

	writeTestFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"), []byte(`{"agent":{"rotta-spec":{"description":"stale","mode":"subagent","prompt":"old"},"rotta-impl":{"description":"remove me"},"user-agent":{"description":"keep me"}},"theme":"user-theme"}`))
	writeTestFile(t, filepath.Join(home, ".config", "opencode", "skills", "rotta-impl", "SKILL.md"), []byte("stale impl skill\n"))
	writeTestFile(t, filepath.Join(home, ".config", "opencode", "skills", "user-skill", "SKILL.md"), []byte("keep user skill\n"))
	writeTestFile(t, filepath.Join(home, ".claude", "skills", "rotta", "implementation-mode", "SKILL.md"), []byte("stale claude impl skill\n"))
	writeTestFile(t, filepath.Join(home, ".claude", "skills", "user-skill", "SKILL.md"), []byte("keep claude user skill\n"))
	writeTestFile(t, filepath.Join(home, ".claude", "settings.json"), []byte(`{"permissions":{"allow":["mcp__clean_workflow__implementation_mode","user-permission"]},"theme":"dark"}`))
	writeTestFile(t, filepath.Join(projectPath, ".rotta", "state-machine.yaml"), []byte("stale: true\n"))

	result, err := Install(Options{
		Target:      "both",
		ProjectPath: projectPath,
		InstallSpec: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.BackupDir == "" {
		t.Fatal("expected install to preserve backup-first behavior")
	}

	opencodeConfig := readJSONFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"))
	agents := opencodeConfig["agent"].(map[string]interface{})
	if _, ok := agents["user-agent"]; !ok {
		t.Fatalf("expected unrelated opencode agent to be preserved, got %#v", agents)
	}
	if _, ok := opencodeConfig["theme"]; !ok {
		t.Fatalf("expected unrelated opencode setting to be preserved, got %#v", opencodeConfig)
	}
	cleanSpec := agents["rotta-spec"].(map[string]interface{})
	if cleanSpec["prompt"] == "old" {
		t.Fatalf("expected stale rotta-spec agent to be replaced, got %#v", cleanSpec)
	}
	if _, ok := agents["rotta-impl"]; ok {
		t.Fatalf("expected unselected stale rotta-impl agent to be removed, got %#v", agents)
	}
	assertPathMissing(t, filepath.Join(home, ".config", "opencode", "skills", "rotta-impl"))
	assertPathExists(t, filepath.Join(home, ".config", "opencode", "skills", "rotta-spec", "SKILL.md"))
	assertPathExists(t, filepath.Join(home, ".config", "opencode", "skills", "user-skill", "SKILL.md"))

	claudeSettings := readJSONFile(t, filepath.Join(home, ".claude", "settings.json"))
	permissions := claudeSettings["permissions"].(map[string]interface{})
	allow := permissions["allow"].([]interface{})
	assertJSONListContains(t, allow, "user-permission")
	assertJSONListContains(t, allow, "mcp__rotta__spec_mode")
	assertJSONListDoesNotContain(t, allow, "mcp__clean_workflow__implementation_mode")
	assertPathMissing(t, filepath.Join(home, ".claude", "skills", "rotta", "implementation-mode"))
	assertPathExists(t, filepath.Join(home, ".claude", "skills", "rotta", "spec-mode", "SKILL.md"))
	assertPathExists(t, filepath.Join(home, ".claude", "skills", "user-skill", "SKILL.md"))

	stateMachine := filepath.Join(projectPath, ".rotta", "state-machine.yaml")
	stateData, err := os.ReadFile(stateMachine)
	if err != nil {
		t.Fatalf("read fresh state machine: %v", err)
	}
	if string(stateData) == "stale: true\n" {
		t.Fatal("expected stale generated project config to be replaced")
	}
}

func TestSCN002_OpenCodeInstallMigratesLegacyBobAndCleanAgents(t *testing.T) {
	// REQ-004 -> SCN-002 -> TestSCN002_OpenCodeInstallMigratesLegacyBobAndCleanAgents
	// Scenario: Successful install cleans previous rotta settings before fresh install
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	t.Setenv("HOME", home)

	writeTestFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"), []byte(`{"$schema":"https://opencode.ai/config.json","default_agent":"clean-orchestrator","agent":{"bob-orchestrator":{"description":"legacy primary","mode":"primary"},"bob-spec":{"description":"legacy spec","mode":"subagent","hidden":true},"bob-impl":{"description":"legacy impl","mode":"subagent","hidden":true},"bob-review":{"description":"legacy review","mode":"subagent","hidden":true},"clean-orchestrator":{"description":"legacy clean primary","mode":"primary"},"clean-spec":{"description":"legacy clean spec","mode":"subagent","hidden":true},"clean-impl":{"description":"legacy clean impl","mode":"subagent","hidden":true},"clean-review":{"description":"legacy clean review","mode":"subagent","hidden":true},"rotta-orchestrator":{"description":"stale rotta","mode":"primary","prompt":"old"},"user-agent":{"description":"keep me","mode":"primary"}},"theme":"user-theme"}`))
	for _, skill := range []string{"bob-orchestrator", "bob-spec", "bob-impl", "bob-review", "clean-orchestrator", "clean-spec", "clean-impl", "clean-review", "user-skill"} {
		writeTestFile(t, filepath.Join(home, ".config", "opencode", "skills", skill, "SKILL.md"), []byte(skill+"\n"))
	}

	result, err := Install(Options{
		Target:        "opencode",
		ProjectPath:   projectPath,
		InstallSpec:   true,
		InstallImpl:   true,
		InstallReview: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.BackupDir == "" {
		t.Fatal("expected install to back up legacy Bob and clean artifacts before migration")
	}
	manifest := readBackupManifest(t, filepath.Join(result.BackupDir, "manifest.json"))
	assertManifestContainsPath(t, manifest.BackedUpPaths, filepath.Join(home, ".config", "opencode", "skills", "bob-orchestrator"))
	assertManifestContainsPath(t, manifest.BackedUpPaths, filepath.Join(home, ".config", "opencode", "skills", "clean-orchestrator"))
	assertFileContains(t, backupDestination(result.BackupDir, home, filepath.Join(home, ".config", "opencode", "skills", "bob-spec", "SKILL.md")), "bob-spec")
	assertFileContains(t, backupDestination(result.BackupDir, home, filepath.Join(home, ".config", "opencode", "skills", "clean-spec", "SKILL.md")), "clean-spec")

	opencodeConfig := readJSONFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"))
	if opencodeConfig["$schema"] == nil || opencodeConfig["theme"] != "user-theme" {
		t.Fatalf("expected unrelated opencode config to be preserved, got %#v", opencodeConfig)
	}
	if opencodeConfig["default_agent"] != "rotta-orchestrator" {
		t.Fatalf("expected legacy default_agent to migrate to rotta-orchestrator, got %#v", opencodeConfig["default_agent"])
	}
	agents := opencodeConfig["agent"].(map[string]interface{})
	for _, legacy := range []string{"bob-orchestrator", "bob-spec", "bob-impl", "bob-review", "clean-orchestrator", "clean-spec", "clean-impl", "clean-review"} {
		if _, ok := agents[legacy]; ok {
			t.Fatalf("expected legacy agent %s to be removed, got %#v", legacy, agents)
		}
		assertPathMissing(t, filepath.Join(home, ".config", "opencode", "skills", legacy))
	}
	if _, ok := agents["user-agent"]; !ok {
		t.Fatalf("expected unrelated user agent to be preserved, got %#v", agents)
	}
	rottaOrchestrator := agents["rotta-orchestrator"].(map[string]interface{})
	if rottaOrchestrator["mode"] != "primary" || rottaOrchestrator["prompt"] == "old" {
		t.Fatalf("expected Rotta orchestrator to be freshly installed as primary, got %#v", rottaOrchestrator)
	}
	for _, builtIn := range []string{"build", "plan"} {
		entry := agents[builtIn].(map[string]interface{})
		if entry["disable"] != true {
			t.Fatalf("expected OpenCode built-in agent %s to be disabled by default, got %#v", builtIn, entry)
		}
	}
	for _, rottaSubagent := range []string{"rotta-spec", "rotta-impl", "rotta-review"} {
		entry := agents[rottaSubagent].(map[string]interface{})
		if entry["mode"] != "subagent" || entry["hidden"] != true {
			t.Fatalf("expected %s to stay hidden subagent, got %#v", rottaSubagent, entry)
		}
	}
	assertPathExists(t, filepath.Join(home, ".config", "opencode", "skills", "user-skill", "SKILL.md"))
}

func TestSCN002_SelectedIntegrationCleanupRunsBeforeOptionalSetup(t *testing.T) {
	// REQ-004 → SCN-002 → TestSCN002_SelectedIntegrationCleanupRunsBeforeOptionalSetup
	// Scenario: Successful install cleans previous rotta settings before fresh install
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	binDir := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	writeTestFile(t, filepath.Join(projectPath, ".vela", "graph.db"), []byte("stale graph"))
	writeTestFile(t, filepath.Join(home, ".claude", "vela-mcp.json"), []byte(`{"stale":true}`))
	writeTestFile(t, filepath.Join(home, ".claude", "vela-instructions.md"), []byte("stale vela instructions"))
	writeTestFile(t, filepath.Join(home, ".claude", "mcp", "ancora.json"), []byte(`{"stale":true}`))
	writeTestFile(t, filepath.Join(home, ".config", "opencode", "instructions.md"), []byte("stale opencode instructions"))
	writeTestFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"), []byte(`{"theme":"user-theme"}`))

	writeExecutable(t, filepath.Join(binDir, "ancora"), `#!/bin/sh
if [ "$1" = setup ]; then
if [ -e "$HOME/.claude/mcp/ancora.json" ] && grep -q stale "$HOME/.claude/mcp/ancora.json"; then
    echo "stale ancora config was not cleaned before setup" >&2
    exit 17
  fi
  mkdir -p "$HOME/.claude/mcp"
  printf '{"fresh":true}' > "$HOME/.claude/mcp/ancora.json"
fi
`)
	writeExecutable(t, filepath.Join(binDir, "vela"), `#!/bin/sh
while [ "$#" -gt 0 ]; do
  if [ "$1" = --project ]; then
    shift
    project="$1"
  fi
  shift
done
if [ -z "$project" ]; then
  echo "missing project" >&2
  exit 18
fi
if [ -e "$project/.vela/graph.db" ] || [ -e "$HOME/.claude/vela-mcp.json" ] || [ -e "$HOME/.claude/vela-instructions.md" ] || [ -e "$HOME/.config/opencode/instructions.md" ]; then
  echo "stale vela artifacts were not cleaned before setup" >&2
  exit 19
fi
mkdir -p "$project/.vela"
printf 'fresh graph' > "$project/.vela/graph.db"
`)

	_, err := Install(Options{
		Target:      "both",
		ProjectPath: projectPath,
		InstallSpec: true,
		SetupAncora: true,
		SetupVela:   true,
	})
	if err != nil {
		t.Fatal(err)
	}

	assertFileContains(t, filepath.Join(projectPath, ".vela", "graph.db"), "fresh graph")
	assertFileContains(t, filepath.Join(home, ".claude", "mcp", "ancora.json"), "fresh")
	assertPathMissing(t, filepath.Join(home, ".claude", "vela-mcp.json"))
	assertPathMissing(t, filepath.Join(home, ".claude", "vela-instructions.md"))
	assertPathMissing(t, filepath.Join(home, ".config", "opencode", "instructions.md"))
}

func TestSCN002_OpenCodeAncoraAndVelaInstallSeparateMCPServers(t *testing.T) {
	// REQ-004 → SCN-002 → TestSCN002_OpenCodeAncoraAndVelaInstallSeparateMCPServers
	// Scenario: Successful install configures selected optional integrations for OpenCode.
	for _, target := range []string{"opencode", "both"} {
		t.Run(target, func(t *testing.T) {
			home := t.TempDir()
			projectPath := filepath.Join(home, "project")
			binDir := filepath.Join(home, "bin")
			logPath := filepath.Join(home, "setup.log")
			t.Setenv("HOME", home)
			t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

			writeExecutable(t, filepath.Join(binDir, "ancora"), `#!/bin/sh
printf 'ancora %s\n' "$*" >> "$HOME/setup.log"
if [ "$1" = setup ] && [ "$2" = opencode ]; then
  mkdir -p "$HOME/.config/opencode"
  printf '{"mcp":{"ancora":{"type":"local","enabled":true}}}' > "$HOME/.config/opencode/opencode.jsonc"
fi
`)
			writeExecutable(t, filepath.Join(binDir, "vela"), `#!/bin/sh
printf 'vela %s\n' "$*" >> "$HOME/setup.log"
project=""
agent=""
opencode_dir=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --project) shift; project="$1" ;;
    --agent) shift; agent="$1" ;;
    --opencode-dir) shift; opencode_dir="$1" ;;
  esac
  shift
done
mkdir -p "$project/.vela"
printf 'fresh graph' > "$project/.vela/graph.db"
if [ "$agent" = opencode ]; then
  mkdir -p "$opencode_dir"
  printf '{"mcp":{"vela":{"type":"local","enabled":true}}}' > "$opencode_dir/opencode.json"
fi
`)

			_, err := Install(Options{
				Target:      target,
				ProjectPath: projectPath,
				InstallSpec: true,
				SetupAncora: true,
				SetupVela:   true,
			})
			if err != nil {
				t.Fatal(err)
			}

			opencodeJSONC := readJSONFile(t, filepath.Join(home, ".config", "opencode", "opencode.jsonc"))
			opencodeJSON := readJSONFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"))
			assertMCPEntryExists(t, opencodeJSONC, "ancora")
			assertMCPEntryExists(t, opencodeJSON, "vela")
			if _, ok := opencodeJSONC["mcp"].(map[string]interface{})["vela"]; ok {
				t.Fatalf("expected Ancora config not to contain direct Vela MCP, got %#v", opencodeJSONC)
			}
			if _, ok := opencodeJSON["mcp"].(map[string]interface{})["ancora"]; ok {
				t.Fatalf("expected Vela config not to contain Ancora MCP, got %#v", opencodeJSON)
			}
			assertFileContains(t, logPath, "ancora setup opencode")
			assertFileContains(t, logPath, "vela install --project "+projectPath+" --agent opencode")
		})
	}
}

func assertMCPEntryExists(t *testing.T, config map[string]interface{}, name string) {
	t.Helper()
	mcp, ok := config["mcp"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected mcp map in config, got %#v", config)
	}
	entry, ok := mcp[name].(map[string]interface{})
	if !ok {
		t.Fatalf("expected mcp.%s entry in config, got %#v", name, config)
	}
	if entry["enabled"] != true {
		t.Fatalf("expected mcp.%s to be enabled, got %#v", name, entry)
	}
}

func TestSCN023_ExternalSetupOutputIsRoutedThroughInstallOptions(t *testing.T) {
	// REQ-004 → SCN-023 → TestSCN023_ExternalSetupOutputIsRoutedThroughInstallOptions
	// Scenario: TUI install can keep external setup output away from the Bubble Tea screen.
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	binDir := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	writeExecutable(t, filepath.Join(binDir, "ancora"), `#!/bin/sh
if [ "$1" = setup ]; then
  echo "external stdout"
  echo "external stderr" >&2
  exit 0
fi
`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	_, err := Install(Options{
		Target:        "opencode",
		ProjectPath:   projectPath,
		InstallSpec:   true,
		SetupAncora:   true,
		CommandStdout: &stdout,
		CommandStderr: &stderr,
	})
	if err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "external stdout\n" {
		t.Fatalf("expected setup stdout to be routed through options, got %q", stdout.String())
	}
	if stderr.String() != "external stderr\n" {
		t.Fatalf("expected setup stderr to be routed through options, got %q", stderr.String())
	}
}

func TestSCN023_DefaultExternalCommandStdinRemainsInteractive(t *testing.T) {
	// REQ-004 → SCN-023 → TestSCN023_DefaultExternalCommandStdinRemainsInteractive
	// Scenario: CLI/default installs can still answer prompts from external setup tools.
	cmd := exec.Command("true")
	configureCommandIO(cmd, Options{})
	if cmd.Stdin != os.Stdin {
		t.Fatal("expected default external command stdin to remain interactive")
	}
}

func TestSCN023_VelaSetupOutputIsRoutedThroughInstallOptions(t *testing.T) {
	// REQ-004 → SCN-023 → TestSCN023_VelaSetupOutputIsRoutedThroughInstallOptions
	// Scenario: Vela setup cannot write directly over the TUI install screen.
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	binDir := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	writeExecutable(t, filepath.Join(binDir, "vela"), `#!/bin/sh
echo "vela stdout"
echo "vela stderr" >&2
`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	_, err := SetupVela(Options{
		Target:        "opencode",
		ProjectPath:   projectPath,
		SetupVela:     true,
		CommandStdout: &stdout,
		CommandStderr: &stderr,
	}, home, projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "vela stdout\n" {
		t.Fatalf("expected vela stdout to be routed through options, got %q", stdout.String())
	}
	if stderr.String() != "vela stderr\n" {
		t.Fatalf("expected vela stderr to be routed through options, got %q", stderr.String())
	}
}

func TestSCN023_BootstrapInstallOutputIsRoutedThroughInstallOptions(t *testing.T) {
	// REQ-004 → SCN-023 → TestSCN023_BootstrapInstallOutputIsRoutedThroughInstallOptions
	// Scenario: bootstrap install commands cannot write directly over the TUI install screen.
	home := t.TempDir()
	binDir := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir)

	writeExecutable(t, filepath.Join(binDir, "brew"), `#!/bin/sh
echo "brew stdout $*"
echo "brew stderr $*" >&2
`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	opts := Options{CommandStdout: &stdout, CommandStderr: &stderr}
	if err := installAncora(opts); err != nil {
		t.Fatalf("install ancora via brew: %v", err)
	}
	if err := installVela(opts); err != nil {
		t.Fatalf("install vela via brew: %v", err)
	}
	if strings.Count(stdout.String(), "brew stdout") != 6 {
		t.Fatalf("expected all brew stdout to be routed through options, got %q", stdout.String())
	}
	if strings.Count(stderr.String(), "brew stderr") != 6 {
		t.Fatalf("expected all brew stderr to be routed through options, got %q", stderr.String())
	}
}

func TestSCN023_AncoraScriptBootstrapOutputIsRoutedThroughInstallOptions(t *testing.T) {
	// REQ-004 → SCN-023 → TestSCN023_AncoraScriptBootstrapOutputIsRoutedThroughInstallOptions
	// Scenario: curl|bash bootstrap fallback cannot write directly over the TUI install screen.
	home := t.TempDir()
	binDir := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir)

	writeExecutable(t, filepath.Join(binDir, "curl"), `#!/bin/sh
printf '%s\n' '#!/bin/sh' 'echo script body'
`)
	writeExecutable(t, filepath.Join(binDir, "bash"), `#!/bin/sh
while IFS= read -r ignored; do :; done
echo "bash stdout"
echo "bash stderr" >&2
`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := installAncoraViaScript(Options{CommandStdout: &stdout, CommandStderr: &stderr}); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "bash stdout\n" {
		t.Fatalf("expected bash stdout to be routed through options, got %q", stdout.String())
	}
	if stderr.String() != "bash stderr\n" {
		t.Fatalf("expected bash stderr to be routed through options, got %q", stderr.String())
	}
}

func TestSCN003_BackupFailureAbortsInstallCompletely(t *testing.T) {
	// REQ-003 → SCN-003 → TestSCN003_BackupFailureAbortsInstallCompletely
	// Scenario: Backup failure aborts install completely
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	t.Setenv("HOME", home)

	writeTestFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"), []byte(`{"agent":{"rotta-spec":{"description":"stale"},"user-agent":{"description":"keep"}}}`))
	writeTestFile(t, filepath.Join(home, ".config", "opencode", "skills", "rotta-spec", "SKILL.md"), []byte("stale skill\n"))
	writeTestFile(t, filepath.Join(projectPath, ".rotta", "state-machine.yaml"), []byte("stale: true\n"))
	writeTestFile(t, filepath.Join(home, ".rotta", "backups"), []byte("not a directory\n"))

	result, err := Install(Options{
		Target:      "opencode",
		ProjectPath: projectPath,
		InstallSpec: true,
	})
	if err == nil {
		t.Fatal("expected backup failure to abort install")
	}
	if result != nil {
		t.Fatalf("expected no install result after backup failure, got %#v", result)
	}
	if !strings.Contains(err.Error(), "backup failure prevented installation") {
		t.Fatalf("expected recovery-safe backup failure message, got %v", err)
	}

	opencodeConfig := readJSONFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"))
	agents := opencodeConfig["agent"].(map[string]interface{})
	cleanSpec := agents["rotta-spec"].(map[string]interface{})
	if cleanSpec["description"] != "stale" {
		t.Fatalf("expected cleanup and fresh install not to mutate opencode agents, got %#v", agents)
	}
	assertPathExists(t, filepath.Join(home, ".config", "opencode", "skills", "rotta-spec", "SKILL.md"))
	assertFileContains(t, filepath.Join(projectPath, ".rotta", "state-machine.yaml"), "stale: true")
	assertFileContains(t, filepath.Join(home, ".rotta", "backups"), "not a directory")
}

func TestSCN007_RestoreAppliesFullBackupAndRemovesAbsentPaths(t *testing.T) {
	// REQ-007 → SCN-007 → TestSCN007_RestoreAppliesFullBackupAndRemovesAbsentPaths
	// Scenario: Restore applies the full backup and removes paths that were absent
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	t.Setenv("HOME", home)

	selectedBackupDir := filepath.Join(home, ".rotta", "backups", "20260629T130000Z")
	restoredOpenCodeConfig := filepath.Join(home, ".config", "opencode", "opencode.json")
	restoredSkillDir := filepath.Join(home, ".config", "opencode", "skills", "rotta-spec")
	absentAtBackupPath := filepath.Join(projectPath, ".rotta", "quality-gates.yaml")

	writeTestFile(t, backupDestination(selectedBackupDir, home, restoredOpenCodeConfig), []byte(`{"agent":{"restored":{"description":"from backup"}}}`))
	writeTestFile(t, backupDestination(selectedBackupDir, home, filepath.Join(restoredSkillDir, "SKILL.md")), []byte("restored skill\n"))
	writeTestFile(t, filepath.Join(selectedBackupDir, "manifest.json"), []byte(`{"version":1,"timestamp":"20260629T130000Z","project_path":"`+projectPath+`","target":"opencode","selected_modes":{"spec":true,"implementation":false,"review":false},"optional_integrations":{"ancora":false,"vela":false},"backed_up_paths":["`+restoredOpenCodeConfig+`","`+restoredSkillDir+`"],"missing_paths":["`+absentAtBackupPath+`"],"status":"complete"}`))

	writeTestFile(t, restoredOpenCodeConfig, []byte(`{"agent":{"current":{"description":"before restore"}}}`))
	writeTestFile(t, filepath.Join(restoredSkillDir, "SKILL.md"), []byte("current skill\n"))
	writeTestFile(t, absentAtBackupPath, []byte("created after selected backup\n"))

	result, err := RestoreBackup(selectedBackupDir)
	if err != nil {
		t.Fatal(err)
	}
	if result.PreRestoreBackupDir == "" {
		t.Fatal("expected restore to create a pre-restore safety backup")
	}
	if result.PreRestoreBackupDir == selectedBackupDir {
		t.Fatal("pre-restore safety backup must be distinct from the selected backup")
	}

	assertFileContains(t, restoredOpenCodeConfig, "from backup")
	assertFileContains(t, filepath.Join(restoredSkillDir, "SKILL.md"), "restored skill")
	assertPathMissing(t, absentAtBackupPath)
	assertFileContains(t, backupDestination(result.PreRestoreBackupDir, home, restoredOpenCodeConfig), "before restore")
	assertFileContains(t, backupDestination(result.PreRestoreBackupDir, home, filepath.Join(restoredSkillDir, "SKILL.md")), "current skill")
}

func TestSCN008_FailedRestoreRollsBackToPreRestoreState(t *testing.T) {
	// REQ-008 → SCN-008 → TestSCN008_FailedRestoreRollsBackToPreRestoreState
	// Scenario: Failed restore rolls back to pre-restore state
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	t.Setenv("HOME", home)

	selectedBackupDir := filepath.Join(home, ".rotta", "backups", "20260629T140000Z")
	restoredOpenCodeConfig := filepath.Join(home, ".config", "opencode", "opencode.json")
	restoredSkillDir := filepath.Join(home, ".config", "opencode", "skills", "rotta-spec")

	writeTestFile(t, backupDestination(selectedBackupDir, home, restoredOpenCodeConfig), []byte(`{"agent":{"restored":{"description":"from selected backup"}}}`))
	writeTestFile(t, backupDestination(selectedBackupDir, home, filepath.Join(restoredSkillDir, "SKILL.md")), []byte("restored skill\n"))
	writeTestFile(t, filepath.Join(selectedBackupDir, "manifest.json"), []byte(`{"version":1,"timestamp":"20260629T140000Z","project_path":"`+projectPath+`","target":"opencode","selected_modes":{"spec":true,"implementation":false,"review":false},"optional_integrations":{"ancora":false,"vela":false},"backed_up_paths":["`+restoredOpenCodeConfig+`","`+restoredSkillDir+`"],"missing_paths":[],"status":"complete"}`))

	writeTestFile(t, restoredOpenCodeConfig, []byte(`{"agent":{"current":{"description":"pre-restore"}}}`))
	writeTestFile(t, filepath.Join(restoredSkillDir, "SKILL.md"), []byte("current skill\n"))

	failedOnce := false
	result, err := restoreBackupWithHooks(selectedBackupDir, restoreHooks{
		afterRestorePath: func(path string) error {
			if path == restoredOpenCodeConfig && !failedOnce {
				failedOnce = true
				return os.ErrPermission
			}
			return nil
		},
	})
	if err == nil {
		t.Fatal("expected restore failure")
	}
	if result == nil || result.PreRestoreBackupDir == "" {
		t.Fatalf("expected failed restore to report pre-restore safety backup, got %#v", result)
	}
	if !strings.Contains(err.Error(), selectedBackupDir) {
		t.Fatalf("expected failure to identify selected backup, got %v", err)
	}
	if !strings.Contains(err.Error(), "rollback to pre-restore state succeeded") {
		t.Fatalf("expected failure to report successful rollback, got %v", err)
	}

	assertFileContains(t, restoredOpenCodeConfig, "pre-restore")
	assertFileContains(t, filepath.Join(restoredSkillDir, "SKILL.md"), "current skill")
}

func TestSCN009_RestoreFailureWithRollbackFailureProvidesManualRecoveryLocations(t *testing.T) {
	// REQ-008 → SCN-009 → TestSCN009_RestoreFailureWithRollbackFailureProvidesManualRecoveryLocations
	// Scenario: Restore failure with rollback failure provides manual recovery locations
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	t.Setenv("HOME", home)

	selectedBackupDir := filepath.Join(home, ".rotta", "backups", "20260629T150000Z")
	restoredOpenCodeConfig := filepath.Join(home, ".config", "opencode", "opencode.json")

	writeTestFile(t, backupDestination(selectedBackupDir, home, restoredOpenCodeConfig), []byte(`{"agent":{"restored":{"description":"from selected backup"}}}`))
	writeTestFile(t, filepath.Join(selectedBackupDir, "manifest.json"), []byte(`{"version":1,"timestamp":"20260629T150000Z","project_path":"`+projectPath+`","target":"opencode","selected_modes":{"spec":true,"implementation":false,"review":false},"optional_integrations":{"ancora":false,"vela":false},"backed_up_paths":["`+restoredOpenCodeConfig+`"],"missing_paths":[],"status":"complete"}`))
	writeTestFile(t, restoredOpenCodeConfig, []byte(`{"agent":{"current":{"description":"pre-restore"}}}`))

	result, err := restoreBackupWithHooks(selectedBackupDir, restoreHooks{
		afterRestorePath: func(path string) error {
			if path != restoredOpenCodeConfig {
				return nil
			}
			preRestoreBackupDir := newestBackupDirExcept(t, filepath.Dir(selectedBackupDir), selectedBackupDir)
			writeTestFile(t, filepath.Join(preRestoreBackupDir, "manifest.json"), []byte(`not json`))
			return os.ErrPermission
		},
	})
	if err == nil {
		t.Fatal("expected restore and rollback failure")
	}
	if result == nil || result.PreRestoreBackupDir == "" {
		t.Fatalf("expected failed restore to report pre-restore safety backup, got %#v", result)
	}
	if !strings.Contains(err.Error(), selectedBackupDir) {
		t.Fatalf("expected failure to identify selected backup, got %v", err)
	}
	if !strings.Contains(err.Error(), result.PreRestoreBackupDir) {
		t.Fatalf("expected failure to identify pre-restore safety backup %s, got %v", result.PreRestoreBackupDir, err)
	}
	if strings.Contains(err.Error(), "restore succeeded") || strings.Contains(err.Error(), "restore successful") {
		t.Fatalf("expected failed restore not to report success, got %v", err)
	}
}

func TestSCN011_GeneratedArtifactsAndUserFacingTextAvoidExternalReferenceWording(t *testing.T) {
	// REQ-009 → SCN-011 → TestSCN011_GeneratedArtifactsAndUserFacingTextAvoidExternalReferenceWording
	// Scenario: Generated acceptance artifacts and user-facing text avoid external-reference wording
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	forbidden := []string{
		"gentle" + "-" + "ai",
		"Gentle" + " AI",
	}

	err := filepath.WalkDir(repoRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".vela":
				return filepath.SkipDir
			}
			return nil
		}
		if !isNeutralWordingArtifact(path) {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content := string(data)
		for _, term := range forbidden {
			if strings.Contains(content, term) {
				t.Fatalf("expected neutral wording in %s", path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func writeTestFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

func readBackupManifest(t *testing.T, path string) backupManifest {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest backupManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	return manifest
}

func assertManifestContainsPath(t *testing.T, paths []string, want string) {
	t.Helper()
	for _, path := range paths {
		if path == want {
			return
		}
	}
	t.Fatalf("expected manifest paths to contain %q, got %#v", want, paths)
}

func readJSONFile(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read JSON file %s: %v", path, err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("parse JSON file %s: %v", path, err)
	}
	return out
}

func assertPathExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected path to exist %s: %v", path, err)
	}
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected path to be missing %s, got %v", path, err)
	}
}

func assertFileContains(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	if !strings.Contains(string(data), want) {
		t.Fatalf("expected %s to contain %q, got %q", path, want, string(data))
	}
}

func assertJSONListContains(t *testing.T, values []interface{}, want string) {
	t.Helper()
	for _, value := range values {
		if value == want {
			return
		}
	}
	t.Fatalf("expected list to contain %q, got %#v", want, values)
}

func assertJSONListDoesNotContain(t *testing.T, values []interface{}, want string) {
	t.Helper()
	for _, value := range values {
		if value == want {
			t.Fatalf("expected list not to contain %q, got %#v", want, values)
		}
	}
}

func isNeutralWordingArtifact(path string) bool {
	switch filepath.Ext(path) {
	case ".feature", ".go", ".json", ".jsonc", ".md", ".yaml", ".yml":
		return true
	default:
		return false
	}
}

func newestBackupDirExcept(t *testing.T, backupRoot, excluded string) string {
	t.Helper()
	entries, err := os.ReadDir(backupRoot)
	if err != nil {
		t.Fatalf("read backup root: %v", err)
	}
	var newest string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(backupRoot, entry.Name())
		if path == excluded {
			continue
		}
		if path > newest {
			newest = path
		}
	}
	if newest == "" {
		t.Fatal("expected a pre-restore safety backup directory")
	}
	return newest
}
