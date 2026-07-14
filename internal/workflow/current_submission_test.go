package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSCN234_InitializeCurrentSubmissionUsesExplicitContractScope(t *testing.T) {
	// REQ-032 → SCN-234 → TestSCN234_InitializeCurrentSubmissionUsesExplicitContractScope
	// Scenario: Create an isolated active submission
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "specs", ".approved"), "SCN-001\n")
	mustWrite(t, filepath.Join(repo, ".rotta", "tdd-log.md"), "## SCN-002\n")
	mustWrite(t, filepath.Join(repo, "specs", "workflow_lifecycle_hard_spec.md"), "# lifecycle\n")
	mustWrite(t, filepath.Join(repo, "features", "workflow_lifecycle.feature"), "@SCN-234\n")

	submission, err := InitializeCurrentSubmission(repo, CurrentSubmissionRequest{
		ID:           "workflow-lifecycle-scn-234",
		SpecPath:     "specs/workflow_lifecycle_hard_spec.md",
		FeaturePaths: []string{"features/workflow_lifecycle.feature"},
		ScenarioIDs:  []string{"SCN-234"},
	})
	if err != nil {
		t.Fatalf("InitializeCurrentSubmission returned error: %v", err)
	}

	if submission.Manifest.SubmissionID != "workflow-lifecycle-scn-234" {
		t.Fatalf("manifest submission ID = %q, want %q", submission.Manifest.SubmissionID, "workflow-lifecycle-scn-234")
	}
	if submission.Manifest.SpecPath != "specs/workflow_lifecycle_hard_spec.md" || len(submission.Manifest.FeaturePaths) != 1 || submission.Manifest.FeaturePaths[0] != "features/workflow_lifecycle.feature" {
		t.Fatalf("manifest contract paths = %#v, want explicit spec and feature paths", submission.Manifest)
	}
	if got := submission.Manifest.ScenarioIDs; len(got) != 1 || got[0] != "SCN-234" {
		t.Fatalf("manifest scenario scope = %v, want [SCN-234] without legacy scenarios", got)
	}
	if submission.Manifest.Worktree != repo || submission.Manifest.Status != "in_progress" {
		t.Fatalf("manifest worktree/status = %q/%q, want %q/in_progress", submission.Manifest.Worktree, submission.Manifest.Status, repo)
	}
	if submission.State.Phase == "" || submission.State.CompletedWork == nil || len(submission.State.RemainingWork) != 1 || submission.State.RemainingWork[0] != "SCN-234" || submission.State.LastAction == "" || submission.State.SafeResumePoint == "" {
		t.Fatalf("state does not contain required initial resume data: %#v", submission.State)
	}

	for _, path := range []string{submission.ManifestPath, submission.StatePath} {
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read current submission file %s: %v", path, readErr)
		}
		if strings.Contains(string(content), "SCN-001") || strings.Contains(string(content), "SCN-002") {
			t.Fatalf("current submission file %s inherited legacy scope: %s", path, content)
		}
	}
}

func TestSCN235_LoadCurrentSubmissionRejectsUnusableActiveState(t *testing.T) {
	// REQ-032 → SCN-235 → TestSCN235_LoadCurrentSubmissionRejectsUnusableActiveState
	// Scenario: Reject malformed or missing active submission state
	for _, testCase := range []struct {
		name     string
		setup    func(t *testing.T, repo string)
		contains string
	}{
		{
			name: "missing manifest",
			setup: func(t *testing.T, repo string) {
				mustWrite(t, filepath.Join(repo, "specs", ".approved"), "SCN-001\n")
				mustWrite(t, filepath.Join(repo, ".rotta", "archive", "old", "manifest.yaml"), "scenario_ids:\n  - SCN-002\n")
			},
			contains: "current submission state cannot be safely used",
		},
		{
			name: "malformed manifest",
			setup: func(t *testing.T, repo string) {
				mustWrite(t, filepath.Join(repo, ".rotta", "current", "manifest.yaml"), "scenario_ids: SCN-235\n")
			},
			contains: "current submission state cannot be safely used",
		},
		{
			name: "missing feature",
			setup: func(t *testing.T, repo string) {
				mustWrite(t, filepath.Join(repo, ".rotta", "current", "manifest.yaml"), "submission_id: lifecycle\nspec_path: specs/workflow_lifecycle_hard_spec.md\nfeature_paths:\n  - features/missing.feature\nscenario_ids:\n  - SCN-235\nworktree: "+repo+"\nstatus: in_progress\n")
			},
			contains: "current submission state cannot be safely used",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repo := t.TempDir()
			testCase.setup(t, repo)

			_, err := LoadCurrentSubmission(repo)
			if err == nil || !strings.Contains(err.Error(), testCase.contains) {
				t.Fatalf("LoadCurrentSubmission error = %v, want unusable current-state error", err)
			}
		})
	}
}

