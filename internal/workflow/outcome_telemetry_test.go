package workflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestREQ091OutcomeRecordRoundTripsExactVersionedSchema(t *testing.T) {
	record := req091ComparableRun("ses-root-1", REQ091NonCacheTokenLimit, REQ091ChildSessionLimit)
	record.Tokens.Reasoning = NotObservableTelemetry("opencode usage API did not expose reasoning")

	encoded, err := MarshalOutcomeRecord(record)
	if err != nil {
		t.Fatalf("MarshalOutcomeRecord() error = %v", err)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"format", "run_identity", "root_session_id", "child_sessions", "role_invocations", "tokens", "cache_tokens", "human_decisions", "continuations", "correction_cycles", "deterministic_commands", "deterministic_validation", "independent_final_review"} {
		if _, ok := payload[required]; !ok {
			t.Errorf("outcome schema omitted %q: %s", required, encoded)
		}
	}
	forbidden := []string{"provider_cost", "cost", "non_cache_tokens"}
	for _, field := range forbidden {
		if _, ok := payload[field]; ok {
			t.Errorf("outcome schema included unsupported %q: %s", field, encoded)
		}
	}
	var reasoning map[string]json.RawMessage
	var tokens map[string]json.RawMessage
	if err := json.Unmarshal(payload["tokens"], &tokens); err != nil || json.Unmarshal(tokens["reasoning_tokens"], &reasoning) != nil {
		t.Fatalf("decode not_observable metric: %v", err)
	}
	if string(reasoning["status"]) != `"not_observable"` || string(reasoning["source"]) == "" {
		t.Fatalf("reasoning telemetry = %s, want field-level not_observable with source", tokens["reasoning_tokens"])
	}
	if _, ok := reasoning["value"]; ok {
		t.Fatalf("not_observable telemetry must not serialize a zero or value: %s", tokens["reasoning_tokens"])
	}
	decoded, err := UnmarshalOutcomeRecord(encoded)
	if err != nil || decoded.Tokens.Reasoning.Status != TelemetryNotObservable || decoded.Tokens.Reasoning.Value != nil {
		t.Fatalf("UnmarshalOutcomeRecord() = %#v, %v", decoded.Tokens.Reasoning, err)
	}
}

func TestREQ091ComparatorPassesComparableBoundaryMedianAndSeparatesCache(t *testing.T) {
	worktree, runs := req091ComparableRuns(t)
	runs[0].Tokens.Input = ObservedTelemetry(1_600_000, "host.usage.input")
	runs[1].Tokens.Input = ObservedTelemetry(REQ091NonCacheTokenLimit, "host.usage.input")
	runs[1].ChildSessions = ObservedTelemetry(24, "host.sessions.recursive_children")
	runs[0].CacheTokens = OutcomeCacheTokens{Read: ObservedTelemetry(99_999_999, "host.cache.read"), Write: ObservedTelemetry(88_888_888, "host.cache.write")}
	runs[1].CacheTokens = OutcomeCacheTokens{Read: NotObservableTelemetry("host did not expose cache reads"), Write: NotObservableTelemetry("host did not expose cache writes")}

	result := CompareOutcomeBenchmarkInWorktree(worktree, runs)
	if result.Status != OutcomeBenchmarkPassed || result.MedianNonCacheTokens == nil || *result.MedianNonCacheTokens != REQ091NonCacheTokenLimit {
		t.Fatalf("CompareOutcomeBenchmark() = %#v, want pass at median boundary", result)
	}
	if len(result.CacheTokens) != 3 || result.CacheTokens[0].Read.Value == nil || *result.CacheTokens[0].Read.Value != 99_999_999 || result.CacheTokens[1].Read.Status != TelemetryNotObservable {
		t.Fatalf("cache telemetry was not reported separately: %#v", result.CacheTokens)
	}
	if strings.Contains(strings.ToLower(strings.Join(result.Reasons, " ")), "cost") {
		t.Fatalf("benchmark result made a provider cost claim: %#v", result)
	}
}

