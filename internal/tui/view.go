package tui

import (
	"fmt"
	"os"
	"path/filepath"
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
		ScreenWelcome:              m.viewWelcome,
		ScreenTargetSelect:         m.viewTargetSelect,
		ScreenProjectPath:          m.viewProjectPath,
		ScreenModelRouting:         m.viewModelRouting,
		ScreenCustomModelRouting:   m.viewCustomModelRouting,
		ScreenModelPicker:          m.viewModelPicker,
		ScreenPiModelRouting:       m.viewPiModelRouting,
		ScreenPiCustomModelRouting: m.viewPiCustomModelRouting,
		ScreenPiModelPicker:        m.viewPiModelPicker,
		ScreenModeSelect:           m.viewModeSelect,
		ScreenQualityGates:         m.viewQualityGates,
		ScreenAncora:               m.viewAncora,
		ScreenVela:                 m.viewVela,
		ScreenContext7:             m.viewContext7,
		ScreenTypeSafe:             m.viewTypeSafe,
		ScreenTypeSafeKey:          m.viewTypeSafeKey,
		ScreenConfirm:              m.viewConfirm,
		ScreenInstalling:           m.viewInstalling,
		ScreenSuccess:              m.viewSuccess,
		ScreenError:                m.viewError,
		ScreenRecoveryList:         m.viewRecoveryList,
		ScreenRecoveryPreview:      m.viewRecoveryPreview,
		ScreenRecoveryConfirm:      m.viewRecoveryConfirm,
	}
}

func (m Model) viewWelcome() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Rotta Installer") + "\n")
	b.WriteString(subtitleStyle.Render("Lightweight reviewed coding workflow for Claude Code, OpenCode, and Codex") + "\n\n")

	b.WriteString(sectionStyle.Render("What this installs") + "\n")
	b.WriteString(menuItemStyle.Render("  Fast mode        — One coherent slice, focused tests, independent review") + "\n")
	b.WriteString(menuItemStyle.Render("  Strict mode      — Approved compact contract for high-risk work") + "\n")
	b.WriteString(menuItemStyle.Render("  Five roles       — Orchestrator, Explore, Implementation, Review, Operations") + "\n\n")

	b.WriteString(cardStyle.Render(
		warningStyle.Render("Philosophy")+"\n"+
			"  Fast mode minimizes coordination without skipping practical\n"+
			"  safeguards: focused verification and a fresh independent\n"+
			"  review. Strict mode adds approval only where risk warrants it.",
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
		{"Claude Code", "Managed core and role skills → ~/.claude/skills/rotta-next/"},
		{"OpenCode", "Agent entries + managed core and role skills for Rotta Next"},
		{"Codex", "Codex instructions → ~/.codex/AGENTS.md"},
		{"Pi", "Executable Rotta extension → ~/.pi/agent/extensions/rotta.ts"},
		{"Both", "Install for both tools"},
		{"All", "Install every supported host integration once"},
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

func (m Model) viewModelRouting() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("OpenCode Model Routing") + "\n\n")
	b.WriteString(inputHintStyle.Render("Select the installer-managed seven-role routing action.") + "\n\n")
	for index, label := range []string{"Default", "Custom", "Disabled"} {
		style, prefix := menuItemStyle, "  "
		if m.ModelRoutingCursor == index {
			style, prefix = menuSelectedStyle, "▸ "
		}
		b.WriteString(style.Render(prefix+label) + "\n")
	}
	b.WriteString("\n" + helpStyle.Render("j/k to move · Enter to select · Esc to go back"))
	return appStyle.Render(b.String())
}

func (m Model) viewPiModelRouting() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("Pi Model Routing") + "\n\n")
	b.WriteString(inputHintStyle.Render("Select the installer-managed four-role routing action.") + "\n\n")
	for index, label := range []string{"Default", "Custom", "Disabled"} {
		style, prefix := menuItemStyle, "  "
		if m.PiModelRoutingCursor == index {
			style, prefix = menuSelectedStyle, "▸ "
		}
		b.WriteString(style.Render(prefix+label) + "\n")
	}
	b.WriteString("\n" + helpStyle.Render("j/k to move · Enter to select · Esc to go back"))
	return appStyle.Render(b.String())
}

