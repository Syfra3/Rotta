package workflow

import (
	"errors"
	"fmt"
	"sync"
)

const maxMaterialCorrectionCycles = 2

type MaterialReviewReport struct {
	Revision            uint64
	ReviewID            string
	Material            bool
	InScope             bool
	Findings            []string
	EvidenceRefs        []string
	ReviewedSnapshot    string
	ContractFingerprint string
	PolicyFingerprint   string
	Baseline            string
	Target              string
}

type RemediationEvidence struct {
	Revision             uint64
	ChangedPaths         []string
	DeterministicResults []string
	Snapshot             string
	ContractFingerprint  string
	PolicyFingerprint    string
	Baseline             string
	Target               string
}

type RemediationCycle struct {
	FailureEvidence        []string
	ChangedPaths           []string
	DeterministicResults   []string
	ReviewID               string
	ReviewSnapshot         string
	RequiredReviewSnapshot string
}

// RemediationState is a revisioned, concurrency-safe state machine for one
// unchanged approved slice. Callers persist its exported value at their normal
// workflow boundary; stale reports cannot mutate a newer revision.
type RemediationState struct {
	mu                  sync.Mutex
	Revision            uint64
	Scope               []string
	ContractFingerprint string
	PolicyFingerprint   string
	Baseline            string
	Target              string
	Cycles              []RemediationCycle
	AwaitingRemediation bool
	AwaitingFreshReview bool
	Stopped             bool
	UnresolvedFindings  []string
}

func NewRemediationState(scope []string, contractFingerprint, policyFingerprint, baseline, target string) (*RemediationState, error) {
	if len(scope) == 0 || contractFingerprint == "" || policyFingerprint == "" || baseline == "" || target == "" {
		return nil, errors.New("remediation state requires bound scope, contract, policy, baseline, and target")
	}
	return &RemediationState{Scope: append([]string(nil), scope...), ContractFingerprint: contractFingerprint, PolicyFingerprint: policyFingerprint, Baseline: baseline, Target: target}, nil
}

// RecordMaterialReview starts one correction cycle, or hard-stops after the
// second fresh remediation review. Non-material/out-of-scope reports are kept
// outside cycle accounting.
func (state *RemediationState) RecordMaterialReview(report MaterialReviewReport) (bool, error) {
	state.mu.Lock()
	defer state.mu.Unlock()
	if report.Revision != state.Revision {
		return false, errors.New("stale concurrent review report")
	}
	if !state.matchesBinding(report.ContractFingerprint, report.PolicyFingerprint, report.Baseline, report.Target) {
		return false, errors.New("material workflow drift requires a renewed bound decision")
	}
	if state.Stopped {
		return false, errors.New("remediation is stopped; choose explicit resume, scope-change, or cancellation")
	}
	if !report.Material || !report.InScope {
		return false, nil
	}
	if report.ReviewID == "" || report.ReviewedSnapshot == "" || len(report.Findings) == 0 || len(report.EvidenceRefs) == 0 {
		return false, errors.New("material review requires findings, evidence, and a reviewed snapshot")
	}
	if state.AwaitingRemediation || state.AwaitingFreshReview {
		return false, errors.New("material review is not the required fresh review for the current remediation state")
	}
	if len(state.Cycles) == maxMaterialCorrectionCycles {
		state.Stopped = true
		state.UnresolvedFindings = append([]string(nil), report.Findings...)
		state.Revision++
		return false, nil
	}
	state.Cycles = append(state.Cycles, RemediationCycle{FailureEvidence: append([]string(nil), report.EvidenceRefs...), ReviewID: report.ReviewID, ReviewSnapshot: report.ReviewedSnapshot})
	state.AwaitingRemediation = true
	state.Revision++
	return true, nil
}

func (state *RemediationState) RecordRemediation(evidence RemediationEvidence) error {
	state.mu.Lock()
	defer state.mu.Unlock()
	if evidence.Revision != state.Revision {
		return errors.New("stale concurrent remediation evidence")
	}
	if !state.matchesBinding(evidence.ContractFingerprint, evidence.PolicyFingerprint, evidence.Baseline, evidence.Target) {
		return errors.New("material workflow drift requires a renewed bound decision")
	}
	if state.Stopped || !state.AwaitingRemediation {
		return errors.New("remediation is not currently delegated")
	}
	if evidence.Snapshot == "" || len(evidence.ChangedPaths) == 0 || len(evidence.DeterministicResults) == 0 {
		return errors.New("remediation requires changed paths, deterministic results, and a new snapshot")
	}
	for _, path := range evidence.ChangedPaths {
		if !inHandoffScope(path, state.Scope) {
			return fmt.Errorf("remediation changed path %q is outside approved scope", path)
		}
	}
	cycle := &state.Cycles[len(state.Cycles)-1]
	cycle.ChangedPaths = append([]string(nil), evidence.ChangedPaths...)
	cycle.DeterministicResults = append([]string(nil), evidence.DeterministicResults...)
	cycle.RequiredReviewSnapshot = evidence.Snapshot
	state.AwaitingRemediation = false
	state.AwaitingFreshReview = true
	state.Revision++
	return nil
}

// RecordFreshReview closes a remediation only when it reviews the new changed
// snapshot. A material result immediately enters the next cycle or hard-stops.
func (state *RemediationState) RecordFreshReview(report MaterialReviewReport) (bool, error) {
	state.mu.Lock()
	defer state.mu.Unlock()
	if report.Revision != state.Revision {
		return false, errors.New("stale concurrent review report")
	}
	if !state.matchesBinding(report.ContractFingerprint, report.PolicyFingerprint, report.Baseline, report.Target) {
		return false, errors.New("material workflow drift requires a renewed bound decision")
	}
	if state.Stopped {
		return false, errors.New("remediation is stopped; choose explicit resume, scope-change, or cancellation")
	}
	if !state.AwaitingFreshReview {
		return false, errors.New("fresh review is not currently required")
	}
	cycle := &state.Cycles[len(state.Cycles)-1]
	if report.ReviewID == "" || report.ReviewedSnapshot == "" || cycle.RequiredReviewSnapshot == "" || report.ReviewedSnapshot != cycle.RequiredReviewSnapshot {
		return false, errors.New("fresh review must exactly match the required remediation snapshot")
	}
	if report.Material && report.InScope && (len(report.Findings) == 0 || len(report.EvidenceRefs) == 0) {
		return false, errors.New("material fresh review requires findings and evidence")
	}
	// All validation is complete before advancing the revision or recording a
	// result, so a malformed report can be corrected and retried.
	cycle.ReviewID, cycle.ReviewSnapshot = report.ReviewID, report.ReviewedSnapshot
	state.AwaitingFreshReview = false
	state.Revision++
	if !report.Material || !report.InScope {
		return false, nil
	}
	if len(state.Cycles) == maxMaterialCorrectionCycles {
		state.Stopped = true
		state.UnresolvedFindings = append([]string(nil), report.Findings...)
		return false, nil
	}
	state.Cycles = append(state.Cycles, RemediationCycle{FailureEvidence: append([]string(nil), report.EvidenceRefs...), ReviewID: report.ReviewID, ReviewSnapshot: report.ReviewedSnapshot})
	state.AwaitingRemediation = true
	return true, nil
}

func (state *RemediationState) matchesBinding(contract, policy, baseline, target string) bool {
	return contract == state.ContractFingerprint && policy == state.PolicyFingerprint && baseline == state.Baseline && target == state.Target
}
