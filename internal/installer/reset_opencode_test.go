package installer

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// REQ-001 → SCN-001 → TestSCN001_ResetTargetIsAdvertisedAndRunnable
func TestSCN001_ResetTargetIsAdvertisedAndRunnable(t *testing.T) {
	// The reset target is intentionally only inspected, never executed by tests.
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
