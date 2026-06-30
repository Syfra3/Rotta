package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

var _ = tea.Quit // suppress unused import

func (m Model) View() string {
	switch m.Screen {
	case ScreenWelcome:
		return m.viewWelcome()
	case ScreenTargetSelect:
		return m.viewTargetSelect()
	case ScreenProjectPath:
		return m.viewProjectPath()
	case ScreenModeSelect:
		return m.viewModeSelect()
	case ScreenQualityGates:
		return m.viewQualityGates()
	case ScreenAncora:
		return m.viewAncora()
	case ScreenVela:
		return m.viewVela()
	case ScreenConfirm:
		return m.viewConfirm()
	case ScreenInstalling:
		return m.viewInstalling()
	case ScreenSuccess:
		return m.viewSuccess()
	case ScreenError:
		return m.viewError()
	case ScreenRecoveryList:
		return m.viewRecoveryList()
	case ScreenRecoveryPreview:
		return m.viewRecoveryPreview()
	case ScreenRecoveryConfirm:
		return m.viewRecoveryConfirm()
	}
	return ""
}

func (m Model) viewWelcome() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Clean Workflow Installer") + "\n")
	b.WriteString(subtitleStyle.Render("Contract-driven AI coding for Claude Code and OpenCode") + "\n\n")

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
	return ancora + ", " + vela
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
		{"Claude Code", "SKILL.md files → ~/.claude/skills/clean-workflow/"},
		{"OpenCode", "Agent entries + skill files for clean-orchestrator, clean-spec, clean-impl, clean-review"},
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
	b.WriteString(menuItemStyle.Render("  Clean Workflow agents call ancora_save / ancora_search to remember decisions") + "\n")
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

