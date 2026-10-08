package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCopilotTargetFlowsToConfirmationWithoutOtherHostRouting(t *testing.T) {
	model := New()
	model.Screen = ScreenTargetSelect
	model.TargetCursor = len(targetKeys) - 1
	chosen, _ := model.updateTargetSelect(tea.KeyMsg{Type: tea.KeyEnter})
	selected := chosen.(Model)
	if selected.Target != TargetCopilot || selected.Screen != ScreenProjectPath {
		t.Fatalf("Copilot target not selected: %+v", selected)
	}
	selected.ProjectInput.SetValue("/tmp/copilot-project")
	next, _ := selected.updateProjectPath(tea.KeyMsg{Type: tea.KeyEnter})
	if got := next.(Model); got.Screen != ScreenAncora {
		t.Fatalf("Copilot entered unrelated model routing: %v", got.Screen)
	}
	selected.Screen = ScreenConfirm
	if view := selected.View(); !strings.Contains(view, "~/.copilot/agents/") || !strings.Contains(view, "~/.copilot/rotta-next/") {
		t.Fatalf("confirmation omits Copilot artifacts: %s", view)
	}
}
