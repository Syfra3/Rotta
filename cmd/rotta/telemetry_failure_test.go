package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLISelectedOpenCodeTelemetryHost(t *testing.T) {
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
	var out, stderr bytes.Buffer
	if err := runCLI([]string{"install", "--target", "opencode", "--project", home, "--model-routing", "disabled", "--confirm-model-routing", "--local-telemetry"}, &out, &stderr); err != nil {
		t.Fatalf("%v: %s", err, stderr.String())
	}
	data, err := os.ReadFile(marker)
	if err != nil || !strings.HasSuffix(string(data), "install --host opencode") {
		t.Fatal(fmt.Sprintf("host handoff %q: %v", data, err))
	}
}

func TestCLITelemetryFailureDoesNotFailCore(t *testing.T) {
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
	var out, stderr bytes.Buffer
	if err := runCLI([]string{"install", "--target", "copilot", "--project", home, "--local-telemetry"}, &out, &stderr); err != nil {
		t.Fatalf("core failed: %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("telemetry not attempted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".copilot", "agents", "rotta-orchestrator.agent.md")); err != nil {
		t.Fatalf("core missing: %v", err)
	}
	if !strings.Contains(stderr.String(), "optional local telemetry setup failed; core installation succeeded") || strings.Contains(out.String(), "Local telemetry installed.") {
		t.Fatalf("out=%q stderr=%q", out.String(), stderr.String())
	}
}
