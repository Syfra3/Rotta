package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSCN324_ValidFeatureApprovalAuthorizesOnlyItsApprovedScenarios(t *testing.T) {
	// REQ-001 → SCN-324 → TestSCN324_ValidFeatureApprovalAuthorizesOnlyItsApprovedScenarios
	// Scenario: A valid feature approval record authorizes its approved scenarios
	repo := t.TempDir()
	runGit(t, repo, "init")
	mustWrite(t, filepath.Join(repo, "baseline"), "approved contract baseline\n")
	runGit(t, repo, "add", "baseline")
	runGit(t, repo, "commit", "-m", "test: approved contract baseline")
	baseline := runGitOutput(t, repo, "rev-parse", "HEAD")
	mustWrite(t, filepath.Join(repo, "specs", "approvals", "unified-workflow-authority.yaml"), strings.ReplaceAll(`format: rotta.feature-approval/v2
contract_id: unified-workflow-authority
status: approved
feature_paths:
  - features/unified-workflow-authority.feature
approved_scenarios:
  - feature_path: features/unified-workflow-authority.feature
    scenario_id: SCN-324
    requirement_ids: [REQ-001]
contract_fingerprints:
  specs/hard_spec.md: matching-fingerprint
  features/unified-workflow-authority.feature: matching-fingerprint
baseline_confirmation:
  status: confirmed
  baseline_commit: 8801bf810c730720f5e01e156bb66c3c3efc4be6
`, "8801bf810c730720f5e01e156bb66c3c3efc4be6", baseline))

	approved, err := EvaluateImplementationGate(repo, ContractScope{
		SpecPath:    "specs/hard_spec.md",
		FeaturePath: "features/unified-workflow-authority.feature",
		ScenarioID:  "SCN-324",
	})
	if err != nil {
		t.Fatalf("EvaluateImplementationGate returned error: %v", err)
	}
	if !approved.Approved {
		t.Fatalf("expected SCN-324 to be authorized, got reason %q", approved.Reason)
	}

	notApproved, err := EvaluateImplementationGate(repo, ContractScope{
		SpecPath:    "specs/hard_spec.md",
		FeaturePath: "features/unified-workflow-authority.feature",
		ScenarioID:  "SCN-325",
	})
	if err != nil {
		t.Fatalf("EvaluateImplementationGate returned error: %v", err)
	}
	if notApproved.Approved {
		t.Fatalf("expected SCN-325 to remain unauthorized, got reason %q", notApproved.Reason)
	}
}

func TestSCN359_ValidStructuredScenarioReferenceAuthorizesExactScenario(t *testing.T) {
	// REQ-001 → SCN-359 → TestSCN359_ValidStructuredScenarioReferenceAuthorizesExactScenario
	// Scenario: A valid structured approved-scenario reference authorizes its exact scenario
	repo, baseline := committedApprovalBaseline(t)
	mustWrite(t, filepath.Join(repo, "specs", "hard_spec.md"), "approved specification\n")
	mustWrite(t, filepath.Join(repo, "features", "unified-workflow-authority.feature"), "@SCN-359 @REQ-001\nScenario: approved scenario\n")
	specFingerprint, err := contractFileFingerprint(repo, "specs/hard_spec.md")
	if err != nil {
		t.Fatalf("fingerprint specification: %v", err)
	}
	featureFingerprint, err := contractFileFingerprint(repo, "features/unified-workflow-authority.feature")
	if err != nil {
		t.Fatalf("fingerprint feature: %v", err)
	}
	mustWrite(t, filepath.Join(repo, "specs", "approvals", "unified-workflow-authority.yaml"), "format: rotta.feature-approval/v2\ncontract_id: unified-workflow-authority\nstatus: approved\nfeature_paths:\n  - features/unified-workflow-authority.feature\napproved_scenarios:\n  - scenario_id: SCN-359\n    requirement_ids: [REQ-001]\n    feature_path: features/unified-workflow-authority.feature\ncontract_fingerprints:\n  specs/hard_spec.md: "+specFingerprint+"\n  features/unified-workflow-authority.feature: "+featureFingerprint+"\nbaseline_confirmation:\n  status: confirmed\n  baseline_commit: "+baseline+"\n")

	decision, err := EvaluateImplementationGate(repo, ContractScope{SpecPath: "specs/hard_spec.md", FeaturePath: "features/unified-workflow-authority.feature", ScenarioID: "SCN-359"})
	if err != nil {
		t.Fatalf("EvaluateImplementationGate returned error: %v", err)
	}
	if !decision.Approved {
		t.Fatalf("expected the exact structured scenario to be authorized, got reason %q", decision.Reason)
	}
}

