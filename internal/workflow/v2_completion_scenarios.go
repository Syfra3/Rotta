package workflow

import "errors"

// ValidateV2ContractHandoff prevents Contract from reusing Draft exploration
// as a reason to issue more exploratory Vela queries.
func ValidateV2ContractHandoff(ledger V2SubmissionLedger, exploration V2DraftExplorationPacket) error {
	if ledger.Status != V2DraftStatus || exploration.BaseCommit != ledger.BaseCommit || len(exploration.QueryPurposes) > v2MaximumDraftQueries {
		return errors.New("v2 Contract handoff requires the bounded Draft packet at the selected base")
	}
	return nil
}

func VerifyV2PostRemoval(inventory []V2InventoryEntry, reachable []string) error {
	for _, entry := range inventory {
		if entry.Classification == "legacy_only" {
			for _, path := range reachable {
				if path == entry.Path {
					return errors.New("v2 post-removal verification found reachable legacy execution path")
				}
			}
		}
	}
	return nil
}

type V2LocalVelaExecution struct {
	Endpoint, Storage, Purpose string
	NonLoopbackAttempts        int
	OutboundPayload            string
}

func ValidateV2LocalVelaExecution(execution V2LocalVelaExecution, sentinel string) error {
	if execution.Endpoint == "" || execution.Storage == "" || execution.Purpose == "" || execution.NonLoopbackAttempts != 0 || (sentinel != "" && execution.OutboundPayload != "") {
		return errors.New("v2 Vela execution is not local-only")
	}
	return nil
}
