package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	compactSliceCheckpointMode = "compact_slice"
	maxCompactSliceScenarios   = 3
	maxCompactSlicePaths       = 12
)

// CompactSliceScenarioEvidence identifies one scenario's complete local TDD
// and focused-test evidence within a compact execution slice.
type CompactSliceScenarioEvidence struct {
	ScenarioID              string
	RedEvidencePath         string
	GreenEvidencePath       string
	RefactorEvidencePath    string
	FocusedTestEvidencePath string
}

// CompactSliceRequest contains the bounded, already-approved work to execute
// as one compact checkpoint boundary.
type CompactSliceRequest struct {
	FeatureWorktree      string
	SliceID              string
	ComponentScope       string
	ScenarioEvidence     []CompactSliceScenarioEvidence
	ExpectedChangedPaths []string
	CapsuleDecisionPath  string
	RunFullValidation    func() error
}

// CompactSliceReport records the compact checkpoint and its scenario traces.
type CompactSliceReport struct {
	SliceID             string
	Scenarios           []CompactSliceScenarioEvidence
	CapsuleDecisionPath string
	Checkpoint          string
}

// ExecuteCompactSlice records a bounded compact slice only after each
// scenario has complete evidence, then runs one full validation and creates
// one checkpoint for the completed slice.
func ExecuteCompactSlice(request CompactSliceRequest) (CompactSliceReport, error) {
	if err := validateCompactSliceRequest(request); err != nil {
		return CompactSliceReport{}, err
	}
	worktree, err := filepath.Abs(request.FeatureWorktree)
	if err != nil {
		return CompactSliceReport{}, fmt.Errorf("resolve compact slice worktree: %w", err)
	}
	if err := requireCompactSliceMode(worktree); err != nil {
		return CompactSliceReport{}, err
	}
	if err := validateCompactSliceApproval(worktree, request.ScenarioEvidence); err != nil {
		return CompactSliceReport{}, err
	}
	if err := validateCompactSliceEvidence(worktree, request.ScenarioEvidence); err != nil {
		return CompactSliceReport{}, err
	}
	capsuleDecisionPath, err := validateCompactSliceCapsuleDecision(worktree, request.CapsuleDecisionPath)
	if err != nil {
		return CompactSliceReport{}, err
	}
	if err := request.RunFullValidation(); err != nil {
		return CompactSliceReport{}, fmt.Errorf("compact slice full validation: %w", err)
	}
	if err := validateCompactSliceChanges(worktree, request.ExpectedChangedPaths); err != nil {
		return CompactSliceReport{}, err
	}
	if err := stageScenarioChanges(worktree, request.ExpectedChangedPaths); err != nil {
		return CompactSliceReport{}, err
	}
	checkpoint, err := createCompactSliceCheckpoint(worktree, request.SliceID)
	if err != nil {
		return CompactSliceReport{}, err
	}
	if err := appendCompactSliceState(worktree, request, capsuleDecisionPath, checkpoint); err != nil {
		return CompactSliceReport{}, err
	}
	return CompactSliceReport{
		SliceID:             request.SliceID,
		Scenarios:           append([]CompactSliceScenarioEvidence(nil), request.ScenarioEvidence...),
		CapsuleDecisionPath: capsuleDecisionPath,
		Checkpoint:          checkpoint,
	}, nil
}

func validateCompactSliceRequest(request CompactSliceRequest) error {
	if request.SliceID == "" || filepath.Base(request.SliceID) != request.SliceID || request.ComponentScope == "" || request.RunFullValidation == nil {
		return fmt.Errorf("compact slice requires a safe slice ID, component scope, and full validation")
	}
	if len(request.ScenarioEvidence) == 0 || len(request.ScenarioEvidence) > maxCompactSliceScenarios {
		return fmt.Errorf("compact slice requires one to %d scenarios", maxCompactSliceScenarios)
	}
	if len(request.ExpectedChangedPaths) == 0 || len(request.ExpectedChangedPaths) > maxCompactSlicePaths {
		return fmt.Errorf("compact slice requires at most %d expected changed paths", maxCompactSlicePaths)
	}
	return validateScenarioPaths(request.ExpectedChangedPaths)
}

