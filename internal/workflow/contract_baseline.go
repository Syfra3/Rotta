package workflow

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ApprovedContractBaselineRequest struct {
	Submission        NewImplementationSubmission
	SpecPath          string
	FeaturePath       string
	ApprovedScenarios []string
	ApprovedAt        time.Time
}

type ApprovedContractBaseline struct {
	ApprovalRecordPath        string
	ApprovalRecordFingerprint string
	CommitID                  string
}

type ApprovedPhase3Request struct {
	ScenarioID               string
	DelegateScenario         func() error
	CreateScenarioCheckpoint func() error
}

type ApprovedPhase3Decision struct {
	Allowed bool
	Reason  string
}

type ApprovedScenarioRunRequest struct {
	ScenarioID string
	Delegate   func(ApprovedScenarioDelegation) error
}

type ApprovedScenarioDelegation struct {
	ScenarioID       string
	WorktreePath     string
	RequiredEvidence []string
}

type ApprovedScenarioBoundaryRequest struct {
	ScenarioID        string
	ExpectedPaths     []string
	RequiredEvidence  []string
	TDDComplete       bool
	TestsPassed       bool
	ValidationPassed  bool
	Submission        NewImplementationSubmission
	StartNextScenario func(string) error
}

var requiredApprovedScenarioEvidence = []string{
	"Red",
	"Green",
	"Refactor",
	"traceable-test",
	"required-test",
	"active-gate",
	"feature-worktree-identity",
}

// RunNextApprovedScenario delegates exactly the approved next scenario from
// the recorded feature worktree and supplies its required boundary evidence.
func RunNextApprovedScenario(repoRoot string, request ApprovedScenarioRunRequest) (ApprovedPhase3Decision, error) {
	decision, err := BeginApprovedPhase3(repoRoot, ApprovedPhase3Request{ScenarioID: request.ScenarioID})
	if err != nil || !decision.Allowed {
		return decision, err
	}
	if request.Delegate == nil {
		return ApprovedPhase3Decision{}, fmt.Errorf("approved scenario delegation requires a delegate")
	}

	current, err := LoadCurrentSubmission(repoRoot)
	if err != nil {
		return ApprovedPhase3Decision{}, err
	}
	recordedWorktree, err := filepath.EvalSymlinks(current.Manifest.Worktree)
	if err != nil {
		return ApprovedPhase3Decision{}, fmt.Errorf("verify recorded feature-worktree identity: %w", err)
	}
	actualWorktree, err := filepath.EvalSymlinks(repoRoot)
	if err != nil || actualWorktree != recordedWorktree {
		return ApprovedPhase3Decision{}, fmt.Errorf("verify recorded feature-worktree identity before checkpointing")
	}

	delegation := ApprovedScenarioDelegation{
		ScenarioID:       request.ScenarioID,
		WorktreePath:     current.Manifest.Worktree,
		RequiredEvidence: append([]string(nil), requiredApprovedScenarioEvidence...),
	}
	if err := request.Delegate(delegation); err != nil {
		return ApprovedPhase3Decision{}, err
	}
	return decision, nil
}

