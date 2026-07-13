package workflow

import (
	"path/filepath"
	"strings"
	"testing"
)

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