func TestSCN360_MalformedStructuredScenarioReferenceBlocksWorkflowProgress(t *testing.T) {
	// REQ-001 → SCN-360 → TestSCN360_MalformedStructuredScenarioReferenceBlocksWorkflowProgress
	// Scenario: A malformed structured approved-scenario reference blocks workflow progress
	for _, test := range []struct {
		name  string
		entry string
	}{
		{
			name:  "missing required requirement IDs",
			entry: "  - feature_path: features/unified-workflow-authority.feature\n    scenario_id: SCN-360\n",
		},
		{
			name:  "additional authoritative field",
			entry: "  - feature_path: features/unified-workflow-authority.feature\n    scenario_id: SCN-360\n    requirement_ids: [REQ-001]\n    authority: inherited\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo, baseline := committedApprovalBaseline(t)
			mustWrite(t, filepath.Join(repo, "specs", "hard_spec.md"), "approved specification\n")
			mustWrite(t, filepath.Join(repo, "features", "unified-workflow-authority.feature"), "@SCN-360 @REQ-001\nScenario: malformed scenario reference\n")
			specFingerprint, err := contractFileFingerprint(repo, "specs/hard_spec.md")
			if err != nil {
				t.Fatalf("fingerprint specification: %v", err)
			}
			featureFingerprint, err := contractFileFingerprint(repo, "features/unified-workflow-authority.feature")
			if err != nil {
				t.Fatalf("fingerprint feature: %v", err)
			}
			record := "format: rotta.feature-approval/v2\ncontract_id: unified-workflow-authority\nstatus: approved\nfeature_paths:\n  - features/unified-workflow-authority.feature\napproved_scenarios:\n" + test.entry + "contract_fingerprints:\n  specs/hard_spec.md: " + specFingerprint + "\n  features/unified-workflow-authority.feature: " + featureFingerprint + "\nbaseline_confirmation:\n  status: confirmed\n  baseline_commit: " + baseline + "\n"
			mustWrite(t, filepath.Join(repo, "specs", "approvals", "unified-workflow-authority.yaml"), record)

			decision, err := EvaluateImplementationGate(repo, ContractScope{SpecPath: "specs/hard_spec.md", FeaturePath: "features/unified-workflow-authority.feature", ScenarioID: "SCN-360"})
			if err != nil {
				t.Fatalf("EvaluateImplementationGate returned error: %v", err)
			}
			if decision.Approved {
				t.Fatal("expected malformed approved-scenario reference to block workflow progress")
			}
			if decision.Reason != "approved-scenario reference is malformed" {
				t.Fatalf("reason = %q, want malformed approved-scenario reference", decision.Reason)
			}
		})
	}
}

