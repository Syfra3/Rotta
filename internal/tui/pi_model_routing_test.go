package tui

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Syfra3/Rotta/internal/installer"
	tea "github.com/charmbracelet/bubbletea"
)

func TestPiCustomRoutingUsesOfflineDiscoveryAndRetainsDefaultsOnFailure(t *testing.T) {
	original := discoverPiModels
	discoverPiModels = func() ([]string, error) {
		return nil, errors.New("Pi CLI is unavailable; install Pi or choose Default or Disabled")
	}
	t.Cleanup(func() { discoverPiModels = original })
	model := New()
	model.Target = TargetPi
	model.Screen = ScreenPiModelRouting
	model.PiModelRoutingCursor = 1
	next, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated, _ := next.(Model).Update(command())
	got := updated.(Model)
	if !reflect.DeepEqual(got.PiCustomRouting, installer.DefaultPiRouting()) || !reflect.DeepEqual(got.PiCustomEfforts, installer.DefaultPiRoutingEfforts()) || got.PiOrchestratorModel != installer.DefaultPiOrchestratorModel() || got.PiOrchestratorEffort != installer.DefaultPiOrchestratorEffort() {
		t.Fatalf("defaults lost: orchestrator=%s/%s models=%#v efforts=%#v", got.PiOrchestratorModel, got.PiOrchestratorEffort, got.PiCustomRouting, got.PiCustomEfforts)
	}
	if !strings.Contains(got.View(), "Pi CLI is unavailable") {
		t.Fatalf("discovery limitation absent: %s", got.View())
	}
}

