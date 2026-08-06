package tui

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Syfra3/Rotta/internal/installer"
)

func (m Model) velaEffectiveConfig() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "unavailable until confirmation"
	}
	resolution, err := installer.OpenCodeVelaConfigResolution(m.Target, m.ProjectPath, home)
	if err != nil {
		return "unavailable: " + err.Error()
	}
	return resolution.Path + " (source: " + resolution.Source + ")"
}

func writeMCPStatuses(b *strings.Builder, statuses map[string]map[string]installer.MCPStatusResult) {
	if len(statuses) == 0 {
		return
	}
	b.WriteString(sectionStyle.Render("MCP status") + "\n")
	hosts := make([]string, 0, len(statuses))
	for host := range statuses {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)
	for _, host := range hosts {
		writeHostMCPStatuses(b, host, statuses[host])
	}
	b.WriteString("\n")
}

func writeHostMCPStatuses(b *strings.Builder, host string, statuses map[string]installer.MCPStatusResult) {
	mcps := make([]string, 0, len(statuses))
	for mcp := range statuses {
		mcps = append(mcps, mcp)
	}
	sort.Strings(mcps)
	for _, mcp := range mcps {
		status := statuses[mcp]
		b.WriteString(menuItemStyle.Render(fmt.Sprintf("  %s / %s: %s", host, mcp, status.Status)) + "\n")
		b.WriteString(menuItemStyle.Render("    Reason: "+status.Reason) + "\n")
		b.WriteString(menuItemStyle.Render("    Remediation: "+status.Remediation) + "\n")
		b.WriteString(menuItemStyle.Render("    Runtime fallback: "+string(status.RuntimeFallback.State)) + "\n")
	}
}

func (m Model) viewVela() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("Vela — Optional Graph Intelligence") + "\n\n")
	b.WriteString(sectionStyle.Render("What Vela does") + "\n")
	b.WriteString(menuItemStyle.Render("  Extracts local code graphs for structural, dependency, and impact questions") + "\n")
	b.WriteString(menuItemStyle.Render("  Provides bounded advisory evidence for named structural questions") + "\n")
	b.WriteString(menuItemStyle.Render("  Requires an explicit operations request before graph indexing") + "\n\n")
	b.WriteString(sectionStyle.Render("Workflow boundary") + "\n")
	b.WriteString(menuItemStyle.Render("  Vela is advisory only and cannot approve, block, or create workflow state") + "\n")
	b.WriteString(menuItemStyle.Render("  Ancora is compact context, not a graph or lifecycle authority") + "\n\n")
	b.WriteString(warningStyle.Render("Note: ") + inputHintStyle.Render("Skip is the default. Setup is OpenCode-only and requires a separate host-level confirmation.") + "\n\n")
	options := []struct{ label, desc string }{{"Install and configure Vela for OpenCode", "Runs Homebrew bootstrap, writes only mcp.vela, and never indexes"}, {"Skip (default)", "Do not install or configure Vela"}}
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

func (m Model) viewVelaConfirm() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("Confirm OpenCode Vela host setup") + "\n\n")
	b.WriteString(warningStyle.Render("This is a host-level action, not project setup.") + "\n\n")
	b.WriteString(menuItemStyle.Render("  Effective OpenCode config: "+m.velaEffectiveConfig()) + "\n")
	b.WriteString(menuItemStyle.Render("  Bootstrap: brew tap Syfra3/tap → brew install vela → vela version") + "\n")
	b.WriteString(menuItemStyle.Render("  MCP: mcp.vela type local, command [vela serve --mcp], enabled") + "\n")
	b.WriteString(menuItemStyle.Render("  Guarantee: no index, reindex, vela update ., or .vela/ creation") + "\n\n")
	b.WriteString(menuItemStyle.Render("Press Enter/Y to confirm; Esc/N to return to Skip.") + "\n")
	return appStyle.Render(b.String())
}
