package workflow

import "testing"

func TestSCN702_FreshGraphBuildsBoundedExplorationPacket(t *testing.T) {
	// REQ-108, REQ-109, REQ-117 -> SCN-702
	preflight := V2VelaPreflightResult{Status: V2VelaGraphFresh, SelectedBaseCommit: v2TestCommit}
	packet, err := BuildV2DraftExplorationPacket(V2DraftExplorationRequest{
		BaseCommit: v2TestCommit, Preflight: preflight,
		Queries:      []V2VelaQueryRecord{{Purpose: "identify affected modules"}},
		Affected:     []V2ExplorationFinding{{Subject: "internal/workflow", Confidence: "high"}},
		Risks:        []V2ExplorationFinding{{Subject: "lifecycle authority", Confidence: "medium"}},
		Constraints:  []V2ExplorationFinding{{Subject: "OpenCode only", Confidence: "high"}},
		EvidenceRefs: []string{"evidence/explore.json"},
	})
	if err != nil {
		t.Fatalf("BuildV2DraftExplorationPacket() error = %v", err)
	}
	if packet.BaseCommit != v2TestCommit || packet.GraphStatus != V2VelaGraphFresh || len(packet.QueryPurposes) != 1 || len(packet.Affected) != 1 {
		t.Fatalf("packet = %#v, want bounded fresh packet", packet)
	}
}

func TestSCN704_StaleGraphFindingsAreExplicitlyUncertain(t *testing.T) {
	// REQ-108, REQ-109, REQ-117 -> SCN-704
	packet, err := BuildV2DraftExplorationPacket(V2DraftExplorationRequest{BaseCommit: v2TestCommit, Preflight: V2VelaPreflightResult{Status: V2VelaGraphStale, SelectedBaseCommit: v2TestCommit, Uncertainty: "stale graph"}, Affected: []V2ExplorationFinding{{Subject: "internal/workflow", Confidence: "high"}}})
	if err != nil {
		t.Fatalf("BuildV2DraftExplorationPacket() error = %v", err)
	}
	if packet.Affected[0].Confidence != "uncertain" || packet.Uncertainty == "" {
		t.Fatalf("stale packet = %#v, want explicit uncertainty", packet)
	}
}

func TestSCN729_ExplorationCapsBoundsWithoutAdditionalQuery(t *testing.T) {
	// REQ-109, REQ-117 -> SCN-729
	risks := make([]V2ExplorationFinding, 11)
	for index := range risks {
		risks[index] = V2ExplorationFinding{Subject: "risk", Confidence: "low"}
	}
	queries := make([]V2VelaQueryRecord, 5)
	for index := range queries {
		queries[index] = V2VelaQueryRecord{Purpose: "bounded question"}
	}
	packet, err := BuildV2DraftExplorationPacket(V2DraftExplorationRequest{BaseCommit: v2TestCommit, Preflight: V2VelaPreflightResult{Status: V2VelaGraphFresh, SelectedBaseCommit: v2TestCommit}, Queries: queries, Risks: risks})
	if err != nil {
		t.Fatalf("BuildV2DraftExplorationPacket() error = %v", err)
	}
	if len(packet.QueryPurposes) != 5 || len(packet.Risks) != 10 || packet.Uncertainty == "" {
		t.Fatalf("capped packet = %#v, want five queries, ten risks, and uncertainty", packet)
	}
}
