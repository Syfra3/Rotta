package workflow

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// REQ-045 → SCN-312 → TestSCN312_BeginSpecificationPhaseWritesContractOnlyInRecordedFeatureWorktree
func TestSCN312_BeginSpecificationPhaseWritesContractOnlyInRecordedFeatureWorktree(t *testing.T) {
	// Scenario: Prepare the isolated feature worktree before specification writes
	parent := t.TempDir()
	initiatingWorktree := filepath.Join(parent, "repository")
	if err := os.Mkdir(initiatingWorktree, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, initiatingWorktree, "init", "-b", "main")
	runGit(t, initiatingWorktree, "config", "user.email", "test@example.invalid")
	runGit(t, initiatingWorktree, "config", "user.name", "Test User")
	mustWrite(t, filepath.Join(initiatingWorktree, "README.md"), "base\n")
	runGit(t, initiatingWorktree, "add", "README.md")
	runGit(t, initiatingWorktree, "commit", "-m", "test: establish specification base")

	submission, err := BeginSpecificationPhase(initiatingWorktree, NewImplementationSubmissionRequest{
		Slug:              "feature-lifecycle",
		IntegrationBranch: "main",
	}, func(recordedWorktree string) error {
		mustWrite(t, filepath.Join(recordedWorktree, "specs", "hard_spec.md"), "# Contract\n")
		mustWrite(t, filepath.Join(recordedWorktree, "features", "feature.feature"), "Feature: Contract\n")
		return nil
	})
	if err != nil {
		t.Fatalf("BeginSpecificationPhase returned error: %v", err)
	}

	wantWorktree := filepath.Join(parent, "repository-feature-lifecycle")
	if submission.WorktreePath != wantWorktree || submission.BaseBranch != "main" || submission.FeatureBranch != "feature/feature-lifecycle" {
		t.Fatalf("recorded submission = %#v, want %q on main as feature/feature-lifecycle", submission, wantWorktree)
	}
	for _, path := range []string{"specs/hard_spec.md", "features/feature.feature"} {
		if _, err := os.Stat(filepath.Join(submission.WorktreePath, filepath.FromSlash(path))); err != nil {
			t.Fatalf("recorded worktree is missing contract artifact %q: %v", path, err)
		}
		if _, err := os.Stat(filepath.Join(initiatingWorktree, filepath.FromSlash(path))); !os.IsNotExist(err) {
			t.Fatalf("initiating worktree received contract artifact %q: %v", path, err)
		}
	}
}

// REQ-045 → SCN-312 → TestSCN312_BeginSpecificationPhaseReportsContractWriteFailure
func TestSCN312_BeginSpecificationPhaseReportsContractWriteFailure(t *testing.T) {
	// Scenario: Prepare the isolated feature worktree before specification writes
	parent := t.TempDir()
	initiatingWorktree := filepath.Join(parent, "repository")
	if err := os.Mkdir(initiatingWorktree, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, initiatingWorktree, "init", "-b", "main")
	runGit(t, initiatingWorktree, "config", "user.email", "test@example.invalid")
	runGit(t, initiatingWorktree, "config", "user.name", "Test User")
	mustWrite(t, filepath.Join(initiatingWorktree, "README.md"), "base\n")
	runGit(t, initiatingWorktree, "add", "README.md")
	runGit(t, initiatingWorktree, "commit", "-m", "test: establish specification write failure base")

	submission, err := BeginSpecificationPhase(initiatingWorktree, NewImplementationSubmissionRequest{
		Slug:              "feature-lifecycle",
		IntegrationBranch: "main",
	}, func(string) error {
		return fmt.Errorf("contract storage unavailable")
	})
	if err == nil || !strings.Contains(err.Error(), "write specification contract in recorded feature worktree: contract storage unavailable") {
		t.Fatalf("BeginSpecificationPhase error = %v, want recorded-worktree contract write failure", err)
	}
	if submission != (NewImplementationSubmission{}) {
		t.Fatalf("submission = %#v, want no submission after contract write failure", submission)
	}
	if _, err := os.Stat(filepath.Join(initiatingWorktree, "specs")); !os.IsNotExist(err) {
		t.Fatalf("initiating worktree received specification artifact: %v", err)
	}
}

// REQ-045, REQ-048 → SCN-313 → TestSCN313_PrepareFeatureWorktreeStopsSafelyWhenIsolationIsUnsafe
func TestSCN313_PrepareFeatureWorktreeStopsSafelyWhenIsolationIsUnsafe(t *testing.T) {
	// Scenario: Stop before specification when isolation is unsafe
	for _, testCase := range []struct {
		name       string
		makeUnsafe func(t *testing.T, repo string)
		wantError  string
	}{
		{
			name: "initiating checkout has a non-ignored change",
			makeUnsafe: func(t *testing.T, repo string) {
				mustWrite(t, filepath.Join(repo, "user-change.txt"), "preserve me\n")
			},
			wantError: "initiating worktree has non-ignored changes",
		},
		{
			name: "feature branch cannot be created exclusively",
			makeUnsafe: func(t *testing.T, repo string) {
				runGit(t, repo, "branch", "feature/unsafe-isolation")
			},
			wantError: "feature branch already exists",
		},
		{
			name: "initiating checkout is detached",
			makeUnsafe: func(t *testing.T, repo string) {
				runGit(t, repo, "checkout", "--detach")
			},
			wantError: "detached HEAD",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			parent := t.TempDir()
			repo := filepath.Join(parent, "repository")
			if err := os.Mkdir(repo, 0o755); err != nil {
				t.Fatal(err)
			}
			runGit(t, repo, "init", "-b", "main")
			runGit(t, repo, "config", "user.email", "test@example.invalid")
			runGit(t, repo, "config", "user.name", "Test User")
			mustWrite(t, filepath.Join(repo, "README.md"), "base\n")
			runGit(t, repo, "add", "README.md")
			runGit(t, repo, "commit", "-m", "test: establish unsafe isolation base")
			testCase.makeUnsafe(t, repo)

			submission, err := PrepareNewImplementationSubmission(repo, NewImplementationSubmissionRequest{
				Slug:              "unsafe-isolation",
				IntegrationBranch: "main",
			})
			if err == nil || !strings.Contains(err.Error(), testCase.wantError) || !strings.Contains(err.Error(), "recovery:") {
				t.Fatalf("PrepareNewImplementationSubmission error = %v, want validation %q with a recovery action", err, testCase.wantError)
			}
			if submission != (NewImplementationSubmission{}) {
				t.Fatalf("submission = %#v, want no unsafe submission", submission)
			}
			if _, err := os.Stat(filepath.Join(parent, "repository-unsafe-isolation")); !os.IsNotExist(err) {
				t.Fatalf("unsafe preparation created a feature worktree: %v", err)
			}
			for _, path := range []string{"specs", "features", ".rotta"} {
				if _, err := os.Stat(filepath.Join(repo, path)); !os.IsNotExist(err) {
					t.Fatalf("unsafe preparation wrote submission artifact %q in initiating checkout: %v", path, err)
				}
			}
		})
	}
}

// REQ-045, REQ-048 → SCN-313 → TestSCN313_ReportsRecoveryForEveryPreparationFailure
func TestSCN313_ReportsRecoveryForEveryPreparationFailure(t *testing.T) {
	// Scenario: Stop before specification when isolation is unsafe
	for _, testCase := range []struct {
		name      string
		failure   string
		useBegin  bool
		longPath  bool
		wantError string
	}{
		{name: "initiating checkout cannot resolve", failure: "root", useBegin: true, wantError: "resolve initiating Git worktree"},
		{name: "initiating checkout cannot report status", failure: "status", wantError: "check initiating worktree cleanliness"},
		{name: "repository default branch cannot resolve", failure: "default", wantError: "resolve repository-default integration branch"},
		{name: "configured integration branch cannot resolve", failure: "base", wantError: "resolve integration branch"},
		{name: "feature branch availability cannot be inspected", failure: "branch", wantError: "check feature branch availability"},
		{name: "prescribed worktree path cannot be inspected", failure: "path", longPath: true, wantError: "inspect prescribed worktree path"},
		{name: "worktree ownership cannot be inspected", failure: "worktrees", wantError: "inspect worktree ownership"},
		{name: "isolated worktree cannot be created", failure: "add", wantError: "create isolated feature worktree"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			initiatingWorktree := t.TempDir()
			parent := t.TempDir()
			repoName := "repository"
			if testCase.longPath {
				repoName = strings.Repeat("r", 240)
			}
			repoRoot := filepath.Join(parent, repoName)
			if err := os.Mkdir(repoRoot, 0o755); err != nil {
				t.Fatal(err)
			}
			installSCN313GitFailure(t, testCase.failure, repoRoot)

			request := NewImplementationSubmissionRequest{Slug: "unsafe-isolation", IntegrationBranch: "main"}
			if testCase.failure == "default" {
				request.IntegrationBranch = ""
			}
			var err error
			if testCase.useBegin {
				_, err = BeginSpecificationPhase(initiatingWorktree, request, func(string) error {
					t.Fatal("specification writer was called after unsafe preparation")
					return nil
				})
			} else {
				_, err = PrepareNewImplementationSubmission(initiatingWorktree, request)
			}
			if err == nil || !strings.Contains(err.Error(), testCase.wantError) || !strings.Contains(err.Error(), "recovery:") {
				t.Fatalf("unsafe preparation error = %v, want %q with recovery", err, testCase.wantError)
			}
		})
	}
}

func installSCN313GitFailure(t *testing.T, failure, repoRoot string) {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	script := filepath.Join(bin, "git")
	content := "#!/bin/sh\ncase \"$*\" in\n" +
		"  'rev-parse --show-toplevel') " + failureCase("root", failure, "exit 1", "printf '%s\\n' \"$SCN313_REPO_ROOT\"") + ";;\n" +
		"  'status --short') " + failureCase("status", failure, "exit 1", "exit 0") + ";;\n" +
		"  'symbolic-ref --quiet --short HEAD') printf '%s\\n' main;;\n" +
		"  'symbolic-ref --short refs/remotes/origin/HEAD') " + failureCase("default", failure, "exit 1", "printf '%s\\n' origin/main") + ";;\n" +
		"  'rev-parse --verify main^{commit}') " + failureCase("base", failure, "exit 1", "exit 0") + ";;\n" +
		"  'branch --list --format=%(refname:short) feature/unsafe-isolation') " + failureCase("branch", failure, "exit 1", "exit 0") + ";;\n" +
		"  'worktree list --porcelain') " + failureCase("worktrees", failure, "exit 1", "exit 0") + ";;\n" +
		"  'worktree add -b feature/unsafe-isolation '* ) " + failureCase("add", failure, "exit 1", "exec \""+git+"\" \"$@\"") + ";;\n" +
		"  *) exec \"" + git + "\" \"$@\";;\nesac\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SCN313_REPO_ROOT", repoRoot)
}

func failureCase(expected, actual, failed, succeeded string) string {
	if expected == actual {
		return failed
	}
	return succeeded
}

