package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type QualityOutcome string

const (
	QualityPass          QualityOutcome = "pass"
	QualityFail          QualityOutcome = "fail"
	QualityBlocked       QualityOutcome = "blocked"
	QualityNotApplicable QualityOutcome = "not_applicable"
)

var CanonicalQualityDimensions = []string{
	"code_smells", "duplication", "complexity", "maintainability", "good_practices",
	"security", "risk", "tests", "static_analysis", "coverage",
}

// QualityDimensionResult is the normalized quality-result/v1 decision for
// one policy dimension. Evidence pointers remain external and redacted.
type QualityDimensionResult struct {
	Dimension  string           `json:"dimension"`
	Required   bool             `json:"required"`
	Applicable bool             `json:"applicable"`
	Outcome    QualityOutcome   `json:"outcome"`
	Reason     string           `json:"reason,omitempty"`
	Findings   []QualityFinding `json:"findings,omitempty"`
	Metrics    []QualityMetric  `json:"metrics,omitempty"`
}

type QualityMetric struct {
	ID    string      `json:"id"`
	Value interface{} `json:"value"`
	Unit  string      `json:"unit,omitempty"`
}

type QualityFinding struct {
	ID                        string `json:"id"`
	Dimension                 string `json:"dimension"`
	Severity                  string `json:"severity"`
	Message                   string `json:"message"`
	Location                  string `json:"location,omitempty"`
	LocationUnavailableReason string `json:"location_unavailable_reason,omitempty"`
}

type QualityReviewRequest struct {
	SubmissionID      string
	LedgerVersion     uint64
	Authorizer        string
	CandidateCommit   string
	PolicyID          string
	PolicyVersion     string
	PolicyFingerprint string
	Dimensions        []QualityDimensionResult
	Results           []QualityResult
	ChangedPaths      []string
	EvidenceRefs      []string
}

// ApplyQualityReview evaluates normalized quality outcomes already returned
// by a bounded Review worker. It does not run analyzers or infer outcomes.
func ApplyQualityReview(repoRoot string, request QualityReviewRequest) (SubmissionLedger, error) {
	if err := validateSubmissionID(request.SubmissionID); err != nil {
		return SubmissionLedger{}, err
	}
	if request.Authorizer != orchestratorAuthorizer || !isFullCommitID(request.CandidateCommit) || strings.TrimSpace(request.PolicyID) == "" || strings.TrimSpace(request.PolicyVersion) == "" || strings.TrimSpace(request.PolicyFingerprint) == "" || len(request.EvidenceRefs) == 0 {
		return SubmissionLedger{}, errors.New(" quality review is incomplete or unauthorized")
	}
	policy, fingerprint, err := LoadQualityPolicy(repoRoot)
	if err != nil {
		return SubmissionLedger{}, fmt.Errorf(" quality review is blocked: %w", err)
	}
	if request.PolicyID != policy.ID || request.PolicyVersion != policy.Version || request.PolicyFingerprint != fingerprint {
		return SubmissionLedger{}, errors.New(" quality review is blocked: request policy identity does not match the committed policy")
	}
	if len(request.Results) == 0 || len(request.Dimensions) != 0 {
		return SubmissionLedger{}, errors.New(" quality review is blocked: normalized analyzer results are required and raw dimensions are not accepted")
	}
	dimensions, err := MergeQualityResults(request.Results, policy, fingerprint, request.CandidateCommit)
	if err != nil {
		return SubmissionLedger{}, fmt.Errorf(" quality review is blocked: %w", err)
	}
	decision, err := validateQualityDimensions(dimensions, policy, request.ChangedPaths)
	if err != nil {
		return SubmissionLedger{}, err
	}

	unlock, err := lockLedger(repoRoot, request.SubmissionID)
	if err != nil {
		return SubmissionLedger{}, err
	}
	defer unlock()
	ledger, err := LoadSubmissionLedger(repoRoot, request.SubmissionID)
	if err != nil {
		return SubmissionLedger{}, err
	}
	if ledger.Status != ReviewStatus || ledger.LedgerVersion != request.LedgerVersion {
		return SubmissionLedger{}, fmt.Errorf(" quality review rejected: expected Review at ledger version %d", request.LedgerVersion)
	}
	ledger.ImplementationCommit = strings.ToLower(request.CandidateCommit)
	ledger.QualityPolicyFingerprint = request.PolicyFingerprint
	ledger.QualityEvidence = append([]QualityDimensionResult(nil), dimensions...)
	if decision == QualityPass {
		ledger.ReviewedCommit = ledger.ImplementationCommit
		ledger.Status = ArchiveStatus
	} else if decision == QualityFail {
		ledger.Status = TDDStatus
	}
	ledger.LedgerVersion++
	if err := writeLedgerAtomically(ledgerPath(repoRoot, request.SubmissionID), ledger); err != nil {
		return SubmissionLedger{}, err
	}
	return ledger, nil
}

