package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type V2QualityOutcome string

const (
	V2QualityPass          V2QualityOutcome = "pass"
	V2QualityFail          V2QualityOutcome = "fail"
	V2QualityBlocked       V2QualityOutcome = "blocked"
	V2QualityNotApplicable V2QualityOutcome = "not_applicable"
)

var v2CanonicalQualityDimensions = []string{
	"code_smells", "duplication", "complexity", "maintainability", "good_practices",
	"security", "risk", "tests", "static_analysis", "coverage",
}

// V2QualityDimensionResult is the normalized quality-result/v1 decision for
// one policy dimension. Evidence pointers remain external and redacted.
type V2QualityDimensionResult struct {
	Dimension  string             `json:"dimension"`
	Required   bool               `json:"required"`
	Applicable bool               `json:"applicable"`
	Outcome    V2QualityOutcome   `json:"outcome"`
	Reason     string             `json:"reason,omitempty"`
	Findings   []V2QualityFinding `json:"findings,omitempty"`
	Metrics    []V2QualityMetric  `json:"metrics,omitempty"`
}

type V2QualityMetric struct {
	ID    string      `json:"id"`
	Value interface{} `json:"value"`
	Unit  string      `json:"unit,omitempty"`
}

type V2QualityFinding struct {
	ID                        string `json:"id"`
	Dimension                 string `json:"dimension"`
	Severity                  string `json:"severity"`
	Message                   string `json:"message"`
	Location                  string `json:"location,omitempty"`
	LocationUnavailableReason string `json:"location_unavailable_reason,omitempty"`
}

type V2QualityReviewRequest struct {
	SubmissionID      string
	LedgerVersion     uint64
	Authorizer        string
	CandidateCommit   string
	PolicyID          string
	PolicyVersion     string
	PolicyFingerprint string
	Dimensions        []V2QualityDimensionResult
	Results           []V2QualityResult
	ChangedPaths      []string
	EvidenceRefs      []string
}

// ApplyV2QualityReview evaluates normalized quality outcomes already returned
// by a bounded Review worker. It does not run analyzers or infer outcomes.
func ApplyV2QualityReview(repoRoot string, request V2QualityReviewRequest) (V2SubmissionLedger, error) {
	if err := validateV2SubmissionID(request.SubmissionID); err != nil {
		return V2SubmissionLedger{}, err
	}
	if request.Authorizer != v2OrchestratorAuthorizer || !isFullCommitID(request.CandidateCommit) || strings.TrimSpace(request.PolicyID) == "" || strings.TrimSpace(request.PolicyVersion) == "" || strings.TrimSpace(request.PolicyFingerprint) == "" || len(request.EvidenceRefs) == 0 {
		return V2SubmissionLedger{}, errors.New("v2 quality review is incomplete or unauthorized")
	}
	policy, fingerprint, err := LoadV2QualityPolicy(repoRoot)
	if err != nil {
		return V2SubmissionLedger{}, fmt.Errorf("v2 quality review is blocked: %w", err)
	}
	if request.PolicyID != policy.ID || request.PolicyVersion != policy.Version || request.PolicyFingerprint != fingerprint {
		return V2SubmissionLedger{}, errors.New("v2 quality review is blocked: request policy identity does not match the committed policy")
	}
	if len(request.Results) == 0 || len(request.Dimensions) != 0 {
		return V2SubmissionLedger{}, errors.New("v2 quality review is blocked: normalized analyzer results are required and raw dimensions are not accepted")
	}
	dimensions, err := MergeV2QualityResults(request.Results, policy, fingerprint, request.CandidateCommit)
	if err != nil {
		return V2SubmissionLedger{}, fmt.Errorf("v2 quality review is blocked: %w", err)
	}
	decision, err := validateV2QualityDimensions(dimensions, policy, request.ChangedPaths)
	if err != nil {
		return V2SubmissionLedger{}, err
	}

	unlock, err := lockV2Ledger(repoRoot, request.SubmissionID)
	if err != nil {
		return V2SubmissionLedger{}, err
	}
	defer unlock()
	ledger, err := LoadV2SubmissionLedger(repoRoot, request.SubmissionID)
	if err != nil {
		return V2SubmissionLedger{}, err
	}
	if ledger.Status != V2ReviewStatus || ledger.LedgerVersion != request.LedgerVersion {
		return V2SubmissionLedger{}, fmt.Errorf("v2 quality review rejected: expected Review at ledger version %d", request.LedgerVersion)
	}
	ledger.ImplementationCommit = strings.ToLower(request.CandidateCommit)
	ledger.QualityPolicyFingerprint = request.PolicyFingerprint
	ledger.QualityEvidence = append([]V2QualityDimensionResult(nil), dimensions...)
	if decision == V2QualityPass {
		ledger.ReviewedCommit = ledger.ImplementationCommit
		ledger.Status = V2ArchiveStatus
	} else if decision == V2QualityFail {
		ledger.Status = V2TDDStatus
	}
	ledger.LedgerVersion++
	if err := writeV2LedgerAtomically(v2LedgerPath(repoRoot, request.SubmissionID), ledger); err != nil {
		return V2SubmissionLedger{}, err
	}
	return ledger, nil
}

