package workflow

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSCN012_TrackedHardSpecAndFeatureAreAuthoritativeContractSources(t *testing.T) {
	// REQ-011 → REQ-012 → SCN-012 → TestSCN012_TrackedHardSpecAndFeatureAreAuthoritativeContractSources
	// Scenario: Active hard spec and feature files are tracked as the contract source of truth
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")

	mustWrite(t, filepath.Join(repo, "specs", "workflow_artifact_lifecycle.md"), "# approved hard spec\n")
	mustWrite(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle.feature"), "@REQ-011 @REQ-012 @SCN-012\nScenario: Active hard spec and feature files are tracked as the contract source of truth\n")
	runGit(t, repo, "add", "specs/workflow_artifact_lifecycle.md", "features/workflow_artifact_lifecycle.feature")
	runGit(t, repo, "commit", "-m", "test: track approved contract artifacts")

	status, err := EvaluateContractSourceOfTruth(repo, ContractScope{
		SpecPath:    "specs/workflow_artifact_lifecycle.md",
		FeaturePath: "features/workflow_artifact_lifecycle.feature",
		ScenarioID:  "SCN-012",
	})
	if err != nil {
		t.Fatalf("EvaluateContractSourceOfTruth returned error: %v", err)
	}
	if !status.Authoritative {
		t.Fatalf("expected tracked spec and feature to be authoritative, got: %#v", status)
	}
	if !status.SpecTracked || !status.FeatureTracked {
		t.Fatalf("expected both contract files to be tracked, got: %#v", status)
	}
	if status.RequiresAncoraContractText {
		t.Fatalf("expected repository files to recover behavior without full Ancora contract text")
	}
}

func TestSCN013_NamespacedWorkflowPolicyArtifactsDoNotOverwriteExistingActiveContract(t *testing.T) {
	// REQ-011 → REQ-020 → SCN-013 → TestSCN013_NamespacedWorkflowPolicyArtifactsDoNotOverwriteExistingActiveContract
	// Scenario: Namespaced workflow-policy artifacts do not overwrite an existing active contract
	repo := t.TempDir()
	existingSpec := "# Active installer recovery hard spec\n"
	existingFeature := "Feature: Installer recovery\n"
	newSpec := "# Workflow Artifact Lifecycle\n"
	newFeature := "Feature: Workflow artifact lifecycle\n"
	mustWrite(t, filepath.Join(repo, "specs", "hard_spec.md"), existingSpec)
	mustWrite(t, filepath.Join(repo, "features", "installer_recovery.feature"), existingFeature)

	artifacts, err := GenerateNamespacedWorkflowPolicyArtifacts(repo, WorkflowPolicyArtifactRequest{
		ContractID:        "workflow_artifact_lifecycle",
		HardSpec:          newSpec,
		Feature:           newFeature,
		LegacySpecPath:    "specs/hard_spec.md",
		LegacyFeaturePath: "features/installer_recovery.feature",
	})
	if err != nil {
		t.Fatalf("GenerateNamespacedWorkflowPolicyArtifacts returned error: %v", err)
	}

	if artifacts.SpecPath != "specs/workflow_artifact_lifecycle.md" {
		t.Fatalf("expected namespaced hard spec path, got %q", artifacts.SpecPath)
	}
	if artifacts.FeaturePath != "features/workflow_artifact_lifecycle.feature" {
		t.Fatalf("expected namespaced feature path, got %q", artifacts.FeaturePath)
	}
	assertFileContent(t, filepath.Join(repo, artifacts.SpecPath), newSpec)
	assertFileContent(t, filepath.Join(repo, artifacts.FeaturePath), newFeature)
	assertFileContent(t, filepath.Join(repo, "specs", "hard_spec.md"), existingSpec)
	assertFileContent(t, filepath.Join(repo, "features", "installer_recovery.feature"), existingFeature)
}

func TestSCN014_ImplementedFeatureFileClassifiesAsActiveRegressionContract(t *testing.T) {
	// REQ-012 → REQ-016 → SCN-014 → TestSCN014_ImplementedFeatureFileClassifiesAsActiveRegressionContract
	// Scenario: Implemented feature files remain active regression contracts
	classification := ClassifyWorkflowArtifactLifecycle(WorkflowArtifactLifecycleInput{
		Path:        "features/workflow_artifact_lifecycle.feature",
		Implemented: true,
		Approved:    true,
	})

	if classification.Kind != WorkflowArtifactActiveRegressionContract {
		t.Fatalf("expected active regression contract classification, got %#v", classification)
	}
	if classification.ArchiveCandidate {
		t.Fatalf("implemented active feature must not be an archive candidate merely because complete: %#v", classification)
	}
}

func TestSCN014_ArchivePreparationKeepsImplementedActiveFeatureUnderFeatures(t *testing.T) {
	// REQ-012 → REQ-016 → SCN-014 → TestSCN014_ArchivePreparationKeepsImplementedActiveFeatureUnderFeatures
	// Scenario: Implemented feature files remain active regression contracts
	repo := t.TempDir()
	featurePath := filepath.Join(repo, "features", "workflow_artifact_lifecycle.feature")
	featureBody := "@REQ-012 @REQ-016 @SCN-014\nScenario: Implemented feature files remain active regression contracts\n"
	mustWrite(t, featurePath, featureBody)
	mustWrite(t, filepath.Join(repo, "specs", "approvals", "workflow_artifact_lifecycle.approved"), "SCN-014\n")
	mustWrite(t, filepath.Join(repo, "specs", ".implementation-complete"), "completed_scenarios:\n  - SCN-014\n")

	plan, err := PrepareCompletedChangeArchive(repo)
	if err != nil {
		t.Fatalf("PrepareCompletedChangeArchive returned error: %v", err)
	}

	assertArchivePlanKeepsPath(t, plan, "features/workflow_artifact_lifecycle.feature")
	assertFileContent(t, featurePath, featureBody)
}

func TestSCN014_ArchivePreparationHasNoMovePlanWithoutCompletionMarker(t *testing.T) {
	// REQ-012 → REQ-016 → SCN-014 → TestSCN014_ArchivePreparationHasNoMovePlanWithoutCompletionMarker
	// Scenario: Implemented feature files remain active regression contracts
	plan, err := PrepareCompletedChangeArchive(t.TempDir())
	if err != nil {
		t.Fatalf("PrepareCompletedChangeArchive returned error: %v", err)
	}
	if len(plan.KeptActivePaths) != 0 {
		t.Fatalf("expected no archive preparation paths without completion marker, got %#v", plan)
	}
}

func TestSCN014_ArchivePreparationIgnoresUnapprovedCompletedFeature(t *testing.T) {
	// REQ-012 → REQ-016 → SCN-014 → TestSCN014_ArchivePreparationIgnoresUnapprovedCompletedFeature
	// Scenario: Implemented feature files remain active regression contracts
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle.feature"), "@REQ-012 @REQ-016 @SCN-014\nScenario: Implemented feature files remain active regression contracts\n")
	mustWrite(t, filepath.Join(repo, "specs", ".implementation-complete"), "completed_scenarios:\n  - SCN-014\n")

	plan, err := PrepareCompletedChangeArchive(repo)
	if err != nil {
		t.Fatalf("PrepareCompletedChangeArchive returned error: %v", err)
	}
	if len(plan.KeptActivePaths) != 0 {
		t.Fatalf("expected unapproved completed feature to stay out of archive preparation plan, got %#v", plan)
	}
}

func TestSCN014_ArchivePreparationSurfacesCompletionMarkerReadErrors(t *testing.T) {
	// REQ-012 → REQ-016 → SCN-014 → TestSCN014_ArchivePreparationSurfacesCompletionMarkerReadErrors
	// Scenario: Implemented feature files remain active regression contracts
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "specs", ".implementation-complete"), 0o755); err != nil {
		t.Fatalf("create unreadable completion marker path: %v", err)
	}

	plan, err := PrepareCompletedChangeArchive(repo)
	if err == nil || !strings.Contains(err.Error(), "read implementation completion marker") {
		t.Fatalf("expected completion marker read error, got %v", err)
	}
	if len(plan.KeptActivePaths) != 0 {
		t.Fatalf("expected no archive plan when completion marker cannot be read, got %#v", plan)
	}
}