func TestSCN361_NonCanonicalScenarioPathBlocksWorkflowProgress(t *testing.T) {
	// REQ-001 → SCN-361 → TestSCN361_NonCanonicalScenarioPathBlocksWorkflowProgress
	// Scenario: A non-canonical scenario path cannot authorize a scenario
	for _, featurePath := range []string{
		"/features/unified-workflow-authority.feature",
		"features/../features/unified-workflow-authority.feature",
		"./features/unified-workflow-authority.feature",
	} {
		t.Run(featurePath, func(t *testing.T) {
			repo, baseline := committedApprovalBaseline(t)
			mustWrite(t, filepath.Join(repo, "specs", "hard_spec.md"), "approved specification\n")
			mustWrite(t, filepath.Join(repo, "features", "unified-workflow-authority.feature"), "@SCN-361 @REQ-001\nScenario: invalid feature path\n")
			specFingerprint, err := contractFileFingerprint(repo, "specs/hard_spec.md")
			if err != nil {
				t.Fatalf("fingerprint specification: %v", err)
			}
			featureFingerprint, err := contractFileFingerprint(repo, "features/unified-workflow-authority.feature")
			if err != nil {
				t.Fatalf("fingerprint feature: %v", err)
			}
			record := "format: rotta.feature-approval/v2\ncontract_id: unified-workflow-authority\nstatus: approved\nfeature_paths:\n  - features/unified-workflow-authority.feature\napproved_scenarios:\n  - feature_path: " + featurePath + "\n    scenario_id: SCN-361\n    requirement_ids: [REQ-001]\ncontract_fingerprints:\n  specs/hard_spec.md: " + specFingerprint + "\n  features/unified-workflow-authority.feature: " + featureFingerprint + "\nbaseline_confirmation:\n  status: confirmed\n  baseline_commit: " + baseline + "\n"
			mustWrite(t, filepath.Join(repo, "specs", "approvals", "unified-workflow-authority.yaml"), record)

			decision, err := EvaluateImplementationGate(repo, ContractScope{SpecPath: "specs/hard_spec.md", FeaturePath: "features/unified-workflow-authority.feature", ScenarioID: "SCN-361"})
			if err != nil {
				t.Fatalf("EvaluateImplementationGate returned error: %v", err)
			}
			if decision.Approved {
				t.Fatal("expected non-canonical approved-scenario feature path to block workflow progress")
			}
			if decision.Reason != "approved-scenario feature path is invalid" {
				t.Fatalf("reason = %q, want invalid approved-scenario feature path", decision.Reason)
			}
		})
	}
}

func TestSCN362_UnresolvedOrAmbiguousScenarioIDBlocksWorkflowProgress(t *testing.T) {
	// REQ-001 → SCN-362 → TestSCN362_UnresolvedOrAmbiguousScenarioIDBlocksWorkflowProgress
	// Scenario: An unresolved or ambiguous scenario ID cannot authorize a scenario
	for _, feature := range []string{
		"@SCN-999 @REQ-001\nScenario: another scenario\n",
		"@SCN-362 @REQ-001\nScenario: first scenario\n\n@SCN-362 @REQ-001\nScenario: second scenario\n",
	} {
		t.Run(feature, func(t *testing.T) {
			repo, baseline := committedApprovalBaseline(t)
			mustWrite(t, filepath.Join(repo, "specs", "hard_spec.md"), "approved specification\n")
			mustWrite(t, filepath.Join(repo, "features", "unified-workflow-authority.feature"), feature)
			specFingerprint, err := contractFileFingerprint(repo, "specs/hard_spec.md")
			if err != nil {
				t.Fatalf("fingerprint specification: %v", err)
			}
			featureFingerprint, err := contractFileFingerprint(repo, "features/unified-workflow-authority.feature")
			if err != nil {
				t.Fatalf("fingerprint feature: %v", err)
			}
			record := "format: rotta.feature-approval/v2\ncontract_id: unified-workflow-authority\nstatus: approved\nfeature_paths:\n  - features/unified-workflow-authority.feature\napproved_scenarios:\n  - feature_path: features/unified-workflow-authority.feature\n    scenario_id: SCN-362\n    requirement_ids: [REQ-001]\ncontract_fingerprints:\n  specs/hard_spec.md: " + specFingerprint + "\n  features/unified-workflow-authority.feature: " + featureFingerprint + "\nbaseline_confirmation:\n  status: confirmed\n  baseline_commit: " + baseline + "\n"
			mustWrite(t, filepath.Join(repo, "specs", "approvals", "unified-workflow-authority.yaml"), record)

			decision, err := EvaluateImplementationGate(repo, ContractScope{SpecPath: "specs/hard_spec.md", FeaturePath: "features/unified-workflow-authority.feature", ScenarioID: "SCN-362"})
			if err != nil {
				t.Fatalf("EvaluateImplementationGate returned error: %v", err)
			}
			if decision.Approved {
				t.Fatal("expected an unresolved or ambiguous scenario ID to block workflow progress")
			}
			if decision.Reason != "approved-scenario ID did not resolve exactly once" {
				t.Fatalf("reason = %q, want scenario ID resolution failure", decision.Reason)
			}
		})
	}
}

