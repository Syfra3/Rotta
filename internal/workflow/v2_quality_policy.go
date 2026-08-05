package workflow

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"path"
)

const v2QualityPolicyPath = ".rotta/v2/quality-policy.yaml"

type V2QualityPolicy struct {
	Format     string                     `json:"format"`
	ID         string                     `json:"id"`
	Version    string                     `json:"version"`
	Analyzers  []V2QualityPolicyAnalyzer  `json:"analyzers"`
	Dimensions []V2QualityPolicyDimension `json:"dimensions"`
}

type V2QualityPolicyDimension struct {
	ID                    string                     `json:"id"`
	Analyzer              string                     `json:"analyzer"`
	Gate                  string                     `json:"gate"`
	AppliesWhen           string                     `json:"applies_when"`
	ChangedPathMatchesAny []string                   `json:"changed_path_matches_any"`
	SeverityScale         []string                   `json:"severity_scale"`
	BlockingSeverity      string                     `json:"blocking_severity"`
	Thresholds            []V2QualityMetricThreshold `json:"thresholds"`
	Required              bool                       `json:"-"`
}

type V2QualityMetricThreshold struct {
	MetricID   string          `json:"metric_id"`
	Comparator string          `json:"comparator"`
	Target     json.RawMessage `json:"target"`
	Unit       string          `json:"unit,omitempty"`
}

type V2QualityPolicyAnalyzer struct {
	ID                    string   `json:"id"`
	Command               []string `json:"command"`
	Adapter               string   `json:"adapter"`
	SupportedDimensions   []string `json:"supported_dimensions"`
	AppliesWhen           string   `json:"applies_when"`
	ChangedPathMatchesAny []string `json:"changed_path_matches_any"`
	Parser                string   `json:"parser"`
}

func LoadV2QualityPolicy(repoRoot string) (V2QualityPolicy, string, error) {
	contents, err := readRepositoryFile(repoRoot, v2QualityPolicyPath)
	if err != nil {
		return V2QualityPolicy{}, "", fmt.Errorf("read v2 quality policy: %w", err)
	}
	var policy V2QualityPolicy
	if err := json.Unmarshal(contents, &policy); err != nil {
		return V2QualityPolicy{}, "", fmt.Errorf("parse v2 quality policy: %w", err)
	}
	if err := validateV2QualityPolicy(policy); err != nil {
		return V2QualityPolicy{}, "", err
	}
	return policy, fmt.Sprintf("sha256:%x", sha256.Sum256(contents)), nil
}

func validateV2QualityPolicy(policy V2QualityPolicy) error {
	if policy.Format != "rotta-quality-policy/v1" || policy.ID == "" || policy.Version == "" || len(policy.Analyzers) == 0 || len(policy.Dimensions) != len(v2CanonicalQualityDimensions) {
		return errors.New("v2 quality policy is incomplete or has an unsupported format")
	}
	analyzers := make(map[string]V2QualityPolicyAnalyzer, len(policy.Analyzers))
	for _, analyzer := range policy.Analyzers {
		if analyzer.ID == "" || analyzers[analyzer.ID].ID != "" || (len(analyzer.Command) == 0) == (analyzer.Adapter == "") || !validV2Predicate(analyzer.AppliesWhen, analyzer.ChangedPathMatchesAny) || analyzer.Parser != "quality-result/v1" || len(analyzer.SupportedDimensions) == 0 {
			return errors.New("v2 quality policy has an invalid analyzer")
		}
		analyzers[analyzer.ID] = analyzer
	}
	seen := make(map[string]bool, len(policy.Dimensions))
	for index := range policy.Dimensions {
		dimension := &policy.Dimensions[index]
		analyzer, knownAnalyzer := analyzers[dimension.Analyzer]
		if !containsV2Scenario(v2CanonicalQualityDimensions, dimension.ID) || seen[dimension.ID] || !knownAnalyzer || !containsV2Scenario(analyzer.SupportedDimensions, dimension.ID) || (dimension.Gate != "required" && dimension.Gate != "advisory") || !validV2Predicate(dimension.AppliesWhen, dimension.ChangedPathMatchesAny) || len(dimension.SeverityScale) == 0 || !containsV2Scenario(dimension.SeverityScale, dimension.BlockingSeverity) {
			return errors.New("v2 quality policy has duplicate or unknown dimensions")
		}
		for _, threshold := range dimension.Thresholds {
			if !validV2Threshold(threshold) {
				return errors.New("v2 quality policy has an invalid metric threshold")
			}
		}
		dimension.Required = dimension.Gate == "required"
		seen[dimension.ID] = true
	}
	return nil
}

func validV2Threshold(threshold V2QualityMetricThreshold) bool {
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

func validV2Predicate(predicate string, patterns []string) bool {
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

func evaluateV2Applicability(predicate string, patterns, changedPaths []string) (bool, string) {
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
