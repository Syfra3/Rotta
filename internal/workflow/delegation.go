package workflow

import "errors"

type WorkerEvidence struct {
	SubmissionID, Phase, Scope, Outcome, Commit, EvidenceRef string
	LedgerVersion                                            uint64
}

func ValidateWorkerEvidence(e WorkerEvidence) error {
	if e.SubmissionID == "" || e.Phase == "" || e.Scope == "" || e.Outcome == "" || e.Commit == "" || e.EvidenceRef == "" || e.LedgerVersion == 0 {
		return errors.New("worker evidence is incomplete and cannot authorize a transition")
	}
	return nil
}
