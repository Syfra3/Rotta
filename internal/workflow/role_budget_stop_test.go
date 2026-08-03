package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// REQ-085 → SCN-615 → TestSCN615_RoleBudgetStopsRemainResumableAndTraceable
func TestSCN615_RoleBudgetStopsRemainResumableAndTraceable(t *testing.T) {
	// Scenario: A recorded role budget stop does not hide unfinished work
	for _, testCase := range []struct {
		role  WorkflowRole
		steps int
	}{
		{RoleExploration, 8},
		{RoleSpec, 12},
		{RoleOrchestration, 16},
		{RoleReview, 16},
		{RoleImplementation, 24},
	} {
		t.Run(string(testCase.role), func(t *testing.T) {
			repo := t.TempDir()
			manifestPath := filepath.Join(repo, ".rotta", "current", "manifest.yaml")
			evidencePath := filepath.Join(repo, ".rotta", "current", "evidence", "focused-test.txt")
			mustWrite(t, manifestPath, "feature_id: budget-stop\n")
			mustWrite(t, evidencePath, "focused test failed\n")

			stop, err := RecordRoleBudgetStop(RoleBudgetStopRequest{
				FeatureWorktree: repo,
				Role:            testCase.role,
				StepsUsed:       testCase.steps,
				Slice:           "SCN-615",
				EvidencePath:    evidencePath,
			})
			if err != nil {
				t.Fatalf("RecordRoleBudgetStop() error = %v", err)
			}
			if stop.Success || !stop.ValidationRequired || !stop.Resumable {
				t.Fatalf("stop completion = %#v, want unsuccessful resumable stop requiring validation", stop)
			}
			if stop.ManifestPath != manifestPath || stop.Slice != "SCN-615" || stop.EvidencePath != evidencePath {
				t.Fatalf("stop references = %#v, want current manifest, slice, and evidence", stop)
			}
			record, err := os.ReadFile(stop.RecordPath)
			if err != nil {
				t.Fatalf("read stop record: %v", err)
			}
			if strings.Contains(string(record), "prior_transcript") {
				t.Fatalf("stop record stores a prior transcript: %s", record)
			}

			resumed, err := ResumeRoleBudgetStop(stop.RecordPath)
			if err != nil {
				t.Fatalf("ResumeRoleBudgetStop() error = %v", err)
			}
			if resumed.Success || !resumed.ValidationRequired {
				t.Fatalf("resumed stop = %#v, want unfinished validation with no prior transcript", resumed)
			}
		})
	}
}
