package workflow

import (
	"errors"
	"fmt"
	"strings"
)

type V2ScenarioRGREvidence struct {
	ScenarioID string `json:"scenario_id"`
	Red        string `json:"red"`
	Green      string `json:"green"`
	Refactor   string `json:"refactor"`
}

// V2TargetedVelaQuestion exists only when TDD is structurally blocked. It is
// deliberately narrower than the Draft exploration packet.
type V2TargetedVelaQuestion struct {
	Question string `json:"question"`
	Boundary string `json:"boundary"`
	Commit   string `json:"commit"`
	Outcome  string `json:"outcome"`
}

type V2TDDBatchRequest struct {
	SubmissionID  string
	LedgerVersion uint64
	Authorizer    string
	Worktree      V2WorktreeObservation
	Scenarios     []V2ScenarioRGREvidence
	TargetedVela  *V2TargetedVelaQuestion
	EvidenceRefs  []string
}

// AcceptV2TDDBatch records orchestrator acceptance of at most three approved
// scenarios. It never advances the lifecycle or selects another scenario.
func AcceptV2TDDBatch(repoRoot string, request V2TDDBatchRequest) (V2SubmissionLedger, error) {
	if err := validateV2SubmissionID(request.SubmissionID); err != nil {
		return V2SubmissionLedger{}, err
	}
	if request.Authorizer != v2OrchestratorAuthorizer {
		return V2SubmissionLedger{}, errors.New("v2 TDD batch is unauthorized: expected orchestrator authorizer")
	}
	if len(request.Scenarios) == 0 || len(request.Scenarios) > 3 || len(request.EvidenceRefs) == 0 {
		return V2SubmissionLedger{}, errors.New("v2 TDD batch must contain one to three scenarios and evidence references")
	}
	if request.TargetedVela != nil && (strings.TrimSpace(request.TargetedVela.Question) == "" || strings.TrimSpace(request.TargetedVela.Boundary) == "" || !isFullCommitID(request.TargetedVela.Commit) || strings.TrimSpace(request.TargetedVela.Outcome) == "") {
		return V2SubmissionLedger{}, errors.New("targeted TDD Vela evidence requires a blocking question, boundary, commit, and outcome or uncertainty")
	}

	unlock, err := lockV2Ledger(repoRoot, request.SubmissionID)
	if err != nil {
		return V2SubmissionLedger{}, err
	}
	defer unlock()
	ledger, err := LoadV2SubmissionLedger(repoRoot, request.SubmissionID)
	if err != nil {
		return V2SubmissionLedger{}, err
	}
	if ledger.Status != V2TDDStatus || ledger.LedgerVersion != request.LedgerVersion || ledger.Worktree == nil {
		return V2SubmissionLedger{}, fmt.Errorf("v2 TDD batch rejected: expected TDD at ledger version %d with a recorded worktree", request.LedgerVersion)
	}
	if err := ValidateV2RecordedWorktree(*ledger.Worktree, request.Worktree); err != nil {
		return V2SubmissionLedger{}, err
	}
	accepted := make(map[string]bool, len(ledger.AcceptedScenarioIDs))
	for _, id := range ledger.AcceptedScenarioIDs {
		accepted[id] = true
	}
	for _, scenario := range request.Scenarios {
		if !containsV2Scenario(ledger.ApprovedScenarioIDs, scenario.ScenarioID) || accepted[scenario.ScenarioID] || strings.TrimSpace(scenario.Red) == "" || strings.TrimSpace(scenario.Green) == "" || strings.TrimSpace(scenario.Refactor) == "" {
			return V2SubmissionLedger{}, fmt.Errorf("v2 TDD batch has invalid or incomplete Red-Green-Refactor evidence for %q", scenario.ScenarioID)
		}
		accepted[scenario.ScenarioID] = true
		ledger.AcceptedScenarioIDs = append(ledger.AcceptedScenarioIDs, scenario.ScenarioID)
		ledger.TDDEvidence = append(ledger.TDDEvidence, scenario)
	}
	if request.TargetedVela != nil {
		ledger.TargetedVelaEvidence = append(ledger.TargetedVelaEvidence, *request.TargetedVela)
	}
	ledger.LedgerVersion++
	if err := writeV2LedgerAtomically(v2LedgerPath(repoRoot, request.SubmissionID), ledger); err != nil {
		return V2SubmissionLedger{}, err
	}
	return ledger, nil
}

func containsV2Scenario(scenarios []string, candidate string) bool {
	for _, scenario := range scenarios {
		if scenario == candidate {
			return true
		}
	}
	return false
}
