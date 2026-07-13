package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/Syfra3/Rotta/internal/installer"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		}
		return m.handleKey(msg)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.InstallSpinner, cmd = m.InstallSpinner.Update(msg)
		return m, cmd

	case installDoneMsg:
		if msg.err != nil {
			m.InstallResult = msg.result
			m.InstallError = msg.err.Error()
			m.Screen = ScreenError
		} else {
			m.InstallResult = msg.result
			m.Screen = ScreenSuccess
		}
		m.Installing = false
		return m, nil
	}

	// Forward textinput events on ScreenProjectPath
	if m.Screen == ScreenProjectPath {
		var cmd tea.Cmd
		m.ProjectInput, cmd = m.ProjectInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if handler, ok := m.keyHandler(); ok {
		return handler(msg)
	}
	return m, nil
}

func (m Model) keyHandler() (func(tea.KeyMsg) (tea.Model, tea.Cmd), bool) {
	handlers := map[Screen]func(tea.KeyMsg) (tea.Model, tea.Cmd){
		ScreenWelcome: m.updateWelcome, ScreenTargetSelect: m.updateTargetSelect, ScreenProjectPath: m.updateProjectPath,
		ScreenModeSelect: m.updateModeSelect, ScreenQualityGates: m.updateQualityGates, ScreenAncora: m.updateAncora,
		ScreenVela: m.updateVela, ScreenContext7: m.updateContext7, ScreenConfirm: m.updateConfirm,
		ScreenSuccess: m.updateDone, ScreenError: m.updateDone, ScreenRecoveryList: m.updateRecoveryList,
		ScreenRecoveryPreview: m.updateRecoveryPreview, ScreenRecoveryConfirm: m.updateRecoveryConfirm,
	}
	handler, ok := handlers[m.Screen]
	return handler, ok
}

func (m Model) updateWelcome(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", " ":
		m.PrevScreen = ScreenWelcome
		m.Screen = ScreenTargetSelect
	case "r":
		m.PrevScreen = ScreenWelcome
		m.Screen = ScreenRecoveryList
		m.RecoveryBackups, m.RecoveryError = loadRecoveryBackups()
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) updateRecoveryList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.RecoveryCursor < len(m.RecoveryBackups)-1 {
			m.RecoveryCursor++
		}
	case "k", "up":
		if m.RecoveryCursor > 0 {
			m.RecoveryCursor--
		}
	case "enter", " ":
		if len(m.RecoveryBackups) > 0 {
			m.Screen = ScreenRecoveryPreview
		}
	case "esc", "b":
		m.Screen = ScreenWelcome
	}
	return m, nil
}

func (m Model) updateRecoveryPreview(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "r":
		m.Screen = ScreenRecoveryConfirm
	case "esc", "b":
		m.Screen = ScreenRecoveryList
	}
	return m, nil
}

func (m Model) updateRecoveryConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "enter":
		if len(m.RecoveryBackups) == 0 || m.RecoveryCursor >= len(m.RecoveryBackups) {
			return m, nil
		}
		backupDir := m.RecoveryBackups[m.RecoveryCursor].BackupDir
		m.Installing = true
		m.Screen = ScreenInstalling
		return m, restoreBackupCmd(backupDir)
	case "esc", "b":
		m.Screen = ScreenRecoveryPreview
	}
	return m, nil
}

func restoreBackupCmd(backupDir string) tea.Cmd {
	return func() tea.Msg {
		_, err := installer.RestoreBackup(backupDir)
		return installDoneMsg{
			result: &installer.Result{Target: "restore", Files: []string{"Restored backup: " + backupDir}},
			err:    err,
		}
	}
}

func (m Model) updateTargetSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.TargetCursor < len(targets)-1 {
			m.TargetCursor++
		}
	case "k", "up":
		if m.TargetCursor > 0 {
			m.TargetCursor--
		}
	case "enter", " ":
		m.Target = targetKeys[m.TargetCursor]
		m.PrevScreen = ScreenTargetSelect
		m.Screen = ScreenProjectPath
		focusCmd := m.ProjectInput.Focus()
		return m, focusCmd
	case "esc", "b":
		m.Screen = ScreenWelcome
	}
	return m, nil
}

func (m Model) updateProjectPath(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		path := m.ProjectInput.Value()
		if path == "" {
			path = "~"
		}
		m.ProjectPath = path
		m.PrevScreen = ScreenProjectPath
		m.Screen = ScreenModeSelect
	case "esc", "b":
		m.Screen = ScreenTargetSelect
		m.ProjectInput.Blur()
	}
	return m, nil
}

func (m Model) updateModeSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.ModeCursor < len(modeNames)-1 {
			m.ModeCursor++
		}
	case "k", "up":
		if m.ModeCursor > 0 {
			m.ModeCursor--
		}
	case " ":
		m.SelectedModes[m.ModeCursor] = !m.SelectedModes[m.ModeCursor]
	case "enter":
		if hasSelectedModes(m.SelectedModes) {
			m.PrevScreen = ScreenModeSelect
			m.Screen = ScreenQualityGates
		}
	case "esc", "b":
		m.Screen = ScreenProjectPath
	}
	return m, nil
}

func hasSelectedModes(modes [3]bool) bool {
	for _, selected := range modes {
		if selected {
			return true
		}
	}
	return false
}