func validateV2QualityDimensions(dimensions []V2QualityDimensionResult, policy V2QualityPolicy, changedPaths []string) (V2QualityOutcome, error) {
	if len(dimensions) != len(v2CanonicalQualityDimensions) {
		return "", errors.New("v2 quality review is blocked: every canonical quality dimension requires an explicit result")
	}
	seen := make(map[string]bool, len(dimensions))
	policyDimensions := make(map[string]V2QualityPolicyDimension, len(policy.Dimensions))
	for _, dimension := range policy.Dimensions {
		policyDimensions[dimension.ID] = dimension
	}
	decision := V2QualityPass
	for _, dimension := range dimensions {
		if !containsV2Scenario(v2CanonicalQualityDimensions, dimension.Dimension) || seen[dimension.Dimension] {
			return "", errors.New("v2 quality review is blocked: duplicate or unknown quality dimension")
		}
		seen[dimension.Dimension] = true
		if dimension.Required != policyDimensions[dimension.Dimension].Required {
			return "", fmt.Errorf("v2 quality review is blocked: %s requiredness does not match the committed policy", dimension.Dimension)
		}
		expectedApplicable, _ := evaluateV2Applicability(policyDimensions[dimension.Dimension].AppliesWhen, policyDimensions[dimension.Dimension].ChangedPathMatchesAny, changedPaths)
		if dimension.Applicable != expectedApplicable {
			return "", fmt.Errorf("v2 quality review is blocked: %s applicability does not match recorded changed paths", dimension.Dimension)
		}
		if dimension.Applicable && dimension.Outcome == V2QualityNotApplicable {
			return "", fmt.Errorf("v2 quality review is blocked: applicable %s cannot be not_applicable", dimension.Dimension)
		}
		if !dimension.Applicable && dimension.Outcome != V2QualityNotApplicable {
			return "", fmt.Errorf("v2 quality review is blocked: non-applicable %s requires not_applicable", dimension.Dimension)
		}
		if dimension.Outcome != V2QualityPass && dimension.Outcome != V2QualityFail && dimension.Outcome != V2QualityBlocked && dimension.Outcome != V2QualityNotApplicable {
			return "", fmt.Errorf("v2 quality review is blocked: invalid outcome for %s", dimension.Dimension)
		}
		if err := validateV2FindingSeverities(dimension, policyDimensions[dimension.Dimension]); err != nil {
			return "", err
		}
		if err := validateV2MetricThresholds(dimension, policyDimensions[dimension.Dimension]); err != nil {
			return "", err
		}
		if dimension.Required && dimension.Applicable && dimension.Outcome == V2QualityBlocked {
			decision = V2QualityBlocked
		}
		if dimension.Required && dimension.Applicable && dimension.Outcome == V2QualityFail && decision != V2QualityBlocked {
			decision = V2QualityFail
		}
	}
	return decision, nil
}

func validateV2MetricThresholds(result V2QualityDimensionResult, policy V2QualityPolicyDimension) error {
	metrics := make(map[string]V2QualityMetric, len(result.Metrics))
	for _, metric := range result.Metrics {
		if metric.ID == "" || metrics[metric.ID].ID != "" {
			return fmt.Errorf("v2 quality review is blocked: malformed metrics for %s", result.Dimension)
		}
		metrics[metric.ID] = metric
	}
	for _, threshold := range policy.Thresholds {
		metric, found := metrics[threshold.MetricID]
		if !found || metric.Unit != threshold.Unit {
			return fmt.Errorf("v2 quality review is blocked: missing or mismatched metric %s for %s", threshold.MetricID, result.Dimension)
		}
		breached, valid := v2ThresholdBreached(metric.Value, threshold)
		if !valid {
			return fmt.Errorf("v2 quality review is blocked: ill-typed metric %s for %s", threshold.MetricID, result.Dimension)
		}
		if breached && result.Outcome != V2QualityFail {
			return fmt.Errorf("v2 quality review is blocked: %s reports pass despite a breached metric threshold", result.Dimension)
		}
	}
	return nil
}

func v2ThresholdBreached(value interface{}, threshold V2QualityMetricThreshold) (bool, bool) {
	var targetNumber float64
	if json.Unmarshal(threshold.Target, &targetNumber) == nil {
		actual, ok := value.(float64)
		if !ok {
			return false, false
		}
		switch threshold.Comparator {
		case "lt":
			return actual >= targetNumber, true
		case "lte":
			return actual > targetNumber, true
		case "eq":
			return actual != targetNumber, true
		case "neq":
			return actual == targetNumber, true
		case "gte":
			return actual < targetNumber, true
		case "gt":
			return actual <= targetNumber, true
		}
	}
	var targetBoolean bool
	if json.Unmarshal(threshold.Target, &targetBoolean) != nil {
		return false, false
	}
	actual, ok := value.(bool)
	if !ok {
		return false, false
	}
	if threshold.Comparator == "eq" {
		return actual != targetBoolean, true
	}
	if threshold.Comparator == "neq" {
		return actual == targetBoolean, true
	}
	return false, false
}

func validateV2FindingSeverities(result V2QualityDimensionResult, policy V2QualityPolicyDimension) error {
	blockingIndex := v2SeverityIndex(policy.SeverityScale, policy.BlockingSeverity)
	for _, finding := range result.Findings {
		if finding.ID == "" || finding.Dimension != result.Dimension || finding.Message == "" || (finding.Location == "" && finding.LocationUnavailableReason == "") {
			return fmt.Errorf("v2 quality review is blocked: malformed finding for %s", result.Dimension)
		}
		severityIndex := v2SeverityIndex(policy.SeverityScale, finding.Severity)
		if severityIndex < 0 {
			return fmt.Errorf("v2 quality review is blocked: %s has severity absent from the committed scale", result.Dimension)
		}
		if severityIndex >= blockingIndex && result.Outcome != V2QualityFail {
			return fmt.Errorf("v2 quality review is blocked: %s reports pass despite a blocking-severity finding", result.Dimension)
		}
	}
	return nil
}

func v2SeverityIndex(scale []string, severity string) int {
	for index, candidate := range scale {
		if candidate == severity {
			return index
		}
	}
	return -1
}