func (m Model) viewCustomModelRouting() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("Custom OpenCode Model Routing") + "\n\n")
	b.WriteString(inputHintStyle.Render("Choose a model for each phase. Listed models are discovered locally; credentials are not verified.") + "\n\n")
	if !m.ModelDiscoveryDone {
		b.WriteString(menuItemStyle.Render("Discovering models from OpenCode…") + "\n\n")
	} else if m.ModelDiscoveryError != "" {
		b.WriteString(errorStyle.Render(m.ModelDiscoveryError) + "\n\n")
	}
	for index, role := range routingRoles {
		style, prefix := menuItemStyle, "  "
		if index == m.CustomRoutingCursor {
			style, prefix = menuSelectedStyle, "▸ "
		}
		b.WriteString(style.Render(fmt.Sprintf("%s%s: %s", prefix, role.label, m.CustomRouting[role.key])) + "\n")
	}
	if m.ModelDiscoveryError != "" && !m.ModelDiscoveryInFlight {
		b.WriteString(helpStyle.Render("r to retry discovery · "))
	}
	b.WriteString("\n" + helpStyle.Render("j/k to move · Enter to choose · n to continue · Esc to go back"))
	return appStyle.Render(b.String())
}

func (m Model) viewPiCustomModelRouting() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("Custom Pi Model Routing") + "\n\n")
	b.WriteString(inputHintStyle.Render("Choose a model for each delegated role. Models come from pi --offline --list-models; credentials are not verified.") + "\n\n")
	if !m.PiModelDiscoveryDone {
		b.WriteString(menuItemStyle.Render("Discovering offline Pi models…") + "\n\n")
	} else if m.PiModelDiscoveryError != "" {
		b.WriteString(errorStyle.Render(m.PiModelDiscoveryError) + "\n\n")
	}
	for index, role := range piRoutingRoles {
		style, prefix := menuItemStyle, "  "
		if index == m.PiCustomRoutingCursor {
			style, prefix = menuSelectedStyle, "▸ "
		}
		b.WriteString(style.Render(fmt.Sprintf("%s%s: %s", prefix, role.label, m.PiCustomRouting[role.key])) + "\n")
	}
	if m.PiModelDiscoveryError != "" && !m.PiModelDiscoveryInFlight {
		b.WriteString(helpStyle.Render("r to retry discovery · "))
	}
	b.WriteString("\n" + helpStyle.Render("j/k to move · Enter to choose · n to continue · Esc to go back"))
	return appStyle.Render(b.String())
}

func (m Model) viewTypeSafe() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("TypeSafe/Jev — Pi Session Judgments") + "\n\n")
	b.WriteString(sectionStyle.Render("What this does") + "\n")
	b.WriteString(menuItemStyle.Render("  Enables pi-typesafe's agent tool by default in future Pi sessions") + "\n")
	b.WriteString(menuItemStyle.Render("  Avoids running /typesafe enable every session after Rotta installation") + "\n\n")
	b.WriteString(warningStyle.Render("Note: ") + inputHintStyle.Render("Jev requests are billed to your TypeSafe account; no request is sent during installation.") + "\n\n")
	options := []struct{ label, desc string }{{"Enable TypeSafe/Jev by default", "Rotta's Pi extension will set PI_TYPESAFE_ENABLED=1 when managed config is valid"}, {"Skip", "Leave TypeSafe/Jev disabled unless you enable it manually"}}
	for i, opt := range options {
		if m.TypeSafeCursor == i {
			b.WriteString(menuSelectedStyle.Render("▸ "+opt.label) + "\n")
			b.WriteString("    " + inputHintStyle.Render(opt.desc) + "\n\n")
		} else {
			b.WriteString(menuItemStyle.Render("  "+opt.label) + "\n\n")
		}
	}
	b.WriteString(helpStyle.Render("j/k to move · Enter to select · Esc to go back"))
	return appStyle.Render(b.String())
}

