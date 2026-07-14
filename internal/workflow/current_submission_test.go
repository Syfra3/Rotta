package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
