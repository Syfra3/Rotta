package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestREQ093_RTKDefaultsToSkipAndRequiresDedicatedConfirmation(t *testing.T) {
	model := New()
	if model.SetupRTK || model.RTKCursor != 1 {
		t.Fatalf("RTK default = selected=%v cursor=%d, want Skip", model.SetupRTK, model.RTKCursor)
	}
	model.Screen = ScreenRTK
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	skipped := next.(Model)
	if skipped.Screen != ScreenConfirm || skipped.SetupRTK || skipped.ConfirmRTK {
		t.Fatalf("RTK skip must bypass confirmation and host action: %#v", skipped)
	}
	model.Screen, model.RTKCursor = ScreenRTK, 0
	next, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	selected := next.(Model)
	if selected.Screen != ScreenRTKConfirm || selected.ConfirmRTK {
		t.Fatalf("RTK selected state = %#v", selected)
	}
	for _, want := range []string{"rtk --version", "host-level installation action", "has not started"} {
		if !strings.Contains(selected.View(), want) {
			t.Fatalf("confirmation missing %q", want)
		}
	}
	next, _ = selected.Update(tea.KeyMsg{Type: tea.KeyEsc})
	cancelled := next.(Model)
	if cancelled.Screen != ScreenRTK || cancelled.SetupRTK || cancelled.ConfirmRTK {
		t.Fatalf("RTK cancellation = %#v", cancelled)
	}

	selected.Screen, selected.SetupRTK = ScreenRTKConfirm, true
	next, _ = selected.Update(tea.KeyMsg{Type: tea.KeyEnter})
	confirmed := next.(Model)
	if confirmed.Screen != ScreenConfirm || !confirmed.SetupRTK || !confirmed.ConfirmRTK {
		t.Fatalf("RTK confirmation = %#v", confirmed)
	}
}
