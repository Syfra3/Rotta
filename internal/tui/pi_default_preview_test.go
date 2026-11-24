package tui

import (
	"strings"
	"testing"

	"github.com/Syfra3/Rotta/internal/installer"
)

func TestPiDefaultConfirmationShowsCatalogPreviewAsRechecked(t *testing.T) {
	m := New()
	m.Target, m.Screen, m.PiModelRouting = TargetPi, ScreenConfirm, installer.ModelRoutingEnabled
	m.PiModelDiscoveryDone = true
	m.PiAvailableModels = []string{"openai-codex/gpt-6-sol", "openai-codex/gpt-6.1-sol", "openai-codex/gpt-6.1-luna"}
	view := m.View()
	for _, want := range []string{"pinned preset", "Orchestrator:  openai-codex/gpt-6.1-sol (low)", "Implementation:  openai-codex/gpt-6.1-terra (high)", "Reviewer:  openai-codex/gpt-6.1-sol (medium)", "Exploration:  openai-codex/gpt-6-luna (medium)", "Operations:  openai-codex/gpt-6-luna (medium)", "not all present"} {
		if !strings.Contains(view, want) {
			t.Errorf("missing %q in %s", want, view)
		}
	}
}
