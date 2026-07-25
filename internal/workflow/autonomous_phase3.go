package workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var scenarioIDPattern = regexp.MustCompile(`^SCN-[0-9]{3}$`)

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
	Submission       NewImplementationSubmission
}

type ScenarioCheckpointRecord struct {
	ScenarioID string
	CommitID   string
}

type AutonomousPhase3WorkflowState struct {
	Checkpoints        map[string]string `json:"checkpoints"`
	CompletedScenario  string            `json:"completed_scenario"`
	RemainingScenarios []string          `json:"remaining_scenarios"`
	NextScenario       string            `json:"next_scenario"`
}

type AutonomousPhase3BoundaryDecision struct {
	Phase              string
	FinalHumanApproval bool
}

type AutonomousWorkflowCompletionReport struct {
	HumanMayPushOnce bool
	Message          string
}

func ReportAutonomousWorkflowCompletion() AutonomousWorkflowCompletionReport {
	return AutonomousWorkflowCompletionReport{
		HumanMayPushOnce: true,
		Message:          "workflow and Phase 4 review are complete; a human may manually push the feature branch once",
	}
}

func StartAutonomousScenarioLoop(repoRoot string, request AutonomousScenarioLoopRequest) (AutonomousScenarioLoopDecision, error) {
	gate, err := EvaluateImplementationGate(repoRoot, request.Scope)
	if err != nil {
		return AutonomousScenarioLoopDecision{}, err
	}
	if !gate.Approved {
		if gate.Reason == "baseline confirmation is pending" {
			return AutonomousScenarioLoopDecision{Reason: gate.Reason}, nil
		}
		return AutonomousScenarioLoopDecision{
			Reason: fmt.Sprintf("explicit human Gherkin approval is required for %s#%s", request.Scope.FeaturePath, request.Scope.ScenarioID),
		}, nil
	}
	if request.LaunchScenario != nil {
		if err := request.LaunchScenario(); err != nil {
			return AutonomousScenarioLoopDecision{}, err
		}
	}

	return AutonomousScenarioLoopDecision{Approved: true, Reason: gate.Reason}, nil
}