func TestSCN363_RequirementTagMismatchBlocksWorkflowProgress(t *testing.T) {
	// REQ-001 → SCN-363 → TestSCN363_RequirementTagMismatchBlocksWorkflowProgress
	// Scenario: A requirement-tag mismatch cannot authorize a scenario
	repo, baseline := committedApprovalBaseline(t)
	mustWrite(t, filepath.Join(repo, "specs", "hard_spec.md"), "approved specification\n")
	mustWrite(t, filepath.Join(repo, "features", "unified-workflow-authority.feature"), "@SCN-363 @REQ-002\nScenario: requirement mismatch\n")
	specFingerprint, err := contractFileFingerprint(repo, "specs/hard_spec.md")
	if err != nil {
		t.Fatalf("fingerprint specification: %v", err)
	}
	featureFingerprint, err := contractFileFingerprint(repo, "features/unified-workflow-authority.feature")
	if err != nil {
		t.Fatalf("fingerprint feature: %v", err)
	}
	record := "format: rotta.feature-approval/v2\ncontract_id: unified-workflow-authority\nstatus: approved\nfeature_paths:\n  - features/unified-workflow-authority.feature\napproved_scenarios:\n  - feature_path: features/unified-workflow-authority.feature\n    scenario_id: SCN-363\n    requirement_ids: [REQ-001]\ncontract_fingerprints:\n  specs/hard_spec.md: " + specFingerprint + "\n  features/unified-workflow-authority.feature: " + featureFingerprint + "\nbaseline_confirmation:\n  status: confirmed\n  baseline_commit: " + baseline + "\n"
	mustWrite(t, filepath.Join(repo, "specs", "approvals", "unified-workflow-authority.yaml"), record)

	decision, err := EvaluateImplementationGate(repo, ContractScope{SpecPath: "specs/hard_spec.md", FeaturePath: "features/unified-workflow-authority.feature", ScenarioID: "SCN-363"})
	if err != nil {
		t.Fatalf("EvaluateImplementationGate returned error: %v", err)
	}
	if decision.Approved {
		t.Fatal("expected requirement-tag mismatch to block workflow progress")
	}
	if decision.Reason != "approved-scenario requirement IDs do not match feature requirement tags" {
		t.Fatalf("reason = %q, want requirement-ID mismatch", decision.Reason)
	}
}

