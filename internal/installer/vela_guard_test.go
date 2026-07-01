package installer

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSCN002_OpenCodeInstallPersistsVelaFreshnessGuard(t *testing.T) {
	// REQ-004 → SCN-002 → TestSCN002_OpenCodeInstallPersistsVelaFreshnessGuard
	// Scenario: Successful install cleans previous rotta settings before fresh install
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	binDir := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	pluginPath := filepath.Join(home, ".config", "opencode", "plugin", "rotta-vela-freshness-guard.js")
	writeTestFile(t, pluginPath, []byte("stale plugin should be backed up before cleanup\n"))
	pluginURL := openCodePluginFileURL(pluginPath)
	writeTestFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"), []byte(`{
  "plugin": ["user-plugin", "`+pluginURL+`", "`+pluginURL+`"],
  "agent": {"user-agent": {"description": "keep"}},
  "theme": "user-theme"
}`))
	writeExecutable(t, filepath.Join(binDir, "vela"), `#!/bin/sh
while [ "$#" -gt 0 ]; do
  if [ "$1" = --project ]; then
    shift
    project="$1"
  fi
  shift
done
mkdir -p "$project/.vela"
printf 'fresh graph' > "$project/.vela/graph.db"
`)

	result, err := Install(Options{
		Target:      "opencode",
		ProjectPath: projectPath,
		InstallSpec: true,
		SetupVela:   true,
	})
	if err != nil {
		t.Fatal(err)
	}

	assertPathExists(t, pluginPath)
	assertFileContains(t, pluginPath, "tool.execute.before")
	assertFileContains(t, pluginPath, "vela update")
	assertFileContains(t, pluginPath, "vela build")
	assertStringListContains(t, result.Files, pluginPath)

	manifest := readBackupManifest(t, filepath.Join(result.BackupDir, "manifest.json"))
	assertManifestContainsPath(t, manifest.BackedUpPaths, pluginPath)

	config := readJSONFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"))
	if config["theme"] != "user-theme" {
		t.Fatalf("expected unrelated opencode config to be preserved, got %#v", config)
	}
	plugins := config["plugin"].([]interface{})
	assertJSONListContains(t, plugins, "user-plugin")
	if countJSONListOccurrences(plugins, pluginURL) != 1 {
		t.Fatalf("expected guard plugin to be registered once, got %#v", plugins)
	}
}

func TestSCN002_ClaudeCodeInstallPersistsVelaFreshnessHooks(t *testing.T) {
	// REQ-004 → SCN-002 → TestSCN002_ClaudeCodeInstallPersistsVelaFreshnessHooks
	// Scenario: Successful install cleans previous rotta settings before fresh install
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	binDir := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	settingsPath := filepath.Join(home, ".claude", "settings.json")
	hookPath := filepath.Join(home, ".claude", "hooks", "rotta-vela-freshness-guard.sh")
	writeTestFile(t, settingsPath, []byte(`{
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "echo keep-session"}]}],
    "PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "echo keep-pretool"}]}]
  },
  "permissions": {"allow": ["user-permission"]},
  "theme": "dark"
}`))
	writeExecutable(t, filepath.Join(binDir, "vela"), `#!/bin/sh
while [ "$#" -gt 0 ]; do
  if [ "$1" = --project ]; then
    shift
    project="$1"
  fi
  shift
done
mkdir -p "$project/.vela"
printf 'fresh graph' > "$project/.vela/graph.db"
`)

	result, err := Install(Options{
		Target:      "claude-code",
		ProjectPath: projectPath,
		InstallSpec: true,
		SetupVela:   true,
	})
	if err != nil {
		t.Fatal(err)
	}

	assertPathExists(t, hookPath)
	info, err := os.Stat(hookPath)
	if err != nil {
		t.Fatalf("stat hook script: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("expected hook script to be executable, mode=%v", info.Mode())
	}
	assertFileContains(t, hookPath, "vela update")
	assertFileContains(t, hookPath, "vela build")
	assertStringListContains(t, result.Files, hookPath)

	settings := readJSONFile(t, settingsPath)
	if settings["theme"] != "dark" {
		t.Fatalf("expected unrelated Claude Code settings to be preserved, got %#v", settings)
	}
	hooks := settings["hooks"].(map[string]interface{})
	assertClaudeHookCommandContains(t, hooks, "SessionStart", hookPath)
	assertClaudeHookCommandContains(t, hooks, "SessionStart", "echo keep-session")
	assertClaudeHookCommandContains(t, hooks, "PreToolUse", hookPath)
	assertClaudeHookCommandContains(t, hooks, "PreToolUse", "echo keep-pretool")
	assertClaudePreToolMatcherContains(t, hooks, "mcp__vela__.*")
	assertClaudePreToolMatcherContains(t, hooks, "ancora_vela_.*")
}

func TestSCN002_ReinstallCleansStaleVelaFreshnessGuardBeforeSetup(t *testing.T) {
	// REQ-004 → SCN-002 → TestSCN002_ReinstallCleansStaleVelaFreshnessGuardBeforeSetup
	// Scenario: Successful install cleans previous rotta settings before fresh install
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	binDir := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	pluginPath := filepath.Join(home, ".config", "opencode", "plugin", "rotta-vela-freshness-guard.js")
	hookPath := filepath.Join(home, ".claude", "hooks", "rotta-vela-freshness-guard.sh")
	writeTestFile(t, pluginPath, []byte("stale opencode guard"))
	writeTestFile(t, hookPath, []byte("stale claude guard"))
	writeTestFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"), []byte(`{"plugin":["`+openCodePluginFileURL(pluginPath)+`"]}`))
	writeTestFile(t, filepath.Join(home, ".claude", "settings.json"), []byte(`{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"`+hookPath+`"}]}]}}`))
	writeExecutable(t, filepath.Join(binDir, "vela"), `#!/bin/sh