func TestSCN236_ResumeCurrentSubmissionUsesLocalStateWhenAncoraIsUnavailableOrStale(t *testing.T) {
	// REQ-033, REQ-036 → SCN-236 → TestSCN236_ResumeCurrentSubmissionUsesLocalStateWhenAncoraIsUnavailableOrStale
	// Scenario: Resume an interrupted submission from local state when memory is unavailable
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "specs", "workflow_lifecycle_hard_spec.md"), "# local contract\n")
	mustWrite(t, filepath.Join(repo, "features", "workflow_lifecycle.feature"), "@SCN-236\n")
	mustWrite(t, filepath.Join(repo, ".rotta", "current", "manifest.yaml"), "submission_id: workflow-lifecycle\nspec_path: specs/workflow_lifecycle_hard_spec.md\nfeature_paths:\n  - features/workflow_lifecycle.feature\nscenario_ids:\n  - SCN-236\nworktree: "+repo+"\nstatus: interrupted\n")
	mustWrite(t, filepath.Join(repo, ".rotta", "current", "state.yaml"), "phase: implementation\ncompleted_work:\n  - SCN-234\nremaining_work:\n  - SCN-236\nblocked_work:\n  - awaiting review\nlast_action: TestSCN234_InitializeCurrentSubmissionUsesExplicitContractScope\nsafe_resume_point: implement SCN-236\n")
	mustWrite(t, filepath.Join(repo, ".rotta", "current", "tdd-log.md"), "## SCN-234\n")

	resumed, err := ResumeCurrentSubmission(repo, &CurrentSubmissionAncoraPointer{
		SubmissionID:   "stale-submission",
		LocalStatePath: ".rotta/current/deleted-state.yaml",
	})
	if err != nil {
		t.Fatalf("ResumeCurrentSubmission returned error: %v", err)
	}
	if got, want := strings.Join(resumed.CompletedWork, ","), "SCN-234"; got != want {
		t.Fatalf("completed work = %v, want %v", got, want)
	}
	if got, want := strings.Join(resumed.RemainingWork, ","), "SCN-236"; got != want {
		t.Fatalf("remaining work = %v, want %v", got, want)
	}
	if got, want := strings.Join(resumed.BlockedWork, ","), "awaiting review"; got != want {
		t.Fatalf("blocked work = %v, want %v", got, want)
	}
	if !resumed.AncoraPointer.Stale || resumed.AncoraPointer.Repaired.SubmissionID != "workflow-lifecycle" || resumed.AncoraPointer.Repaired.LocalStatePath != ".rotta/current/state.yaml" {
		t.Fatalf("expected stale Ancora pointer to be reported with local repair, got %#v", resumed.AncoraPointer)
	}

	unavailable, err := ResumeCurrentSubmission(repo, nil)
	if err != nil {
		t.Fatalf("ResumeCurrentSubmission without Ancora returned error: %v", err)
	}
	if !unavailable.AncoraPointer.Unavailable || unavailable.State.SafeResumePoint != "implement SCN-236" {
		t.Fatalf("expected local resume despite unavailable Ancora, got %#v", unavailable)
	}
}

