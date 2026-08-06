package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

const (
	// OutcomeTelemetryFormat versions retained per-run telemetry records.
	OutcomeTelemetryFormat = "rotta.workflow-outcome/v1"
	// OutcomeBenchmarkFormat versions deterministic comparator results.
	OutcomeBenchmarkFormat = "rotta.workflow-benchmark/v1"

	REQ091NonCacheTokenLimit uint64 = 1_624_762
	REQ091ChildSessionLimit  uint64 = 25
	REQ091BenchmarkRunCount         = 3
)

// TelemetryMetric records an exposed host metric without turning an unavailable
// value into zero. Source identifies the host field or collection source.
type TelemetryMetric struct {
	Status TelemetryMetricStatus `json:"status"`
	Value  *uint64               `json:"value,omitempty"`
	Source string                `json:"source"`
}

type TelemetryMetricStatus string

const (
	TelemetryObserved      TelemetryMetricStatus = "observed"
	TelemetryNotObservable TelemetryMetricStatus = "not_observable"
)

func ObservedTelemetry(value uint64, source string) TelemetryMetric {
	return TelemetryMetric{Status: TelemetryObserved, Value: &value, Source: source}
}

func NotObservableTelemetry(source string) TelemetryMetric {
	return TelemetryMetric{Status: TelemetryNotObservable, Source: source}
}

// OutcomeRunIdentity contains every material input that must agree before
// REQ-091 efficiency thresholds can be evaluated.
type OutcomeRunIdentity struct {
	FeatureID                 string   `json:"feature_id"`
	FeatureRequestFingerprint string   `json:"feature_request_fingerprint"`
	ContractFingerprint       string   `json:"contract_fingerprint"`
	PolicyFingerprint         string   `json:"policy_fingerprint"`
	RepositoryBaseline        string   `json:"repository_baseline"`
	ProviderIdentifier        string   `json:"provider_identifier"`
	ModelIdentifier           string   `json:"model_identifier"`
	ModelFamily               string   `json:"model_family"`
	EnabledIntegrations       []string `json:"enabled_integrations"`
	OperationalPermissions    []string `json:"operational_permissions"`
	AcceptanceChecks          []string `json:"acceptance_checks"`
}

type OutcomeTokens struct {
	Input     TelemetryMetric `json:"input_tokens"`
	Output    TelemetryMetric `json:"output_tokens"`
	Reasoning TelemetryMetric `json:"reasoning_tokens"`
}

// OutcomeCacheTokens remains separate because cache reads and writes are not
// part of the REQ-091 non-cache token threshold.
type OutcomeCacheTokens struct {
	Read  TelemetryMetric `json:"read_tokens"`
	Write TelemetryMetric `json:"write_tokens"`
}

type OutcomeStatus string

const (
	OutcomePassed        OutcomeStatus = "passed"
	OutcomeFailed        OutcomeStatus = "failed"
	OutcomeNotObservable OutcomeStatus = "not_observable"
)

// DurableEvidenceReference identifies a retained local result. It deliberately
// holds no command output, provider pricing, or provider cost assertion.
type DurableEvidenceReference struct {
	Check  string        `json:"check"`
	Path   string        `json:"path"`
	Hash   string        `json:"hash"`
	Status OutcomeStatus `json:"status"`
}

const OutcomeEvidenceBindingFormat = "rotta.workflow-outcome-evidence/v1"

type outcomeEvidenceKind string

const (
	deterministicValidationEvidence outcomeEvidenceKind = "deterministic_validation"
	independentReviewEvidence       outcomeEvidenceKind = "independent_final_review"
)

