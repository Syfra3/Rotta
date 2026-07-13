package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

var _ = tea.Quit // suppress unused import

func (m Model) View() string {
	view, ok := m.screenViews()[m.Screen]
	if !ok {
		return ""
	}
	return view()
}

func (m Model) screenViews() map[Screen]func() string {
	return map[Screen]func() string{
		ScreenWelcome:         m.viewWelcome,
		ScreenTargetSelect:    m.viewTargetSelect,
		ScreenProjectPath:     m.viewProjectPath,
		ScreenModeSelect:      m.viewModeSelect,
		ScreenQualityGates:    m.viewQualityGates,
		ScreenAncora:          m.viewAncora,
		ScreenVela:            m.viewVela,
		ScreenContext7:        m.viewContext7,
		ScreenConfirm:         m.viewConfirm,
		ScreenInstalling:      m.viewInstalling,
		ScreenSuccess:         m.viewSuccess,
		ScreenError:           m.viewError,
		ScreenRecoveryList:    m.viewRecoveryList,
		ScreenRecoveryPreview: m.viewRecoveryPreview,
		ScreenRecoveryConfirm: m.viewRecoveryConfirm,
	}
}

func (m Model) viewWelcome() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Rotta Installer") + "\n")
	b.WriteString(subtitleStyle.Render("Contract-driven AI coding for Claude Code, OpenCode, and Codex") + "\n\n")

	b.WriteString(sectionStyle.Render("What this installs") + "\n")
	b.WriteString(menuItemStyle.Render("  Spec Mode        — Hard spec + Gherkin authoring with human approval gate") + "\n")
	b.WriteString(menuItemStyle.Render("  Implementation Mode — Strict TDD: Red → Green → Refactor per scenario") + "\n")
	b.WriteString(menuItemStyle.Render("  Review Mode       — Metrics-based quality gates, no line-by-line review") + "\n\n")

	b.WriteString(cardStyle.Render(
		warningStyle.Render("Philosophy")+"\n"+
			"  AI should not write code freely. It should be constrained\n"+
			"  by human-approved contracts, TDD loops, traceability,\n"+
			"  and measurable quality gates. The human manages the system\n"+
			"  at the level of behavior and risk — not implementation details.",
	) + "\n\n")

	b.WriteString(helpStyle.Render("Press Enter to start · r for recovery · q to quit"))
	return appStyle.Render(b.String())
}