// REQ-046, REQ-051 → SCN-314 → TestSCN314_CheckpointApprovedFeatureContract
func TestSCN314_CheckpointApprovedFeatureContract(t *testing.T) {
	// Scenario: Checkpoint an explicitly approved feature contract
	repo := prepareSCN248Repository(t)
	runGit(t, repo, "checkout", "-b", "feature/feature-worktree-lifecycle")
	mustWrite(t, filepath.Join(repo, "specs", "hard_spec.md"), "# Approved contract\n")
	mustWrite(t, filepath.Join(repo, "features", "feature_worktree_lifecycle.feature"), "@SCN-314\n")
	if _, err := InitializeCurrentSubmission(repo, CurrentSubmissionRequest{
		ID:           "feature-worktree-lifecycle",
		SpecPath:     "specs/hard_spec.md",
		FeaturePaths: []string{"features/feature_worktree_lifecycle.feature"},
		ScenarioIDs:  []string{"SCN-314"},
	}); err != nil {
		t.Fatalf("InitializeCurrentSubmission returned error: %v", err)
	}

	baseline, err := CheckpointApprovedContractBaseline(repo, ApprovedContractBaselineRequest{
		Submission:        NewImplementationSubmission{WorktreePath: repo, BaseBranch: "main", FeatureBranch: "feature/feature-worktree-lifecycle"},
		SpecPath:          "specs/hard_spec.md",
		FeaturePath:       "features/feature_worktree_lifecycle.feature",
		ApprovedScenarios: []string{"SCN-314"},
		ApprovedAt:        time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("CheckpointApprovedContractBaseline returned error: %v", err)
	}

	if baseline.ApprovalRecordPath != "specs/approvals/feature-worktree-lifecycle.yaml" || baseline.CommitID == "" || baseline.ApprovalRecordFingerprint == "" {
		t.Fatalf("baseline = %#v, want approval identity and checkpoint", baseline)
	}
	record, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(baseline.ApprovalRecordPath)))
	if err != nil {
		t.Fatalf("read approval record: %v", err)
	}
	for _, want := range []string{"status: approved", "approved_scenarios:", "  - features/feature_worktree_lifecycle.feature#SCN-314", "specs/hard_spec.md:", "features/feature_worktree_lifecycle.feature:"} {
		if !strings.Contains(string(record), want) {
			t.Fatalf("approval record missing %q:\n%s", want, record)
		}
	}
	if names := strings.Fields(runGitOutput(t, repo, "show", "--format=", "--name-only", baseline.CommitID)); strings.Join(names, ",") != "features/feature_worktree_lifecycle.feature,specs/approvals/feature-worktree-lifecycle.yaml,specs/hard_spec.md" {
		t.Fatalf("baseline commit files = %v, want only approved contract and record", names)
	}
	state, err := os.ReadFile(filepath.Join(repo, ".rotta", "current", "state.yaml"))
	if err != nil {
		t.Fatalf("read current workflow state: %v", err)
	}
	for _, want := range []string{"baseline_checkpoint: " + baseline.CommitID, "approval_record_path: " + baseline.ApprovalRecordPath, "approval_record_fingerprint: " + baseline.ApprovalRecordFingerprint} {
		if !strings.Contains(string(state), want) {
			t.Fatalf("current state missing %q:\n%s", want, state)
		}
	}
	if _, err := os.Stat(filepath.Join(repo, "specs", ".approved")); !os.IsNotExist(err) {
		t.Fatalf("legacy approval marker was used or modified: %v", err)
	}
}

// REQ-046, REQ-051 → SCN-314 → TestSCN314_RejectsBaselineWithoutCurrentWorkflowState
func TestSCN314_RejectsBaselineWithoutCurrentWorkflowState(t *testing.T) {
	// Scenario: Checkpoint an explicitly approved feature contract
	repo := prepareSCN248Repository(t)
	mustWrite(t, filepath.Join(repo, "specs", "hard_spec.md"), "# Approved contract\n")
	mustWrite(t, filepath.Join(repo, "features", "feature_worktree_lifecycle.feature"), "@SCN-314\n")
	before := runGitOutput(t, repo, "rev-parse", "HEAD")

	_, err := CheckpointApprovedContractBaseline(repo, ApprovedContractBaselineRequest{
		Submission:        NewImplementationSubmission{WorktreePath: repo, BaseBranch: "main", FeatureBranch: "feature/worktree-handoff"},
		SpecPath:          "specs/hard_spec.md",
		FeaturePath:       "features/feature_worktree_lifecycle.feature",
		ApprovedScenarios: []string{"SCN-314"},
		ApprovedAt:        time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC),
	})
	if err == nil || !strings.Contains(err.Error(), "current workflow state") {
		t.Fatalf("CheckpointApprovedContractBaseline error = %v, want current workflow state failure", err)
	}
	if after := runGitOutput(t, repo, "rev-parse", "HEAD"); after != before {
		t.Fatalf("baseline checkpoint advanced HEAD from %q to %q without current workflow state", before, after)
	}
}

// REQ-046, REQ-051 → SCN-314 → TestSCN314_RejectsUnverifiableApprovalInputs
func TestSCN314_RejectsUnverifiableApprovalInputs(t *testing.T) {
	// Scenario: Checkpoint an explicitly approved feature contract
	for _, testCase := range []struct {
		name      string
		configure func(*ApprovedContractBaselineRequest)
		wantError string
	}{
		{
			name: "unrecorded worktree",
			configure: func(request *ApprovedContractBaselineRequest) {
				request.Submission.WorktreePath = filepath.Join(filepath.Dir(request.Submission.WorktreePath), "other-worktree")
			},
			wantError: "recorded feature worktree",
		},
		{
			name: "wrong active feature branch",
			configure: func(request *ApprovedContractBaselineRequest) {
				request.Submission.FeatureBranch = "feature/other"
			},
			wantError: "recorded feature branch",
		},
		{
			name: "incomplete approval scope",
			configure: func(request *ApprovedContractBaselineRequest) {
				request.ApprovedScenarios = nil
			},
			wantError: "contract paths and approved scenarios",
		},
		{
			name: "missing approved hard spec",
			configure: func(request *ApprovedContractBaselineRequest) {
				request.SpecPath = "specs/missing.md"
			},
			wantError: "fingerprint approved contract",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repo := prepareSCN248Repository(t)
			mustWrite(t, filepath.Join(repo, "specs", "hard_spec.md"), "# Approved contract\n")
			mustWrite(t, filepath.Join(repo, "features", "feature_worktree_lifecycle.feature"), "@SCN-314\n")
			if _, err := InitializeCurrentSubmission(repo, CurrentSubmissionRequest{
				ID:           "feature-worktree-lifecycle",
				SpecPath:     "specs/hard_spec.md",
				FeaturePaths: []string{"features/feature_worktree_lifecycle.feature"},
				ScenarioIDs:  []string{"SCN-314"},
			}); err != nil {
				t.Fatalf("InitializeCurrentSubmission returned error: %v", err)
			}
			request := ApprovedContractBaselineRequest{
				Submission:        NewImplementationSubmission{WorktreePath: repo, BaseBranch: "main", FeatureBranch: "feature/worktree-handoff"},
				SpecPath:          "specs/hard_spec.md",
				FeaturePath:       "features/feature_worktree_lifecycle.feature",
				ApprovedScenarios: []string{"SCN-314"},
				ApprovedAt:        time.Date(2026, time.July, 15, 0, 0, 0, 0, time.UTC),
			}
			testCase.configure(&request)

			if _, err := CheckpointApprovedContractBaseline(repo, request); err == nil || !strings.Contains(err.Error(), testCase.wantError) {
				t.Fatalf("CheckpointApprovedContractBaseline error = %v, want %q", err, testCase.wantError)
			}
		})
	}
}

