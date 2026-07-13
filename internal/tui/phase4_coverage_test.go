package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Syfra3/Rotta/internal/installer"
	tea "github.com/charmbracelet/bubbletea"
)

func TestSCN222_ConfirmViewDescribesCodexAndDeferredGates(t *testing.T) {
	// REQ-014 → SCN-222 → TestSCN222_ConfirmViewDescribesCodexAndDeferredGates
	// Scenario: Expose selected MCP configuration and runtime fallback states
	view := Model{Target: TargetCodex, UseDefaults: false}.viewConfirm()
	for _, want := range []string{"review later", "~/.codex/AGENTS.md"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected confirmation to contain %q: %s", want, view)
		}
	}
	if view := (Model{Screen: Screen(999)}).View(); view != "" {
		t.Fatalf("expected unknown screen to have no view, got %q", view)
	}
}

func TestSCN222_MCPStatusViewHandlesEmptyAndStandaloneVelaStates(t *testing.T) {
	// REQ-014 → SCN-222 → TestSCN222_MCPStatusViewHandlesEmptyAndStandaloneVelaStates
	// Scenario: Expose selected MCP configuration and runtime fallback states
	var empty strings.Builder
	writeMCPStatuses(&empty, nil)
	if empty.Len() != 0 {
		t.Fatalf("expected no MCP section without statuses, got %q", empty.String())
	}
	if view := (Model{SetupAncora: false}).viewVela(); !strings.Contains(view, "standalone MCP graph server") {
		t.Fatalf("expected standalone Vela description, got %s", view)
	}

	model := Model{Screen: ScreenModeSelect}
	updated, _ := model.updateModeSelect(tea.KeyMsg{Type: tea.KeyEnter})
	if updated.(Model).Screen != ScreenModeSelect {
		t.Fatal("expected mode selection with no modes to remain on the selection screen")
	}
	model.SelectedModes[1] = true
	updated, _ = model.updateModeSelect(tea.KeyMsg{Type: tea.KeyEnter})
	if updated.(Model).Screen != ScreenQualityGates {
		t.Fatal("expected selected mode to advance to quality gates")
	}

	var status strings.Builder
	writeMCPStatuses(&status, map[string]map[string]installer.MCPStatusResult{"codex": {"context7": {Status: installer.MCPStatusDegraded}}})
	if !strings.Contains(status.String(), "codex / context7") {
		t.Fatalf("expected rendered MCP status, got %s", status.String())
	}
}

func TestSCN222_RecoveryManifestReaderRejectsInvalidLocations(t *testing.T) {
	// REQ-014 → SCN-222 → TestSCN222_RecoveryManifestReaderRejectsInvalidLocations
	// Scenario: Expose selected MCP configuration and runtime fallback states
	if _, ok := readRecoveryBackup(filepath.Join(t.TempDir(), "missing", "manifest.json")); ok {
		t.Fatal("expected missing recovery manifest parent to be rejected")
	}
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := readRecoveryBackup(filepath.Join(blocked, "manifest.json")); ok {
		t.Fatal("expected non-directory recovery manifest parent to be rejected")
	}
}