func TestSCN014_ArchivePreparationSurfacesFeatureParseErrors(t *testing.T) {
	// REQ-012 → REQ-016 → SCN-014 → TestSCN014_ArchivePreparationSurfacesFeatureParseErrors
	// Scenario: Implemented feature files remain active regression contracts
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle.feature"), strings.Repeat("x", 65*1024))
	mustWrite(t, filepath.Join(repo, "specs", "approvals", "workflow_artifact_lifecycle.approved"), "SCN-014\n")
	mustWrite(t, filepath.Join(repo, "specs", ".implementation-complete"), "completed_scenarios:\n  - SCN-014\n")

	_, err := PrepareCompletedChangeArchive(repo)
	if err == nil || !strings.Contains(err.Error(), "find approved feature for completed scenario") {
		t.Fatalf("expected feature parse error from archive preparation, got %v", err)
	}
}

func TestSCN016_AncoraWorkflowStateSerializesPointersWithoutFullContractText(t *testing.T) {
	// REQ-014 → SCN-016 → TestSCN016_AncoraWorkflowStateSerializesPointersWithoutFullContractText
	// Scenario: Ancora records pointer-only workflow state
	fullSpecBody := "# Hard Spec: Workflow Artifact Lifecycle\n\n## Requirements\nFull Markdown body must stay in repo files.\n"
	fullFeatureBody := "Feature: Workflow artifact lifecycle\n  Scenario: Ancora records pointer-only workflow state\n    Given a hard spec exists at \"specs/workflow_artifact_lifecycle.md\"\n"

	payload, err := SerializeAncoraWorkflowState(AncoraWorkflowState{
		SpecPath:       "specs/workflow_artifact_lifecycle.md",
		FeaturePaths:   []string{"features/workflow_artifact_lifecycle.feature"},
		Phase:          "implementation",
		ApprovalStatus: "approved",
		RiskLevel:      "high",
		RequirementIDs: []string{"REQ-014"},
		ScenarioIDs:    []string{"SCN-016"},
		ObservationIDs: []string{"obs-7404"},
		Checksums: map[string]string{
			"specs/workflow_artifact_lifecycle.md": "sha256:spec",
		},
	})
	if err != nil {
		t.Fatalf("SerializeAncoraWorkflowState returned error: %v", err)
	}
	serialized := string(payload)

	for _, want := range []string{
		"specs/workflow_artifact_lifecycle.md",
		"features/workflow_artifact_lifecycle.feature",
		"implementation",
		"approved",
		"high",
		"REQ-014",
		"SCN-016",
		"obs-7404",
		"sha256:spec",
	} {
		if !strings.Contains(serialized, want) {
			t.Fatalf("expected serialized pointer state to contain %q, got %s", want, serialized)
		}
	}
	for _, forbidden := range []string{fullSpecBody, fullFeatureBody, "Full Markdown body must stay in repo files", "Given a hard spec exists"} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("expected serialized pointer state to omit full contract text %q, got %s", forbidden, serialized)
		}
	}
}

