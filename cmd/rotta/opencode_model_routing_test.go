package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Syfra3/Rotta/internal/installer"
)

// REQ-002 → SCN-002 → TestSCN002_CLIExplicitDisableRemovesOnlyOwnedModels
func TestSCN002_CLIExplicitDisableRemovesOnlyOwnedModels(t *testing.T) {
	// Scenario: CLI explicitly disables routing.
	home := t.TempDir()
	xdg := filepath.Join(home, "xdg")
	project := filepath.Join(home, "project")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	if _, err := installer.Install(installer.Options{Target: "opencode", ProjectPath: project}); err != nil {
		t.Fatalf("initial enabled install: %v", err)
	}
	if err := runCLI([]string{"install", "--target", "opencode", "--project", project, "--model-routing", "disabled", "--confirm-model-routing"}, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("explicit disabled routing: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(xdg, "opencode", "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"rotta-orchestrator", "rotta-architect", "rotta-review", "rotta-impl", "rotta-ops", "rotta-explore", "rotta-cleaner"} {
		agent := config["agent"].(map[string]interface{})[role].(map[string]interface{})
		if _, exists := agent["model"]; exists {
			t.Fatalf("%s model remains after explicit disable", role)
		}
		if _, exists := agent["permission"]; !exists {
			t.Fatalf("%s agent fields were removed with its model", role)
		}
	}
}

func TestCLIRoutingRequiresConfirmationBeforeMutation(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(home, "xdg")
	state := filepath.Join(home, "state")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("XDG_STATE_HOME", state)

	var stderr bytes.Buffer
	err := runCLI([]string{"install", "--target", "opencode", "--model-routing", "enabled"}, &bytes.Buffer{}, &stderr)
	if err == nil || err.Error() != "OpenCode model routing requires --confirm-model-routing" {
		t.Fatalf("missing confirmation error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(xdg, "opencode", "opencode.json")); !os.IsNotExist(err) {
		t.Fatalf("unconfirmed command wrote OpenCode config: %v", err)
	}
	if _, err := os.Stat(filepath.Join(state, "rotta", "installer-transactions")); !os.IsNotExist(err) {
		t.Fatalf("unconfirmed command created a backup: %v", err)
	}

	stderr.Reset()
	err = runCLI([]string{"install", "--help"}, &bytes.Buffer{}, &stderr)
	if err == nil || !strings.Contains(stderr.String(), "-confirm-model-routing") || !strings.Contains(stderr.String(), "confirm OpenCode model-routing changes") {
		t.Fatalf("confirmation help = %q, %v", stderr.String(), err)
	}
}