// CompleteApprovedScenarioBoundary checkpoints one approved scenario and
// advances the recorded workflow only from its clean successful boundary.
func CompleteApprovedScenarioBoundary(repoRoot string, request ApprovedScenarioBoundaryRequest) (CurrentSubmissionState, error) {
	decision, err := BeginApprovedPhase3(repoRoot, ApprovedPhase3Request{ScenarioID: request.ScenarioID})
	if err != nil {
		return CurrentSubmissionState{}, haltedApprovedScenarioBoundary(request.ScenarioID, err)
	}
	if !decision.Allowed {
		return CurrentSubmissionState{}, haltedApprovedScenarioBoundary(request.ScenarioID, fmt.Errorf("%s", decision.Reason))
	}
	current, err := ResumeCurrentSubmission(repoRoot, nil)
	if err != nil {
		return CurrentSubmissionState{}, haltedApprovedScenarioBoundary(request.ScenarioID, err)
	}
	if len(current.State.RemainingWork) < 2 || current.State.RemainingWork[0] != request.ScenarioID {
		return CurrentSubmissionState{}, haltedApprovedScenarioBoundary(request.ScenarioID, fmt.Errorf("approved scenario boundary requires a recorded next scenario"))
	}
	if !containsAllEvidence(request.RequiredEvidence, requiredApprovedScenarioEvidence) {
		return CurrentSubmissionState{}, haltedApprovedScenarioBoundary(request.ScenarioID, fmt.Errorf("approved scenario boundary requires Red, Green, Refactor, traceable-test, required-test, and active-gate evidence"))
	}
	nextScenario := current.State.RemainingWork[1]
	record, err := readRepositoryFile(repoRoot, current.State.ApprovalRecordPath)
	if err != nil || !approvedRecordIncludesScenario(string(record), current.Manifest.FeaturePaths[0], nextScenario) {
		return CurrentSubmissionState{}, haltedApprovedScenarioBoundary(request.ScenarioID, fmt.Errorf("approved scenario boundary requires the recorded next scenario to be approved"))
	}
	checkpoint, err := CheckpointApprovedScenario(repoRoot, ScenarioCheckpointRequest{
		ScenarioID:       request.ScenarioID,
		ExpectedPaths:    request.ExpectedPaths,
		TDDComplete:      request.TDDComplete,
		TestsPassed:      request.TestsPassed,
		ValidationPassed: request.ValidationPassed,
		Submission:       request.Submission,
	})
	if err != nil {
		return CurrentSubmissionState{}, haltedApprovedScenarioBoundary(request.ScenarioID, err)
	}
	state := current.State
	state.CompletedWork = append(state.CompletedWork, request.ScenarioID)
	state.RemainingWork = append([]string(nil), state.RemainingWork[1:]...)
	state.Evidence = append([]string(nil), request.RequiredEvidence...)
	state.Checkpoint = checkpoint.CommitID
	state.NextScenario = nextScenario
	state.LastAction = "checkpointed " + request.ScenarioID
	state.SafeResumePoint = "begin " + nextScenario
	if err := os.WriteFile(current.StatePath, []byte(serializeCurrentSubmissionState(state)), 0o600); err != nil {
		return CurrentSubmissionState{}, haltedApprovedScenarioBoundary(request.ScenarioID, fmt.Errorf("record approved scenario boundary state: %w", err))
	}
	if status, err := gitSubmissionOutput(repoRoot, "status", "--short"); err != nil {
		return CurrentSubmissionState{}, haltedApprovedScenarioBoundary(request.ScenarioID, fmt.Errorf("verify approved scenario boundary cleanliness: %w", err))
	} else if status != "" {
		return CurrentSubmissionState{}, haltedApprovedScenarioBoundary(request.ScenarioID, fmt.Errorf("approved scenario boundary has non-ignored changes: %s", status))
	}
	if request.StartNextScenario != nil {
		if err := request.StartNextScenario(nextScenario); err != nil {
			return CurrentSubmissionState{}, haltedApprovedScenarioBoundary(request.ScenarioID, err)
		}
	}
	return state, nil
}

func haltedApprovedScenarioBoundary(scenarioID string, cause error) error {
	return fmt.Errorf("approved scenario boundary halted for %s: %w; recovery: preserve evidence and user changes, resolve the reported condition, then retry the recorded scenario", scenarioID, cause)
}

