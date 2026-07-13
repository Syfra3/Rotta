package workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type AutonomousPhase3WorktreeRequest struct {
	Scope        ContractScope
	Branch       string
	WorktreePath string
}

type AutonomousPhase3WorktreePreparation struct {
	Branch       string
	WorktreePath string
}

type AutonomousScenarioLoopRequest struct {
	Scope                ContractScope
	LaunchScenario       func() error
	CreateScenarioCommit func() error
}

type AutonomousScenarioLoopDecision struct {
	Approved bool
	Reason   string
}

type ScenarioCheckpointRequest struct {
	ScenarioID       string
	ExpectedPaths    []string
	TDDComplete      bool
	TestsPassed      bool
	ValidationPassed bool
}

type ScenarioCheckpointRecord struct {
	ScenarioID string
	CommitID   string
}

type AutonomousPhase3WorkflowState struct {
	Checkpoints map[string]string `json:"checkpoints"`
}

func StartAutonomousScenarioLoop(repoRoot string, request AutonomousScenarioLoopRequest) (AutonomousScenarioLoopDecision, error) {
	gate, err := EvaluateImplementationGate(repoRoot, request.Scope)
	if err != nil {
		return AutonomousScenarioLoopDecision{}, err
	}
	if !gate.Approved {
		return AutonomousScenarioLoopDecision{
			Reason: fmt.Sprintf("explicit human Gherkin approval is required for %s#%s", request.Scope.FeaturePath, request.Scope.ScenarioID),
		}, nil
	}

	return AutonomousScenarioLoopDecision{Approved: true, Reason: gate.Reason}, nil
}

func CheckpointApprovedScenario(repoRoot string, request ScenarioCheckpointRequest) (ScenarioCheckpointRecord, error) {
	if !request.TDDComplete {
		return ScenarioCheckpointRecord{}, fmt.Errorf("strict Red, Green, and Refactor evidence is required before checkpointing")
	}
	if !request.TestsPassed {
		return ScenarioCheckpointRecord{}, fmt.Errorf("required tests must pass before checkpointing")
	}
	if !request.ValidationPassed {
		return ScenarioCheckpointRecord{}, fmt.Errorf("active objective validation must pass before checkpointing")
	}

	untracked, err := untrackedNonIgnoredPaths(repoRoot)
	if err != nil {
		return ScenarioCheckpointRecord{}, err
	}
	if len(untracked) > 0 {
		return ScenarioCheckpointRecord{}, fmt.Errorf("unexpected untracked change before checkpointing: %s", untracked[0])
	}

	changed, err := trackedChangedPaths(repoRoot)
	if err != nil {
		return ScenarioCheckpointRecord{}, err
	}
	for _, path := range changed {
		if !containsPath(request.ExpectedPaths, path) {
			return ScenarioCheckpointRecord{}, fmt.Errorf("unexpected tracked change before checkpointing: %s", path)
		}
	}

	add := exec.Command("git", append([]string{"add", "--"}, request.ExpectedPaths...)...)
	add.Dir = repoRoot
	if output, err := add.CombinedOutput(); err != nil {
		return ScenarioCheckpointRecord{}, fmt.Errorf("stage scenario changes: %w: %s", err, strings.TrimSpace(string(output)))
	}

	commit := exec.Command("git", "commit", "-m", "checkpoint: "+request.ScenarioID)
	commit.Dir = repoRoot
	if output, err := commit.CombinedOutput(); err != nil {
		return ScenarioCheckpointRecord{}, fmt.Errorf("create scenario checkpoint: %w: %s", err, strings.TrimSpace(string(output)))
	}

	revision := exec.Command("git", "rev-parse", "HEAD")
	revision.Dir = repoRoot
	output, err := revision.CombinedOutput()
	if err != nil {
		return ScenarioCheckpointRecord{}, fmt.Errorf("read scenario checkpoint commit: %w: %s", err, strings.TrimSpace(string(output)))
	}
	record := ScenarioCheckpointRecord{ScenarioID: request.ScenarioID, CommitID: strings.TrimSpace(string(output))}
	if err := writeAutonomousPhase3WorkflowState(repoRoot, record); err != nil {
		return ScenarioCheckpointRecord{}, err
	}
	return record, nil
}

func untrackedNonIgnoredPaths(repoRoot string) ([]string, error) {
	status := exec.Command("git", "ls-files", "--others", "--exclude-standard")
	status.Dir = repoRoot
	output, err := status.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("inspect untracked scenario changes: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return strings.Fields(string(output)), nil
}

func trackedChangedPaths(repoRoot string) ([]string, error) {
	status := exec.Command("git", "diff", "--name-only")
	status.Dir = repoRoot
	output, err := status.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("inspect tracked scenario changes: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return strings.Fields(string(output)), nil
}

func containsPath(paths []string, candidate string) bool {
	for _, path := range paths {
		if path == candidate {
			return true
		}
	}
	return false
}

func writeAutonomousPhase3WorkflowState(repoRoot string, record ScenarioCheckpointRecord) error {
	state := AutonomousPhase3WorkflowState{Checkpoints: map[string]string{record.ScenarioID: record.CommitID}}
	content, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("serialize autonomous Phase 3 workflow state: %w", err)
	}
	path := filepath.Join(repoRoot, ".rotta", "autonomous-phase3-state.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create autonomous Phase 3 workflow state directory: %w", err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("write autonomous Phase 3 workflow state: %w", err)
	}
	return nil
}

func PrepareAutonomousPhase3Worktree(repoRoot string, request AutonomousPhase3WorktreeRequest) (AutonomousPhase3WorktreePreparation, error) {
	command := exec.Command("git", "worktree", "add", "-b", request.Branch, request.WorktreePath, "HEAD")
	command.Dir = repoRoot
	if output, err := command.CombinedOutput(); err != nil {
		return AutonomousPhase3WorktreePreparation{}, fmt.Errorf("create isolated Phase 3 worktree: %w: %s", err, strings.TrimSpace(string(output)))
	}

	for _, path := range []string{request.Scope.SpecPath, request.Scope.FeaturePath} {
		if _, err := os.Stat(filepath.Join(request.WorktreePath, filepath.FromSlash(path))); err != nil {
			return AutonomousPhase3WorktreePreparation{}, fmt.Errorf("verify approved contract artifact %s: %w", path, err)
		}
	}

	status := exec.Command("git", "status", "--short")
	status.Dir = request.WorktreePath
	output, err := status.CombinedOutput()
	if err != nil {
		return AutonomousPhase3WorktreePreparation{}, fmt.Errorf("check isolated Phase 3 worktree status: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if strings.TrimSpace(string(output)) != "" {
		return AutonomousPhase3WorktreePreparation{}, fmt.Errorf("isolated Phase 3 worktree has non-ignored changes: %s", strings.TrimSpace(string(output)))
	}

	return AutonomousPhase3WorktreePreparation{Branch: request.Branch, WorktreePath: request.WorktreePath}, nil
}
