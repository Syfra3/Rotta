package workflow

import (
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
