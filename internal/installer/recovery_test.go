package installer

import (
	"os"
	"path/filepath"
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

	assertBackupCreatedBeforeMutation(t, result, home, projectPath, preInstallOpenCodeConfig)
}

func assertBackupCreatedBeforeMutation(t *testing.T, result *Result, home, projectPath string, preInstallOpenCodeConfig []byte) {
	t.Helper()
	if result.BackupDir == "" {
		t.Fatal("expected install result to include backup directory")
	}
	if filepath.Dir(result.BackupDir) != filepath.Join(home, ".rotta", "backups") {
		t.Fatalf("expected backup under ~/.rotta/backups, got %s", result.BackupDir)
	}
	assertBackupManifest(t, result.BackupDir, home, projectPath)
	backupContent, err := os.ReadFile(filepath.Join(result.BackupDir, "files", "home", ".config", "opencode", "opencode.json"))
	if err != nil {
		t.Fatalf("read backed-up opencode config: %v", err)
	}
	if string(backupContent) != string(preInstallOpenCodeConfig) {
		t.Fatalf("backup should capture pre-install content before mutation, got %s", backupContent)
	}
}

func assertBackupManifest(t *testing.T, backupDir, home, projectPath string) {
	t.Helper()
	manifest := readBackupManifest(t, filepath.Join(backupDir, "manifest.json"))
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
	assertLegacyOpenCodeMigration(t, result, home)
}

func assertLegacyOpenCodeMigration(t *testing.T, result *Result, home string) {
	t.Helper()
	if result.BackupDir == "" {
		t.Fatal("expected install to back up legacy Bob and clean artifacts before migration")
	}
	manifest := readBackupManifest(t, filepath.Join(result.BackupDir, "manifest.json"))
	assertManifestContainsPath(t, manifest.BackedUpPaths, filepath.Join(home, ".config", "opencode", "skills", "bob-orchestrator"))
	assertManifestContainsPath(t, manifest.BackedUpPaths, filepath.Join(home, ".config", "opencode", "skills", "clean-orchestrator"))
	assertFileContains(t, backupDestination(result.BackupDir, home, filepath.Join(home, ".config", "opencode", "skills", "bob-spec", "SKILL.md")), "bob-spec")
	assertFileContains(t, backupDestination(result.BackupDir, home, filepath.Join(home, ".config", "opencode", "skills", "clean-spec", "SKILL.md")), "clean-spec")
	assertMigratedOpenCodeConfig(t, home)
}

func assertMigratedOpenCodeConfig(t *testing.T, home string) {
	t.Helper()
	config := readJSONFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"))
	if config["$schema"] == nil || config["theme"] != "user-theme" {
		t.Fatalf("expected unrelated opencode config to be preserved, got %#v", config)
	}
	if config["default_agent"] != "rotta-orchestrator" {
		t.Fatalf("expected legacy default_agent to migrate to rotta-orchestrator, got %#v", config["default_agent"])
	}
	agents := config["agent"].(map[string]interface{})
	assertLegacyAgentsRemoved(t, home, agents)
	if _, ok := agents["user-agent"]; !ok {
		t.Fatalf("expected unrelated user agent to be preserved, got %#v", agents)
	}
	assertRottaOpenCodeAgents(t, agents)
	assertPathExists(t, filepath.Join(home, ".config", "opencode", "skills", "user-skill", "SKILL.md"))
}

func assertLegacyAgentsRemoved(t *testing.T, home string, agents map[string]interface{}) {
	t.Helper()
	for _, legacy := range append(legacyBobOpenCodeAgentKeys, legacyCleanOpenCodeAgentKeys...) {
		if _, ok := agents[legacy]; ok {
			t.Fatalf("expected legacy agent %s to be removed, got %#v", legacy, agents)
		}
		assertPathMissing(t, filepath.Join(home, ".config", "opencode", "skills", legacy))
	}
}

func assertRottaOpenCodeAgents(t *testing.T, agents map[string]interface{}) {
	t.Helper()
	orchestrator := agents["rotta-orchestrator"].(map[string]interface{})
	if orchestrator["mode"] != "primary" || orchestrator["prompt"] == "old" {
		t.Fatalf("expected Rotta orchestrator to be freshly installed as primary, got %#v", orchestrator)
	}
	for _, builtIn := range disabledOpenCodeDefaultAgentKeys {
		entry := agents[builtIn].(map[string]interface{})
		if entry["disable"] != true {
			t.Fatalf("expected OpenCode built-in agent %s to be disabled by default, got %#v", builtIn, entry)
		}
	}
	for _, subagent := range []string{"rotta-spec", "rotta-impl", "rotta-review"} {
		entry := agents[subagent].(map[string]interface{})
		if entry["mode"] != "subagent" || entry["hidden"] != true {
			t.Fatalf("expected %s to stay hidden subagent, got %#v", subagent, entry)
		}
	}
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

func TestSCN002_VelaSetupUpgradesExistingHomebrewInstallBeforeAgentInstall(t *testing.T) {
	// REQ-004 → SCN-002 → TestSCN002_VelaSetupUpgradesExistingHomebrewInstallBeforeAgentInstall
	// Scenario: Successful install refreshes an existing Vela CLI before configuring integration.
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	binDir := filepath.Join(home, "bin")
	logPath := filepath.Join(home, "setup.log")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	writeExecutable(t, filepath.Join(binDir, "vela"), `#!/bin/sh
printf 'vela %s\n' "$*" >> "$HOME/setup.log"
`)
	writeExecutable(t, filepath.Join(binDir, "brew"), `#!/bin/sh
printf 'brew %s\n' "$*" >> "$HOME/setup.log"
`)

	_, err := SetupVela(Options{
		Target:      "opencode",
		ProjectPath: projectPath,
		SetupVela:   true,
	}, home, projectPath)
	if err != nil {
		t.Fatal(err)
	}

	assertFileContains(t, logPath, "brew tap Syfra3/tap")
	assertFileContains(t, logPath, "brew update")
	assertFileContains(t, logPath, "brew upgrade vela")
	assertFileContains(t, logPath, "vela install --project "+projectPath+" --agent opencode")
}

func TestSCN002_VelaSetupUsesVelaCLIForProjectClusteringDependencies(t *testing.T) {
	// REQ-004 → SCN-002 → TestSCN002_VelaSetupUsesVelaCLIForProjectClusteringDependencies
	// Scenario: Successful install delegates clustering dependency setup to the Vela CLI installer.
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	binDir := filepath.Join(home, "bin")
	logPath := filepath.Join(home, "setup.log")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	writeTestFile(t, filepath.Join(projectPath, "requirements-clustering.txt"), []byte("networkx\n"))

	writeExecutable(t, filepath.Join(binDir, "vela"), `#!/bin/sh
printf 'vela %s\n' "$*" >> "$HOME/setup.log"
`)

	_, err := SetupVela(Options{
		Target:      "opencode",
		ProjectPath: projectPath,
		SetupVela:   true,
	}, home, projectPath)
	if err != nil {
		t.Fatal(err)
	}

	assertFileContains(t, logPath, "vela install --project "+projectPath+" --clustering --repair-venv")
	assertFileContains(t, logPath, "vela install --project "+projectPath+" --agent opencode")
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
