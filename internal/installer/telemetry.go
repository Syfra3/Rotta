package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// InstallLocalTelemetry invokes the separately managed, local-only telemetry installer.
// It is never called unless the user explicitly selected and confirmed telemetry.
func InstallLocalTelemetry(target ...string) error {
	hosts := []string{"pi"} // historical default for callers without a target
	if len(target) > 0 {
		switch target[0] {
		case "pi":
			hosts = []string{"pi"}
		case "opencode", "both":
			hosts = []string{"opencode"}
		case "all":
			hosts = []string{"pi", "opencode"}
		default:
			hosts = []string{target[0]} // installer rejects unsupported hosts without touching Pi
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	roots := []string{
		filepath.Join(filepath.Dir(exe), "rotta-assets"),       // standalone release
		filepath.Join(filepath.Dir(exe), "..", "rotta-assets"), // Homebrew keg: prefix/bin/rotta
		filepath.Dir(exe),
	}
	// A checkout build may be launched from the repository root. Never search
	// arbitrary parent directories or run an unrelated working directory script.
	if wd, err := os.Getwd(); err == nil {
		roots = append(roots, wd)
	}
	for _, root := range roots {
		script := filepath.Join(root, "observability", "install.py")
		if info, err := os.Stat(script); err != nil || !info.Mode().IsRegular() {
			continue
		}
		for _, asset := range []string{"compose.yaml", "collector.yaml", "prometheus.yaml", "pi-telemetry.ts", "fixture.py", "opencode-telemetry.mjs", "grafana/dashboards/samples.json"} {
			if info, err := os.Stat(filepath.Join(root, "observability", asset)); err != nil || !info.Mode().IsRegular() {
				return fmt.Errorf("telemetry asset %s missing: %w", asset, err)
			}
		}
		if info, err := os.Stat(filepath.Join(root, "assets/pi/rotta-extension.ts")); err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("telemetry Pi asset missing in %s", root)
		}
		for _, host := range hosts {
			cmd := exec.Command("python3", script, "install", "--host", host)
			cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("local telemetry setup failed for %s: %w", host, err)
			}
		}
		return nil
	}
	return fmt.Errorf("local telemetry assets unavailable beside executable or in checkout; Rotta core installation is unaffected")
}