// REQ-046, REQ-048 → SCN-315 → TestSCN315_RefusesImplementationWithoutMatchingApprovedBaseline
func TestSCN315_RefusesImplementationWithoutMatchingApprovedBaseline(t *testing.T) {
	// Scenario: Refuse implementation without a matching approved baseline
	for _, testCase := range []struct {
		name       string
		prepare    func(t *testing.T, repo string, state CurrentSubmissionState)
		wantReason string
	}{
		{
			name:       "no explicit feature-scoped approval record",
			wantReason: "feature-scoped approval record is missing",
			prepare: func(t *testing.T, repo string, state CurrentSubmissionState) {
				mustWrite(t, filepath.Join(repo, "specs", ".approved"), "SCN-315\n")
				if err := os.Remove(filepath.Join(repo, filepath.FromSlash(state.ApprovalRecordPath))); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name:       "approval record excludes the next scenario",
			wantReason: "feature-scoped approval record excludes the next scenario",
			prepare: func(t *testing.T, repo string, state CurrentSubmissionState) {
				mustWrite(t, filepath.Join(repo, filepath.FromSlash(state.ApprovalRecordPath)), "approved_scenarios:\n  - features/feature_worktree_lifecycle.feature#SCN-316\ncontract_fingerprints:\n  specs/hard_spec.md: "+mustContractFingerprint(t, repo, "specs/hard_spec.md")+"\n  features/feature_worktree_lifecycle.feature: "+mustContractFingerprint(t, repo, "features/feature_worktree_lifecycle.feature")+"\n")
				state.ApprovalRecordFingerprint = mustContractFingerprint(t, repo, state.ApprovalRecordPath)
				mustWrite(t, filepath.Join(repo, ".rotta", "current", "state.yaml"), serializeCurrentSubmissionState(state))
			},
		},
		{
			name:       "approval record mismatches its baseline identity",
			wantReason: "feature-scoped approval record does not match its baseline identity",
			prepare: func(t *testing.T, repo string, state CurrentSubmissionState) {
				mustWrite(t, filepath.Join(repo, filepath.FromSlash(state.ApprovalRecordPath)), "approved_scenarios:\n  - features/feature_worktree_lifecycle.feature#SCN-315\ncontract_fingerprints:\n  specs/hard_spec.md: altered\n")
			},
		},
		{
			name:       "contract changed after its approved baseline checkpoint",
			wantReason: "approved contract has changed after its baseline checkpoint",
			prepare: func(t *testing.T, repo string, state CurrentSubmissionState) {
				mustWrite(t, filepath.Join(repo, "features", "feature_worktree_lifecycle.feature"), "@SCN-315\nchanged\n")
			},
		},
		{
			name:       "approval baseline cannot be committed",
			wantReason: "approved baseline checkpoint cannot be committed or found",
			prepare: func(t *testing.T, repo string, state CurrentSubmissionState) {
				state.BaselineCheckpoint = "not-a-commit"
				mustWrite(t, filepath.Join(repo, ".rotta", "current", "state.yaml"), serializeCurrentSubmissionState(state))
			},
		},
		{
			name:       "approval baseline checkpoint is missing",
			wantReason: "approved baseline checkpoint is missing",
			prepare: func(t *testing.T, repo string, state CurrentSubmissionState) {
				state.BaselineCheckpoint = ""
				mustWrite(t, filepath.Join(repo, ".rotta", "current", "state.yaml"), serializeCurrentSubmissionState(state))
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repo, state := prepareSCN315ApprovedBaseline(t)
			testCase.prepare(t, repo, state)

			delegated := false
			checkpointed := false
			decision, err := BeginApprovedPhase3(repo, ApprovedPhase3Request{
				ScenarioID: "SCN-315",
				DelegateScenario: func() error {
					delegated = true
					return nil
				},
				CreateScenarioCheckpoint: func() error {
					checkpointed = true
					return nil
				},
			})
			if err != nil {
				t.Fatalf("BeginApprovedPhase3 returned error: %v", err)
			}
			if decision.Allowed || !strings.Contains(decision.Reason, "implementation blocked") || !strings.Contains(decision.Reason, testCase.wantReason) || !strings.Contains(decision.Reason, "recovery:") {
				t.Fatalf("expected blocked decision with recovery action, got %#v", decision)
			}
			if delegated || checkpointed {
				t.Fatalf("blocked implementation delegated=%t checkpointed=%t", delegated, checkpointed)
			}
		})
	}
}

// REQ-047 → SCN-316 → TestSCN316_DelegatesOnlyTheRecordedApprovedScenarioWithRequiredEvidence
func TestSCN316_DelegatesOnlyTheRecordedApprovedScenarioWithRequiredEvidence(t *testing.T) {
	// Scenario: Run exactly one approved scenario through its required evidence and gate boundary
	repo := prepareSCN316ApprovedBaseline(t)
	delegated := []ApprovedScenarioDelegation{}

	decision, err := RunNextApprovedScenario(repo, ApprovedScenarioRunRequest{
		ScenarioID: "SCN-316",
		Delegate: func(delegation ApprovedScenarioDelegation) error {
			delegated = append(delegated, delegation)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("RunNextApprovedScenario returned error: %v", err)
	}
	if !decision.Allowed || len(delegated) != 1 {
		t.Fatalf("decision=%#v delegated=%#v, want one approved delegation", decision, delegated)
	}
	delegation := delegated[0]
	if delegation.ScenarioID != "SCN-316" || delegation.WorktreePath != repo {
		t.Fatalf("delegation=%#v, want only SCN-316 in recorded worktree %q", delegation, repo)
	}
	for _, evidence := range []string{"Red", "Green", "Refactor", "traceable-test", "required-test", "active-gate", "feature-worktree-identity"} {
		if !containsPath(delegation.RequiredEvidence, evidence) {
			t.Fatalf("delegation evidence=%v, missing %q", delegation.RequiredEvidence, evidence)
		}
	}
}

// REQ-047 → SCN-316 → TestSCN316_StopsUnsafeOrFailedDelegationBeforeEvidenceCanAdvance
func TestSCN316_StopsUnsafeOrFailedDelegationBeforeEvidenceCanAdvance(t *testing.T) {
	// Scenario: Run exactly one approved scenario through its required evidence and gate boundary
	for _, testCase := range []struct {
		name      string
		prepare   func(t *testing.T, repo string)
		delegate  func(ApprovedScenarioDelegation) error
		wantError string
	}{
		{
			name:      "missing delegate",
			wantError: "approved scenario delegation requires a delegate",
		},
		{
			name: "different recorded worktree",
			prepare: func(t *testing.T, repo string) {
				current, err := LoadCurrentSubmission(repo)
				if err != nil {
					t.Fatalf("LoadCurrentSubmission returned error: %v", err)
				}
				current.Manifest.Worktree = t.TempDir()
				mustWrite(t, current.ManifestPath, serializeCurrentSubmissionManifest(current.Manifest))
			},
			delegate:  func(ApprovedScenarioDelegation) error { return nil },
			wantError: "verify recorded feature-worktree identity before checkpointing",
		},
		{
			name: "missing recorded worktree",
			prepare: func(t *testing.T, repo string) {
				current, err := LoadCurrentSubmission(repo)
				if err != nil {
					t.Fatalf("LoadCurrentSubmission returned error: %v", err)
				}
				current.Manifest.Worktree = filepath.Join(t.TempDir(), "missing-worktree")
				mustWrite(t, current.ManifestPath, serializeCurrentSubmissionManifest(current.Manifest))
			},
			delegate:  func(ApprovedScenarioDelegation) error { return nil },
			wantError: "verify recorded feature-worktree identity",
		},
		{
			name:      "delegate failure",
			delegate:  func(ApprovedScenarioDelegation) error { return fmt.Errorf("delegate failed") },
			wantError: "delegate failed",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repo := prepareSCN316ApprovedBaseline(t)
			if testCase.prepare != nil {
				testCase.prepare(t, repo)
			}

			delegated := false
			delegate := testCase.delegate
			if delegate != nil {
				delegate = func(delegation ApprovedScenarioDelegation) error {
					delegated = true
					return testCase.delegate(delegation)
				}
			}
			_, err := RunNextApprovedScenario(repo, ApprovedScenarioRunRequest{ScenarioID: "SCN-316", Delegate: delegate})
			if err == nil || !strings.Contains(err.Error(), testCase.wantError) {
				t.Fatalf("RunNextApprovedScenario error = %v, want %q", err, testCase.wantError)
			}
			if testCase.name == "different recorded worktree" && delegated {
				t.Fatal("delegated after recorded feature-worktree identity failure")
			}
		})
	}
}

// REQ-047 → SCN-316 → TestSCN316_RejectsUnrecordedScenarioBeforeDelegation
func TestSCN316_RejectsUnrecordedScenarioBeforeDelegation(t *testing.T) {
	// Scenario: Run exactly one approved scenario through its required evidence and gate boundary
	repo := prepareSCN316ApprovedBaseline(t)
	delegated := false

	decision, err := RunNextApprovedScenario(repo, ApprovedScenarioRunRequest{
		ScenarioID: "SCN-317",
		Delegate: func(ApprovedScenarioDelegation) error {
			delegated = true
			return nil
		},
	})
	if err != nil {
		t.Fatalf("RunNextApprovedScenario returned error: %v", err)
	}
	if decision.Allowed || !strings.Contains(decision.Reason, "next scenario is not recorded") {
		t.Fatalf("decision=%#v, want unrecorded scenario refusal", decision)
	}
	if delegated {
		t.Fatal("delegated an unrecorded scenario")
	}
}

// REQ-047 → SCN-317 → TestSCN317_CheckpointsAndAdvancesFromCleanSuccessfulBoundary
func TestSCN317_CheckpointsAndAdvancesFromCleanSuccessfulBoundary(t *testing.T) {
	// Scenario: Automatically checkpoint and advance from a clean successful scenario boundary
	repo := prepareSCN317ApprovedBaseline(t)
	mustWrite(t, filepath.Join(repo, "scenario.go"), "package workflow\n\nfunc scenario() { _ = 1 }\n")

	started := ""
	state, err := CompleteApprovedScenarioBoundary(repo, ApprovedScenarioBoundaryRequest{
		ScenarioID:       "SCN-317",
		ExpectedPaths:    []string{"scenario.go"},
		RequiredEvidence: append([]string(nil), requiredApprovedScenarioEvidence...),
		TDDComplete:      true,
		TestsPassed:      true,
		ValidationPassed: true,
		StartNextScenario: func(scenarioID string) error {
			started = scenarioID
			return nil
		},
	})
	if err != nil {
		t.Fatalf("CompleteApprovedScenarioBoundary returned error: %v", err)
	}
	if state.Checkpoint == "" || strings.Join(state.CompletedWork, ",") != "SCN-317" || strings.Join(state.RemainingWork, ",") != "SCN-318" || state.NextScenario != "SCN-318" {
		t.Fatalf("state=%#v, want evidence, checkpoint, SCN-317 completed, and SCN-318 next", state)
	}
	if strings.Join(state.Evidence, ",") != strings.Join(requiredApprovedScenarioEvidence, ",") {
		t.Fatalf("state evidence=%v, want required evidence=%v", state.Evidence, requiredApprovedScenarioEvidence)
	}
	if started != "SCN-318" {
		t.Fatalf("started=%q, want next approved scenario SCN-318", started)
	}
	persisted, err := ResumeCurrentSubmission(repo, nil)
	if err != nil {
		t.Fatalf("ResumeCurrentSubmission returned error: %v", err)
	}
	if persisted.State.Checkpoint != state.Checkpoint || persisted.State.NextScenario != "SCN-318" || strings.Join(persisted.State.Evidence, ",") != strings.Join(requiredApprovedScenarioEvidence, ",") {
		t.Fatalf("persisted state=%#v, want recorded evidence, checkpoint, and next scenario", persisted.State)
	}
	if status := runGitOutput(t, repo, "status", "--short"); status != "" {
		t.Fatalf("checkpoint boundary has non-ignored changes: %q", status)
	}
	if commits := runGitOutput(t, repo, "rev-list", "--count", "HEAD"); commits != "3" {
		t.Fatalf("checkpoint commits=%s, want exactly one local scenario checkpoint", commits)
	}
}

// REQ-047 → SCN-317 → TestSCN317_RejectsIncompleteOrUnapprovedProgressBoundary
func TestSCN317_RejectsIncompleteOrUnapprovedProgressBoundary(t *testing.T) {
	// Scenario: Automatically checkpoint and advance from a clean successful scenario boundary
	for _, testCase := range []struct {
		name               string
		prepare            func(t *testing.T, repo string)
		incompleteEvidence bool
		startNext          func(string) error
		wantError          string
		wantCheckouts      string
	}{
		{
			name: "blocked approved baseline",
			prepare: func(t *testing.T, repo string) {
				current, err := ResumeCurrentSubmission(repo, nil)
				if err != nil {
					t.Fatal(err)
				}
				current.State.BaselineCheckpoint = ""
				mustWrite(t, current.StatePath, serializeCurrentSubmissionState(current.State))
			},
			wantError:     "approved baseline checkpoint is missing",
			wantCheckouts: "2",
		},
		{
			name: "missing progress state",
			prepare: func(t *testing.T, repo string) {
				if err := os.Remove(filepath.Join(repo, ".rotta", "current", "state.yaml")); err != nil {
					t.Fatal(err)
				}
			},
			wantError:     "current workflow state cannot be verified",
			wantCheckouts: "2",
		},
		{
			name: "invalid progress state",
			prepare: func(t *testing.T, repo string) {
				mustWrite(t, filepath.Join(repo, ".rotta", "current", "state.yaml"), "invalid state\n")
			},
			wantError:     "current workflow state cannot be verified",
			wantCheckouts: "2",
		},
		{
			name: "missing recorded feature contract",
			prepare: func(t *testing.T, repo string) {
				if err := os.Remove(filepath.Join(repo, "features", "feature_worktree_lifecycle.feature")); err != nil {
					t.Fatal(err)
				}
			},
			wantError:     "current workflow state cannot be verified",
			wantCheckouts: "2",
		},
		{
			name: "missing approval record identity",
			prepare: func(t *testing.T, repo string) {
				current, err := ResumeCurrentSubmission(repo, nil)
				if err != nil {
					t.Fatal(err)
				}
				current.State.ApprovalRecordPath = ""
				mustWrite(t, current.StatePath, serializeCurrentSubmissionState(current.State))
			},
			wantError:     "feature-scoped approval record is missing",
			wantCheckouts: "2",
		},
		{
			name: "drifted checkpoint contract",
			prepare: func(t *testing.T, repo string) {
				mustWrite(t, filepath.Join(repo, "specs", "hard_spec.md"), "# Drifted contract\n")
				current, err := ResumeCurrentSubmission(repo, nil)
				if err != nil {
					t.Fatal(err)
				}
				record := "approved_scenarios:\n  - features/feature_worktree_lifecycle.feature#SCN-317\n  - features/feature_worktree_lifecycle.feature#SCN-318\ncontract_fingerprints:\n  specs/hard_spec.md: " + mustContractFingerprint(t, repo, "specs/hard_spec.md") + "\n  features/feature_worktree_lifecycle.feature: " + mustContractFingerprint(t, repo, "features/feature_worktree_lifecycle.feature") + "\n"
				mustWrite(t, filepath.Join(repo, filepath.FromSlash(current.State.ApprovalRecordPath)), record)
				current.State.ApprovalRecordFingerprint = mustContractFingerprint(t, repo, current.State.ApprovalRecordPath)
				mustWrite(t, current.StatePath, serializeCurrentSubmissionState(current.State))
			},
			wantError:     "approved contract has changed after its baseline checkpoint",
			wantCheckouts: "2",
		},
		{
			name: "no recorded next scenario",
			prepare: func(t *testing.T, repo string) {
				current, err := ResumeCurrentSubmission(repo, nil)
				if err != nil {
					t.Fatal(err)
				}
				current.State.RemainingWork = []string{"SCN-317"}
				mustWrite(t, current.StatePath, serializeCurrentSubmissionState(current.State))
			},
			wantError:     "requires a recorded next scenario",
			wantCheckouts: "2",
		},
		{
			name:               "incomplete required evidence",
			incompleteEvidence: true,
			wantError:          "requires Red, Green, Refactor",
			wantCheckouts:      "2",
		},
		{
			name: "unapproved recorded next scenario",
			prepare: func(t *testing.T, repo string) {
				current, err := ResumeCurrentSubmission(repo, nil)
				if err != nil {
					t.Fatal(err)
				}
				current.State.RemainingWork = []string{"SCN-317", "SCN-319"}
				mustWrite(t, current.StatePath, serializeCurrentSubmissionState(current.State))
			},
			wantError:     "requires the recorded next scenario to be approved",
			wantCheckouts: "2",
		},
		{
			name: "next scenario start failure after checkpoint",
			prepare: func(t *testing.T, repo string) {
				mustWrite(t, filepath.Join(repo, "scenario.go"), "package workflow\n\nfunc scenario() { _ = 2 }\n")
			},
			startNext:     func(string) error { return fmt.Errorf("next scenario failed") },
			wantError:     "next scenario failed",
			wantCheckouts: "3",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repo := prepareSCN317ApprovedBaseline(t)
			if testCase.prepare != nil {
				testCase.prepare(t, repo)
			}

			requiredEvidence := append([]string(nil), requiredApprovedScenarioEvidence...)
			if testCase.incompleteEvidence {
				requiredEvidence = nil
			}
			_, err := CompleteApprovedScenarioBoundary(repo, ApprovedScenarioBoundaryRequest{
				ScenarioID:        "SCN-317",
				ExpectedPaths:     []string{"scenario.go"},
				RequiredEvidence:  requiredEvidence,
				TDDComplete:       true,
				TestsPassed:       true,
				ValidationPassed:  true,
				StartNextScenario: testCase.startNext,
			})
			if err == nil || !strings.Contains(err.Error(), testCase.wantError) || !strings.Contains(err.Error(), "recovery:") {
				t.Fatalf("CompleteApprovedScenarioBoundary error = %v, want %q with recovery", err, testCase.wantError)
			}
			if commits := runGitOutput(t, repo, "rev-list", "--count", "HEAD"); commits != testCase.wantCheckouts {
				t.Fatalf("checkpoint commits=%s, want %s", commits, testCase.wantCheckouts)
			}
		})
	}
}

// REQ-047 → SCN-318 → TestSCN318_SendsFinalCleanCheckpointToReviewWithoutPublication
func TestSCN318_SendsFinalCleanCheckpointToReviewWithoutPublication(t *testing.T) {
	// Scenario: Send the final clean checkpoint to review without publication
	repo := prepareSCN317ApprovedBaseline(t)
	current, err := ResumeCurrentSubmission(repo, nil)
	if err != nil {
		t.Fatalf("ResumeCurrentSubmission returned error: %v", err)
	}
	state := current.State
	state.CompletedWork = []string{"SCN-317"}
	state.RemainingWork = []string{"SCN-318"}
	state.LastAction = "checkpointed SCN-317"
	state.SafeResumePoint = "begin SCN-318"
	mustWrite(t, current.StatePath, serializeCurrentSubmissionState(state))
	mustWrite(t, filepath.Join(repo, "scenario.go"), "package workflow\n\nfunc scenario() { _ = 2 }\n")

	reviewStarted := false
	state, err = CompleteFinalApprovedScenarioBoundary(repo, ApprovedScenarioBoundaryRequest{
		ScenarioID:       "SCN-318",
		ExpectedPaths:    []string{"scenario.go"},
		RequiredEvidence: append([]string(nil), requiredApprovedScenarioEvidence...),
		TDDComplete:      true,
		TestsPassed:      true,
		ValidationPassed: true,
	}, func() error {
		reviewStarted = true
		return nil
	})
	if err != nil {
		t.Fatalf("CompleteFinalApprovedScenarioBoundary returned error: %v", err)
	}
	if !reviewStarted || state.Phase != "Phase 4 review" || len(state.RemainingWork) != 0 || state.Checkpoint == "" {
		t.Fatalf("state=%#v reviewStarted=%t, want final checkpoint routed only to Phase 4 review", state, reviewStarted)
	}
	if status := runGitOutput(t, repo, "status", "--short"); status != "" {
		t.Fatalf("final checkpoint boundary has non-ignored changes: %q", status)
	}
	if commits := runGitOutput(t, repo, "rev-list", "--count", "HEAD"); commits != "3" {
		t.Fatalf("checkpoint commits=%s, want exactly one final local scenario checkpoint", commits)
	}
}

// REQ-048 → SCN-319 → TestSCN319_HaltsOnRequiredGateFailureWithoutCheckpointing
func TestSCN319_HaltsOnRequiredGateFailureWithoutCheckpointing(t *testing.T) {
	// Scenario: Halt autonomously without discarding evidence or user changes
	repo := prepareSCN317ApprovedBaseline(t)
	mustWrite(t, filepath.Join(repo, "scenario.go"), "package workflow\n\nfunc scenario() { _ = 3 }\n")

	started := false
	_, err := CompleteApprovedScenarioBoundary(repo, ApprovedScenarioBoundaryRequest{
		ScenarioID:       "SCN-317",
		ExpectedPaths:    []string{"scenario.go"},
		RequiredEvidence: append([]string(nil), requiredApprovedScenarioEvidence...),
		TDDComplete:      true,
		TestsPassed:      false,
		ValidationPassed: true,
		StartNextScenario: func(string) error {
			started = true
			return nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "required tests") || !strings.Contains(err.Error(), "recovery:") {
		t.Fatalf("boundary error=%v, want required-test failure with safe recovery", err)
	}
	if started {
		t.Fatal("boundary started the next scenario after a required-test failure")
	}
	if commits := runGitOutput(t, repo, "rev-list", "--count", "HEAD"); commits != "2" {
		t.Fatalf("checkpoint commits=%s, want no checkpoint after failure", commits)
	}
	if status := runGitOutput(t, repo, "status", "--short"); status != "M scenario.go" {
		t.Fatalf("status=%q, want the user change preserved", status)
	}
}

// REQ-048 → SCN-319 → TestSCN319_HaltsOnFeatureWorktreeIdentityFailure
func TestSCN319_HaltsOnFeatureWorktreeIdentityFailure(t *testing.T) {
	// Scenario: Halt autonomously without discarding evidence or user changes
	repo := prepareSCN317ApprovedBaseline(t)
	runGit(t, repo, "checkout", "--detach")

	started := false
	_, err := CompleteApprovedScenarioBoundary(repo, ApprovedScenarioBoundaryRequest{
		ScenarioID:       "SCN-317",
		ExpectedPaths:    []string{"scenario.go"},
		RequiredEvidence: append([]string(nil), requiredApprovedScenarioEvidence...),
		TDDComplete:      true,
		TestsPassed:      true,
		ValidationPassed: true,
		Submission: NewImplementationSubmission{
			WorktreePath:  repo,
			BaseBranch:    "main",
			FeatureBranch: "feature/feature-worktree-lifecycle",
		},
		StartNextScenario: func(string) error {
			started = true
			return nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "attached feature branch") || !strings.Contains(err.Error(), "recovery:") {
		t.Fatalf("boundary error=%v, want worktree identity failure with safe recovery", err)
	}
	if started {
		t.Fatal("boundary started the next scenario after an identity failure")
	}
	if commits := runGitOutput(t, repo, "rev-list", "--count", "HEAD"); commits != "2" {
		t.Fatalf("checkpoint commits=%s, want no checkpoint after identity failure", commits)
	}
}

// REQ-048 → SCN-319 → TestSCN319_PreservesChangesWhenBaselineOrCheckpointGatesFail
func TestSCN319_PreservesChangesWhenBaselineOrCheckpointGatesFail(t *testing.T) {
	// Scenario: Halt autonomously without discarding evidence or user changes
	for _, testCase := range []struct {
		name       string
		prepare    func(t *testing.T, repo string)
		validation bool
		wantError  string
	}{
		{
			name: "objective validation gate failure",
			prepare: func(t *testing.T, repo string) {
				mustWrite(t, filepath.Join(repo, "scenario.go"), "package workflow\n\nfunc scenario() { _ = 4 }\n")
			},
			wantError: "active objective validation",
		},
		{
			name: "unexpected untracked user change",
			prepare: func(t *testing.T, repo string) {
				mustWrite(t, filepath.Join(repo, "ambiguous.txt"), "preserve me\n")
			},
			validation: true,
			wantError:  "unexpected untracked change",
		},
		{
			name: "contract fingerprint drift after approval",
			prepare: func(t *testing.T, repo string) {
				mustWrite(t, filepath.Join(repo, "specs", "hard_spec.md"), "# Drifted contract\n")
			},
			validation: true,
			wantError:  "approved contract has changed",
		},
		{
			name: "approval record differs from its baseline checkpoint",
			prepare: func(t *testing.T, repo string) {
				current, err := ResumeCurrentSubmission(repo, nil)
				if err != nil {
					t.Fatal(err)
				}
				recordPath := filepath.Join(repo, filepath.FromSlash(current.State.ApprovalRecordPath))
				record, err := os.ReadFile(recordPath)
				if err != nil {
					t.Fatal(err)
				}
				mustWrite(t, recordPath, string(record)+"# changed after approval\n")
				current.State.ApprovalRecordFingerprint = mustContractFingerprint(t, repo, current.State.ApprovalRecordPath)
				mustWrite(t, current.StatePath, serializeCurrentSubmissionState(current.State))
			},
			validation: true,
			wantError:  "approved contract has changed",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repo := prepareSCN317ApprovedBaseline(t)
			testCase.prepare(t, repo)

			started := false
			_, err := CompleteApprovedScenarioBoundary(repo, ApprovedScenarioBoundaryRequest{
				ScenarioID:       "SCN-317",
				ExpectedPaths:    []string{"scenario.go"},
				RequiredEvidence: append([]string(nil), requiredApprovedScenarioEvidence...),
				TDDComplete:      true,
				TestsPassed:      true,
				ValidationPassed: testCase.validation,
				StartNextScenario: func(string) error {
					started = true
					return nil
				},
			})
			if err == nil || !strings.Contains(err.Error(), testCase.wantError) || !strings.Contains(err.Error(), "recovery:") {
				t.Fatalf("boundary error=%v, want %q with safe recovery", err, testCase.wantError)
			}
			if started {
				t.Fatal("boundary started the next scenario after a fail-closed error")
			}
			if commits := runGitOutput(t, repo, "rev-list", "--count", "HEAD"); commits != "2" {
				t.Fatalf("checkpoint commits=%s, want no checkpoint after failure", commits)
			}
			if status := runGitOutput(t, repo, "status", "--short"); status == "" {
				t.Fatal("failure discarded the user or contract change")
			}
		})
	}
}

// REQ-048 → SCN-319 → TestSCN319_RejectsUnsafeRecordedWorktreeOrBranchVariants
func TestSCN319_RejectsUnsafeRecordedWorktreeOrBranchVariants(t *testing.T) {
	// Scenario: Halt autonomously without discarding evidence or user changes
	for _, testCase := range []struct {
		name    string
		prepare func(t *testing.T, repo string, submission *NewImplementationSubmission)
	}{
		{
			name: "missing recorded worktree",
			prepare: func(t *testing.T, repo string, submission *NewImplementationSubmission) {
				submission.WorktreePath = filepath.Join(repo, "missing-worktree")
			},
		},
		{
			name: "different recorded worktree",
			prepare: func(t *testing.T, _ string, submission *NewImplementationSubmission) {
				submission.WorktreePath = t.TempDir()
			},
		},
		{
			name: "wrong recorded feature branch",
			prepare: func(_ *testing.T, _ string, submission *NewImplementationSubmission) {
				submission.FeatureBranch = "feature/other-worktree"
			},
		},
		{
			name: "feature branch recorded as base branch",
			prepare: func(t *testing.T, repo string, submission *NewImplementationSubmission) {
				submission.BaseBranch = runGitOutput(t, repo, "branch", "--show-current")
			},
		},
		{
			name: "non feature branch",
			prepare: func(t *testing.T, repo string, submission *NewImplementationSubmission) {
				runGit(t, repo, "checkout", "-b", "scenario-checkpoint")
				submission.FeatureBranch = "scenario-checkpoint"
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repo := prepareSCN317ApprovedBaseline(t)
			submission := NewImplementationSubmission{
				WorktreePath:  repo,
				BaseBranch:    "main",
				FeatureBranch: "feature/feature-worktree-lifecycle",
			}
			testCase.prepare(t, repo, &submission)

			started := false
			_, err := CompleteApprovedScenarioBoundary(repo, ApprovedScenarioBoundaryRequest{
				ScenarioID:       "SCN-317",
				ExpectedPaths:    []string{"scenario.go"},
				RequiredEvidence: append([]string(nil), requiredApprovedScenarioEvidence...),
				TDDComplete:      true,
				TestsPassed:      true,
				ValidationPassed: true,
				Submission:       submission,
				StartNextScenario: func(string) error {
					started = true
					return nil
				},
			})
			if err == nil || !strings.Contains(err.Error(), "recovery:") {
				t.Fatalf("boundary error=%v, want identity failure with safe recovery", err)
			}
			if started {
				t.Fatal("boundary started the next scenario after an identity failure")
			}
			if commits := runGitOutput(t, repo, "rev-list", "--count", "HEAD"); commits != "2" {
				t.Fatalf("checkpoint commits=%s, want no checkpoint after identity failure", commits)
			}
		})
	}
}

// REQ-049 → SCN-320 → TestSCN320_ArchivesTerminalReviewStateAndRetainsFeatureWorktree
func TestSCN320_ArchivesTerminalReviewStateAndRetainsFeatureWorktree(t *testing.T) {
	// Scenario: Archive terminal state while retaining the reviewable feature worktree
	repo := prepareSCN317ApprovedBaseline(t)
	mustWrite(t, filepath.Join(repo, ".rotta", "current", "manifest.yaml"), "submission_id: feature-worktree-lifecycle\nspec_path: specs/hard_spec.md\nfeature_paths:\n  - features/feature_worktree_lifecycle.feature\nscenario_ids:\n  - SCN-317\n  - SCN-318\nworktree: "+repo+"\nstatus: review_failed\n")

	if err := ArchiveTerminalFeatureWorkflow(repo); err != nil {
		t.Fatalf("ArchiveTerminalFeatureWorkflow returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(repo, ".rotta", "current")); !os.IsNotExist(err) {
		t.Fatalf("active execution state remains after terminal archive: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".rotta", "archive", "feature-worktree-lifecycle", "manifest.yaml")); err != nil {
		t.Fatalf("terminal execution state was not archived: %v", err)
	}
	if branch := runGitOutput(t, repo, "branch", "--show-current"); branch != "feature/feature-worktree-lifecycle" {
		t.Fatalf("feature branch = %q, want retained reviewable feature branch", branch)
	}
	for _, path := range []string{"specs/hard_spec.md", "features/feature_worktree_lifecycle.feature", "specs/approvals/feature-worktree-lifecycle.yaml"} {
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(path))); err != nil {
			t.Fatalf("retained committed contract artifact %q is missing: %v", path, err)
		}
	}
}

// REQ-049 → SCN-320 → TestSCN320_ArchivesTerminalStatesOnlyFromRecordedFeatureWorktree
func TestSCN320_ArchivesTerminalStatesOnlyFromRecordedFeatureWorktree(t *testing.T) {
	// Scenario: Archive terminal state while retaining the reviewable feature worktree
	for _, testCase := range []struct {
		name      string
		status    string
		worktree  func(t *testing.T, repo string) string
		wantError string
	}{
		{name: "completed", status: "completed"},
		{name: "abandoned", status: "abandoned"},
		{name: "cancelled", status: "cancelled"},
		{
			name:      "different recorded worktree",
			status:    "completed",
			worktree:  func(t *testing.T, _ string) string { return t.TempDir() },
			wantError: "recorded feature worktree",
		},
		{
			name:      "non-terminal result",
			status:    "in_progress",
			wantError: "terminal review result",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repo := prepareSCN317ApprovedBaseline(t)
			recordedWorktree := repo
			if testCase.worktree != nil {
				recordedWorktree = testCase.worktree(t, repo)
			}
			mustWrite(t, filepath.Join(repo, ".rotta", "current", "manifest.yaml"), "submission_id: feature-worktree-lifecycle\nspec_path: specs/hard_spec.md\nfeature_paths:\n  - features/feature_worktree_lifecycle.feature\nscenario_ids:\n  - SCN-317\nworktree: "+recordedWorktree+"\nstatus: "+testCase.status+"\n")

			err := ArchiveTerminalFeatureWorkflow(repo)
			if testCase.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), testCase.wantError) {
					t.Fatalf("ArchiveTerminalFeatureWorkflow error = %v, want %q", err, testCase.wantError)
				}
				if _, statErr := os.Stat(filepath.Join(repo, ".rotta", "current")); statErr != nil {
					t.Fatalf("rejected archive removed active execution state: %v", statErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ArchiveTerminalFeatureWorkflow returned error: %v", err)
			}
			if _, statErr := os.Stat(filepath.Join(repo, ".rotta", "archive", "feature-worktree-lifecycle", "manifest.yaml")); statErr != nil {
				t.Fatalf("terminal execution state was not archived: %v", statErr)
			}
			if branch := runGitOutput(t, repo, "branch", "--show-current"); branch != "feature/feature-worktree-lifecycle" {
				t.Fatalf("feature branch = %q, want retained reviewable feature branch", branch)
			}
		})
	}
}

// REQ-049 → SCN-321 → TestSCN321_RemovesEligibleFeatureWorktreeOnlyAfterExplicitCleanup
func TestSCN321_RemovesEligibleFeatureWorktreeOnlyAfterExplicitCleanup(t *testing.T) {
	// Scenario: Remove a feature worktree only through eligible explicit cleanup
	for _, terminalStatus := range []string{"published", "abandoned", "cancelled"} {
		t.Run(terminalStatus, func(t *testing.T) {
			initiatingWorktree := filepath.Join(t.TempDir(), "initiating")
			if err := os.Mkdir(initiatingWorktree, 0o755); err != nil {
				t.Fatalf("create initiating worktree: %v", err)
			}
			runGit(t, initiatingWorktree, "init", "-b", "main")
			runGit(t, initiatingWorktree, "config", "user.email", "test@example.invalid")
			runGit(t, initiatingWorktree, "config", "user.name", "Test User")
			mustWrite(t, filepath.Join(initiatingWorktree, "README.md"), "base\n")
			mustWrite(t, filepath.Join(initiatingWorktree, ".gitignore"), ".rotta/\n")
			runGit(t, initiatingWorktree, "add", "README.md", ".gitignore")
			runGit(t, initiatingWorktree, "commit", "-m", "test: establish cleanup base")

			submission, err := PrepareNewImplementationSubmission(initiatingWorktree, NewImplementationSubmissionRequest{Slug: "feature-worktree-lifecycle", IntegrationBranch: "main"})
			if err != nil {
				t.Fatalf("PrepareNewImplementationSubmission returned error: %v", err)
			}
			mustWrite(t, filepath.Join(submission.WorktreePath, "specs", "hard_spec.md"), "# Approved contract\n")
			mustWrite(t, filepath.Join(submission.WorktreePath, "features", "feature_worktree_lifecycle.feature"), "@SCN-321\n")
			runGit(t, submission.WorktreePath, "add", "specs/hard_spec.md", "features/feature_worktree_lifecycle.feature")
			runGit(t, submission.WorktreePath, "commit", "-m", "test: retain cleanup contract")
			if _, err := InitializeCurrentSubmission(submission.WorktreePath, CurrentSubmissionRequest{ID: "feature-worktree-lifecycle", SpecPath: "specs/hard_spec.md", FeaturePaths: []string{"features/feature_worktree_lifecycle.feature"}, ScenarioIDs: []string{"SCN-321"}}); err != nil {
				t.Fatalf("InitializeCurrentSubmission returned error: %v", err)
			}
			mustWrite(t, filepath.Join(submission.WorktreePath, ".rotta", "current", "manifest.yaml"), "submission_id: feature-worktree-lifecycle\nspec_path: specs/hard_spec.md\nfeature_paths:\n  - features/feature_worktree_lifecycle.feature\nscenario_ids:\n  - SCN-321\nworktree: "+submission.WorktreePath+"\nstatus: "+terminalStatus+"\n")

			if err := CleanupTerminalFeatureWorktree(submission.WorktreePath); err != nil {
				t.Fatalf("CleanupTerminalFeatureWorktree returned error: %v", err)
			}

			if _, err := os.Stat(submission.WorktreePath); !os.IsNotExist(err) {
				t.Fatalf("recorded feature worktree remains after explicit cleanup: %v", err)
			}
			if _, err := os.Stat(initiatingWorktree); err != nil {
				t.Fatalf("initiating checkout was removed: %v", err)
			}
			if content := runGitOutput(t, initiatingWorktree, "show", submission.FeatureBranch+":specs/hard_spec.md"); content != "# Approved contract" {
				t.Fatalf("durable hard spec = %q, want retained feature-branch contract", content)
			}
			if content := runGitOutput(t, initiatingWorktree, "show", submission.FeatureBranch+":features/feature_worktree_lifecycle.feature"); content != "@SCN-321" {
				t.Fatalf("durable feature contract = %q, want retained feature-branch contract", content)
			}
		})
	}
}

// REQ-049 → SCN-321 → TestSCN321_RejectsEligibleCleanupWhenValidationFails
func TestSCN321_RejectsEligibleCleanupWhenValidationFails(t *testing.T) {
	// Scenario: Remove a feature worktree only through eligible explicit cleanup
	for _, testCase := range []struct {
		name      string
		setup     func(t *testing.T, repo string) string
		wantError string
	}{
		{
			name: "recorded worktree cannot be resolved",
			setup: func(t *testing.T, repo string) string {
				return filepath.Join(t.TempDir(), "missing-worktree")
			},
			wantError: "resolve recorded feature worktree",
		},
		{
			name: "cleanup is requested from another worktree",
			setup: func(t *testing.T, repo string) string {
				return t.TempDir()
			},
			wantError: "requires the recorded feature worktree",
		},
		{
			name: "recorded checkout is not attached to its feature branch",
			setup: func(t *testing.T, repo string) string {
				runGit(t, repo, "checkout", "main")
				return repo
			},
			wantError: "attached feature branch",
		},
		{
			name: "recorded checkout no longer provides Git status",
			setup: func(t *testing.T, repo string) string {
				if err := os.RemoveAll(filepath.Join(repo, ".git")); err != nil {
					t.Fatalf("remove test Git directory: %v", err)
				}
				return repo
			},
			wantError: "check recorded feature worktree cleanliness",
		},
		{
			name: "primary feature checkout cannot be removed as a worktree",
			setup: func(t *testing.T, repo string) string {
				return repo
			},
			wantError: "remove recorded feature worktree",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repo := prepareSCN321EligibleCleanupRepository(t)
			recordedWorktree := testCase.setup(t, repo)
			mustWrite(t, filepath.Join(repo, ".rotta", "current", "manifest.yaml"), "submission_id: feature-worktree-lifecycle\nspec_path: specs/hard_spec.md\nfeature_paths:\n  - features/feature_worktree_lifecycle.feature\nscenario_ids:\n  - SCN-321\nworktree: "+recordedWorktree+"\nstatus: published\n")

			err := CleanupTerminalFeatureWorktree(repo)
			if err == nil || !strings.Contains(err.Error(), testCase.wantError) {
				t.Fatalf("CleanupTerminalFeatureWorktree error = %v, want %q", err, testCase.wantError)
			}
			if _, statErr := os.Stat(repo); statErr != nil {
				t.Fatalf("rejected cleanup removed its checkout: %v", statErr)
			}
		})
	}
}

func prepareSCN321EligibleCleanupRepository(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")
	mustWrite(t, filepath.Join(repo, ".gitignore"), ".rotta/\n")
	mustWrite(t, filepath.Join(repo, "specs", "hard_spec.md"), "# Approved contract\n")
	mustWrite(t, filepath.Join(repo, "features", "feature_worktree_lifecycle.feature"), "@SCN-321\n")
	runGit(t, repo, "add", ".gitignore", "specs/hard_spec.md", "features/feature_worktree_lifecycle.feature")
	runGit(t, repo, "commit", "-m", "test: establish eligible cleanup validation")
	runGit(t, repo, "checkout", "-b", "feature/feature-worktree-lifecycle")
	if _, err := InitializeCurrentSubmission(repo, CurrentSubmissionRequest{ID: "feature-worktree-lifecycle", SpecPath: "specs/hard_spec.md", FeaturePaths: []string{"features/feature_worktree_lifecycle.feature"}, ScenarioIDs: []string{"SCN-321"}}); err != nil {
		t.Fatalf("InitializeCurrentSubmission returned error: %v", err)
	}
	return repo
}

// REQ-049 → SCN-322 → TestSCN322_RefusesPrematureOrUnsafeFeatureWorktreeCleanup
func TestSCN322_RefusesPrematureOrUnsafeFeatureWorktreeCleanup(t *testing.T) {
	// Scenario: Refuse premature or unsafe feature-worktree cleanup
	for _, testCase := range []struct {
		name          string
		terminalState string
		dirty         bool
		wantError     string
	}{
		{
			name:          "reviewed but unpublished",
			terminalState: "completed",
			wantError:     "publication confirmation or explicit abandonment is required",
		},
		{
			name:          "unexpected non-ignored changes",
			terminalState: "published",
			dirty:         true,
			wantError:     "non-ignored changes",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			initiatingWorktree := filepath.Join(t.TempDir(), "initiating")
			if err := os.Mkdir(initiatingWorktree, 0o755); err != nil {
				t.Fatalf("create initiating worktree: %v", err)
			}
			runGit(t, initiatingWorktree, "init", "-b", "main")
			runGit(t, initiatingWorktree, "config", "user.email", "test@example.invalid")
			runGit(t, initiatingWorktree, "config", "user.name", "Test User")
			mustWrite(t, filepath.Join(initiatingWorktree, "README.md"), "base\n")
			mustWrite(t, filepath.Join(initiatingWorktree, ".gitignore"), ".rotta/\n")
			runGit(t, initiatingWorktree, "add", "README.md", ".gitignore")
			runGit(t, initiatingWorktree, "commit", "-m", "test: establish cleanup refusal base")

			submission, err := PrepareNewImplementationSubmission(initiatingWorktree, NewImplementationSubmissionRequest{Slug: "feature-worktree-lifecycle", IntegrationBranch: "main"})
			if err != nil {
				t.Fatalf("PrepareNewImplementationSubmission returned error: %v", err)
			}
			mustWrite(t, filepath.Join(submission.WorktreePath, "specs", "hard_spec.md"), "# Approved contract\n")
			mustWrite(t, filepath.Join(submission.WorktreePath, "features", "feature_worktree_lifecycle.feature"), "@SCN-322\n")
			runGit(t, submission.WorktreePath, "add", "specs/hard_spec.md", "features/feature_worktree_lifecycle.feature")
			runGit(t, submission.WorktreePath, "commit", "-m", "test: retain cleanup refusal contract")
			if _, err := InitializeCurrentSubmission(submission.WorktreePath, CurrentSubmissionRequest{ID: "feature-worktree-lifecycle", SpecPath: "specs/hard_spec.md", FeaturePaths: []string{"features/feature_worktree_lifecycle.feature"}, ScenarioIDs: []string{"SCN-322"}}); err != nil {
				t.Fatalf("InitializeCurrentSubmission returned error: %v", err)
			}
			mustWrite(t, filepath.Join(submission.WorktreePath, ".rotta", "current", "manifest.yaml"), "submission_id: feature-worktree-lifecycle\nspec_path: specs/hard_spec.md\nfeature_paths:\n  - features/feature_worktree_lifecycle.feature\nscenario_ids:\n  - SCN-322\nworktree: "+submission.WorktreePath+"\nstatus: "+testCase.terminalState+"\n")
			if testCase.dirty {
				mustWrite(t, filepath.Join(submission.WorktreePath, "unexpected.txt"), "preserve me\n")
			}

			err = CleanupTerminalFeatureWorktree(submission.WorktreePath)
			if err == nil || !strings.Contains(err.Error(), testCase.wantError) {
				t.Fatalf("CleanupTerminalFeatureWorktree error = %v, want %q", err, testCase.wantError)
			}
			if _, err := os.Stat(submission.WorktreePath); err != nil {
				t.Fatalf("recorded feature worktree was removed: %v", err)
			}
			if branch := runGitOutput(t, submission.WorktreePath, "branch", "--show-current"); branch != submission.FeatureBranch {
				t.Fatalf("feature branch = %q, want preserved %q", branch, submission.FeatureBranch)
			}
		})
	}
}

