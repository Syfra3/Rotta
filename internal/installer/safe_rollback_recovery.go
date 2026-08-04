package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RollbackRecoveryAction is a non-destructive action offered when a workflow
// cannot safely resume from its current authority boundary.
type RollbackRecoveryAction string

const (
	RollbackRecoveryHandoff RollbackRecoveryAction = "handoff"
	RollbackRecoveryArchive RollbackRecoveryAction = "archive"
	RollbackRecoveryRepair  RollbackRecoveryAction = "repair"
	RollbackRecoveryRestart RollbackRecoveryAction = "restart"
)

// SafeRollbackRequest identifies the narrow recovery boundary that may be
// acted on. Feature artifacts are inspected only; selected-host recovery is
// delegated to its scoped transaction.
type SafeRollbackRequest struct {
	FeatureWorktree         string
	InitiatingCheckout      string
	PreservePaths           []string
	InstallerEvidencePath   string
	SelectedHost            string
	SelectedHostTransaction *openCodeVelaTransaction
	RollbackCause           error
}

// SafeRollbackResult describes preserved paths and the scoped action, without
// promoting any legacy workflow artifact to authority.
type SafeRollbackResult struct {
	PreservedPaths        []string
	InstallerEvidencePath string
	OfferedActions        []RollbackRecoveryAction
	SelectedHostRollback  *openCodeVelaRollbackResult
}

// RecoverAvailableSafeRollback preserves the verified feature artifacts in
// place and restores only an OpenCode Vela scoped transaction when requested.
func RecoverAvailableSafeRollback(request SafeRollbackRequest) (SafeRollbackResult, error) {
	result := SafeRollbackResult{
		OfferedActions: []RollbackRecoveryAction{
			RollbackRecoveryHandoff,
			RollbackRecoveryArchive,
			RollbackRecoveryRepair,
			RollbackRecoveryRestart,
		},
	}
	if err := preserveWorkflowRollbackArtifacts(request, &result); err != nil {
		return SafeRollbackResult{}, err
	}
	if request.SelectedHostTransaction == nil {
		return result, nil
	}
	if request.SelectedHost != "opencode" || request.RollbackCause == nil {
		return SafeRollbackResult{}, fmt.Errorf("safe rollback requires a selected OpenCode transaction and failure cause")
	}
	rollback, err := request.SelectedHostTransaction.recover(request.RollbackCause)
	result.SelectedHostRollback = &rollback
	if err != nil {
		return result, err
	}
	return result, nil
}

func preserveWorkflowRollbackArtifacts(request SafeRollbackRequest, result *SafeRollbackResult) error {
	if len(request.PreservePaths) > 0 {
		if request.FeatureWorktree == "" {
			return fmt.Errorf("safe workflow rollback requires its recorded feature worktree")
		}
		for _, path := range request.PreservePaths {
			if !isWithinPath(request.FeatureWorktree, path) {
				return fmt.Errorf("safe workflow rollback refuses artifact outside recorded feature worktree: %s", path)
			}
			if _, err := os.Stat(path); err != nil {
				return fmt.Errorf("safe workflow rollback preserves unavailable artifact %s: %w", path, err)
			}
			result.PreservedPaths = append(result.PreservedPaths, path)
		}
	}
	if request.InstallerEvidencePath == "" {
		return nil
	}
	if request.InitiatingCheckout != "" && isWithinPath(request.InitiatingCheckout, request.InstallerEvidencePath) {
		return fmt.Errorf("installer evidence must remain host-local, not in the initiating checkout")
	}
	if _, err := os.Stat(request.InstallerEvidencePath); err != nil {
		return fmt.Errorf("preserve host-local installer evidence: %w", err)
	}
	result.InstallerEvidencePath = request.InstallerEvidencePath
	return nil
}

func isWithinPath(root, path string) bool {
	root, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) && rel != ""
}