func TestPiDiscoveryParserAndExactOfflineCommand(t *testing.T) {
	output := []byte("provider  model  context  max-out\n--------------------------------\nopenai-codex  gpt-5.6-terra  1  2\nanthropic  claude-sonnet-4-5  1  2\nopenai-codex  gpt-5.6-terra  1  2\ninvalid row\n")
	want := []string{"anthropic/claude-sonnet-4-5", "openai-codex/gpt-5.6-terra"}
	if got := parsePiModels(output); !reflect.DeepEqual(got, want) {
		t.Fatalf("parsed = %#v", got)
	}
	if got := parsePiModels([]byte("provider model\n---\n")); len(got) != 0 {
		t.Fatalf("empty table = %#v", got)
	}
	dir := t.TempDir()
	observed := filepath.Join(dir, "args")
	command := filepath.Join(dir, "pi")
	if err := os.WriteFile(command, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > "+observed+"\nprintf 'provider model context max-out\\nacme model-a 1 2\\n'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	original := piModelLookPath
	piModelLookPath = func(name string) (string, error) {
		if name != "pi" {
			t.Fatalf("looked up %q", name)
		}
		return command, nil
	}
	t.Cleanup(func() { piModelLookPath = original })
	got, err := discoverPiModelsDirect()
	if err != nil || !reflect.DeepEqual(got, []string{"acme/model-a"}) {
		t.Fatalf("discover = %#v, %v", got, err)
	}
	args, err := os.ReadFile(observed)
	if err != nil || string(args) != "--offline\n--list-models\n" {
		t.Fatalf("args = %q, %v", args, err)
	}
}

func TestAllTargetDiscoveryResultsStayHostSpecificAcrossNavigation(t *testing.T) {
	model := New()
	model.Target = TargetAll
	model.Screen = ScreenModelRouting
	model.ModelRoutingCursor = 1
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	openCode := next.(Model)
	if !openCode.ModelDiscoveryInFlight || openCode.PiModelDiscoveryInFlight {
		t.Fatalf("OpenCode state = %#v", openCode)
	}
	openCode.Screen = ScreenPiModelRouting
	openCode.PiModelRoutingCursor = 1
	next, _ = openCode.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pi := next.(Model)
	if !pi.ModelDiscoveryInFlight || !pi.PiModelDiscoveryInFlight {
		t.Fatalf("in-flight states lost: %#v", pi)
	}
	afterPi, _ := pi.Update(modelDiscoveryResultMsg{pi: true, generation: pi.PiModelDiscoveryGeneration, models: []string{"pi/model"}})
	afterOpenCode, _ := afterPi.(Model).Update(modelDiscoveryResultMsg{generation: pi.ModelDiscoveryGeneration, models: []string{"open/code"}})
	got := afterOpenCode.(Model)
	if !reflect.DeepEqual(got.PiAvailableModels, []string{"pi/model"}) || !reflect.DeepEqual(got.AvailableModels, []string{"open/code"}) {
		t.Fatalf("cross-host catalog leak: %#v", got)
	}
	got.Screen = ScreenPiModelPicker
	if view := got.View(); !strings.Contains(view, "pi/model") || strings.Contains(view, "open/code") {
		t.Fatalf("Pi view leaked catalog: %s", view)
	}
	got.Screen = ScreenModelPicker
	if view := got.View(); !strings.Contains(view, "open/code") || strings.Contains(view, "pi/model") {
		t.Fatalf("OpenCode view leaked catalog: %s", view)
	}
}

func TestDiscoveryBackNavigationDoesNotCrossHostCatalog(t *testing.T) {
	model := New()
	model.Target = TargetAll
	model.Screen = ScreenPiCustomModelRouting
	model.PiAvailableModels = []string{"pi/model"}
	model.PiModelDiscoveryDone = true
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := next.(Model)
	if got.Screen != ScreenPiModelRouting || !reflect.DeepEqual(got.PiAvailableModels, []string{"pi/model"}) || len(got.AvailableModels) != 0 {
		t.Fatalf("Pi back navigation = %#v", got)
	}
}

func TestPiRoutingSelectionReachesConfirmationIndependently(t *testing.T) {
	model := New()
	model.Target = TargetPi
	model.Screen = ScreenPiModelRouting
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(Model)
	if got.PiModelRouting != installer.ModelRoutingEnabled || got.Screen != ScreenAncora {
		t.Fatalf("Pi default = %#v", got)
	}
	got.Screen = ScreenConfirm
	if view := got.View(); !strings.Contains(view, "Pi model routing:") || !strings.Contains(view, "model-routing.json") || !strings.Contains(view, "models.json") || !strings.Contains(view, "gpt-6-sol and gpt-6-luna context windows: 1000000") {
		t.Fatalf("Pi confirmation missing: %s", view)
	}
}

func TestPiDisabledRoutingConfirmationOmitsOrchestratorSettings(t *testing.T) {
	model := New()
	model.Target = TargetPi
	model.Screen = ScreenConfirm
	model.PiModelRouting = installer.ModelRoutingDisabled
	view := model.View()
	if strings.Contains(view, "~/.pi/agent/settings.json") || strings.Contains(view, "~/.pi/agent/models.json") {
		t.Fatalf("disabled Pi routing should not claim settings or models write: %s", view)
	}
}

func TestPiCustomRoutingCyclesEffortAndConfirmsSelection(t *testing.T) {
	model := New()
	model.Target = TargetPi
	model.Screen = ScreenPiModelRouting
	model.PiModelRoutingCursor = 1
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(Model)
	got.PiModelDiscoveryDone = true
	got.PiAvailableModels = []string{"openai-codex/gpt-5.6-sol"}
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	got = next.(Model)
	if got.PiOrchestratorEffort != "medium" {
		t.Fatalf("orchestrator effort did not cycle from low to medium: %#v", got.PiOrchestratorEffort)
	}
	if view := got.View(); !strings.Contains(view, "Orchestrator: openai-codex/gpt-6-sol (medium)") || !strings.Contains(view, "Implementation: openai-codex/gpt-6-sol (low)") {
		t.Fatalf("custom view missing effort: %s", view)
	}
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyDown})
	got = next.(Model)
	next, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	got = next.(Model)
	if got.PiCustomEfforts["implementation"] != "medium" {
		t.Fatalf("implementation effort did not cycle from low to medium: %#v", got.PiCustomEfforts)
	}
	got.Screen = ScreenConfirm
	if view := got.View(); !strings.Contains(view, "Orchestrator:") || !strings.Contains(view, "Implementation:") || !strings.Contains(view, "(medium)") {
		t.Fatalf("confirm view missing effort: %s", view)
	}
}