// CompleteFinalApprovedScenarioBoundary checkpoints the final approved scenario
// and enters review after its clean recorded-worktree boundary.
func CompleteFinalApprovedScenarioBoundary(repoRoot string, request ApprovedScenarioBoundaryRequest, startReview func() error) (CurrentSubmissionState, error) {
	decision, err := BeginApprovedPhase3(repoRoot, ApprovedPhase3Request{ScenarioID: request.ScenarioID})
	if err != nil {
		return CurrentSubmissionState{}, err
	}
	if !decision.Allowed {
		return CurrentSubmissionState{}, fmt.Errorf("%s", decision.Reason)
	}
	current, err := ResumeCurrentSubmission(repoRoot, nil)
	if err != nil {
		return CurrentSubmissionState{}, err
	}
	if len(current.State.RemainingWork) != 1 || current.State.RemainingWork[0] != request.ScenarioID {
		return CurrentSubmissionState{}, fmt.Errorf("final approved scenario boundary requires the recorded final scenario")
	}
	if !containsAllEvidence(request.RequiredEvidence, requiredApprovedScenarioEvidence) {
		return CurrentSubmissionState{}, fmt.Errorf("final approved scenario boundary requires Red, Green, Refactor, traceable-test, required-test, and active-gate evidence")
	}
	checkpoint, err := CheckpointApprovedScenario(repoRoot, ScenarioCheckpointRequest{
		ScenarioID:       request.ScenarioID,
		ExpectedPaths:    request.ExpectedPaths,
		TDDComplete:      request.TDDComplete,
		TestsPassed:      request.TestsPassed,
		ValidationPassed: request.ValidationPassed,
	})
	if err != nil {
		return CurrentSubmissionState{}, err
	}
	state := current.State
	state.Phase = "Phase 4 review"
	state.CompletedWork = append(state.CompletedWork, request.ScenarioID)
	state.RemainingWork = []string{}
	state.Evidence = append([]string(nil), request.RequiredEvidence...)
	state.Checkpoint = checkpoint.CommitID
	state.NextScenario = ""
	state.LastAction = "checkpointed " + request.ScenarioID
	state.SafeResumePoint = "begin Phase 4 review"
	if err := os.WriteFile(current.StatePath, []byte(serializeCurrentSubmissionState(state)), 0o600); err != nil {
		return CurrentSubmissionState{}, fmt.Errorf("record final approved scenario boundary state: %w", err)
	}
	if status, err := gitSubmissionOutput(repoRoot, "status", "--short"); err != nil {
		return CurrentSubmissionState{}, fmt.Errorf("verify final approved scenario boundary cleanliness: %w", err)
	} else if status != "" {
		return CurrentSubmissionState{}, fmt.Errorf("final approved scenario boundary has non-ignored changes: %s", status)
	}
	if startReview != nil {
		if err := startReview(); err != nil {
			return CurrentSubmissionState{}, err
		}
	}
	return state, nil
}

func containsAllEvidence(actual, required []string) bool {
	for _, evidence := range required {
		if !containsPath(actual, evidence) {
			return false
		}
	}
	return true
}

// BeginApprovedPhase3 proves the recorded feature-scoped approval baseline
// before an implementation scenario can be delegated.
func BeginApprovedPhase3(repoRoot string, request ApprovedPhase3Request) (ApprovedPhase3Decision, error) {
	resume, err := ResumeCurrentSubmission(repoRoot, nil)
	if err != nil {
		return blockedApprovedPhase3("current workflow state cannot be verified"), nil
	}
	if request.ScenarioID == "" || !containsPath(resume.State.RemainingWork, request.ScenarioID) {
		return blockedApprovedPhase3("the next scenario is not recorded in the approved workflow"), nil
	}
	state := resume.State
	if state.BaselineCheckpoint == "" {
		return blockedApprovedPhase3("the approved baseline checkpoint is missing"), nil
	}
	if _, err := gitSubmissionOutput(repoRoot, "cat-file", "-e", state.BaselineCheckpoint+"^{commit}"); err != nil {
		return blockedApprovedPhase3("the approved baseline checkpoint cannot be committed or found"), nil
	}
	if state.ApprovalRecordPath == "" || state.ApprovalRecordFingerprint == "" {
		return blockedApprovedPhase3("the feature-scoped approval record is missing"), nil
	}
	record, err := readRepositoryFile(repoRoot, state.ApprovalRecordPath)
	if err != nil {
		return blockedApprovedPhase3("the feature-scoped approval record is missing"), nil
	}
	recordFingerprint, err := contractFileFingerprint(repoRoot, state.ApprovalRecordPath)
	if err != nil || recordFingerprint != state.ApprovalRecordFingerprint {
		return blockedApprovedPhase3("the feature-scoped approval record does not match its baseline identity"), nil
	}
	if !approvedRecordIncludesScenario(string(record), resume.Manifest.FeaturePaths[0], request.ScenarioID) {
		return blockedApprovedPhase3("the feature-scoped approval record excludes the next scenario"), nil
	}
	contractPaths := approvedContractPaths(resume.Manifest)
	for _, path := range contractPaths {
		fingerprint, err := contractFileFingerprint(repoRoot, path)
		if err != nil || approvedRecordFingerprint(string(record), path) != fingerprint {
			return blockedApprovedPhase3("the approved contract has changed after its baseline checkpoint"), nil
		}
	}
	paths := append(contractPaths, state.ApprovalRecordPath)
	if _, err := gitSubmissionOutput(repoRoot, append([]string{"diff", "--quiet", state.BaselineCheckpoint, "--"}, paths...)...); err != nil {
		return blockedApprovedPhase3("the approved contract has changed after its baseline checkpoint"), nil
	}
	return ApprovedPhase3Decision{Allowed: true, Reason: "matching feature-scoped approved baseline recorded"}, nil
}

