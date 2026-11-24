package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Syfra3/Rotta/internal/installer"
	tea "github.com/charmbracelet/bubbletea"
)

func TestPiDefaultTUIInstallsOfflineProfile(t *testing.T) {
	home := t.TempDir()
	bin := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	t.Setenv("PATH", bin)
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > '" + filepath.Join(home, "args") + "'\nprintf 'provider model context max-out\\nopenai-codex gpt-6-sol 1 2\\nopenai-codex gpt-6.1-sol 1 2\\nopenai-codex gpt-6.1-terra 1 2\\nopenai-codex gpt-6-luna 1 2\\nopenai-codex gpt-6.2-sol 1 2\\n'\n"
	if err := os.WriteFile(filepath.Join(bin, "pi"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	m := New()
	m.Target = TargetPi
	m.SetupTelemetry = false
	m.Screen = ScreenPiModelRouting
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.PiModelRouting != installer.ModelRoutingEnabled {
		t.Fatal(m.PiModelRouting)
	}
	result := runInstall(m)().(installDoneMsg)
	if result.err != nil {
		t.Fatal(result.err)
	}
	profile, err := os.ReadFile(filepath.Join(home, ".pi/agent/rotta-next/model-routing.json"))
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Roles map[string]installer.PiRoutingAssignment `json:"roles"`
	}
	if err = json.Unmarshal(profile, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Roles["reviewer"] != (installer.PiRoutingAssignment{Model: "openai-codex/gpt-6.1-sol", Effort: "medium"}) || decoded.Roles["implementation"] != (installer.PiRoutingAssignment{Model: "openai-codex/gpt-6.1-terra", Effort: "high"}) || decoded.Roles["exploration"] != (installer.PiRoutingAssignment{Model: "openai-codex/gpt-6-luna", Effort: "medium"}) || decoded.Roles["operations"] != (installer.PiRoutingAssignment{Model: "openai-codex/gpt-6-luna", Effort: "medium"}) {
		t.Fatalf("profile %s", profile)
	}
	settings, err := os.ReadFile(filepath.Join(home, ".pi/agent/settings.json"))
	if err != nil || !strings.Contains(string(settings), `"defaultModel": "gpt-6.1-sol"`) || !strings.Contains(string(settings), `"defaultThinkingLevel": "low"`) {
		t.Fatalf("settings %s %v", settings, err)
	}
	args, _ := os.ReadFile(filepath.Join(home, "args"))
	if string(args) != "--offline\n--list-models\n" {
		t.Fatalf("args %q", args)
	}
}

func TestPiDefaultTUIWarnsWhenCatalogUnavailable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	t.Setenv("PATH", t.TempDir())
	m := New()
	m.Target = TargetPi
	m.SetupTelemetry = false
	m.PiModelRouting = installer.ModelRoutingEnabled
	result := runInstall(m)().(installDoneMsg)
	if result.err != nil {
		t.Fatal(result.err)
	}
	if len(result.result.Warnings) == 0 || !strings.Contains(strings.Join(result.result.Warnings, " "), "without verified availability") {
		t.Fatalf("warnings %v", result.result.Warnings)
	}
	profile, err := os.ReadFile(filepath.Join(home, ".pi/agent/rotta-next/model-routing.json"))
	if err != nil || !strings.Contains(string(profile), "gpt-6.1-terra") {
		t.Fatalf("profile %s %v", profile, err)
	}
}
