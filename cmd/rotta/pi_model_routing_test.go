package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestCLIPiCustomRoutingAcceptsFourIndependentAssignments(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	args := []string{"install", "--target", "pi", "--pi-model-routing", "custom",
		"--pi-model", "implementation=openai-codex/gpt-5.6-terra",
		"--pi-model", "reviewer=openai-codex/gpt-5.6-sol",
		"--pi-model", "exploration=openai-codex/gpt-5.6-luna",
		"--pi-model", "operations=openai-codex/gpt-5.6-luna"}
	if err := runCLI(args, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".pi", "agent", "rotta-next", "model-routing.json")); err != nil {
		t.Fatal(err)
	}
	settings, err := os.ReadFile(filepath.Join(home, ".pi", "agent", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(settings, []byte(`"defaultModel": "gpt-6-sol"`)) ||
		!bytes.Contains(settings, []byte(`"defaultThinkingLevel": "low"`)) {
		t.Fatalf("CLI custom routing lost default orchestrator: %s", settings)
	}
}
