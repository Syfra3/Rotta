package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/Syfra3/Rotta/internal/installer"
	tea "github.com/charmbracelet/bubbletea"
)

func TestViewConfirmRendersAncoraVelaCombinations(t *testing.T) {
	for _, tt := range confirmViewCombinations {
		t.Run(tt.name, func(t *testing.T) { assertConfirmViewCombination(t, tt) })
	}
}

type confirmViewCombination struct {
	name              string
	ancora, vela      bool
	want, notExpected []string
}

var confirmViewCombinations = []confirmViewCombination{
	{
		name:   "ancora and vela enabled",
		ancora: true,
		vela:   true,
		want: []string{
			"Ancora memory:",
			"Vela graph:",
			"yes (install + configure)",
			"~/.claude/mcp/ancora.json",
			"<project>/.vela/graph.db  (initialized, not extracted)",
		},
		notExpected: []string{"~/.claude/vela-mcp.json"},
	},
	{
		name:   "ancora enabled and vela disabled",
		ancora: true,
		vela:   false,
		want: []string{
			"Ancora memory:",
			"yes (install + configure)",
			"Vela graph:",
			"skip",
			"~/.claude/mcp/ancora.json",
		},
		notExpected: []string{"<project>/.vela/graph.db", "~/.claude/vela-mcp.json"},
	},
	{
		name:   "ancora disabled and vela enabled",
		ancora: false,
		vela:   true,
		want: []string{
			"Ancora memory:",
			"skip",
			"Vela graph:",
			"yes (install + configure)",
			"<project>/.vela/graph.db  (initialized, not extracted)",
			"~/.claude/vela-mcp.json",
		},
		notExpected: []string{"~/.claude/mcp/ancora.json"},
	},
	{
		name:   "ancora and vela disabled",
		ancora: false,
		vela:   false,
		want: []string{
			"Ancora memory:",
			"Vela graph:",
			"skip",
		},
		notExpected: []string{"~/.claude/mcp/ancora.json", "<project>/.vela/graph.db", "~/.claude/vela-mcp.json"},
	},
}

func assertConfirmViewCombination(t *testing.T, tt confirmViewCombination) {
	t.Helper()
	model := New()
	model.Screen = ScreenConfirm
	model.Target = TargetClaudeCode
	model.ProjectPath = "/tmp/project"
	model.SetupAncora = tt.ancora
	model.SetupVela = tt.vela
	got := model.viewConfirm()
	for _, want := range tt.want {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q:\n%s", want, got)
		}
	}
	for _, unwanted := range tt.notExpected {
		if strings.Contains(got, unwanted) {
			t.Fatalf("unexpected %q:\n%s", unwanted, got)
		}
	}
}

func TestSCN222_ViewSuccessReportsMCPInstallStatusAndLaterRuntimeFallback(t *testing.T) {
	// REQ-014, REQ-011, REQ-012, REQ-013 → SCN-222 → TestSCN222_ViewSuccessReportsMCPInstallStatusAndLaterRuntimeFallback
	// Scenario: Expose selected MCP configuration and runtime fallback states
	model := New()
	model.Screen = ScreenSuccess
	model.InstallResult = &installer.Result{MCPStatuses: map[string]map[string]installer.MCPStatusResult{
		"codex": {
			"context7": {
				Status:      installer.MCPStatusDegraded,
				Reason:      "Codex has no observable Context7 health check.",
				Remediation: "Verify Context7 from Codex after install.",
				RuntimeFallback: installer.MCPRuntimeFallback{
					State: installer.MCPRuntimeFallbackNotObserved,
				},
			},
		},
	}}

	got := model.viewSuccess()
	for _, want := range []string{
		"MCP status",
		"codex / context7: degraded",
		"Codex has no observable Context7 health check.",
		"Verify Context7 from Codex after install.",
		"Runtime fallback: not observed during installation",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q:\n%s", want, got)
		}
	}
}

