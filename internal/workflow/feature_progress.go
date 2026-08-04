package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FeatureProgressRecord is mutable execution progress bound to one feature
// worktree's current manifest.
type FeatureProgressRecord struct {
	FeatureID       string
	WorktreePath    string
	CheckpointState string
	Evidence        string
}

type featureRuntimeManifest struct {
	FeatureID string
	Worktree  string
	Branch    string
}

// RecordFeatureProgress writes progress and evidence only after binding the
// supplied record to the current manifest in the recorded feature worktree.
func RecordFeatureProgress(worktreePath string, progress FeatureProgressRecord) error {
	manifest, actualWorktree, err := loadFeatureRuntimeManifest(worktreePath)
	if err != nil {
		return err
	}
	if err := validateFeatureProgress(manifest, actualWorktree, progress); err != nil {
		return err
	}

	currentDirectory := filepath.Join(actualWorktree, ".rotta", "current")
	state := fmt.Sprintf("format: rotta.feature-progress/v1\nfeature_id: %s\nworktree: %s\ncheckpoint_state: %s\nlast_action: recorded feature progress\n",
		progress.FeatureID, actualWorktree, progress.CheckpointState)
	if err := os.WriteFile(filepath.Join(currentDirectory, "state.yaml"), []byte(state), 0o600); err != nil {
		return fmt.Errorf("record feature-local progress state: %w", err)
	}

	evidence, err := os.OpenFile(filepath.Join(currentDirectory, "tdd-log.md"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("record feature-local progress evidence: %w", err)
	}
	if _, err := fmt.Fprintf(evidence, "## %s\n- checkpoint state: %s\n- evidence: %s\n", progress.FeatureID, progress.CheckpointState, progress.Evidence); err != nil {
		_ = evidence.Close()
		return fmt.Errorf("write feature-local progress evidence: %w", err)
	}
	if err := evidence.Close(); err != nil {
		return fmt.Errorf("close feature-local progress evidence: %w", err)
	}
	return nil
}

func loadFeatureRuntimeManifest(worktreePath string) (featureRuntimeManifest, string, error) {
	actualWorktree, err := gitSubmissionOutput(worktreePath, "rev-parse", "--show-toplevel")
	if err != nil {
		return featureRuntimeManifest{}, "", fmt.Errorf("resolve feature runtime worktree: %w", err)
	}
	actualWorktree, err = filepath.EvalSymlinks(actualWorktree)
	if err != nil {
		return featureRuntimeManifest{}, "", fmt.Errorf("resolve feature runtime worktree: %w", err)
	}
	contents, err := readRepositoryFile(actualWorktree, ".rotta/current/manifest.yaml")
	if err != nil {
		return featureRuntimeManifest{}, "", fmt.Errorf("read feature runtime manifest: %w", err)
	}
	manifest, err := parseFeatureRuntimeManifest(string(contents))
	if err != nil {
		return featureRuntimeManifest{}, "", fmt.Errorf("read feature runtime manifest: %w", err)
	}
	return manifest, actualWorktree, nil
}

func validateFeatureProgress(manifest featureRuntimeManifest, actualWorktree string, progress FeatureProgressRecord) error {
	manifestWorktree, err := filepath.EvalSymlinks(manifest.Worktree)
	if err != nil {
		return fmt.Errorf("feature runtime identity does not match current manifest")
	}
	requestedWorktree, err := filepath.EvalSymlinks(progress.WorktreePath)
	if err != nil || manifest.FeatureID != progress.FeatureID || manifestWorktree != actualWorktree || requestedWorktree != actualWorktree || progress.CheckpointState == "" || progress.Evidence == "" {
		return fmt.Errorf("feature runtime identity does not match current manifest")
	}
	branch, err := gitSubmissionOutput(actualWorktree, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil || branch != manifest.Branch {
		return fmt.Errorf("feature runtime identity does not match current manifest")
	}
	return nil
}

func parseFeatureRuntimeManifest(contents string) (featureRuntimeManifest, error) {
	var manifest featureRuntimeManifest
	for _, line := range strings.Split(strings.TrimSuffix(contents, "\n"), "\n") {
		name, value, ok := strings.Cut(line, ": ")
		if !ok {
			continue
		}
		switch name {
		case "feature_id":
			manifest.FeatureID = value
		case "worktree":
			manifest.Worktree = value
		case "branch":
			manifest.Branch = value
		}
	}
	if manifest.FeatureID == "" || manifest.Worktree == "" || manifest.Branch == "" {
		return featureRuntimeManifest{}, fmt.Errorf("current manifest is missing feature identity")
	}
	return manifest, nil
}