func (m Model) viewRecoveryList() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("Recovery") + "\n\n")
	b.WriteString(sectionStyle.Render("Available backups") + "\n")

	if m.RecoveryError != "" {
		b.WriteString(errorStyle.Render(m.RecoveryError) + "\n\n")
	} else if len(m.RecoveryBackups) == 0 {
		b.WriteString(menuItemStyle.Render("  No valid backups found") + "\n\n")
	} else {
		for i, backup := range m.RecoveryBackups {
			prefix := "  "
			style := menuItemStyle
			if i == m.RecoveryCursor {
				prefix = "▸ "
				style = menuSelectedStyle
			}
			b.WriteString(style.Render(fmt.Sprintf("%s%s — %s — %s", prefix, backup.Timestamp, backup.ProjectPath, backup.Target)) + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("j/k to move · Enter to preview · Esc to go back"))
	return appStyle.Render(b.String())
}

func (m Model) viewRecoveryPreview() string {
	if len(m.RecoveryBackups) == 0 || m.RecoveryCursor >= len(m.RecoveryBackups) {
		return m.viewRecoveryList()
	}

	backup := m.RecoveryBackups[m.RecoveryCursor]
	var b strings.Builder
	b.WriteString(headerStyle.Render("Backup preview") + "\n\n")
	b.WriteString(labelStyle.Render("Timestamp:") + " " + valueStyle.Render(backup.Timestamp) + "\n")
	b.WriteString(labelStyle.Render("Project path:") + " " + valueStyle.Render(backup.ProjectPath) + "\n")
	b.WriteString(labelStyle.Render("Target:") + " " + valueStyle.Render(backup.Target) + "\n")
	b.WriteString(labelStyle.Render("Selected modes:") + " " + valueStyle.Render(formatRecoveryModes(backup.SelectedModes)) + "\n")
	b.WriteString(labelStyle.Render("Optional integrations:") + " " + valueStyle.Render(formatRecoveryIntegrations(backup.OptionalIntegrations)) + "\n\n")

	b.WriteString(sectionStyle.Render("Backed-up paths") + "\n")
	writeRecoveryPaths(&b, backup.BackedUpPaths)
	b.WriteString("\n")
	b.WriteString(sectionStyle.Render("Missing paths") + "\n")
	writeRecoveryPaths(&b, backup.MissingPaths)
	b.WriteString("\n")
	b.WriteString(warningStyle.Render("Restore is full-backup restore only") + "\n\n")
	b.WriteString(helpStyle.Render("r to restore · Esc to go back"))
	return appStyle.Render(b.String())
}

func (m Model) viewRecoveryConfirm() string {
	if len(m.RecoveryBackups) == 0 || m.RecoveryCursor >= len(m.RecoveryBackups) {
		return m.viewRecoveryList()
	}

	backup := m.RecoveryBackups[m.RecoveryCursor]
	var b strings.Builder
	b.WriteString(headerStyle.Render("Confirm full restore") + "\n\n")
	b.WriteString(labelStyle.Render("Backup:") + " " + valueStyle.Render(backup.Timestamp) + "\n")
	b.WriteString(labelStyle.Render("Project path:") + " " + valueStyle.Render(backup.ProjectPath) + "\n\n")
	b.WriteString(warningStyle.Render("Restore has not started") + "\n")
	b.WriteString(menuItemStyle.Render("This will restore the full backup after confirmation.") + "\n\n")
	b.WriteString(helpStyle.Render("y to confirm restore · Esc to go back"))
	return appStyle.Render(b.String())
}

func formatRecoveryModes(modes recoverySelectedModes) string {
	var selected []string
	if modes.Spec {
		selected = append(selected, "Spec")
	}
	if modes.Implementation {
		selected = append(selected, "Implementation")
	}
	if modes.Review {
		selected = append(selected, "Review")
	}
	if len(selected) == 0 {
		return "none"
	}
	return strings.Join(selected, ", ")
}

func formatRecoveryIntegrations(integrations recoveryOptionalIntegrations) string {
	ancora := "Ancora: no"
	if integrations.Ancora {
		ancora = "Ancora: yes"
	}
	vela := "Vela: no"
	if integrations.Vela {
		vela = "Vela: yes"
	}
	context7 := "Context7: no"
	if integrations.Context7 {
		context7 = "Context7: yes"
	}
	return ancora + ", " + vela + ", " + context7
}

func writeRecoveryPaths(b *strings.Builder, paths []string) {
	if len(paths) == 0 {
		b.WriteString(menuItemStyle.Render("  None") + "\n")
		return
	}
	for _, path := range paths {
		b.WriteString(menuItemStyle.Render("  "+path) + "\n")
	}
}

func (m Model) viewTargetSelect() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("Select Installation Target") + "\n\n")

	items := []struct {
		label string
		desc  string
	}{
		{"Claude Code", "SKILL.md files → ~/.claude/skills/rotta/"},
		{"OpenCode", "Agent entries + skill files for rotta-orchestrator, rotta-spec, rotta-impl, rotta-review"},
		{"Codex", "Codex instructions → ~/.codex/AGENTS.md"},
		{"Both", "Install for both tools"},
	}

	for i, item := range items {
		if m.TargetCursor == i {
			b.WriteString(menuSelectedStyle.Render("▸ "+item.label) + "\n")
			b.WriteString("    " + inputHintStyle.Render(item.desc) + "\n\n")
		} else {
			b.WriteString(menuItemStyle.Render("  "+item.label) + "\n\n")
		}
	}

	b.WriteString(helpStyle.Render("j/k to move · Enter to select · Esc to go back"))
	return appStyle.Render(b.String())
}

func (m Model) viewProjectPath() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("Project Path") + "\n\n")
	b.WriteString(inputLabelStyle.Render("Where is your project?") + "\n")
	b.WriteString(inputHintStyle.Render("Leave empty to use your home directory (~).") + "\n\n")
	b.WriteString(m.ProjectInput.View() + "\n\n")
	b.WriteString(helpStyle.Render("Enter to confirm · Esc to go back"))
	return appStyle.Render(b.String())
}

