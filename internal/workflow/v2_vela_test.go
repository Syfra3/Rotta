package workflow

import "testing"

func TestSCN702_FreshLocalGraphProducesFreshPreflight(t *testing.T) {
	// REQ-108, REQ-109, REQ-117 -> SCN-702
	result, err := PreflightV2VelaGraph(V2VelaPreflightRequest{
		BaseCommit:    v2TestCommit,
		Endpoint:      V2VelaEndpoint{Endpoint: "http://127.0.0.1:8080", GraphStorageLocation: ".vela/graph.db"},
		ExistingGraph: V2VelaGraphEvidence{Available: true, Complete: true, BaseCommit: v2TestCommit},
	})
	if err != nil {
		t.Fatalf("PreflightV2VelaGraph() error = %v", err)
	}
	if result.Status != V2VelaGraphFresh || result.NeedConsent || result.EndpointClassification != "loopback_endpoint" {
		t.Fatalf("preflight result = %#v, want fresh local graph without consent", result)
	}
}

func TestSCN703_MissingGraphNeedsConsentBeforeIndexing(t *testing.T) {
	// REQ-108, REQ-109 -> SCN-703
	pending, err := PreflightV2VelaGraph(V2VelaPreflightRequest{BaseCommit: v2TestCommit, Endpoint: V2VelaEndpoint{}, Consent: V2VelaConsentPending})
	if err != nil {
		t.Fatalf("PreflightV2VelaGraph() error = %v", err)
	}
	if pending.Status != V2VelaGraphMissing || !pending.NeedConsent {
		t.Fatalf("pending preflight = %#v, want missing graph awaiting consent", pending)
	}
	indexed, err := PreflightV2VelaGraph(V2VelaPreflightRequest{BaseCommit: v2TestCommit, Endpoint: V2VelaEndpoint{}, Consent: V2VelaConsentGranted, ReindexedGraph: V2VelaGraphEvidence{Available: true, Complete: true, BaseCommit: v2TestCommit}})
	if err != nil || indexed.Status != V2VelaGraphFresh || indexed.NeedConsent {
		t.Fatalf("consented preflight = %#v, %v; want fresh graph", indexed, err)
	}
}

func TestSCN704_StaleGraphRemainsVisibleWhenReindexingDeclined(t *testing.T) {
	// REQ-108, REQ-109, REQ-117 -> SCN-704
	result, err := PreflightV2VelaGraph(V2VelaPreflightRequest{BaseCommit: v2TestCommit, Endpoint: V2VelaEndpoint{}, ExistingGraph: V2VelaGraphEvidence{Available: true, Complete: true, BaseCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, Consent: V2VelaConsentDeclined})
	if err != nil {
		t.Fatalf("PreflightV2VelaGraph() error = %v", err)
	}
	if result.Status != V2VelaGraphStale || result.NeedConsent || result.Uncertainty == "" {
		t.Fatalf("declined preflight = %#v, want visible stale uncertainty", result)
	}
}

func TestSCN705_IncompleteGraphIsNotFreshAfterConsentedIndexing(t *testing.T) {
	// REQ-108, REQ-109 -> SCN-705
	result, err := PreflightV2VelaGraph(V2VelaPreflightRequest{BaseCommit: v2TestCommit, Endpoint: V2VelaEndpoint{}, ExistingGraph: V2VelaGraphEvidence{Available: true, Complete: false, BaseCommit: v2TestCommit}, Consent: V2VelaConsentGranted, ReindexedGraph: V2VelaGraphEvidence{Available: true, Complete: false, BaseCommit: v2TestCommit}})
	if err != nil {
		t.Fatalf("PreflightV2VelaGraph() error = %v", err)
	}
	if result.Status != V2VelaGraphIncomplete || result.Uncertainty == "" {
		t.Fatalf("incomplete preflight = %#v, want incomplete uncertainty", result)
	}
}

func TestSCN740_RemoteVelaEndpointIsRejectedBeforeAnyIndexWork(t *testing.T) {
	// REQ-116, REQ-108 -> SCN-740
	result, err := PreflightV2VelaGraph(V2VelaPreflightRequest{BaseCommit: v2TestCommit, Endpoint: V2VelaEndpoint{Endpoint: "https://vela.example.test"}, Consent: V2VelaConsentGranted})
	if err != nil {
		t.Fatalf("PreflightV2VelaGraph() error = %v", err)
	}
	if result.Status != V2VelaGraphUnavailable || result.Uncertainty == "" {
		t.Fatalf("remote preflight = %#v, want rejected unavailable uncertainty", result)
	}
}