func (m Model) updateQualityGates(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.GatesCursor < 1 {
			m.GatesCursor++
		}
	case "k", "up":
		if m.GatesCursor > 0 {
			m.GatesCursor--
		}
	case "enter", " ":
		m.UseDefaults = m.GatesCursor == 0
		m.PrevScreen = ScreenQualityGates
		m.Screen = ScreenAncora
	case "esc", "b":
		m.Screen = ScreenModeSelect
	}
	return m, nil
}

func (m Model) updateAncora(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.AncoraCursor < 1 {
			m.AncoraCursor++
		}
	case "k", "up":
		if m.AncoraCursor > 0 {
			m.AncoraCursor--
		}
	case "enter", " ":
		m.SetupAncora = m.AncoraCursor == 0
		m.PrevScreen = ScreenAncora
		m.Screen = ScreenVela
	case "esc", "b":
		m.Screen = ScreenQualityGates
	}
	return m, nil
}

func (m Model) updateVela(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.VelaCursor < 1 {
			m.VelaCursor++
		}
	case "k", "up":
		if m.VelaCursor > 0 {
			m.VelaCursor--
		}
	case "enter", " ":
		m.SetupVela = m.VelaCursor == 0
		m.PrevScreen = ScreenVela
		m.Screen = ScreenContext7
	case "esc", "b":
		m.Screen = ScreenAncora
	}
	return m, nil
}

func (m Model) updateContext7(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.Context7Cursor < 1 {
			m.Context7Cursor++
		}
	case "k", "up":
		if m.Context7Cursor > 0 {
			m.Context7Cursor--
		}
	case "enter", " ":
		m.SetupContext7 = m.Context7Cursor == 0
		m.PrevScreen = ScreenContext7
		m.Screen = ScreenConfirm
	case "esc", "b":
		m.Screen = ScreenVela
	}
	return m, nil
}

func (m Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down", "tab":
		if m.ConfirmCursor < 1 {
			m.ConfirmCursor++
		}
	case "k", "up":
		if m.ConfirmCursor > 0 {
			m.ConfirmCursor--
		}
	case "enter", " ":
		if m.ConfirmCursor == 0 {
			return m, tea.Quit
		}
		m.Installing = true
		m.Screen = ScreenInstalling
		return m, tea.Batch(m.InstallSpinner.Tick, runInstall(m))
	case "esc", "b":
		m.Screen = ScreenContext7
	}
	return m, nil
}

func (m Model) updateDone(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "enter", "esc":
		return m, tea.Quit
	}
	return m, nil
}

// runInstall kicks off the installation as a Bubble Tea command.
func runInstall(m Model) tea.Cmd {
	return func() tea.Msg {
		opts := installer.Options{
			Target:          m.Target,
			ProjectPath:     m.ProjectPath,
			InstallSpec:     m.SelectedModes[0],
			InstallImpl:     m.SelectedModes[1],
			InstallReview:   m.SelectedModes[2],
			UseDefaultGates: m.UseDefaults,
			SetupAncora:     m.SetupAncora,
			SetupVela:       m.SetupVela,
			SetupContext7:   m.SetupContext7,
			CommandStdin:    bytes.NewReader(nil),
			CommandStdout:   io.Discard,
			CommandStderr:   io.Discard,
		}
		result, err := installer.Install(opts)
		return installDoneMsg{result: result, err: err}
	}
}

func loadRecoveryBackups() ([]recoveryBackup, string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Sprintf("cannot resolve home directory: %v", err)
	}
	root := filepath.Join(home, ".rotta", "backups")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ""
		}
		return nil, fmt.Sprintf("cannot read backups: %v", err)
	}

	var backups []recoveryBackup
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		backupDir := filepath.Join(root, entry.Name())
		backup, ok := readRecoveryBackup(filepath.Join(backupDir, "manifest.json"))
		if ok {
			backup.BackupDir = backupDir
			backups = append(backups, backup)
		}
	}
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Timestamp > backups[j].Timestamp
	})
	return backups, ""
}

func readRecoveryBackup(path string) (recoveryBackup, bool) {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return recoveryBackup{}, false
	}
	defer root.Close()
	file, err := root.Open(filepath.Base(path))
	if err != nil {
		return recoveryBackup{}, false
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return recoveryBackup{}, false
	}
	var manifest struct {
		Timestamp            string                       `json:"timestamp"`
		ProjectPath          string                       `json:"project_path"`
		Target               string                       `json:"target"`
		SelectedModes        recoverySelectedModes        `json:"selected_modes"`
		OptionalIntegrations recoveryOptionalIntegrations `json:"optional_integrations"`
		BackedUpPaths        []string                     `json:"backed_up_paths"`
		MissingPaths         []string                     `json:"missing_paths"`
		Status               string                       `json:"status"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return recoveryBackup{}, false
	}
	if manifest.Status != "complete" || manifest.Timestamp == "" || manifest.ProjectPath == "" {
		return recoveryBackup{}, false
	}
	return recoveryBackup{
		Timestamp:            manifest.Timestamp,
		ProjectPath:          manifest.ProjectPath,
		Target:               manifest.Target,
		SelectedModes:        manifest.SelectedModes,
		OptionalIntegrations: manifest.OptionalIntegrations,
		BackedUpPaths:        manifest.BackedUpPaths,
		MissingPaths:         manifest.MissingPaths,
	}, true
}
