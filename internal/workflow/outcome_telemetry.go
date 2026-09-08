package workflow

import (
	"fmt"
	"regexp"
)

const OutcomeRecordSchemaV1 = "rotta.retained-outcome/v1"

var retainedIdentifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type TelemetryAvailability string

const (
	TelemetryAvailable   TelemetryAvailability = "available"
	TelemetryUnavailable TelemetryAvailability = "unavailable"
)

// SourceMetric keeps absence distinct from the numeric zero observed at source.
type SourceMetric struct {
	Availability TelemetryAvailability `json:"availability"`
	Value        int64                 `json:"value,omitempty"`
}

type OutcomeIdentity struct {
	TaskRequestFingerprint      string `json:"task_request_fingerprint"`
	PolicyContractFingerprint   string `json:"policy_contract_fingerprint"`
	Baseline                    string `json:"baseline"`
	Provider                    string `json:"provider"`
	Model                       string `json:"model"`
	Family                      string `json:"family"`
	IntegrationsFingerprint     string `json:"integrations_fingerprint"`
	PermissionsFingerprint      string `json:"permissions_fingerprint"`
	AcceptanceChecksFingerprint string `json:"acceptance_checks_fingerprint"`
}

type OutcomeTelemetry struct {
	ElapsedMilliseconds SourceMetric `json:"elapsed_milliseconds"`
	Accepted            bool         `json:"accepted"`
}

// CacheMetrics is intentionally separate from outcome telemetry.
type CacheMetrics struct {
	ReadTokens  SourceMetric `json:"read_tokens"`
	WriteTokens SourceMetric `json:"write_tokens"`
}

type OutcomeRecord struct {
	SchemaVersion string           `json:"schema_version"`
	RunID         string           `json:"run_id"`
	RootID        string           `json:"root_id"`
	EvidenceID    string           `json:"evidence_id"`
	Identity      OutcomeIdentity  `json:"identity"`
	Outcome       OutcomeTelemetry `json:"outcome"`
	Cache         CacheMetrics     `json:"cache_metrics"`
}

func (r OutcomeRecord) Validate() error {
	if err := r.validateStructure(); err != nil {
		return err
	}
	for name, metric := range r.requiredMetrics() {
		if metric.Availability != TelemetryAvailable {
			return fmt.Errorf("required telemetry unavailable: %s", name)
		}
	}
	return nil
}

func (r OutcomeRecord) validateStructure() error {
	if r.SchemaVersion != OutcomeRecordSchemaV1 {
		return fmt.Errorf("unsupported outcome record schema %q", r.SchemaVersion)
	}
	for name, value := range map[string]string{"run_id": r.RunID, "root_id": r.RootID, "evidence_id": r.EvidenceID, "task_request_fingerprint": r.Identity.TaskRequestFingerprint, "policy_contract_fingerprint": r.Identity.PolicyContractFingerprint, "baseline": r.Identity.Baseline, "provider": r.Identity.Provider, "model": r.Identity.Model, "family": r.Identity.Family, "integrations_fingerprint": r.Identity.IntegrationsFingerprint, "permissions_fingerprint": r.Identity.PermissionsFingerprint, "acceptance_checks_fingerprint": r.Identity.AcceptanceChecksFingerprint} {
		if value == "" {
			return fmt.Errorf("missing %s", name)
		}
	}
	for name, value := range map[string]string{"run_id": r.RunID, "root_id": r.RootID, "evidence_id": r.EvidenceID} {
		if !retainedIdentifierPattern.MatchString(value) {
			return fmt.Errorf("invalid %s", name)
		}
	}
	for name, metric := range r.requiredMetrics() {
		if metric.Availability != TelemetryAvailable && metric.Availability != TelemetryUnavailable {
			return fmt.Errorf("invalid telemetry availability for %s", name)
		}
		if metric.Availability == TelemetryUnavailable && metric.Value != 0 {
			return fmt.Errorf("unavailable telemetry has value for %s", name)
		}
	}
	return nil
}

func (r OutcomeRecord) requiredMetrics() map[string]SourceMetric {
	return map[string]SourceMetric{"outcome.elapsed_milliseconds": r.Outcome.ElapsedMilliseconds, "cache.read_tokens": r.Cache.ReadTokens, "cache.write_tokens": r.Cache.WriteTokens}
}
