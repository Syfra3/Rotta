package installer

import (
	"os"
	"path/filepath"
	"testing"
)

// REQ-100 → SCN-402 → TestSCN402_InstallsOnlyCopilotCLI
func TestSCN402_InstallsOnlyCopilotCLI(t *testing.T) {
	// Scenario: Select GitHub Copilot CLI as the only installation target
	home := t.TempDir()
	projectPath := filepath.Join(home, "project")
	t.Setenv("HOME", home)

	unchanged := map[string]string{
		filepath.Join(home, ".claude", "settings.json"):             "claude settings",
		filepath.Join(home, ".config", "opencode", "opencode.json"): "opencode settings",
		filepath.Join(home, ".codex", "config.toml"):                "codex settings",
	}
	for path, content := range unchanged {
		writeTestFile(t, path, []byte(content))
	}

	result, err := Install(Options{Target: "copilot-cli", ProjectPath: projectPath})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Hosts) != 1 || result.Hosts["copilot-cli"].Status != HostInstallStatusInstalled {
		t.Fatalf("expected only Copilot CLI to be installed and reported, got %#v", result.Hosts)
	}
	for path, want := range unchanged {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read unchanged host configuration %s: %v", path, err)
		}
		if string(got) != want {
			t.Fatalf("expected %s to remain unchanged, got %q", path, got)
		}
	}
}
