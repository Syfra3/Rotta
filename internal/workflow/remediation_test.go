package workflow

import (
	"reflect"
	"strings"
	"testing"
)

func TestRemediationStopsAfterOneFreshMaterialReview(t *testing.T) {
	state := testRemediationState(t)
	if delegate, err := state.RecordMaterialReview(testReview(state, "initial", "snapshot-0")); err != nil || !delegate {
		t.Fatalf("initial review = %v, %v", delegate, err)
	}
	if err := state.RecordRemediation(testRemediationEvidence(state, "snapshot-1")); err != nil {
		t.Fatal(err)
	}
	freshReview := testReview(state, "first-fresh", "snapshot-1")
	freshReview.EvidenceRefs = []string{"evidence/first-fresh", "evidence/command-output", "evidence/review-log"}
	if delegate, err := state.RecordFreshReview(freshReview); err != nil || delegate || !state.Stopped || len(state.UnresolvedFindings) == 0 {
		t.Fatalf("first fresh review = delegate %v, err %v, state %#v", delegate, err, state)
	}
	if got, want := state.UnresolvedEvidenceRefs, freshReview.EvidenceRefs; !reflect.DeepEqual(got, want) {
		t.Fatalf("unresolved evidence = %#v, want %#v", got, want)
	}
	if err := state.RecordRemediation(testRemediationEvidence(state, "snapshot-2")); err == nil || !strings.Contains(err.Error(), "not currently delegated") {
		t.Fatalf("second automatic remediation = %v, want rejection", err)
	}
	if _, err := state.RecordFreshReview(testReview(state, "late", "snapshot-1")); err == nil || !strings.Contains(err.Error(), "stopped") {
		t.Fatalf("late report = %v, want stopped rejection", err)
	}
}

func TestRemediationRequiresChangedScopedEvidenceFreshReviewAndBoundState(t *testing.T) {
	state := testRemediationState(t)
	if _, err := state.RecordMaterialReview(testReview(state, "initial", "snapshot-0")); err != nil {
		t.Fatal(err)
	}
	bad := testRemediationEvidence(state, "snapshot-1")
	bad.ChangedPaths = []string{"cmd/rotta/main.go"}
	if err := state.RecordRemediation(bad); err == nil || !strings.Contains(err.Error(), "outside approved scope") {
		t.Fatalf("out of scope remediation = %v", err)
	}
	bad = testRemediationEvidence(state, "snapshot-1")
	bad.ContractFingerprint = "changed"
	if err := state.RecordRemediation(bad); err == nil || !strings.Contains(err.Error(), "renewed") {
		t.Fatalf("drift remediation = %v", err)
	}
	if err := state.RecordRemediation(testRemediationEvidence(state, "snapshot-1")); err != nil {
		t.Fatal(err)
	}
	stale := testReview(state, "fresh", "snapshot-1")
	stale.Revision--
	if _, err := state.RecordFreshReview(stale); err == nil || !strings.Contains(err.Error(), "stale concurrent") {
		t.Fatalf("stale review = %v", err)
	}
	if _, err := state.RecordFreshReview(testReview(state, "fresh", "snapshot-0")); err == nil || !strings.Contains(err.Error(), "exactly match") {
		t.Fatalf("non-fresh review = %v", err)
	}
}

func TestFreshReviewRequiresTheCycleRemediationSnapshotAndIsRetryable(t *testing.T) {
	state := testRemediationState(t)
	if _, err := state.RecordMaterialReview(testReview(state, "initial", "snapshot-0")); err != nil {
		t.Fatal(err)
	}
	if err := state.RecordRemediation(testRemediationEvidence(state, "snapshot-1")); err != nil {
		t.Fatal(err)
	}
	beforeRevision := state.Revision
	beforeCycle := state.Cycles[0]
	wrongSnapshot := testReview(state, "wrong-snapshot", "snapshot-other")
	if _, err := state.RecordFreshReview(wrongSnapshot); err == nil || !strings.Contains(err.Error(), "exactly match") {
		t.Fatalf("wrong snapshot fresh review = %v, want exact snapshot rejection", err)
	}
	if state.Revision != beforeRevision || !state.AwaitingFreshReview || state.Cycles[0].ReviewID != beforeCycle.ReviewID || state.Cycles[0].ReviewSnapshot != beforeCycle.ReviewSnapshot {
		t.Fatalf("wrong snapshot mutated retryable state: %#v", state)
	}
	invalid := testReview(state, "invalid", "snapshot-1")
	invalid.Findings = nil
	if _, err := state.RecordFreshReview(invalid); err == nil || !strings.Contains(err.Error(), "findings") {
		t.Fatalf("incomplete fresh review = %v, want validation rejection", err)
	}
	if state.Revision != beforeRevision || !state.AwaitingFreshReview {
		t.Fatalf("invalid review mutated retryable state: %#v", state)
	}
	corrected := testReview(state, "retry", "snapshot-1")
	corrected.EvidenceRefs = []string{"evidence/retry", "evidence/retry-log"}
	if delegate, err := state.RecordFreshReview(corrected); err != nil || delegate || !state.Stopped {
		t.Fatalf("corrected retry = %v, %v", delegate, err)
	}
	if got, want := state.UnresolvedEvidenceRefs, corrected.EvidenceRefs; !reflect.DeepEqual(got, want) {
		t.Fatalf("retry unresolved evidence = %#v, want %#v", got, want)
	}
}

func testRemediationState(t *testing.T) *RemediationState {
	t.Helper()
	state, err := NewRemediationState([]string{"internal/workflow/"}, "contract", "policy", "baseline", "implementation")
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func testReview(state *RemediationState, id, snapshot string) MaterialReviewReport {
	return MaterialReviewReport{Revision: state.Revision, ReviewID: id, Material: true, InScope: true, Findings: []string{"material defect"}, EvidenceRefs: []string{"evidence/" + id}, ReviewedSnapshot: snapshot, ContractFingerprint: "contract", PolicyFingerprint: "policy", Baseline: "baseline", Target: "implementation"}
}

func testRemediationEvidence(state *RemediationState, snapshot string) RemediationEvidence {
	return RemediationEvidence{Revision: state.Revision, ChangedPaths: []string{"internal/workflow/remediation.go"}, DeterministicResults: []string{"go test ./internal/workflow: passed"}, Snapshot: snapshot, ContractFingerprint: "contract", PolicyFingerprint: "policy", Baseline: "baseline", Target: "implementation"}
}
