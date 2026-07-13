package tui

import (
	"strings"
)

func (m Model) viewConfirm() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("Confirm Installation") + "\n\n")
	m.writeConfirmSummary(&b)
	b.WriteString(sectionStyle.Render("Files to create") + "\n")
	m.writeConfirmFiles(&b)
	m.writeConfirmChoices(&b)
	return appStyle.Render(b.String())
}

func (m Model) writeConfirmSummary(b *strings.Builder) {
	b.WriteString(sectionStyle.Render("Summary") + "\n")
	writeConfirmValue(b, "Target:", m.Target)
	writeConfirmValue(b, "Project path:", m.ProjectPath)
	writeConfirmValue(b, "Modes:", strings.Join(selectedConfirmModes(m.SelectedModes), ", "))
	writeConfirmValue(b, "Quality gates:", confirmGateLabel(m.UseDefaults))
	writeConfirmValue(b, "Ancora memory:", confirmSetupLabel(m.SetupAncora))
	writeConfirmValue(b, "Vela graph:", confirmSetupLabel(m.SetupVela))
	writeConfirmValue(b, "Context7 docs:", confirmSetupLabel(m.SetupContext7))
	b.WriteString("\n")
}

func writeConfirmValue(b *strings.Builder, label, value string) {
	b.WriteString(labelStyle.Render(label) + " " + valueStyle.Render(value) + "\n")
}

func selectedConfirmModes(selected [3]bool) []string {
	labels := []string{"Spec", "Implementation", "Review"}
	var modes []string
	for index, enabled := range selected {
		if enabled {
			modes = append(modes, labels[index])
		}
	}
	return modes
}

func confirmGateLabel(useDefaults bool) string {
	if useDefaults {
		return "defaults"
	}
	return "review later"
}

func confirmSetupLabel(enabled bool) string {
	if enabled {
		return "yes (install + configure)"
	}
	return "skip"
}

func (m Model) writeConfirmFiles(b *strings.Builder) {
	m.writeConfirmHostFiles(b)
	writeConfirmFile(b, "  .rotta/state-machine.yaml")
	writeConfirmFile(b, "  .rotta/quality-gates.yaml")
	m.writeConfirmIntegrationFiles(b)
}

func (m Model) writeConfirmHostFiles(b *strings.Builder) {
	if m.Target == TargetClaudeCode || m.Target == TargetBoth {
		writeSelectedConfirmFiles(b, m.SelectedModes, []string{
			"  ~/.claude/skills/rotta/spec-mode/SKILL.md",
			"  ~/.claude/skills/rotta/implementation-mode/SKILL.md",
			"  ~/.claude/skills/rotta/review-mode/SKILL.md",
		})
	}
	if m.Target == TargetOpenCode || m.Target == TargetBoth {
		writeConfirmFile(b, "  ~/.config/opencode/opencode.json  (agent entries)")
		writeConfirmFile(b, "  ~/.config/opencode/skills/rotta-orchestrator/SKILL.md")
		writeSelectedConfirmFiles(b, m.SelectedModes, []string{
			"  ~/.config/opencode/skills/rotta-spec/SKILL.md",
			"  ~/.config/opencode/skills/rotta-impl/SKILL.md",
			"  ~/.config/opencode/skills/rotta-review/SKILL.md",
		})
	}
	if m.Target == TargetCodex {
		writeConfirmFile(b, "  ~/.codex/AGENTS.md  (Codex instructions)")
	}
}

func writeSelectedConfirmFiles(b *strings.Builder, selected [3]bool, files []string) {
	for index, enabled := range selected {
		if enabled {
			writeConfirmFile(b, files[index])
		}
	}
}

func writeConfirmFile(b *strings.Builder, file string) {
	b.WriteString(menuItemStyle.Render(file) + "\n")
}

func (m Model) writeConfirmIntegrationFiles(b *strings.Builder) {
	if m.SetupAncora {
		m.writeConfirmAncoraFiles(b)
	}
	if m.SetupVela {
		m.writeConfirmVelaFiles(b)
	}
	if m.SetupContext7 {
		writeConfirmFile(b, "  ~/.claude/mcp/context7.json  (mcp.context7)")
		writeConfirmFile(b, "  ~/.config/opencode/opencode.json  (mcp.context7)")
	}
}

func (m Model) writeConfirmAncoraFiles(b *strings.Builder) {
	if m.Target == TargetClaudeCode || m.Target == TargetBoth {
		writeConfirmFile(b, "  ~/.claude/mcp/ancora.json")
		writeConfirmFile(b, "  ~/.claude/settings.json  (permissions.allow)")
	}
	if m.Target == TargetOpenCode || m.Target == TargetBoth {
		writeConfirmFile(b, "  ~/.config/opencode/opencode.jsonc  (mcp.ancora)")
	}
}

func (m Model) writeConfirmVelaFiles(b *strings.Builder) {
	writeConfirmFile(b, "  <project>/.vela/graph.db  (initialized, not extracted)")
	writeConfirmFile(b, "  graph freshness guard  (non-blocking refresh before Vela graph queries)")
	if m.Target == TargetClaudeCode || m.Target == TargetBoth {
		writeConfirmFile(b, "  ~/.claude/hooks/rotta-vela-freshness-guard.sh")
		writeConfirmFile(b, "  ~/.claude/settings.json  (PreToolUse hook)")
	}
	if m.Target == TargetOpenCode || m.Target == TargetBoth {
		writeConfirmFile(b, "  ~/.config/opencode/plugin/rotta-vela-freshness-guard.js")
		writeConfirmFile(b, "  ~/.config/opencode/opencode.json  (plugin entry)")
	}
	if !m.SetupAncora && (m.Target == TargetClaudeCode || m.Target == TargetBoth) {
		writeConfirmFile(b, "  ~/.claude/vela-mcp.json")
	}
	if m.Target == TargetOpenCode || m.Target == TargetBoth {
		writeConfirmFile(b, "  ~/.config/opencode/opencode.json  (mcp.vela)")
	}
}

func (m Model) writeConfirmChoices(b *strings.Builder) {
	b.WriteString("\n")
	for index, choice := range []string{"Cancel", "Install"} {
		if m.ConfirmCursor == index {
			b.WriteString(menuSelectedStyle.Render("▸ "+choice) + "\n")
			continue
		}
		b.WriteString(menuItemStyle.Render("  "+choice) + "\n")
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("j/k to move · Enter to select · Esc to go back"))
}
