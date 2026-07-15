package workflow

import (
	"os"
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

func TestSCN012_SourceOfTruthRequiresBothSpecAndFeatureTracked(t *testing.T) {
	// REQ-011 → REQ-012 → SCN-012 → TestSCN012_SourceOfTruthRequiresBothSpecAndFeatureTracked
	// Scenario: Active hard spec and feature files are tracked as the contract source of truth
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")
	mustWrite(t, filepath.Join(repo, "specs", "workflow_artifact_lifecycle.md"), "# approved hard spec\n")
	mustWrite(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle.feature"), "@REQ-011 @REQ-012 @SCN-012\nScenario: Active hard spec and feature files are tracked as the contract source of truth\n")
	runGit(t, repo, "add", "specs/workflow_artifact_lifecycle.md")
	runGit(t, repo, "commit", "-m", "test: track only approved spec")

	status, err := EvaluateContractSourceOfTruth(repo, ContractScope{
		SpecPath:    "specs/workflow_artifact_lifecycle.md",
		FeaturePath: "features/workflow_artifact_lifecycle.feature",
		ScenarioID:  "SCN-012",
	})
	if err != nil {
		t.Fatalf("EvaluateContractSourceOfTruth returned error: %v", err)
	}
	if status.Authoritative {
		t.Fatalf("expected contract not to be authoritative until both spec and feature are tracked, got %#v", status)
	}
	if !status.SpecTracked || status.FeatureTracked {
		t.Fatalf("expected only the spec to be tracked, got %#v", status)
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

func TestSCN013_NamespacedWorkflowPolicyArtifactsRejectEitherLegacyPathCollision(t *testing.T) {
	// REQ-011 → REQ-020 → SCN-013 → TestSCN013_NamespacedWorkflowPolicyArtifactsRejectEitherLegacyPathCollision
	// Scenario: Namespaced workflow-policy artifacts do not overwrite an existing active contract
	tests := []struct {
		name              string
		legacySpecPath    string
		legacyFeaturePath string
	}{
		{name: "spec collision", legacySpecPath: "specs/workflow_artifact_lifecycle.md", legacyFeaturePath: "features/installer_recovery.feature"},
		{name: "feature collision", legacySpecPath: "specs/hard_spec.md", legacyFeaturePath: "features/workflow_artifact_lifecycle.feature"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := t.TempDir()
			_, err := GenerateNamespacedWorkflowPolicyArtifacts(repo, WorkflowPolicyArtifactRequest{
				ContractID:        "workflow_artifact_lifecycle",
				HardSpec:          "# Workflow Artifact Lifecycle\n",
				Feature:           "Feature: Workflow artifact lifecycle\n",
				LegacySpecPath:    tt.legacySpecPath,
				LegacyFeaturePath: tt.legacyFeaturePath,
			})
			if err == nil || !strings.Contains(err.Error(), "overwrite an active contract") {
				t.Fatalf("expected legacy path collision to be rejected, got %v", err)
			}
			assertFileDoesNotExist(t, filepath.Join(repo, "specs", "workflow_artifact_lifecycle.md"))
			assertFileDoesNotExist(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle.feature"))
		})
	}
}

func TestSCN013_NamespacedWorkflowPolicyArtifactsStopAtRequiredWriteFailures(t *testing.T) {
	// REQ-011 → REQ-020 → SCN-013 → TestSCN013_NamespacedWorkflowPolicyArtifactsStopAtRequiredWriteFailures
	// Scenario: Namespaced workflow-policy artifacts do not overwrite an existing active contract
	t.Run("requires a contract id", func(t *testing.T) {
		_, err := GenerateNamespacedWorkflowPolicyArtifacts(t.TempDir(), WorkflowPolicyArtifactRequest{})
		if err == nil || !strings.Contains(err.Error(), "contract id is required") {
			t.Fatalf("expected missing contract id error, got %v", err)
		}
	})

	t.Run("does not continue when the hard spec cannot be written", func(t *testing.T) {
		repo := t.TempDir()
		mustWrite(t, filepath.Join(repo, "specs"), "not a directory\n")
		_, err := GenerateNamespacedWorkflowPolicyArtifacts(repo, WorkflowPolicyArtifactRequest{ContractID: "workflow_artifact_lifecycle", HardSpec: "# hard spec\n", Feature: "Feature: artifact lifecycle\n"})
		if err == nil || !strings.Contains(err.Error(), "create workflow artifact parent") {
			t.Fatalf("expected hard-spec write failure, got %v", err)
		}
		assertFileDoesNotExist(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle.feature"))
	})
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

func TestSCN014_ActiveRegressionContractRequiresApprovedFeaturePath(t *testing.T) {
	// REQ-012 → REQ-016 → SCN-014 → TestSCN014_ActiveRegressionContractRequiresApprovedFeaturePath
	// Scenario: Implemented feature files remain active regression contracts
	tests := []struct {
		name string
		in   WorkflowArtifactLifecycleInput
	}{
		{name: "approved spec is not an active feature contract", in: WorkflowArtifactLifecycleInput{Path: "specs/workflow_artifact_lifecycle.md", Approved: true, Implemented: true}},
		{name: "unapproved feature is not active", in: WorkflowArtifactLifecycleInput{Path: "features/workflow_artifact_lifecycle.feature", Implemented: true}},
		{name: "approved non-feature under features is not active", in: WorkflowArtifactLifecycleInput{Path: "features/workflow_artifact_lifecycle.md", Approved: true, Implemented: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classification := ClassifyWorkflowArtifactLifecycle(tt.in)
			if classification.Kind == WorkflowArtifactActiveRegressionContract {
				t.Fatalf("expected only approved .feature paths under features/ to be active regression contracts, got %#v", classification)
			}
		})
	}
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

func TestSCN020_RetiredSupersededAndProcessOnlyArtifactsClassifyAsArchiveCandidates(t *testing.T) {
	// REQ-016 → SCN-020 → TestSCN020_RetiredSupersededAndProcessOnlyArtifactsClassifyAsArchiveCandidates
	// Scenario: Retired or superseded process artifacts can be archived without hiding active contracts
	tests := []struct {
		name string
		in   WorkflowArtifactLifecycleInput
		want WorkflowArtifactLifecycleKind
	}{
		{
			name: "retired",
			in: WorkflowArtifactLifecycleInput{
				Path:             "specs/old_process.md",
				Retired:          true,
				RetirementReason: "replaced by workflow_artifact_lifecycle",
			},
			want: WorkflowArtifactRetired,
		},
		{
			name: "superseded",
			in: WorkflowArtifactLifecycleInput{
				Path:             "features/old_workflow.feature",
				Superseded:       true,
				RetirementReason: "superseded by workflow_artifact_lifecycle.feature",
			},
			want: WorkflowArtifactSuperseded,
		},
		{
			name: "process only",
			in: WorkflowArtifactLifecycleInput{
				Path:             "docs/process-notes.md",
				ProcessOnly:      true,
				RetirementReason: "temporary implementation notes",
			},
			want: WorkflowArtifactProcessOnly,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertArchiveCandidateClassification(t, tt.name, tt.in, tt.want)
		})
	}
}

func assertArchiveCandidateClassification(t *testing.T, name string, in WorkflowArtifactLifecycleInput, want WorkflowArtifactLifecycleKind) {
	t.Helper()
	classification := ClassifyWorkflowArtifactLifecycle(in)
	if classification.Kind != want {
		t.Fatalf("expected %s classification, got %#v", want, classification)
	}
	if !classification.ArchiveCandidate {
		t.Fatalf("expected archive candidate for %s, got %#v", name, classification)
	}
	if classification.ArchiveReason != in.RetirementReason {
		t.Fatalf("expected archive reason %q, got %#v", in.RetirementReason, classification)
	}
}

func TestSCN020_ArchiveEligibilityRequiresRetirementReason(t *testing.T) {
	// REQ-016 → SCN-020 → TestSCN020_ArchiveEligibilityRequiresRetirementReason
	// Scenario: Retired or superseded process artifacts can be archived without hiding active contracts
	classification := ClassifyWorkflowArtifactLifecycle(WorkflowArtifactLifecycleInput{
		Path:    "specs/old_process.md",
		Retired: true,
	})

	if classification.ArchiveCandidate {
		t.Fatalf("expected retired artifact without a reason to stay out of archive moves, got %#v", classification)
	}
}

func TestSCN020_ArchivePlanMovesOnlyRetiredProcessArtifactsAndKeepsActiveRegressionContracts(t *testing.T) {
	// REQ-016 → SCN-020 → TestSCN020_ArchivePlanMovesOnlyRetiredProcessArtifactsAndKeepsActiveRegressionContracts
	// Scenario: Retired or superseded process artifacts can be archived without hiding active contracts
	repo := t.TempDir()
	activeFeature := "Feature: active regression contract\n"
	retiredSpec := "# Retired process note\n"
	processNote := "# Process-only note\n"
	mustWrite(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle.feature"), activeFeature)
	mustWrite(t, filepath.Join(repo, "specs", "old_process.md"), retiredSpec)
	mustWrite(t, filepath.Join(repo, "docs", "handoff_notes.md"), processNote)

	plan := PlanWorkflowArtifactArchive([]WorkflowArtifactLifecycleInput{
		{
			Path:        "features/workflow_artifact_lifecycle.feature",
			Implemented: true,
			Approved:    true,
		},
		{
			Path:             "specs/old_process.md",
			Retired:          true,
			RetirementReason: "replaced by workflow_artifact_lifecycle.md",
		},
		{
			Path:             "docs/handoff_notes.md",
			ProcessOnly:      true,
			RetirementReason: "temporary implementation handoff",
		},
	})

	assertArchivePlanKeepsPath(t, plan, "features/workflow_artifact_lifecycle.feature")
	assertArchiveMove(t, plan, "specs/old_process.md", "archive/specs/old_process.md", "replaced by workflow_artifact_lifecycle.md")
	assertArchiveMove(t, plan, "docs/handoff_notes.md", "archive/docs/handoff_notes.md", "temporary implementation handoff")
	assertArchivePlanDoesNotMovePath(t, plan, "features/workflow_artifact_lifecycle.feature")
	assertFileContent(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle.feature"), activeFeature)
	assertFileContent(t, filepath.Join(repo, "specs", "old_process.md"), retiredSpec)
	assertFileContent(t, filepath.Join(repo, "docs", "handoff_notes.md"), processNote)
}

func TestSCN020_ArchivePlanDoesNotKeepApprovedSpecAsActiveFeatureContract(t *testing.T) {
	// REQ-016 → SCN-020 → TestSCN020_ArchivePlanDoesNotKeepApprovedSpecAsActiveFeatureContract
	// Scenario: Retired or superseded process artifacts can be archived without hiding active contracts
	plan := PlanWorkflowArtifactArchive([]WorkflowArtifactLifecycleInput{
		{Path: "specs/workflow_artifact_lifecycle.md", Approved: true, Implemented: true},
		{Path: "features/workflow_artifact_lifecycle.feature", Approved: true, Implemented: true},
	})

	assertArchivePlanKeepsPath(t, plan, "features/workflow_artifact_lifecycle.feature")
	assertArchivePlanDoesNotKeepPath(t, plan, "specs/workflow_artifact_lifecycle.md")
}

func TestSCN021_LocalGraphAndCacheArtifactsClassifyOutsideReviewSet(t *testing.T) {
	// REQ-017 → SCN-021 → TestSCN021_LocalGraphAndCacheArtifactsClassifyOutsideReviewSet
	// Scenario: Local graph and cache artifacts are excluded unless intentionally promoted
	tests := []struct {
		name string
		path string
	}{
		{name: "vela graph", path: ".vela/graph.db"},
		{name: "nested vela cache", path: "subproject/.vela/cache.json"},
		{name: "tool cache", path: ".cache/rotta/planner.json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classification := ClassifyWorkflowArtifactLifecycle(WorkflowArtifactLifecycleInput{Path: tt.path})

			if classification.Kind != WorkflowArtifactLocalGeneratedCache {
				t.Fatalf("expected local generated cache classification, got %#v", classification)
			}
			if classification.ReviewCandidate {
				t.Fatalf("expected local generated cache to stay out of review set, got %#v", classification)
			}
			if classification.RequiresProjectArtifactDecision {
				t.Fatalf("expected unpromoted local generated cache not to require a decision just to exclude, got %#v", classification)
			}
		})
	}

	classification := ClassifyWorkflowArtifactLifecycle(WorkflowArtifactLifecycleInput{
		Path:                                  ".vela/promoted-review-snapshot.json",
		IntentionallyTrackedGeneratedArtifact: true,
	})
	if classification.ReviewCandidate {
		t.Fatalf("expected intentionally tracked generated artifact without decision to stay out of review set, got %#v", classification)
	}
	if !classification.RequiresProjectArtifactDecision {
		t.Fatalf("expected intentionally tracked generated artifact to require explicit project-artifact decision, got %#v", classification)
	}
}

func TestSCN021_ReviewSetPreparationExcludesVelaCacheAndKeepsContracts(t *testing.T) {
	// REQ-017 → SCN-021 → TestSCN021_ReviewSetPreparationExcludesVelaCacheAndKeepsContracts
	// Scenario: Local graph and cache artifacts are excluded unless intentionally promoted
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "specs", "workflow_artifact_lifecycle.md"), "# Workflow Artifact Lifecycle\n")
	mustWrite(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle.feature"), "@REQ-017 @SCN-021\nScenario: Local graph and cache artifacts are excluded unless intentionally promoted\n")
	mustWrite(t, filepath.Join(repo, ".vela", "graph.db"), "generated graph\n")
	mustWrite(t, filepath.Join(repo, ".cache", "rotta", "planner.json"), "{}\n")

	plan := PrepareWorkflowArtifactReviewSet([]WorkflowArtifactLifecycleInput{
		{Path: "specs/workflow_artifact_lifecycle.md"},
		{Path: "features/workflow_artifact_lifecycle.feature", Approved: true, Implemented: true},
		{Path: ".vela/graph.db"},
		{Path: ".cache/rotta/planner.json"},
	})

	assertReviewSetIncludesPath(t, plan, "specs/workflow_artifact_lifecycle.md")
	assertReviewSetIncludesPath(t, plan, "features/workflow_artifact_lifecycle.feature")
	assertReviewSetExcludesPath(t, plan, ".vela/graph.db")
	assertReviewSetExcludesPath(t, plan, ".cache/rotta/planner.json")
	assertFileContent(t, filepath.Join(repo, ".vela", "graph.db"), "generated graph\n")
}
