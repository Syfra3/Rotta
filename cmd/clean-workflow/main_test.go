package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSCN010_CLIInstallCannotSkipBackupDuringNormalUsage(t *testing.T) {
	// REQ-005, REQ-010 → SCN-010 → TestSCN010_CLIInstallCannotSkipBackupDuringNormalUsage
	// Scenario: CLI install path cannot skip backup during normal usage
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	t.Setenv("HOME", home)

	preInstallConfig := []byte(`{"agent":{"clean-spec":{"description":"stale"},"user-agent":{"description":"keep"}}}`)
	writeCLITestFile(t, filepath.Join(home, ".config", "opencode", "opencode.json"), preInstallConfig)

	var stdout bytes.Buffer
	err := runCLI([]string{"install", "--target", "opencode", "--project", projectPath, "--spec"}, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}

	backupDir := singleCLIBackupDir(t, filepath.Join(home, ".clean-workflow", "backups"))
	manifest := readCLIBackupManifest(t, filepath.Join(backupDir, "manifest.json"))
	if manifest["status"] != "complete" {
		t.Fatalf("expected complete backup manifest, got %#v", manifest)
	}
	if !strings.Contains(stdout.String(), backupDir) {
		t.Fatalf("expected install output to include backup location %s, got %q", backupDir, stdout.String())
	}
	backupConfig := filepath.Join(backupDir, "files", "home", ".config", "opencode", "opencode.json")
	data, err := os.ReadFile(backupConfig)
	if err != nil {
		t.Fatalf("read backed-up config: %v", err)
	}
	if string(data) != string(preInstallConfig) {
		t.Fatalf("expected CLI install backup to capture config before cleanup/install, got %s", data)
	}

	err = runCLI([]string{"install", "--skip-backup", "--target", "opencode", "--project", projectPath, "--spec"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected CLI install to reject backup-skipping option")
	}
	if !strings.Contains(err.Error(), "skip-backup") {
		t.Fatalf("expected rejected option to identify skip-backup, got %v", err)
	}

	oldVersion := version
	version = "test-version"
	t.Cleanup(func() { version = oldVersion })
	var versionOut bytes.Buffer
	if err := runCLI([]string{"--version"}, &versionOut, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(versionOut.String(), "test-version") {
		t.Fatalf("expected version output to remain available, got %q", versionOut.String())
	}
}

func TestSCN005_CLIBackupRestoreCommandsAreDiscoverableAndUnknownCommandsFail(t *testing.T) {
	// REQ-005 → SCN-005 → TestSCN005_CLIBackupRestoreCommandsAreDiscoverableAndUnknownCommandsFail
	// Scenario: CLI exposes backup and restore commands and rejects unknown commands
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	t.Setenv("HOME", home)
	configPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	writeCLITestFile(t, configPath, []byte(`{"before":true}`))

	var backupOut bytes.Buffer
	if err := runCLI([]string{"backup", "--target", "opencode", "--project", projectPath, "--spec"}, &backupOut, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	backupDir := singleCLIBackupDir(t, filepath.Join(home, ".clean-workflow", "backups"))
	if !strings.Contains(backupOut.String(), backupDir) {
		t.Fatalf("expected backup command output to include backup location %s, got %q", backupDir, backupOut.String())
	}

	writeCLITestFile(t, configPath, []byte(`{"after":true}`))
	var restoreOut bytes.Buffer
	if err := runCLI([]string{"restore", backupDir}, &restoreOut, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read restored config: %v", err)
	}
	if string(data) != `{"before":true}` {
		t.Fatalf("expected restore command to restore backup content, got %s", data)
	}
	if !strings.Contains(restoreOut.String(), backupDir) {
		t.Fatalf("expected restore command output to include selected backup %s, got %q", backupDir, restoreOut.String())
	}

	if err := runCLI([]string{"unknown"}, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Fatal("expected unknown command to fail")
	}
}

func writeCLITestFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func singleCLIBackupDir(t *testing.T, backupRoot string) string {
	t.Helper()
	entries, err := os.ReadDir(backupRoot)
	if err != nil {
		t.Fatalf("read backup root: %v", err)
	}
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, filepath.Join(backupRoot, entry.Name()))
		}
	}
	if len(dirs) != 1 {
		t.Fatalf("expected one backup directory, got %#v", dirs)
	}
	return dirs[0]
}

func readCLIBackupManifest(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest map[string]interface{}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	return manifest
}
