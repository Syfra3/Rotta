//go:build legacy_v2

package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// REQ-085, REQ-089 → SCN-628 → TestSCN628_InstallerTransactionEvidenceIsHostLocalBoundedAndNonAuthoritative
func TestSCN628_InstallerTransactionEvidenceIsHostLocalBoundedAndNonAuthoritative(t *testing.T) {
	// Scenario: Installer transaction evidence is host-local, bounded, and non-authoritative
	home := t.TempDir()
	stateHome := filepath.Join(home, "state")
	initiatingCheckout := filepath.Join(home, "initiating-checkout")
	featureWorktree := filepath.Join(home, "feature-worktree")
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", stateHome)

	result, err := Install(Options{Target: "opencode", ProjectPath: initiatingCheckout})
	if err != nil {
		t.Fatal(err)
	}

	transactionRoot := filepath.Join(stateHome, "rotta", "installer-transactions")
	if filepath.Dir(result.BackupDir) != transactionRoot {
		t.Fatalf("transaction path = %q, want direct child of host-local root %q", result.BackupDir, transactionRoot)
	}
	transactionID := filepath.Base(result.BackupDir)
	if transactionID == "." || transactionID == string(filepath.Separator) {
		t.Fatalf("transaction ID = %q, want a scoped host-local ID", transactionID)
	}

	manifestData, err := os.ReadFile(filepath.Join(result.BackupDir, "manifest.json"))
	if err != nil {
		t.Fatalf("read host-local transaction evidence: %v", err)
	}
	var manifest backupManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("decode host-local transaction evidence: %v", err)
	}
	if manifest.RetentionExpiresAt == "" {
		t.Fatal("transaction evidence must record its retention expiry")
	}
	expiresAt, err := time.Parse(time.RFC3339, manifest.RetentionExpiresAt)
	if err != nil {
		t.Fatalf("parse retention expiry: %v", err)
	}
	retention := expiresAt.Sub(time.Now().UTC())
	if retention < 29*24*time.Hour || retention > 30*24*time.Hour+time.Minute {
		t.Fatalf("retention = %s, want 30 days", retention)
	}
	transactionEvidence, err := os.ReadFile(filepath.Join(result.BackupDir, "transaction.json"))
	if err != nil {
		t.Fatalf("read redacted transaction status: %v", err)
	}
	if strings.Contains(string(transactionEvidence), initiatingCheckout) || strings.Contains(string(transactionEvidence), featureWorktree) {
		t.Fatalf("transaction status contains workflow path: %s", transactionEvidence)
	}
	var transaction struct {
		ConfigurationStatus string   `json:"configuration_status"`
		BackupReferences    []string `json:"backup_references"`
	}
	if err := json.Unmarshal(transactionEvidence, &transaction); err != nil {
		t.Fatalf("decode redacted transaction status: %v", err)
	}
	if transaction.ConfigurationStatus == "" {
		t.Fatal("transaction must record redacted configuration status")
	}
	if len(transaction.BackupReferences) == 0 {
		t.Fatal("transaction must record scoped backup references")
	}
	for _, reference := range transaction.BackupReferences {
		if filepath.IsAbs(reference) || strings.HasPrefix(reference, "..") {
			t.Fatalf("backup reference = %q, want a transaction-scoped relative reference", reference)
		}
	}

	for _, path := range []string{
		filepath.Join(initiatingCheckout, ".rotta", "current", "evidence"),
		filepath.Join(featureWorktree, ".rotta", "current", "evidence"),
		filepath.Join(featureWorktree, ".rotta", "archive"),
		filepath.Join(featureWorktree, "specs", "approvals"),
		filepath.Join(featureWorktree, ".rotta", "current", "state.yaml"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("installer transaction evidence leaked into workflow authority path %q: %v", path, err)
		}
	}

	if err := CleanupInstallerTransaction(stateHome, transactionID, time.Now().UTC()); err != nil {
		t.Fatalf("explicit cleanup for transaction ID: %v", err)
	}
	if _, err := os.Stat(result.BackupDir); !os.IsNotExist(err) {
		t.Fatalf("transaction directory remains after explicit cleanup: %v", err)
	}
	if err := CleanupInstallerTransaction(stateHome, "../other-transaction", time.Now().UTC()); err == nil {
		t.Fatal("cleanup accepted a path rather than one transaction ID")
	}
}