// OutcomeEvidenceBinding is the complete JSON value retained in lifecycle
// evidence stdout. Because commandOutputHash covers stdout, it binds a durable
// validation or review result to one telemetry run rather than trusting the
// caller's reference fields.
type OutcomeEvidenceBinding struct {
	Format        string              `json:"format"`
	Kind          outcomeEvidenceKind `json:"kind"`
	RunID         string              `json:"run_id"`
	RootSessionID string              `json:"root_session_id"`
	RunIdentity   OutcomeRunIdentity  `json:"run_identity"`
	Check         string              `json:"check"`
	Status        OutcomeStatus       `json:"status"`
	Independent   bool                `json:"independent"`
}

type DeterministicValidationOutcome struct {
	Status   OutcomeStatus              `json:"status"`
	Evidence []DurableEvidenceReference `json:"evidence"`
}

type IndependentFinalReviewOutcome struct {
	Status      OutcomeStatus              `json:"status"`
	Independent bool                       `json:"independent"`
	Evidence    []DurableEvidenceReference `json:"evidence"`
}

// WorkflowOutcomeRecord is the complete REQ-091 outcome schema. Counts and
// token fields use TelemetryMetric so absent host telemetry is explicit.
type WorkflowOutcomeRecord struct {
	Format                  string                         `json:"format"`
	RunID                   string                         `json:"run_id"`
	RunIdentity             OutcomeRunIdentity             `json:"run_identity"`
	RootSessionID           string                         `json:"root_session_id"`
	ChildSessions           TelemetryMetric                `json:"child_sessions"`
	RoleInvocations         map[string]TelemetryMetric     `json:"role_invocations"`
	Tokens                  OutcomeTokens                  `json:"tokens"`
	CacheTokens             OutcomeCacheTokens             `json:"cache_tokens"`
	HumanDecisions          TelemetryMetric                `json:"human_decisions"`
	Continuations           TelemetryMetric                `json:"continuations"`
	CorrectionCycles        TelemetryMetric                `json:"correction_cycles"`
	DeterministicCommands   TelemetryMetric                `json:"deterministic_commands"`
	DeterministicValidation DeterministicValidationOutcome `json:"deterministic_validation"`
	IndependentFinalReview  IndependentFinalReviewOutcome  `json:"independent_final_review"`
}

// MarshalOutcomeRecord emits only the versioned schema after validating that
// every unavailable metric has an explicit source.
func MarshalOutcomeRecord(record WorkflowOutcomeRecord) ([]byte, error) {
	if err := ValidateOutcomeRecord(record); err != nil {
		return nil, err
	}
	return json.Marshal(record)
}

