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
	writeMCPStatuses(&status, map[string]map[string]installer.MCPStatusResult{"codex": {"context7": {Status: installer.MCPStatusDegraded, Reason: "health deferred", Remediation: "verify in Codex", RuntimeFallback: installer.MCPRuntimeFallback{State: installer.MCPRuntimeFallbackNotObserved}}}})
	for _, want := range []string{"codex / context7", "Reason: health deferred", "Remediation: verify in Codex", "Runtime fallback:"} {
		if !strings.Contains(status.String(), want) {
			t.Fatalf("expected rendered MCP status to contain %q: %s", want, status.String())
		}
	}
}

func TestSCN222_MCPStatusViewOrdersHostsAndCapabilitiesWithDetails(t *testing.T) {
	// REQ-014 → SCN-222 → TestSCN222_MCPStatusViewOrdersHostsAndCapabilitiesWithDetails
	// Scenario: Expose selected MCP configuration and runtime fallback states
	var output strings.Builder
	writeMCPStatuses(&output, map[string]map[string]installer.MCPStatusResult{
		"opencode": {"vela": degradedMCPStatus("Vela stale"), "ancora": degradedMCPStatus("Ancora unavailable")},
		"claude":   {"context7": degradedMCPStatus("Context7 timeout")},
	})
	assertOrderedMCPStatus(t, output.String(), "claude / context7", "opencode / ancora", "opencode / vela")
	assertMCPStatusDetails(t, output.String())
}

func TestSCN222_MCPStatusViewAlwaysOrdersHostsDeterministically(t *testing.T) {
	// REQ-014 → SCN-222 → TestSCN222_MCPStatusViewAlwaysOrdersHostsDeterministically
	// Scenario: Expose selected MCP configuration and runtime fallback states
	statuses := map[string]map[string]installer.MCPStatusResult{
		"opencode": {"context7": degradedMCPStatus("Context7 timeout")},
		"claude":   {"context7": degradedMCPStatus("Context7 timeout")},
	}
	for range 128 {
		var output strings.Builder
		writeMCPStatuses(&output, statuses)
		assertOrderedMCPStatus(t, output.String(), "claude / context7", "opencode / context7")
	}
}

func TestSCN222_HostMCPStatusViewOrdersCapabilitiesDirectly(t *testing.T) {
	// REQ-014 → SCN-222 → TestSCN222_HostMCPStatusViewOrdersCapabilitiesDirectly
	// Scenario: Expose selected MCP configuration and runtime fallback states
	var output strings.Builder
	writeHostMCPStatuses(&output, "opencode", map[string]installer.MCPStatusResult{
		"vela":     degradedMCPStatus("Vela stale"),
		"context7": degradedMCPStatus("Context7 timeout"),
	})
	assertOrderedMCPStatus(t, output.String(), "opencode / context7", "opencode / vela")
}

func degradedMCPStatus(reason string) installer.MCPStatusResult {
	return installer.MCPStatusResult{Status: installer.MCPStatusDegraded, Reason: reason, Remediation: "retry safely", RuntimeFallback: installer.MCPRuntimeFallback{State: installer.MCPRuntimeFallbackNotObserved}}
}

func assertOrderedMCPStatus(t *testing.T, text string, entries ...string) {
	t.Helper()
	previous := -1
	for _, entry := range entries {
		current := strings.Index(text, entry)
		if current < 0 || current <= previous {
			t.Fatalf("expected deterministic MCP status order %q in %q", entries, text)
		}
		previous = current
	}
}

func assertMCPStatusDetails(t *testing.T, text string) {
	t.Helper()
	for _, detail := range []string{"Reason: Vela stale", "Reason: Ancora unavailable", "Reason: Context7 timeout", "Remediation: retry safely", "Runtime fallback:"} {
		if !strings.Contains(text, detail) {
			t.Fatalf("expected selected MCP status detail %q in %q", detail, text)
		}
	}
	if !strings.HasSuffix(text, "\n\n") {
		t.Fatalf("expected MCP status block to terminate before the next view section: %q", text)
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
