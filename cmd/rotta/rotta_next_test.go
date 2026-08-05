package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestRottaNextCLIInstallsWithoutRetiredModeFlags(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("OPENCODE_CONFIG", "")
	var stdout, stderr bytes.Buffer
	if err := runCLI([]string{"install", "--target", "opencode", "--project", filepath.Join(home, "project")}, &stdout, &stderr); err != nil {
		t.Fatalf("install Rotta Next: %v", err)
	}
	if !strings.Contains(stdout.String(), "Installed rotta for opencode") {
		t.Fatalf("unexpected install output: %q", stdout.String())
	}
	if err := runCLI([]string{"install", "--spec"}, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Fatal("retired mode flag was accepted")
	}
}
