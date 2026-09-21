package tui

import (
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTypeSafeKeyInputAcceptsTypedAndPastedText(t *testing.T) {
	model := New()
	model.Target = TargetPi
	model.Screen = ScreenTypeSafeKey
	model.TypeSafeKeyInput.Focus()

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ts-key")})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("-pasted")})
	model = updated.(Model)
	if got := model.TypeSafeKeyInput.Value(); got != "ts-key-pasted" {
		t.Fatalf("TypeSafe key input value = %q, want typed and pasted text", got)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.TypeSafeAPIKey != "ts-key-pasted" || model.Screen != ScreenConfirm {
		t.Fatalf("submitted TypeSafe key/screen = %q/%v", model.TypeSafeAPIKey, model.Screen)
	}
}

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
	piOnly := Model{Target: TargetPi}
	piOnly.writeConfirmSummary(&summary)
	if strings.Contains(summary.String(), "OpenCode model routing:") {
		t.Fatalf("Pi-only confirmation leaked OpenCode routing: %s", summary.String())
	}
	if !targetIncludesOpenCode(TargetAll) || !targetIncludesPi(TargetAll) {
		t.Fatal("all target did not retain both routing summaries")
	}
}