func (m Model) viewModeSelect() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("Select Workflow Modes") + "\n\n")
	b.WriteString(inputHintStyle.Render("Space to toggle · Enter to confirm") + "\n\n")

	for i, name := range modeNames {
		selected := m.SelectedModes[i]
		cursor := "  "
		checkMark := menuUncheckedStyle.Render("[ ]")
		if selected {
			checkMark = menuCheckStyle.Render("[✓]")
		}
		if m.ModeCursor == i {
			cursor = menuSelectedStyle.Render("▸ ")
			b.WriteString(fmt.Sprintf("%s%s %s\n", cursor, checkMark, menuSelectedStyle.Render(name)))
			b.WriteString("        " + inputHintStyle.Render(modeDescriptions[i]) + "\n\n")
		} else {
			b.WriteString(fmt.Sprintf("%s%s %s\n\n", cursor, checkMark, menuItemStyle.Render(name)))
		}
	}

	b.WriteString(helpStyle.Render("j/k to move · Space to toggle · Enter to continue · Esc to go back"))
	return appStyle.Render(b.String())
}

func (m Model) viewAncora() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("Ancora — Persistent Memory") + "\n\n")

	b.WriteString(sectionStyle.Render("What Ancora does") + "\n")
	b.WriteString(menuItemStyle.Render("  Persists workflow state across sessions (phase, approved scenarios, TDD logs)") + "\n")
	b.WriteString(menuItemStyle.Render("  Rotta agents call ancora_save / ancora_search to remember decisions") + "\n")
	b.WriteString(menuItemStyle.Render("  Survives compaction — the Judge can always recover prior run context") + "\n\n")

	b.WriteString(sectionStyle.Render("What gets configured") + "\n")
	if m.Target == "claude-code" || m.Target == "both" {
		b.WriteString(menuItemStyle.Render("  ~/.claude/mcp/ancora.json    — MCP server entry for Claude Code") + "\n")
		b.WriteString(menuItemStyle.Render("  ~/.claude/settings.json      — ancora_* tools added to permissions.allow") + "\n")
	}
	if m.Target == "opencode" || m.Target == "both" {
		b.WriteString(menuItemStyle.Render("  opencode.jsonc               — ancora MCP entry injected under [mcp]") + "\n")
	}
	b.WriteString("\n")

	b.WriteString(warningStyle.Render("Note: ") + inputHintStyle.Render("If Ancora is not installed, it will be installed via Homebrew.") + "\n\n")

	options := []struct{ label, desc string }{
		{"Install + configure Ancora (recommended)", "Install binary via Homebrew if needed, then write all MCP configs"},
		{"Skip", "Do not set up Ancora — agents will work but won't persist state between sessions"},
	}
	for i, opt := range options {
		if m.AncoraCursor == i {
			b.WriteString(menuSelectedStyle.Render("▸ "+opt.label) + "\n")
			b.WriteString("    " + inputHintStyle.Render(opt.desc) + "\n\n")
		} else {
			b.WriteString(menuItemStyle.Render("  "+opt.label) + "\n\n")
		}
	}

	b.WriteString(helpStyle.Render("j/k to move · Enter to select · Esc to go back"))
	return appStyle.Render(b.String())
}

func (m Model) viewContext7() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("Context7 — Optional Library/API Documentation MCP") + "\n\n")

	b.WriteString(sectionStyle.Render("Main optional MCP tools") + "\n")
	b.WriteString(menuItemStyle.Render("  Ancora — persistent memory") + "\n")
	b.WriteString(menuItemStyle.Render("  Vela — graph intelligence") + "\n")
	b.WriteString(menuItemStyle.Render("  Context7 — up-to-date library/API documentation through MCP") + "\n\n")

	b.WriteString(sectionStyle.Render("What gets configured") + "\n")
	b.WriteString(menuItemStyle.Render("  OpenCode MCP server: context7 → npx -y @upstash/context7-mcp") + "\n")
	b.WriteString(menuItemStyle.Render("  Claude Code MCP server: context7 → npx -y @upstash/context7-mcp") + "\n")
	b.WriteString(menuItemStyle.Render("  Health check initializes the MCP server and discovers documentation tools") + "\n\n")

	options := []struct{ label, desc string }{
		{"Install + configure Context7", "Checked by default; configure docs MCP for OpenCode and Claude Code"},
		{"Skip", "Do not configure Context7 or run Context7 checks"},
	}
	for i, opt := range options {
		if m.Context7Cursor == i {
			b.WriteString(menuSelectedStyle.Render("▸ "+opt.label) + "\n")
			b.WriteString("    " + inputHintStyle.Render(opt.desc) + "\n\n")
		} else {
			b.WriteString(menuItemStyle.Render("  "+opt.label) + "\n\n")
		}
	}

	b.WriteString(helpStyle.Render("j/k to move · Enter to select · Esc to go back"))
	return appStyle.Render(b.String())
}