func (m Model) viewVela() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("Vela — Optional Graph Intelligence") + "\n\n")

	b.WriteString(sectionStyle.Render("What Vela does") + "\n")
	b.WriteString(menuItemStyle.Render("  Extracts local code graphs for structural, dependency, and impact questions") + "\n")
	b.WriteString(menuItemStyle.Render("  Provides vela_* graph tools when graph data exists and is fresh") + "\n")
	b.WriteString(menuItemStyle.Render("  Enriches workflow exploration with facts, provenance, confidence, and source") + "\n\n")

	b.WriteString(sectionStyle.Render("Workflow boundary") + "\n")
	b.WriteString(menuItemStyle.Render("  Clean Workflow still controls phases, gates, and delegation") + "\n")
	b.WriteString(menuItemStyle.Render("  Vela is advisory graph intelligence, not the workflow controller") + "\n")
	if m.SetupAncora {
		b.WriteString(menuItemStyle.Render("  Ancora remains the primary MCP surface; Vela graph tools are exposed through Ancora when available") + "\n\n")
	} else {
		b.WriteString(menuItemStyle.Render("  Vela is configured as a standalone MCP graph server") + "\n\n")
	}

	b.WriteString(warningStyle.Render("Note: ") + inputHintStyle.Render("If Vela is missing, the installer tries Homebrew. If unavailable, install Vela from source and rerun setup.") + "\n\n")

	options := []struct{ label, desc string }{
		{"Install + configure Vela", "Install binary if needed, initialize the project graph, and configure graph instructions/MCP"},
		{"Skip", "Do not set up Vela — agents will use normal code exploration only"},
	}
	for i, opt := range options {
		if m.VelaCursor == i {
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
		{"Use defaults (recommended)", "Reasonable starting thresholds, editable in .clean-workflow/quality-gates.yaml"},
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

func (m Model) viewConfirm() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("Confirm Installation") + "\n\n")

	b.WriteString(sectionStyle.Render("Summary") + "\n")
	b.WriteString(labelStyle.Render("Target:") + " " + valueStyle.Render(m.Target) + "\n")
	b.WriteString(labelStyle.Render("Project path:") + " " + valueStyle.Render(m.ProjectPath) + "\n")

	var modes []string
	labels := []string{"Spec", "Implementation", "Review"}
	for i, sel := range m.SelectedModes {
		if sel {
			modes = append(modes, labels[i])
		}
	}
	b.WriteString(labelStyle.Render("Modes:") + " " + valueStyle.Render(strings.Join(modes, ", ")) + "\n")

	gates := "defaults"
	if !m.UseDefaults {
		gates = "review later"
	}
	b.WriteString(labelStyle.Render("Quality gates:") + " " + valueStyle.Render(gates) + "\n")

	ancora := "yes (install + configure)"
	if !m.SetupAncora {
		ancora = "skip"
	}
	b.WriteString(labelStyle.Render("Ancora memory:") + " " + valueStyle.Render(ancora) + "\n")

	vela := "yes (install + configure)"
	if !m.SetupVela {
		vela = "skip"
	}
	b.WriteString(labelStyle.Render("Vela graph:") + " " + valueStyle.Render(vela) + "\n\n")

	b.WriteString(sectionStyle.Render("Files to create") + "\n")
	if m.Target == "claude-code" || m.Target == "both" {
		if m.SelectedModes[0] {
			b.WriteString(menuItemStyle.Render("  ~/.claude/skills/clean-workflow/spec-mode/SKILL.md") + "\n")
		}
		if m.SelectedModes[1] {
			b.WriteString(menuItemStyle.Render("  ~/.claude/skills/clean-workflow/implementation-mode/SKILL.md") + "\n")
		}
		if m.SelectedModes[2] {
			b.WriteString(menuItemStyle.Render("  ~/.claude/skills/clean-workflow/review-mode/SKILL.md") + "\n")
		}
	}
	if m.Target == "opencode" || m.Target == "both" {
		b.WriteString(menuItemStyle.Render("  ~/.config/opencode/opencode.json  (agent entries)") + "\n")
		b.WriteString(menuItemStyle.Render("  ~/.config/opencode/skills/clean-orchestrator/SKILL.md") + "\n")
		if m.SelectedModes[0] {
			b.WriteString(menuItemStyle.Render("  ~/.config/opencode/skills/clean-spec/SKILL.md") + "\n")
		}
		if m.SelectedModes[1] {
			b.WriteString(menuItemStyle.Render("  ~/.config/opencode/skills/clean-impl/SKILL.md") + "\n")
		}
		if m.SelectedModes[2] {
			b.WriteString(menuItemStyle.Render("  ~/.config/opencode/skills/clean-review/SKILL.md") + "\n")
		}
	}
	b.WriteString(menuItemStyle.Render("  .clean-workflow/state-machine.yaml") + "\n")
	b.WriteString(menuItemStyle.Render("  .clean-workflow/quality-gates.yaml") + "\n")
	if m.SetupAncora {
		if m.Target == "claude-code" || m.Target == "both" {
			b.WriteString(menuItemStyle.Render("  ~/.claude/mcp/ancora.json") + "\n")
			b.WriteString(menuItemStyle.Render("  ~/.claude/settings.json  (permissions.allow)") + "\n")
		}
		if m.Target == "opencode" || m.Target == "both" {
			b.WriteString(menuItemStyle.Render("  ~/.config/opencode/opencode.jsonc  (mcp.ancora)") + "\n")
		}
	}
	if m.SetupVela {
		b.WriteString(menuItemStyle.Render("  <project>/.vela/graph.db  (initialized, not extracted)") + "\n")
		if !m.SetupAncora && (m.Target == "claude-code" || m.Target == "both") {
			b.WriteString(menuItemStyle.Render("  ~/.claude/vela-mcp.json") + "\n")
		}
		if !m.SetupAncora && (m.Target == "opencode" || m.Target == "both") {
			b.WriteString(menuItemStyle.Render("  ~/.config/opencode/opencode.json  (mcp.vela)") + "\n")
		}
	}
	b.WriteString("\n")

	choices := []string{"Cancel", "Install"}
	for i, ch := range choices {
		if m.ConfirmCursor == i {
			b.WriteString(menuSelectedStyle.Render("▸ "+ch) + "\n")
		} else {
			b.WriteString(menuItemStyle.Render("  "+ch) + "\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("j/k to move · Enter to select · Esc to go back"))
	return appStyle.Render(b.String())
}

func (m Model) viewInstalling() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("Installing...") + "\n\n")
	b.WriteString(fmt.Sprintf("  %s Installing Clean Workflow\n\n", m.InstallSpinner.View()))
	b.WriteString(helpStyle.Render("Please wait..."))
	return appStyle.Render(b.String())
}

func (m Model) viewSuccess() string {
	var b strings.Builder
	b.WriteString(successStyle.Render("✓ Clean Workflow Installed") + "\n\n")

	if m.InstallResult != nil {
		b.WriteString(sectionStyle.Render("Installed") + "\n")
		for _, f := range m.InstallResult.Files {
			b.WriteString(progressDoneStyle.Render("  ✓ ") + valueStyle.Render(f) + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(sectionStyle.Render("Next steps") + "\n")
	b.WriteString(menuItemStyle.Render("  1. In your project, run /clean-spec-mode to start a feature spec") + "\n")
	b.WriteString(menuItemStyle.Render("  2. The Spec Partner will ask clarifying questions") + "\n")
	b.WriteString(menuItemStyle.Render("  3. Approve the Gherkin contract to unlock Implementation Mode") + "\n")
	b.WriteString(menuItemStyle.Render("  4. After TDD, run /clean-review-mode for quality gate evaluation") + "\n\n")

	b.WriteString(helpStyle.Render("Press Enter or q to exit"))
	return appStyle.Render(b.String())
}

func (m Model) viewError() string {
	var b strings.Builder
	b.WriteString(errorStyle.Render("✗ Installation Failed") + "\n\n")
	b.WriteString(valueStyle.Render(m.InstallError) + "\n\n")
	b.WriteString(helpStyle.Render("Press Enter or q to exit"))
	return appStyle.Render(b.String())
}
