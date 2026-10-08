package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIInstallsCopilotTargetWithoutOpenCodeRouting(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	var output, errors bytes.Buffer
	if err := runCLI([]string{"install", "--target", "copilot", "--project", home}, &output, &errors); err != nil {
		t.Fatalf("Copilot CLI install: %v (%s)", err, errors.String())
	}
	if !strings.Contains(output.String(), "Installed rotta for copilot") {
		t.Fatalf("missing CLI outcome: %s", output.String())
	}
	if _, err := os.Stat(filepath.Join(home, ".copilot", "agents", "rotta-orchestrator.agent.md")); err != nil {
		t.Fatalf("missing Copilot agent: %v", err)
	}
}
