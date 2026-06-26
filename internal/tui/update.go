package tui

import (
	"github.com/Syfra3/clean-workflow/internal/installer"
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
	switch m.Screen {
	case ScreenWelcome:
		return m.updateWelcome(msg)
	case ScreenTargetSelect:
		return m.updateTargetSelect(msg)
	case ScreenProjectPath:
		return m.updateProjectPath(msg)
	case ScreenModeSelect:
		return m.updateModeSelect(msg)
	case ScreenQualityGates:
		return m.updateQualityGates(msg)
	case ScreenAncora:
		return m.updateAncora(msg)
	case ScreenConfirm:
		return m.updateConfirm(msg)
	case ScreenSuccess, ScreenError:
		return m.updateDone(msg)
	}
	return m, nil
}

func (m Model) updateWelcome(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", " ":
		m.PrevScreen = ScreenWelcome
		m.Screen = ScreenTargetSelect
	case "q":
		return m, tea.Quit
	}
	return m, nil
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
		anySelected := false
		for _, v := range m.SelectedModes {
			if v {
				anySelected = true
				break
			}
		}
		if anySelected {
			m.PrevScreen = ScreenModeSelect
			m.Screen = ScreenQualityGates
		}
	case "esc", "b":
		m.Screen = ScreenProjectPath
	}
	return m, nil
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
		m.Screen = ScreenConfirm
	case "esc", "b":
		m.Screen = ScreenQualityGates
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
		m.Screen = ScreenQualityGates
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
		}
		result, err := installer.Install(opts)
		return installDoneMsg{result: result, err: err}
	}
}
