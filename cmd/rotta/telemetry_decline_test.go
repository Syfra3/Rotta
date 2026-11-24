package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIAbsentAndDeclinedTelemetryKeepCoreInstall(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags []string
	}{{"absent", nil}, {"declined", []string{"--local-telemetry=false"}}} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_STATE_HOME", "")
			t.Setenv("XDG_CONFIG_HOME", "")
			bin := t.TempDir()
			marker := filepath.Join(home, "python-called")
			if err := os.WriteFile(filepath.Join(bin, "python3"), []byte("#!/bin/sh\ntouch \"$TELEMETRY_MARKER\"\nexit 99\n"), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
			t.Setenv("TELEMETRY_MARKER", marker)
			args := append([]string{"install", "--target", "copilot", "--project", home}, tc.flags...)
			var out, errs bytes.Buffer
			if err := runCLI(args, &out, &errs); err != nil {
				t.Fatalf("core install: %v %s", err, errs.String())
			}
			if !strings.Contains(out.String(), "Installed rotta for copilot") {
				t.Fatalf("core result: %s", out.String())
			}
			if _, err := os.Stat(filepath.Join(home, ".copilot", "agents", "rotta-orchestrator.agent.md")); err != nil {
				t.Fatalf("core asset: %v", err)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("telemetry invoked: %v", err)
			}
			if _, err := os.Stat(filepath.Join(home, ".pi", "agent", ".rotta-telemetry-owned")); !os.IsNotExist(err) {
				t.Fatalf("telemetry receipt: %v", err)
			}
		})
	}
}
