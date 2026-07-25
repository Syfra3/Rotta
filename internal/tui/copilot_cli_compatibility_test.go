package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// REQ-100 → SCN-402 → TestSCN402_SelectsCopilotCLIAndPreviewsResolvedGlobalFiles
func TestSCN402_SelectsCopilotCLIAndPreviewsResolvedGlobalFiles(t *testing.T) {
	// Scenario: Select GitHub Copilot CLI as the only installation target
	root := filepath.Join(t.TempDir(), "copilot-global")
	t.Setenv("COPILOT_HOME", root)

	model := New()
	model.Screen = ScreenTargetSelect
	model.TargetCursor = 3
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	selected := updated.(Model)
	if selected.Target != TargetCopilotCLI {
		t.Fatalf("expected selecting Copilot CLI to set target %q, got %q", TargetCopilotCLI, selected.Target)
	}

	selected.Screen = ScreenConfirm
	selected.SetupAncora = false
	selected.SetupVela = false
	selected.SetupContext7 = false
	confirmation := selected.viewConfirm()
	for _, want := range []string{
		filepath.Join(root, "agents", "rotta-orchestrator.agent.md"),
		filepath.Join(root, "instructions", "rotta.instructions.md"),
	} {
		if !strings.Contains(confirmation, want) {
			t.Fatalf("expected confirmation to identify resolved Copilot global file %q:\n%s", want, confirmation)
		}
	}
}