if [ -e "$HOME/.config/opencode/plugin/rotta-vela-freshness-guard.js" ] || [ -e "$HOME/.claude/hooks/rotta-vela-freshness-guard.sh" ]; then
  echo "stale guard was not removed before vela setup" >&2
  exit 31
fi
while [ "$#" -gt 0 ]; do
  if [ "$1" = --project ]; then
    shift
    project="$1"
  fi
  shift
done
mkdir -p "$project/.vela"
printf 'fresh graph' > "$project/.vela/graph.db"
`)

	_, err := Install(Options{
		Target:      "both",
		ProjectPath: projectPath,
		InstallSpec: true,
		SetupVela:   true,
	})
	if err != nil {
		t.Fatal(err)
	}

	assertFileContains(t, pluginPath, "RottaVelaFreshnessGuard")
	assertFileContains(t, hookPath, "Rotta Vela freshness guard")
	assertFileDoesNotContain(t, pluginPath, "stale opencode guard")
	assertFileDoesNotContain(t, hookPath, "stale claude guard")
}

func TestSCN002_VelaFreshnessGuardContentTargetsGraphQueriesOnly(t *testing.T) {
	// REQ-004 → SCN-002 → TestSCN002_VelaFreshnessGuardContentTargetsGraphQueriesOnly
	// Scenario: Successful install cleans previous rotta settings before fresh install
	home := t.TempDir()
	pluginPath := filepath.Join(home, ".config", "opencode", "plugin", "rotta-vela-freshness-guard.js")
	hookPath := filepath.Join(home, ".claude", "hooks", "rotta-vela-freshness-guard.sh")

	plugin, err := os.ReadFile(pluginPath)
	if !os.IsNotExist(err) || plugin != nil {
		t.Fatalf("guard content test should start without generated files, err=%v", err)
	}

	writeTestFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"), []byte(`{}`))
	if _, err := installOpenCodeVelaFreshnessGuard(home); err != nil {
		t.Fatal(err)
	}
	if _, err := installClaudeCodeVelaFreshnessGuard(home); err != nil {
		t.Fatal(err)
	}

	pluginContent := readFileString(t, pluginPath)
	hookContent := readFileString(t, hookPath)
	for _, content := range []struct {
		name string
		text string
	}{
		{name: "opencode plugin", text: pluginContent},
		{name: "claude hook", text: hookContent},
	} {
		for _, want := range []string{
			"vela update",
			"vela build",
			"vela_status",
			"isVelaGraphQueryTool",
			"vela_explore",
			"vela_lookup",
			"vela_dependencies",
			"ancora_vela_",
			"mcp__vela__",
		} {
			if !strings.Contains(content.text, want) {
				t.Fatalf("expected %s to contain %q:\n%s", content.name, want, content.text)
			}
		}
	}
	assertFileContains(t, pluginPath, "toolName.includes(\"status\")")
	assertFileContains(t, hookPath, "return 1")
}

func TestSCN002_ClaudeVelaFreshnessHookSkipsNonQueriesAndBuildsWhenUpdateFails(t *testing.T) {
	// REQ-004 → SCN-002 → TestSCN002_ClaudeVelaFreshnessHookSkipsNonQueriesAndBuildsWhenUpdateFails
	// Scenario: Successful install cleans previous rotta settings before fresh install
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	binDir := filepath.Join(home, "bin")
	logPath := filepath.Join(home, "vela.log")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("CLAUDE_PROJECT_DIR", projectPath)

	writeExecutable(t, filepath.Join(binDir, "vela"), `#!/bin/sh
printf '%s %s\n' "$1" "$2" >> "$HOME/vela.log"
if [ "$1" = update ]; then
  exit 42
