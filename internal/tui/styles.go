package tui

import "github.com/charmbracelet/lipgloss"

// ─── Colors (Syfra Identity Palette) ─────────────────────────────────────────

var (
	colorBase     = lipgloss.Color("#242426")
	colorSurface  = lipgloss.Color("#2a2a2d")
	colorOverlay  = lipgloss.Color("#4a4a4e")
	colorText     = lipgloss.Color("#e0e0e2")
	colorSubtext  = lipgloss.Color("#8a8a8e")
	colorLavender = lipgloss.Color("#C8B6FF")
	colorMint     = lipgloss.Color("#B4FFDD")
	colorGreen    = lipgloss.Color("#B4FFDD")
	colorPeach    = lipgloss.Color("#FFD4B8")
	colorRed      = lipgloss.Color("#FF9EB8")
	colorBlue     = lipgloss.Color("#9DB8FF")
	colorMauve    = lipgloss.Color("#E8C8FF")
	colorYellow   = lipgloss.Color("#FFF4B8")
)

// ─── Layout ───────────────────────────────────────────────────────────────────

var (
	appStyle = lipgloss.NewStyle().
			Foreground(colorText).
			Padding(1, 2)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorLavender).
			MarginBottom(1)

	helpStyle = lipgloss.NewStyle().
			Foreground(colorSubtext).
			MarginTop(1)

	errorStyle = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true).
			Padding(0, 1)

	successStyle = lipgloss.NewStyle().
			Foreground(colorGreen).
			Bold(true)

	warningStyle = lipgloss.NewStyle().
			Foreground(colorPeach).
			Bold(true)
)

// ─── Title ────────────────────────────────────────────────────────────────────

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorMauve).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(colorSubtext).
			Italic(true).
			MarginBottom(2)
)

// ─── Menu ─────────────────────────────────────────────────────────────────────

var (
	menuItemStyle = lipgloss.NewStyle().
			Foreground(colorText).
			PaddingLeft(2)

	menuSelectedStyle = lipgloss.NewStyle().
				Foreground(colorLavender).
				Bold(true).
				PaddingLeft(1)

	menuCheckStyle = lipgloss.NewStyle().
			Foreground(colorMint).
			Bold(true)

	menuUncheckedStyle = lipgloss.NewStyle().
				Foreground(colorSubtext)
)

// ─── Detail ───────────────────────────────────────────────────────────────────

var (
	labelStyle = lipgloss.NewStyle().
			Foreground(colorSubtext).
			Width(22).
			Align(lipgloss.Right).
			PaddingRight(1)

	valueStyle = lipgloss.NewStyle().
			Foreground(colorText)

	sectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorMauve).
			MarginTop(1).
			MarginBottom(1)

	cardStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorLavender).
			Padding(1, 2).
			MarginBottom(1)

	badgeStyle = lipgloss.NewStyle().
			Foreground(colorPeach).
			Bold(true)
)

// ─── Input ────────────────────────────────────────────────────────────────────

var (
	inputLabelStyle = lipgloss.NewStyle().
			Foreground(colorLavender).
			Bold(true).
			MarginBottom(1)

	inputHintStyle = lipgloss.NewStyle().
			Foreground(colorSubtext).
			Italic(true)
)

// ─── Progress ─────────────────────────────────────────────────────────────────

var (
	progressLabelStyle = lipgloss.NewStyle().
				Foreground(colorMint)

	progressDoneStyle = lipgloss.NewStyle().
				Foreground(colorGreen).
				Bold(true)
)

// ─── Unused suppressor ───────────────────────────────────────────────────────

var _ = colorBase
var _ = colorSurface
var _ = colorOverlay
var _ = colorBlue
var _ = colorYellow
