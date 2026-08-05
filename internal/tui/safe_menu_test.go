package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// REQ-003 -> SCN-004 -> TestSCN004_DefaultMenuSelectsOnlyInstallStatusQuit
func TestSCN004_DefaultMenuSelectsOnlyInstallStatusQuit(t *testing.T) {
	// Scenario: Default invocation offers the normal terminal workflow
	model := New()
	view := model.View()
	for _, want := range []string{"Install", "Status", "Quit"} {
		if !strings.Contains(view, want) {
			t.Fatalf("default menu missing %q:\n%s", want, view)
		}
	}
	for _, unwanted := range []string{"Target", "Recovery", "Ancora", "Context7"} {
		if strings.Contains(view, unwanted) {
			t.Fatalf("default menu exposes %q:\n%s", unwanted, view)
		}
	}
	selected, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	selected, _ = selected.(Model).Update(tea.KeyMsg{Type: tea.KeyDown})
	_, cmd := selected.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("choosing Quit did not return an exit command")
	}
}
