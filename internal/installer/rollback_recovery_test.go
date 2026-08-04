package installer

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// REQ-090 → SCN-627 → TestSCN627_SafeRollbackPreservesContractsAndNeverRevivesLegacyAuthority
func TestSCN627_SafeRollbackPreservesContractsAndNeverRevivesLegacyAuthority(t *testing.T) {
	// Scenario: Rollback preserves contracts and never revives legacy authority
	t.Run("workflow recovery preserves feature artifacts and offers safe alternatives", func(t *testing.T) {
		featureWorktree := t.TempDir()
		initiatingCheckout := t.TempDir()
		installerEvidence := filepath.Join(t.TempDir(), "transaction", "vela-rollback.json")
		artifacts := map[string][]byte{
			"specs/workflow_ergonomics_hard_spec.md":           []byte("approved hard spec\n"),
			"features/workflow_ergonomics.feature":             []byte("approved feature\n"),
			"specs/approvals/workflow-ergonomics.yaml":         []byte("approved record\n"),
			".rotta/current/evidence/failing-command.json":     []byte("current evidence\n"),
			".rotta/archive/workflow-ergonomics/base/evidence": []byte("archived evidence\n"),
		}
		for path, contents := range artifacts {
			writeTestFile(t, filepath.Join(featureWorktree, path), contents)
		}
		writeTestFile(t, filepath.Join(featureWorktree, ".rotta", "state-machine.yaml"), []byte("retired\n"))
		writeTestFile(t, installerEvidence, []byte("host-local transaction evidence\n"))

		result, err := RecoverAvailableSafeRollback(SafeRollbackRequest{
			FeatureWorktree:       featureWorktree,
			InitiatingCheckout:    initiatingCheckout,
			PreservePaths:         workflowArtifactPaths(featureWorktree),
			InstallerEvidencePath: installerEvidence,
		})
		if err != nil {
			t.Fatal(err)
		}
		for path, want := range artifacts {
			if got := mustReadFile(t, filepath.Join(featureWorktree, path)); !bytes.Equal(got, want) {
				t.Fatalf("preserved artifact %s = %q, want %q", path, got, want)
			}
		}
		if got := result.InstallerEvidencePath; got != installerEvidence {
			t.Fatalf("installer evidence path = %q, want retained host-local path %q", got, installerEvidence)
		}
		if _, err := os.Stat(filepath.Join(initiatingCheckout, "vela-rollback.json")); !os.IsNotExist(err) {
			t.Fatalf("installer evidence was copied into initiating checkout: %v", err)
		}
		for _, action := range []RollbackRecoveryAction{RollbackRecoveryHandoff, RollbackRecoveryArchive, RollbackRecoveryRepair, RollbackRecoveryRestart} {
			if !containsRollbackRecoveryAction(result.OfferedActions, action) {
				t.Fatalf("offered actions = %v, missing %q", result.OfferedActions, action)
			}
		}
		if containsRollbackRecoveryAction(result.OfferedActions, RollbackRecoveryAction("legacy-authority")) {
			t.Fatalf("offered actions = %v, must not reactivate legacy authority", result.OfferedActions)
		}
	})

	t.Run("selected host restores only its scoped transaction", func(t *testing.T) {
		home := t.TempDir()
		backupDir := filepath.Join(home, ".local", "state", "rotta", "installer-transactions", "transaction")
		configPath := filepath.Join(home, ".config", "opencode", "opencode.json")
		baseline := []byte(`{"mcp":{"ancora":{"command":["ancora","mcp"],"enabled":true},"context7":{"command":["npx","context7"],"enabled":true}},"theme":"dark"}`)
		writeTestFile(t, configPath, baseline)
		transaction, err := beginOpenCodeVelaTransaction(backupDir, configPath)
		if err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, configPath, openCodeConfigWithVela(t, baseline, false))

		result, err := RecoverAvailableSafeRollback(SafeRollbackRequest{
			SelectedHost:            "opencode",
			SelectedHostTransaction: transaction,
			RollbackCause:           errors.New("Vela configuration failed"),
		})
		if err != nil {
			t.Fatal(err)
		}
		if result.SelectedHostRollback == nil || !result.SelectedHostRollback.Restored || result.SelectedHostRollback.ConcurrentModification {
			t.Fatalf("selected-host rollback = %#v, want safe scoped restore", result.SelectedHostRollback)
		}
		if got := mustReadFile(t, configPath); !bytes.Equal(got, baseline) {
			t.Fatalf("OpenCode config = %s, want only Vela transaction restored to %s", got, baseline)
		}
		if _, err := os.Stat(filepath.Join(backupDir, "vela-rollback.json")); err != nil {
			t.Fatalf("scoped rollback evidence missing from host-local transaction: %v", err)
		}
	})
}

func workflowArtifactPaths(featureWorktree string) []string {
	return []string{
		filepath.Join(featureWorktree, "specs", "workflow_ergonomics_hard_spec.md"),
		filepath.Join(featureWorktree, "features", "workflow_ergonomics.feature"),
		filepath.Join(featureWorktree, "specs", "approvals", "workflow-ergonomics.yaml"),
		filepath.Join(featureWorktree, ".rotta", "current", "evidence", "failing-command.json"),
		filepath.Join(featureWorktree, ".rotta", "archive", "workflow-ergonomics", "base", "evidence"),
	}
}

func containsRollbackRecoveryAction(actions []RollbackRecoveryAction, want RollbackRecoveryAction) bool {
	for _, action := range actions {
		if action == want {
			return true
		}
	}
	return false
}
