package workflow

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// REQ-082 → SCN-606 → TestSCN606_ContinuationAndReviewUseOnlyFeatureLocalPolicyAndEvidence
func TestSCN606_ContinuationAndReviewUseOnlyFeatureLocalPolicyAndEvidence(t *testing.T) {
	// Scenario: Only feature-local policy and evidence paths are active
	for _, testCase := range []struct {
		name       string
		legacyPath string
	}{
		{name: "root TDD log", legacyPath: ".rotta/tdd-log.md"},
		{name: "root state", legacyPath: ".rotta/state.yaml"},
		{name: "legacy approval marker", legacyPath: "specs/.approved"},
		{name: "root review evidence", legacyPath: ".rotta/review-evidence.yaml"},
		{name: "root judge report", legacyPath: "reports/judge_report.md"},
		{name: "legacy state-machine asset", legacyPath: "assets/config/state-machine.yaml"},
		{name: "legacy Rotta state-machine", legacyPath: ".rotta/state-machine.yaml"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repo, baseline := prepareSCN606FeatureWorkflow(t)
			mustWrite(t, filepath.Join(repo, filepath.FromSlash(testCase.legacyPath)), "SCN-999 legacy authority\n")

			continued, err := ResumeFeatureWorkflow(repo)
			if err != nil {
				t.Fatalf("ResumeFeatureWorkflow returned error: %v", err)
			}
			if continued.FeatureID != "feature-local" || continued.ScenarioOrSlice != "SCN-606" {
				t.Fatalf("continuation selected %#v, want only feature-local SCN-606", continued)
			}
			if got := strings.Join(continued.RetiredArtifacts, ","); got != testCase.legacyPath {
				t.Fatalf("continuation retired artifacts = %q, want %q", got, testCase.legacyPath)
			}

			review, err := ReviewFeatureWorkflow(repo)
			if err != nil {
				t.Fatalf("ReviewFeatureWorkflow returned error: %v", err)
			}
			if !review.Passed || review.FeatureID != "feature-local" || review.ScenarioOrSlice != "SCN-606" {
				t.Fatalf("review = %#v, want passed feature-local SCN-606 review", review)
			}
			if review.PolicyPath != ".rotta/quality-gates.yaml" || strings.Join(review.EvidencePaths, ",") != ".rotta/current/tdd-log.md" {
				t.Fatalf("review active paths = %q / %q, want manifest-bound policy and current evidence", review.PolicyPath, review.EvidencePaths)
			}
			if got := strings.Join(review.RetiredArtifacts, ","); got != testCase.legacyPath {
				t.Fatalf("review retired artifacts = %q, want %q", got, testCase.legacyPath)
			}
			if review.BaselineSHA != baseline {
				t.Fatalf("review baseline = %q, want %q", review.BaselineSHA, baseline)
			}
		})
	}
}

// REQ-082 → SCN-607 → TestSCN607_InvalidFeatureLocalPolicyBlocksWithoutFallback
func TestSCN607_InvalidFeatureLocalPolicyBlocksWithoutFallback(t *testing.T) {
	// Scenario: Missing or drifted feature-local policy blocks rather than falls back
	for _, testCase := range []struct {
		name       string
		invalidate func(t *testing.T, repo string)
	}{
		{
			name: "missing",
			invalidate: func(t *testing.T, repo string) {
				t.Helper()
				if err := os.Remove(filepath.Join(repo, ".rotta", "quality-gates.yaml")); err != nil {
					t.Fatalf("remove feature-local policy: %v", err)
				}
			},
		},
		{
			name: "unreadable",
			invalidate: func(t *testing.T, repo string) {
				t.Helper()
				policyPath := filepath.Join(repo, ".rotta", "quality-gates.yaml")
				if err := os.Remove(policyPath); err != nil {
					t.Fatalf("remove feature-local policy: %v", err)
				}
				if err := os.Mkdir(policyPath, 0o700); err != nil {
					t.Fatalf("make feature-local policy unreadable: %v", err)
				}
			},
		},
		{
			name: "fingerprint drifted",
			invalidate: func(t *testing.T, repo string) {
				t.Helper()
				mustWrite(t, filepath.Join(repo, ".rotta", "quality-gates.yaml"), "format: rotta.quality-gates/v1\ndrifted: true\n")
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repo, _ := prepareSCN606FeatureWorkflow(t)
			initiatingCheckout := filepath.Join(t.TempDir(), "initiating")
			runGit(t, repo, "worktree", "add", "-b", "feature/initiating", initiatingCheckout)
			siblingWorktree := filepath.Join(t.TempDir(), "sibling")
			runGit(t, repo, "worktree", "add", "-b", "feature/sibling", siblingWorktree)
			testCase.invalidate(t, repo)

			if _, err := ResumeFeatureWorkflow(repo); err == nil || !strings.Contains(err.Error(), "policy remediation") {
				t.Fatalf("ResumeFeatureWorkflow error = %v, want policy remediation without fallback", err)
			}
			if _, err := ReviewFeatureWorkflow(repo); err == nil || !strings.Contains(err.Error(), "policy remediation") {
				t.Fatalf("ReviewFeatureWorkflow error = %v, want policy remediation without fallback", err)
			}
		})
	}
}

func prepareSCN606FeatureWorkflow(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")
	runGit(t, repo, "checkout", "-b", "feature/feature-local")
	mustWrite(t, filepath.Join(repo, ".rotta", "quality-gates.yaml"), "format: rotta.quality-gates/v1\n")
	mustWrite(t, filepath.Join(repo, "specs", "feature-local_hard_spec.md"), "# approved contract\n")
	mustWrite(t, filepath.Join(repo, "features", "feature-local.feature"), "@REQ-082 @SCN-606\nScenario: Only feature-local policy and evidence paths are active\n")
	mustWrite(t, filepath.Join(repo, "specs", "approvals", "feature-local.yaml"), "status: approved\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "test: establish SCN-606 baseline")
	baseline := runGitOutput(t, repo, "rev-parse", "HEAD")

	policy := []byte("format: rotta.quality-gates/v1\n")
	approval := []byte("status: approved\n")
	manifest := fmt.Sprintf("format: rotta.workflow-manifest/v1\nfeature_id: feature-local\nworktree: %s\nbranch: feature/feature-local\nbase_sha: %s\npolicy_path: .rotta/quality-gates.yaml\npolicy_fingerprint: %x\nspec_path: specs/feature-local_hard_spec.md\nfeature_path: features/feature-local.feature\ncheckpoint_mode: strict_per_scenario\n", repo, baseline, sha256.Sum256(policy))
	mustWrite(t, filepath.Join(repo, ".rotta", "current", "manifest.yaml"), manifest)
	state := fmt.Sprintf("format: rotta.feature-runtime-state/v1\nfeature_id: feature-local\nworktree: %s\nbranch: feature/feature-local\nbaseline_sha: %s\nmanifest_fingerprint: %x\napproval_path: specs/approvals/feature-local.yaml\napproval_fingerprint: %x\nscenario_or_slice: SCN-606\nstatus: checkpointed\n", repo, baseline, sha256.Sum256([]byte(manifest)), sha256.Sum256(approval))
	mustWrite(t, filepath.Join(repo, ".rotta", "current", "state.yaml"), state)
	mustWrite(t, filepath.Join(repo, ".rotta", "current", "tdd-log.md"), "## SCN-606\n")
	return repo, baseline
}