func TestREQ091ComparatorRejectsEveryMaterialIdentityDifferenceBeforeThresholds(t *testing.T) {
	for _, test := range []struct {
		name  string
		field string
		alter func(*OutcomeRunIdentity)
	}{
		{"feature ID", "feature_id", func(identity *OutcomeRunIdentity) { identity.FeatureID = "other-feature" }},
		{"feature request", "feature_request_fingerprint", func(identity *OutcomeRunIdentity) { identity.FeatureRequestFingerprint = "different-request" }},
		{"contract", "contract_fingerprint", func(identity *OutcomeRunIdentity) { identity.ContractFingerprint = "different-contract" }},
		{"policy", "policy_fingerprint", func(identity *OutcomeRunIdentity) { identity.PolicyFingerprint = "different-policy" }},
		{"baseline", "repository_baseline", func(identity *OutcomeRunIdentity) { identity.RepositoryBaseline = "different-baseline" }},
		{"model ID", "model_identifier", func(identity *OutcomeRunIdentity) { identity.ModelIdentifier = "different-model" }},
		{"model family", "model_family", func(identity *OutcomeRunIdentity) { identity.ModelFamily = "different-family" }},
		{"integrations", "enabled_integrations", func(identity *OutcomeRunIdentity) { identity.EnabledIntegrations = []string{"ancora", "vela"} }},
		{"permissions", "operational_permissions", func(identity *OutcomeRunIdentity) { identity.OperationalPermissions = []string{"read", "write"} }},
		{"acceptance checks", "acceptance_checks", func(identity *OutcomeRunIdentity) {
			identity.AcceptanceChecks = []string{"go test ./internal/workflow", "git diff --check"}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			worktree, runs := req091ComparableRuns(t)
			test.alter(&runs[2].RunIdentity)
			rebindReq091RunEvidence(t, worktree, &runs[2])
			result := CompareOutcomeBenchmarkInWorktree(worktree, runs)
			if result.Status != OutcomeBenchmarkNotEvaluable || result.MedianNonCacheTokens != nil || !containsReason(result.Reasons, test.field) {
				t.Fatalf("identity difference %s result = %#v", test.field, result)
			}
		})
	}
}

func TestREQ091ComparatorUsesElementWiseIdentityListComparison(t *testing.T) {
	worktree, runs := req091ComparableRuns(t)
	runs[0].RunIdentity.EnabledIntegrations = []string{"a", "b\x00c"}
	runs[1].RunIdentity.EnabledIntegrations = []string{"a\x00b", "c"}
	rebindReq091RunEvidence(t, worktree, &runs[0])
	rebindReq091RunEvidence(t, worktree, &runs[1])
	result := CompareOutcomeBenchmarkInWorktree(worktree, runs)
	if result.Status != OutcomeBenchmarkNotEvaluable || !containsReason(result.Reasons, "enabled_integrations") {
		t.Fatalf("NUL list collision result = %#v", result)
	}
}

func TestREQ091ComparatorDoesNotTreatNotObservableRequiredMetricsAsZero(t *testing.T) {
	worktree, runs := req091ComparableRuns(t)
	runs[1].Tokens.Reasoning = NotObservableTelemetry("host usage response omits reasoning")
	result := CompareOutcomeBenchmarkInWorktree(worktree, runs)
	if result.Status != OutcomeBenchmarkNotEvaluable || result.MedianNonCacheTokens != nil || !containsReason(result.Reasons, "reasoning_tokens") {
		t.Fatalf("not_observable reasoning result = %#v", result)
	}
	if runs[1].Tokens.Reasoning.Value != nil {
		t.Fatal("test setup inferred an unavailable value")
	}
}

func TestREQ091ComparatorFailsMedianAndChildBoundaries(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func([]WorkflowOutcomeRecord)
		want string
	}{
		{
			name: "median above threshold",
			edit: func(runs []WorkflowOutcomeRecord) {
				runs[1].Tokens.Input = ObservedTelemetry(REQ091NonCacheTokenLimit+1, "host.input")
				runs[2].Tokens.Input = ObservedTelemetry(REQ091NonCacheTokenLimit+1, "host.input")
			},
			want: "median non-cache tokens",
		},
		{
			name: "one run over child cap",
			edit: func(runs []WorkflowOutcomeRecord) {
				runs[2].ChildSessions = ObservedTelemetry(REQ091ChildSessionLimit+1, "host.children")
			},
			want: "child sessions",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			worktree, runs := req091ComparableRuns(t)
			test.edit(runs)
			result := CompareOutcomeBenchmarkInWorktree(worktree, runs)
			if result.Status != OutcomeBenchmarkFailed || result.MedianNonCacheTokens == nil || !containsReason(result.Reasons, test.want) {
				t.Fatalf("threshold result = %#v", result)
			}
		})
	}
}

