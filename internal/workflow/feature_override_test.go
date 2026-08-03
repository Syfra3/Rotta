package workflow

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// REQ-084 → SCN-611 → TestSCN611_ScopedOverrideIsConsumedOnceAndRetainedAsEvidence
func TestSCN611_ScopedOverrideIsConsumedOnceAndRetainedAsEvidence(t *testing.T) {
	// Scenario: A valid scoped override is consumed once and remains auditable
	repo := t.TempDir()
	const (
		featureID           = "workflow-ergonomics"
		ruleID              = "relevant-package-tests"
		operation           = "review"
		baseline            = "baseline-sha"
		contractFingerprint = "contract-fingerprint"
	)
	gateOutcomePath := ".rotta/current/evidence/relevant-package-tests.yaml"
	if err := os.MkdirAll(filepath.Join(repo, ".rotta", "current", "evidence"), 0o700); err != nil {
		t.Fatalf("create gate evidence directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, gateOutcomePath), []byte("feature_id: workflow-ergonomics\nrule_id: relevant-package-tests\nbaseline: baseline-sha\ncontract_fingerprint: contract-fingerprint\nstatus: failed\n"), 0o600); err != nil {
		t.Fatalf("write persisted non-passing gate outcome: %v", err)
	}

	for _, testCase := range []struct {
		name            string
		target          OverrideTarget
		targetEvidence  string
		targetReference string
	}{
		{
			name:            "persisted non-passing gate outcome",
			target:          OverrideTarget{PersistedGateOutcomePath: gateOutcomePath},
			targetEvidence:  "persisted_gate_outcome",
			targetReference: gateOutcomePath,
		},
		{
			name:            "eligible process rule",
			target:          OverrideTarget{EligibleProcessRule: true},
			targetEvidence:  "eligible_process_rule",
			targetReference: ruleID,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			display := NewDisplayedOverrideAction(DisplayedOverrideActionInput{
				AuthorizationActionID: "override-prompt-1",
				FeatureID:             featureID,
				RuleID:                ruleID,
				Operation:             operation,
				Baseline:              baseline,
				ContractFingerprint:   contractFingerprint,
				Reason:                "preserve the blocked handoff for human review",
				ExpiresAt:             time.Date(2026, time.August, 4, 12, 0, 0, 0, time.UTC),
				Target:                testCase.target,
			})

			override, err := display.Authorize(repo, "approve")
			if err != nil {
				t.Fatalf("Authorize() returned error: %v", err)
			}
			if override.UsesRemaining != 1 || override.Status != "active" {
				t.Fatalf("override after authorization = %#v, want one active use", override)
			}
			if !strings.HasPrefix(override.Path, filepath.Join(repo, ".rotta", "current", "overrides")+string(filepath.Separator)) {
				t.Fatalf("override path = %q, want feature-local overrides directory", override.Path)
			}
			for _, identityField := range []string{"Actor", "ActorID", "Reviewer", "HumanIdentity"} {
				if _, found := reflect.TypeOf(override).FieldByName(identityField); found {
					t.Fatalf("override records forbidden human identity field %q", identityField)
				}
			}

			applied, err := ApplyFeatureLocalOverride(override.Path, operation)
			if err != nil || !applied {
				t.Fatalf("ApplyFeatureLocalOverride() = (%t, %v), want (true, nil)", applied, err)
			}
			contents, err := os.ReadFile(override.Path)
			if err != nil {
				t.Fatalf("read consumed override evidence: %v", err)
			}
			for _, want := range []string{
				"format: rotta.feature-override/v1",
				"authorization_action_id: override-prompt-1",
				"feature_id: " + featureID,
				"rule_id: " + ruleID,
				"operation: " + operation,
				"baseline: " + baseline,
				"contract_fingerprint: " + contractFingerprint,
				"reason: preserve the blocked handoff for human review",
				"expires_at: 2026-08-04T12:00:00Z",
				"target: " + testCase.targetEvidence,
				"target_reference: " + testCase.targetReference,
				"uses_remaining: 0",
				"status: consumed",
			} {
				if !strings.Contains(string(contents), want) {
					t.Errorf("consumed override evidence missing %q:\n%s", want, contents)
				}
			}
			if applied, err := ApplyFeatureLocalOverride(override.Path, "archive"); err != nil || applied {
				t.Fatalf("ApplyFeatureLocalOverride() for another operation = (%t, %v), want (false, nil)", applied, err)
			}
		})
	}
}
