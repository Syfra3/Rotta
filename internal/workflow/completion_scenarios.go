package workflow

import "errors"

// ValidateContractHandoff prevents Contract from reusing Draft exploration
// as a reason to issue more exploratory Vela queries.
func ValidateContractHandoff(ledger SubmissionLedger, exploration DraftExplorationPacket) error {
	if ledger.Status != DraftStatus || exploration.BaseCommit != ledger.BaseCommit || len(exploration.QueryPurposes) > MaximumDraftQueries {
		return errors.New(" Contract handoff requires the bounded Draft packet at the selected base")
	}
	return nil
}

func VerifyPostRemoval(inventory []InventoryEntry, reachable []string) error {
	for _, entry := range inventory {
		if entry.Classification == "legacy_only" {
			for _, path := range reachable {
				if path == entry.Path {
					return errors.New(" post-removal verification found reachable legacy execution path")
				}
			}
		}
	}
	return nil
}

type LocalVelaExecution struct {
	Endpoint, Storage, Purpose string
	NonLoopbackAttempts        int
	OutboundPayload            string
}

func ValidateLocalVelaExecution(execution LocalVelaExecution, sentinel string) error {
	if execution.Endpoint == "" || execution.Storage == "" || execution.Purpose == "" || execution.NonLoopbackAttempts != 0 || (sentinel != "" && execution.OutboundPayload != "") {
		return errors.New(" Vela execution is not local-only")
	}
	return nil
}