func approvedContractPaths(manifest CurrentSubmissionManifest) []string {
	paths := make([]string, 0, len(manifest.FeaturePaths)+1)
	paths = append(paths, manifest.SpecPath)
	return append(paths, manifest.FeaturePaths...)
}

func blockedApprovedPhase3(reason string) ApprovedPhase3Decision {
	return ApprovedPhase3Decision{Reason: "implementation blocked: " + reason + "; recovery: restore or explicitly reapprove and checkpoint the recorded feature contract before Phase 3"}
}

func approvedRecordIncludesScenario(record, featurePath, scenarioID string) bool {
	return strings.Contains(record, "  - "+featurePath+"#"+scenarioID+"\n")
}

func approvedRecordFingerprint(record, path string) string {
	for _, line := range strings.Split(record, "\n") {
		if value, found := strings.CutPrefix(line, "  "+path+": "); found {
			return value
		}
	}
	return ""
}

// CheckpointApprovedContractBaseline records explicit human approval and
// commits exactly the approved contract and its feature-scoped record.
func CheckpointApprovedContractBaseline(repoRoot string, request ApprovedContractBaselineRequest) (ApprovedContractBaseline, error) {
	if request.Submission.WorktreePath != repoRoot || !strings.HasPrefix(request.Submission.FeatureBranch, "feature/") {
		return ApprovedContractBaseline{}, fmt.Errorf("approved contract baseline requires the recorded feature worktree")
	}
	if branch, err := gitSubmissionOutput(repoRoot, "branch", "--show-current"); err != nil || branch != request.Submission.FeatureBranch {
		return ApprovedContractBaseline{}, fmt.Errorf("approved contract baseline requires recorded feature branch %q", request.Submission.FeatureBranch)
	}
	if request.SpecPath == "" || request.FeaturePath == "" || len(request.ApprovedScenarios) == 0 {
		return ApprovedContractBaseline{}, fmt.Errorf("approved contract baseline requires contract paths and approved scenarios")
	}
	if err := verifyCurrentSubmissionBaselineState(repoRoot); err != nil {
		return ApprovedContractBaseline{}, err
	}

	slug := strings.TrimPrefix(request.Submission.FeatureBranch, "feature/")
	recordPath := filepath.ToSlash(filepath.Join("specs", "approvals", slug+".yaml"))
	contractFingerprints, err := approvedContractFingerprints(repoRoot, request.SpecPath, request.FeaturePath)
	if err != nil {
		return ApprovedContractBaseline{}, err
	}
	recordFilePath, err := repositoryFilePath(repoRoot, recordPath)
	if err != nil {
		return ApprovedContractBaseline{}, fmt.Errorf("resolve feature-scoped approval record path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(recordFilePath), 0o750); err != nil {
		return ApprovedContractBaseline{}, fmt.Errorf("create feature-scoped approval directory: %w", err)
	}
	approvedAt := request.ApprovedAt.UTC()
	if approvedAt.IsZero() {
		approvedAt = time.Now().UTC()
	}
	record := serializeApprovedContractRecord(request, approvedAt, recordPath, contractFingerprints)
	if err := os.WriteFile(recordFilePath, []byte(record), 0o600); err != nil {
		return ApprovedContractBaseline{}, fmt.Errorf("write feature-scoped approval record: %w", err)
	}
	recordFingerprint, err := contractFileFingerprint(repoRoot, recordPath)
	if err != nil {
		return ApprovedContractBaseline{}, fmt.Errorf("fingerprint feature-scoped approval record: %w", err)
	}

	paths := []string{request.SpecPath, request.FeaturePath, recordPath}
	if _, err := gitSubmissionOutput(repoRoot, append([]string{"add", "--"}, paths...)...); err != nil {
		return ApprovedContractBaseline{}, fmt.Errorf("stage approved contract baseline: %w", err)
	}
	commitArgs := append([]string{"commit", "--only", "-m", "checkpoint: approved contract baseline", "--"}, paths...)
	if _, err := gitSubmissionOutput(repoRoot, commitArgs...); err != nil {
		return ApprovedContractBaseline{}, fmt.Errorf("create approved contract baseline checkpoint: %w", err)
	}
	commitID, err := gitSubmissionOutput(repoRoot, "rev-parse", "HEAD")
	if err != nil {
		return ApprovedContractBaseline{}, fmt.Errorf("read approved contract baseline checkpoint: %w", err)
	}
	baseline := ApprovedContractBaseline{ApprovalRecordPath: recordPath, ApprovalRecordFingerprint: recordFingerprint, CommitID: commitID}
	if err := recordCurrentSubmissionBaseline(repoRoot, baseline); err != nil {
		return ApprovedContractBaseline{}, err
	}
	return baseline, nil
}

func approvedContractFingerprints(repoRoot, specPath, featurePath string) (map[string]string, error) {
	fingerprints := make(map[string]string, 2)
	for _, path := range []string{specPath, featurePath} {
		fingerprint, err := contractFileFingerprint(repoRoot, path)
		if err != nil {
			return nil, fmt.Errorf("fingerprint approved contract %q: %w", path, err)
		}
		fingerprints[path] = fingerprint
	}
	return fingerprints, nil
}

func contractFileFingerprint(repoRoot, path string) (string, error) {
	contents, err := readRepositoryFile(repoRoot, path)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(contents)), nil
}

