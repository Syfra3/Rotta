package workflow

import "testing"

func TestHarnessReliabilityOutcomeRecordValidationRejectsUnavailableAsZero(t *testing.T) {
	record := validOutcomeRecord("run-1", "root-1", "evidence-1")
	record.Outcome.ElapsedMilliseconds = SourceMetric{Availability: TelemetryUnavailable}
	if err := record.Validate(); err == nil {
		t.Fatal("unavailable required telemetry must not validate as zero")
	}
}

func TestHarnessReliabilityOutcomeRecordValidationSeparatesCacheMetrics(t *testing.T) {
	record := validOutcomeRecord("run-1", "root-1", "evidence-1")
	if err := record.Validate(); err != nil {
		t.Fatalf("validate record: %v", err)
	}
	if record.Cache.ReadTokens.Value == record.Outcome.ElapsedMilliseconds.Value {
		t.Fatal("test fixture must keep cache and outcome metrics distinct")
	}
}

func validOutcomeRecord(runID, rootID, evidenceID string) OutcomeRecord {
	return OutcomeRecord{
		SchemaVersion: OutcomeRecordSchemaV1,
		RunID:         runID, RootID: rootID, EvidenceID: evidenceID,
		Identity: OutcomeIdentity{TaskRequestFingerprint: "task", PolicyContractFingerprint: "policy", Baseline: "baseline", Provider: "provider", Model: "model", Family: "family", IntegrationsFingerprint: "integrations", PermissionsFingerprint: "permissions", AcceptanceChecksFingerprint: "checks"},
		Outcome:  OutcomeTelemetry{ElapsedMilliseconds: SourceMetric{Availability: TelemetryAvailable, Value: 120}, Accepted: true},
		Cache:    CacheMetrics{ReadTokens: SourceMetric{Availability: TelemetryAvailable, Value: 10}, WriteTokens: SourceMetric{Availability: TelemetryAvailable, Value: 5}},
	}
}
