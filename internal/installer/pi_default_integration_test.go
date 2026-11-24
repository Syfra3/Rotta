package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallPiCustomDoesNotDiscoverOrChangeAssignments(t *testing.T) {
	home, bin := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	t.Setenv("PATH", bin)
	marker := filepath.Join(home, "invoked")
	if err := os.WriteFile(filepath.Join(bin, "pi"), []byte("#!/bin/sh\necho invoked > '"+marker+"'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	custom := map[string]string{"implementation": "custom/impl", "reviewer": "custom/review", "exploration": "custom/explore", "operations": "custom/ops"}
	_, err := Install(Options{Target: "pi", PiModelRouting: ModelRoutingCustom, PiModelRoutingModels: custom, PiModelRoutingEfforts: map[string]string{"implementation": "max", "reviewer": "high", "exploration": "low", "operations": "off"}, PiOrchestratorModel: "custom/parent", PiOrchestratorEffort: "high"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("unexpected discovery: %v", err)
	}
	profile, err := os.ReadFile(filepath.Join(home, ".pi/agent/rotta-next/model-routing.json"))
	if err != nil {
		t.Fatal(err)
	}
	var p struct {
		Roles map[string]PiRoutingAssignment `json:"roles"`
	}
	if err = json.Unmarshal(profile, &p); err != nil {
		t.Fatal(err)
	}
	efforts := map[string]string{"implementation": "max", "reviewer": "high", "exploration": "low", "operations": "off"}
	for role, model := range custom {
		if p.Roles[role].Model != model || p.Roles[role].Effort != efforts[role] {
			t.Fatalf("%s: %s", role, profile)
		}
	}
	settings, err := os.ReadFile(filepath.Join(home, ".pi/agent/settings.json"))
	if err != nil || (!strings.Contains(string(settings), `"defaultModel": "parent"`) || !strings.Contains(string(settings), `"defaultThinkingLevel": "high"`)) {
		t.Fatalf("settings %s %v", settings, err)
	}
}