func (m Model) viewTypeSafeKey() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("TypeSafe/Jev API Key") + "\n\n")
	b.WriteString(inputHintStyle.Render("Optional. Paste a key to store in ~/.pi/agent/pi-typesafe/auth.json, or leave empty and press Enter to use /typesafe login later.") + "\n\n")
	b.WriteString(m.TypeSafeKeyInput.View() + "\n\n")
	b.WriteString(helpStyle.Render("Enter to continue · Esc to go back"))
	return appStyle.Render(b.String())
}

func (m Model) viewModelPicker() string {
	var b strings.Builder
	role := routingRoles[m.CustomRoutingCursor]
	b.WriteString(headerStyle.Render("Choose model for "+role.label) + "\n\n")
	b.WriteString(inputHintStyle.Render("Type to search · Backspace to clear · Enter to select") + "\n")
	b.WriteString(valueStyle.Render("Search: "+m.ModelPickerQuery) + "\n\n")
	models := m.filteredModels()
	if len(models) == 0 {
		b.WriteString(menuItemStyle.Render("  No matching models") + "\n")
	} else {
		start := m.ModelPickerCursor - 5
		if start < 0 {
			start = 0
		}
		end := start + 11
		if end > len(models) {
			end = len(models)
		}
		for index := start; index < end; index++ {
			style, prefix := menuItemStyle, "  "
			if index == m.ModelPickerCursor {
				style, prefix = menuSelectedStyle, "▸ "
			}
			b.WriteString(style.Render(prefix+models[index]) + "\n")
		}
	}
	b.WriteString("\n" + helpStyle.Render("↑/↓ to move · Enter to select · Esc to go back"))
	return appStyle.Render(b.String())
}

func (m Model) viewPiModelPicker() string {
	var b strings.Builder
	role := piRoutingRoles[m.PiCustomRoutingCursor]
	b.WriteString(headerStyle.Render("Choose model for "+role.label) + "\n\n")
	b.WriteString(inputHintStyle.Render("Type to search · Backspace to clear · Enter to select") + "\n")
	b.WriteString(valueStyle.Render("Search: "+m.ModelPickerQuery) + "\n\n")
	models := m.filteredPiModels()
	if len(models) == 0 {
		b.WriteString(menuItemStyle.Render("  No matching models") + "\n")
	} else {
		for index, model := range models {
			style, prefix := menuItemStyle, "  "
			if index == m.PiModelPickerCursor {
				style, prefix = menuSelectedStyle, "▸ "
			}
			b.WriteString(style.Render(prefix+model) + "\n")
		}
	}
	b.WriteString("\n" + helpStyle.Render("↑/↓ to move · Enter to select · Esc to go back"))
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
	b.WriteString(menuItemStyle.Render("  Stores compact decisions, discoveries, and result pointers across sessions") + "\n")
	b.WriteString(menuItemStyle.Render("  Rotta agents recover relevant context without making memory workflow authority") + "\n")
	b.WriteString(menuItemStyle.Render("  Workspace and Git state remain authoritative when memory is unavailable") + "\n\n")

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

	b.WriteString(sectionStyle.Render("Required generic gates") + "\n")
	for _, gate := range []string{"build", "tests", "changed-file scope", "static analysis", "dependency checks", "security checks"} {
		b.WriteString(menuItemStyle.Render("  "+gate) + "\n")
	}
	b.WriteString("\n")
	writeQualityGatePreview(&b, m.ProjectPath)
	b.WriteString("\n")
	b.WriteString(inputHintStyle.Render("Commands are detected from project conventions and metadata during review.") + "\n")
	b.WriteString(inputHintStyle.Render("Unresolved required commands leave readiness blocked with remediation.") + "\n")
	b.WriteString(inputHintStyle.Render("Generated configuration: .rotta/quality-gates.yaml") + "\n\n")
	b.WriteString(menuSelectedStyle.Render("▸ Use generic threshold defaults (recommended)") + "\n")
	b.WriteString("    " + inputHintStyle.Render("Generate the v2 generic-gate policy in .rotta/quality-gates.yaml") + "\n\n")

	b.WriteString(helpStyle.Render("Enter to select · Esc to go back"))
	return appStyle.Render(b.String())
}