func serializeApprovedContractRecord(request ApprovedContractBaselineRequest, approvedAt time.Time, recordPath string, fingerprints map[string]string) string {
	var scenarios strings.Builder
	for _, scenario := range request.ApprovedScenarios {
		scenarios.WriteString("  - ")
		scenarios.WriteString(request.FeaturePath)
		scenarios.WriteString("#")
		scenarios.WriteString(scenario)
		scenarios.WriteString("\n")
	}
	return fmt.Sprintf("format: rotta.feature-approval/v1\nstatus: approved\napproved_on: %s\nsubmission_worktree: %s\nfeature_branch: %s\nbase_branch: %s\napproval_record: %s\napproved_scenarios:\n%scontract_fingerprints:\n  %s: %s\n  %s: %s\n", approvedAt.Format(time.RFC3339), request.Submission.WorktreePath, request.Submission.FeatureBranch, request.Submission.BaseBranch, recordPath, scenarios.String(), request.SpecPath, fingerprints[request.SpecPath], request.FeaturePath, fingerprints[request.FeaturePath])
}

func recordCurrentSubmissionBaseline(repoRoot string, baseline ApprovedContractBaseline) error {
	statePath := filepath.Join(repoRoot, ".rotta", "current", "state.yaml")
	contents, err := readRepositoryFile(repoRoot, filepath.ToSlash(filepath.Join(".rotta", "current", "state.yaml")))
	if err != nil {
		return fmt.Errorf("record approved contract baseline in current workflow state: %w", err)
	}
	state, err := parseCurrentSubmissionState(string(contents))
	if err != nil {
		return fmt.Errorf("record approved contract baseline in current workflow state: %w", err)
	}
	state.BaselineCheckpoint = baseline.CommitID
	state.ApprovalRecordPath = baseline.ApprovalRecordPath
	state.ApprovalRecordFingerprint = baseline.ApprovalRecordFingerprint
	if err := os.WriteFile(statePath, []byte(serializeCurrentSubmissionState(state)), 0o600); err != nil {
		return fmt.Errorf("record approved contract baseline in current workflow state: %w", err)
	}
	return nil
}

func verifyCurrentSubmissionBaselineState(repoRoot string) error {
	contents, err := readRepositoryFile(repoRoot, filepath.ToSlash(filepath.Join(".rotta", "current", "state.yaml")))
	if err != nil {
		return fmt.Errorf("record approved contract baseline in current workflow state: %w", err)
	}
	if _, err := parseCurrentSubmissionState(string(contents)); err != nil {
		return fmt.Errorf("record approved contract baseline in current workflow state: %w", err)
	}
	return nil
}

// repositoryFilePath confines lifecycle artifacts to the recorded worktree.
func repositoryFilePath(repoRoot, path string) (string, error) {
	if path == "" || filepath.IsAbs(path) {
		return "", fmt.Errorf("path must be relative to the recorded worktree")
	}
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", fmt.Errorf("resolve recorded worktree: %w", err)
	}
	filePath := filepath.Join(root, filepath.FromSlash(path))
	relative, err := filepath.Rel(root, filePath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes the recorded worktree")
	}
	return filePath, nil
}
