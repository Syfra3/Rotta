package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSCN004_TUIListsAvailableBackupsFromRecovery(t *testing.T) {
	// REQ-006 → SCN-004 → TestSCN004_TUIListsAvailableBackupsFromRecovery
	// Scenario: TUI lists available backups from recovery
	home := t.TempDir()
	t.Setenv("HOME", home)
	writeBackupManifest(t, home, "20260629T120000Z", `/tmp/project-alpha`, "both")
	writeBackupManifest(t, home, "20260629T121500Z", `/tmp/project-beta`, "opencode")

	model := New()
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	view := updated.(Model).View()

	for _, want := range []string{
		"Recovery",
		"20260629T120000Z",
		"/tmp/project-alpha",
		"both",
		"20260629T121500Z",
		"/tmp/project-beta",
		"opencode",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected recovery view to contain %q:\n%s", want, view)
		}
	}
}

func TestSCN005_TUIPreviewsBackupContentsAndMetadata(t *testing.T) {
	// REQ-006, REQ-009 → SCN-005 → TestSCN005_TUIPreviewsBackupContentsAndMetadata
	// Scenario: TUI previews backup contents and metadata
	home := t.TempDir()
	t.Setenv("HOME", home)
	projectPath := filepath.Join(home, "project with spaces")
	writeBackupManifest(t, home, "20260629T123000Z", projectPath, "both")

	model := New()
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	preview, _ := updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	view := preview.(Model).View()

	for _, want := range []string{
		"Backup preview",
		"20260629T123000Z",
		projectPath,
		"both",
		"Spec, Review",
		"Ancora",
		"Vela: no",
		filepath.Join(projectPath, ".rotta", "state-machine.yaml"),
		filepath.Join(projectPath, ".vela", "graph.db"),
		"full-backup restore only",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected backup preview to contain %q:\n%s", want, view)
		}
	}
}

func TestSCN006_TUIRequiresConfirmationBeforeFullRestore(t *testing.T) {
	// REQ-006, REQ-007 → SCN-006 → TestSCN006_TUIRequiresConfirmationBeforeFullRestore
	// Scenario: TUI requires confirmation before full restore
	home := t.TempDir()
	t.Setenv("HOME", home)
	projectPath := filepath.Join(home, "project")
	writeBackupManifest(t, home, "20260629T124500Z", projectPath, "both")

	model := New()
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	preview, _ := updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	confirm, cmd := preview.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	confirmedModel := confirm.(Model)
	view := confirmedModel.View()

	if cmd != nil {
		t.Fatalf("restore choice should ask for confirmation before starting work")
	}
	if confirmedModel.Screen != ScreenRecoveryConfirm {
		t.Fatalf("expected restore confirmation screen, got %v", confirmedModel.Screen)
	}
	for _, want := range []string{
		"Confirm full restore",
		"20260629T124500Z",
		projectPath,
		"Restore has not started",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected restore confirmation to contain %q:\n%s", want, view)
		}
	}
}

func TestSCN007_TUIConfirmationExecutesFullRestore(t *testing.T) {
	// REQ-006, REQ-007 → SCN-007 → TestSCN007_TUIConfirmationExecutesFullRestore
	// Scenario: TUI confirmation executes a full restore
	home := t.TempDir()
	t.Setenv("HOME", home)
	projectPath := filepath.Join(home, "project")
	configPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	missingPath := filepath.Join(projectPath, ".vela", "graph.db")
	backupDir := writeRestorableBackupManifest(t, home, "20260629T130000Z", projectPath, "opencode", configPath, missingPath)
	writeTestFile(t, configPath, []byte(`{"current":true}`))
	writeTestFile(t, missingPath, []byte("remove during restore"))

	model := New()
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	preview, _ := updated.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	confirm, _ := preview.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	restoring, cmd := confirm.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})

	if cmd == nil {
		t.Fatal("expected confirmation key to start restore command")
	}
	if restoring.(Model).Screen != ScreenInstalling {
		t.Fatalf("expected restore to show progress screen, got %v", restoring.(Model).Screen)
	}
	cmd()
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read restored file: %v", err)
	}
	if string(data) != `{"restored":true}` {
		t.Fatalf("expected TUI restore to apply selected backup %s, got %s", backupDir, data)
	}
	if _, err := os.Stat(missingPath); !os.IsNotExist(err) {
		t.Fatalf("expected full restore to remove path absent from backup, stat err=%v", err)
	}
}