func TestSCN237_ReviewCurrentSubmissionUsesOnlyManifestScenarioScope(t *testing.T) {
	// REQ-034 → SCN-237 → TestSCN237_ReviewCurrentSubmissionUsesOnlyManifestScenarioScope
	// Scenario: Review only scenarios declared by the current submission
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "features", "workflow_lifecycle.feature"), "@SCN-237\n")
	mustWrite(t, filepath.Join(repo, ".rotta", "current", "manifest.yaml"), "submission_id: workflow-lifecycle\nspec_path: specs/workflow_lifecycle_hard_spec.md\nfeature_paths:\n  - features/workflow_lifecycle.feature\nscenario_ids:\n  - SCN-237\nworktree: "+repo+"\nstatus: in_progress\n")
	mustWrite(t, filepath.Join(repo, ".rotta", "current", "tdd-log.md"), "## SCN-237\n")
	mustWrite(t, filepath.Join(repo, "specs", ".approved"), "SCN-001\n")
	mustWrite(t, filepath.Join(repo, ".rotta", "tdd-log.md"), "## SCN-002\n")
	mustWrite(t, filepath.Join(repo, ".rotta", "archive", "old", "tdd-log.md"), "## SCN-003\n")

	review, err := ReviewCurrentSubmission(repo)
	if err != nil {
		t.Fatalf("ReviewCurrentSubmission returned error: %v", err)
	}
	if !review.Passed {
		t.Fatalf("review failed from unrelated legacy evidence: %#v", review)
	}
	if got, want := strings.Join(review.ScenarioIDs, ","), "SCN-237"; got != want {
		t.Fatalf("review scenario scope = %q, want %q", got, want)
	}
	if len(review.MissingEvidence) != 0 {
		t.Fatalf("missing evidence = %v, want only manifest scenario evidence checked", review.MissingEvidence)
	}
	if len(review.Warnings) == 0 {
		t.Fatal("expected legacy artifacts to be reported as non-blocking warnings")
	}
}

func TestSCN238_ArchiveTerminalCurrentSubmissionRetainsDurableContracts(t *testing.T) {
	// REQ-035 → SCN-238 → TestSCN238_ArchiveTerminalCurrentSubmissionRetainsDurableContracts
	// Scenario: Archive a completed submission without removing durable contracts
	for _, status := range []string{"completed", "abandoned", "cancelled"} {
		t.Run(status, func(t *testing.T) {
			repo := t.TempDir()
			const submissionID = "workflow-lifecycle-scn-238"
			specPath := filepath.Join(repo, "specs", "workflow_lifecycle_hard_spec.md")
			featurePath := filepath.Join(repo, "features", "workflow_lifecycle.feature")
			mustWrite(t, specPath, "# durable hard spec\n")
			mustWrite(t, featurePath, "@SCN-238\n")
			mustWrite(t, filepath.Join(repo, ".rotta", "current", "manifest.yaml"), "submission_id: "+submissionID+"\nspec_path: specs/workflow_lifecycle_hard_spec.md\nfeature_paths:\n  - features/workflow_lifecycle.feature\nscenario_ids:\n  - SCN-238\nworktree: "+repo+"\nstatus: "+status+"\n")
			mustWrite(t, filepath.Join(repo, ".rotta", "current", "state.yaml"), "phase: complete\ncompleted_work:\n  - SCN-238\nremaining_work:\n  []\nblocked_work:\n  []\nlast_action: committed feature changes\nsafe_resume_point: none\n")
			mustWrite(t, filepath.Join(repo, ".rotta", "current", "tdd-log.md"), "## SCN-238\n")
			mustWrite(t, filepath.Join(repo, ".rotta", "tdd-log.md"), "## legacy evidence\n")

			if err := ArchiveTerminalCurrentSubmission(repo, true); err != nil {
				t.Fatalf("ArchiveTerminalCurrentSubmission returned error: %v", err)
			}

			archivePath := filepath.Join(repo, ".rotta", "archive", submissionID)
			if _, err := os.Stat(filepath.Join(archivePath, "manifest.yaml")); err != nil {
				t.Fatalf("archived manifest is missing: %v", err)
			}
			if _, err := os.Stat(filepath.Join(repo, ".rotta", "current")); !os.IsNotExist(err) {
				t.Fatalf("current execution state still exists after archive: %v", err)
			}
			if _, err := LoadCurrentSubmission(repo); err == nil {
				t.Fatal("archived submission remained in active review scope")
			}
			for _, contract := range []string{specPath, featurePath} {
				if _, err := os.Stat(contract); err != nil {
					t.Fatalf("durable contract %s was removed: %v", contract, err)
				}
			}
		})
	}
}

