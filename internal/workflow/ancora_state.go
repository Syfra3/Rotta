package workflow

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type AncoraWorkflowState struct {
	SpecPath       string            `json:"spec_path"`
	FeaturePaths   []string          `json:"feature_paths"`
	Phase          string            `json:"phase"`
	ApprovalStatus string            `json:"approval_status"`
	RiskLevel      string            `json:"risk_level"`
	RequirementIDs []string          `json:"requirement_ids"`
	ScenarioIDs    []string          `json:"scenario_ids"`
	ObservationIDs []string          `json:"observation_ids,omitempty"`
	Checksums      map[string]string `json:"checksums,omitempty"`
}

func SerializeAncoraWorkflowState(state AncoraWorkflowState) ([]byte, error) {
	return json.Marshal(state)
}

type PointerIssueKind string

const (
	PointerIssueMissing          PointerIssueKind = "missing"
	PointerIssueChecksumMismatch PointerIssueKind = "checksum_mismatch"
)

type WorkflowPointerIssue struct {
	Path string
	Kind PointerIssueKind
}

type WorkflowPointerValidationReport struct {
	RepositoryContentAuthoritative bool
	Issues                         []WorkflowPointerIssue
	RepairedState                  AncoraWorkflowState
}

func ValidateAncoraWorkflowPointers(repoRoot string, state AncoraWorkflowState, _ map[string]string) (WorkflowPointerValidationReport, error) {
	report := WorkflowPointerValidationReport{
		RepositoryContentAuthoritative: true,
		RepairedState:                  state,
	}

	for _, path := range append([]string{state.SpecPath}, state.FeaturePaths...) {
		issue, ok, err := validateWorkflowPointer(repoRoot, path, state.Checksums[path])
		if err != nil {
			return WorkflowPointerValidationReport{}, err
		}
		if ok {
			report.Issues = append(report.Issues, issue)
		}
	}

	if len(report.Issues) > 0 {
		repaired, err := repairWorkflowPointersFromRepository(repoRoot, state)
		if err == nil {
			report.RepairedState = repaired
		}
	}

	return report, nil
}

func validateWorkflowPointer(repoRoot, path, expectedChecksum string) (WorkflowPointerIssue, bool, error) {
	content, err := readRepositoryFile(repoRoot, path)
	if err != nil {
		if os.IsNotExist(err) {
			return WorkflowPointerIssue{Path: path, Kind: PointerIssueMissing}, true, nil
		}
		return WorkflowPointerIssue{}, false, fmt.Errorf("read workflow pointer %s: %w", path, err)
	}
	if expectedChecksum != "" && expectedChecksum != fmt.Sprintf("sha256:%x", sha256.Sum256(content)) {
		return WorkflowPointerIssue{Path: path, Kind: PointerIssueChecksumMismatch}, true, nil
	}
	return WorkflowPointerIssue{}, false, nil
}

func repairWorkflowPointersFromRepository(repoRoot string, state AncoraWorkflowState) (AncoraWorkflowState, error) {
	repaired := state
	tracked, err := trackedWorkflowArtifactPaths(repoRoot)
	if err != nil {
		return AncoraWorkflowState{}, err
	}

	if replacement := findTrackedScenarioPath(repoRoot, tracked, "specs/", state.ScenarioIDs); replacement != "" {
		repaired.SpecPath = replacement
	}
	if replacement := findTrackedScenarioPath(repoRoot, tracked, "features/", state.ScenarioIDs); replacement != "" {
		repaired.FeaturePaths = []string{replacement}
	}
	return repaired, nil
}

func trackedWorkflowArtifactPaths(repoRoot string) ([]string, error) {
	cmd := exec.Command("git", "ls-files", "--", "specs", "features")
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("list tracked workflow artifacts: %w: %s", err, output)
	}
	var paths []string
	for _, path := range strings.Fields(string(output)) {
		paths = append(paths, filepath.ToSlash(path))
	}
	return paths, nil
}

func findTrackedScenarioPath(repoRoot string, paths []string, prefix string, scenarioIDs []string) string {
	var contractCandidate string
	contractCandidateCount := 0
	for _, path := range paths {
		if !strings.HasPrefix(path, prefix) {
			continue
		}
		if strings.Contains(path, "workflow_artifact_lifecycle") {
			contractCandidate = path
			contractCandidateCount++
		}
		content, err := readRepositoryFile(repoRoot, path)
		if err != nil {
			continue
		}
		for _, scenarioID := range scenarioIDs {
			if strings.Contains(string(content), scenarioID) {
				return path
			}
		}
	}
	if contractCandidateCount != 1 {
		return ""
	}
	return contractCandidate
}
