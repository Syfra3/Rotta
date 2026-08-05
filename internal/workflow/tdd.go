package workflow

import (
	"errors"
	"fmt"
	"strings"
)

type ScenarioRGREvidence struct {
	ScenarioID string `json:"scenario_id"`
	Red        string `json:"red"`
	Green      string `json:"green"`
	Refactor   string `json:"refactor"`
}

// TargetedVelaQuestion exists only when TDD is structurally blocked. It is
// deliberately narrower than the Draft exploration packet.
type TargetedVelaQuestion struct {
	Question string `json:"question"`
	Boundary string `json:"boundary"`
	Commit   string `json:"commit"`
	Outcome  string `json:"outcome"`
}

type TDDBatchRequest struct {
	SubmissionID  string
	LedgerVersion uint64
	Authorizer    string
	Worktree      WorktreeObservation
	Scenarios     []ScenarioRGREvidence
	TargetedVela  *TargetedVelaQuestion
	EvidenceRefs  []string
}

// AcceptTDDBatch records orchestrator acceptance of at most three approved
// scenarios. It never advances the lifecycle or selects another scenario.
func AcceptTDDBatch(repoRoot string, request TDDBatchRequest) (SubmissionLedger, error) {
	if err := validateSubmissionID(request.SubmissionID); err != nil {
		return SubmissionLedger{}, err
	}
	if request.Authorizer != orchestratorAuthorizer {
		return SubmissionLedger{}, errors.New(" TDD batch is unauthorized: expected orchestrator authorizer")
	}
	if len(request.Scenarios) == 0 || len(request.Scenarios) > 3 || len(request.EvidenceRefs) == 0 {
		return SubmissionLedger{}, errors.New(" TDD batch must contain one to three scenarios and evidence references")
	}
	if request.TargetedVela != nil && (strings.TrimSpace(request.TargetedVela.Question) == "" || strings.TrimSpace(request.TargetedVela.Boundary) == "" || !isFullCommitID(request.TargetedVela.Commit) || strings.TrimSpace(request.TargetedVela.Outcome) == "") {
		return SubmissionLedger{}, errors.New("targeted TDD Vela evidence requires a blocking question, boundary, commit, and outcome or uncertainty")
	}

	unlock, err := lockLedger(repoRoot, request.SubmissionID)
	if err != nil {
		return SubmissionLedger{}, err
	}
	defer unlock()
	ledger, err := LoadSubmissionLedger(repoRoot, request.SubmissionID)
	if err != nil {
		return SubmissionLedger{}, err
	}
	if ledger.Status != TDDStatus || ledger.LedgerVersion != request.LedgerVersion || ledger.Worktree == nil {
		return SubmissionLedger{}, fmt.Errorf(" TDD batch rejected: expected TDD at ledger version %d with a recorded worktree", request.LedgerVersion)
	}
	if err := ValidateRecordedWorktree(*ledger.Worktree, request.Worktree); err != nil {
		return SubmissionLedger{}, err
	}
	accepted := make(map[string]bool, len(ledger.AcceptedScenarioIDs))
	for _, id := range ledger.AcceptedScenarioIDs {
		accepted[id] = true
	}
	for _, scenario := range request.Scenarios {
		if !containsScenario(ledger.ApprovedScenarioIDs, scenario.ScenarioID) || accepted[scenario.ScenarioID] || strings.TrimSpace(scenario.Red) == "" || strings.TrimSpace(scenario.Green) == "" || strings.TrimSpace(scenario.Refactor) == "" {
			return SubmissionLedger{}, fmt.Errorf(" TDD batch has invalid or incomplete Red-Green-Refactor evidence for %q", scenario.ScenarioID)
		}
		accepted[scenario.ScenarioID] = true
		ledger.AcceptedScenarioIDs = append(ledger.AcceptedScenarioIDs, scenario.ScenarioID)
		ledger.TDDEvidence = append(ledger.TDDEvidence, scenario)
	}
	if request.TargetedVela != nil {
		ledger.TargetedVelaEvidence = append(ledger.TargetedVelaEvidence, *request.TargetedVela)
	}
	ledger.LedgerVersion++
	if err := writeLedgerAtomically(ledgerPath(repoRoot, request.SubmissionID), ledger); err != nil {
		return SubmissionLedger{}, err
	}
	return ledger, nil
}

func containsScenario(scenarios []string, candidate string) bool {
	for _, scenario := range scenarios {
		if scenario == candidate {
			return true
		}
	}
	return false
}
