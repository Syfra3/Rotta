package workflow

import "errors"

type V2WorkerEvidence struct {
	SubmissionID, Phase, Scope, Outcome, Commit, EvidenceRef string
	LedgerVersion                                            uint64
}

func ValidateV2WorkerEvidence(e V2WorkerEvidence) error {
	if e.SubmissionID == "" || e.Phase == "" || e.Scope == "" || e.Outcome == "" || e.Commit == "" || e.EvidenceRef == "" || e.LedgerVersion == 0 {
		return errors.New("v2 worker evidence is incomplete and cannot authorize a transition")
	}
	return nil
}
