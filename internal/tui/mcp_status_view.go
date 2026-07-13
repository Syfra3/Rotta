package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Syfra3/Rotta/internal/installer"
)

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
	b.WriteString(menuItemStyle.Render("  Provides vela_* graph tools when graph data exists and is fresh") + "\n")
	b.WriteString(menuItemStyle.Render("  Installs a freshness guard that schedules non-blocking refresh before graph queries") + "\n")
	b.WriteString(menuItemStyle.Render("  Enriches workflow exploration with facts, provenance, confidence, and source") + "\n\n")
	b.WriteString(sectionStyle.Render("Workflow boundary") + "\n")
	b.WriteString(menuItemStyle.Render("  Rotta still controls phases, gates, and delegation") + "\n")
	b.WriteString(menuItemStyle.Render("  Vela is advisory graph intelligence, not the workflow controller") + "\n")
	if m.SetupAncora {
		b.WriteString(menuItemStyle.Render("  Ancora remains the primary MCP surface; Vela graph tools are exposed through Ancora when available") + "\n\n")
	} else {
		b.WriteString(menuItemStyle.Render("  Vela is configured as a standalone MCP graph server") + "\n\n")
	}
	b.WriteString(sectionStyle.Render("Freshness guard") + "\n")
	b.WriteString(menuItemStyle.Render("  OpenCode plugin: schedules background refresh before Vela graph tools") + "\n")
	b.WriteString(menuItemStyle.Render("  Claude Code hook: schedules background refresh before Vela graph tools") + "\n")
	b.WriteString(menuItemStyle.Render("  The cached graph may be used while refresh runs; run vela update/build manually for foreground refresh") + "\n\n")
	b.WriteString(warningStyle.Render("Note: ") + inputHintStyle.Render("If Vela is missing, the installer tries Homebrew. If unavailable, install Vela from source and rerun setup.") + "\n\n")
	options := []struct{ label, desc string }{{"Install + configure Vela", "Install binary if needed, initialize the project graph, and install graph freshness guard/MCP"}, {"Skip", "Do not set up Vela — agents will use normal code exploration only"}}
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
