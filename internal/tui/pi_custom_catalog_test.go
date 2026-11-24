package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPiCustomCatalogUpdateAndUserEdits(t *testing.T) {
	m := New()
	m.Target, m.Screen, m.PiModelRoutingCursor = TargetPi, ScreenPiModelRouting, 1
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected offline discovery")
	}
	m = next.(Model)
	// Effort changes while discovery is in flight must survive the result.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = next.(Model)
	catalog := []string{"openai-codex/gpt-6-sol", "openai-codex/gpt-6-luna", "openai-codex/gpt-6.1-sol", "openai-codex/gpt-6.1-luna", "other/model"}
	next, _ = m.Update(modelDiscoveryResultMsg{pi: true, generation: m.PiModelDiscoveryGeneration, models: catalog})
	m = next.(Model)
	view := m.View()
	for _, want := range []string{"Orchestrator: openai-codex/gpt-6.1-sol (medium)", "Implementation: openai-codex/gpt-6.1-terra (high)", "Reviewer: openai-codex/gpt-6.1-sol (medium)", "Exploration: openai-codex/gpt-6-luna (medium)", "Operations: openai-codex/gpt-6-luna (medium)"} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q in %s", want, view)
		}
	}
	// Choose an explicit orchestrator model too; refreshed defaults must not overwrite it.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	m.PiModelPickerCursor = 0
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	// Choose an explicit role model through the real picker, then change its effort.
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	m.PiModelPickerCursor = 4
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	m.PiModelDiscoveryGeneration++
	next, _ = m.Update(modelDiscoveryResultMsg{pi: true, generation: m.PiModelDiscoveryGeneration, models: []string{"openai-codex/gpt-7-sol", "openai-codex/gpt-7-luna"}})
	m = next.(Model)
	view = m.View()
	for _, want := range []string{"Orchestrator: openai-codex/gpt-6-sol (medium)", "Implementation: other/model (xhigh)", "Exploration: openai-codex/gpt-6-luna (medium)"} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q after refresh in %s", want, view)
		}
	}
}

func TestPiCustomAlreadyDiscoveredAndFallback(t *testing.T) {
	for _, tc := range []struct {
		name      string
		models    []string
		err, want string
	}{
		{"cached", []string{"openai-codex/gpt-6-sol", "openai-codex/gpt-6.1-sol", "openai-codex/gpt-6.1-luna"}, "", "Implementation: openai-codex/gpt-6.1-terra (high)"},
		{"no numeric", []string{"other/model"}, "", "Pinned defaults not all present in offline catalog; selections retained, provider access not verified"},
		{"unavailable", nil, "offline unavailable", "offline unavailable; existing defaults retained, availability unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := New()
			m.Target, m.Screen, m.PiModelRoutingCursor = TargetPi, ScreenPiModelRouting, 1
			m.PiModelDiscoveryDone, m.PiAvailableModels, m.PiModelDiscoveryError = true, tc.models, tc.err
			next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
			m = next.(Model)
			if cmd != nil {
				t.Fatal("unexpected discovery")
			}
			if view := m.View(); !strings.Contains(view, tc.want) {
				t.Fatalf("missing %q in %s", tc.want, view)
			}
		})
	}
}
