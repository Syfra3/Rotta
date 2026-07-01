package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestViewConfirmRendersAncoraVelaCombinations(t *testing.T) {
	tests := []struct {
		name        string
		ancora      bool
		vela        bool
		want        []string
		notExpected []string
	}{
		{
			name:   "ancora and vela enabled",
			ancora: true,
			vela:   true,
			want: []string{
				"Ancora memory:",
				"Vela graph:",
				"yes (install + configure)",
				"~/.claude/mcp/ancora.json",
				"<project>/.vela/graph.db  (initialized, not extracted)",
			},
			notExpected: []string{"~/.claude/vela-mcp.json"},
		},
		{
			name:   "ancora enabled and vela disabled",
			ancora: true,
			vela:   false,
			want: []string{
				"Ancora memory:",
				"yes (install + configure)",
				"Vela graph:",
				"skip",
				"~/.claude/mcp/ancora.json",
			},
			notExpected: []string{"<project>/.vela/graph.db", "~/.claude/vela-mcp.json"},
		},
		{
			name:   "ancora disabled and vela enabled",
			ancora: false,
			vela:   true,
			want: []string{
				"Ancora memory:",
				"skip",
				"Vela graph:",
				"yes (install + configure)",
				"<project>/.vela/graph.db  (initialized, not extracted)",
				"~/.claude/vela-mcp.json",
			},
			notExpected: []string{"~/.claude/mcp/ancora.json"},
		},
		{
			name:   "ancora and vela disabled",
			ancora: false,
			vela:   false,
			want: []string{
				"Ancora memory:",
				"Vela graph:",
				"skip",
			},
			notExpected: []string{"~/.claude/mcp/ancora.json", "<project>/.vela/graph.db", "~/.claude/vela-mcp.json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := New()
			model.Screen = ScreenConfirm
			model.Target = TargetClaudeCode
			model.ProjectPath = "/tmp/project"
			model.SetupAncora = tt.ancora
			model.SetupVela = tt.vela

			got := model.viewConfirm()
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Fatalf("missing %q:\n%s", want, got)
				}
			}
			for _, unwanted := range tt.notExpected {
				if strings.Contains(got, unwanted) {
					t.Fatalf("unexpected %q:\n%s", unwanted, got)
				}
			}
		})
	}
}

func TestSCN002_TUIVelaCopyMentionsFreshnessGuard(t *testing.T) {
	// REQ-004 → SCN-002 → TestSCN002_TUIVelaCopyMentionsFreshnessGuard
	// Scenario: Successful install cleans previous rotta settings before fresh install
	model := New()
	model.Screen = ScreenVela
	model.Target = TargetBoth
	model.SetupAncora = true

	velaView := model.viewVela()
	for _, want := range []string{
		"freshness guard",
		"updates or builds stale graphs before graph queries",
		"OpenCode plugin",
		"Claude Code hooks",
	} {
		if !strings.Contains(velaView, want) {
			t.Fatalf("expected Vela screen to mention %q:\n%s", want, velaView)
		}
	}

	model.Screen = ScreenConfirm
	model.SetupVela = true
	confirmView := model.viewConfirm()
	for _, want := range []string{
		"~/.config/opencode/plugin/rotta-vela-freshness-guard.js",
		"~/.claude/hooks/rotta-vela-freshness-guard.sh",
		"graph freshness guard",
	} {
		if !strings.Contains(confirmView, want) {
			t.Fatalf("expected confirm screen to mention %q:\n%s", want, confirmView)
		}
	}
}

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

	msg := runInstall(model)()
	done, ok := msg.(installDoneMsg)
	if !ok {
		t.Fatalf("expected installDoneMsg, got %T", msg)
	}
	if done.err != nil {
		t.Fatalf("expected non-interactive TUI install to complete, got %v", done.err)
	}
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