// UnmarshalOutcomeRecord rejects duplicate fields, case-variant aliases, and
// unknown fields so retained telemetry cannot silently change schema semantics
// between comparator runs.
func UnmarshalOutcomeRecord(data []byte) (WorkflowOutcomeRecord, error) {
	if err := rejectAmbiguousOutcomeJSON(data); err != nil {
		return WorkflowOutcomeRecord{}, fmt.Errorf("decode workflow outcome: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var record WorkflowOutcomeRecord
	if err := decoder.Decode(&record); err != nil {
		return WorkflowOutcomeRecord{}, fmt.Errorf("decode workflow outcome: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return WorkflowOutcomeRecord{}, fmt.Errorf("decode workflow outcome: multiple JSON values")
	}
	if err := ValidateOutcomeRecord(record); err != nil {
		return WorkflowOutcomeRecord{}, err
	}
	return record, nil
}

func ValidateOutcomeRecord(record WorkflowOutcomeRecord) error {
	if record.Format != OutcomeTelemetryFormat {
		return fmt.Errorf("workflow outcome format must be %q", OutcomeTelemetryFormat)
	}
	if strings.TrimSpace(record.RunID) == "" {
		return fmt.Errorf("workflow outcome run ID is required")
	}
	if err := validateOutcomeRunIdentity(record.RunIdentity); err != nil {
		return err
	}
	if strings.TrimSpace(record.RootSessionID) == "" {
		return fmt.Errorf("workflow outcome root session ID is required")
	}
	for _, field := range []struct {
		name   string
		metric TelemetryMetric
	}{
		{"child_sessions", record.ChildSessions},
		{"input_tokens", record.Tokens.Input},
		{"output_tokens", record.Tokens.Output},
		{"reasoning_tokens", record.Tokens.Reasoning},
		{"cache_read_tokens", record.CacheTokens.Read},
		{"cache_write_tokens", record.CacheTokens.Write},
		{"human_decisions", record.HumanDecisions},
		{"continuations", record.Continuations},
		{"correction_cycles", record.CorrectionCycles},
		{"deterministic_commands", record.DeterministicCommands},
	} {
		if err := validateTelemetryMetric(field.name, field.metric); err != nil {
			return err
		}
	}
	if record.RoleInvocations == nil {
		return fmt.Errorf("workflow outcome role invocations are required")
	}
	roles := make([]string, 0, len(record.RoleInvocations))
	for role := range record.RoleInvocations {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	for _, role := range roles {
		metric := record.RoleInvocations[role]
		if strings.TrimSpace(role) == "" {
			return fmt.Errorf("workflow outcome role invocation has an empty role")
		}
		if err := validateTelemetryMetric("role_invocations."+role, metric); err != nil {
			return err
		}
	}
	if err := validateValidationOutcome(record.DeterministicValidation); err != nil {
		return err
	}
	return validateReviewOutcome(record.IndependentFinalReview)
}

func validateOutcomeRunIdentity(identity OutcomeRunIdentity) error {
	for _, field := range []struct {
		name  string
		value string
	}{
		{"feature_id", identity.FeatureID},
		{"feature_request_fingerprint", identity.FeatureRequestFingerprint},
		{"contract_fingerprint", identity.ContractFingerprint},
		{"policy_fingerprint", identity.PolicyFingerprint},
		{"repository_baseline", identity.RepositoryBaseline},
		{"provider_identifier", identity.ProviderIdentifier},
		{"model_identifier", identity.ModelIdentifier},
		{"model_family", identity.ModelFamily},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("workflow outcome run identity %s is required", field.name)
		}
	}
	for _, field := range []struct {
		name   string
		values []string
	}{
		{"enabled_integrations", identity.EnabledIntegrations},
		{"operational_permissions", identity.OperationalPermissions},
		{"acceptance_checks", identity.AcceptanceChecks},
	} {
		if !strictlySortedUnique(field.values) {
			return fmt.Errorf("workflow outcome run identity %s must be a non-nil, strictly sorted unique list", field.name)
		}
	}
	if len(identity.AcceptanceChecks) == 0 {
		return fmt.Errorf("workflow outcome run identity acceptance checks are required")
	}
	return nil
}

func strictlySortedUnique(values []string) bool {
	if values == nil {
		return false
	}
	for index, value := range values {
		if strings.TrimSpace(value) == "" || index > 0 && values[index-1] >= value {
			return false
		}
	}
	return true
}

func validateTelemetryMetric(name string, metric TelemetryMetric) error {
	if strings.TrimSpace(metric.Source) == "" {
		return fmt.Errorf("workflow outcome metric %s is missing its source", name)
	}
	switch metric.Status {
	case TelemetryObserved:
		if metric.Value == nil {
			return fmt.Errorf("workflow outcome metric %s is observed without a value", name)
		}
	case TelemetryNotObservable:
		if metric.Value != nil {
			return fmt.Errorf("workflow outcome metric %s is not_observable with a value", name)
		}
	default:
		return fmt.Errorf("workflow outcome metric %s has invalid status %q", name, metric.Status)
	}
	return nil
}

func validateValidationOutcome(validation DeterministicValidationOutcome) error {
	if !validOutcomeStatus(validation.Status) {
		return fmt.Errorf("workflow outcome deterministic validation has invalid status %q", validation.Status)
	}
	return validateEvidenceReferences("deterministic validation", validation.Evidence)
}

func validateReviewOutcome(review IndependentFinalReviewOutcome) error {
	if !validOutcomeStatus(review.Status) {
		return fmt.Errorf("workflow outcome independent final review has invalid status %q", review.Status)
	}
	return validateEvidenceReferences("independent final review", review.Evidence)
}

func validOutcomeStatus(status OutcomeStatus) bool {
	return status == OutcomePassed || status == OutcomeFailed || status == OutcomeNotObservable
}

func validateEvidenceReferences(name string, evidence []DurableEvidenceReference) error {
	for _, reference := range evidence {
		if strings.TrimSpace(reference.Check) == "" || strings.TrimSpace(reference.Path) == "" || !lowerHexSHA256(reference.Hash) || !validOutcomeStatus(reference.Status) {
			return fmt.Errorf("workflow outcome %s contains an invalid durable evidence reference", name)
		}
	}
	return nil
}

func lowerHexSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !(character >= '0' && character <= '9' || character >= 'a' && character <= 'f') {
			return false
		}
	}
	return true
}

type OutcomeBenchmarkStatus string

const (
	OutcomeBenchmarkPassed       OutcomeBenchmarkStatus = "passed"
	OutcomeBenchmarkFailed       OutcomeBenchmarkStatus = "failed"
	OutcomeBenchmarkNotEvaluable OutcomeBenchmarkStatus = "not_evaluable"
)

type OutcomeBenchmarkCacheReport struct {
	RootSessionID string          `json:"root_session_id"`
	Read          TelemetryMetric `json:"read_tokens"`
	Write         TelemetryMetric `json:"write_tokens"`
}

// OutcomeBenchmarkResult contains thresholds and cache telemetry, but no
// provider pricing or cost claim.
type OutcomeBenchmarkResult struct {
	Format               string                        `json:"format"`
	Status               OutcomeBenchmarkStatus        `json:"status"`
	Reasons              []string                      `json:"reasons"`
	MedianNonCacheTokens *uint64                       `json:"median_non_cache_tokens,omitempty"`
	NonCacheTokenLimit   uint64                        `json:"non_cache_token_limit"`
	ChildSessionLimit    uint64                        `json:"child_session_limit"`
	CacheTokens          []OutcomeBenchmarkCacheReport `json:"cache_tokens"`
}

// CompareOutcomeBenchmark deterministically rejects malformed, non-equivalent,
// or telemetry-incomplete runs before evaluating REQ-091 thresholds.
func CompareOutcomeBenchmark(runs []WorkflowOutcomeRecord) OutcomeBenchmarkResult {
	return CompareOutcomeBenchmarkInWorktree(".", runs)
}

// CompareOutcomeBenchmarkInWorktree compares retained telemetry only after
// every passing validation/review reference has been resolved beneath the
// supplied worktree and verified against its durable output.
func CompareOutcomeBenchmarkInWorktree(worktree string, runs []WorkflowOutcomeRecord) OutcomeBenchmarkResult {
	result := OutcomeBenchmarkResult{
		Format:             OutcomeBenchmarkFormat,
		Status:             OutcomeBenchmarkNotEvaluable,
		NonCacheTokenLimit: REQ091NonCacheTokenLimit,
		ChildSessionLimit:  REQ091ChildSessionLimit,
		CacheTokens:        cacheReports(runs),
	}
	if len(runs) != REQ091BenchmarkRunCount {
		result.Reasons = []string{fmt.Sprintf("REQ-091 requires exactly %d retained runs", REQ091BenchmarkRunCount)}
		return result
	}
	for index, run := range runs {
		if err := ValidateOutcomeRecord(run); err != nil {
			result.Reasons = append(result.Reasons, fmt.Sprintf("run %d is invalid: %v", index+1, err))
		}
	}
	if len(result.Reasons) != 0 {
		return result
	}
	result.Reasons = append(result.Reasons, distinctRunReasons(runs)...)
	result.Reasons = append(result.Reasons, durableEvidenceReasons(worktree, runs)...)
	if len(result.Reasons) != 0 {
		return result
	}
	result.Reasons = append(result.Reasons, nonEquivalentReasons(runs)...)
	result.Reasons = append(result.Reasons, unavailableRequiredMetricReasons(runs)...)
	if len(result.Reasons) != 0 {
		return result
	}

	nonCache := make([]uint64, 0, len(runs))
	childrenWithinLimit := true
	for _, run := range runs {
		value, overflow := nonCacheTokens(run.Tokens)
		if overflow {
			result.Reasons = []string{"non-cache token total overflows uint64"}
			return result
		}
		nonCache = append(nonCache, value)
		if *run.ChildSessions.Value > REQ091ChildSessionLimit {
			childrenWithinLimit = false
		}
	}
	sort.Slice(nonCache, func(left, right int) bool { return nonCache[left] < nonCache[right] })
	median := nonCache[len(nonCache)/2]
	result.MedianNonCacheTokens = &median

	qualityReasons := qualityFailureReasons(runs)
	if median > REQ091NonCacheTokenLimit {
		result.Reasons = append(result.Reasons, fmt.Sprintf("median non-cache tokens %d exceeds %d", median, REQ091NonCacheTokenLimit))
	}
	if !childrenWithinLimit {
		result.Reasons = append(result.Reasons, fmt.Sprintf("one or more runs exceed %d child sessions", REQ091ChildSessionLimit))
	}
	result.Reasons = append(result.Reasons, qualityReasons...)
	if len(result.Reasons) == 0 {
		result.Status = OutcomeBenchmarkPassed
		return result
	}
	result.Status = OutcomeBenchmarkFailed
	return result
}

func distinctRunReasons(runs []WorkflowOutcomeRecord) []string {
	rootSessions := make(map[string]bool, len(runs))
	runIDs := make(map[string]bool, len(runs))
	evidencePaths := make(map[string]bool)
	var reasons []string
	for index, run := range runs {
		if rootSessions[run.RootSessionID] {
			reasons = append(reasons, fmt.Sprintf("run %d duplicates a retained root session ID", index+1))
		}
		rootSessions[run.RootSessionID] = true
		if runIDs[run.RunID] {
			reasons = append(reasons, fmt.Sprintf("run %d duplicates a retained run ID", index+1))
		}
		runIDs[run.RunID] = true
		for _, reference := range append(append([]DurableEvidenceReference(nil), run.DeterministicValidation.Evidence...), run.IndependentFinalReview.Evidence...) {
			if evidencePaths[reference.Path] {
				reasons = append(reasons, fmt.Sprintf("run %d reuses durable evidence %q", index+1, reference.Path))
			}
			evidencePaths[reference.Path] = true
		}
	}
	return reasons
}

func durableEvidenceReasons(worktree string, runs []WorkflowOutcomeRecord) []string {
	var reasons []string
	for index, run := range runs {
		for _, reference := range run.DeterministicValidation.Evidence {
			if err := validateBoundOutcomeEvidence(worktree, run, deterministicValidationEvidence, reference); err != nil {
				reasons = append(reasons, fmt.Sprintf("run %d deterministic validation evidence is invalid: %v", index+1, err))
			}
		}
		for _, reference := range run.IndependentFinalReview.Evidence {
			if err := validateBoundOutcomeEvidence(worktree, run, independentReviewEvidence, reference); err != nil {
				reasons = append(reasons, fmt.Sprintf("run %d independent final review evidence is invalid: %v", index+1, err))
			}
		}
	}
	return reasons
}

func validateBoundOutcomeEvidence(worktree string, run WorkflowOutcomeRecord, kind outcomeEvidenceKind, reference DurableEvidenceReference) error {
	evidence, err := loadWorkflowEvidence(worktree, reference.Path, reference.Hash)
	if err != nil {
		return err
	}
	if reference.Status == OutcomePassed && (evidence.ExitStatus != 0 || evidence.TimedOut) {
		return fmt.Errorf("passing evidence has non-passing command result")
	}
	binding, err := unmarshalOutcomeEvidenceBinding([]byte(evidence.Stdout))
	if err != nil {
		return fmt.Errorf("decode bound evidence: %w", err)
	}
	if binding.Kind != kind || binding.RunID != run.RunID || binding.RootSessionID != run.RootSessionID || binding.Check != reference.Check || binding.Status != reference.Status || len(runIdentityDifferences(binding.RunIdentity, run.RunIdentity)) != 0 {
		return fmt.Errorf("bound evidence belongs to another run, check, or identity")
	}
	if (kind == independentReviewEvidence) != binding.Independent {
		return fmt.Errorf("bound evidence has incorrect independent-review state")
	}
	return nil
}

func unmarshalOutcomeEvidenceBinding(data []byte) (OutcomeEvidenceBinding, error) {
	if err := rejectAmbiguousJSON(data, outcomeEvidenceBindingShape()); err != nil {
		return OutcomeEvidenceBinding{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var binding OutcomeEvidenceBinding
	if err := decoder.Decode(&binding); err != nil {
		return OutcomeEvidenceBinding{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return OutcomeEvidenceBinding{}, fmt.Errorf("multiple JSON values")
	}
	if binding.Format != OutcomeEvidenceBindingFormat || (binding.Kind != deterministicValidationEvidence && binding.Kind != independentReviewEvidence) || strings.TrimSpace(binding.RunID) == "" || strings.TrimSpace(binding.RootSessionID) == "" || strings.TrimSpace(binding.Check) == "" || !validOutcomeStatus(binding.Status) {
		return OutcomeEvidenceBinding{}, fmt.Errorf("invalid outcome evidence binding")
	}
	if err := validateOutcomeRunIdentity(binding.RunIdentity); err != nil {
		return OutcomeEvidenceBinding{}, err
	}
	return binding, nil
}

func cacheReports(runs []WorkflowOutcomeRecord) []OutcomeBenchmarkCacheReport {
	reports := make([]OutcomeBenchmarkCacheReport, 0, len(runs))
	for _, run := range runs {
		reports = append(reports, OutcomeBenchmarkCacheReport{RootSessionID: run.RootSessionID, Read: run.CacheTokens.Read, Write: run.CacheTokens.Write})
	}
	return reports
}

func nonEquivalentReasons(runs []WorkflowOutcomeRecord) []string {
	baseline := runs[0].RunIdentity
	var reasons []string
	for index, run := range runs[1:] {
		for _, difference := range runIdentityDifferences(baseline, run.RunIdentity) {
			reasons = append(reasons, fmt.Sprintf("run %d is not equivalent: %s differs", index+2, difference))
		}
	}
	return reasons
}

func runIdentityDifferences(left, right OutcomeRunIdentity) []string {
	var differences []string
	for _, field := range []struct {
		name  string
		left  string
		right string
	}{
		{"feature_id", left.FeatureID, right.FeatureID},
		{"feature_request_fingerprint", left.FeatureRequestFingerprint, right.FeatureRequestFingerprint},
		{"contract_fingerprint", left.ContractFingerprint, right.ContractFingerprint},
		{"policy_fingerprint", left.PolicyFingerprint, right.PolicyFingerprint},
		{"repository_baseline", left.RepositoryBaseline, right.RepositoryBaseline},
		{"provider_identifier", left.ProviderIdentifier, right.ProviderIdentifier},
		{"model_identifier", left.ModelIdentifier, right.ModelIdentifier},
		{"model_family", left.ModelFamily, right.ModelFamily},
	} {
		if field.left != field.right {
			differences = append(differences, field.name)
		}
	}
	for _, field := range []struct {
		name  string
		left  []string
		right []string
	}{
		{"enabled_integrations", left.EnabledIntegrations, right.EnabledIntegrations},
		{"operational_permissions", left.OperationalPermissions, right.OperationalPermissions},
		{"acceptance_checks", left.AcceptanceChecks, right.AcceptanceChecks},
	} {
		if !sameStrings(field.left, field.right) {
			differences = append(differences, field.name)
		}
	}
	return differences
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func unavailableRequiredMetricReasons(runs []WorkflowOutcomeRecord) []string {
	type metricSet struct {
		name   string
		values []TelemetryMetric
	}
	sets := []metricSet{
		{"child_sessions", metricsForRuns(runs, func(run WorkflowOutcomeRecord) TelemetryMetric { return run.ChildSessions })},
		{"input_tokens", metricsForRuns(runs, func(run WorkflowOutcomeRecord) TelemetryMetric { return run.Tokens.Input })},
		{"output_tokens", metricsForRuns(runs, func(run WorkflowOutcomeRecord) TelemetryMetric { return run.Tokens.Output })},
		{"reasoning_tokens", metricsForRuns(runs, func(run WorkflowOutcomeRecord) TelemetryMetric { return run.Tokens.Reasoning })},
	}
	var reasons []string
	for _, set := range sets {
		available := set.values[0].Status == TelemetryObserved
		for index, value := range set.values[1:] {
			if (value.Status == TelemetryObserved) != available {
				reasons = append(reasons, fmt.Sprintf("required metric availability differs for %s at run %d", set.name, index+2))
			}
		}
		if !available {
			reasons = append(reasons, fmt.Sprintf("required metric %s is not_observable", set.name))
		}
	}
	return reasons
}

func metricsForRuns(runs []WorkflowOutcomeRecord, metric func(WorkflowOutcomeRecord) TelemetryMetric) []TelemetryMetric {
	values := make([]TelemetryMetric, 0, len(runs))
	for _, run := range runs {
		values = append(values, metric(run))
	}
	return values
}

func nonCacheTokens(tokens OutcomeTokens) (uint64, bool) {
	first := *tokens.Input.Value + *tokens.Output.Value
	if first < *tokens.Input.Value {
		return 0, true
	}
	total := first + *tokens.Reasoning.Value
	return total, total < first
}

func qualityFailureReasons(runs []WorkflowOutcomeRecord) []string {
	var reasons []string
	for index, run := range runs {
		if run.DeterministicCommands.Status != TelemetryObserved || *run.DeterministicCommands.Value == 0 {
			reasons = append(reasons, fmt.Sprintf("run %d lacks an observed deterministic command", index+1))
		}
		if !passingValidation(run.DeterministicValidation, run.RunIdentity.AcceptanceChecks) {
			reasons = append(reasons, fmt.Sprintf("run %d lacks passing durable deterministic validation for applicable checks", index+1))
		}
		if !passingIndependentReview(run.IndependentFinalReview) {
			reasons = append(reasons, fmt.Sprintf("run %d lacks a passing independent final review with durable evidence", index+1))
		}
	}
	return reasons
}

func passingValidation(validation DeterministicValidationOutcome, checks []string) bool {
	if validation.Status != OutcomePassed || len(validation.Evidence) == 0 {
		return false
	}
	passingChecks := make(map[string]bool, len(validation.Evidence))
	for _, evidence := range validation.Evidence {
		if evidence.Status == OutcomePassed {
			passingChecks[evidence.Check] = true
		}
	}
	for _, check := range checks {
		if !passingChecks[check] {
			return false
		}
	}
	return true
}

func passingIndependentReview(review IndependentFinalReviewOutcome) bool {
	if review.Status != OutcomePassed || !review.Independent || len(review.Evidence) == 0 {
		return false
	}
	for _, evidence := range review.Evidence {
		if evidence.Status != OutcomePassed {
			return false
		}
	}
	return true
}

type outcomeJSONShape struct {
	fields     map[string]outcomeJSONShape
	element    *outcomeJSONShape
	additional *outcomeJSONShape
}

func rejectAmbiguousOutcomeJSON(data []byte) error {
	return rejectAmbiguousJSON(data, workflowOutcomeRecordShape())
}

func rejectAmbiguousJSON(data []byte, shape outcomeJSONShape) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := scanOutcomeJSONValue(decoder, shape); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return fmt.Errorf("multiple JSON values")
	}
	return nil
}

func scanOutcomeJSONValue(decoder *json.Decoder, shape outcomeJSONShape) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	switch delimiter := token.(type) {
	case json.Delim:
		switch delimiter {
		case '{':
			seen := make(map[string]bool)
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := keyToken.(string)
				if !ok {
					return fmt.Errorf("object key is not a string")
				}
				if seen[key] {
					return fmt.Errorf("duplicate JSON field %q", key)
				}
				seen[key] = true
				child, known := shape.fields[key]
				if !known {
					for canonical := range shape.fields {
						if strings.EqualFold(key, canonical) {
							return fmt.Errorf("ambiguous JSON field alias %q for %q", key, canonical)
						}
					}
					if shape.additional != nil {
						child = *shape.additional
					}
				}
				if err := scanOutcomeJSONValue(decoder, child); err != nil {
					return err
				}
			}
			end, err := decoder.Token()
			if err != nil || end != json.Delim('}') {
				return fmt.Errorf("malformed JSON object")
			}
		case '[':
			for decoder.More() {
				child := outcomeJSONShape{}
				if shape.element != nil {
					child = *shape.element
				}
				if err := scanOutcomeJSONValue(decoder, child); err != nil {
					return err
				}
			}
			end, err := decoder.Token()
			if err != nil || end != json.Delim(']') {
				return fmt.Errorf("malformed JSON array")
			}
		default:
			return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
		}
	}
	return nil
}

func workflowOutcomeRecordShape() outcomeJSONShape {
	metric := outcomeJSONShape{fields: map[string]outcomeJSONShape{"status": {}, "value": {}, "source": {}}}
	identity := outcomeJSONShape{fields: map[string]outcomeJSONShape{
		"feature_id": {}, "feature_request_fingerprint": {}, "contract_fingerprint": {}, "policy_fingerprint": {}, "repository_baseline": {}, "provider_identifier": {}, "model_identifier": {}, "model_family": {},
		"enabled_integrations": {element: &outcomeJSONShape{}}, "operational_permissions": {element: &outcomeJSONShape{}}, "acceptance_checks": {element: &outcomeJSONShape{}},
	}}
	evidence := outcomeJSONShape{fields: map[string]outcomeJSONShape{"check": {}, "path": {}, "hash": {}, "status": {}}}
	validation := outcomeJSONShape{fields: map[string]outcomeJSONShape{"status": {}, "evidence": {element: &evidence}}}
	review := outcomeJSONShape{fields: map[string]outcomeJSONShape{"status": {}, "independent": {}, "evidence": {element: &evidence}}}
	roles := outcomeJSONShape{additional: &metric}
	return outcomeJSONShape{fields: map[string]outcomeJSONShape{
		"format": {}, "run_id": {}, "run_identity": identity, "root_session_id": {}, "child_sessions": metric, "role_invocations": roles,
		"tokens":          {fields: map[string]outcomeJSONShape{"input_tokens": metric, "output_tokens": metric, "reasoning_tokens": metric}},
		"cache_tokens":    {fields: map[string]outcomeJSONShape{"read_tokens": metric, "write_tokens": metric}},
		"human_decisions": metric, "continuations": metric, "correction_cycles": metric, "deterministic_commands": metric,
		"deterministic_validation": validation, "independent_final_review": review,
	}}
}

func outcomeEvidenceBindingShape() outcomeJSONShape {
	identity := workflowOutcomeRecordShape().fields["run_identity"]
	return outcomeJSONShape{fields: map[string]outcomeJSONShape{
		"format": {}, "kind": {}, "run_id": {}, "root_session_id": {}, "run_identity": identity, "check": {}, "status": {}, "independent": {},
	}}
}