func (m Model) viewQualityGates() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("Quality Gates Configuration") + "\n\n")

	defaults := []string{
		"Changed-line coverage    ≥ 90%",
		"Critical-branch coverage ≥ 95%",
		"Mutation score           ≥ 80% (≥ 90% for auth/payments)",
		"Cyclomatic complexity    ≤ 10 per function",
		"Circular dependencies    0",
	}

	b.WriteString(sectionStyle.Render("Defaults") + "\n")
	for _, d := range defaults {
		b.WriteString(menuItemStyle.Render("  "+d) + "\n")
	}
	b.WriteString("\n")

	options := []struct {
		label string
		desc  string
	}{
		{"Use defaults (recommended)", "Reasonable starting thresholds, editable in .rotta/quality-gates.yaml"},
		{"Review later", "Install defaults now; customize the YAML file after installation"},
	}

	for i, opt := range options {
		if m.GatesCursor == i {
			b.WriteString(menuSelectedStyle.Render("▸ "+opt.label) + "\n")
			b.WriteString("    " + inputHintStyle.Render(opt.desc) + "\n\n")
		} else {
			b.WriteString(menuItemStyle.Render("  "+opt.label) + "\n\n")
		}
	}

	b.WriteString(helpStyle.Render("j/k to move · Enter to select · Esc to go back"))
	return appStyle.Render(b.String())
}

func (m Model) viewInstalling() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("Installing...") + "\n\n")
	b.WriteString(fmt.Sprintf("  %s Installing Rotta\n\n", m.InstallSpinner.View()))
	b.WriteString(helpStyle.Render("Please wait..."))
	return appStyle.Render(b.String())
}

func (m Model) viewSuccess() string {
	var b strings.Builder
	b.WriteString(successStyle.Render("✓ Rotta Installed") + "\n\n")

	if m.InstallResult != nil {
		b.WriteString(sectionStyle.Render("Installed") + "\n")
		for _, f := range m.InstallResult.Files {
			b.WriteString(progressDoneStyle.Render("  ✓ ") + valueStyle.Render(f) + "\n")
		}
		b.WriteString("\n")
		writeMCPStatuses(&b, m.InstallResult.MCPStatuses)
	}

	b.WriteString(sectionStyle.Render("Next steps") + "\n")
	b.WriteString(menuItemStyle.Render("  1. In your project, run /rotta-spec-mode to start a feature spec") + "\n")
	b.WriteString(menuItemStyle.Render("  2. The Spec Partner will ask clarifying questions") + "\n")
	b.WriteString(menuItemStyle.Render("  3. Approve the Gherkin contract to unlock Implementation Mode") + "\n")
	b.WriteString(menuItemStyle.Render("  4. After TDD, run /rotta-review-mode for quality gate evaluation") + "\n\n")

	b.WriteString(helpStyle.Render("Press Enter or q to exit"))
	return appStyle.Render(b.String())
}

func (m Model) viewError() string {
	var b strings.Builder
	b.WriteString(errorStyle.Render("✗ Installation Failed") + "\n\n")
	b.WriteString(valueStyle.Render(m.InstallError) + "\n\n")
	if m.InstallResult != nil {
		writeMCPStatuses(&b, m.InstallResult.MCPStatuses)
	}
	b.WriteString(helpStyle.Render("Press Enter or q to exit"))
	return appStyle.Render(b.String())
}