func TestSCN364_DuplicateScenarioIdentityBlocksAuthorization(t *testing.T) {
	// REQ-001 → SCN-364 → TestSCN364_DuplicateScenarioIdentityBlocksAuthorization
	// Scenario: Duplicate approved scenario identity blocks authorization
	for _, test := range []struct {
		name              string
		duplicateEntry    string
		otherActiveRecord string
	}{
		{
			name:           "duplicated in feature scoped record",
			duplicateEntry: "  - feature_path: features/unified-workflow-authority.feature\n    scenario_id: SCN-364\n    requirement_ids: [REQ-001]\n",
		},
		{
			name:              "present in another active feature record",
			otherActiveRecord: "format: rotta.feature-approval/v2\ncontract_id: other\nstatus: approved\nfeature_paths:\n  - features/other.feature\napproved_scenarios:\n  - feature_path: features/other.feature\n    scenario_id: SCN-364\n    requirement_ids: [REQ-001]\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo, baseline := committedApprovalBaseline(t)
			mustWrite(t, filepath.Join(repo, "specs", "hard_spec.md"), "approved specification\n")
			mustWrite(t, filepath.Join(repo, "features", "unified-workflow-authority.feature"), "@SCN-364 @REQ-001\nScenario: duplicate identity\n")
			specFingerprint, err := contractFileFingerprint(repo, "specs/hard_spec.md")
			if err != nil {
				t.Fatalf("fingerprint specification: %v", err)
			}
			featureFingerprint, err := contractFileFingerprint(repo, "features/unified-workflow-authority.feature")
			if err != nil {
				t.Fatalf("fingerprint feature: %v", err)
			}
			record := "format: rotta.feature-approval/v2\ncontract_id: unified-workflow-authority\nstatus: approved\nfeature_paths:\n  - features/unified-workflow-authority.feature\napproved_scenarios:\n  - feature_path: features/unified-workflow-authority.feature\n    scenario_id: SCN-364\n    requirement_ids: [REQ-001]\n" + test.duplicateEntry + "contract_fingerprints:\n  specs/hard_spec.md: " + specFingerprint + "\n  features/unified-workflow-authority.feature: " + featureFingerprint + "\nbaseline_confirmation:\n  status: confirmed\n  baseline_commit: " + baseline + "\n"
			mustWrite(t, filepath.Join(repo, "specs", "approvals", "unified-workflow-authority.yaml"), record)
			if test.otherActiveRecord != "" {
				mustWrite(t, filepath.Join(repo, "specs", "approvals", "other.yaml"), test.otherActiveRecord)
			}

			decision, err := EvaluateImplementationGate(repo, ContractScope{SpecPath: "specs/hard_spec.md", FeaturePath: "features/unified-workflow-authority.feature", ScenarioID: "SCN-364"})
			if err != nil {
				t.Fatalf("EvaluateImplementationGate returned error: %v", err)
			}
			if decision.Approved {
				t.Fatal("expected duplicate scenario identity to block authorization")
			}
			if decision.Reason != "duplicate approved-scenario identity" {
				t.Fatalf("reason = %q, want duplicate scenario identity", decision.Reason)
			}
		})
	}
}