fi
exit 0
`)
	if _, err := installClaudeCodeVelaFreshnessGuard(home); err != nil {
		t.Fatal(err)
	}
	hookPath := filepath.Join(home, ".claude", "hooks", "rotta-vela-freshness-guard.sh")

	runHook(t, hookPath, `{"tool_name":"Read","cwd":"`+projectPath+`"}`)
	runHook(t, hookPath, `{"tool_name":"mcp__vela__status","cwd":"`+projectPath+`"}`)
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatalf("expected non-query and vela_status hooks not to run vela, stat err=%v", err)
	}

	runHook(t, hookPath, `{"tool_name":"mcp__vela__dependencies","cwd":"`+projectPath+`"}`)
	log := readFileString(t, logPath)
	if !strings.Contains(log, "update "+projectPath) || !strings.Contains(log, "build "+projectPath) {
		t.Fatalf("expected hook to try update then build, got log %q", log)
	}
}

func TestSCN002_ClaudeVelaFreshnessHookRegistrationAvoidsDuplicates(t *testing.T) {
	// REQ-004 → SCN-002 → TestSCN002_ClaudeVelaFreshnessHookRegistrationAvoidsDuplicates
	// Scenario: Successful install cleans previous rotta settings before fresh install
	home := t.TempDir()
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	hookPath := filepath.Join(home, ".claude", "hooks", "rotta-vela-freshness-guard.sh")

	if err := addClaudeCodeVelaFreshnessHooks(settingsPath, hookPath); err != nil {
		t.Fatal(err)
	}
	if err := addClaudeCodeVelaFreshnessHooks(settingsPath, hookPath); err != nil {
		t.Fatal(err)
	}

	settings := readJSONFile(t, settingsPath)
	hooks := settings["hooks"].(map[string]interface{})
	if countClaudeHookCommandsContaining(hooks, "SessionStart", hookPath) != 1 {
		t.Fatalf("expected one SessionStart guard hook, got %#v", hooks["SessionStart"])
	}
	if countClaudeHookCommandsContaining(hooks, "PreToolUse", hookPath) != 1 {
		t.Fatalf("expected one PreToolUse guard hook, got %#v", hooks["PreToolUse"])
	}
}

func assertStringListContains(t *testing.T, values []string, want string) {
	t.Helper()
	for _, value := range values {
		if value == want {
			return
		}
	}
	t.Fatalf("expected list to contain %q, got %#v", want, values)
}

func countJSONListOccurrences(values []interface{}, want string) int {
	count := 0
	for _, value := range values {
		if value == want {
			count++
		}
	}
	return count
}

func assertClaudeHookCommandContains(t *testing.T, hooks map[string]interface{}, event, want string) {
	t.Helper()
	if countClaudeHookCommandsContaining(hooks, event, want) == 0 {
		t.Fatalf("expected %s hooks to contain command %q, got %#v", event, want, hooks[event])
	}
}

func countClaudeHookCommandsContaining(hooks map[string]interface{}, event, want string) int {
	entries, _ := hooks[event].([]interface{})
	count := 0
	for _, entry := range entries {
		entryMap, _ := entry.(map[string]interface{})
		hookList, _ := entryMap["hooks"].([]interface{})
		for _, hook := range hookList {
			hookMap, _ := hook.(map[string]interface{})
			command, _ := hookMap["command"].(string)
			if strings.Contains(command, want) {
				count++
			}
		}
	}
	return count
}

func assertClaudePreToolMatcherContains(t *testing.T, hooks map[string]interface{}, want string) {
	t.Helper()
	entries, _ := hooks["PreToolUse"].([]interface{})
	for _, entry := range entries {
		entryMap, _ := entry.(map[string]interface{})
		matcher, _ := entryMap["matcher"].(string)
		if strings.Contains(matcher, want) {
			return
		}
	}
	t.Fatalf("expected PreToolUse matcher to contain %q, got %#v", want, hooks["PreToolUse"])
}

func assertFileDoesNotContain(t *testing.T, path, unwanted string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	if strings.Contains(string(data), unwanted) {
		t.Fatalf("expected %s not to contain %q, got %q", path, unwanted, string(data))
	}
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	return string(data)
}

func runHook(t *testing.T, hookPath, input string) {
	t.Helper()
	cmd := exec.Command(hookPath)
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run hook %s with %s: %v\n%s", hookPath, input, err, out)
	}
}

func TestSCN002_OpenCodePluginRegistrationAvoidsDuplicates(t *testing.T) {
	// REQ-004 → SCN-002 → TestSCN002_OpenCodePluginRegistrationAvoidsDuplicates
	// Scenario: Successful install cleans previous rotta settings before fresh install
	config := map[string]interface{}{
		"plugin": []interface{}{"user-plugin", "guard.js", "guard.js"},
	}
	addOpenCodePluginEntry(config, "guard.js")

	encoded, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(encoded), "guard.js") != 1 {
		t.Fatalf("expected guard plugin once after dedupe, got %s", encoded)
	}
}
