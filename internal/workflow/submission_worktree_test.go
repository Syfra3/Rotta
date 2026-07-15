package workflow

import (
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

// REQ-046, REQ-048 → SCN-315 → TestSCN315_RefusesImplementationWithoutMatchingApprovedBaseline
func TestSCN315_RefusesImplementationWithoutMatchingApprovedBaseline(t *testing.T) {
	// Scenario: Refuse implementation without a matching approved baseline
	for _, testCase := range []struct {
		name    string
		prepare func(t *testing.T, repo string, state CurrentSubmissionState)
	}{
		{
			name: "no explicit feature-scoped approval record",
			prepare: func(t *testing.T, repo string, state CurrentSubmissionState) {
				mustWrite(t, filepath.Join(repo, "specs", ".approved"), "SCN-315\n")
				if err := os.Remove(filepath.Join(repo, filepath.FromSlash(state.ApprovalRecordPath))); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "approval record excludes the next scenario",
			prepare: func(t *testing.T, repo string, state CurrentSubmissionState) {
				mustWrite(t, filepath.Join(repo, filepath.FromSlash(state.ApprovalRecordPath)), "approved_scenarios:\n  - features/feature_worktree_lifecycle.feature#SCN-316\ncontract_fingerprints:\n  specs/hard_spec.md: "+mustContractFingerprint(t, repo, "specs/hard_spec.md")+"\n  features/feature_worktree_lifecycle.feature: "+mustContractFingerprint(t, repo, "features/feature_worktree_lifecycle.feature")+"\n")
				state.ApprovalRecordFingerprint = mustContractFingerprint(t, repo, state.ApprovalRecordPath)
				mustWrite(t, filepath.Join(repo, ".rotta", "current", "state.yaml"), serializeCurrentSubmissionState(state))
			},
		},
		{
			name: "contract changed after its approved baseline checkpoint",
			prepare: func(t *testing.T, repo string, state CurrentSubmissionState) {
				mustWrite(t, filepath.Join(repo, "features", "feature_worktree_lifecycle.feature"), "@SCN-315\nchanged\n")
			},
		},
		{
			name: "approval baseline cannot be committed",
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
			if decision.Allowed || !strings.Contains(decision.Reason, "implementation blocked") || !strings.Contains(decision.Reason, "recovery:") {
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
