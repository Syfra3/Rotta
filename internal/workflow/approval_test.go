package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSCN018_PendingContractRequiresScopedApproval(t *testing.T) {
	// REQ-015 → SCN-018 → TestSCN018_PendingContractRequiresScopedApproval
	// Scenario: Pending generated contracts do not pass the implementation gate
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "specs", ".approved"), "SCN-018\n")
	mustWrite(t, filepath.Join(repo, "specs", "workflow_artifact_lifecycle.md"), "# pending spec\n")
	mustWrite(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle.feature"), "@REQ-015 @SCN-018\nScenario: Pending generated contracts do not pass the implementation gate\n")

	decision, err := EvaluateImplementationGate(repo, ContractScope{
		SpecPath:    "specs/workflow_artifact_lifecycle.md",
		FeaturePath: "features/workflow_artifact_lifecycle.feature",
		ScenarioID:  "SCN-018",
	})
	if err != nil {
		t.Fatalf("EvaluateImplementationGate returned error: %v", err)
	}
	if decision.Approved {
		t.Fatalf("expected pending contract to fail closed despite legacy specs/.approved marker")
	}
	if !strings.Contains(decision.Reason, "human approval is still required") {
		t.Fatalf("expected human approval required message, got %q", decision.Reason)
	}
}

func TestSCN018_ScopedApprovalAllowsImplementationGate(t *testing.T) {
	// REQ-015 → SCN-018 → TestSCN018_ScopedApprovalAllowsImplementationGate
	// Scenario: Pending generated contracts do not pass the implementation gate
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "specs", "approvals", "workflow_artifact_lifecycle.approved"), "SCN-018\n")

	decision, err := EvaluateImplementationGate(repo, workflowArtifactLifecycleScope())
	if err != nil {
		t.Fatalf("EvaluateImplementationGate returned error: %v", err)
	}
	if !decision.Approved {
		t.Fatalf("expected scoped approval to allow implementation gate, got reason %q", decision.Reason)
	}
	if !strings.Contains(decision.Reason, "scoped human approval recorded") {
		t.Fatalf("expected scoped approval reason, got %q", decision.Reason)
	}
}

func TestSCN018_FeatureQualifiedScopedApprovalAllowsImplementationGate(t *testing.T) {
	// REQ-015 → SCN-018 → TestSCN018_FeatureQualifiedScopedApprovalAllowsImplementationGate
	// Scenario: Pending generated contracts do not pass the implementation gate
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "specs", "approvals", "workflow_artifact_lifecycle.approved"), "features/workflow_artifact_lifecycle.feature#SCN-018\n")

	decision, err := EvaluateImplementationGate(repo, workflowArtifactLifecycleScope())
	if err != nil {
		t.Fatalf("EvaluateImplementationGate returned error: %v", err)
	}
	if !decision.Approved {
		t.Fatalf("expected feature-qualified scoped approval to allow implementation gate, got reason %q", decision.Reason)
	}
}

func TestSCN018_MissingScopedApprovalFileFailsClosed(t *testing.T) {
	// REQ-015 → SCN-018 → TestSCN018_MissingScopedApprovalFileFailsClosed
	// Scenario: Pending generated contracts do not pass the implementation gate
	decision, err := EvaluateImplementationGate(t.TempDir(), workflowArtifactLifecycleScope())
	if err != nil {
		t.Fatalf("EvaluateImplementationGate returned error: %v", err)
	}
	if decision.Approved {
		t.Fatalf("expected missing scoped approval file to fail closed")
	}
}

func TestSCN018_UnreadableScopedApprovalFileReturnsError(t *testing.T) {
	// REQ-015 → SCN-018 → TestSCN018_UnreadableScopedApprovalFileReturnsError
	// Scenario: Pending generated contracts do not pass the implementation gate
	repo := t.TempDir()
	approvalPath := filepath.Join(repo, "specs", "approvals", "workflow_artifact_lifecycle.approved")
	if err := os.MkdirAll(approvalPath, 0o755); err != nil {
		t.Fatalf("create unreadable approval path: %v", err)
	}

	if _, err := EvaluateImplementationGate(repo, workflowArtifactLifecycleScope()); err == nil {
		t.Fatalf("expected unreadable scoped approval path to return an error")
	}
}

func TestSCN018_MalformedScopedApprovalFileReturnsError(t *testing.T) {
	// REQ-015 → SCN-018 → TestSCN018_MalformedScopedApprovalFileReturnsError
	// Scenario: Pending generated contracts do not pass the implementation gate
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "specs", "approvals", "workflow_artifact_lifecycle.approved"), strings.Repeat("x", 65*1024))

	if _, err := EvaluateImplementationGate(repo, workflowArtifactLifecycleScope()); err == nil {
		t.Fatalf("expected malformed scoped approval file to return an error")
	}
}

func TestSCN324_ValidFeatureApprovalAuthorizesOnlyItsApprovedScenarios(t *testing.T) {
	// REQ-001 → SCN-324 → TestSCN324_ValidFeatureApprovalAuthorizesOnlyItsApprovedScenarios
	// Scenario: A valid feature approval record authorizes its approved scenarios
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "specs", "approvals", "unified-workflow-authority.yaml"), `format: rotta.feature-approval/v2
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
`)

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

func workflowArtifactLifecycleScope() ContractScope {
	return ContractScope{
		SpecPath:    "specs/workflow_artifact_lifecycle.md",
		FeaturePath: "features/workflow_artifact_lifecycle.feature",
		ScenarioID:  "SCN-018",
	}
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
