package installer

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The subprocess has its own executable location so asset discovery is tested,
// rather than replaced with a mock of InstallLocalTelemetry.
func TestTelemetryHandoffProcess(t *testing.T) {
	if os.Getenv("ROTTA_TELEMETRY_HELPER") == "1" {
		target := os.Getenv("ROTTA_TELEMETRY_TARGET")
		var err error
		if target == "" {
			err = InstallLocalTelemetry()
		} else {
			err = InstallLocalTelemetry(target)
		}
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	for _, layout := range []string{"checkout", "release", "homebrew"} {
		t.Run(layout, func(t *testing.T) {
			root := t.TempDir()
			home := filepath.Join(root, "home")
			bin := filepath.Join(root, "bin")
			for _, dir := range []string{home, bin} {
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
			}
			source, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			executable := filepath.Join(bin, "rotta-test")
			if err := os.WriteFile(executable, data, 0700); err != nil {
				t.Fatal(err)
			}
			assetRoot := root
			if layout == "release" {
				assetRoot = filepath.Join(bin, "rotta-assets")
			} else if layout == "homebrew" {
				assetRoot = filepath.Join(root, "rotta-assets")
			}
			for _, name := range []string{"observability/install.py", "observability/compose.yaml", "observability/collector.yaml", "observability/prometheus.yaml", "observability/pi-telemetry.ts", "observability/fixture.py", "observability/opencode-telemetry.mjs", "observability/grafana/dashboards/samples.json", "assets/pi/rotta-extension.ts"} {
				path := filepath.Join(assetRoot, name)
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			calls := filepath.Join(root, "calls")
			python := filepath.Join(bin, "python3")
			if err := os.WriteFile(python, []byte("#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$CALL_LOG\"\n"), 0700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(executable, "-test.run=^TestTelemetryHandoffProcess$")
			cmd.Dir = home
			if layout == "checkout" {
				cmd.Dir = root
			}
			cmd.Env = append(os.Environ(), "ROTTA_TELEMETRY_HELPER=1", "HOME="+home, "PATH="+bin+":"+os.Getenv("PATH"), "CALL_LOG="+calls)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("handoff: %v: %s", err, out)
			}
			got, err := os.ReadFile(calls)
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(string(got)) != filepath.Join(assetRoot, "observability/install.py")+" install --host pi" {
				t.Fatalf("unexpected handoff: %s", got)
			}
			for _, target := range []string{"opencode", "both", "all"} {
				if err := os.WriteFile(calls, nil, 0600); err != nil {
					t.Fatal(err)
				}
				check := exec.Command(executable, "-test.run=^TestTelemetryHandoffProcess$")
				check.Dir = cmd.Dir
				check.Env = append(cmd.Env, "ROTTA_TELEMETRY_TARGET="+target)
				if out, err := check.CombinedOutput(); err != nil {
					t.Fatalf("%s: %v %s", target, err, out)
				}
				got, err := os.ReadFile(calls)
				if err != nil {
					t.Fatal(err)
				}
				want := filepath.Join(assetRoot, "observability/install.py") + " install --host opencode\n"
				if target == "all" {
					want = filepath.Join(assetRoot, "observability/install.py") + " install --host pi\n" + want
				}
				if string(got) != want {
					t.Fatalf("%s: got %q want %q", target, got, want)
				}
			}
			if entries, _ := os.ReadDir(home); len(entries) != 0 {
				t.Fatal("fake home changed")
			}
			if err := os.Remove(filepath.Join(assetRoot, "observability/collector.yaml")); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(calls); err != nil {
				t.Fatal(err)
			}
			cmd = exec.Command(executable, "-test.run=^TestTelemetryHandoffProcess$")
			cmd.Dir = home
			if layout == "checkout" {
				cmd.Dir = root
			}
			cmd.Env = append(os.Environ(), "ROTTA_TELEMETRY_HELPER=1", "HOME="+home, "PATH="+bin+":"+os.Getenv("PATH"), "CALL_LOG="+calls)
			out, err = cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(out), "telemetry asset collector.yaml missing") {
				t.Fatalf("missing asset: %v %s", err, out)
			}
			if _, err := os.Stat(calls); !os.IsNotExist(err) {
				t.Fatal("python invoked with missing asset")
			}
		})
	}
}