func CheckpointApprovedScenario(repoRoot string, request ScenarioCheckpointRequest) (ScenarioCheckpointRecord, error) {
	if err := validateScenarioCheckpointEvidence(request); err != nil {
		return ScenarioCheckpointRecord{}, err
	}
	if err := validateScenarioCheckpointBranch(repoRoot, request.Submission); err != nil {
		return ScenarioCheckpointRecord{}, err
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

	if err := stageScenarioChanges(repoRoot, request.ExpectedPaths); err != nil {
		return ScenarioCheckpointRecord{}, err
	}

	commitID, err := createScenarioCheckpointCommit(repoRoot, request.ScenarioID)
	if err != nil {
		return ScenarioCheckpointRecord{}, err
	}
	record := ScenarioCheckpointRecord{ScenarioID: request.ScenarioID, CommitID: commitID}
	if err := writeAutonomousPhase3WorkflowState(repoRoot, record); err != nil {
		return ScenarioCheckpointRecord{}, err
	}
	return record, nil
}

func validateScenarioCheckpointBranch(repoRoot string, submission NewImplementationSubmission) error {
	if submission.WorktreePath == "" && submission.BaseBranch == "" && submission.FeatureBranch == "" {
		return nil
	}
	if submission.WorktreePath == "" || submission.BaseBranch == "" || submission.FeatureBranch == "" {
		return fmt.Errorf("scenario checkpoint requires a complete recorded isolated feature worktree")
	}
	recordedWorktree, err := filepath.EvalSymlinks(submission.WorktreePath)
	if err != nil {
		return fmt.Errorf("scenario checkpoint requires the recorded isolated feature worktree: %w", err)
	}
	checkpointWorktree, err := filepath.EvalSymlinks(repoRoot)
	if err != nil || checkpointWorktree != recordedWorktree {
		return fmt.Errorf("scenario checkpoint requires the recorded isolated feature worktree")
	}
	branch, err := gitSubmissionOutput(repoRoot, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return fmt.Errorf("scenario checkpoint requires an attached feature branch: detached HEAD")
	}
	if branch != submission.FeatureBranch {
		return fmt.Errorf("scenario checkpoint branch %q does not match recorded feature branch %q", branch, submission.FeatureBranch)
	}
	if branch == "main" || branch == "master" || branch == "develop" || strings.HasPrefix(branch, "release/") || strings.HasPrefix(branch, "hotfix/") {
		return fmt.Errorf("scenario checkpoint cannot commit on protected branch %q", branch)
	}
	if branch == submission.BaseBranch {
		return fmt.Errorf("scenario checkpoint cannot commit on base branch %q", branch)
	}
	if !strings.HasPrefix(branch, "feature/") {
		return fmt.Errorf("scenario checkpoint requires a feature branch, got %q", branch)
	}
	return nil
}

func validateScenarioCheckpointEvidence(request ScenarioCheckpointRequest) error {
	if !request.TDDComplete {
		return fmt.Errorf("strict Red, Green, and Refactor evidence is required before checkpointing")
	}
	if !request.TestsPassed {
		return fmt.Errorf("required tests must pass before checkpointing")
	}
	if !request.ValidationPassed {
		return fmt.Errorf("active objective validation must pass before checkpointing")
	}
	return nil
}

func stageScenarioChanges(repoRoot string, paths []string) error {
	if err := validateScenarioPaths(paths); err != nil {
		return err
	}
	add := exec.Command("git", "add", "--")
	add.Args = append(add.Args, paths...)
	add.Dir = repoRoot
	if output, err := add.CombinedOutput(); err != nil {
		return fmt.Errorf("stage scenario changes: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func createScenarioCheckpointCommit(repoRoot, scenarioID string) (string, error) {
	if !scenarioIDPattern.MatchString(scenarioID) {
		return "", fmt.Errorf("invalid scenario ID %q", scenarioID)
	}
	commit := exec.Command("git", "commit", "-m")
	commit.Args = append(commit.Args, "checkpoint: "+scenarioID)
	commit.Dir = repoRoot
	if output, err := commit.CombinedOutput(); err != nil {
		return "", fmt.Errorf("create scenario checkpoint: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return scenarioCheckpointCommitID(repoRoot)
}

func scenarioCheckpointCommitID(repoRoot string) (string, error) {
	revision := exec.Command("git", "rev-parse", "HEAD")
	revision.Dir = repoRoot
	output, err := revision.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("read scenario checkpoint commit: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
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

func validateScenarioPaths(paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("invalid scenario path set: no paths provided")
	}
	for _, path := range paths {
		clean := filepath.Clean(filepath.FromSlash(path))
		if path == "" || filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("invalid scenario path %q", path)
		}
	}
	return nil
}

func writeAutonomousPhase3WorkflowState(repoRoot string, record ScenarioCheckpointRecord) error {
	state := AutonomousPhase3WorkflowState{Checkpoints: map[string]string{record.ScenarioID: record.CommitID}}
	return writeAutonomousPhase3WorkflowStateValue(repoRoot, state)
}

func ContinueFromAutonomousScenarioCheckpoint(repoRoot string, record ScenarioCheckpointRecord, remainingScenarios []string, startScenario func(string) error) (AutonomousPhase3WorkflowState, error) {
	status := exec.Command("git", "status", "--short")
	status.Dir = repoRoot
	output, err := status.CombinedOutput()
	if err != nil {
		return AutonomousPhase3WorkflowState{}, fmt.Errorf("check scenario checkpoint boundary: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if strings.TrimSpace(string(output)) != "" {
		return AutonomousPhase3WorkflowState{}, fmt.Errorf("scenario checkpoint boundary has non-ignored changes: %s", strings.TrimSpace(string(output)))
	}

	state := AutonomousPhase3WorkflowState{
		Checkpoints:        map[string]string{record.ScenarioID: record.CommitID},
		CompletedScenario:  record.ScenarioID,
		RemainingScenarios: remainingScenarios,
		NextScenario:       remainingScenarios[0],
	}
	if err := writeAutonomousPhase3WorkflowStateValue(repoRoot, state); err != nil {
		return AutonomousPhase3WorkflowState{}, err
	}
	if err := startScenario(state.NextScenario); err != nil {
		return AutonomousPhase3WorkflowState{}, err
	}
	return state, nil
}

func CompleteAutonomousPhase3Boundary(repoRoot string, record ScenarioCheckpointRecord, startReview func() error) (AutonomousPhase3BoundaryDecision, error) {
	status := exec.Command("git", "status", "--short")
	status.Dir = repoRoot
	output, err := status.CombinedOutput()
	if err != nil {
		return AutonomousPhase3BoundaryDecision{}, fmt.Errorf("check final scenario checkpoint boundary: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if strings.TrimSpace(string(output)) != "" {
		return AutonomousPhase3BoundaryDecision{}, fmt.Errorf("final scenario checkpoint boundary has non-ignored changes: %s", strings.TrimSpace(string(output)))
	}

	state := AutonomousPhase3WorkflowState{
		Checkpoints:        map[string]string{record.ScenarioID: record.CommitID},
		CompletedScenario:  record.ScenarioID,
		RemainingScenarios: []string{},
	}
	if err := writeAutonomousPhase3WorkflowStateValue(repoRoot, state); err != nil {
		return AutonomousPhase3BoundaryDecision{}, err
	}
	if err := startReview(); err != nil {
		return AutonomousPhase3BoundaryDecision{}, err
	}
	return AutonomousPhase3BoundaryDecision{Phase: "Phase 4 review"}, nil
}

func writeAutonomousPhase3WorkflowStateValue(repoRoot string, state AutonomousPhase3WorkflowState) error {
	content, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("serialize autonomous Phase 3 workflow state: %w", err)
	}
	path := filepath.Join(repoRoot, ".rotta", "autonomous-phase3-state.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create autonomous Phase 3 workflow state directory: %w", err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return fmt.Errorf("write autonomous Phase 3 workflow state: %w", err)
	}
	return nil
}

func PrepareAutonomousPhase3Worktree(repoRoot string, request AutonomousPhase3WorktreeRequest) (AutonomousPhase3WorktreePreparation, error) {
	if err := validateWorktreeRequest(request); err != nil {
		return AutonomousPhase3WorktreePreparation{}, err
	}
	command := exec.Command("git", "worktree", "add", "-b")
	command.Args = append(command.Args, request.Branch, request.WorktreePath, "HEAD")
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

func validateWorktreeRequest(request AutonomousPhase3WorktreeRequest) error {
	if !isSafeGitBranchName(request.Branch) {
		return fmt.Errorf("invalid Phase 3 worktree branch %q", request.Branch)
	}
	if request.WorktreePath == "" || !filepath.IsAbs(request.WorktreePath) {
		return fmt.Errorf("invalid Phase 3 worktree path %q", request.WorktreePath)
	}
	return nil
}

func isSafeGitBranchName(branch string) bool {
	if branch == "" || strings.HasPrefix(branch, "-") || strings.HasSuffix(branch, ".") || strings.HasSuffix(branch, ".lock") || strings.Contains(branch, "..") || strings.Contains(branch, "@{") {
		return false
	}
	for _, character := range branch {
		if character <= ' ' || character == '~' || character == '^' || character == ':' || character == '?' || character == '*' || character == '[' || character == '\\' || character == 0x7f {
			return false
		}
	}
	for _, component := range strings.Split(branch, "/") {
		if component == "" || component == "." || component == ".." {
			return false
		}
	}
	return true
}