func TestSCN017_ChangedRepositoryArtifactReportsStalePointerWithoutOverwritingContent(t *testing.T) {
	// REQ-014 → SCN-017 → TestSCN017_ChangedRepositoryArtifactReportsStalePointerWithoutOverwritingContent
	// Scenario: Repository content wins when an Ancora pointer is stale
	repo := t.TempDir()
	reviewedSpec := "# Reviewed hard spec\n\nRepository-approved content.\n"
	olderAncoraText := "# Older hard spec\n\nStale memory text must not be restored.\n"
	mustWrite(t, filepath.Join(repo, "specs", "workflow_artifact_lifecycle.md"), reviewedSpec)

	report, err := ValidateAncoraWorkflowPointers(repo, AncoraWorkflowState{
		SpecPath:       "specs/workflow_artifact_lifecycle.md",
		FeaturePaths:   []string{"features/workflow_artifact_lifecycle.feature"},
		ScenarioIDs:    []string{"SCN-017"},
		ObservationIDs: []string{"obs-stale"},
		Checksums: map[string]string{
			"specs/workflow_artifact_lifecycle.md": "sha256:old-spec",
		},
	}, map[string]string{
		"specs/workflow_artifact_lifecycle.md": olderAncoraText,
	})
	if err != nil {
		t.Fatalf("ValidateAncoraWorkflowPointers returned error: %v", err)
	}

	if !report.RepositoryContentAuthoritative {
		t.Fatalf("expected repository content to remain authoritative, got %#v", report)
	}
	assertPointerIssue(t, report, "specs/workflow_artifact_lifecycle.md", PointerIssueChecksumMismatch)
	assertPointerIssue(t, report, "features/workflow_artifact_lifecycle.feature", PointerIssueMissing)
	assertFileContent(t, filepath.Join(repo, "specs", "workflow_artifact_lifecycle.md"), reviewedSpec)
}

