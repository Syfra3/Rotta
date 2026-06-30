package workflow

import (
	"crypto/sha256"
	"fmt"
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

func TestSCN014_ArchivePreparationIgnoresNonFeatureFilesUnderFeatures(t *testing.T) {
	// REQ-012 → REQ-016 → SCN-014 → TestSCN014_ArchivePreparationIgnoresNonFeatureFilesUnderFeatures
	// Scenario: Implemented feature files remain active regression contracts
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle.feature"), "@REQ-012 @REQ-016 @SCN-014\nScenario: Implemented feature files remain active regression contracts\n")
	mustWrite(t, filepath.Join(repo, "features", "notes.txt"), strings.Repeat("x", 65*1024))
	mustWrite(t, filepath.Join(repo, "specs", "approvals", "workflow_artifact_lifecycle.approved"), "SCN-014\n")
	mustWrite(t, filepath.Join(repo, "specs", ".implementation-complete"), "completed_scenarios:\n  - SCN-014\n")

	plan, err := PrepareCompletedChangeArchive(repo)
	if err != nil {
		t.Fatalf("PrepareCompletedChangeArchive should ignore non-feature files, got %v", err)
	}
	assertArchivePlanKeepsPath(t, plan, "features/workflow_artifact_lifecycle.feature")
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
			classification := ClassifyWorkflowArtifactLifecycle(tt.in)

			if classification.Kind != tt.want {
				t.Fatalf("expected %s classification, got %#v", tt.want, classification)
			}
			if !classification.ArchiveCandidate {
				t.Fatalf("expected archive candidate for %s, got %#v", tt.name, classification)
			}
			if classification.ArchiveReason != tt.in.RetirementReason {
				t.Fatalf("expected archive reason %q, got %#v", tt.in.RetirementReason, classification)
			}
		})
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

func TestSCN022_SensitiveBackupAndMachineStateArtifactsAreRejected(t *testing.T) {
	// REQ-018 → SCN-022 → TestSCN022_SensitiveBackupAndMachineStateArtifactsAreRejected
	// Scenario: Backup outputs and sensitive config captures are rejected as workflow artifacts
	tests := []struct {
		name    string
		path    string
		content string
	}{
		{name: "backup output", path: ".rotta/backups/20260630/manifest.json", content: `{"target":"opencode"}`},
		{name: "redacted example under backup output", path: ".rotta/backups/example/redacted-opencode.json", content: `{"api_key":"<redacted>"}`},
		{name: "restore snapshot", path: ".rotta/restore/pre-restore-snapshot.json", content: `{"snapshot":"pre-restore"}`},
		{name: "user config capture", path: "captures/home/geen/.config/opencode/opencode.json", content: `{"mcp":{"auth":"fake"}}`},
		{name: "redacted example under user config capture", path: "captures/example/opencode.json", content: `{"token":"<redacted>"}`},
		{name: "token-bearing file path", path: "fixtures/token.env", content: "API_TOKEN=fake-token-for-test\n"},
		{name: "redacted token-bearing example path", path: "docs/examples/api-token.redacted.env", content: "API_TOKEN=<redacted>\n"},
		{name: "private machine state", path: "machine-state/ssh/config", content: "Host synthetic-test\n  IdentityFile ~/.ssh/id_ed25519\n"},
		{name: "redacted example under private machine state", path: "machine-state/example/ssh-config", content: "IdentityFile <redacted>\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			classification := ClassifyWorkflowArtifactLifecycle(WorkflowArtifactLifecycleInput{Path: tt.path, Content: tt.content})

			if classification.Kind != WorkflowArtifactRejectedSensitive {
				t.Fatalf("expected sensitive rejection classification, got %#v", classification)
			}
			if classification.ReviewCandidate {
				t.Fatalf("expected sensitive artifact to stay out of review set, got %#v", classification)
			}
			if !classification.RequiresSanitizedReplacement {
				t.Fatalf("expected sensitive artifact to require delete, ignore, or sanitized authored replacement, got %#v", classification)
			}
		})
	}
}

