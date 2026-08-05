package workflow

import "testing"

func TestSCN702_FreshGraphBuildsBoundedExplorationPacket(t *testing.T) {
	// REQ-108, REQ-109, REQ-117 -> SCN-702
	preflight := VelaPreflightResult{Status: VelaGraphFresh, SelectedBaseCommit: workflowTestCommit}
	packet, err := BuildDraftExplorationPacket(DraftExplorationRequest{
		BaseCommit: workflowTestCommit, Preflight: preflight,
		Queries:      []VelaQueryRecord{{Purpose: "identify affected modules"}},
		Affected:     []ExplorationFinding{{Subject: "internal/workflow", Confidence: "high"}},
		Risks:        []ExplorationFinding{{Subject: "lifecycle authority", Confidence: "medium"}},
		Constraints:  []ExplorationFinding{{Subject: "OpenCode only", Confidence: "high"}},
		EvidenceRefs: []string{"evidence/explore.json"},
	})
	if err != nil {
		t.Fatalf("BuildDraftExplorationPacket() error = %v", err)
	}
	if packet.BaseCommit != workflowTestCommit || packet.GraphStatus != VelaGraphFresh || len(packet.QueryPurposes) != 1 || len(packet.Affected) != 1 {
		t.Fatalf("packet = %#v, want bounded fresh packet", packet)
	}
}

func TestSCN704_StaleGraphFindingsAreExplicitlyUncertain(t *testing.T) {
	// REQ-108, REQ-109, REQ-117 -> SCN-704
	packet, err := BuildDraftExplorationPacket(DraftExplorationRequest{BaseCommit: workflowTestCommit, Preflight: VelaPreflightResult{Status: VelaGraphStale, SelectedBaseCommit: workflowTestCommit, Uncertainty: "stale graph"}, Affected: []ExplorationFinding{{Subject: "internal/workflow", Confidence: "high"}}})
	if err != nil {
		t.Fatalf("BuildDraftExplorationPacket() error = %v", err)
	}
	if packet.Affected[0].Confidence != "uncertain" || packet.Uncertainty == "" {
		t.Fatalf("stale packet = %#v, want explicit uncertainty", packet)
	}
}

func TestSCN729_ExplorationCapsBoundsWithoutAdditionalQuery(t *testing.T) {
	// REQ-109, REQ-117 -> SCN-729
	risks := make([]ExplorationFinding, 11)
	for index := range risks {
		risks[index] = ExplorationFinding{Subject: "risk", Confidence: "low"}
	}
	queries := make([]VelaQueryRecord, 5)
	for index := range queries {
		queries[index] = VelaQueryRecord{Purpose: "bounded question"}
	}
	packet, err := BuildDraftExplorationPacket(DraftExplorationRequest{BaseCommit: workflowTestCommit, Preflight: VelaPreflightResult{Status: VelaGraphFresh, SelectedBaseCommit: workflowTestCommit}, Queries: queries, Risks: risks})
	if err != nil {
		t.Fatalf("BuildDraftExplorationPacket() error = %v", err)
	}
	if len(packet.QueryPurposes) != 5 || len(packet.Risks) != 10 || packet.Uncertainty == "" {
		t.Fatalf("capped packet = %#v, want five queries, ten risks, and uncertainty", packet)
	}
}
