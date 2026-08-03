package workflow

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FeatureWorkflowResume identifies the sole feature slice selected from a
// verified feature-local recovery boundary.
type FeatureWorkflowResume struct {
	FeatureID        string
	ScenarioOrSlice  string
	RetiredArtifacts []string
}

type featureWorkflowManifest struct {
	FeatureID         string
	Worktree          string
	Branch            string
	BaseSHA           string
	PolicyPath        string
	PolicyFingerprint string
}

type featureWorkflowState struct {
	FeatureID           string
	Worktree            string
	Branch              string
	BaselineSHA         string
	ManifestFingerprint string
	ApprovalPath        string
	ApprovalFingerprint string
	ScenarioOrSlice     string
	Status              string
}

// ResumeFeatureWorkflow reloads one feature's current runtime state only after
// its recorded worktree, baseline, and fingerprints still agree.
func ResumeFeatureWorkflow(repoRoot string) (FeatureWorkflowResume, error) {
	actualWorktree, err := resolveFeatureWorkflowWorktree(repoRoot)
	if err != nil {
		return FeatureWorkflowResume{}, err
	}

	manifestContents, err := readRepositoryFile(actualWorktree, ".rotta/current/manifest.yaml")
	if err != nil {
		return FeatureWorkflowResume{}, fmt.Errorf("read feature workflow manifest: %w", err)
	}
	manifest, err := parseFeatureWorkflowManifest(string(manifestContents))
	if err != nil {
		return FeatureWorkflowResume{}, err
	}
	stateContents, err := readRepositoryFile(actualWorktree, ".rotta/current/state.yaml")
	if err != nil {
		return FeatureWorkflowResume{}, fmt.Errorf("read feature workflow state: %w", err)
	}
	state, err := parseFeatureWorkflowState(string(stateContents))
	if err != nil {
		return FeatureWorkflowResume{}, err
	}
	if err := verifyFeatureWorkflowRecoveryBoundary(actualWorktree, manifestContents, manifest, state); err != nil {
		return FeatureWorkflowResume{}, err
	}
	return FeatureWorkflowResume{
		FeatureID:        manifest.FeatureID,
		ScenarioOrSlice:  state.ScenarioOrSlice,
		RetiredArtifacts: findRetiredFeatureWorkflowArtifacts(actualWorktree),
	}, nil
}

// ArchiveTerminalFeatureRuntime moves exactly the verified worktree's current
// runtime directory while retaining all durable handoff artifacts in place.
func ArchiveTerminalFeatureRuntime(repoRoot string) error {
	resumed, err := ResumeFeatureWorkflow(repoRoot)
	if err != nil {
		return err
	}
	stateContents, err := readRepositoryFile(repoRoot, ".rotta/current/state.yaml")
	if err != nil {
		return fmt.Errorf("read feature workflow state: %w", err)
	}
	state, err := parseFeatureWorkflowState(string(stateContents))
	if err != nil {
		return err
	}
	if state.Status != "terminal" {
		return fmt.Errorf("archive feature workflow requires a verified terminal state")
	}

	actualWorktree, err := resolveFeatureWorkflowWorktree(repoRoot)
	if err != nil {
		return err
	}
	archive := filepath.Join(actualWorktree, ".rotta", "archive", resumed.FeatureID, state.BaselineSHA)
	if err := os.MkdirAll(filepath.Dir(archive), 0o700); err != nil {
		return fmt.Errorf("create feature workflow archive: %w", err)
	}
	if err := os.Rename(filepath.Join(actualWorktree, ".rotta", "current"), archive); err != nil {
		return fmt.Errorf("archive feature workflow runtime: %w", err)
	}
	return nil
}

func resolveFeatureWorkflowWorktree(repoRoot string) (string, error) {
	actualWorktree, err := gitSubmissionOutput(repoRoot, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("resolve feature workflow worktree: %w", err)
	}
	actualWorktree, err = filepath.EvalSymlinks(actualWorktree)
	if err != nil {
		return "", fmt.Errorf("resolve feature workflow worktree: %w", err)
	}
	return actualWorktree, nil
}