func TestSCN325_InvalidFeatureApprovalFailsClosedWithSpecificReason(t *testing.T) {
	// REQ-001 → SCN-325 → TestSCN325_InvalidFeatureApprovalFailsClosedWithSpecificReason
	// Scenario: An invalid approval record fails closed with its specific reason
	decision, err := EvaluateImplementationGate(t.TempDir(), ContractScope{
		SpecPath:    "specs/hard_spec.md",
		FeaturePath: "features/unified-workflow-authority.feature",
		ScenarioID:  "SCN-325",
	})
	if err != nil {
		t.Fatalf("EvaluateImplementationGate returned error: %v", err)
	}
	if decision.Approved {
		t.Fatal("expected a missing approval record to block workflow activity")
	}
	if decision.Reason != "approval record is missing" {
		t.Fatalf("reason = %q, want %q", decision.Reason, "approval record is missing")
	}

	t.Run("malformed", func(t *testing.T) {
		repo := t.TempDir()
		mustWrite(t, filepath.Join(repo, "specs", "approvals", "unified-workflow-authority.yaml"), "format: rotta.feature-approval/v2\n")

		decision, err := EvaluateImplementationGate(repo, ContractScope{
			SpecPath:    "specs/hard_spec.md",
			FeaturePath: "features/unified-workflow-authority.feature",
			ScenarioID:  "SCN-325",
		})
		if err != nil {
			t.Fatalf("EvaluateImplementationGate returned error: %v", err)
		}
		if decision.Approved {
			t.Fatal("expected a malformed approval record to block workflow activity")
		}
		if decision.Reason != "approval record is malformed" {
			t.Fatalf("reason = %q, want %q", decision.Reason, "approval record is malformed")
		}
	})

	t.Run("uncommitted baseline", func(t *testing.T) {
		repo := t.TempDir()
		mustWrite(t, filepath.Join(repo, "specs", "approvals", "unified-workflow-authority.yaml"), validSCN325ApprovalRecord)

		decision, err := EvaluateImplementationGate(repo, ContractScope{
			SpecPath:    "specs/hard_spec.md",
			FeaturePath: "features/unified-workflow-authority.feature",
			ScenarioID:  "SCN-325",
		})
		if err != nil {
			t.Fatalf("EvaluateImplementationGate returned error: %v", err)
		}
		if decision.Approved {
			t.Fatal("expected an uncommitted baseline to block workflow activity")
		}
		if decision.Reason != "approval baseline is not committed" {
			t.Fatalf("reason = %q, want %q", decision.Reason, "approval baseline is not committed")
		}
	})

	t.Run("unreachable baseline", func(t *testing.T) {
		repo := t.TempDir()
		runGit(t, repo, "init")
		mustWrite(t, filepath.Join(repo, "baseline"), "unreachable baseline\n")
		runGit(t, repo, "add", "baseline")
		runGit(t, repo, "commit", "-m", "test: unreachable baseline")
		baseline := runGitOutput(t, repo, "rev-parse", "HEAD")
		runGit(t, repo, "checkout", "--orphan", "current")
		runGit(t, repo, "rm", "-rf", ".")
		mustWrite(t, filepath.Join(repo, "current"), "current history\n")
		runGit(t, repo, "add", "current")
		runGit(t, repo, "commit", "-m", "test: current history")
		mustWrite(t, filepath.Join(repo, "specs", "approvals", "unified-workflow-authority.yaml"), strings.ReplaceAll(validSCN325ApprovalRecord, "8801bf810c730720f5e01e156bb66c3c3efc4be6", baseline))

		decision, err := EvaluateImplementationGate(repo, ContractScope{SpecPath: "specs/hard_spec.md", FeaturePath: "features/unified-workflow-authority.feature", ScenarioID: "SCN-325"})
		if err != nil {
			t.Fatalf("EvaluateImplementationGate returned error: %v", err)
		}
		if decision.Approved {
			t.Fatal("expected an unreachable baseline to block workflow activity")
		}
		if decision.Reason != "approval baseline is unreachable" {
			t.Fatalf("reason = %q, want %q", decision.Reason, "approval baseline is unreachable")
		}
	})

	t.Run("feature identity mismatch", func(t *testing.T) {
		repo, baseline := committedApprovalBaseline(t)
		record := strings.ReplaceAll(validSCN325ApprovalRecord, "8801bf810c730720f5e01e156bb66c3c3efc4be6", baseline)
		record = strings.Replace(record, "- features/unified-workflow-authority.feature", "- features/other.feature", 1)
		mustWrite(t, filepath.Join(repo, "specs", "approvals", "unified-workflow-authority.yaml"), record)

		decision, err := EvaluateImplementationGate(repo, ContractScope{SpecPath: "specs/hard_spec.md", FeaturePath: "features/unified-workflow-authority.feature", ScenarioID: "SCN-325"})
		if err != nil {
			t.Fatalf("EvaluateImplementationGate returned error: %v", err)
		}
		if decision.Approved {
			t.Fatal("expected a feature identity mismatch to block workflow activity")
		}
		if decision.Reason != "approval record has an identity or scenario-scope mismatch" {
			t.Fatalf("reason = %q, want %q", decision.Reason, "approval record has an identity or scenario-scope mismatch")
		}
	})

	t.Run("contract fingerprint drift", func(t *testing.T) {
		repo, baseline := committedApprovalBaseline(t)
		mustWrite(t, filepath.Join(repo, "specs", "hard_spec.md"), "approved specification\n")
		mustWrite(t, filepath.Join(repo, "features", "unified-workflow-authority.feature"), "@SCN-325\nFeature: authority\n")
		record := strings.ReplaceAll(validSCN325ApprovalRecord, "8801bf810c730720f5e01e156bb66c3c3efc4be6", baseline)
		mustWrite(t, filepath.Join(repo, "specs", "approvals", "unified-workflow-authority.yaml"), record)

		decision, err := EvaluateImplementationGate(repo, ContractScope{SpecPath: "specs/hard_spec.md", FeaturePath: "features/unified-workflow-authority.feature", ScenarioID: "SCN-325"})
		if err != nil {
			t.Fatalf("EvaluateImplementationGate returned error: %v", err)
		}
		if decision.Approved {
			t.Fatal("expected contract fingerprint drift to block workflow activity")
		}
		if decision.Reason != "approval record has contract fingerprint drift" {
			t.Fatalf("reason = %q, want %q", decision.Reason, "approval record has contract fingerprint drift")
		}
	})
}