// REQ-049 → SCN-322 → TestSCN322_RefusesCleanupWithoutUsableCurrentSubmission
func TestSCN322_RefusesCleanupWithoutUsableCurrentSubmission(t *testing.T) {
	// Scenario: Refuse premature or unsafe feature-worktree cleanup
	repo := prepareSCN321EligibleCleanupRepository(t)
	if err := os.Remove(filepath.Join(repo, ".rotta", "current", "manifest.yaml")); err != nil {
		t.Fatalf("remove current submission manifest: %v", err)
	}

	err := CleanupTerminalFeatureWorktree(repo)
	if err == nil || !strings.Contains(err.Error(), "current submission state cannot be safely used") {
		t.Fatalf("CleanupTerminalFeatureWorktree error = %v, want unusable current-submission refusal", err)
	}
	if _, err := os.Stat(repo); err != nil {
		t.Fatalf("unsafe cleanup removed recorded feature worktree: %v", err)
	}
	if branch := runGitOutput(t, repo, "branch", "--show-current"); branch != "feature/feature-worktree-lifecycle" {
		t.Fatalf("feature branch = %q, want preserved feature worktree branch", branch)
	}
}

func prepareSCN316ApprovedBaseline(t *testing.T) string {
	t.Helper()
	repo := prepareSCN248Repository(t)
	mustWrite(t, filepath.Join(repo, "specs", "hard_spec.md"), "# Approved contract\n")
	mustWrite(t, filepath.Join(repo, "features", "feature_worktree_lifecycle.feature"), "@SCN-316\n")
	recordPath := "specs/approvals/feature-worktree-lifecycle.yaml"
	mustWrite(t, filepath.Join(repo, filepath.FromSlash(recordPath)), "approved_scenarios:\n  - features/feature_worktree_lifecycle.feature#SCN-316\ncontract_fingerprints:\n  specs/hard_spec.md: "+mustContractFingerprint(t, repo, "specs/hard_spec.md")+"\n  features/feature_worktree_lifecycle.feature: "+mustContractFingerprint(t, repo, "features/feature_worktree_lifecycle.feature")+"\n")
	runGit(t, repo, "add", "specs/hard_spec.md", "features/feature_worktree_lifecycle.feature", recordPath)
	runGit(t, repo, "commit", "-m", "test: checkpoint approved SCN-316 contract")
	if _, err := InitializeCurrentSubmission(repo, CurrentSubmissionRequest{ID: "feature-worktree-lifecycle", SpecPath: "specs/hard_spec.md", FeaturePaths: []string{"features/feature_worktree_lifecycle.feature"}, ScenarioIDs: []string{"SCN-316"}}); err != nil {
		t.Fatalf("InitializeCurrentSubmission returned error: %v", err)
	}
	state := CurrentSubmissionState{Phase: "implementation", CompletedWork: []string{}, RemainingWork: []string{"SCN-316"}, BlockedWork: []string{}, LastAction: "ready for approved scenario", SafeResumePoint: "begin implementation", BaselineCheckpoint: runGitOutput(t, repo, "rev-parse", "HEAD"), ApprovalRecordPath: recordPath, ApprovalRecordFingerprint: mustContractFingerprint(t, repo, recordPath)}
	mustWrite(t, filepath.Join(repo, ".rotta", "current", "state.yaml"), serializeCurrentSubmissionState(state))
	return repo
}