func validateQualityDimensions(dimensions []QualityDimensionResult, policy QualityPolicy, changedPaths []string) (QualityOutcome, error) {
	if len(dimensions) != len(CanonicalQualityDimensions) {
		return "", errors.New(" quality review is blocked: every canonical quality dimension requires an explicit result")
	}
	seen := make(map[string]bool, len(dimensions))
	policyDimensions := make(map[string]QualityPolicyDimension, len(policy.Dimensions))
	for _, dimension := range policy.Dimensions {
		policyDimensions[dimension.ID] = dimension
	}
	decision := QualityPass
	for _, dimension := range dimensions {
		if !containsScenario(CanonicalQualityDimensions, dimension.Dimension) || seen[dimension.Dimension] {
			return "", errors.New(" quality review is blocked: duplicate or unknown quality dimension")
		}
		seen[dimension.Dimension] = true
		if dimension.Required != policyDimensions[dimension.Dimension].Required {
			return "", fmt.Errorf(" quality review is blocked: %s requiredness does not match the committed policy", dimension.Dimension)
		}
		expectedApplicable, _ := evaluateApplicability(policyDimensions[dimension.Dimension].AppliesWhen, policyDimensions[dimension.Dimension].ChangedPathMatchesAny, changedPaths)
		if dimension.Applicable != expectedApplicable {
			return "", fmt.Errorf(" quality review is blocked: %s applicability does not match recorded changed paths", dimension.Dimension)
		}
		if dimension.Applicable && dimension.Outcome == QualityNotApplicable {
			return "", fmt.Errorf(" quality review is blocked: applicable %s cannot be not_applicable", dimension.Dimension)
		}
		if !dimension.Applicable && dimension.Outcome != QualityNotApplicable {
			return "", fmt.Errorf(" quality review is blocked: non-applicable %s requires not_applicable", dimension.Dimension)
		}
		if dimension.Outcome != QualityPass && dimension.Outcome != QualityFail && dimension.Outcome != QualityBlocked && dimension.Outcome != QualityNotApplicable {
			return "", fmt.Errorf(" quality review is blocked: invalid outcome for %s", dimension.Dimension)
		}
		if err := validateFindingSeverities(dimension, policyDimensions[dimension.Dimension]); err != nil {
			return "", err
		}
		if err := validateMetricThresholds(dimension, policyDimensions[dimension.Dimension]); err != nil {
			return "", err
		}
		if dimension.Required && dimension.Applicable && dimension.Outcome == QualityBlocked {
			decision = QualityBlocked
		}
		if dimension.Required && dimension.Applicable && dimension.Outcome == QualityFail && decision != QualityBlocked {
			decision = QualityFail
		}
	}
	return decision, nil
}

func validateMetricThresholds(result QualityDimensionResult, policy QualityPolicyDimension) error {
	metrics := make(map[string]QualityMetric, len(result.Metrics))
	for _, metric := range result.Metrics {
		if metric.ID == "" || metrics[metric.ID].ID != "" {
			return fmt.Errorf(" quality review is blocked: malformed metrics for %s", result.Dimension)
		}
		metrics[metric.ID] = metric
	}
	for _, threshold := range policy.Thresholds {
		metric, found := metrics[threshold.MetricID]
		if !found || metric.Unit != threshold.Unit {
			return fmt.Errorf(" quality review is blocked: missing or mismatched metric %s for %s", threshold.MetricID, result.Dimension)
		}
		breached, valid := ThresholdBreached(metric.Value, threshold)
		if !valid {
			return fmt.Errorf(" quality review is blocked: ill-typed metric %s for %s", threshold.MetricID, result.Dimension)
		}
		if breached && result.Outcome != QualityFail {
			return fmt.Errorf(" quality review is blocked: %s reports pass despite a breached metric threshold", result.Dimension)
		}
	}
	return nil
}

func ThresholdBreached(value interface{}, threshold QualityMetricThreshold) (bool, bool) {
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

func validateFindingSeverities(result QualityDimensionResult, policy QualityPolicyDimension) error {
	blockingIndex := SeverityIndex(policy.SeverityScale, policy.BlockingSeverity)
	for _, finding := range result.Findings {
		if finding.ID == "" || finding.Dimension != result.Dimension || finding.Message == "" || (finding.Location == "" && finding.LocationUnavailableReason == "") {
			return fmt.Errorf(" quality review is blocked: malformed finding for %s", result.Dimension)
		}
		severityIndex := SeverityIndex(policy.SeverityScale, finding.Severity)
		if severityIndex < 0 {
			return fmt.Errorf(" quality review is blocked: %s has severity absent from the committed scale", result.Dimension)
		}
		if severityIndex >= blockingIndex && result.Outcome != QualityFail {
			return fmt.Errorf(" quality review is blocked: %s reports pass despite a blocking-severity finding", result.Dimension)
		}
	}
	return nil
}

func SeverityIndex(scale []string, severity string) int {
	for index, candidate := range scale {
		if candidate == severity {
			return index
		}
	}
	return -1
}
