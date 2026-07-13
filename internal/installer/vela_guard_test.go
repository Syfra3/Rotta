package installer

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	assertFileContains(t, pluginPath, "scheduleVelaGraphRefresh")
	assertFileContains(t, pluginPath, "Vela refresh scheduled in background")
	assertFileContains(t, pluginPath, "vela update")
	assertFileContains(t, pluginPath, "vela build")
	assertFileDoesNotContain(t, pluginPath, "stale plugin should be backed up")
	assertFileDoesNotContain(t, pluginPath, "session.created")
	assertFileDoesNotContain(t, pluginPath, "spawnSync")
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
	home, result := installClaudeCodeWithVelaFreshnessGuard(t)
	hookPath := claudeCodeVelaFreshnessHookPath(home)
	assertClaudeVelaGuardFile(t, hookPath, result)
	assertClaudeVelaGuardSettings(t, filepath.Join(home, ".claude", "settings.json"), hookPath)
}

func installClaudeCodeWithVelaFreshnessGuard(t *testing.T) (string, *Result) {
	t.Helper()
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	binDir := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	writeTestFile(t, filepath.Join(home, ".claude", "settings.json"), []byte(`{
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
	return home, result
}

func assertClaudeVelaGuardFile(t *testing.T, hookPath string, result *Result) {
	t.Helper()
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
}

func assertClaudeVelaGuardSettings(t *testing.T, settingsPath, hookPath string) {
	t.Helper()
	settings := readJSONFile(t, settingsPath)
	if settings["theme"] != "dark" {
		t.Fatalf("expected unrelated Claude Code settings to be preserved, got %#v", settings)
	}
	hooks := settings["hooks"].(map[string]interface{})
	assertClaudeHookCommandContains(t, hooks, "SessionStart", "echo keep-session")
	if countClaudeHookCommandsContaining(hooks, "SessionStart", hookPath) != 0 {
		t.Fatalf("expected Claude guard not to register a SessionStart hook, got %#v", hooks["SessionStart"])
	}
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
	settings := readJSONFile(t, filepath.Join(home, ".claude", "settings.json"))
	hooks := settings["hooks"].(map[string]interface{})
	if countClaudeHookCommandsContaining(hooks, "SessionStart", hookPath) != 0 {
		t.Fatalf("expected reinstall cleanup to remove stale SessionStart guard hook, got %#v", hooks["SessionStart"])
	}
}

func TestSCN002_VelaFreshnessGuardContentTargetsGraphQueriesOnly(t *testing.T) {
	// REQ-004 → SCN-002 → TestSCN002_VelaFreshnessGuardContentTargetsGraphQueriesOnly
	// Scenario: Successful install cleans previous rotta settings before fresh install
	pluginPath, hookPath := installVelaGuardContent(t)
	assertVelaGuardQueryContent(t, pluginPath, hookPath)
}

func installVelaGuardContent(t *testing.T) (string, string) {
	t.Helper()
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
	return pluginPath, hookPath
}

func assertVelaGuardQueryContent(t *testing.T, pluginPath, hookPath string) {
	t.Helper()
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
			"Vela refresh scheduled in background",
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
	assertFileDoesNotContain(t, pluginPath, "session.created")
	assertFileDoesNotContain(t, pluginPath, "warmedSession")
	assertFileDoesNotContain(t, pluginPath, "event: async")
	assertFileContains(t, pluginPath, ".vela/registry.json")
	assertFileContains(t, pluginPath, "repo_root")
	assertFileContains(t, pluginPath, "homeRoot")
	assertFileContains(t, hookPath, "registry.json")
	assertFileContains(t, hookPath, "repo_root")
	assertFileContains(t, pluginPath, "spawn(")
	assertFileContains(t, pluginPath, "child.on(\"error\"")
	assertFileContains(t, pluginPath, "detached: true")
	assertFileContains(t, pluginPath, "scheduleVelaGraphRefresh")
	assertFileDoesNotContain(t, pluginPath, "spawnSync")
	assertFileDoesNotContain(t, pluginPath, "throw new Error")
	assertFileContains(t, hookPath, "schedule_refresh")
	assertFileContains(t, hookPath, "Vela refresh scheduled in background")
	assertFileContains(t, hookPath, "&")
	assertFileDoesNotContain(t, hookPath, "freshness guard blocked")
}

func TestSCN002_ClaudeVelaFreshnessHookSchedulesRegisteredWorkspaceRefreshInBackground(t *testing.T) {
	// REQ-004 → SCN-002 → TestSCN002_ClaudeVelaFreshnessHookSchedulesRegisteredWorkspaceRefreshInBackground
	// Scenario: Successful install cleans previous rotta settings before fresh install
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	nestedPath := filepath.Join(projectPath, "nested")
	binDir := filepath.Join(home, "bin")
	logPath := filepath.Join(home, "vela.log")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	writeTestFile(t, filepath.Join(home, ".vela", "registry.json"), []byte(`{
  "workspaces": [
    {"repo_root": "`+home+`"},
    {"repo_root": "`+projectPath+`"}
  ]
}`))

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

	runHook(t, hookPath, `{"tool_name":"Read","cwd":"`+nestedPath+`"}`)
	runHook(t, hookPath, `{"tool_name":"mcp__vela__status","cwd":"`+nestedPath+`"}`)
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatalf("expected non-query and vela_status hooks not to run vela, stat err=%v", err)
	}

	stdout, stderr := runHookOutput(t, hookPath, `{"tool_name":"mcp__vela__dependencies","cwd":"`+nestedPath+`"}`)
	if stdout != "" {
		t.Fatalf("expected hook stdout to stay clean, got %q", stdout)
	}
	if !strings.Contains(stderr, "Vela refresh scheduled in background") {
		t.Fatalf("expected scheduled feedback, got stderr %q", stderr)
	}
	log := waitForFileContains(t, logPath, "build "+projectPath)
	if !strings.Contains(log, "update "+projectPath) || !strings.Contains(log, "build "+projectPath) {
		t.Fatalf("expected hook to try update then build for registered project root, got log %q", log)
	}
	if strings.Contains(log, "update "+home+"\n") || strings.Contains(log, "build "+home+"\n") {
		t.Fatalf("expected hook not to refresh broad home root, got log %q", log)
	}
}

func TestSCN002_ClaudeVelaFreshnessHookPrintsScheduledFeedbackToStderrAndKeepsStdoutClean(t *testing.T) {
	// REQ-004 → SCN-002 → TestSCN002_ClaudeVelaFreshnessHookPrintsFeedbackToStderrAndKeepsStdoutClean
	// Scenario: Successful install cleans previous rotta settings before fresh install
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	binDir := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	writeTestFile(t, filepath.Join(home, ".vela", "registry.json"), []byte(`{"workspaces":[{"repo_root":"`+projectPath+`"}]}`))
	writeExecutable(t, filepath.Join(binDir, "vela"), `#!/bin/sh
printf '%s\n' "$1" >> "$HOME/vela.log"
if [ "$1" = update ]; then
  exit 42
fi
exit 0
`)
	if _, err := installClaudeCodeVelaFreshnessGuard(home); err != nil {
		t.Fatal(err)
	}
	hookPath := filepath.Join(home, ".claude", "hooks", "rotta-vela-freshness-guard.sh")

	stdout, stderr := runHookOutput(t, hookPath, `{"tool_name":"mcp__vela__dependencies","cwd":"`+projectPath+`"}`)
	if stdout != "" {
		t.Fatalf("expected Claude hook stdout to stay clean, got %q", stdout)
	}
	if !strings.Contains(stderr, "Rotta Vela refresh scheduled in background for "+projectPath) {
		t.Fatalf("expected concise scheduled feedback, got %q", stderr)
	}
	if strings.Contains(stderr, "stdout from vela") || strings.Contains(stderr, "stderr from vela") || strings.Contains(stderr, "update failed") {
		t.Fatalf("expected hook not to stream background refresh output, got %q", stderr)
	}
	waitForFileContains(t, filepath.Join(home, "vela.log"), "build")
}

func TestSCN002_ClaudeVelaFreshnessHookUsesXDGCacheSubdirectory(t *testing.T) {
	// REQ-004 → SCN-002 → TestSCN002_ClaudeVelaFreshnessHookUsesXDGCacheSubdirectory
	// Scenario: Successful install cleans previous rotta settings before fresh install
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	binDir := filepath.Join(home, "bin")
	xdgCacheHome := filepath.Join(home, "xdg-cache")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", xdgCacheHome)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	writeTestFile(t, filepath.Join(home, ".vela", "registry.json"), []byte(`{"workspaces":[{"repo_root":"`+projectPath+`"}]}`))
	writeExecutable(t, filepath.Join(binDir, "vela"), `#!/bin/sh
printf '%s\n' "$1" >> "$HOME/vela.log"
exit 0
`)
	if _, err := installClaudeCodeVelaFreshnessGuard(home); err != nil {
		t.Fatal(err)
	}
	hookPath := filepath.Join(home, ".claude", "hooks", "rotta-vela-freshness-guard.sh")

	runHook(t, hookPath, `{"tool_name":"mcp__vela__dependencies","cwd":"`+projectPath+`"}`)
	waitForFileContains(t, filepath.Join(home, "vela.log"), "update")
	cacheDir := filepath.Join(xdgCacheHome, "rotta-vela-freshness")
	assertPathExists(t, cacheDir)
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatalf("read cache dir %s: %v", cacheDir, err)
	}
	if len(entries) == 0 {
		t.Fatalf("expected debounce files under %s", cacheDir)
	}
	rootEntries, err := os.ReadDir(xdgCacheHome)
	if err != nil {
		t.Fatalf("read XDG cache home %s: %v", xdgCacheHome, err)
	}
	for _, entry := range rootEntries {
		if strings.HasSuffix(entry.Name(), ".stamp") || strings.HasSuffix(entry.Name(), ".lock") {
			t.Fatalf("expected debounce files under rotta-vela-freshness subdir, found %s in XDG cache root", entry.Name())
		}
	}
}

func TestSCN002_ClaudeVelaFreshnessHookSkipsWhenOnlyHomeRootMatches(t *testing.T) {
	// REQ-004 → SCN-002 → TestSCN002_ClaudeVelaFreshnessHookSkipsWhenOnlyHomeRootMatches
	// Scenario: Successful install cleans previous rotta settings before fresh install
	home := t.TempDir()
	binDir := filepath.Join(home, "bin")
	logPath := filepath.Join(home, "vela.log")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	writeTestFile(t, filepath.Join(home, ".vela", "registry.json"), []byte(`{"workspaces":[{"repo_root":"`+home+`"}]}`))
	writeExecutable(t, filepath.Join(binDir, "vela"), `#!/bin/sh
printf '%s %s\n' "$1" "$2" >> "$HOME/vela.log"
exit 0
`)
	if _, err := installClaudeCodeVelaFreshnessGuard(home); err != nil {
		t.Fatal(err)
	}
	hookPath := filepath.Join(home, ".claude", "hooks", "rotta-vela-freshness-guard.sh")

	runHook(t, hookPath, `{"tool_name":"mcp__vela__dependencies","cwd":"`+filepath.Join(home, "unregistered")+`"}`)
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatalf("expected query under home-only registry match not to run vela, stat err=%v", err)
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
	if countClaudeHookCommandsContaining(hooks, "SessionStart", hookPath) != 0 {
		t.Fatalf("expected no SessionStart guard hook, got %#v", hooks["SessionStart"])
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
	stdout, stderr := runHookOutput(t, hookPath, input)
	if stdout != "" || stderr != "" {
		return
	}
}

func runHookOutput(t *testing.T, hookPath, input string) (string, string) {
	t.Helper()
	cmd := exec.Command(hookPath)
	cmd.Stdin = strings.NewReader(input)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		t.Fatalf("run hook %s with %s: %v\nstdout: %s\nstderr: %s", hookPath, input, err, stdout.String(), stderr.String())
	}
	return stdout.String(), stderr.String()
}

func waitForFileContains(t *testing.T, path, want string) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	var last string
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			last = string(data)
			if strings.Contains(last, want) {
				return last
			}
		} else if !os.IsNotExist(err) {
			t.Fatalf("read %s while waiting for %q: %v", path, want, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s to contain %q; last content %q", path, want, last)
	return ""
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
