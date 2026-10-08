package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Syfra3/Rotta/internal/installer"
)

func TestInstallOpenCodeCustomModelEffortFlags(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(home, "xdg")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	args := []string{"install", "--target", "opencode", "--model-routing", "custom", "--confirm-model-routing"}
	for role, model := range installer.DefaultOpenCodeRouting() {
		args = append(args, "--model", role+"="+model)
	}
	for role, effort := range installer.DefaultOpenCodeRoutingEfforts() {
		if role == "rotta-impl" {
			effort = "high"
		}
		args = append(args, "--model-effort", role+"="+effort)
	}
	if err := runCLI(args, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(xdg, "opencode", "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Agent map[string]struct {
			Variant string `json:"variant"`
		} `json:"agent"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	if config.Agent["rotta-impl"].Variant != "high" {
		t.Fatalf("CLI selected variant = %q", config.Agent["rotta-impl"].Variant)
	}
}

func TestInstallOpenCodeRejectsEffortWithoutCustomRouting(t *testing.T) {
	err := runCLI([]string{"install", "--target", "opencode", "--confirm-model-routing", "--model-effort", "rotta-impl=high"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("effort without custom selection accepted")
	}
}