func TestSCN017_RenamedRepositoryArtifactRepairsPointerMetadataWithoutRestoringMemoryText(t *testing.T) {
	// REQ-014 → SCN-017 → TestSCN017_RenamedRepositoryArtifactRepairsPointerMetadataWithoutRestoringMemoryText
	// Scenario: Repository content wins when an Ancora pointer is stale
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")
	reviewedSpec := "# Reviewed renamed spec\n"
	reviewedFeature := "@REQ-014 @SCN-017\nScenario: Repository content wins when an Ancora pointer is stale\n"
	mustWrite(t, filepath.Join(repo, "specs", "workflow_artifact_lifecycle_reviewed.md"), reviewedSpec)
	mustWrite(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle_reviewed.feature"), reviewedFeature)
	runGit(t, repo, "add", "specs/workflow_artifact_lifecycle_reviewed.md", "features/workflow_artifact_lifecycle_reviewed.feature")
	runGit(t, repo, "commit", "-m", "test: track reviewed renamed artifacts")

	report, err := ValidateAncoraWorkflowPointers(repo, AncoraWorkflowState{
		SpecPath:     "specs/workflow_artifact_lifecycle.md",
		FeaturePaths: []string{"features/workflow_artifact_lifecycle.feature"},
		ScenarioIDs:  []string{"SCN-017"},
	}, map[string]string{
		"specs/workflow_artifact_lifecycle.md":         "# Older memory spec\n",
		"features/workflow_artifact_lifecycle.feature": "Feature: stale memory feature\n",
	})
	if err != nil {
		t.Fatalf("ValidateAncoraWorkflowPointers returned error: %v", err)
	}

	if report.RepairedState.SpecPath != "specs/workflow_artifact_lifecycle_reviewed.md" {
		t.Fatalf("expected repaired spec pointer to reviewed repository path, got %#v", report.RepairedState)
	}
	if len(report.RepairedState.FeaturePaths) != 1 || report.RepairedState.FeaturePaths[0] != "features/workflow_artifact_lifecycle_reviewed.feature" {
		t.Fatalf("expected repaired feature pointer to reviewed repository path, got %#v", report.RepairedState)
	}
	assertPointerIssue(t, report, "specs/workflow_artifact_lifecycle.md", PointerIssueMissing)
	assertPointerIssue(t, report, "features/workflow_artifact_lifecycle.feature", PointerIssueMissing)
	assertFileContent(t, filepath.Join(repo, "specs", "workflow_artifact_lifecycle_reviewed.md"), reviewedSpec)
	assertFileContent(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle_reviewed.feature"), reviewedFeature)
}

func TestSCN019_UntrackedActiveContractsRequireTrackingInsteadOfDeletion(t *testing.T) {
	// REQ-015 → REQ-020 → SCN-019 → TestSCN019_UntrackedActiveContractsRequireTrackingInsteadOfDeletion
	// Scenario: Untracked active contracts are tracked instead of deleted to clean the tree
	repo := t.TempDir()
	runGit(t, repo, "init")
	mustWrite(t, filepath.Join(repo, "specs", "workflow_artifact_lifecycle.md"), "# approved hard spec\n")
	mustWrite(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle.feature"), "@REQ-015 @REQ-020 @SCN-019\nScenario: Untracked active contracts are tracked instead of deleted to clean the tree\n")
	mustWrite(t, filepath.Join(repo, "specs", "approvals", "workflow_artifact_lifecycle.approved"), "SCN-019\n")

	plan, err := PlanCleanTreeContractActions(repo, ContractScope{
		SpecPath:    "specs/workflow_artifact_lifecycle.md",
		FeaturePath: "features/workflow_artifact_lifecycle.feature",
		ScenarioID:  "SCN-019",
	})
	if err != nil {
		t.Fatalf("PlanCleanTreeContractActions returned error: %v", err)
	}

	assertContractAction(t, plan, "specs/workflow_artifact_lifecycle.md", ContractCleanupTrack)
	assertContractAction(t, plan, "features/workflow_artifact_lifecycle.feature", ContractCleanupTrack)
	assertFileContent(t, filepath.Join(repo, "specs", "workflow_artifact_lifecycle.md"), "# approved hard spec\n")
	assertFileContent(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle.feature"), "@REQ-015 @REQ-020 @SCN-019\nScenario: Untracked active contracts are tracked instead of deleted to clean the tree\n")
}

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

func TestSCN019_CleanTreePlanningSurfacesApprovalReadErrors(t *testing.T) {
	// REQ-015 → REQ-020 → SCN-019 → TestSCN019_CleanTreePlanningSurfacesApprovalReadErrors
	// Scenario: Untracked active contracts are tracked instead of deleted to clean the tree
	repo := t.TempDir()
	runGit(t, repo, "init")
	mustWrite(t, filepath.Join(repo, "specs", "workflow_artifact_lifecycle.md"), "# approved hard spec\n")
	mustWrite(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle.feature"), "@REQ-015 @REQ-020 @SCN-019\nScenario: Untracked active contracts are tracked instead of deleted to clean the tree\n")
	if err := os.MkdirAll(filepath.Join(repo, "specs", "approvals", "workflow_artifact_lifecycle.approved"), 0o755); err != nil {
		t.Fatalf("create unreadable approval path: %v", err)
	}

	plan, err := PlanCleanTreeContractActions(repo, ContractScope{
		SpecPath:    "specs/workflow_artifact_lifecycle.md",
		FeaturePath: "features/workflow_artifact_lifecycle.feature",
		ScenarioID:  "SCN-019",
	})

	if err == nil {
		t.Fatalf("expected approval read error, got nil")
	}
	if plan != nil {
		t.Fatalf("expected no cleanup actions when scoped approval cannot be read, got %#v", plan)
	}
	assertFileContent(t, filepath.Join(repo, "specs", "workflow_artifact_lifecycle.md"), "# approved hard spec\n")
	assertFileContent(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle.feature"), "@REQ-015 @REQ-020 @SCN-019\nScenario: Untracked active contracts are tracked instead of deleted to clean the tree\n")
}

func TestSCN019_TrackedActiveContractsRequireNoCleanTreeAction(t *testing.T) {
	// REQ-015 → REQ-020 → SCN-019 → TestSCN019_TrackedActiveContractsRequireNoCleanTreeAction
	// Scenario: Untracked active contracts are tracked instead of deleted to clean the tree
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")
	mustWrite(t, filepath.Join(repo, "specs", "workflow_artifact_lifecycle.md"), "# approved hard spec\n")
	mustWrite(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle.feature"), "@REQ-015 @REQ-020 @SCN-019\nScenario: Untracked active contracts are tracked instead of deleted to clean the tree\n")
	mustWrite(t, filepath.Join(repo, "specs", "approvals", "workflow_artifact_lifecycle.approved"), "SCN-019\n")
	runGit(t, repo, "add", "specs/workflow_artifact_lifecycle.md", "features/workflow_artifact_lifecycle.feature")
	runGit(t, repo, "commit", "-m", "test: track approved contract artifacts")

	plan, err := PlanCleanTreeContractActions(repo, ContractScope{
		SpecPath:    "specs/workflow_artifact_lifecycle.md",
		FeaturePath: "features/workflow_artifact_lifecycle.feature",
		ScenarioID:  "SCN-019",
	})
	if err != nil {
		t.Fatalf("PlanCleanTreeContractActions returned error: %v", err)
	}
	if len(plan) != 0 {
		t.Fatalf("expected no cleanup actions for already tracked active contracts, got %#v", plan)
	}
	assertFileContent(t, filepath.Join(repo, "specs", "workflow_artifact_lifecycle.md"), "# approved hard spec\n")
	assertFileContent(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle.feature"), "@REQ-015 @REQ-020 @SCN-019\nScenario: Untracked active contracts are tracked instead of deleted to clean the tree\n")
}

func TestSCN019_CleanTreePlanningReportsGitMetadataErrors(t *testing.T) {
	// REQ-015 → REQ-020 → SCN-019 → TestSCN019_CleanTreePlanningReportsGitMetadataErrors
	// Scenario: Untracked active contracts are tracked instead of deleted to clean the tree
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "specs", "workflow_artifact_lifecycle.md"), "# approved hard spec\n")
	mustWrite(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle.feature"), "@REQ-015 @REQ-020 @SCN-019\nScenario: Untracked active contracts are tracked instead of deleted to clean the tree\n")
	mustWrite(t, filepath.Join(repo, "specs", "approvals", "workflow_artifact_lifecycle.approved"), "SCN-019\n")

	plan, err := PlanCleanTreeContractActions(repo, ContractScope{
		SpecPath:    "specs/workflow_artifact_lifecycle.md",
		FeaturePath: "features/workflow_artifact_lifecycle.feature",
		ScenarioID:  "SCN-019",
	})

	if err == nil {
		t.Fatalf("expected git metadata error, got nil")
	}
	if !strings.Contains(err.Error(), "check tracked path specs/workflow_artifact_lifecycle.md") {
		t.Fatalf("expected tracked-path error, got %v", err)
	}
	if plan != nil {
		t.Fatalf("expected no cleanup actions when git metadata cannot be read, got %#v", plan)
	}
	assertFileContent(t, filepath.Join(repo, "specs", "workflow_artifact_lifecycle.md"), "# approved hard spec\n")
	assertFileContent(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle.feature"), "@REQ-015 @REQ-020 @SCN-019\nScenario: Untracked active contracts are tracked instead of deleted to clean the tree\n")
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("unexpected content for %s: got %q want %q", path, got, want)
	}
}

func assertContractAction(t *testing.T, plan []ContractCleanupAction, path string, want ContractCleanupActionKind) {
	t.Helper()
	for _, action := range plan {
		if action.Path == path {
			if action.Kind != want {
				t.Fatalf("expected %s action for %s, got %s", want, path, action.Kind)
			}
			return
		}
	}
	t.Fatalf("expected action for %s in %#v", path, plan)
}

func assertArchivePlanKeepsPath(t *testing.T, plan CompletedChangeArchivePlan, path string) {
	t.Helper()
	for _, keptPath := range plan.KeptActivePaths {
		if keptPath == path {
			return
		}
	}
	t.Fatalf("expected archive preparation to keep %s active under features, got %#v", path, plan)
}

func assertPointerIssue(t *testing.T, report WorkflowPointerValidationReport, path string, want PointerIssueKind) {
	t.Helper()
	for _, issue := range report.Issues {
		if issue.Path == path {
			if issue.Kind != want {
				t.Fatalf("expected %s issue for %s, got %s", want, path, issue.Kind)
			}
			return
		}
	}
	t.Fatalf("expected issue for %s in %#v", path, report.Issues)
}
