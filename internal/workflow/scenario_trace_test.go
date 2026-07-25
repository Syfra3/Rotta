package workflow

import (
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
	scenarios := traceValidationScenarios()
	assertTraceValidationInputErrors(t, scenarios)
	assertTraceValidationAcceptsQualifiedTraces(t, scenarios)
	assertTraceValidationRejectsUnknownFeature(t, scenarios)
}

func traceValidationScenarios() []FeatureScenario {
	return []FeatureScenario{{FeaturePath: "features/workflow_artifact_lifecycle.feature", ScenarioID: "SCN-015", RequirementIDs: []string{"REQ-013", "REQ-019"}, Name: "Tests reference stable scenario IDs from feature files"}, {FeaturePath: "features/other_contract.feature", ScenarioID: "SCN-015", RequirementIDs: []string{"REQ-999"}, Name: "Another local SCN-015"}}
}

func assertTraceValidationInputErrors(t *testing.T, scenarios []FeatureScenario) {
	t.Helper()
	missingScenarioID := ValidateTestScenarioTrace(scenarios, TestScenarioTrace{FeaturePath: "features/workflow_artifact_lifecycle.feature", TestName: "TestTraceValidatorRejectsUnstableNames"})
	if missingScenarioID == nil || !strings.Contains(missingScenarioID.Error(), "SCN-015") {
		t.Fatalf("expected missing scenario ID error, got %v", missingScenarioID)
	}
	missingFeatureIdentity := ValidateTestScenarioTrace(scenarios, TestScenarioTrace{ScenarioID: "SCN-015", TestName: "TestSCN015_TraceIncludesStableScenarioIDOnly"})
	if missingFeatureIdentity == nil || !strings.Contains(missingFeatureIdentity.Error(), "feature identity") {
		t.Fatalf("expected missing feature identity error, got %v", missingFeatureIdentity)
	}
}

func assertTraceValidationAcceptsQualifiedTraces(t *testing.T, scenarios []FeatureScenario) {
	t.Helper()
	metadata := TestScenarioTrace{FeaturePath: "features/workflow_artifact_lifecycle.feature", ScenarioID: "SCN-015", TestName: "TestSCN015_TraceIncludesFeatureIdentityInMetadata", Metadata: map[string]string{"scenario": "features/workflow_artifact_lifecycle.feature#SCN-015"}}
	if err := ValidateTestScenarioTrace(scenarios, metadata); err != nil {
		t.Fatalf("expected metadata trace to validate: %v", err)
	}
	subtest := TestScenarioTrace{FeaturePath: "features/workflow_artifact_lifecycle.feature", ScenarioID: "SCN-015", SubtestNames: []string{"features/workflow_artifact_lifecycle.feature#SCN-015 Tests reference stable scenario IDs from feature files"}}
	if err := ValidateTestScenarioTrace(scenarios, subtest); err != nil {
		t.Fatalf("expected feature-qualified subtest trace to validate: %v", err)
	}
}

func assertTraceValidationRejectsUnknownFeature(t *testing.T, scenarios []FeatureScenario) {
	t.Helper()
	trace := TestScenarioTrace{FeaturePath: "features/missing_contract.feature", ScenarioID: "SCN-015", TestName: "TestSCN015_UnknownFeatureIdentityIsNotAccepted", Metadata: map[string]string{"scenario": "features/workflow_artifact_lifecycle.feature#SCN-015"}}
	err := ValidateTestScenarioTrace(scenarios, trace)
	if err == nil || !strings.Contains(err.Error(), "approved feature scenario") {
		t.Fatalf("expected unknown feature identity to be rejected, got %v", err)
	}
}

func TestSCN015_AmbiguityDetectionRequiresSameScenarioAcrossDifferentFeatures(t *testing.T) {
	// REQ-013 → REQ-019 → SCN-015 → TestSCN015_AmbiguityDetectionRequiresSameScenarioAcrossDifferentFeatures
	// Scenario: Tests reference stable scenario IDs from feature files
	target := FeatureScenario{FeaturePath: "features/workflow_artifact_lifecycle.feature", ScenarioID: "SCN-015"}
	scenarios := []FeatureScenario{
		target,
		{FeaturePath: "features/workflow_artifact_lifecycle.feature", ScenarioID: "SCN-015", Name: "duplicate within same feature"},
		{FeaturePath: "features/other_contract.feature", ScenarioID: "SCN-099"},
	}

	if scenarioIDIsAmbiguous(scenarios, target) {
		t.Fatalf("expected duplicate IDs in the same feature or unrelated IDs elsewhere not to require feature-qualified trace")
	}
	if scenarioIDMatchesMultipleFeatures(scenarios, "SCN-015") {
		t.Fatalf("expected SCN-015 to appear in only one feature identity")
	}

	ambiguous := append(scenarios, FeatureScenario{FeaturePath: "features/another_contract.feature", ScenarioID: "SCN-015"})
	if !scenarioIDIsAmbiguous(ambiguous, target) {
		t.Fatalf("expected same scenario ID in another feature to be ambiguous")
	}
	if !scenarioIDMatchesMultipleFeatures(ambiguous, "SCN-015") {
		t.Fatalf("expected SCN-015 to match multiple feature identities")
	}
}

func TestSCN015_TestTraceValidatorAcceptsExplicitFeaturePathWithoutDuplicateIdentity(t *testing.T) {
	// REQ-013 → REQ-019 → SCN-015 → TestSCN015_TestTraceValidatorAcceptsExplicitFeaturePathWithoutDuplicateIdentity
	// Scenario: Tests reference stable scenario IDs from feature files
	unique := []FeatureScenario{
		{FeaturePath: "features/workflow_artifact_lifecycle.feature", ScenarioID: "SCN-015", RequirementIDs: []string{"REQ-013"}},
	}
	if err := ValidateTestScenarioTrace(unique, TestScenarioTrace{
		FeaturePath: "features/workflow_artifact_lifecycle.feature",
		ScenarioID:  "SCN-015",
		TestName:    "TestSCN015_TestTraceValidatorAcceptsExplicitFeaturePathWithoutDuplicateIdentity",
	}); err != nil {
		t.Fatalf("expected explicit feature path plus stable scenario ID to validate for a unique scenario: %v", err)
	}

	ambiguous := append(unique, FeatureScenario{FeaturePath: "features/other_contract.feature", ScenarioID: "SCN-015"})
	if err := ValidateTestScenarioTrace(ambiguous, TestScenarioTrace{
		FeaturePath: "features/workflow_artifact_lifecycle.feature",
		ScenarioID:  "SCN-015",
		TestName:    "TestSCN015_TestTraceValidatorAcceptsExplicitFeaturePathWithoutDuplicateIdentity",
	}); err != nil {
		t.Fatalf("expected explicit feature path to disambiguate duplicate scenario IDs: %v", err)
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