func prepareSCN317ApprovedBaseline(t *testing.T) string {
	t.Helper()
	repo := prepareSCN248Repository(t)
	runGit(t, repo, "checkout", "-b", "feature/feature-worktree-lifecycle")
	mustWrite(t, filepath.Join(repo, ".gitignore"), ".rotta/\n")
	mustWrite(t, filepath.Join(repo, "specs", "hard_spec.md"), "# Approved contract\n")
	mustWrite(t, filepath.Join(repo, "features", "feature_worktree_lifecycle.feature"), "@SCN-317\n@SCN-318\n")
	mustWrite(t, filepath.Join(repo, "scenario.go"), "package workflow\n\nfunc scenario() {}\n")
	recordPath := "specs/approvals/feature-worktree-lifecycle.yaml"
	mustWrite(t, filepath.Join(repo, filepath.FromSlash(recordPath)), "approved_scenarios:\n  - features/feature_worktree_lifecycle.feature#SCN-317\n  - features/feature_worktree_lifecycle.feature#SCN-318\ncontract_fingerprints:\n  specs/hard_spec.md: "+mustContractFingerprint(t, repo, "specs/hard_spec.md")+"\n  features/feature_worktree_lifecycle.feature: "+mustContractFingerprint(t, repo, "features/feature_worktree_lifecycle.feature")+"\n")
	runGit(t, repo, "add", ".gitignore", "specs/hard_spec.md", "features/feature_worktree_lifecycle.feature", "scenario.go", recordPath)
	runGit(t, repo, "commit", "-m", "test: checkpoint approved SCN-317 contract")
	if _, err := InitializeCurrentSubmission(repo, CurrentSubmissionRequest{ID: "feature-worktree-lifecycle", SpecPath: "specs/hard_spec.md", FeaturePaths: []string{"features/feature_worktree_lifecycle.feature"}, ScenarioIDs: []string{"SCN-317", "SCN-318"}}); err != nil {
		t.Fatalf("InitializeCurrentSubmission returned error: %v", err)
	}
	state := CurrentSubmissionState{Phase: "implementation", CompletedWork: []string{}, RemainingWork: []string{"SCN-317", "SCN-318"}, BlockedWork: []string{}, LastAction: "ready for approved scenario", SafeResumePoint: "begin implementation", BaselineCheckpoint: runGitOutput(t, repo, "rev-parse", "HEAD"), ApprovalRecordPath: recordPath, ApprovalRecordFingerprint: mustContractFingerprint(t, repo, recordPath)}
	mustWrite(t, filepath.Join(repo, ".rotta", "current", "state.yaml"), serializeCurrentSubmissionState(state))
	return repo
}

