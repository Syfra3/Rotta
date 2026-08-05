package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRottaNextTUIShowsFastAndStrictModes(t *testing.T) {
	view := New().View()
	for _, want := range []string{"Fast mode", "Strict mode", "Orchestrator, Explore, Implementation, Review, Operations"} {
		if !strings.Contains(view, want) {
			t.Fatalf("welcome view missing %q", want)
		}
	}
}

func TestRottaNextTUIProjectSelectionSkipsRetiredModeScreens(t *testing.T) {
	model := New()
	model.Screen = ScreenProjectPath
	model.ProjectInput.SetValue("/tmp/project")
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := next.(Model).Screen; got != ScreenAncora {
		t.Fatalf("project selection screen = %v, want Ancora setup", got)
	}
}

func TestRottaNextTUIVelaConfirmationDoesNotPromiseIndexing(t *testing.T) {
	model := New()
	model.Screen = ScreenConfirm
	model.SetupVela = true
	view := model.View()
	if !strings.Contains(view, "No graph state is created during installation") {
		t.Fatal("Vela confirmation does not disclose no-indexing behavior")
	}
	if strings.Contains(view, "freshness guard") {
		t.Fatal("Vela confirmation still promises retired automatic refresh behavior")
	}
}