func TestSCN222_ViewErrorRetainsFailedMCPStatus(t *testing.T) {
	// REQ-014 → SCN-222 → TestSCN222_ViewErrorRetainsFailedMCPStatus
	// Scenario: Expose selected MCP configuration and runtime fallback states
	model := New()
	result := &installer.Result{MCPStatuses: map[string]map[string]installer.MCPStatusResult{
		"opencode": {
			"context7": {
				Status:      installer.MCPStatusFailed,
				Reason:      "MCP health check failed during startup.",
				Remediation: "Ensure the MCP command starts before rerunning Rotta.",
				RuntimeFallback: installer.MCPRuntimeFallback{
					State: installer.MCPRuntimeFallbackNotObserved,
				},
			},
		},
	}}
	updated, _ := model.Update(installDoneMsg{result: result, err: errors.New("context7 health: startup")})
	got := updated.(Model).viewError()
	for _, want := range []string{
		"opencode / context7: failed",
		"MCP health check failed during startup.",
		"Ensure the MCP command starts before rerunning Rotta.",
		"Runtime fallback: not observed during installation",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q:\n%s", want, got)
		}
	}
}

func TestSCN101_TUIContext7VisibleSelectedByDefault(t *testing.T) {
	// REQ-001, REQ-005 → SCN-101 → TestSCN101_TUIContext7VisibleSelectedByDefault
	// Scenario: Context7 is visible and selected by default.
	model := New()

	if !model.SetupContext7 {
		t.Fatal("expected Context7 to be selected by default")
	}

	model.Screen = ScreenContext7
	view := model.View()
	for _, want := range []string{
		"Context7",
		"Ancora",
		"Vela",
		"up-to-date library/API documentation through MCP",
		"Install + configure Context7",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected Context7 selection view to contain %q:\n%s", want, view)
		}
	}
}

func TestSCN201_TUITargetSelectionIncludesCodexAsSingleHost(t *testing.T) {
	// REQ-001, REQ-002 → SCN-201 → TestSCN201_TUITargetSelectionIncludesCodexAsSingleHost
	// Scenario: Install Rotta into a single supported host
	model := New()
	model.Screen = ScreenTargetSelect
	model.TargetCursor = 2

	view := model.View()
	if !strings.Contains(view, "Codex") || !strings.Contains(view, "~/.codex/AGENTS.md") {
		t.Fatalf("expected target selection to expose Codex as a single host:\n%s", view)
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(Model)
	if got.Target != TargetCodex {
		t.Fatalf("expected selecting Codex to set target %q, got %q", TargetCodex, got.Target)
	}
}

func TestSCN101_TUIConfirmShowsSelectedContext7ByDefault(t *testing.T) {
	// REQ-001, REQ-005 → SCN-101 → TestSCN101_TUIConfirmShowsSelectedContext7ByDefault
	// Scenario: Context7 is visible and selected by default.
	model := New()
	model.Screen = ScreenConfirm

	confirm := model.viewConfirm()
	if !strings.Contains(context7SummaryLine(confirm), "yes (install + configure)") {
		t.Fatalf("expected confirmation to show default selected Context7:\n%s", confirm)
	}
}

func TestSCN111_TUIContext7CanBeDeselectedBeforeInstall(t *testing.T) {
	// REQ-001, REQ-005 → SCN-111 → TestSCN111_TUIContext7CanBeDeselectedBeforeInstall
	// Scenario: User can deselect the default-checked Context7 option before installation.
	model := New()
	model.Screen = ScreenContext7

	moved, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	selected, _ := moved.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := selected.(Model)

	if updated.SetupContext7 {
		t.Fatal("expected Context7 to be deselected")
	}
	if updated.Screen != ScreenConfirm {
		t.Fatalf("expected deselection to continue to confirmation, got screen %v", updated.Screen)
	}
	confirm := updated.viewConfirm()
	if !strings.Contains(context7SummaryLine(confirm), "skip") {
		t.Fatalf("expected confirmation to show Context7 skipped:\n%s", confirm)
	}
}

func context7SummaryLine(view string) string {
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "Context7 docs:") {
			return line
		}
	}
	return ""
}

