package workflow

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const strictPerScenarioCheckpointMode = "strict_per_scenario"

// StrictScenarioEvidence identifies one scenario's complete local TDD and
// focused-test evidence at its own checkpoint boundary.
type StrictScenarioEvidence struct {
	ScenarioID              string
	RedEvidencePath         string
	GreenEvidencePath       string
	RefactorEvidencePath    string
	FocusedTestEvidencePath string
}

// StrictScenarioRequest contains one approved scenario to execute at its own
// full validation and checkpoint boundary.
type StrictScenarioRequest struct {
	FeatureWorktree      string
	Scenario             StrictScenarioEvidence
	ExpectedChangedPaths []string
	RunFullValidation    func() error
}

// StrictScenarioReport records the one-scenario strict checkpoint.
type StrictScenarioReport struct {
	Scenarios  []StrictScenarioEvidence
	Checkpoint string
}

// ExecuteStrictScenario keeps an approved scenario isolated: it validates the
// scenario's evidence, runs full validation, and creates its own checkpoint.
func ExecuteStrictScenario(request StrictScenarioRequest) (StrictScenarioReport, error) {
	if err := validateStrictScenarioRequest(request); err != nil {
		return StrictScenarioReport{}, err
	}
	worktree, err := filepath.Abs(request.FeatureWorktree)
	if err != nil {
		return StrictScenarioReport{}, fmt.Errorf("resolve strict scenario worktree: %w", err)
	}
	if err := requireStrictPerScenarioMode(worktree); err != nil {
		return StrictScenarioReport{}, err
	}
	if err := validateStrictScenarioApproval(worktree, request.Scenario.ScenarioID); err != nil {
		return StrictScenarioReport{}, err
	}
	if err := validateStrictScenarioEvidence(worktree, request.Scenario); err != nil {
		return StrictScenarioReport{}, err
	}
	if err := request.RunFullValidation(); err != nil {
		return StrictScenarioReport{}, fmt.Errorf("strict scenario full validation: %w", err)
	}
	if err := validateStrictScenarioChanges(worktree, request.ExpectedChangedPaths); err != nil {
		return StrictScenarioReport{}, err
	}
	if err := stageScenarioChanges(worktree, request.ExpectedChangedPaths); err != nil {
		return StrictScenarioReport{}, err
	}
	checkpoint, err := createStrictScenarioCheckpoint(worktree, request.Scenario.ScenarioID)
	if err != nil {
		return StrictScenarioReport{}, err
	}
	if err := appendStrictScenarioState(worktree, request.Scenario, checkpoint); err != nil {
		return StrictScenarioReport{}, err
	}
	return StrictScenarioReport{Scenarios: []StrictScenarioEvidence{request.Scenario}, Checkpoint: checkpoint}, nil
}

func validateStrictScenarioRequest(request StrictScenarioRequest) error {
	if request.RunFullValidation == nil || !scenarioIDPattern.MatchString(request.Scenario.ScenarioID) {
		return fmt.Errorf("strict scenario requires one valid scenario and full validation")
	}
	if len(request.ExpectedChangedPaths) == 0 {
		return fmt.Errorf("strict scenario requires expected changed paths")
	}
	return validateScenarioPaths(request.ExpectedChangedPaths)
}

func requireStrictPerScenarioMode(worktree string) error {
	contents, err := os.ReadFile(filepath.Join(worktree, ".rotta", "current", "manifest.yaml"))
	if err != nil {
		return fmt.Errorf("read strict scenario manifest: %w", err)
	}
	if !strings.Contains(string(contents), "checkpoint_mode: "+strictPerScenarioCheckpointMode+"\n") {
		return fmt.Errorf("strict scenario requires manifest checkpoint_mode: %s before execution", strictPerScenarioCheckpointMode)
	}
	return nil
}

func validateStrictScenarioApproval(worktree, scenarioID string) error {
	contents, err := os.ReadFile(filepath.Join(worktree, ".rotta", "current", "state.yaml"))
	if err != nil {
		return fmt.Errorf("read strict scenario state: %w", err)
	}
	if !strictScenarioIsApproved(string(contents), scenarioID) {
		return fmt.Errorf("strict scenario %q is not approved", scenarioID)
	}
	return nil
}

func strictScenarioIsApproved(state, scenarioID string) bool {
	for _, line := range strings.Split(state, "\n") {
		key, value, found := strings.Cut(line, ":")
		if !found || strings.TrimSpace(key) != "approved_scenarios" {
			continue
		}
		for _, approvedScenario := range strings.Split(strings.Trim(strings.TrimSpace(value), "[]"), ",") {
			if strings.TrimSpace(approvedScenario) == scenarioID {
				return true
			}
		}
	}
	return false
}

func validateStrictScenarioEvidence(worktree string, scenario StrictScenarioEvidence) error {
	for _, evidencePath := range []string{scenario.RedEvidencePath, scenario.GreenEvidencePath, scenario.RefactorEvidencePath, scenario.FocusedTestEvidencePath} {
		path, err := localScopeEvidencePath(worktree, evidencePath)
		if err != nil {
			return fmt.Errorf("strict scenario %s evidence: %w", scenario.ScenarioID, err)
		}
		if err := requireRegularFile(path); err != nil {
			return fmt.Errorf("strict scenario %s evidence: %w", scenario.ScenarioID, err)
		}
	}
	return nil
}

func validateStrictScenarioChanges(worktree string, expectedPaths []string) error {
	if untracked, err := untrackedNonIgnoredPaths(worktree); err != nil {
		return err
	} else if len(untracked) > 0 {
		return fmt.Errorf("unexpected untracked change before strict scenario checkpoint: %s", untracked[0])
	}
	changed, err := trackedChangedPaths(worktree)
	if err != nil {
		return err
	}
	for _, path := range changed {
		if !containsPath(expectedPaths, path) {
			return fmt.Errorf("unexpected tracked change before strict scenario checkpoint: %s", path)
		}
	}
	return nil
}

func createStrictScenarioCheckpoint(worktree, scenarioID string) (string, error) {
	commit := exec.Command("git", "commit", "-m", "checkpoint: strict scenario "+scenarioID)
	commit.Dir = worktree
	if output, err := commit.CombinedOutput(); err != nil {
		return "", fmt.Errorf("create strict scenario checkpoint: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return scenarioCheckpointCommitID(worktree)
}

func appendStrictScenarioState(worktree string, scenario StrictScenarioEvidence, checkpoint string) error {
	statePath := filepath.Join(worktree, ".rotta", "current", "state.yaml")
	state, err := os.ReadFile(statePath)
	if err != nil {
		return fmt.Errorf("read strict scenario state: %w", err)
	}
	var record strings.Builder
	fmt.Fprintf(&record, "\nstrict_slices:\n  - scenario_id: %s\n    checkpoint: %s\n    red_evidence: %s\n    green_evidence: %s\n    refactor_evidence: %s\n    focused_test_evidence: %s\n", scenario.ScenarioID, checkpoint, scenario.RedEvidencePath, scenario.GreenEvidencePath, scenario.RefactorEvidencePath, scenario.FocusedTestEvidencePath)
	if err := os.WriteFile(statePath, append(state, record.String()...), 0o600); err != nil {
		return fmt.Errorf("write strict scenario state: %w", err)
	}
	return nil
}