func TestSCN326_ApprovalAuthorityIsIsolatedBetweenFeatureWorktrees(t *testing.T) {
	// REQ-001 → SCN-326 → TestSCN326_ApprovalAuthorityIsIsolatedBetweenFeatureWorktrees
	// Scenario: Approval authority remains isolated between feature worktrees
	firstWorktree, firstBaseline := committedApprovalBaseline(t)
	secondWorktree, secondBaseline := committedApprovalBaseline(t)
	firstRecord := strings.ReplaceAll(validSCN325ApprovalRecord, "8801bf810c730720f5e01e156bb66c3c3efc4be6", firstBaseline) + "submission_worktree: " + firstWorktree + "\n"
	secondRecord := strings.ReplaceAll(validSCN325ApprovalRecord, "8801bf810c730720f5e01e156bb66c3c3efc4be6", secondBaseline) + "submission_worktree: " + secondWorktree + "\n"

	mustWrite(t, filepath.Join(firstWorktree, "specs", "approvals", "unified-workflow-authority.yaml"), firstRecord)
	mustWrite(t, filepath.Join(secondWorktree, "specs", "approvals", "unified-workflow-authority.yaml"), secondRecord)
	scope := ContractScope{SpecPath: "specs/hard_spec.md", FeaturePath: "features/unified-workflow-authority.feature", ScenarioID: "SCN-325"}
	for _, repoRoot := range []string{firstWorktree, secondWorktree} {
		decision, err := EvaluateImplementationGate(repoRoot, scope)
		if err != nil {
			t.Fatalf("EvaluateImplementationGate returned error: %v", err)
		}
		if !decision.Approved {
			t.Fatalf("expected worktree %q to authorize its own record, got reason %q", repoRoot, decision.Reason)
		}
	}

	mustWrite(t, filepath.Join(secondWorktree, "specs", "approvals", "unified-workflow-authority.yaml"), firstRecord)

	decision, err := EvaluateImplementationGate(secondWorktree, scope)
	if err != nil {
		t.Fatalf("EvaluateImplementationGate returned error: %v", err)
	}
	if decision.Approved {
		t.Fatal("expected another worktree's record with the same scenario ID to be rejected")
	}
}

