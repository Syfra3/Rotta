package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Syfra3/Rotta/internal/workflow"
)

func TestHarnessReliabilityWorkflowBenchmarkCommandEmitsMachineReadablePaths(t *testing.T) {
	worktree, input := t.TempDir(), t.TempDir()
	for i, ids := range [][3]string{{"run-1", "root-1", "evidence-1"}, {"run-2", "root-2", "evidence-2"}, {"run-3", "root-3", "evidence-3"}} {
		record := commandRecord(ids[0], ids[1], ids[2])
		data, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(input, string(rune('a'+i))+".json"), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var stdout bytes.Buffer
	if err := runCLI([]string{"workflow", "benchmark", "--input", input, "--worktree", worktree, "--id", "fixture-v1"}, &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("run benchmark command: %v", err)
	}
	var result workflow.BenchmarkResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}
	if result.Status != workflow.BenchmarkPassed || len(result.CanonicalInputPaths) != 3 || result.CanonicalResultPath == "" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func commandRecord(runID, rootID, evidenceID string) workflow.OutcomeRecord {
	return workflow.OutcomeRecord{SchemaVersion: workflow.OutcomeRecordSchemaV1, RunID: runID, RootID: rootID, EvidenceID: evidenceID, Identity: workflow.OutcomeIdentity{TaskRequestFingerprint: "task", PolicyContractFingerprint: "policy", Baseline: "baseline", Provider: "provider", Model: "model", Family: "family", IntegrationsFingerprint: "integrations", PermissionsFingerprint: "permissions", AcceptanceChecksFingerprint: "checks"}, Outcome: workflow.OutcomeTelemetry{ElapsedMilliseconds: workflow.SourceMetric{Availability: workflow.TelemetryAvailable, Value: 120}, Accepted: true}, Cache: workflow.CacheMetrics{ReadTokens: workflow.SourceMetric{Availability: workflow.TelemetryAvailable, Value: 10}, WriteTokens: workflow.SourceMetric{Availability: workflow.TelemetryAvailable, Value: 5}}}
}
