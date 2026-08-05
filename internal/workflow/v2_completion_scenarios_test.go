package workflow

import "testing"

func TestSCN706_ContractHandoffUsesBoundedDraftPacket(t *testing.T) {
	ledger := V2SubmissionLedger{Status: V2DraftStatus, BaseCommit: v2TestCommit}
	if err := ValidateV2ContractHandoff(ledger, V2DraftExplorationPacket{BaseCommit: v2TestCommit, QueryPurposes: []string{"impact"}}); err != nil {
		t.Fatal(err)
	}
}
func TestSCN732_PostRemovalRejectsReachableLegacyPath(t *testing.T) {
	if err := VerifyV2PostRemoval([]V2InventoryEntry{{Path: "legacy/runner.go", Classification: "legacy_only"}}, []string{"legacy/runner.go"}); err == nil {
		t.Fatal("want reachable legacy rejection")
	}
}
func TestSCN739_LocalVelaEvidenceRejectsEgressAndPayload(t *testing.T) {
	if err := ValidateV2LocalVelaExecution(V2LocalVelaExecution{Endpoint: "local_process", Storage: ".vela/graph", Purpose: "index"}, "LOCAL_ONLY_SOURCE_SENTINEL"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateV2LocalVelaExecution(V2LocalVelaExecution{Endpoint: "local_process", Storage: ".vela/graph", Purpose: "index", NonLoopbackAttempts: 1}, ""); err == nil {
		t.Fatal("want egress rejection")
	}
}
