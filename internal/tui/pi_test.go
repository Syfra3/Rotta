package tui

import (
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPiAndAllTargetsAreVisibleAndAllRoutesThroughOpenCodeSetup(t *testing.T) {
	model := New()
	model.Width, model.Height = 100, 40
	model.Screen = ScreenTargetSelect
	if !slices.Contains(targets, "Pi") || !slices.Contains(targets, "All") || !slices.Contains(targetKeys, TargetPi) || !slices.Contains(targetKeys, TargetAll) {
		t.Fatal("Pi targets missing from target-selection model")
	}
	model.TargetCursor = 5
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	selected := updated.(Model)
	if selected.Target != TargetAll {
		t.Fatalf("target = %q, want all", selected.Target)
	}
	selected.Screen = ScreenProjectPath
	updated, _ = selected.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if updated.(Model).Screen != ScreenModelRouting {
		t.Fatal("all must retain OpenCode routing setup")
	}
	var summary strings.Builder
	Model{Target: TargetPi}.writeConfirmHostFiles(&summary)
	confirm := summary.String()
	if !strings.Contains(confirm, "executable global Pi extension") {
		t.Fatalf("Pi confirmation missing extension: %s", confirm)
	}
}