func TestREQ091ComparatorFailsWithoutDurableValidationOrIndependentReview(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*WorkflowOutcomeRecord)
		want string
	}{
		{"no deterministic evidence", func(run *WorkflowOutcomeRecord) { run.DeterministicValidation.Evidence = nil }, "deterministic validation"},
		{"failed deterministic validation", func(run *WorkflowOutcomeRecord) { run.DeterministicValidation.Status = OutcomeFailed }, "deterministic validation"},
		{"no observed command", func(run *WorkflowOutcomeRecord) {
			run.DeterministicCommands = NotObservableTelemetry("command collector unavailable")
		}, "deterministic command"},
		{"review is not independent", func(run *WorkflowOutcomeRecord) { run.IndependentFinalReview.Independent = false }, "independent final review"},
		{"no review evidence", func(run *WorkflowOutcomeRecord) { run.IndependentFinalReview.Evidence = nil }, "independent final review"},
		{"failed review", func(run *WorkflowOutcomeRecord) { run.IndependentFinalReview.Status = OutcomeFailed }, "independent final review"},
	} {
		t.Run(test.name, func(t *testing.T) {
			worktree, runs := req091ComparableRuns(t)
			test.edit(&runs[0])
			result := CompareOutcomeBenchmarkInWorktree(worktree, runs)
			if result.Status != OutcomeBenchmarkFailed || !containsReason(result.Reasons, test.want) {
				t.Fatalf("durable quality result = %#v", result)
			}
		})
	}
}

func TestREQ091OutcomeRecordRejectsUnavailableMetricWithoutSourceOrValue(t *testing.T) {
	record := req091ComparableRun("ses-root-1", 1, 1)
	record.HumanDecisions = TelemetryMetric{Status: TelemetryNotObservable}
	if _, err := MarshalOutcomeRecord(record); err == nil || !strings.Contains(err.Error(), "human_decisions") {
		t.Fatalf("MarshalOutcomeRecord() error = %v, want field-level unavailable source rejection", err)
	}
	record.HumanDecisions = TelemetryMetric{Status: TelemetryNotObservable, Value: uint64Pointer(0), Source: "host omitted human decisions"}
	if _, err := MarshalOutcomeRecord(record); err == nil || !strings.Contains(err.Error(), "not_observable with a value") {
		t.Fatalf("MarshalOutcomeRecord() error = %v, want unavailable value rejection", err)
	}
}

func TestREQ091ComparatorRequiresExactlyThreeDistinctRetainedRuns(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func([]WorkflowOutcomeRecord) []WorkflowOutcomeRecord
		want string
	}{
		{"too few", func(runs []WorkflowOutcomeRecord) []WorkflowOutcomeRecord { return runs[:2] }, "exactly 3 retained runs"},
		{"too many", func(runs []WorkflowOutcomeRecord) []WorkflowOutcomeRecord { return append(runs, runs[2]) }, "exactly 3 retained runs"},
		{"duplicate root session", func(runs []WorkflowOutcomeRecord) []WorkflowOutcomeRecord {
			runs[2].RootSessionID = runs[1].RootSessionID
			return runs
		}, "duplicates a retained root session ID"},
		{"duplicate run ID", func(runs []WorkflowOutcomeRecord) []WorkflowOutcomeRecord { runs[2].RunID = runs[1].RunID; return runs }, "duplicates a retained run ID"},
		{"reused durable evidence", func(runs []WorkflowOutcomeRecord) []WorkflowOutcomeRecord {
			runs[2].DeterministicValidation.Evidence = runs[1].DeterministicValidation.Evidence
			return runs
		}, "reuses durable evidence"},
	} {
		t.Run(test.name, func(t *testing.T) {
			worktree, runs := req091ComparableRuns(t)
			result := CompareOutcomeBenchmarkInWorktree(worktree, test.edit(runs))
			if result.Status != OutcomeBenchmarkNotEvaluable || !containsReason(result.Reasons, test.want) {
				t.Fatalf("distinct retained runs result = %#v", result)
			}
		})
	}
}