func TestSCN327_PartialHumanApprovalSelectsOnlyEligibleScenarios(t *testing.T) {
	// REQ-001 → SCN-327 → TestSCN327_PartialHumanApprovalSelectsOnlyEligibleScenarios
	// Scenario: A partial human approval limits eligible scenarios
	repo, baseline := committedApprovalBaseline(t)
	record := strings.ReplaceAll(validSCN325ApprovalRecord, "SCN-325", "SCN-324")
	record = strings.ReplaceAll(record, "8801bf810c730720f5e01e156bb66c3c3efc4be6", baseline)
	record = strings.Replace(record, "    requirement_ids: [REQ-001]\ncontract_fingerprints:", "    requirement_ids: [REQ-001]\n  - feature_path: features/unified-workflow-authority.feature\n    scenario_id: SCN-326\n    requirement_ids: [REQ-001]\ncontract_fingerprints:", 1)
	mustWrite(t, filepath.Join(repo, "specs", "approvals", "unified-workflow-authority.yaml"), record)

	selected, err := SelectApprovedScenarios(repo, ContractScope{
		SpecPath:    "specs/hard_spec.md",
		FeaturePath: "features/unified-workflow-authority.feature",
	}, []string{"SCN-324", "SCN-325", "SCN-326"})
	if err != nil {
		t.Fatalf("SelectApprovedScenarios returned error: %v", err)
	}
	if got, want := strings.Join(selected, ","), "SCN-324,SCN-326"; got != want {
		t.Fatalf("selected scenarios = %q, want %q", got, want)
	}
}

func TestSCN328_LegacyArtifactsDoNotAuthorizeOrAffectReview(t *testing.T) {
	// REQ-002 → SCN-328 → TestSCN328_LegacyArtifactsDoNotAuthorizeOrAffectReview
	// Scenario: Legacy markers do not authorize a workflow
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "specs", ".approved"), "SCN-328\n")
	mustWrite(t, filepath.Join(repo, "specs", "approvals", "hard_spec.approved"), "SCN-328\n")
	mustWrite(t, filepath.Join(repo, ".rotta", "tdd-log.md"), "[GREEN] SCN-328\n")

	decision, err := EvaluateImplementationGate(repo, ContractScope{
		SpecPath:    "specs/hard_spec.md",
		FeaturePath: "features/unified-workflow-authority.feature",
		ScenarioID:  "SCN-328",
	})
	if err != nil {
		t.Fatalf("EvaluateImplementationGate returned error: %v", err)
	}
	if decision.Approved || decision.Reason != "approval record is missing" {
		t.Errorf("legacy approval artifacts produced decision %#v, want fresh-flow missing approval", decision)
	}

	mustWrite(t, filepath.Join(repo, "features", "unified-workflow-authority.feature"), "@SCN-328\n")
	mustWrite(t, filepath.Join(repo, ".rotta", "current", "manifest.yaml"), "submission_id: unified-workflow-authority\nspec_path: specs/hard_spec.md\nfeature_paths:\n  - features/unified-workflow-authority.feature\nscenario_ids:\n  - SCN-328\nworktree: "+repo+"\nstatus: in_progress\n")
	mustWrite(t, filepath.Join(repo, ".rotta", "current", "tdd-log.md"), "## SCN-328\n")
	review, err := ReviewCurrentSubmission(repo)
	if err != nil {
		t.Fatalf("ReviewCurrentSubmission returned error: %v", err)
	}
	if !review.Passed || len(review.Warnings) != 0 {
		t.Errorf("legacy artifacts affected review result: %#v", review)
	}
}

const validSCN325ApprovalRecord = `format: rotta.feature-approval/v2
contract_id: unified-workflow-authority
status: approved
feature_paths:
  - features/unified-workflow-authority.feature
approved_scenarios:
  - feature_path: features/unified-workflow-authority.feature
    scenario_id: SCN-325
    requirement_ids: [REQ-001]
contract_fingerprints:
  specs/hard_spec.md: matching-fingerprint
  features/unified-workflow-authority.feature: matching-fingerprint
baseline_confirmation:
  status: confirmed
  baseline_commit: 8801bf810c730720f5e01e156bb66c3c3efc4be6
`

func committedApprovalBaseline(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	runGit(t, repo, "init")
	mustWrite(t, filepath.Join(repo, "baseline"), "approved contract baseline\n")
	runGit(t, repo, "add", "baseline")
	runGit(t, repo, "commit", "-m", "test: approved contract baseline")
	return repo, runGitOutput(t, repo, "rev-parse", "HEAD")
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