func TestSCN023_TUIRunInstallUsesNonInteractiveExternalCommandInput(t *testing.T) {
	// REQ-004 → SCN-023 → TestSCN023_TUIRunInstallUsesNonInteractiveExternalCommandInput
	// Scenario: TUI install must not let external setup tools read from the Bubble Tea terminal.
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	binDir := filepath.Join(home, "bin")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	writeTUITestExecutable(t, filepath.Join(binDir, "ancora"), `#!/bin/sh
if [ "$1" = setup ]; then
  if IFS= read -r line; then
    echo "unexpected interactive stdin: $line" >&2
    exit 23
  fi
  echo "external setup output should be discarded"
  exit 0
fi
`)

	model := New()
	model.Target = TargetOpenCode
	model.ProjectPath = projectPath
	model.SelectedModes = [3]bool{true, false, false}
	model.SetupAncora = true
	model.SetupVela = false
	model.SetupContext7 = false

	msg := runInstall(model)()
	done, ok := msg.(installDoneMsg)
	if !ok {
		t.Fatalf("expected installDoneMsg, got %T", msg)
	}
	if done.err != nil {
		t.Fatalf("expected non-interactive TUI install to complete, got %v", done.err)
	}
}

func TestSCN002_TUIRunInstallMaintainsHomebrewVelaBeforeSetup(t *testing.T) {
	// REQ-004 → SCN-002 → TestSCN002_TUIRunInstallMaintainsHomebrewVelaBeforeSetup
	// Scenario: TUI install refreshes Homebrew package metadata and upgrades an existing Vela before configuring integration.
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	binDir := filepath.Join(home, "bin")
	logPath := filepath.Join(home, "setup.log")
	t.Setenv("HOME", home)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	writeTUITestExecutable(t, filepath.Join(binDir, "vela"), `#!/bin/sh
printf 'vela %s\n' "$*" >> "$HOME/setup.log"
`)
	writeTUITestExecutable(t, filepath.Join(binDir, "brew"), `#!/bin/sh
printf 'brew %s\n' "$*" >> "$HOME/setup.log"
`)

	model := New()
	model.Target = TargetOpenCode
	model.ProjectPath = projectPath
	model.SelectedModes = [3]bool{true, false, false}
	model.SetupAncora = false
	model.SetupVela = true
	model.SetupContext7 = false

	msg := runInstall(model)()
	done, ok := msg.(installDoneMsg)
	if !ok {
		t.Fatalf("expected installDoneMsg, got %T", msg)
	}
	if done.err != nil {
		t.Fatalf("expected TUI Vela setup to complete, got %v", done.err)
	}

	assertTUIFileContains(t, logPath, "brew tap Syfra3/tap")
	assertTUIFileContains(t, logPath, "brew update")
	assertTUIFileContains(t, logPath, "brew upgrade vela")
	assertTUIFileContains(t, logPath, "vela install --project "+projectPath+" --agent opencode")
}

func writeBackupManifest(t *testing.T, home, timestamp, projectPath, target string) {
	t.Helper()
	dir := filepath.Join(home, ".rotta", "backups", timestamp)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"version":1,"timestamp":"` + timestamp + `","project_path":"` + projectPath + `","target":"` + target + `","selected_modes":{"spec":true,"implementation":false,"review":true},"optional_integrations":{"ancora":true,"vela":false},"backed_up_paths":["` + filepath.Join(projectPath, ".rotta", "state-machine.yaml") + `"],"missing_paths":["` + filepath.Join(projectPath, ".vela", "graph.db") + `"],"status":"complete"}`
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeTUITestExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

func assertTUIFileContains(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	if !strings.Contains(string(data), want) {
		t.Fatalf("expected %s to contain %q, got %q", path, want, string(data))
	}
}

func writeRestorableBackupManifest(t *testing.T, home, timestamp, projectPath, target, backedUpPath, missingPath string) string {
	t.Helper()
	dir := filepath.Join(home, ".rotta", "backups", timestamp)
	backupFile := filepath.Join(dir, "files", "home", strings.TrimPrefix(backedUpPath, home+string(os.PathSeparator)))
	writeTestFile(t, backupFile, []byte(`{"restored":true}`))
	manifest := `{"version":1,"timestamp":"` + timestamp + `","project_path":"` + projectPath + `","target":"` + target + `","selected_modes":{"spec":true,"implementation":false,"review":true},"optional_integrations":{"ancora":true,"vela":false},"backed_up_paths":["` + backedUpPath + `"],"missing_paths":["` + missingPath + `"],"status":"complete"}`
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
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