func prepareSCN315ApprovedBaseline(t *testing.T) (string, CurrentSubmissionState) {
	t.Helper()
	repo := prepareSCN248Repository(t)
	mustWrite(t, filepath.Join(repo, "specs", "hard_spec.md"), "# Approved contract\n")
	mustWrite(t, filepath.Join(repo, "features", "feature_worktree_lifecycle.feature"), "@SCN-315\n")
	recordPath := "specs/approvals/feature-worktree-lifecycle.yaml"
	mustWrite(t, filepath.Join(repo, filepath.FromSlash(recordPath)), "approved_scenarios:\n  - features/feature_worktree_lifecycle.feature#SCN-315\ncontract_fingerprints:\n  specs/hard_spec.md: "+mustContractFingerprint(t, repo, "specs/hard_spec.md")+"\n  features/feature_worktree_lifecycle.feature: "+mustContractFingerprint(t, repo, "features/feature_worktree_lifecycle.feature")+"\n")
	runGit(t, repo, "add", "specs/hard_spec.md", "features/feature_worktree_lifecycle.feature", recordPath)
	runGit(t, repo, "commit", "-m", "test: checkpoint approved contract")
	if _, err := InitializeCurrentSubmission(repo, CurrentSubmissionRequest{ID: "feature-worktree-lifecycle", SpecPath: "specs/hard_spec.md", FeaturePaths: []string{"features/feature_worktree_lifecycle.feature"}, ScenarioIDs: []string{"SCN-315"}}); err != nil {
		t.Fatalf("InitializeCurrentSubmission returned error: %v", err)
	}
	state := CurrentSubmissionState{Phase: "implementation", CompletedWork: []string{}, RemainingWork: []string{"SCN-315"}, BlockedWork: []string{}, LastAction: "initialized current submission", SafeResumePoint: "begin implementation", BaselineCheckpoint: runGitOutput(t, repo, "rev-parse", "HEAD"), ApprovalRecordPath: recordPath, ApprovalRecordFingerprint: mustContractFingerprint(t, repo, recordPath)}
	mustWrite(t, filepath.Join(repo, ".rotta", "current", "state.yaml"), serializeCurrentSubmissionState(state))
	return repo, state
}

func mustContractFingerprint(t *testing.T, repo, path string) string {
	t.Helper()
	fingerprint, err := contractFileFingerprint(filepath.Join(repo, filepath.FromSlash(path)))
	if err != nil {
		t.Fatalf("fingerprint %q: %v", path, err)
	}
	return fingerprint
}

