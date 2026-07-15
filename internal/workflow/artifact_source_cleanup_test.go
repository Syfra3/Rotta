package workflow

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSCN019_CleanTreePlanningRequiresScopedApproval(t *testing.T) {
	// REQ-015 → REQ-020 → SCN-019 → TestSCN019_CleanTreePlanningRequiresScopedApproval
	// Scenario: Untracked active contracts are tracked instead of deleted to clean the tree
	repo := t.TempDir()
	runGit(t, repo, "init")
	mustWrite(t, filepath.Join(repo, "specs", "workflow_artifact_lifecycle.md"), "# approved hard spec\n")
	mustWrite(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle.feature"), "@REQ-015 @REQ-020 @SCN-019\nScenario: Untracked active contracts are tracked instead of deleted to clean the tree\n")

	plan, err := PlanCleanTreeContractActions(repo, ContractScope{
		SpecPath:    "specs/workflow_artifact_lifecycle.md",
		FeaturePath: "features/workflow_artifact_lifecycle.feature",
		ScenarioID:  "SCN-019",
	})

	if err == nil {
		t.Fatalf("expected scoped approval error, got nil")
	}
	if !strings.Contains(err.Error(), "human approval is still required") {
		t.Fatalf("expected human approval error, got %v", err)
	}
	if plan != nil {
		t.Fatalf("expected no cleanup actions without scoped approval, got %#v", plan)
	}
	assertFileContent(t, filepath.Join(repo, "specs", "workflow_artifact_lifecycle.md"), "# approved hard spec\n")
	assertFileContent(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle.feature"), "@REQ-015 @REQ-020 @SCN-019\nScenario: Untracked active contracts are tracked instead of deleted to clean the tree\n")
}

func TestSCN024_WorkflowCleanupGuidanceLabelsArtifactLifecycleActions(t *testing.T) {
	// REQ-020 → SCN-024 → TestSCN024_WorkflowCleanupGuidanceLabelsArtifactLifecycleActions
	// Scenario: Workflow cleanup explains artifact lifecycle actions explicitly
	inputs := []WorkflowArtifactLifecycleInput{
		{Path: "specs/workflow_artifact_lifecycle.md", Approved: true, Implemented: true},
		{Path: "features/workflow_artifact_lifecycle.feature", Approved: true, Implemented: true},
		{Path: "features/pending_contract.feature"},
		{Path: "docs/old_process.md", ProcessOnly: true, RetirementReason: "temporary implementation notes"},
		{Path: "docs/review_checklist.md"},
		{Path: ".vela/graph.db"},
		{Path: "captures/home/config.json", Content: `{"token":"fake-token-for-test"}`},
	}

	report := PrepareWorkflowArtifactCleanupGuidance(inputs)

	assertCleanupGuidanceAction(t, report, "specs/workflow_artifact_lifecycle.md", WorkflowArtifactCleanupTrack)
	assertCleanupGuidanceAction(t, report, "features/workflow_artifact_lifecycle.feature", WorkflowArtifactCleanupTrack)
	assertCleanupGuidanceAction(t, report, "features/pending_contract.feature", WorkflowArtifactCleanupKeepPending)
	assertCleanupGuidanceAction(t, report, "docs/old_process.md", WorkflowArtifactCleanupArchive)
	assertCleanupGuidanceAction(t, report, "docs/review_checklist.md", WorkflowArtifactCleanupTrack)
	assertCleanupGuidanceAction(t, report, ".vela/graph.db", WorkflowArtifactCleanupIgnore)
	assertCleanupGuidanceAction(t, report, "captures/home/config.json", WorkflowArtifactCleanupDelete)
	assertCleanupGuidanceDoesNotUseAction(t, report, "features/workflow_artifact_lifecycle.feature", WorkflowArtifactCleanupDelete)
	assertCleanupGuidanceReason(t, report, "features/pending_contract.feature", "pending contract remains pending until human approval")
	assertCleanupGuidanceReason(t, report, "features/workflow_artifact_lifecycle.feature", "active behavior contract remains tracked")
}

func TestSCN024_WorkflowCleanupGuidanceRequiresContractPathAndApprovalForActiveTrackingReason(t *testing.T) {
	// REQ-020 → SCN-024 → TestSCN024_WorkflowCleanupGuidanceRequiresContractPathAndApprovalForActiveTrackingReason
	// Scenario: Workflow cleanup explains artifact lifecycle actions explicitly
	report := PrepareWorkflowArtifactCleanupGuidance([]WorkflowArtifactLifecycleInput{
		{Path: "docs/workflow_artifact_lifecycle.md", Approved: true, Implemented: true},
		{Path: "features/pending_contract.feature"},
	})

	assertCleanupGuidanceAction(t, report, "docs/workflow_artifact_lifecycle.md", WorkflowArtifactCleanupTrack)
	assertCleanupGuidanceReason(t, report, "docs/workflow_artifact_lifecycle.md", "project artifact remains tracked for review")
	assertCleanupGuidanceAction(t, report, "features/pending_contract.feature", WorkflowArtifactCleanupKeepPending)
}

func TestSCN024_WorkflowCleanupGuidanceDoesNotTreatFeatureExtensionOutsideFeaturesAsPendingContract(t *testing.T) {
	// REQ-020 → SCN-024 → TestSCN024_WorkflowCleanupGuidanceDoesNotTreatFeatureExtensionOutsideFeaturesAsPendingContract
	// Scenario: Workflow cleanup explains artifact lifecycle actions explicitly
	report := PrepareWorkflowArtifactCleanupGuidance([]WorkflowArtifactLifecycleInput{
		{Path: "docs/pending_contract.feature"},
	})

	assertCleanupGuidanceAction(t, report, "docs/pending_contract.feature", WorkflowArtifactCleanupTrack)
	assertCleanupGuidanceReason(t, report, "docs/pending_contract.feature", "project artifact remains tracked for review")
	assertCleanupGuidanceDoesNotUseAction(t, report, "docs/pending_contract.feature", WorkflowArtifactCleanupKeepPending)
}

func TestSCN024_WorkflowCleanupGuidanceKeepsPendingContractBeforeArchiveCandidate(t *testing.T) {
	// REQ-020 → SCN-024 → TestSCN024_WorkflowCleanupGuidanceKeepsPendingContractBeforeArchiveCandidate
	// Scenario: Workflow cleanup explains artifact lifecycle actions explicitly
	inputs := []WorkflowArtifactLifecycleInput{
		{Path: "specs/pending_contract.md", ProcessOnly: true, RetirementReason: "stale draft contract"},
	}

	report := PrepareWorkflowArtifactCleanupGuidance(inputs)

	assertCleanupGuidanceAction(t, report, "specs/pending_contract.md", WorkflowArtifactCleanupKeepPending)
	assertCleanupGuidanceDoesNotUseAction(t, report, "specs/pending_contract.md", WorkflowArtifactCleanupArchive)
	assertCleanupGuidanceReason(t, report, "specs/pending_contract.md", "pending contract remains pending until human approval")
}