func TestREQ091ComparatorRejectsForgedTamperedOrMismatchedBoundEvidence(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(t *testing.T, worktree string, runs []WorkflowOutcomeRecord)
		want string
	}{
		{
			name: "caller forged passing status",
			edit: func(t *testing.T, worktree string, runs []WorkflowOutcomeRecord) {
				rewriteReq091EvidenceBinding(t, worktree, &runs[0].DeterministicValidation.Evidence[0], func(binding *OutcomeEvidenceBinding) { binding.Status = OutcomeFailed })
			},
			want: "belongs to another run, check, or identity",
		},
		{
			name: "tampered content",
			edit: func(t *testing.T, worktree string, runs []WorkflowOutcomeRecord) {
				path := filepath.Join(worktree, filepath.FromSlash(runs[0].DeterministicValidation.Evidence[0].Path))
				contents, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				var evidence lifecycleCommandEvidence
				if err := json.Unmarshal(contents, &evidence); err != nil {
					t.Fatal(err)
				}
				evidence.Stdout = `{"tampered":true}`
				updated, err := json.Marshal(evidence)
				if err != nil {
					t.Fatal(err)
				}
				mustWrite(t, path, string(updated))
			},
			want: "evidence content hash does not match persisted stdout/stderr",
		},
		{
			name: "mismatched check",
			edit: func(t *testing.T, worktree string, runs []WorkflowOutcomeRecord) {
				runs[0].DeterministicValidation.Evidence[0].Check = "git diff --check"
			},
			want: "belongs to another run, check, or identity",
		},
		{
			name: "caller forged independent review",
			edit: func(t *testing.T, worktree string, runs []WorkflowOutcomeRecord) {
				rewriteReq091EvidenceBinding(t, worktree, &runs[0].IndependentFinalReview.Evidence[0], func(binding *OutcomeEvidenceBinding) { binding.Independent = false })
			},
			want: "incorrect independent-review state",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			worktree, runs := req091ComparableRuns(t)
			test.edit(t, worktree, runs)
			result := CompareOutcomeBenchmarkInWorktree(worktree, runs)
			if result.Status != OutcomeBenchmarkNotEvaluable || !containsReason(result.Reasons, test.want) {
				t.Fatalf("bound evidence result = %#v", result)
			}
		})
	}
}

func TestREQ091OutcomeRecordRejectsDuplicateAndCaseVariantJSONFields(t *testing.T) {
	record := req091ComparableRun("ses-root-1", 1, 1)
	encoded, err := MarshalOutcomeRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		data string
		want string
	}{
		{"duplicate canonical root field", strings.Replace(string(encoded), `"format":`, `"format":"forged","format":`, 1), "duplicate JSON field"},
		{"case variant root alias", strings.Replace(string(encoded), `"format":`, `"FORMAT":`, 1), "ambiguous JSON field alias"},
		{"duplicate nested field", strings.Replace(string(encoded), `"feature_id":`, `"feature_id":"forged","feature_id":`, 1), "duplicate JSON field"},
		{"case variant nested alias", strings.Replace(string(encoded), `"feature_id":`, `"FEATURE_ID":`, 1), "ambiguous JSON field alias"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := UnmarshalOutcomeRecord([]byte(test.data)); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("UnmarshalOutcomeRecord() error = %v, want %q", err, test.want)
			}
		})
	}
}

func req091ComparableRuns(t *testing.T) (string, []WorkflowOutcomeRecord) {
	t.Helper()
	worktree := t.TempDir()
	runs := []WorkflowOutcomeRecord{
		req091ComparableRun("ses-root-1", REQ091NonCacheTokenLimit, REQ091ChildSessionLimit),
		req091ComparableRun("ses-root-2", REQ091NonCacheTokenLimit, REQ091ChildSessionLimit),
		req091ComparableRun("ses-root-3", REQ091NonCacheTokenLimit, REQ091ChildSessionLimit),
	}
	for index := range runs {
		runs[index].DeterministicValidation.Evidence[0] = writeReq091BoundEvidence(t, worktree, index, runs[index], deterministicValidationEvidence, "go test ./internal/workflow")
		runs[index].IndependentFinalReview.Evidence[0] = writeReq091BoundEvidence(t, worktree, index, runs[index], independentReviewEvidence, "independent-final-review")
	}
	return worktree, runs
}

