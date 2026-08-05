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
	if err := os.WriteFile(filepath.Join(repo, ".rotta", "quality-gates.yaml"), []byte("format: rotta.quality-gates/v1\n"), 0o600); err != nil {
		t.Fatalf("write quality-gates interface policy: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".rotta", "current", "review-evidence.yaml"), []byte("provided by quality-gates evaluator\n"), 0o600); err != nil {
		t.Fatalf("write quality-gates interface review evidence: %v", err)
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

// REQ-084 → SCN-612 → TestSCN612_InvalidOverrideIsRejectedOnDriftOrExpiry
func TestSCN612_InvalidOverrideIsRejectedOnDriftOrExpiry(t *testing.T) {
	// Scenario: An invalid override is rejected on drift or expiry
	const (
		featureID           = "workflow-ergonomics"
		ruleID              = "relevant-package-tests"
		operation           = "review"
		baseline            = "baseline-sha"
		contractFingerprint = "contract-fingerprint"
		policyFingerprint   = "policy-fingerprint"
		evidenceFingerprint = "evidence-fingerprint"
		scope               = "review-handoff"
	)
	now := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)

	for _, testCase := range []struct {
		name          string
		mutate        func(*FeatureLocalOverride)
		removeExpiry  bool
		addCompetitor bool
	}{
		{name: "expired", mutate: func(override *FeatureLocalOverride) { override.ExpiresAt = now.Add(-time.Hour) }},
		{name: "already consumed", mutate: func(override *FeatureLocalOverride) { override.UsesRemaining, override.Status = 0, "consumed" }},
		{name: "missing reason", mutate: func(override *FeatureLocalOverride) { override.Reason = "" }},
		{name: "missing expiry", removeExpiry: true},
		{name: "baseline drift", mutate: func(override *FeatureLocalOverride) { override.Baseline = "other-baseline" }},
		{name: "contract drift", mutate: func(override *FeatureLocalOverride) { override.ContractFingerprint = "other-contract" }},
		{name: "policy drift", mutate: func(override *FeatureLocalOverride) { override.PolicyFingerprint = "other-policy" }},
		{name: "evidence drift", mutate: func(override *FeatureLocalOverride) { override.EvidenceFingerprint = "other-evidence" }},
		{name: "scope mismatch", mutate: func(override *FeatureLocalOverride) { override.Scope = "other-scope" }},
		{name: "competing override", addCompetitor: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repo := t.TempDir()
			gateOutcomePath := filepath.Join(repo, ".rotta", "current", "evidence", "relevant-package-tests.yaml")
			if err := os.MkdirAll(filepath.Dir(gateOutcomePath), 0o700); err != nil {
				t.Fatalf("create gate evidence directory: %v", err)
			}
			const gateOutcome = "status: failed\n"
			if err := os.WriteFile(gateOutcomePath, []byte(gateOutcome), 0o600); err != nil {
				t.Fatalf("write failed gate outcome: %v", err)
			}

			override := FeatureLocalOverride{
				Path:                  filepath.Join(repo, ".rotta", "current", "overrides", "override-1.yaml"),
				AuthorizationActionID: "override-prompt-1",
				FeatureID:             featureID,
				RuleID:                ruleID,
				Operation:             operation,
				Baseline:              baseline,
				ContractFingerprint:   contractFingerprint,
				PolicyFingerprint:     policyFingerprint,
				EvidenceFingerprint:   evidenceFingerprint,
				Scope:                 scope,
				Reason:                "preserve the blocked handoff for human review",
				ExpiresAt:             now.Add(time.Hour),
				Target:                "persisted_gate_outcome",
				TargetReference:       ".rotta/current/evidence/relevant-package-tests.yaml",
				UsesRemaining:         1,
				Status:                "active",
			}
			if testCase.mutate != nil {
				testCase.mutate(&override)
			}
			if err := writeFeatureLocalOverride(override); err != nil {
				t.Fatalf("write override: %v", err)
			}
			if testCase.removeExpiry {
				contents, err := os.ReadFile(override.Path)
				if err != nil {
					t.Fatalf("read override before removing expiry: %v", err)
				}
				if err := os.WriteFile(override.Path, []byte(strings.Replace(string(contents), "expires_at: "+override.ExpiresAt.UTC().Format(time.RFC3339)+"\n", "", 1)), 0o600); err != nil {
					t.Fatalf("remove override expiry: %v", err)
				}
			}

			context := OverrideEvaluationContext{
				Operation:           operation,
				Baseline:            baseline,
				ContractFingerprint: contractFingerprint,
				PolicyFingerprint:   policyFingerprint,
				EvidenceFingerprint: evidenceFingerprint,
				Scope:               scope,
				Now:                 now,
			}
			if testCase.addCompetitor {
				competitor := override
				competitor.Path = filepath.Join(repo, ".rotta", "current", "overrides", "override-2.yaml")
				if err := writeFeatureLocalOverride(competitor); err != nil {
					t.Fatalf("write competing override: %v", err)
				}
			}

			result, err := EvaluateFeatureLocalOverride(override.Path, context)
			if err != nil {
				t.Fatalf("EvaluateFeatureLocalOverride() returned error: %v", err)
			}
			if result.Applied {
				t.Fatal("EvaluateFeatureLocalOverride() applied an invalid override")
			}
			if !strings.HasPrefix(result.Remediation, "remediation:") {
				t.Fatalf("remediation = %q, want actionable remediation", result.Remediation)
			}
			contents, err := os.ReadFile(gateOutcomePath)
			if err != nil {
				t.Fatalf("read gate outcome after rejection: %v", err)
			}
			if string(contents) != gateOutcome {
				t.Fatalf("gate outcome after rejected override = %q, want unchanged %q", contents, gateOutcome)
			}
		})
	}
}

