package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestREQ005A_CLIRejectsVelaFlagBeforeInstallation(t *testing.T) {
	err := runCLI([]string{"install", "--vela", "--target", "opencode"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "TUI-only") {
		t.Fatalf("error = %v, want TUI-only guard", err)
	}
}
