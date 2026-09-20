package main

import (
	"bytes"
	"os"
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
	if err := runCLI([]string{"install", "--target", "opencode", "--project", filepath.Join(home, "project"), "--confirm-model-routing"}, &stdout, &stderr); err != nil {
		t.Fatalf("install Rotta Next: %v", err)
	}
	if !strings.Contains(stdout.String(), "Installed rotta for opencode") {
		t.Fatalf("unexpected install output: %q", stdout.String())
	}
	for _, want := range []string{"OpenCode policy loading: exact", "have not been verified", "Restart"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("missing source-loading diagnostic %q: %s", want, stdout.String())
		}
	}
	if err := runCLI([]string{"install", "--spec"}, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Fatal("retired mode flag was accepted")
	}
}

func TestCLIReportsPreservedCustomLoaderOnNoOpInstall(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("OPENCODE_CONFIG", "")
	args := []string{"install", "--target", "opencode", "--confirm-model-routing"}
	if err := runCLI(args, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".config", "opencode", "opencode.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Change only the loader's text without invalidating JSON or managed model fields.
	data = bytes.ReplaceAll(data, []byte("Do not use the name-based skill tool"), []byte("Custom loader instruction"))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	if err := runCLI(args, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"OpenCode policy loading: degraded", "custom or unrecognized loader preserved", "Preserved user-owned configuration", "Restart"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("custom-loader issue hidden: missing %q in %s", want, stdout.String())
		}
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, after) {
		t.Fatalf("no-op installation changed custom configuration: %v", err)
	}
}

func TestCLIInstallsGlobalPiExtensionWithoutOpenCodeConfirmation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	var stdout bytes.Buffer
	if err := runCLI([]string{"install", "--target", "pi", "--project", filepath.Join(home, "project")}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".pi", "agent", "extensions", "rotta.ts")); err != nil {
		t.Fatalf("managed Pi extension was not installed: %v", err)
	}
}
