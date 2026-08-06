package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestREQ005A_VelaDefaultsToSkipAndRequiresDedicatedConfirmation(t *testing.T) {
	model := New()
	if model.SetupVela || model.VelaCursor != 1 {
		t.Fatalf("Vela default = selected=%v cursor=%d, want Skip", model.SetupVela, model.VelaCursor)
	}
	model.Target = TargetOpenCode
	model.Screen = ScreenVela
	model.VelaCursor = 0
	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	selected := next.(Model)
	if selected.Screen != ScreenVelaConfirm || selected.VelaConfirmed {
		t.Fatalf("selection = %#v, want unconfirmed Vela confirmation screen", selected)
	}
	view := selected.View()
	for _, want := range []string{"Effective OpenCode config:", "source: XDG global configuration", "brew tap Syfra3/tap", "mcp.vela", "no index, reindex, vela update ., or .vela/ creation"} {
		if !strings.Contains(view, want) {
			t.Fatalf("confirmation missing %q:\n%s", want, view)
		}
	}
	next, _ = selected.Update(tea.KeyMsg{Type: tea.KeyEnter})
	confirmed := next.(Model)
	if !confirmed.VelaConfirmed || confirmed.Screen != ScreenContext7 {
		t.Fatalf("confirmation = %#v", confirmed)
	}
}

func TestREQ005A_VelaConfirmationDisclosesEffectiveConfigSource(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, home, project string)
		wantSource string
		wantPath   func(home, project string) string
	}{
		{
			name: "XDG global",
			setup: func(t *testing.T, home, _ string) {
				t.Setenv("OPENCODE_CONFIG", "")
				t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
			},
			wantSource: "XDG global configuration",
			wantPath:   func(home, _ string) string { return filepath.Join(home, "xdg", "opencode", "opencode.json") },
		},
		{
			name: "OPENCODE_CONFIG",
			setup: func(t *testing.T, home, _ string) {
				t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
				t.Setenv("OPENCODE_CONFIG", filepath.Join(home, "custom", "opencode.jsonc"))
			},
			wantSource: "OPENCODE_CONFIG override",
			wantPath:   func(home, _ string) string { return filepath.Join(home, "custom", "opencode.jsonc") },
		},
		{
			name: "project",
			setup: func(t *testing.T, _, project string) {
				t.Setenv("OPENCODE_CONFIG", "")
				if err := os.MkdirAll(project, 0o750); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(project, "opencode.json"), []byte(`{}`), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			wantSource: "documented project configuration",
			wantPath:   func(_, project string) string { return filepath.Join(project, "opencode.json") },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			project := filepath.Join(home, "project")
			t.Setenv("HOME", home)
			tt.setup(t, home, project)
			model := New()
			model.Target, model.ProjectPath, model.Screen = TargetOpenCode, project, ScreenVelaConfirm
			view := model.View()
			for _, want := range []string{tt.wantPath(home, project), "source: " + tt.wantSource} {
				if !strings.Contains(view, want) {
					t.Fatalf("confirmation missing %q:\n%s", want, view)
				}
			}
		})
	}
}
