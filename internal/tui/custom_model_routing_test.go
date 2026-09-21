package tui

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Syfra3/Rotta/internal/installer"
	tea "github.com/charmbracelet/bubbletea"
)

func TestCustomRoutingCopiesDefaultsAndShowsResolvedAssignments(t *testing.T) {
	model := New()
	model.Target = TargetOpenCode
	model.Screen = ScreenModelRouting
	model.ModelRoutingCursor = 1
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(Model)
	if got.Screen != ScreenCustomModelRouting || got.ModelRouting != installer.ModelRoutingCustom {
		t.Fatalf("custom selection = screen %v, routing %q", got.Screen, got.ModelRouting)
	}
	if !reflect.DeepEqual(got.CustomRouting, installer.DefaultOpenCodeRouting()) {
		t.Fatalf("custom routing = %#v, want exact default copy", got.CustomRouting)
	}
	got.AvailableModels = []string{"anthropic/claude-sonnet", "openai/gpt-5.6-terra"}
	got.ModelDiscoveryDone = true
	picker, _ := got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	selected, _ := picker.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = selected.(Model)
	if got.CustomRouting["rotta-orchestrator"] != "anthropic/claude-sonnet" {
		t.Fatalf("selected model = %q", got.CustomRouting["rotta-orchestrator"])
	}
	got.Screen = ScreenConfirm
	view := got.View()
	for _, want := range []string{"Custom", "Orchestration:", "anthropic/claude-sonnet", "Implementation:"} {
		if !strings.Contains(view, want) {
			t.Fatalf("confirmation missing %q:\n%s", want, view)
		}
	}
}

func TestParseOpenCodeModelsDeduplicatesAndSortsProviderModelLines(t *testing.T) {
	got := parseOpenCodeModels([]byte("zeta/model-z\nalpha/model-a\nzeta/model-z\nprovider|bad/model\nprovider\\bad/model\nnot a model\n"))
	want := []string{"alpha/model-a", "zeta/model-z"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseOpenCodeModels() = %#v, want %#v", got, want)
	}
}

func TestCustomRoutingShowsDiscoveryFailureWithoutDiscardingDefaults(t *testing.T) {
	original := discoverOpenCodeModels
	discoverOpenCodeModels = func() ([]string, error) {
		return nil, errors.New("OpenCode CLI is unavailable; install OpenCode or choose Default or Disabled")
	}
	t.Cleanup(func() { discoverOpenCodeModels = original })

	model := New()
	model.Target = TargetOpenCode
	model.Screen = ScreenModelRouting
	model.ModelRoutingCursor = 1
	next, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	message := command()
	updated, _ := next.(Model).Update(message)
	got := updated.(Model)
	if !reflect.DeepEqual(got.CustomRouting, installer.DefaultOpenCodeRouting()) {
		t.Fatalf("discovery failure discarded defaults: %#v", got.CustomRouting)
	}
	if view := got.View(); !strings.Contains(view, "OpenCode CLI is unavailable") {
		t.Fatalf("custom routing does not show discovery failure:\n%s", view)
	}
}

func TestModelPickerSearchAcceptsNavigationLetters(t *testing.T) {
	model := New()
	model.Screen = ScreenModelPicker
	model.AvailableModels = []string{"alpha/bravo", "zeta/model"}
	model.CustomRouting = installer.DefaultOpenCodeRouting()
	var got Model
	for _, key := range "jkb" {
		next, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
		model = next.(Model)
		got = model
	}
	if got.ModelPickerQuery != "jkb" {
		t.Fatalf("picker query = %q, want printable navigation letters", got.ModelPickerQuery)
	}
}

func TestCustomDiscoveryIgnoresStaleResultsAndRetriesExplicitly(t *testing.T) {
	model := New()
	model.Screen = ScreenCustomModelRouting
	model.Target = TargetOpenCode
	model.CustomRouting = installer.DefaultOpenCodeRouting()
	model.ModelDiscoveryError = "OpenCode CLI is unavailable"
	model.ModelDiscoveryDone = true
	retried, command := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	got := retried.(Model)
	if !got.ModelDiscoveryInFlight || got.ModelDiscoveryGeneration != 1 || command == nil {
		t.Fatalf("retry did not start one discovery: %#v", got)
	}
	stale, _ := got.Update(modelDiscoveryResultMsg{generation: 0, models: []string{"stale/model"}})
	if afterStale := stale.(Model); len(afterStale.AvailableModels) != 0 {
		t.Fatalf("stale discovery overwrote models: %#v", afterStale.AvailableModels)
	}
}

func TestCustomDiscoveryDoesNotStartDuplicateRequestOnReentry(t *testing.T) {
	model := New()
	model.Target = TargetOpenCode
	model.Screen = ScreenModelRouting
	model.ModelRoutingCursor = 1
	entered, firstCommand := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if firstCommand == nil {
		t.Fatal("initial custom entry did not start discovery")
	}
	back, _ := entered.(Model).Update(tea.KeyMsg{Type: tea.KeyEsc})
	reentered, secondCommand := back.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := reentered.(Model)
	if secondCommand != nil || got.ModelDiscoveryGeneration != 1 || !got.ModelDiscoveryInFlight {
		t.Fatalf("custom reentry started duplicate discovery: %#v", got)
	}
}

func TestNonOpenCodeTargetBypassesRoutingAndNeverStartsDiscovery(t *testing.T) {
	for _, target := range []string{TargetClaudeCode, TargetCodex} {
		t.Run(target, func(t *testing.T) {
			model := New()
			model.Target = target
			model.Screen = ScreenProjectPath
			model.ProjectInput.SetValue("~/project")
			next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
			got := next.(Model)
			if got.Screen != ScreenAncora || got.ModelRouting != installer.ModelRoutingUnset || got.ModelDiscoveryInFlight {
				t.Fatalf("non-OpenCode target entered routing: %#v", got)
			}
		})
	}
}
