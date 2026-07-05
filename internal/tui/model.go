// Package tui implements the Bubbletea terminal UI for the Rotta installer.
//
// Patterns (Gentleman Bubbletea):
// - Screen constants as iota
// - Single Model struct holds ALL state
// - Update() with type switch
// - Per-screen key handlers returning (tea.Model, tea.Cmd)
// - Vim keys (j/k) for navigation
// - PrevScreen for back navigation
package tui

import (
	"github.com/Syfra3/Rotta/internal/installer"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── Screens ──────────────────────────────────────────────────────────────────

type Screen int

const (
	ScreenWelcome Screen = iota
	ScreenTargetSelect
	ScreenProjectPath
	ScreenModeSelect
	ScreenQualityGates
	ScreenAncora
	ScreenVela
	ScreenContext7
	ScreenConfirm
	ScreenInstalling
	ScreenSuccess
	ScreenError
	ScreenRecoveryList
	ScreenRecoveryPreview
	ScreenRecoveryConfirm
)

// ─── Targets ──────────────────────────────────────────────────────────────────

const (
	TargetClaudeCode = "claude-code"
	TargetOpenCode   = "opencode"
	TargetCodex      = "codex"
	TargetBoth       = "both"
)

// ─── Messages ─────────────────────────────────────────────────────────────────

type installDoneMsg struct {
	result *installer.Result
	err    error
}

type recoveryBackup struct {
	BackupDir            string
	Timestamp            string
	ProjectPath          string
	Target               string
	SelectedModes        recoverySelectedModes
	OptionalIntegrations recoveryOptionalIntegrations
	BackedUpPaths        []string
	MissingPaths         []string
}

type recoverySelectedModes struct {
	Spec           bool
	Implementation bool
	Review         bool
}

type recoveryOptionalIntegrations struct {
	Ancora   bool
	Vela     bool
	Context7 bool
}

// ─── Model ────────────────────────────────────────────────────────────────────

type Model struct {
	Screen     Screen
	PrevScreen Screen
	Width      int
	Height     int

	// Target selection
	TargetCursor int // 0=Claude Code, 1=OpenCode, 2=Codex, 3=Both
	Target       string

	// Project path
	ProjectInput textinput.Model
	ProjectPath  string

	// Mode selection: [0]=spec, [1]=implementation, [2]=review
	ModeCursor    int
	SelectedModes [3]bool

	// Quality gates
	GatesCursor int // 0=Use defaults, 1=Configure
	UseDefaults bool

	// Ancora memory
	AncoraCursor int  // 0=Install+configure, 1=Skip
	SetupAncora  bool // resolved choice

	// Vela graph intelligence
	VelaCursor int  // 0=Install+configure, 1=Skip
	SetupVela  bool // resolved choice

	// Context7 documentation MCP
	Context7Cursor int  // 0=Install+configure, 1=Skip
	SetupContext7  bool // resolved choice

	// Confirm
	ConfirmCursor int // 0=Cancel, 1=Install

	// Install
	Installing     bool
	InstallResult  *installer.Result
	InstallError   string
	InstallSpinner spinner.Model
	InstallSteps   []string

	RecoveryBackups []recoveryBackup
	RecoveryCursor  int
	RecoveryError   string
}

var targets = []string{"Claude Code", "OpenCode", "Codex", "Both"}
var targetKeys = []string{TargetClaudeCode, TargetOpenCode, TargetCodex, TargetBoth}
var modeNames = []string{"Spec Mode (Spec Partner + Gherkin Author)", "Implementation Mode (TDD Craftsman)", "Review Mode (Judge + Mutation Tester)"}
var modeDescriptions = []string{
	"Draft → Hard Spec → Gherkin → Human approval",
	"Red → Green → Refactor per Gherkin scenario",
	"Traceability → Coverage → Mutation → Quality gates",
}

func New() Model {
	ti := textinput.New()
	ti.Placeholder = "~/projects/my-feature"
	ti.CharLimit = 256
	ti.Width = 50

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colorLavender)

	return Model{
		Screen:         ScreenWelcome,
		TargetCursor:   0,
		Target:         TargetClaudeCode,
		ProjectInput:   ti,
		SelectedModes:  [3]bool{true, true, true},
		UseDefaults:    true,
		AncoraCursor:   0, // default to "Install + configure"
		SetupAncora:    true,
		VelaCursor:     0, // default to "Install + configure"
		SetupVela:      true,
		Context7Cursor: 0, // default to "Install + configure"
		SetupContext7:  true,
		ConfirmCursor:  1, // default to "Install", not "Cancel"
		InstallSpinner: sp,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.EnterAltScreen
}