// REQ-037, REQ-038 → SCN-241 → TestSCN241_PrepareNewImplementationSubmissionCreatesIsolatedFeatureWorktree
func TestSCN241_PrepareNewImplementationSubmissionCreatesIsolatedFeatureWorktree(t *testing.T) {
	// Scenario: Create an isolated feature worktree before Phase 2 writes a contract
	parent := t.TempDir()
	repo := filepath.Join(parent, "repository")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")
	mustWrite(t, filepath.Join(repo, "README.md"), "base\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "test: establish integration base")

	submission, err := PrepareNewImplementationSubmission(repo, NewImplementationSubmissionRequest{
		Slug:              "worktree-handoff",
		IntegrationBranch: "main",
	})
	if err != nil {
		t.Fatalf("PrepareNewImplementationSubmission returned error: %v", err)
	}

	wantWorktree := filepath.Join(parent, "repository-worktree-handoff")
	if submission.WorktreePath != wantWorktree || !filepath.IsAbs(submission.WorktreePath) {
		t.Fatalf("worktree path = %q, want absolute sibling %q", submission.WorktreePath, wantWorktree)
	}
	if submission.BaseBranch != "main" || submission.FeatureBranch != "feature/worktree-handoff" {
		t.Fatalf("reported branches = %q/%q, want main/feature/worktree-handoff", submission.BaseBranch, submission.FeatureBranch)
	}
	if got := runGitOutput(t, submission.WorktreePath, "branch", "--show-current"); got != "feature/worktree-handoff" {
		t.Fatalf("isolated worktree branch = %q, want feature/worktree-handoff", got)
	}
	if _, err := os.Stat(filepath.Join(repo, "specs")); !os.IsNotExist(err) {
		t.Fatalf("initiating worktree received a Phase 2 artifact directory: %v", err)
	}
}

// REQ-037, REQ-038 → SCN-241 → TestSCN241_PrepareNewImplementationSubmissionResolvesRepositoryDefaultIntegrationBranch
func TestSCN241_PrepareNewImplementationSubmissionResolvesRepositoryDefaultIntegrationBranch(t *testing.T) {
	// Scenario: Create an isolated feature worktree before Phase 2 writes a contract
	parent := t.TempDir()
	repo := filepath.Join(parent, "repository")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")
	mustWrite(t, filepath.Join(repo, "README.md"), "base\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "test: establish repository default")
	runGit(t, repo, "update-ref", "refs/remotes/origin/main", "HEAD")
	runGit(t, repo, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")

	submission, err := PrepareNewImplementationSubmission(repo, NewImplementationSubmissionRequest{Slug: "repository-default"})
	if err != nil {
		t.Fatalf("PrepareNewImplementationSubmission returned error: %v", err)
	}

	if submission.BaseBranch != "origin/main" {
		t.Fatalf("base branch = %q, want repository default %q", submission.BaseBranch, "origin/main")
	}
	if submission.FeatureBranch != "feature/repository-default" {
		t.Fatalf("feature branch = %q, want %q", submission.FeatureBranch, "feature/repository-default")
	}
	wantCommit := runGitOutput(t, repo, "rev-parse", "origin/main")
	if got := runGitOutput(t, submission.WorktreePath, "rev-parse", "HEAD"); got != wantCommit {
		t.Fatalf("feature worktree commit = %q, want repository-default integration commit %q", got, wantCommit)
	}
}

// REQ-042, REQ-043 → SCN-248 → TestSCN248_PresentsManualGitHubPRHandoff
func TestSCN248_PresentsManualGitHubPRHandoff(t *testing.T) {
	// Scenario: Present resolved manual GitHub PR handoff after Phase 4 passes
	repo := prepareSCN248Repository(t)
	submission := NewImplementationSubmission{
		WorktreePath:  repo,
		BaseBranch:    "main",
		FeatureBranch: "feature/worktree-handoff",
	}

	handoff, err := PresentManualGitHubPRHandoff(ManualGitHubPRHandoffRequest{
		Submission:     submission,
		ReviewedPaths:  []string{"internal/workflow/submission_worktree.go"},
		HostDisclaimer: "This host cannot delegate GitHub publication; use your own credentials.",
	})
	if err != nil {
		t.Fatalf("PresentManualGitHubPRHandoff returned error: %v", err)
	}

	for _, want := range []string{
		"cd \"" + repo + "\"",
		"git status --short",
		"git add -- \"internal/workflow/submission_worktree.go\"",
		"git commit",
		"git push origin feature/worktree-handoff",
		"gh pr create --base main --head feature/worktree-handoff",
		"https://github.com/",
		"This host cannot delegate GitHub publication; use your own credentials.",
	} {
		if !strings.Contains(handoff, want) {
			t.Fatalf("handoff missing %q:\n%s", want, handoff)
		}
	}
	if got := runGitOutput(t, repo, "status", "--short"); got != "" {
		t.Fatalf("manual handoff changed the worktree: %q", got)
	}
}

// REQ-042, REQ-043 → SCN-248 → TestSCN248_PresentsManualHandoffForSupportedGitHubURLForms
func TestSCN248_PresentsManualHandoffForSupportedGitHubURLForms(t *testing.T) {
	// Scenario: Present resolved manual GitHub PR handoff after Phase 4 passes
	for _, test := range []struct {
		name       string
		remoteURL  string
		wantWebURL string
	}{
		{
			name:       "SSH URL",
			remoteURL:  "ssh://git@github.com/example/repository.git",
			wantWebURL: "https://github.com/example/repository/compare/feature/worktree-handoff",
		},
		{
			name:       "HTTPS URL",
			remoteURL:  "https://github.com/example/repository.git",
			wantWebURL: "https://github.com/example/repository/compare/feature/worktree-handoff",
		},
		{
			name:       "HTTP URL",
			remoteURL:  "http://github.com/example/repository.git",
			wantWebURL: "https://github.com/example/repository/compare/feature/worktree-handoff",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := prepareSCN248Repository(t)
			runGit(t, repo, "remote", "set-url", "origin", test.remoteURL)

			handoff, err := PresentManualGitHubPRHandoff(ManualGitHubPRHandoffRequest{
				Submission: NewImplementationSubmission{
					WorktreePath:  repo,
					BaseBranch:    "main",
					FeatureBranch: "feature/worktree-handoff",
				},
			})
			if err != nil {
				t.Fatalf("PresentManualGitHubPRHandoff returned error: %v", err)
			}
			if !strings.Contains(handoff, test.wantWebURL) {
				t.Fatalf("handoff missing GitHub web UI URL %q:\n%s", test.wantWebURL, handoff)
			}
		})
	}
}

// REQ-042, REQ-043 → SCN-248 → TestSCN248_RejectsUnsafeManualHandoffCommands
func TestSCN248_RejectsUnsafeManualHandoffCommands(t *testing.T) {
	// Scenario: Present resolved manual GitHub PR handoff after Phase 4 passes
	repo := prepareSCN248Repository(t)
	_, err := PresentManualGitHubPRHandoff(ManualGitHubPRHandoffRequest{
		Submission: NewImplementationSubmission{
			WorktreePath:  repo,
			BaseBranch:    "main; unsafe-command",
			FeatureBranch: "feature/worktree-handoff",
		},
		HostDisclaimer: "This host cannot delegate GitHub publication; use your own credentials.",
	})
	if err == nil {
		t.Fatal("expected unsafe base branch to be rejected before printing a command")
	}
}

// REQ-042, REQ-044 → SCN-249 → TestSCN249_ReportsManualCommandFailureWithoutMutatingSubmission
func TestSCN249_ReportsManualCommandFailureWithoutMutatingSubmission(t *testing.T) {
	// Scenario: Preserve the feature worktree when manual PR creation fails
	repo := prepareSCN248Repository(t)
	submission := NewImplementationSubmission{
		WorktreePath:  repo,
		BaseBranch:    "main",
		FeatureBranch: "feature/worktree-handoff",
	}

	guidance, err := ReportManualGitHubPRFailure(submission, "gh pr create: authentication required")
	if err != nil {
		t.Fatalf("ReportManualGitHubPRFailure returned error: %v", err)
	}
	for _, want := range []string{
		"manual command failed: gh pr create: authentication required",
		"cd \"" + repo + "\"",
		"git status --short",
		"git branch --show-current",
		"feature/worktree-handoff",
		"preserved",
	} {
		if !strings.Contains(guidance, want) {
			t.Fatalf("failure guidance missing %q:\n%s", want, guidance)
		}
	}
	if got := runGitOutput(t, repo, "branch", "--show-current"); got != submission.FeatureBranch {
		t.Fatalf("manual failure guidance changed branch to %q, want %q", got, submission.FeatureBranch)
	}
	if got := runGitOutput(t, repo, "status", "--short"); got != "" {
		t.Fatalf("manual failure guidance changed the worktree: %q", got)
	}
	if !strings.Contains(guidance, "Do not retry automatically, switch publication mechanisms, merge, or modify main.") {
		t.Fatalf("failure guidance omitted the safe manual-only boundary:\n%s", guidance)
	}
}

// REQ-042, REQ-044 → SCN-249 → TestSCN249_RejectsIncompleteFailureGuidanceSubmission
func TestSCN249_RejectsIncompleteFailureGuidanceSubmission(t *testing.T) {
	// Scenario: Preserve the feature worktree when manual PR creation fails
	repo := prepareSCN248Repository(t)
	for _, test := range []struct {
		name       string
		submission NewImplementationSubmission
	}{
		{
			name: "relative worktree",
			submission: NewImplementationSubmission{
				WorktreePath:  "relative-worktree",
				BaseBranch:    "main",
				FeatureBranch: "feature/worktree-handoff",
			},
		},
		{
			name: "unsafe base branch",
			submission: NewImplementationSubmission{
				WorktreePath:  repo,
				BaseBranch:    "main; unsafe-command",
				FeatureBranch: "feature/worktree-handoff",
			},
		},
		{
			name: "unsafe feature branch",
			submission: NewImplementationSubmission{
				WorktreePath:  repo,
				BaseBranch:    "main",
				FeatureBranch: "feature/worktree handoff",
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			guidance, err := ReportManualGitHubPRFailure(test.submission, "push failed")
			if err == nil || guidance != "" {
				t.Fatalf("ReportManualGitHubPRFailure = %q, %v; want empty guidance and validation error", guidance, err)
			}
			if !strings.Contains(err.Error(), "requires the recorded feature worktree and branches") {
				t.Fatalf("validation error = %q, want recorded submission guidance", err)
			}
		})
	}
	if got := runGitOutput(t, repo, "branch", "--show-current"); got != "feature/worktree-handoff" {
		t.Fatalf("validation failure changed branch to %q", got)
	}
	if got := runGitOutput(t, repo, "status", "--short"); got != "" {
		t.Fatalf("validation failure changed the worktree: %q", got)
	}
}

// REQ-042, REQ-043 → SCN-250 → TestSCN250_ReportsRemoteResolutionRequiredWithoutPublicationCommands
func TestSCN250_ReportsRemoteResolutionRequiredWithoutPublicationCommands(t *testing.T) {
	// Scenario: Block guessed PR publication when no GitHub remote is unambiguous
	repo := prepareSCN248Repository(t)
	runGit(t, repo, "remote", "add", "upstream", "git@github.com:example/upstream.git")

	handoff, err := PresentManualGitHubPRHandoff(ManualGitHubPRHandoffRequest{
		Submission: NewImplementationSubmission{
			WorktreePath:  repo,
			BaseBranch:    "main",
			FeatureBranch: "feature/worktree-handoff",
		},
	})
	if err != nil {
		t.Fatalf("PresentManualGitHubPRHandoff returned error: %v", err)
	}
	if !strings.Contains(handoff, "remote selection requires user resolution") {
		t.Fatalf("handoff did not require remote resolution:\n%s", handoff)
	}
	for _, forbidden := range []string{"git push", "gh pr create", "github.com"} {
		if strings.Contains(handoff, forbidden) {
			t.Fatalf("handoff guessed a publication action %q:\n%s", forbidden, handoff)
		}
	}
	if got := runGitOutput(t, repo, "status", "--short"); got != "" {
		t.Fatalf("manual handoff changed the worktree: %q", got)
	}
}

func prepareSCN248Repository(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")
	mustWrite(t, filepath.Join(repo, "README.md"), "base\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "test: establish handoff baseline")
	runGit(t, repo, "checkout", "-b", "feature/worktree-handoff")
	runGit(t, repo, "remote", "add", "origin", "git@github.com:example/repository.git")
	return repo
}

// REQ-037, REQ-044 → SCN-242 → TestSCN242_PrepareNewImplementationSubmissionRejectsDetachedHEAD
func TestSCN242_PrepareNewImplementationSubmissionRejectsDetachedHEAD(t *testing.T) {
	// Scenario: Reject an unsafe starting condition without falling back to the initiating worktree
	parent := t.TempDir()
	repo := filepath.Join(parent, "repository")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")
	mustWrite(t, filepath.Join(repo, "README.md"), "base\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "test: establish integration base")
	runGit(t, repo, "checkout", "--detach")

	submission, err := PrepareNewImplementationSubmission(repo, NewImplementationSubmissionRequest{
		Slug:              "worktree-handoff",
		IntegrationBranch: "main",
	})
	if err == nil || !strings.Contains(err.Error(), "detached HEAD") {
		t.Fatalf("PrepareNewImplementationSubmission error = %v, want detached HEAD validation failure", err)
	}
	if submission != (NewImplementationSubmission{}) {
		t.Fatalf("submission = %#v, want no fallback submission", submission)
	}
	if _, err := os.Stat(filepath.Join(parent, "repository-worktree-handoff")); !os.IsNotExist(err) {
		t.Fatalf("isolated worktree was created after detached HEAD validation: %v", err)
	}
	if got := runGitOutput(t, repo, "status", "--short"); got != "" {
		t.Fatalf("initiating worktree status = %q, want no submission artifacts or code", got)
	}
}

// REQ-038, REQ-044 → SCN-243 → TestSCN243_PrepareNewImplementationSubmissionRejectsInvalidOrExistingFeatureBranch
func TestSCN243_PrepareNewImplementationSubmissionRejectsInvalidOrExistingFeatureBranch(t *testing.T) {
	// Scenario: Reject an invalid or unavailable feature branch
	for _, testCase := range []struct {
		name           string
		slug           string
		existingBranch bool
		wantError      string
	}{
		{name: "uppercase and whitespace", slug: "Feature Name", wantError: "invalid submission slug"},
		{name: "path traversal", slug: "../escape", wantError: "invalid submission slug"},
		{name: "existing feature branch", slug: "release-fix", existingBranch: true, wantError: "feature branch already exists"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			parent := t.TempDir()
			repo := filepath.Join(parent, "repository")
			if err := os.Mkdir(repo, 0o755); err != nil {
				t.Fatal(err)
			}
			runGit(t, repo, "init", "-b", "main")
			runGit(t, repo, "config", "user.email", "test@example.invalid")
			runGit(t, repo, "config", "user.name", "Test User")
			mustWrite(t, filepath.Join(repo, "README.md"), "base\n")
			runGit(t, repo, "add", "README.md")
			runGit(t, repo, "commit", "-m", "test: establish integration base")
			if testCase.existingBranch {
				runGit(t, repo, "branch", "feature/"+testCase.slug)
			}

			submission, err := PrepareNewImplementationSubmission(repo, NewImplementationSubmissionRequest{
				Slug:              testCase.slug,
				IntegrationBranch: "main",
			})
			if err == nil || !strings.Contains(err.Error(), testCase.wantError) {
				t.Fatalf("PrepareNewImplementationSubmission error = %v, want %q", err, testCase.wantError)
			}
			if submission != (NewImplementationSubmission{}) {
				t.Fatalf("submission = %#v, want no created or reused feature branch", submission)
			}
			if got := runGitOutput(t, repo, "branch", "--show-current"); got != "main" {
				t.Fatalf("initiating branch = %q, want main", got)
			}
			if got := runGitOutput(t, repo, "status", "--short"); got != "" {
				t.Fatalf("initiating worktree status = %q, want no submission artifacts or code", got)
			}
		})
	}
}

// REQ-039, REQ-044 → SCN-244 → TestSCN244_PrepareNewImplementationSubmissionRejectsCollidingSiblingWorktreePath
func TestSCN244_PrepareNewImplementationSubmissionRejectsCollidingSiblingWorktreePath(t *testing.T) {
	// Scenario: Reject a colliding sibling worktree path
	for _, testCase := range []struct {
		name    string
		occupy  func(t *testing.T, path string)
		inspect func(t *testing.T, path string)
	}{
		{
			name: "file",
			occupy: func(t *testing.T, path string) {
				mustWrite(t, path, "preserve me\n")
			},
			inspect: func(t *testing.T, path string) {
				content, err := os.ReadFile(path)
				if err != nil || string(content) != "preserve me\n" {
					t.Fatalf("colliding file was changed: content=%q, err=%v", content, err)
				}
			},
		},
		{
			name: "directory",
			occupy: func(t *testing.T, path string) {
				if err := os.Mkdir(path, 0o755); err != nil {
					t.Fatal(err)
				}
				mustWrite(t, filepath.Join(path, "preserve.txt"), "preserve me\n")
			},
			inspect: func(t *testing.T, path string) {
				content, err := os.ReadFile(filepath.Join(path, "preserve.txt"))
				if err != nil || string(content) != "preserve me\n" {
					t.Fatalf("colliding directory was changed: content=%q, err=%v", content, err)
				}
			},
		},
		{
			name: "symlink",
			occupy: func(t *testing.T, path string) {
				target := filepath.Join(filepath.Dir(path), "preserved-target")
				mustWrite(t, target, "preserve me\n")
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			},
			inspect: func(t *testing.T, path string) {
				info, err := os.Lstat(path)
				if err != nil || info.Mode()&os.ModeSymlink == 0 {
					t.Fatalf("colliding symlink was removed or replaced: info=%v, err=%v", info, err)
				}
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			parent := t.TempDir()
			repo := filepath.Join(parent, "repository")
			if err := os.Mkdir(repo, 0o755); err != nil {
				t.Fatal(err)
			}
			runGit(t, repo, "init", "-b", "main")
			runGit(t, repo, "config", "user.email", "test@example.invalid")
			runGit(t, repo, "config", "user.name", "Test User")
			mustWrite(t, filepath.Join(repo, "README.md"), "base\n")
			runGit(t, repo, "add", "README.md")
			runGit(t, repo, "commit", "-m", "test: establish integration base")

			collidingPath := filepath.Join(parent, "repository-worktree-handoff")
			testCase.occupy(t, collidingPath)

			submission, err := PrepareNewImplementationSubmission(repo, NewImplementationSubmissionRequest{
				Slug:              "worktree-handoff",
				IntegrationBranch: "main",
			})
			if err == nil || !strings.Contains(err.Error(), "worktree path collision") {
				t.Fatalf("PrepareNewImplementationSubmission error = %v, want path collision", err)
			}
			if submission != (NewImplementationSubmission{}) {
				t.Fatalf("submission = %#v, want no submission", submission)
			}
			testCase.inspect(t, collidingPath)
			if got := runGitOutput(t, repo, "branch", "--list", "feature/worktree-handoff"); got != "" {
				t.Fatalf("feature branch = %q, want no worktree operation", got)
			}
			if got := runGitOutput(t, repo, "status", "--short"); got != "" {
				t.Fatalf("initiating worktree status = %q, want no Phase 2 or Phase 3 artifacts", got)
			}
		})
	}
}

// REQ-039 → SCN-245 → TestSCN245_PrepareNewImplementationSubmissionRejectsWorktreeOwnedByAnotherSubmission
func TestSCN245_PrepareNewImplementationSubmissionRejectsWorktreeOwnedByAnotherSubmission(t *testing.T) {
	// Scenario: Allow concurrent submissions only with independent worktree ownership
	parent := t.TempDir()
	repo := filepath.Join(parent, "repository")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")
	mustWrite(t, filepath.Join(repo, "README.md"), "base\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "test: establish integration base")

	alpha, err := PrepareNewImplementationSubmission(repo, NewImplementationSubmissionRequest{
		Slug:              "alpha",
		IntegrationBranch: "main",
	})
	if err != nil {
		t.Fatalf("prepare alpha submission: %v", err)
	}
	mustWrite(t, filepath.Join(alpha.WorktreePath, "alpha-only.txt"), "alpha state\n")
	secondInitiatingWorktree := filepath.Join(parent, "repository-second")
	runGit(t, repo, "worktree", "add", "-b", "initiator", secondInitiatingWorktree, "main")
	betaPath := filepath.Join(parent, "repository-second-beta")
	runGit(t, repo, "worktree", "add", "-b", "other-submission", betaPath, "main")
	if err := os.RemoveAll(betaPath); err != nil {
		t.Fatalf("remove stale worktree directory: %v", err)
	}

	blockedBeta, err := PrepareNewImplementationSubmission(secondInitiatingWorktree, NewImplementationSubmissionRequest{
		Slug:              "beta",
		IntegrationBranch: "main",
	})
	if err == nil || !strings.Contains(err.Error(), "worktree ownership conflict") {
		t.Fatalf("PrepareNewImplementationSubmission error = %v, want worktree ownership conflict", err)
	}
	if blockedBeta != (NewImplementationSubmission{}) {
		t.Fatalf("beta submission = %#v, want no submission using another worktree's path", blockedBeta)
	}
	runGit(t, repo, "worktree", "prune")

	beta, err := PrepareNewImplementationSubmission(secondInitiatingWorktree, NewImplementationSubmissionRequest{
		Slug:              "beta",
		IntegrationBranch: "main",
	})
	if err != nil {
		t.Fatalf("prepare beta submission: %v", err)
	}
	if alpha.FeatureBranch == beta.FeatureBranch || alpha.WorktreePath == beta.WorktreePath {
		t.Fatalf("submissions share branch/path: alpha=%#v beta=%#v", alpha, beta)
	}
	if beta.WorktreePath != betaPath || beta.FeatureBranch != "feature/beta" {
		t.Fatalf("beta submission = %#v, want feature/beta at %q", beta, betaPath)
	}
	if got := runGitOutput(t, alpha.WorktreePath, "branch", "--show-current"); got != "feature/alpha" {
		t.Fatalf("alpha worktree branch = %q, want feature/alpha", got)
	}
	if got := runGitOutput(t, beta.WorktreePath, "branch", "--show-current"); got != "feature/beta" {
		t.Fatalf("beta worktree branch = %q, want feature/beta", got)
	}
	if content, err := os.ReadFile(filepath.Join(alpha.WorktreePath, "alpha-only.txt")); err != nil || string(content) != "alpha state\n" {
		t.Fatalf("alpha-only state changed: content=%q, err=%v", content, err)
	}
	if _, err := os.Stat(filepath.Join(beta.WorktreePath, "alpha-only.txt")); !os.IsNotExist(err) {
		t.Fatalf("beta worktree received alpha state: %v", err)
	}
	if got := runGitOutput(t, beta.WorktreePath, "status", "--short"); got != "" {
		t.Fatalf("beta worktree status = %q, want independent clean state", got)
	}
}

// REQ-040, REQ-041 → SCN-246 → TestSCN246_HaltsWhenPhase3SubagentBoundaryLosesFeatureWorktreeIdentity
func TestSCN246_HaltsWhenPhase3SubagentBoundaryLosesFeatureWorktreeIdentity(t *testing.T) {
	// Scenario: Halt when a Phase 3 subagent boundary loses feature-worktree identity
	parent := t.TempDir()
	repo := filepath.Join(parent, "repository")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")
	mustWrite(t, filepath.Join(repo, "README.md"), "base\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "test: establish integration base")
	runGit(t, repo, "checkout", "-b", "initiator")

	submission, err := PrepareNewImplementationSubmission(repo, NewImplementationSubmissionRequest{
		Slug:              "worktree-handoff",
		IntegrationBranch: "main",
	})
	if err != nil {
		t.Fatalf("PrepareNewImplementationSubmission returned error: %v", err)
	}
	runGit(t, submission.WorktreePath, "checkout", "main")
	before := runGitOutput(t, submission.WorktreePath, "rev-parse", "HEAD")
	nextSubagentLaunched := false

	err = ValidatePhase3SubagentBoundary(submission, submission.WorktreePath, func() error {
		nextSubagentLaunched = true
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "feature branch identity") {
		t.Fatalf("ValidatePhase3SubagentBoundary error = %v, want feature branch identity failure", err)
	}
	if nextSubagentLaunched {
		t.Fatal("next subagent launched after feature worktree identity failure")
	}
	if after := runGitOutput(t, submission.WorktreePath, "rev-parse", "HEAD"); after != before {
		t.Fatalf("boundary validation changed HEAD from %q to %q", before, after)
	}
	if branch := runGitOutput(t, submission.WorktreePath, "branch", "--show-current"); branch != "main" {
		t.Fatalf("boundary validation changed branch to %q, want main", branch)
	}
}

// REQ-040, REQ-041 → SCN-246 → TestSCN246_HaltsForOtherDetachedOrWrongPhase3Worktree
func TestSCN246_HaltsForOtherDetachedOrWrongPhase3Worktree(t *testing.T) {
	// Scenario: Halt when a Phase 3 subagent boundary loses feature-worktree identity
	for _, testCase := range []struct {
		name     string
		mutate   func(t *testing.T, repo string, submission NewImplementationSubmission)
		returned func(repo string, submission NewImplementationSubmission) string
	}{
		{
			name: "other branch",
			mutate: func(t *testing.T, _ string, submission NewImplementationSubmission) {
				runGit(t, submission.WorktreePath, "checkout", "-b", "other")
			},
			returned: func(_ string, submission NewImplementationSubmission) string { return submission.WorktreePath },
		},
		{
			name: "detached HEAD",
			mutate: func(t *testing.T, _ string, submission NewImplementationSubmission) {
				runGit(t, submission.WorktreePath, "checkout", "--detach")
			},
			returned: func(_ string, submission NewImplementationSubmission) string { return submission.WorktreePath },
		},
		{
			name:     "wrong worktree",
			mutate:   func(t *testing.T, _ string, _ NewImplementationSubmission) {},
			returned: func(repo string, _ NewImplementationSubmission) string { return repo },
		},
		{
			name:   "missing worktree",
			mutate: func(t *testing.T, _ string, _ NewImplementationSubmission) {},
			returned: func(repo string, _ NewImplementationSubmission) string {
				return filepath.Join(filepath.Dir(repo), "missing-worktree")
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repo, submission := prepareSCN246Submission(t)
			testCase.mutate(t, repo, submission)
			nextSubagentLaunched := false

			err := ValidatePhase3SubagentBoundary(submission, testCase.returned(repo, submission), func() error {
				nextSubagentLaunched = true
				return nil
			})
			if err == nil || !strings.Contains(err.Error(), "identity failure") {
				t.Fatalf("ValidatePhase3SubagentBoundary error = %v, want identity failure", err)
			}
			if nextSubagentLaunched {
				t.Fatal("next subagent launched after feature worktree identity failure")
			}
		})
	}
}

func prepareSCN246Submission(t *testing.T) (string, NewImplementationSubmission) {
	t.Helper()
	parent := t.TempDir()
	repo := filepath.Join(parent, "repository")
	if err := os.Mkdir(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")
	mustWrite(t, filepath.Join(repo, "README.md"), "base\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "test: establish integration base")
	runGit(t, repo, "checkout", "-b", "initiator")
	submission, err := PrepareNewImplementationSubmission(repo, NewImplementationSubmissionRequest{
		Slug:              "worktree-handoff",
		IntegrationBranch: "main",
	})
	if err != nil {
		t.Fatalf("PrepareNewImplementationSubmission returned error: %v", err)
	}
	return repo, submission
}

func runGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}