func requireCompactSliceMode(worktree string) error {
	contents, err := os.ReadFile(filepath.Join(worktree, ".rotta", "current", "manifest.yaml"))
	if err != nil {
		return fmt.Errorf("read compact slice manifest: %w", err)
	}
	if !strings.Contains(string(contents), "checkpoint_mode: "+compactSliceCheckpointMode+"\n") {
		return fmt.Errorf("compact slice requires manifest checkpoint_mode: %s before execution", compactSliceCheckpointMode)
	}
	return nil
}

func validateCompactSliceApproval(worktree string, scenarios []CompactSliceScenarioEvidence) error {
	contents, err := os.ReadFile(filepath.Join(worktree, ".rotta", "current", "state.yaml"))
	if err != nil {
		return fmt.Errorf("read compact slice state: %w", err)
	}
	approved := compactSliceApprovedScenarios(string(contents))
	previousApprovalIndex := -1
	for _, scenario := range scenarios {
		if !scenarioIDPattern.MatchString(scenario.ScenarioID) {
			return fmt.Errorf("compact slice has invalid scenario ID %q", scenario.ScenarioID)
		}
		approvalIndex := compactSliceIndex(approved, scenario.ScenarioID)
		if approvalIndex < 0 {
			return fmt.Errorf("compact slice scenario %q is not approved", scenario.ScenarioID)
		}
		if approvalIndex <= previousApprovalIndex {
			return fmt.Errorf("compact slice scenarios must be approved and ordered")
		}
		previousApprovalIndex = approvalIndex
	}
	return nil
}

func compactSliceApprovedScenarios(state string) []string {
	for _, line := range strings.Split(state, "\n") {
		key, value, found := strings.Cut(line, ":")
		if !found || strings.TrimSpace(key) != "approved_scenarios" {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), "[]")
		if value == "" {
			return nil
		}
		return strings.Split(strings.ReplaceAll(value, " ", ""), ",")
	}
	return nil
}

func compactSliceIndex(scenarios []string, scenarioID string) int {
	for index, candidate := range scenarios {
		if candidate == scenarioID {
			return index
		}
	}
	return -1
}

func validateCompactSliceEvidence(worktree string, scenarios []CompactSliceScenarioEvidence) error {
	for _, scenario := range scenarios {
		for _, evidencePath := range []string{scenario.RedEvidencePath, scenario.GreenEvidencePath, scenario.RefactorEvidencePath, scenario.FocusedTestEvidencePath} {
			path, err := localScopeEvidencePath(worktree, evidencePath)
			if err != nil {
				return fmt.Errorf("compact slice %s evidence: %w", scenario.ScenarioID, err)
			}
			if err := requireRegularFile(path); err != nil {
				return fmt.Errorf("compact slice %s evidence: %w", scenario.ScenarioID, err)
			}
		}
	}
	return nil
}

