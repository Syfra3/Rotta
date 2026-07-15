package installer

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// REQ-001 → SCN-001 → TestSCN001_ResetTargetIsAdvertisedAndRunnable
func TestSCN001_ResetTargetIsAdvertisedAndRunnable(t *testing.T) {
	// Scenario: The reset target is advertised and runnable
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	help := exec.Command("make", "help")
	help.Dir = repoRoot
	helpOutput, err := help.CombinedOutput()
	if err != nil {
		t.Fatalf("make help: %v\n%s", err, helpOutput)
	}
	helpText := string(helpOutput)
	if !strings.Contains(helpText, "reset-opencode") || !strings.Contains(helpText, "removes global OpenCode state before reinstalling OpenCode") {
		t.Fatalf("expected help to advertise reset-opencode with its global-state warning, got:\n%s", helpOutput)
	}

	dryRun := exec.Command("make", "-n", "reset-opencode")
	dryRun.Dir = repoRoot
	dryRunOutput, err := dryRun.CombinedOutput()
	if err != nil {
		t.Fatalf("make -n reset-opencode: %v\n%s", err, dryRunOutput)
	}
	if !strings.Contains(string(dryRunOutput), "Starting global OpenCode reset-and-reinstall workflow") {
		t.Fatalf("expected reset-opencode to start its workflow, got:\n%s", dryRunOutput)
	}
}

// REQ-002 → SCN-002 → TestSCN002_ResetDefaultGlobalLocationsAndReinstallWithoutConfirmation
func TestSCN002_ResetDefaultGlobalLocationsAndReinstallWithoutConfirmation(t *testing.T) {
	// Scenario: Reset default global OpenCode locations and reinstall without confirmation
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	home := t.TempDir()
	for _, path := range []string{
		filepath.Join(home, ".config", "opencode"),
		filepath.Join(home, ".local", "share", "opencode"),
		filepath.Join(home, ".cache", "opencode"),
	} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatalf("create OpenCode directory %q: %v", path, err)
		}
		if err := os.WriteFile(filepath.Join(path, "state"), []byte("OpenCode state"), 0o600); err != nil {
			t.Fatalf("write OpenCode state in %q: %v", path, err)
		}
	}

	unrelated := filepath.Join(home, ".config", "other-app")
	if err := os.MkdirAll(unrelated, 0o755); err != nil {
		t.Fatalf("create unrelated directory: %v", err)
	}

	binDir := t.TempDir()
	curl := filepath.Join(binDir, "curl")
	if err := os.WriteFile(curl, []byte("#!/bin/sh\n[ \"$#\" -eq 2 ] && [ \"$1\" = \"-fsSL\" ] && [ \"$2\" = \"https://opencode.ai/install\" ] || exit 1\nprintf '%s\\n' 'touch \"$INSTALLER_MARKER\"'\n"), 0o755); err != nil {
		t.Fatalf("write fake curl: %v", err)
	}

	installerMarker := filepath.Join(t.TempDir(), "installer-ran")
	command := exec.Command("make", "reset-opencode")
	command.Dir = repoRoot
	command.Env = append(withoutXDG(os.Environ()), "HOME="+home, "PATH="+binDir+":"+os.Getenv("PATH"), "INSTALLER_MARKER="+installerMarker)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("make reset-opencode: %v\n%s", err, output)
	}

	for _, path := range []string{
		filepath.Join(home, ".config", "opencode"),
		filepath.Join(home, ".local", "share", "opencode"),
		filepath.Join(home, ".cache", "opencode"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("expected default OpenCode path %q to be removed, stat error: %v", path, err)
		}
	}
	if _, err := os.Stat(unrelated); err != nil {
		t.Errorf("expected unrelated path to remain: %v", err)
	}
	if _, err := os.Stat(installerMarker); err != nil {
		t.Errorf("expected official installer to run without confirmation: %v", err)
	}
}

// REQ-003 → SCN-004 → TestSCN004_ResetCustomXDGOpenCodeLocationsWithoutRemovingRoots
func TestSCN004_ResetCustomXDGOpenCodeLocationsWithoutRemovingRoots(t *testing.T) {
	// Scenario: Reset custom XDG OpenCode locations without removing their roots
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	configRoot := t.TempDir()
	dataRoot := t.TempDir()
	cacheRoot := t.TempDir()
	for _, root := range []string{configRoot, dataRoot, cacheRoot} {
		opencodePath := filepath.Join(root, "opencode")
		if err := os.MkdirAll(opencodePath, 0o755); err != nil {
			t.Fatalf("create OpenCode directory %q: %v", opencodePath, err)
		}
		if err := os.WriteFile(filepath.Join(opencodePath, "state"), []byte("OpenCode state"), 0o600); err != nil {
			t.Fatalf("write OpenCode state in %q: %v", opencodePath, err)
		}
		if err := os.MkdirAll(filepath.Join(root, "other-app"), 0o755); err != nil {
			t.Fatalf("create unrelated directory in %q: %v", root, err)
		}
	}

	binDir := t.TempDir()
	curl := filepath.Join(binDir, "curl")
	if err := os.WriteFile(curl, []byte("#!/bin/sh\n[ \"$#\" -eq 2 ] && [ \"$1\" = \"-fsSL\" ] && [ \"$2\" = \"https://opencode.ai/install\" ] || exit 1\nprintf '%s\\n' 'touch \"$INSTALLER_MARKER\"'\n"), 0o755); err != nil {
		t.Fatalf("write fake curl: %v", err)
	}

	installerMarker := filepath.Join(t.TempDir(), "installer-ran")
	command := exec.Command("make", "reset-opencode")
	command.Dir = repoRoot
	command.Env = append(os.Environ(), "XDG_CONFIG_HOME="+configRoot, "XDG_DATA_HOME="+dataRoot, "XDG_CACHE_HOME="+cacheRoot, "PATH="+binDir+":"+os.Getenv("PATH"), "INSTALLER_MARKER="+installerMarker)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("make reset-opencode: %v\n%s", err, output)
	}

	for _, root := range []string{configRoot, dataRoot, cacheRoot} {
		if _, err := os.Stat(filepath.Join(root, "opencode")); !os.IsNotExist(err) {
			t.Errorf("expected custom OpenCode path in %q to be removed, stat error: %v", root, err)
		}
		if _, err := os.Stat(filepath.Join(root, "other-app")); err != nil {
			t.Errorf("expected unrelated path in %q to remain: %v", root, err)
		}
	}
	if _, err := os.Stat(installerMarker); err != nil {
		t.Errorf("expected official installer to run: %v", err)
	}
}

func withoutXDG(environment []string) []string {
	filtered := make([]string, 0, len(environment))
	for _, entry := range environment {
		if !strings.HasPrefix(entry, "XDG_CONFIG_HOME=") && !strings.HasPrefix(entry, "XDG_DATA_HOME=") && !strings.HasPrefix(entry, "XDG_CACHE_HOME=") {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}
