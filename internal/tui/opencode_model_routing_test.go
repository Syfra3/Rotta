package tui

import (
	"strings"
	"testing"

	"github.com/Syfra3/Rotta/internal/installer"
	tea "github.com/charmbracelet/bubbletea"
)

// REQ-003 → SCN-003 → TestSCN003_KeyboardRoutingSelectionReachesConfirmation
func TestSCN003_KeyboardRoutingSelectionReachesConfirmation(t *testing.T) {
	// Scenario: TUI selection is keyboard accessible and confirmation-gated.
	model := New()
	model.Screen = ScreenModelRouting
	moved, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	selected, _ := moved.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := selected.(Model)
	if got.ModelRouting != installer.ModelRoutingDisabled {
		t.Fatalf("routing request = %q, want explicitly disabled", got.ModelRouting)
	}
	got.Screen = ScreenConfirm
	if view := got.View(); !strings.Contains(view, "explicitly disabled") {
		t.Fatalf("confirmation does not state pending routing selection:\n%s", view)
	}
	cancelled, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if afterCancel := cancelled.(Model); afterCancel.Screen != ScreenProjectPath || afterCancel.Installing || afterCancel.InstallResult != nil {
		t.Fatalf("selection cancel initiated a mutation: %#v", afterCancel)
	}
	got.ConfirmCursor = 0
	declined, _ := got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if afterDecline := declined.(Model); afterDecline.Installing || afterDecline.InstallResult != nil {
		t.Fatalf("confirmation decline initiated a mutation: %#v", afterDecline)
	}
}

func TestTUIVisibleEnabledSelectionIsExplicitWhileDefaultRemainsUnset(t *testing.T) {
	model := New()
	if model.ModelRouting != installer.ModelRoutingUnset {
		t.Fatalf("default routing request = %q, want unset", model.ModelRouting)
	}
	model.Screen = ScreenModelRouting
	selected, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := selected.(Model)
	if got.ModelRouting != installer.ModelRoutingEnabled {
		t.Fatalf("visible enabled selection = %q, want explicitly enabled", got.ModelRouting)
	}
	got.Screen = ScreenConfirm
	if view := got.View(); !strings.Contains(view, "explicitly enabled") {
		t.Fatalf("confirmation does not state explicit enabled selection:\n%s", view)
	}
}
