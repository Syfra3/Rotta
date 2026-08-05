package workflow

import "testing"

func TestSCN702_FreshLocalGraphProducesFreshPreflight(t *testing.T) {
	// REQ-108, REQ-109, REQ-117 -> SCN-702
	result, err := PreflightVelaGraph(VelaPreflightRequest{
		BaseCommit:    workflowTestCommit,
		Endpoint:      VelaEndpoint{Endpoint: "http://127.0.0.1:8080", GraphStorageLocation: ".vela/graph.db"},
		ExistingGraph: VelaGraphEvidence{Available: true, Complete: true, BaseCommit: workflowTestCommit},
	})
	if err != nil {
		t.Fatalf("PreflightVelaGraph() error = %v", err)
	}
	if result.Status != VelaGraphFresh || result.NeedConsent || result.EndpointClassification != "loopback_endpoint" {
		t.Fatalf("preflight result = %#v, want fresh local graph without consent", result)
	}
}

func TestSCN703_MissingGraphNeedsConsentBeforeIndexing(t *testing.T) {
	// REQ-108, REQ-109 -> SCN-703
	pending, err := PreflightVelaGraph(VelaPreflightRequest{BaseCommit: workflowTestCommit, Endpoint: VelaEndpoint{}, Consent: VelaConsentPending})
	if err != nil {
		t.Fatalf("PreflightVelaGraph() error = %v", err)
	}
	if pending.Status != VelaGraphMissing || !pending.NeedConsent {
		t.Fatalf("pending preflight = %#v, want missing graph awaiting consent", pending)
	}
	indexed, err := PreflightVelaGraph(VelaPreflightRequest{BaseCommit: workflowTestCommit, Endpoint: VelaEndpoint{}, Consent: VelaConsentGranted, ReindexedGraph: VelaGraphEvidence{Available: true, Complete: true, BaseCommit: workflowTestCommit}})
	if err != nil || indexed.Status != VelaGraphFresh || indexed.NeedConsent {
		t.Fatalf("consented preflight = %#v, %v; want fresh graph", indexed, err)
	}
}

func TestSCN704_StaleGraphRemainsVisibleWhenReindexingDeclined(t *testing.T) {
	// REQ-108, REQ-109, REQ-117 -> SCN-704
	result, err := PreflightVelaGraph(VelaPreflightRequest{BaseCommit: workflowTestCommit, Endpoint: VelaEndpoint{}, ExistingGraph: VelaGraphEvidence{Available: true, Complete: true, BaseCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, Consent: VelaConsentDeclined})
	if err != nil {
		t.Fatalf("PreflightVelaGraph() error = %v", err)
	}
	if result.Status != VelaGraphStale || result.NeedConsent || result.Uncertainty == "" {
		t.Fatalf("declined preflight = %#v, want visible stale uncertainty", result)
	}
}

func TestSCN705_IncompleteGraphIsNotFreshAfterConsentedIndexing(t *testing.T) {
	// REQ-108, REQ-109 -> SCN-705
	result, err := PreflightVelaGraph(VelaPreflightRequest{BaseCommit: workflowTestCommit, Endpoint: VelaEndpoint{}, ExistingGraph: VelaGraphEvidence{Available: true, Complete: false, BaseCommit: workflowTestCommit}, Consent: VelaConsentGranted, ReindexedGraph: VelaGraphEvidence{Available: true, Complete: false, BaseCommit: workflowTestCommit}})
	if err != nil {
		t.Fatalf("PreflightVelaGraph() error = %v", err)
	}
	if result.Status != VelaGraphIncomplete || result.Uncertainty == "" {
		t.Fatalf("incomplete preflight = %#v, want incomplete uncertainty", result)
	}
}

func TestSCN740_RemoteVelaEndpointIsRejectedBeforeAnyIndexWork(t *testing.T) {
	// REQ-116, REQ-108 -> SCN-740
	result, err := PreflightVelaGraph(VelaPreflightRequest{BaseCommit: workflowTestCommit, Endpoint: VelaEndpoint{Endpoint: "https://vela.example.test"}, Consent: VelaConsentGranted})
	if err != nil {
		t.Fatalf("PreflightVelaGraph() error = %v", err)
	}
	if result.Status != VelaGraphUnavailable || result.Uncertainty == "" {
		t.Fatalf("remote preflight = %#v, want rejected unavailable uncertainty", result)
	}
}