func TestSCN102_TUIContext7SelectionDoesNotChangeOtherTools(t *testing.T) {
	// REQ-001, REQ-006 → SCN-102 → TestSCN102_TUIContext7SelectionDoesNotChangeOtherTools
	// Scenario: Selecting Context7 does not change other optional MCP choices.
	model := New()
	model.Screen = ScreenContext7
	model.SetupAncora = false
	model.SetupVela = true
	model.Context7Cursor = 0

	selected, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := selected.(Model)

	if !updated.SetupContext7 {
		t.Fatal("expected Context7 selected")
	}
	if updated.SetupAncora {
		t.Fatal("expected Ancora to remain not selected")
	}
	if !updated.SetupVela {
		t.Fatal("expected Vela to remain selected")
	}
}

func TestSCN101_Context7NavigationBackAndRecoveryFormatting(t *testing.T) {
	// REQ-001, REQ-005 → SCN-101 → TestSCN101_Context7NavigationBackAndRecoveryFormatting
	// Scenario: Context7 is visible and selected by default.
	model := New()
	model.Screen = ScreenVela

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	context7Screen := next.(Model)
	if context7Screen.Screen != ScreenContext7 {
		t.Fatalf("expected Vela selection to advance to Context7, got %v", context7Screen.Screen)
	}

	up, _ := context7Screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if up.(Model).Context7Cursor != 0 {
		t.Fatalf("expected Context7 cursor to stay at first item, got %d", up.(Model).Context7Cursor)
	}
	back, _ := context7Screen.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if back.(Model).Screen != ScreenVela {
		t.Fatalf("expected Context7 back navigation to return to Vela, got %v", back.(Model).Screen)
	}

	context7Screen.Screen = ScreenConfirm
	confirmBack, _ := context7Screen.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if confirmBack.(Model).Screen != ScreenContext7 {
		t.Fatalf("expected confirmation back navigation to return to Context7, got %v", confirmBack.(Model).Screen)
	}

	formatted := formatRecoveryIntegrations(recoveryOptionalIntegrations{Ancora: true, Vela: true, Context7: true})
	if !strings.Contains(formatted, "Context7: yes") {
		t.Fatalf("expected recovery integration summary to include Context7 yes, got %q", formatted)
	}
}

func TestSCN002_TUIVelaCopyMentionsFreshnessGuard(t *testing.T) {
	// REQ-004 → SCN-002 → TestSCN002_TUIVelaCopyMentionsFreshnessGuard
	// Scenario: Successful install cleans previous rotta settings before fresh install
	model := New()
	model.Screen = ScreenVela
	model.Target = TargetBoth
	model.SetupAncora = true

	velaView := model.viewVela()
	for _, want := range []string{
		"freshness guard",
		"schedules non-blocking refresh before graph queries",
		"OpenCode plugin",
		"Claude Code hook",
		"cached graph may be used while refresh runs",
	} {
		if !strings.Contains(velaView, want) {
			t.Fatalf("expected Vela screen to mention %q:\n%s", want, velaView)
		}
	}

	model.Screen = ScreenConfirm
	model.SetupVela = true
	confirmView := model.viewConfirm()
	for _, want := range []string{
		"~/.config/opencode/plugin/rotta-vela-freshness-guard.js",
		"~/.claude/hooks/rotta-vela-freshness-guard.sh",
		"graph freshness guard",
		"non-blocking refresh before Vela graph queries",
	} {
		if !strings.Contains(confirmView, want) {
			t.Fatalf("expected confirm screen to mention %q:\n%s", want, confirmView)
		}
	}
}

func TestSCN002_TUIOpenCodePreviewShowsSeparateAncoraAndVelaMCPs(t *testing.T) {
	// REQ-004 → SCN-002 → TestSCN002_TUIOpenCodePreviewShowsSeparateAncoraAndVelaMCPs
	// Scenario: Successful install configures selected optional integrations for OpenCode.
	for _, target := range []string{TargetOpenCode, TargetBoth} {
		t.Run(target, func(t *testing.T) {
			model := New()
			model.Screen = ScreenConfirm
			model.Target = target
			model.ProjectPath = "/tmp/project"
			model.SetupAncora = true
			model.SetupVela = true

			got := model.viewConfirm()
			for _, want := range []string{
				"~/.config/opencode/opencode.jsonc  (mcp.ancora)",
				"~/.config/opencode/opencode.json  (mcp.vela)",
			} {
				if !strings.Contains(got, want) {
					t.Fatalf("missing %q:\n%s", want, got)
				}
			}
		})
	}
}
