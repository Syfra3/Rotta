package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSCN015_FeatureScenarioParserKeepsStableIDsWhenScenarioOrderChanges(t *testing.T) {
	// REQ-013 → REQ-019 → SCN-015 → TestSCN015_FeatureScenarioParserKeepsStableIDsWhenScenarioOrderChanges
	// Scenario: Tests reference stable scenario IDs from feature files
	featurePath := "features/workflow_artifact_lifecycle.feature"
	originalFeature := `Feature: Workflow artifact lifecycle

  @REQ-011 @SCN-012
  Scenario: Active hard spec and feature files are tracked as the contract source of truth
    Then the hard spec remains tracked

  @REQ-013 @REQ-019 @SCN-015
  Scenario: Tests reference stable scenario IDs from feature files
    Then the test references the scenario ID
`
	reorderedFeature := `Feature: Workflow artifact lifecycle

  @REQ-013 @REQ-019 @SCN-015
  Scenario: Tests reference stable scenario IDs from feature files
    Then the test references the scenario ID

  @REQ-011 @SCN-012
  Scenario: Active hard spec and feature files are tracked as the contract source of truth
    Then the hard spec remains tracked
`

	original, err := ParseFeatureScenarioTags(featurePath, strings.NewReader(originalFeature))
	if err != nil {
		t.Fatalf("ParseFeatureScenarioTags returned error: %v", err)
	}
	reordered, err := ParseFeatureScenarioTags(featurePath, strings.NewReader(reorderedFeature))
	if err != nil {
		t.Fatalf("ParseFeatureScenarioTags returned error: %v", err)
	}

	originalScenario := scenarioByName(t, original, "Tests reference stable scenario IDs from feature files")
	reorderedScenario := scenarioByName(t, reordered, "Tests reference stable scenario IDs from feature files")
	if originalScenario.ScenarioID != "SCN-015" || reorderedScenario.ScenarioID != "SCN-015" {
		t.Fatalf("expected stable SCN-015 before and after reordering, got %#v and %#v", originalScenario, reorderedScenario)
	}
	if originalScenario.FeaturePath != featurePath {
		t.Fatalf("expected feature identity %q, got %#v", featurePath, originalScenario)
	}
	assertStringSliceContains(t, originalScenario.RequirementIDs, "REQ-013")
	assertStringSliceContains(t, originalScenario.RequirementIDs, "REQ-019")
}

func TestSCN015_TestTraceValidatorRequiresScenarioIDAndFeatureIdentity(t *testing.T) {
	// REQ-013 → REQ-019 → SCN-015 → TestSCN015_TestTraceValidatorRequiresScenarioIDAndFeatureIdentity
	// Scenario: Tests reference stable scenario IDs from feature files
	scenarios := []FeatureScenario{
		{FeaturePath: "features/workflow_artifact_lifecycle.feature", ScenarioID: "SCN-015", RequirementIDs: []string{"REQ-013", "REQ-019"}, Name: "Tests reference stable scenario IDs from feature files"},
		{FeaturePath: "features/other_contract.feature", ScenarioID: "SCN-015", RequirementIDs: []string{"REQ-999"}, Name: "Another local SCN-015"},
	}

	missingScenarioID := ValidateTestScenarioTrace(scenarios, TestScenarioTrace{
		FeaturePath: "features/workflow_artifact_lifecycle.feature",
		TestName:    "TestTraceValidatorRejectsUnstableNames",
	})
	if missingScenarioID == nil || !strings.Contains(missingScenarioID.Error(), "SCN-015") {
		t.Fatalf("expected missing scenario ID error, got %v", missingScenarioID)
	}

	missingFeatureIdentity := ValidateTestScenarioTrace(scenarios, TestScenarioTrace{
		ScenarioID: "SCN-015",
		TestName:   "TestSCN015_TraceIncludesStableScenarioIDOnly",
	})
	if missingFeatureIdentity == nil || !strings.Contains(missingFeatureIdentity.Error(), "feature identity") {
		t.Fatalf("expected missing feature identity error, got %v", missingFeatureIdentity)
	}

	if err := ValidateTestScenarioTrace(scenarios, TestScenarioTrace{
		FeaturePath: "features/workflow_artifact_lifecycle.feature",
		ScenarioID:  "SCN-015",
		TestName:    "TestSCN015_TraceIncludesFeatureIdentityInMetadata",
		Metadata: map[string]string{
			"scenario": "features/workflow_artifact_lifecycle.feature#SCN-015",
		},
	}); err != nil {
		t.Fatalf("expected metadata trace to validate: %v", err)
	}

	if err := ValidateTestScenarioTrace(scenarios, TestScenarioTrace{
		FeaturePath: "features/workflow_artifact_lifecycle.feature",
		ScenarioID:  "SCN-015",
		SubtestNames: []string{
			"features/workflow_artifact_lifecycle.feature#SCN-015 Tests reference stable scenario IDs from feature files",
		},
	}); err != nil {
		t.Fatalf("expected feature-qualified subtest trace to validate: %v", err)
	}
}

