package tui

import (
	"strings"
	"testing"

	"github.com/Syfra3/Rotta/internal/installer"
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

func TestDeliveryFirstTUIDisplaysReconciliationWarnings(t *testing.T) {
	model := New()
	warning := "OpenCode work-record reconciliation required in /custom/opencode.json: preserved agent.rotta-orchestrator.tools.edit=false"
	model.InstallResult = &installer.Result{Warnings: []string{warning}}
	for _, view := range []string{model.viewSuccess(), model.viewError()} {
		if !strings.Contains(view, "Warning: "+warning) {
			t.Fatalf("TUI hid reconciliation warning: %s", view)
		}
	}
}

func TestRottaNextTUIProjectSelectionSkipsRetiredModeScreens(t *testing.T) {
	model := New()
	model.Screen = ScreenProjectPath
	model.ProjectInput.SetValue("/tmp/project")
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := next.(Model).Screen; got != ScreenModelRouting {
		t.Fatalf("project selection screen = %v, want model-routing selection", got)
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
