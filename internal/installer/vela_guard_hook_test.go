package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
