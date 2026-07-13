package workflow

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type FeatureScenario struct {
	FeaturePath    string
	ScenarioID     string
	RequirementIDs []string
	Name           string
}

type TestScenarioTrace struct {
	FeaturePath  string
	ScenarioID   string
	TestName     string
	Metadata     map[string]string
	SubtestNames []string
}

var (
	scenarioIDTagPattern    = regexp.MustCompile(`^SCN-[0-9]+$`)
	requirementIDTagPattern = regexp.MustCompile(`^REQ-[0-9]+$`)
)

func ParseFeatureScenarioTags(featurePath string, reader io.Reader) ([]FeatureScenario, error) {
	scanner := bufio.NewScanner(reader)
	var scenarios []FeatureScenario
	var pendingTags []string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "@") {
			pendingTags = strings.Fields(line)
			continue
		}

		name, ok := strings.CutPrefix(line, "Scenario:")
		if !ok {
			continue
		}

		scenario := FeatureScenario{
			FeaturePath:    featurePath,
			Name:           strings.TrimSpace(name),
			RequirementIDs: requirementIDsFromTags(pendingTags),
		}
		scenario.ScenarioID = scenarioIDFromTags(pendingTags)
		scenarios = append(scenarios, scenario)
		pendingTags = nil
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("parse feature scenarios %s: %w", featurePath, err)
	}

	return scenarios, nil
}

func ValidateTestScenarioTrace(scenarios []FeatureScenario, trace TestScenarioTrace) error {
	if trace.FeaturePath == "" && trace.ScenarioID != "" && scenarioIDMatchesMultipleFeatures(scenarios, trace.ScenarioID) {
		return fmt.Errorf("test trace for %s must include feature identity", trace.ScenarioID)
	}

	scenario, ok := findScenario(scenarios, trace.FeaturePath, trace.ScenarioID)
	if !ok {
		return fmt.Errorf("test trace must reference an approved feature scenario such as SCN-015")
	}

	identity := scenario.FeaturePath + "#" + scenario.ScenarioID
	traceFields := traceSearchFields(trace)
	if !fieldsContain(traceFields, scenario.ScenarioID) {
		return fmt.Errorf("test trace for %s must include stable scenario ID %s", identity, scenario.ScenarioID)
	}
	if scenarioIDIsAmbiguous(scenarios, scenario) && !fieldsContain(traceFields, identity) && trace.FeaturePath != scenario.FeaturePath {
		return fmt.Errorf("test trace for %s must include feature identity", identity)
	}

	return nil
}

func PlanImplementationReadyScenarios(repoRoot string) ([]FeatureScenario, error) {
	featuresRoot := filepath.Join(repoRoot, "features")
	var planned []FeatureScenario
	err := filepath.WalkDir(featuresRoot, implementationScenarioPlanner(repoRoot, &planned))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("plan implementation-ready scenarios: %w", err)
	}

	return planned, nil
}

func implementationScenarioPlanner(repoRoot string, planned *[]FeatureScenario) fs.WalkDirFunc {
	return func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || filepath.Ext(path) != ".feature" {
			return walkErr
		}
		featurePath, err := relativeFeaturePath(repoRoot, path)
		if err != nil {
			return err
		}
		scenarios, err := implementationScenariosInFeature(repoRoot, featurePath)
		if err != nil {
			return err
		}
		*planned = append(*planned, scenarios...)
		return nil
	}
}

func implementationScenariosInFeature(repoRoot, featurePath string) ([]FeatureScenario, error) {
	file, closeFile, err := openRepositoryFile(repoRoot, featurePath)
	if err != nil {
		return nil, fmt.Errorf("open feature file %s: %w", featurePath, err)
	}
	defer closeFile()
	scenarios, err := ParseFeatureScenarioTags(featurePath, file)
	if err != nil {
		return nil, err
	}
	return approvedFeatureScenarios(repoRoot, featurePath, scenarios)
}

func approvedFeatureScenarios(repoRoot, featurePath string, scenarios []FeatureScenario) ([]FeatureScenario, error) {
	var planned []FeatureScenario
	for _, scenario := range scenarios {
		if scenario.ScenarioID == "" {
			continue
		}
		approved, err := scopedApprovalContains(repoRoot, ContractScope{SpecPath: specPathForFeature(featurePath), FeaturePath: featurePath, ScenarioID: scenario.ScenarioID})
		if err != nil {
			return nil, err
		}
		if approved {
			planned = append(planned, scenario)
		}
	}
	return planned, nil
}

func specPathForFeature(featurePath string) string {
	contractID := strings.TrimSuffix(filepath.Base(featurePath), filepath.Ext(featurePath))
	return filepath.ToSlash(filepath.Join("specs", contractID+".md"))
}

func scenarioIDFromTags(tags []string) string {
	for _, tag := range normalizedTags(tags) {
		if scenarioIDTagPattern.MatchString(tag) {
			return tag
		}
	}
	return ""
}

func requirementIDsFromTags(tags []string) []string {
	var ids []string
	for _, tag := range normalizedTags(tags) {
		if requirementIDTagPattern.MatchString(tag) {
			ids = append(ids, tag)
		}
	}
	return ids
}

func normalizedTags(tags []string) []string {
	normalized := make([]string, 0, len(tags))
	for _, tag := range tags {
		normalized = append(normalized, strings.TrimPrefix(strings.TrimSpace(tag), "@"))
	}
	return normalized
}

func findScenario(scenarios []FeatureScenario, featurePath, scenarioID string) (FeatureScenario, bool) {
	for _, scenario := range scenarios {
		if scenario.FeaturePath == featurePath && scenario.ScenarioID == scenarioID {
			return scenario, true
		}
	}
	return FeatureScenario{}, false
}

func traceSearchFields(trace TestScenarioTrace) []string {
	fields := []string{trace.FeaturePath, trace.ScenarioID, trace.TestName}
	fields = append(fields, trace.SubtestNames...)
	keys := make([]string, 0, len(trace.Metadata))
	for key := range trace.Metadata {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fields = append(fields, key, trace.Metadata[key])
	}
	return fields
}

func fieldsContain(fields []string, want string) bool {
	for _, field := range fields {
		if strings.Contains(field, want) {
			return true
		}
	}
	return false
}

func scenarioIDIsAmbiguous(scenarios []FeatureScenario, target FeatureScenario) bool {
	for _, scenario := range scenarios {
		if scenario.ScenarioID == target.ScenarioID && scenario.FeaturePath != target.FeaturePath {
			return true
		}
	}
	return false
}

func scenarioIDMatchesMultipleFeatures(scenarios []FeatureScenario, scenarioID string) bool {
	features := map[string]bool{}
	for _, scenario := range scenarios {
		if scenario.ScenarioID == scenarioID {
			features[scenario.FeaturePath] = true
		}
	}
	return len(features) > 1
}
