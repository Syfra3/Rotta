package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestLocalTelemetryRequiresExplicitChoiceAndConfirmation(t *testing.T) {
	m := New()
	if !m.SetupTelemetry || m.TelemetryCursor != 0 {
		t.Fatal("interactive telemetry selection and state must default to accept")
	}
	m.Screen = ScreenTelemetry
	view := m.View()
	for _, text := range []string{"▸ Accept local telemetry (default)", "Decline", "local loopback", "Persistent Docker volumes", "7 days", "Pi/OpenCode", "No prompts, transcripts or secrets", "no metrics uploaded to Rotta", "network requests", "install.py uninstall --host", "j/k to move · Enter to select · Esc to go back"} {
		if !strings.Contains(view, text) {
			t.Errorf("rendered telemetry view missing %q", text)
		}
	}
	selected, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	accepted := selected.(Model)
	if cmd != nil || !accepted.SetupTelemetry || accepted.Screen != ScreenConfirm || !strings.Contains(accepted.View(), "OPT IN") {
		t.Fatal("accept must only advance to the final confirmation")
	}
	back, _ := accepted.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = back.(Model)
	if m.Screen != ScreenTelemetry || m.TelemetryCursor != 0 {
		t.Fatal("confirmation must allow revisiting acceptance")
	}
	for _, key := range []tea.KeyMsg{{Type: tea.KeyDown}, {Type: tea.KeyUp}, {Type: tea.KeyRunes, Runes: []rune("j")}} {
		next, _ := m.Update(key)
		m = next.(Model)
	}
	if m.TelemetryCursor != 1 || !strings.Contains(m.View(), "▸ Decline") {
		t.Fatal("decline must be visibly selectable")
	}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	declined := next.(Model)
	if cmd != nil || declined.SetupTelemetry || declined.Screen != ScreenConfirm || !strings.Contains(declined.View(), "off (no telemetry setup)") {
		t.Fatal("decline must only advance to off confirmation")
	}
	next, _ = declined.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.Screen != ScreenTelemetry || m.SetupTelemetry || m.TelemetryCursor != 1 || !strings.Contains(m.View(), "▸ Decline") {
		t.Fatal("decline must survive confirmation back")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(Model)
	if m.Screen != ScreenContext7 {
		t.Fatal("escape must return to Context7")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.Screen != ScreenTelemetry || m.SetupTelemetry || m.TelemetryCursor != 1 {
		t.Fatal("decline must survive Context7 revisit")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if next.(Model).TelemetryCursor != 0 {
		t.Fatal("k must select accept")
	}
}

func TestTUIDeclineRunsCoreWithoutTelemetry(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	bin := t.TempDir()
	marker := filepath.Join(home, "python-called")
	if err := os.WriteFile(filepath.Join(bin, "python3"), []byte("#!/bin/sh\ntouch \"$TELEMETRY_MARKER\"\nexit 99\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	t.Setenv("TELEMETRY_MARKER", marker)
	for _, choice := range []string{"declined", "declined after revisit"} {
		t.Run(choice, func(t *testing.T) {
			m := New()
			m.Target = "copilot"
			m.ProjectPath = home
			m.Screen = ScreenTelemetry
			next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
			next, _ = next.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
			confirmed := next.(Model)
			if choice == "declined after revisit" {
				next, _ = confirmed.Update(tea.KeyMsg{Type: tea.KeyEsc})
				next, _ = next.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
				confirmed = next.(Model)
			}
			if confirmed.SetupTelemetry || confirmed.Screen != ScreenConfirm || !strings.Contains(confirmed.viewConfirm(), "off (no telemetry setup)") {
				t.Fatal("decline not reflected in confirmation")
			}
			result := runInstall(confirmed)().(installDoneMsg)
			if result.err != nil || result.result == nil {
				t.Fatalf("core install: %v", result.err)
			}
			if _, err := os.Stat(filepath.Join(home, ".copilot", "agents", "rotta-orchestrator.agent.md")); err != nil {
				t.Fatalf("core asset: %v", err)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("telemetry invoked: %v", err)
			}
		})
	}
}