func writeQualityGatePreview(b *strings.Builder, projectPath string) {
	metadataPath := filepath.Join(projectPath, ".rotta", "current", "review-snapshot.yaml")
	metadata, _ := os.ReadFile(metadataPath)
	commands := declaredPreviewCommands(metadata)
	gates := []struct {
		category    string
		label       string
		remediation string
	}{
		{category: "build", label: "build", remediation: "build"},
		{category: "tests", label: "tests", remediation: "tests"},
		{category: "changed_file_scope", label: "changed-file scope", remediation: "changed-file-scope"},
		{category: "static_analysis", label: "static analysis", remediation: "static-analysis"},
		{category: "dependency_checks", label: "dependency checks", remediation: "dependency-check"},
		{category: "security_checks", label: "security checks", remediation: "security-check"},
	}

	b.WriteString(sectionStyle.Render("Detection preview") + "\n")
	blocked := make([]string, 0, len(gates))
	for _, gate := range gates {
		if command := commands[gate.category]; command != "" {
			b.WriteString(menuItemStyle.Render(fmt.Sprintf("  %s: resolved %s", gate.label, command)) + "\n")
			b.WriteString("    " + inputHintStyle.Render("source: "+metadataPath) + "\n")
			continue
		}
		blocked = append(blocked, gate.label)
		b.WriteString(warningStyle.Render("  "+gate.label+": blocked") + "\n")
		b.WriteString("    " + inputHintStyle.Render("Declare a supported "+gate.remediation+" convention in project metadata.") + "\n")
	}
	b.WriteString(inputHintStyle.Render(fmt.Sprintf("Blocked metrics (%d): %s", len(blocked), strings.Join(blocked, ", "))) + "\n")
}

func declaredPreviewCommands(metadata []byte) map[string]string {
	commands := make(map[string]string)
	var category string
	for _, line := range strings.Split(string(metadata), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") && strings.HasSuffix(trimmed, ":") && trimmed != "conventions:":
			category = strings.TrimSuffix(trimmed, ":")
		case strings.HasPrefix(line, "    command: ") && category != "":
			commands[category] = strings.TrimSpace(strings.TrimPrefix(line, "    command: "))
		}
	}
	return commands
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
		for _, warning := range m.InstallResult.Warnings {
			b.WriteString(valueStyle.Render("Warning: "+warning) + "\n\n")
		}
		writeMCPStatuses(&b, m.InstallResult.MCPStatuses)
		if capability, ok := m.InstallResult.Hosts["opencode"].Capabilities["source_loading"]; ok {
			b.WriteString(sectionStyle.Render("OpenCode policy loading: "+string(capability.Status)) + "\n")
			b.WriteString(valueStyle.Render(capability.Reason) + "\n")
			b.WriteString(inputHintStyle.Render(capability.Remediation) + "\n\n")
		}
	}

	b.WriteString(sectionStyle.Render("Next steps") + "\n")
	b.WriteString(menuItemStyle.Render("  1. Restart your coding agent, then start a task with Rotta-Orchestrator") + "\n")
	b.WriteString(menuItemStyle.Render("  2. Fast mode delegates one coherent slice and an independent review") + "\n")
	b.WriteString(menuItemStyle.Render("  3. Strict mode asks for approval only when the risk triggers it") + "\n\n")

	b.WriteString(helpStyle.Render("Press Enter or q to exit"))
	return appStyle.Render(b.String())
}

func (m Model) viewError() string {
	var b strings.Builder
	b.WriteString(errorStyle.Render("✗ Installation Failed") + "\n\n")
	b.WriteString(valueStyle.Render(m.InstallError) + "\n\n")
	if m.InstallResult != nil {
		for _, warning := range m.InstallResult.Warnings {
			b.WriteString(valueStyle.Render("Warning: "+warning) + "\n\n")
		}
		writeMCPStatuses(&b, m.InstallResult.MCPStatuses)
	}
	b.WriteString(helpStyle.Render("Press Enter or q to exit"))
	return appStyle.Render(b.String())
}