func TestSCN023_QAPlanningEnumeratesApprovedRepositoryScenarios(t *testing.T) {
	// REQ-019 → SCN-023 → TestSCN023_QAPlanningEnumeratesApprovedRepositoryScenarios
	// Scenario: QA planning enumerates approved scenarios from repository feature files
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle.feature"), `Feature: Workflow artifact lifecycle

  @REQ-019 @SCN-023
  Scenario: QA planning enumerates approved scenarios from repository feature files
    Then each planned test can reference the feature file and scenario ID

  @REQ-020 @SCN-024
  Scenario: Workflow cleanup explains artifact lifecycle actions explicitly
    Then pending contracts remain pending until a human approves them
`)
	mustWrite(t, filepath.Join(repo, "features", "pending_contract.feature"), `Feature: Pending contract

  @REQ-999 @SCN-999
  Scenario: Pending generated behavior is not ready
    Then implementation does not begin
`)
	mustWrite(t, filepath.Join(repo, "specs", "approvals", "workflow_artifact_lifecycle.approved"), "features/workflow_artifact_lifecycle.feature#SCN-023\n")

	items, err := PlanImplementationReadyScenarios(repo)
	if err != nil {
		t.Fatalf("PlanImplementationReadyScenarios returned error: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected only one implementation-ready scenario, got %#v", items)
	}
	item := items[0]
	if item.FeaturePath != "features/workflow_artifact_lifecycle.feature" || item.ScenarioID != "SCN-023" {
		t.Fatalf("expected planned item to reference feature path and SCN-023, got %#v", item)
	}
	if item.Name != "QA planning enumerates approved scenarios from repository feature files" {
		t.Fatalf("expected scenario name from repository feature, got %#v", item)
	}
}

func TestSCN023_QAPlanningRequiresScopedApprovalAndScenarioIdentity(t *testing.T) {
	// REQ-019 → SCN-023 → TestSCN023_QAPlanningRequiresScopedApprovalAndScenarioIdentity
	// Scenario: QA planning enumerates approved scenarios from repository feature files
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle.feature"), `Feature: Workflow artifact lifecycle

  @REQ-019 @SCN-023
  Scenario: QA planning enumerates approved scenarios from repository feature files
    Then each planned test can reference the feature file and scenario ID

  @REQ-019
  Scenario: Missing stable scenario identity is not ready for implementation
    Then implementation does not begin without a scenario ID
`)
	mustWrite(t, filepath.Join(repo, "features", "archive", "workflow_artifact_lifecycle.feature"), `Feature: Archived duplicate lifecycle contract

  @REQ-019 @SCN-023
  Scenario: Archived duplicate is not approved by another feature identity
    Then feature-qualified approval remains scoped to one feature file
`)
	mustWrite(t, filepath.Join(repo, "specs", "approvals", "workflow_artifact_lifecycle.approved"), "features/workflow_artifact_lifecycle.feature#SCN-023\n")

	items, err := PlanImplementationReadyScenarios(repo)
	if err != nil {
		t.Fatalf("PlanImplementationReadyScenarios returned error: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected only the feature-qualified approved scenario to be ready, got %#v", items)
	}
	item := items[0]
	if item.FeaturePath != "features/workflow_artifact_lifecycle.feature" || item.ScenarioID != "SCN-023" {
		t.Fatalf("expected scoped feature identity and SCN-023, got %#v", item)
	}
	if item.Name == "Missing stable scenario identity is not ready for implementation" {
		t.Fatalf("scenario without SCN identity must not become implementation-ready: %#v", item)
	}
}

func TestSCN023_QAPlanningReturnsParseErrors(t *testing.T) {
	// REQ-019 → SCN-023 → TestSCN023_QAPlanningReturnsParseErrors
	// Scenario: QA planning enumerates approved scenarios from repository feature files
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle.feature"), strings.Repeat("x", 65*1024))

	_, err := PlanImplementationReadyScenarios(repo)
	if err == nil || !strings.Contains(err.Error(), "parse feature scenarios features/workflow_artifact_lifecycle.feature") {
		t.Fatalf("expected feature parse error to identify repository feature, got %v", err)
	}
}

func TestSCN023_QAPlanningTreatsMissingFeatureDirectoryAsNoReadyScenarios(t *testing.T) {
	// REQ-019 → SCN-023 → TestSCN023_QAPlanningTreatsMissingFeatureDirectoryAsNoReadyScenarios
	// Scenario: QA planning enumerates approved scenarios from repository feature files
	items, err := PlanImplementationReadyScenarios(t.TempDir())
	if err != nil {
		t.Fatalf("PlanImplementationReadyScenarios returned error for repository without features: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected repository without feature files to have no implementation-ready scenarios, got %#v", items)
	}
}

func TestSCN023_QAPlanningReturnsScopedApprovalReadErrors(t *testing.T) {
	// REQ-019 → SCN-023 → TestSCN023_QAPlanningReturnsScopedApprovalReadErrors
	// Scenario: QA planning enumerates approved scenarios from repository feature files
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "features", "workflow_artifact_lifecycle.feature"), `Feature: Workflow artifact lifecycle

  @REQ-019 @SCN-023
  Scenario: QA planning enumerates approved scenarios from repository feature files
    Then each planned test can reference the feature file and scenario ID
`)
	approvalPath := filepath.Join(repo, "specs", "approvals", "workflow_artifact_lifecycle.approved")
	if err := os.MkdirAll(approvalPath, 0o755); err != nil {
		t.Fatalf("create unreadable approval path: %v", err)
	}

	_, err := PlanImplementationReadyScenarios(repo)
	if err == nil || !strings.Contains(err.Error(), "plan implementation-ready scenarios") {
		t.Fatalf("expected scoped approval read error during planning, got %v", err)
	}
}

func scenarioByName(t *testing.T, scenarios []FeatureScenario, name string) FeatureScenario {
	t.Helper()
	for _, scenario := range scenarios {
		if scenario.Name == name {
			return scenario
		}
	}
	t.Fatalf("expected scenario %q in %#v", name, scenarios)
	return FeatureScenario{}
}

func assertStringSliceContains(t *testing.T, got []string, want string) {
	t.Helper()
	for _, value := range got {
		if value == want {
			return
		}
	}
	t.Fatalf("expected %#v to contain %q", got, want)
}