func TestSCN022_ExamplePathAloneDoesNotSanitizeSecretBearingContent(t *testing.T) {
	// REQ-018 → SCN-022 → TestSCN022_ExamplePathAloneDoesNotSanitizeSecretBearingContent
	// Scenario: Backup outputs and sensitive config captures are rejected as workflow artifacts
	classification := ClassifyWorkflowArtifactLifecycle(WorkflowArtifactLifecycleInput{
		Path:    "docs/examples/opencode-auth.json",
		Content: `{"token":"fake-token-for-test"}`,
	})

	if classification.Kind != WorkflowArtifactRejectedSensitive {
		t.Fatalf("expected example content with unredacted token to be rejected, got %#v", classification)
	}
	if classification.ReviewCandidate {
		t.Fatalf("expected unredacted example secret to stay out of review set, got %#v", classification)
	}
}

func TestSCN022_ReviewSetRejectsSensitiveFixturesAndKeepsSanitizedExamples(t *testing.T) {
	// REQ-018 → SCN-022 → TestSCN022_ReviewSetRejectsSensitiveFixturesAndKeepsSanitizedExamples
	// Scenario: Backup outputs and sensitive config captures are rejected as workflow artifacts
	plan := PrepareWorkflowArtifactReviewSet([]WorkflowArtifactLifecycleInput{
		{Path: "features/workflow_artifact_lifecycle.feature", Approved: true, Implemented: true, Content: "Feature: Workflow artifact lifecycle\n"},
		{Path: "docs/examples/sanitized_opencode.example.json", Content: `{"token":"<redacted-example>"}`},
		{Path: "docs/sanitized_capture.md", Content: "Authorization: <redacted>\n"},
		{Path: "specs/workflow_artifact_lifecycle.md", Content: "captured token: fake-token-for-test\n"},
		{Path: "backups/opencode.json", Content: `{"api_key":"fake-secret-for-test"}`},
		{Path: "backups/example/redacted-opencode.json", Content: `{"api_key":"<redacted>"}`},
		{Path: "docs/examples/api-token.redacted.env", Content: "API_TOKEN=<redacted>\n"},
	})

	assertReviewSetIncludesPath(t, plan, "features/workflow_artifact_lifecycle.feature")
	assertReviewSetIncludesPath(t, plan, "docs/examples/sanitized_opencode.example.json")
	assertReviewSetIncludesPath(t, plan, "docs/sanitized_capture.md")
	assertReviewSetExcludesPath(t, plan, "specs/workflow_artifact_lifecycle.md")
	assertReviewSetExcludesPath(t, plan, "backups/opencode.json")
	assertReviewSetExcludesPath(t, plan, "backups/example/redacted-opencode.json")
	assertReviewSetExcludesPath(t, plan, "docs/examples/api-token.redacted.env")
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

func TestSCN017_CurrentRepositoryPointersNeedNoGitRepair(t *testing.T) {
	// REQ-014 → SCN-017 → TestSCN017_CurrentRepositoryPointersNeedNoGitRepair
	// Scenario: Repository content wins when an Ancora pointer is stale
	repo := t.TempDir()
	specBody := "# Reviewed hard spec\n"
	featureBody := "@REQ-014 @SCN-017\nScenario: Repository content wins when an Ancora pointer is stale\n"
	mustWrite(t, filepath.Join(repo, "specs", "workflow_artifact_lifecycle.md"), specBody)
	mustWrite(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle.feature"), featureBody)

	report, err := ValidateAncoraWorkflowPointers(repo, AncoraWorkflowState{
		SpecPath:     "specs/workflow_artifact_lifecycle.md",
		FeaturePaths: []string{"features/workflow_artifact_lifecycle.feature"},
		ScenarioIDs:  []string{"SCN-017"},
		Checksums: map[string]string{
			"specs/workflow_artifact_lifecycle.md":         checksumFor(specBody),
			"features/workflow_artifact_lifecycle.feature": checksumFor(featureBody),
		},
	}, nil)
	if err != nil {
		t.Fatalf("expected current pointers to validate without git repair, got %v", err)
	}
	if len(report.Issues) != 0 {
		t.Fatalf("expected no stale pointer issues, got %#v", report.Issues)
	}
	if report.RepairedState.SpecPath != "specs/workflow_artifact_lifecycle.md" || report.RepairedState.FeaturePaths[0] != "features/workflow_artifact_lifecycle.feature" {
		t.Fatalf("expected current pointer metadata to remain unchanged, got %#v", report.RepairedState)
	}
}

func TestSCN017_CurrentRepositoryPointersAreNotRepairedToOtherTrackedCandidates(t *testing.T) {
	// REQ-014 → SCN-017 → TestSCN017_CurrentRepositoryPointersAreNotRepairedToOtherTrackedCandidates
	// Scenario: Repository content wins when an Ancora pointer is stale
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")
	specBody := "# Current reviewed spec\n\nSCN-017\n"
	featureBody := "@REQ-014 @SCN-017\nScenario: Repository content wins when an Ancora pointer is stale\n"
	mustWrite(t, filepath.Join(repo, "specs", "workflow_artifact_lifecycle.md"), specBody)
	mustWrite(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle.feature"), featureBody)
	mustWrite(t, filepath.Join(repo, "specs", "workflow_artifact_lifecycle_alternative.md"), "# Alternative tracked spec\n\nSCN-017\n")
	mustWrite(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle_alternative.feature"), "@REQ-014 @SCN-017\nScenario: Alternative stale candidate\n")
	runGit(t, repo, "add", "specs/workflow_artifact_lifecycle.md", "features/workflow_artifact_lifecycle.feature", "specs/workflow_artifact_lifecycle_alternative.md", "features/workflow_artifact_lifecycle_alternative.feature")
	runGit(t, repo, "commit", "-m", "test: track current and alternative contract artifacts")

	report, err := ValidateAncoraWorkflowPointers(repo, AncoraWorkflowState{
		SpecPath:     "specs/workflow_artifact_lifecycle.md",
		FeaturePaths: []string{"features/workflow_artifact_lifecycle.feature"},
		ScenarioIDs:  []string{"SCN-017"},
		Checksums: map[string]string{
			"specs/workflow_artifact_lifecycle.md":         checksumFor(specBody),
			"features/workflow_artifact_lifecycle.feature": checksumFor(featureBody),
		},
	}, nil)
	if err != nil {
		t.Fatalf("ValidateAncoraWorkflowPointers returned error: %v", err)
	}
	if len(report.Issues) != 0 {
		t.Fatalf("expected current pointers to avoid stale repair, got %#v", report.Issues)
	}
	if report.RepairedState.SpecPath != "specs/workflow_artifact_lifecycle.md" || report.RepairedState.FeaturePaths[0] != "features/workflow_artifact_lifecycle.feature" {
		t.Fatalf("expected current pointer metadata to remain authoritative, got %#v", report.RepairedState)
	}
}

func TestSCN017_EmptyChecksumDoesNotReportDriftForExistingRepositoryPointer(t *testing.T) {
	// REQ-014 → SCN-017 → TestSCN017_EmptyChecksumDoesNotReportDriftForExistingRepositoryPointer
	// Scenario: Repository content wins when an Ancora pointer is stale
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "specs", "workflow_artifact_lifecycle.md"), "# Reviewed hard spec without checksum\n")

	report, err := ValidateAncoraWorkflowPointers(repo, AncoraWorkflowState{
		SpecPath:    "specs/workflow_artifact_lifecycle.md",
		ScenarioIDs: []string{"SCN-017"},
		Checksums:   map[string]string{},
	}, nil)
	if err != nil {
		t.Fatalf("ValidateAncoraWorkflowPointers returned error: %v", err)
	}
	if len(report.Issues) != 0 {
		t.Fatalf("expected existing pointer without checksum to avoid false drift, got %#v", report.Issues)
	}
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

func TestSCN017_RenamedPointerRepairUsesScenarioBearingTrackedArtifactWhenLifecycleCandidatesAreAmbiguous(t *testing.T) {
	// REQ-014 → SCN-017 → TestSCN017_RenamedPointerRepairUsesScenarioBearingTrackedArtifactWhenLifecycleCandidatesAreAmbiguous
	// Scenario: Repository content wins when an Ancora pointer is stale
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "test@example.invalid")
	runGit(t, repo, "config", "user.name", "Test User")
	mustWrite(t, filepath.Join(repo, "specs", "workflow_artifact_lifecycle_reviewed.md"), "# Reviewed spec\n\nSCN-017\n")
	mustWrite(t, filepath.Join(repo, "specs", "workflow_artifact_lifecycle_notes.md"), "# Lifecycle notes without matching scenario\n")
	mustWrite(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle_reviewed.feature"), "@REQ-014 @SCN-017\nScenario: Repository content wins when an Ancora pointer is stale\n")
	mustWrite(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle_notes.feature"), "Feature: Lifecycle notes without matching scenario\n")
	runGit(t, repo, "add", "specs/workflow_artifact_lifecycle_reviewed.md", "specs/workflow_artifact_lifecycle_notes.md", "features/workflow_artifact_lifecycle_reviewed.feature", "features/workflow_artifact_lifecycle_notes.feature")
	runGit(t, repo, "commit", "-m", "test: track ambiguous lifecycle artifacts")

	report, err := ValidateAncoraWorkflowPointers(repo, AncoraWorkflowState{
		SpecPath:     "specs/workflow_artifact_lifecycle.md",
		FeaturePaths: []string{"features/workflow_artifact_lifecycle.feature"},
		ScenarioIDs:  []string{"SCN-017"},
	}, nil)
	if err != nil {
		t.Fatalf("ValidateAncoraWorkflowPointers returned error: %v", err)
	}
	if report.RepairedState.SpecPath != "specs/workflow_artifact_lifecycle_reviewed.md" {
		t.Fatalf("expected repair to select scenario-bearing spec path, got %#v", report.RepairedState)
	}
	if len(report.RepairedState.FeaturePaths) != 1 || report.RepairedState.FeaturePaths[0] != "features/workflow_artifact_lifecycle_reviewed.feature" {
		t.Fatalf("expected repair to select scenario-bearing feature path, got %#v", report.RepairedState)
	}
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

func assertArchiveMove(t *testing.T, plan CompletedChangeArchivePlan, sourcePath, destinationPath, reason string) {
	t.Helper()
	for _, move := range plan.ArchiveMoves {
		if move.SourcePath == sourcePath {
			if move.DestinationPath != destinationPath || move.Reason != reason {
				t.Fatalf("unexpected archive move for %s: got %#v", sourcePath, move)
			}
			return
		}
	}
	t.Fatalf("expected archive move for %s in %#v", sourcePath, plan)
}

func assertArchivePlanDoesNotMovePath(t *testing.T, plan CompletedChangeArchivePlan, path string) {
	t.Helper()
	for _, move := range plan.ArchiveMoves {
		if move.SourcePath == path {
			t.Fatalf("expected %s to stay out of archive moves, got %#v", path, plan)
		}
	}
}

func assertArchivePlanDoesNotKeepPath(t *testing.T, plan CompletedChangeArchivePlan, path string) {
	t.Helper()
	for _, keptPath := range plan.KeptActivePaths {
		if keptPath == path {
			t.Fatalf("expected %s not to be kept as an active feature contract, got %#v", path, plan)
		}
	}
}

func assertFileDoesNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("expected %s not to exist", path)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", path, err)
	}
}

func assertReviewSetIncludesPath(t *testing.T, plan WorkflowArtifactReviewSetPlan, path string) {
	t.Helper()
	for _, includedPath := range plan.IncludedPaths {
		if includedPath == path {
			return
		}
	}
	t.Fatalf("expected review set to include %s, got %#v", path, plan)
}

func assertReviewSetExcludesPath(t *testing.T, plan WorkflowArtifactReviewSetPlan, path string) {
	t.Helper()
	for _, excludedPath := range plan.ExcludedPaths {
		if excludedPath == path {
			return
		}
	}
	t.Fatalf("expected review set to exclude %s, got %#v", path, plan)
}

func assertCleanupGuidanceAction(t *testing.T, report WorkflowArtifactCleanupGuidanceReport, path string, want WorkflowArtifactCleanupActionKind) {
	t.Helper()
	for _, item := range report.Items {
		if item.Path == path {
			if item.Action != want {
				t.Fatalf("expected cleanup action %q for %s, got %#v", want, path, item)
			}
			if item.Reason == "" {
				t.Fatalf("expected cleanup guidance reason for %s, got %#v", path, item)
			}
			return
		}
	}
	t.Fatalf("expected cleanup guidance for %s in %#v", path, report)
}

func assertCleanupGuidanceDoesNotUseAction(t *testing.T, report WorkflowArtifactCleanupGuidanceReport, path string, forbidden WorkflowArtifactCleanupActionKind) {
	t.Helper()
	for _, item := range report.Items {
		if item.Path == path && item.Action == forbidden {
			t.Fatalf("expected cleanup guidance for %s not to use %q, got %#v", path, forbidden, item)
		}
	}
}

func assertCleanupGuidanceReason(t *testing.T, report WorkflowArtifactCleanupGuidanceReport, path, want string) {
	t.Helper()
	for _, item := range report.Items {
		if item.Path == path {
			if item.Reason != want {
				t.Fatalf("expected cleanup reason %q for %s, got %#v", want, path, item)
			}
			return
		}
	}
	t.Fatalf("expected cleanup guidance reason for %s in %#v", path, report)
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

func checksumFor(content string) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(content)))
}
