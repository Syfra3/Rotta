package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTUISelectedOpenCodeTelemetryHost(t *testing.T) {
	home, root, bin := t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	marker := filepath.Join(home, "called")
	t.Setenv("TELEMETRY_MARKER", marker)
	if err := os.WriteFile(filepath.Join(bin, "python3"), []byte("#!/bin/sh\nprintf '%s' \"$*\" > \"$TELEMETRY_MARKER\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"observability/install.py", "observability/compose.yaml", "observability/collector.yaml", "observability/prometheus.yaml", "observability/pi-telemetry.ts", "observability/fixture.py", "observability/opencode-telemetry.mjs", "observability/grafana/dashboards/samples.json", "assets/pi/rotta-extension.ts"} {
		p := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(root)
	m := New()
	m.Target, m.ProjectPath, m.SetupTelemetry = "opencode", home, true
	m.ModelRouting = "disabled"
	result := runInstall(m)().(installDoneMsg)
	if result.err != nil {
		t.Fatal(result.err)
	}
	data, err := os.ReadFile(marker)
	if err != nil || !strings.HasSuffix(string(data), "install --host opencode") {
		t.Fatalf("host handoff %q: %v", data, err)
	}
}

func TestTUITelemetryFailureDoesNotFailCore(t *testing.T) {
	home, root, bin := t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	marker := filepath.Join(home, "called")
	t.Setenv("TELEMETRY_MARKER", marker)
	if err := os.WriteFile(filepath.Join(bin, "python3"), []byte("#!/bin/sh\ntouch \"$TELEMETRY_MARKER\"\nexit 9\n"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"observability/install.py", "observability/compose.yaml", "observability/collector.yaml", "observability/prometheus.yaml", "observability/pi-telemetry.ts", "observability/fixture.py", "observability/opencode-telemetry.mjs", "observability/grafana/dashboards/samples.json", "assets/pi/rotta-extension.ts"} {
		p := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(root)
	m := New()
	m.Target, m.ProjectPath, m.SetupTelemetry = "copilot", home, true
	result := runInstall(m)().(installDoneMsg)
	if result.err != nil || result.result == nil {
		t.Fatalf("core failed: %v", result.err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("telemetry not attempted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".copilot", "agents", "rotta-orchestrator.agent.md")); err != nil {
		t.Fatalf("core missing: %v", err)
	}
	if !strings.Contains(strings.Join(result.result.Warnings, "\n"), "optional local telemetry setup failed; core installation succeeded") {
		t.Fatalf("warnings: %v", result.result.Warnings)
	}
}