// REQ-084 → SCN-613 → TestSCN613_NonWaivableIntegrityFailureProvidesSafeRecovery
func TestSCN613_NonWaivableIntegrityFailureProvidesSafeRecovery(t *testing.T) {
	// Scenario: Non-waivable integrity failure provides safe recovery
	repo := t.TempDir()
	paths := []string{
		".rotta/current/manifest.yaml",
		"specs/approvals/workflow-ergonomics.yaml",
		".rotta/current/evidence/blocked-operation.yaml",
	}
	for _, path := range paths {
		fullPath := filepath.Join(repo, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o700); err != nil {
			t.Fatalf("create preserved path parent %q: %v", path, err)
		}
		if err := os.WriteFile(fullPath, []byte("preserve\n"), 0o600); err != nil {
			t.Fatalf("write preserved path %q: %v", path, err)
		}
	}

	for _, testCase := range []struct {
		name      string
		safeguard NonWaivableIntegritySafeguard
		invariant string
		recovery  string
	}{
		{
			name:      "malformed or inconsistent manifest or approval authority",
			safeguard: NonWaivableManifestOrApprovalAuthority,
			invariant: "manifest or approval authority",
			recovery:  "repair",
		},
		{
			name:      "unknown or destructive cleanup target",
			safeguard: NonWaivableCleanupTarget,
			invariant: "cleanup target",
			recovery:  "handoff",
		},
		{
			name:      "incorrect or missing recorded worktree identity",
			safeguard: NonWaivableWorktreeIdentity,
			invariant: "worktree identity",
			recovery:  "verified terminal archive",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			refusal := RefuseNonWaivableOverride(NonWaivableOverrideRequest{
				Safeguard:      testCase.safeguard,
				PreservedPaths: paths,
			})
			if !refusal.Refused {
				t.Fatal("non-waivable integrity override was not refused")
			}
			if !strings.Contains(refusal.FailedInvariant, testCase.invariant) {
				t.Fatalf("failed invariant = %q, want %q", refusal.FailedInvariant, testCase.invariant)
			}
			if !reflect.DeepEqual(refusal.PreservedPaths, paths) {
				t.Fatalf("preserved paths = %#v, want %#v", refusal.PreservedPaths, paths)
			}
			if !strings.Contains(refusal.Recovery, testCase.recovery) {
				t.Fatalf("recovery = %q, want safe %q alternative", refusal.Recovery, testCase.recovery)
			}
			for _, path := range paths {
				contents, err := os.ReadFile(filepath.Join(repo, path))
				if err != nil || string(contents) != "preserve\n" {
					t.Fatalf("preserved path %q was changed by refusal: %q, %v", path, contents, err)
				}
			}
		})
	}
}