func req091ComparableRun(rootSession string, nonCacheTokens, children uint64) WorkflowOutcomeRecord {
	return WorkflowOutcomeRecord{
		Format: OutcomeTelemetryFormat,
		RunID:  "run-" + rootSession,
		RunIdentity: OutcomeRunIdentity{
			FeatureID:                 "token-efficient-workflow",
			FeatureRequestFingerprint: "feature-request-sha256",
			ContractFingerprint:       "contract-sha256",
			PolicyFingerprint:         "policy-sha256",
			RepositoryBaseline:        "8b356db634838a6647b3e287c9f819a0fff0313c",
			ProviderIdentifier:        "openai",
			ModelIdentifier:           "openai/gpt-5.6-terra",
			ModelFamily:               "gpt-5.6",
			EnabledIntegrations:       []string{"vela"},
			OperationalPermissions:    []string{"read"},
			AcceptanceChecks:          []string{"go test ./internal/workflow"},
		},
		RootSessionID:         rootSession,
		ChildSessions:         ObservedTelemetry(children, "host.sessions.recursive_children"),
		RoleInvocations:       map[string]TelemetryMetric{"impl": ObservedTelemetry(1, "host.roles.impl"), "review": ObservedTelemetry(1, "host.roles.review")},
		Tokens:                OutcomeTokens{Input: ObservedTelemetry(nonCacheTokens, "host.usage.input"), Output: ObservedTelemetry(0, "host.usage.output"), Reasoning: ObservedTelemetry(0, "host.usage.reasoning")},
		CacheTokens:           OutcomeCacheTokens{Read: ObservedTelemetry(10, "host.usage.cache_read"), Write: ObservedTelemetry(2, "host.usage.cache_write")},
		HumanDecisions:        ObservedTelemetry(1, "workflow.human_decisions"),
		Continuations:         ObservedTelemetry(0, "workflow.continuations"),
		CorrectionCycles:      ObservedTelemetry(0, "workflow.correction_cycles"),
		DeterministicCommands: ObservedTelemetry(1, "workflow.commands"),
		DeterministicValidation: DeterministicValidationOutcome{
			Status:   OutcomePassed,
			Evidence: []DurableEvidenceReference{{Check: "go test ./internal/workflow", Path: ".rotta/current/evidence/command.json", Hash: strings.Repeat("a", 64), Status: OutcomePassed}},
		},
		IndependentFinalReview: IndependentFinalReviewOutcome{
			Status: OutcomePassed, Independent: true,
			Evidence: []DurableEvidenceReference{{Check: "independent-final-review", Path: ".rotta/current/evidence/review.json", Hash: strings.Repeat("b", 64), Status: OutcomePassed}},
		},
	}
}

func writeReq091BoundEvidence(t *testing.T, worktree string, index int, run WorkflowOutcomeRecord, kind outcomeEvidenceKind, check string) DurableEvidenceReference {
	t.Helper()
	binding := OutcomeEvidenceBinding{
		Format: OutcomeEvidenceBindingFormat, Kind: kind, RunID: run.RunID, RootSessionID: run.RootSessionID, RunIdentity: run.RunIdentity,
		Check: check, Status: OutcomePassed, Independent: kind == independentReviewEvidence,
	}
	stdout, err := json.Marshal(binding)
	if err != nil {
		t.Fatal(err)
	}
	evidence := lifecycleCommandEvidence{Stdout: string(stdout), ExitStatus: 0, ContentHash: commandOutputHash(string(stdout), "")}
	path := filepath.ToSlash(filepath.Join(".rotta", "current", "evidence", "run-"+string(rune('1'+index))+"-"+string(kind)+".json"))
	if err := os.MkdirAll(filepath.Join(worktree, filepath.Dir(path)), 0o700); err != nil {
		t.Fatal(err)
	}
	contents, err := json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(worktree, filepath.FromSlash(path)), string(contents))
	return DurableEvidenceReference{Check: check, Path: path, Hash: evidence.ContentHash, Status: OutcomePassed}
}

func rewriteReq091EvidenceBinding(t *testing.T, worktree string, reference *DurableEvidenceReference, edit func(*OutcomeEvidenceBinding)) {
	t.Helper()
	path := filepath.Join(worktree, filepath.FromSlash(reference.Path))
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var evidence lifecycleCommandEvidence
	if err := json.Unmarshal(contents, &evidence); err != nil {
		t.Fatal(err)
	}
	binding, err := unmarshalOutcomeEvidenceBinding([]byte(evidence.Stdout))
	if err != nil {
		t.Fatal(err)
	}
	edit(&binding)
	stdout, err := json.Marshal(binding)
	if err != nil {
		t.Fatal(err)
	}
	evidence.Stdout = string(stdout)
	evidence.ContentHash = commandOutputHash(evidence.Stdout, evidence.Stderr)
	updated, err := json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, path, string(updated))
	reference.Hash = evidence.ContentHash
}

func rebindReq091RunEvidence(t *testing.T, worktree string, run *WorkflowOutcomeRecord) {
	t.Helper()
	for _, reference := range []*DurableEvidenceReference{&run.DeterministicValidation.Evidence[0], &run.IndependentFinalReview.Evidence[0]} {
		rewriteReq091EvidenceBinding(t, worktree, reference, func(binding *OutcomeEvidenceBinding) {
			binding.RunID = run.RunID
			binding.RootSessionID = run.RootSessionID
			binding.RunIdentity = run.RunIdentity
		})
	}
}

func uint64Pointer(value uint64) *uint64 { return &value }

func containsReason(reasons []string, want string) bool {
	for _, reason := range reasons {
		if strings.Contains(reason, want) {
			return true
		}
	}
	return false
}
