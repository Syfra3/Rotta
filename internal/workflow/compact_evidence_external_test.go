package workflow_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/Syfra3/Rotta/internal/workflow"
)

func TestCompactCapsuleExternalAPIRejectsRawConstructionAndPreservesValidatedJSON(t *testing.T) {
	capsuleType := reflect.TypeFor[workflow.CompactEvidenceResult]()
	if capsuleType.Kind() != reflect.Interface {
		t.Fatalf("CompactEvidenceResult kind = %s, want sealed interface", capsuleType.Kind())
	}
	if _, exists := reflect.TypeFor[workflow.WorkflowCommandResult]().FieldByName("Capsule"); exists {
		t.Fatal("WorkflowCommandResult exposes a public capsule attachment path")
	}

	raw := `{"risk":"api_key=secret-value","canonical_outcome":{"diagnostics":["time=2026-08-06 level=error msg=raw command output"]}}`
	var decoded workflow.CompactEvidenceResult
	if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
		t.Fatal("decoded arbitrary raw capsule was accepted by the sealed interface")
	}
	var decodedResult workflow.WorkflowCommandResult
	if err := json.Unmarshal([]byte(`{"capsule":`+raw+`}`), &decodedResult); err != nil {
		t.Fatalf("decode workflow result with raw capsule: %v", err)
	}
	decodedResultJSON, err := json.Marshal(decodedResult)
	if err != nil {
		t.Fatal(err)
	}
	var decodedResultView map[string]json.RawMessage
	if err := json.Unmarshal(decodedResultJSON, &decodedResultView); err != nil {
		t.Fatal(err)
	}
	if _, exists := decodedResultView["capsule"]; exists {
		t.Fatalf("decoded raw data attached an agent-facing capsule: %s", decodedResultJSON)
	}

	rawResult, err := json.Marshal(workflow.WorkflowCommandResult{
		Diagnostics: []string{"api_key=secret-value", "raw command output"},
		Remediation: "token=secret-value",
	})
	if err != nil {
		t.Fatal(err)
	}
	var rawResultView map[string]json.RawMessage
	if err := json.Unmarshal(rawResult, &rawResultView); err != nil {
		t.Fatal(err)
	}
	if _, exists := rawResultView["capsule"]; exists {
		t.Fatalf("arbitrary command result serialized an agent-facing capsule: %s", rawResult)
	}

	capsule, err := workflow.NewCompactEvidenceResult(externalCompactEvidenceInput())
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(capsule)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "secret-value") || strings.Contains(string(encoded), "raw command output") {
		t.Fatalf("validated compact capsule emitted raw data: %s", encoded)
	}
	var view struct {
		Format   string `json:"format"`
		Evidence struct {
			Path string `json:"path"`
			Hash string `json:"hash"`
		} `json:"evidence"`
	}
	if err := json.Unmarshal(encoded, &view); err != nil {
		t.Fatal(err)
	}
	if view.Format != "rotta.compact-evidence/v1" || view.Evidence.Path == "" || len(view.Evidence.Hash) != 64 {
		t.Fatalf("validated compact capsule is not externally usable: %#v", view)
	}
}

func externalCompactEvidenceInput() workflow.CompactEvidenceInput {
	return workflow.CompactEvidenceInput{
		CanonicalOutcome: workflow.WorkflowCommandResult{
			Format:       workflow.WorkflowCommandFormat,
			Command:      "scoped-verify",
			Status:       "failed",
			EvidencePath: ".rotta/current/evidence/check.json",
			EvidenceHash: strings.Repeat("a", 64),
			CanonicalInputs: workflow.WorkflowCommandInputs{
				Worktree:     ".",
				Feature:      "slice-four",
				ContractPath: "specs/contract.md",
				Baseline:     strings.Repeat("b", 40),
				Scope:        []string{"internal/workflow"},
			},
			Remediation: "correct the in-scope failure and rerun scoped verification",
		},
		Evidence: workflow.DurableEvidenceReference{
			Check: "scoped-verify", Path: ".rotta/current/evidence/check.json", Hash: strings.Repeat("a", 64), Status: workflow.OutcomeFailed,
		},
		ChangedPaths: []string{"internal/workflow/command_evidence.go"},
		Scope:        []string{"internal/workflow"},
		Risk:         "durable_evidence_authoritative",
		Remediation:  "correct the in-scope failure and rerun scoped verification",
	}
}