func TestSCN239_RetainsRecentArchivesAndManuallyRemovesOnlyRequestedArchive(t *testing.T) {
	// REQ-035 → SCN-239 → TestSCN239_RetainsRecentArchivesAndManuallyRemovesOnlyRequestedArchive
	// Scenario: Retain and manually clean archived execution state
	repo := t.TempDir()
	const submissionID = "workflow-lifecycle-scn-239"
	archivePath := filepath.Join(repo, ".rotta", "archive", submissionID)
	mustWrite(t, filepath.Join(archivePath, "manifest.yaml"), "submission_id: "+submissionID+"\n")
	specPath := filepath.Join(repo, "specs", "workflow_lifecycle_hard_spec.md")
	featurePath := filepath.Join(repo, "features", "workflow_lifecycle.feature")
	mustWrite(t, specPath, "# durable hard spec\n")
	mustWrite(t, featurePath, "@SCN-239\n")

	now := time.Date(2026, time.July, 13, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(archivePath, now.Add(-29*24*time.Hour), now.Add(-29*24*time.Hour)); err != nil {
		t.Fatalf("set archive age: %v", err)
	}
	removed, err := CleanupExpiredArchivedSubmissions(repo, now)
	if err != nil {
		t.Fatalf("CleanupExpiredArchivedSubmissions returned error: %v", err)
	}
	if len(removed) != 0 {
		t.Fatalf("recent archive was automatically removed: %v", removed)
	}
	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("recent archive was not retained: %v", err)
	}

	if err := RemoveArchivedSubmission(repo, submissionID); err != nil {
		t.Fatalf("RemoveArchivedSubmission returned error: %v", err)
	}
	if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
		t.Fatalf("requested archive still exists after manual cleanup: %v", err)
	}
	for _, contract := range []string{specPath, featurePath} {
		if _, err := os.Stat(contract); err != nil {
			t.Fatalf("durable contract %s was removed: %v", contract, err)
		}
	}
}

func TestSCN240_RecordCurrentSubmissionAncoraStateIsCompactAndLocal(t *testing.T) {
	// REQ-036 → SCN-240 → TestSCN240_RecordCurrentSubmissionAncoraStateIsCompactAndLocal
	// Scenario: Save only a compact lifecycle pointer to Ancora
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "specs", "workflow_lifecycle_hard_spec.md"), "# full hard spec\n")
	mustWrite(t, filepath.Join(repo, "features", "workflow_lifecycle.feature"), "@SCN-240\n")
	mustWrite(t, filepath.Join(repo, ".rotta", "current", "manifest.yaml"), "submission_id: workflow-lifecycle-scn-240\nspec_path: specs/workflow_lifecycle_hard_spec.md\nfeature_paths:\n  - features/workflow_lifecycle.feature\nscenario_ids:\n  - SCN-240\nworktree: "+repo+"\nstatus: in_progress\n")
	mustWrite(t, filepath.Join(repo, ".rotta", "current", "state.yaml"), "phase: implementation\ncompleted_work:\n  - SCN-239\nremaining_work:\n  - SCN-240\nblocked_work:\n  []\nlast_action: TestSCN239_RetainsRecentArchivesAndManuallyRemovesOnlyRequestedArchive\nsafe_resume_point: implement SCN-240\n")
	mustWrite(t, filepath.Join(repo, ".rotta", "current", "tdd-log.md"), "full TDD log must remain local\n")

	payload, err := RecordCurrentSubmissionAncoraState(repo)
	if err != nil {
		t.Fatalf("RecordCurrentSubmissionAncoraState returned error: %v", err)
	}
	serialized := string(payload)
	for _, want := range []string{"workflow-lifecycle-scn-240", "implementation", "in_progress", "SCN-239", "SCN-240", "TestSCN239_RetainsRecentArchivesAndManuallyRemovesOnlyRequestedArchive", ".rotta/current/state.yaml", ".rotta/current/tdd-log.md"} {
		if !strings.Contains(serialized, want) {
			t.Fatalf("compact Ancora payload missing %q: %s", want, serialized)
		}
	}
	for _, forbidden := range []string{"# full hard spec", "@SCN-240\n", "full TDD log must remain local", "judge report"} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("compact Ancora payload included local artifact content %q: %s", forbidden, serialized)
		}
	}
}