func verifyFeatureWorkflowRecoveryBoundary(actualWorktree string, manifestContents []byte, manifest featureWorkflowManifest, state featureWorkflowState) error {
	recordedWorktree, err := filepath.EvalSymlinks(manifest.Worktree)
	if err != nil || recordedWorktree != actualWorktree || state.Worktree != manifest.Worktree || state.FeatureID != manifest.FeatureID || state.Branch != manifest.Branch || state.BaselineSHA != manifest.BaseSHA {
		return fmt.Errorf("feature workflow recovery boundary does not match its recorded feature worktree")
	}
	branch, err := gitSubmissionOutput(actualWorktree, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil || branch != manifest.Branch {
		return fmt.Errorf("feature workflow recovery boundary does not match its recorded feature branch")
	}
	if _, err := gitSubmissionOutput(actualWorktree, "cat-file", "-e", state.BaselineSHA+"^{commit}"); err != nil {
		return fmt.Errorf("feature workflow recovery boundary does not match its recorded baseline")
	}
	if fmt.Sprintf("%x", sha256.Sum256(manifestContents)) != state.ManifestFingerprint {
		return fmt.Errorf("feature workflow recovery boundary manifest fingerprint does not match")
	}
	if manifest.PolicyPath != featureWorkflowPolicyPath {
		return fmt.Errorf("feature workflow recovery boundary policy path is not feature-local")
	}
	if err := verifyFeatureWorkflowFingerprint(actualWorktree, manifest.PolicyPath, manifest.PolicyFingerprint); err != nil {
		return fmt.Errorf("policy remediation: restore the manifest-bound feature-local policy %q: %w", manifest.PolicyPath, err)
	}
	if err := verifyFeatureWorkflowFingerprint(actualWorktree, state.ApprovalPath, state.ApprovalFingerprint); err != nil {
		return err
	}
	return nil
}

func verifyFeatureWorkflowFingerprint(repoRoot, path, want string) error {
	contents, err := readRepositoryFile(repoRoot, path)
	if err != nil || want == "" || fmt.Sprintf("%x", sha256.Sum256(contents)) != want {
		return fmt.Errorf("feature workflow recovery boundary fingerprint does not match")
	}
	return nil
}

func parseFeatureWorkflowManifest(contents string) (featureWorkflowManifest, error) {
	var manifest featureWorkflowManifest
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
		case "base_sha":
			manifest.BaseSHA = value
		case "policy_path":
			manifest.PolicyPath = value
		case "policy_fingerprint":
			manifest.PolicyFingerprint = value
		}
	}
	if manifest.FeatureID == "" || manifest.Worktree == "" || manifest.Branch == "" || manifest.BaseSHA == "" || manifest.PolicyPath == "" || manifest.PolicyFingerprint == "" {
		return featureWorkflowManifest{}, fmt.Errorf("feature workflow manifest is incomplete")
	}
	return manifest, nil
}

func parseFeatureWorkflowState(contents string) (featureWorkflowState, error) {
	var state featureWorkflowState
	for _, line := range strings.Split(strings.TrimSuffix(contents, "\n"), "\n") {
		name, value, ok := strings.Cut(line, ": ")
		if !ok {
			continue
		}
		switch name {
		case "feature_id":
			state.FeatureID = value
		case "worktree":
			state.Worktree = value
		case "branch":
			state.Branch = value
		case "baseline_sha":
			state.BaselineSHA = value
		case "manifest_fingerprint":
			state.ManifestFingerprint = value
		case "approval_path":
			state.ApprovalPath = value
		case "approval_fingerprint":
			state.ApprovalFingerprint = value
		case "scenario_or_slice":
			state.ScenarioOrSlice = value
		case "status":
			state.Status = value
		}
	}
	if state.FeatureID == "" || state.Worktree == "" || state.Branch == "" || state.BaselineSHA == "" || state.ManifestFingerprint == "" || state.ApprovalPath == "" || state.ApprovalFingerprint == "" || state.ScenarioOrSlice == "" || state.Status == "" {
		return featureWorkflowState{}, fmt.Errorf("feature workflow state is incomplete")
	}
	return state, nil
}
