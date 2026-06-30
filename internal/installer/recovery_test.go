package installer

import (
	"encoding/json"
	"os"
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
	writeTestFile(t, filepath.Join(projectPath, ".clean-workflow", "state-machine.yaml"), []byte("previous: state\n"))

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
	if filepath.Dir(result.BackupDir) != filepath.Join(home, ".clean-workflow", "backups") {
		t.Fatalf("expected backup under ~/.clean-workflow/backups, got %s", result.BackupDir)
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
	// Scenario: Successful install cleans previous clean-workflow settings before fresh install
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	t.Setenv("HOME", home)

	writeTestFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"), []byte(`{"agent":{"clean-spec":{"description":"stale","mode":"subagent","prompt":"old"},"clean-impl":{"description":"remove me"},"user-agent":{"description":"keep me"}},"theme":"user-theme"}`))
	writeTestFile(t, filepath.Join(home, ".config", "opencode", "skills", "clean-impl", "SKILL.md"), []byte("stale impl skill\n"))
	writeTestFile(t, filepath.Join(home, ".config", "opencode", "skills", "user-skill", "SKILL.md"), []byte("keep user skill\n"))
	writeTestFile(t, filepath.Join(home, ".claude", "skills", "clean-workflow", "implementation-mode", "SKILL.md"), []byte("stale claude impl skill\n"))
	writeTestFile(t, filepath.Join(home, ".claude", "skills", "user-skill", "SKILL.md"), []byte("keep claude user skill\n"))
	writeTestFile(t, filepath.Join(home, ".claude", "settings.json"), []byte(`{"permissions":{"allow":["mcp__clean_workflow__implementation_mode","user-permission"]},"theme":"dark"}`))
	writeTestFile(t, filepath.Join(projectPath, ".clean-workflow", "state-machine.yaml"), []byte("stale: true\n"))

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
	cleanSpec := agents["clean-spec"].(map[string]interface{})
	if cleanSpec["prompt"] == "old" {
		t.Fatalf("expected stale clean-spec agent to be replaced, got %#v", cleanSpec)
	}
	if _, ok := agents["clean-impl"]; ok {
		t.Fatalf("expected unselected stale clean-impl agent to be removed, got %#v", agents)
	}
	assertPathMissing(t, filepath.Join(home, ".config", "opencode", "skills", "clean-impl"))
	assertPathExists(t, filepath.Join(home, ".config", "opencode", "skills", "clean-spec", "SKILL.md"))
	assertPathExists(t, filepath.Join(home, ".config", "opencode", "skills", "user-skill", "SKILL.md"))

	claudeSettings := readJSONFile(t, filepath.Join(home, ".claude", "settings.json"))
	permissions := claudeSettings["permissions"].(map[string]interface{})
	allow := permissions["allow"].([]interface{})
	assertJSONListContains(t, allow, "user-permission")
	assertJSONListDoesNotContain(t, allow, "mcp__clean_workflow__implementation_mode")
	assertPathMissing(t, filepath.Join(home, ".claude", "skills", "clean-workflow", "implementation-mode"))
	assertPathExists(t, filepath.Join(home, ".claude", "skills", "clean-workflow", "spec-mode", "SKILL.md"))
	assertPathExists(t, filepath.Join(home, ".claude", "skills", "user-skill", "SKILL.md"))

	stateMachine := filepath.Join(projectPath, ".clean-workflow", "state-machine.yaml")
	stateData, err := os.ReadFile(stateMachine)
	if err != nil {
		t.Fatalf("read fresh state machine: %v", err)
	}
	if string(stateData) == "stale: true\n" {
		t.Fatal("expected stale generated project config to be replaced")
	}
}

func TestSCN002_SelectedIntegrationCleanupRunsBeforeOptionalSetup(t *testing.T) {
	// REQ-004 → SCN-002 → TestSCN002_SelectedIntegrationCleanupRunsBeforeOptionalSetup
	// Scenario: Successful install cleans previous clean-workflow settings before fresh install
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