func validateCompactSliceCapsuleDecision(worktree, decisionPath string) (string, error) {
	path, err := localScopeEvidencePath(worktree, decisionPath)
	if err != nil {
		return "", fmt.Errorf("compact slice capsule decision: %w", err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("compact slice capsule decision: %w", err)
	}
	var header struct {
		CapsuleDecision string `json:"capsule_decision"`
	}
	if err := json.Unmarshal(contents, &header); err != nil {
		return "", fmt.Errorf("compact slice capsule decision: malformed decision: %w", err)
	}
	switch header.CapsuleDecision {
	case CapsuleDecisionNoneRequired:
		var decision localScopeCapsuleDecision
		if err := decodeCompactSliceDecision(contents, &decision); err != nil {
			return "", err
		}
		if decision.ScenarioOrSlice == "" || decision.StatePath != filepath.Join(worktree, ".rotta", "current", "state.yaml") {
			return "", fmt.Errorf("compact slice requires a valid none-required capsule decision")
		}
		evidencePath, err := localScopeEvidencePath(worktree, decision.EvidencePath)
		if err != nil || requireRegularFile(evidencePath) != nil {
			return "", fmt.Errorf("compact slice requires a valid none-required capsule decision")
		}
	case CapsuleDecisionCreated:
		var decision explorationCapsuleDecision
		if err := decodeCompactSliceDecision(contents, &decision); err != nil {
			return "", err
		}
		if err := validateCreatedCapsuleDecision(worktree, decision); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("compact slice requires a valid none-required or created capsule decision")
	}
	return path, nil
}

func decodeCompactSliceDecision(contents []byte, target any) error {
	decoder := json.NewDecoder(strings.NewReader(string(contents)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("compact slice capsule decision: malformed decision: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("compact slice capsule decision: malformed decision")
	}
	return nil
}

func validateCreatedCapsuleDecision(worktree string, decision explorationCapsuleDecision) error {
	if decision.CapsuleID == "" || filepath.Base(decision.CapsuleID) != decision.CapsuleID || decision.ScenarioOrSlice == "" || decision.StatePath != filepath.Join(worktree, ".rotta", "current", "state.yaml") || len(decision.RequiredEvidencePaths) == 0 {
		return fmt.Errorf("compact slice requires a valid created capsule decision")
	}
	fingerprint, err := hex.DecodeString(decision.CapsuleFingerprint)
	if err != nil || len(fingerprint) != sha256.Size {
		return fmt.Errorf("compact slice requires a valid created capsule decision")
	}
	for _, evidencePath := range decision.RequiredEvidencePaths {
		path, err := localScopeEvidencePath(worktree, evidencePath)
		if err != nil || requireRegularFile(path) != nil {
			return fmt.Errorf("compact slice requires a valid created capsule decision")
		}
	}
	capsulePath := filepath.Join(worktree, ".rotta", "current", "capsules", decision.CapsuleID+".md")
	contents, err := os.ReadFile(capsulePath)
	if err != nil || verifyCapsuleFingerprint(string(contents)) != nil {
		return fmt.Errorf("compact slice requires a valid created capsule decision")
	}
	metadata, err := readCapsuleMetadata(string(contents))
	if err != nil || metadata.ScenarioOrSlice != decision.ScenarioOrSlice || !strings.Contains(string(contents), capsuleFingerprintPrefix+decision.CapsuleFingerprint+"\n") {
		return fmt.Errorf("compact slice requires a valid created capsule decision")
	}
	return nil
}

func validateCompactSliceChanges(worktree string, expectedPaths []string) error {
	if untracked, err := untrackedNonIgnoredPaths(worktree); err != nil {
		return err
	} else if len(untracked) > 0 {
		return fmt.Errorf("unexpected untracked change before compact slice checkpoint: %s", untracked[0])
	}
	changed, err := trackedChangedPaths(worktree)
	if err != nil {
		return err
	}
	for _, path := range changed {
		if !containsPath(expectedPaths, path) {
			return fmt.Errorf("unexpected tracked change before compact slice checkpoint: %s", path)
		}
	}
	return nil
}

func createCompactSliceCheckpoint(worktree, sliceID string) (string, error) {
	commit := exec.Command("git", "commit", "-m", "checkpoint: compact slice "+sliceID)
	commit.Dir = worktree
	if output, err := commit.CombinedOutput(); err != nil {
		return "", fmt.Errorf("create compact slice checkpoint: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return scenarioCheckpointCommitID(worktree)
}

func appendCompactSliceState(worktree string, request CompactSliceRequest, capsuleDecisionPath, checkpoint string) error {
	statePath := filepath.Join(worktree, ".rotta", "current", "state.yaml")
	state, err := os.ReadFile(statePath)
	if err != nil {
		return fmt.Errorf("read compact slice state: %w", err)
	}
	var record strings.Builder
	record.WriteString("\ncompact_slices:\n  - slice_id: ")
	record.WriteString(request.SliceID)
	record.WriteString("\n    component_scope: ")
	record.WriteString(request.ComponentScope)
	record.WriteString("\n    capsule_decision: ")
	record.WriteString(capsuleDecisionPath)
	record.WriteString("\n    checkpoint: ")
	record.WriteString(checkpoint)
	record.WriteString("\n    scenarios:\n")
	for _, scenario := range request.ScenarioEvidence {
		fmt.Fprintf(&record, "      - scenario_id: %s\n        red_evidence: %s\n        green_evidence: %s\n        refactor_evidence: %s\n        focused_test_evidence: %s\n", scenario.ScenarioID, scenario.RedEvidencePath, scenario.GreenEvidencePath, scenario.RefactorEvidencePath, scenario.FocusedTestEvidencePath)
	}
	if err := os.WriteFile(statePath, append(state, record.String()...), 0o600); err != nil {
		return fmt.Errorf("write compact slice state: %w", err)
	}
	return nil
}
