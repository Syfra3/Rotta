package workflow

import "testing"

func TestSCN706_ContractHandoffUsesBoundedDraftPacket(t *testing.T) {
	ledger := SubmissionLedger{Status: DraftStatus, BaseCommit: workflowTestCommit}
	if err := ValidateContractHandoff(ledger, DraftExplorationPacket{BaseCommit: workflowTestCommit, QueryPurposes: []string{"impact"}}); err != nil {
		t.Fatal(err)
	}
}
func TestSCN732_PostRemovalRejectsReachableLegacyPath(t *testing.T) {
	if err := VerifyPostRemoval([]InventoryEntry{{Path: "legacy/runner.go", Classification: "legacy_only"}}, []string{"legacy/runner.go"}); err == nil {
		t.Fatal("want reachable legacy rejection")
	}
}
func TestSCN739_LocalVelaEvidenceRejectsEgressAndPayload(t *testing.T) {
	if err := ValidateLocalVelaExecution(LocalVelaExecution{Endpoint: "local_process", Storage: ".vela/graph", Purpose: "index"}, "LOCAL_ONLY_SOURCE_SENTINEL"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateLocalVelaExecution(LocalVelaExecution{Endpoint: "local_process", Storage: ".vela/graph", Purpose: "index", NonLoopbackAttempts: 1}, ""); err == nil {
		t.Fatal("want egress rejection")
	}
}
