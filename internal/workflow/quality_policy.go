package workflow

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"path"
)

const QualityPolicyPath = ".rotta/workflow/quality-policy.yaml"

type QualityPolicy struct {
	Format     string                   `json:"format"`
	ID         string                   `json:"id"`
	Version    string                   `json:"version"`
	Analyzers  []QualityPolicyAnalyzer  `json:"analyzers"`
	Dimensions []QualityPolicyDimension `json:"dimensions"`
}

type QualityPolicyDimension struct {
	ID                    string                   `json:"id"`
	Analyzer              string                   `json:"analyzer"`
	Gate                  string                   `json:"gate"`
	AppliesWhen           string                   `json:"applies_when"`
	ChangedPathMatchesAny []string                 `json:"changed_path_matches_any"`
	SeverityScale         []string                 `json:"severity_scale"`
	BlockingSeverity      string                   `json:"blocking_severity"`
	Thresholds            []QualityMetricThreshold `json:"thresholds"`
	Required              bool                     `json:"-"`
}

type QualityMetricThreshold struct {
	MetricID   string          `json:"metric_id"`
	Comparator string          `json:"comparator"`
	Target     json.RawMessage `json:"target"`
	Unit       string          `json:"unit,omitempty"`
}

type QualityPolicyAnalyzer struct {
	ID                    string   `json:"id"`
	Command               []string `json:"command"`
	Adapter               string   `json:"adapter"`
	SupportedDimensions   []string `json:"supported_dimensions"`
	AppliesWhen           string   `json:"applies_when"`
	ChangedPathMatchesAny []string `json:"changed_path_matches_any"`
	Parser                string   `json:"parser"`
}

func LoadQualityPolicy(repoRoot string) (QualityPolicy, string, error) {
	contents, err := readRepositoryFile(repoRoot, QualityPolicyPath)
	if err != nil {
		return QualityPolicy{}, "", fmt.Errorf("read  quality policy: %w", err)
	}
	var policy QualityPolicy
	if err := json.Unmarshal(contents, &policy); err != nil {
		return QualityPolicy{}, "", fmt.Errorf("parse  quality policy: %w", err)
	}
	if err := validateQualityPolicy(policy); err != nil {
		return QualityPolicy{}, "", err
	}
	return policy, fmt.Sprintf("sha256:%x", sha256.Sum256(contents)), nil
}

func validateQualityPolicy(policy QualityPolicy) error {
	if policy.Format != "rotta-quality-policy/v1" || policy.ID == "" || policy.Version == "" || len(policy.Analyzers) == 0 || len(policy.Dimensions) != len(CanonicalQualityDimensions) {
		return errors.New(" quality policy is incomplete or has an unsupported format")
	}
	analyzers := make(map[string]QualityPolicyAnalyzer, len(policy.Analyzers))
	for _, analyzer := range policy.Analyzers {
		if analyzer.ID == "" || analyzers[analyzer.ID].ID != "" || (len(analyzer.Command) == 0) == (analyzer.Adapter == "") || !validPredicate(analyzer.AppliesWhen, analyzer.ChangedPathMatchesAny) || analyzer.Parser != "quality-result/v1" || len(analyzer.SupportedDimensions) == 0 {
			return errors.New(" quality policy has an invalid analyzer")
		}
		analyzers[analyzer.ID] = analyzer
	}
	seen := make(map[string]bool, len(policy.Dimensions))
	for index := range policy.Dimensions {
		dimension := &policy.Dimensions[index]
		analyzer, knownAnalyzer := analyzers[dimension.Analyzer]
		if !containsScenario(CanonicalQualityDimensions, dimension.ID) || seen[dimension.ID] || !knownAnalyzer || !containsScenario(analyzer.SupportedDimensions, dimension.ID) || (dimension.Gate != "required" && dimension.Gate != "advisory") || !validPredicate(dimension.AppliesWhen, dimension.ChangedPathMatchesAny) || len(dimension.SeverityScale) == 0 || !containsScenario(dimension.SeverityScale, dimension.BlockingSeverity) {
			return errors.New(" quality policy has duplicate or unknown dimensions")
		}
		for _, threshold := range dimension.Thresholds {
			if !validThreshold(threshold) {
				return errors.New(" quality policy has an invalid metric threshold")
			}
		}
		dimension.Required = dimension.Gate == "required"
		seen[dimension.ID] = true
	}
	return nil
}

func validThreshold(threshold QualityMetricThreshold) bool {
	if threshold.MetricID == "" || (threshold.Comparator != "lt" && threshold.Comparator != "lte" && threshold.Comparator != "eq" && threshold.Comparator != "neq" && threshold.Comparator != "gte" && threshold.Comparator != "gt") {
		return false
	}
	var number float64
	if json.Unmarshal(threshold.Target, &number) == nil {
		return true
	}
	var boolean bool
	return json.Unmarshal(threshold.Target, &boolean) == nil && (threshold.Comparator == "eq" || threshold.Comparator == "neq")
}

func validPredicate(predicate string, patterns []string) bool {
	if predicate == "all" {
		return len(patterns) == 0
	}
	if predicate != "changed_path_matches_any" || len(patterns) == 0 {
		return false
	}
	for _, pattern := range patterns {
		if pattern == "" || path.IsAbs(pattern) {
			return false
		}
	}
	return true
}

func evaluateApplicability(predicate string, patterns, changedPaths []string) (bool, string) {
	if predicate == "all" {
		return true, "policy predicate all"
	}
	for _, changedPath := range changedPaths {
		for _, pattern := range patterns {
			matched, err := path.Match(pattern, changedPath)
			if err == nil && matched {
				return true, "changed path matched committed predicate"
			}
		}
	}
	return false, "no recorded changed path matched committed predicate"
}
