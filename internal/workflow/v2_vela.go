package workflow

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

type V2VelaConsent string

const (
	V2VelaConsentPending  V2VelaConsent = "pending"
	V2VelaConsentGranted  V2VelaConsent = "granted"
	V2VelaConsentDeclined V2VelaConsent = "declined"
)

type V2VelaGraphStatus string

const (
	V2VelaGraphFresh       V2VelaGraphStatus = "fresh"
	V2VelaGraphMissing     V2VelaGraphStatus = "missing"
	V2VelaGraphStale       V2VelaGraphStatus = "stale"
	V2VelaGraphIncomplete  V2VelaGraphStatus = "incomplete"
	V2VelaGraphUnavailable V2VelaGraphStatus = "unavailable"
)

type V2VelaEndpoint struct {
	Endpoint             string
	GraphStorageLocation string
}

type V2VelaGraphEvidence struct {
	Available  bool
	BaseCommit string
	Complete   bool
}

type V2VelaPreflightRequest struct {
	BaseCommit     string
	Endpoint       V2VelaEndpoint
	ExistingGraph  V2VelaGraphEvidence
	Consent        V2VelaConsent
	ReindexedGraph V2VelaGraphEvidence
}

// V2VelaPreflightResult is bounded, local-only advisory evidence. Callers use
// NeedConsent to request a human decision before any extraction is dispatched.
type V2VelaPreflightResult struct {
	Status                 V2VelaGraphStatus
	SelectedBaseCommit     string
	GraphBaseCommit        string
	EndpointClassification string
	GraphStorageLocation   string
	NeedConsent            bool
	Consent                V2VelaConsent
	Uncertainty            string
}

// PreflightV2VelaGraph performs no extraction or source reads. It only
// classifies supplied local graph evidence and records whether consent is
// required before a separate index worker can be dispatched.
func PreflightV2VelaGraph(request V2VelaPreflightRequest) (V2VelaPreflightResult, error) {
	if !isFullCommitID(request.BaseCommit) {
		return V2VelaPreflightResult{}, errors.New("Vela preflight requires the selected full immutable base commit")
	}
	classification, err := classifyV2LocalVelaEndpoint(request.Endpoint.Endpoint)
	if err != nil {
		return V2VelaPreflightResult{
			Status:             V2VelaGraphUnavailable,
			SelectedBaseCommit: strings.ToLower(request.BaseCommit),
			Consent:            request.Consent,
			Uncertainty:        "Vela endpoint is non-local and was rejected before graph or source input was read",
		}, nil
	}
	result := V2VelaPreflightResult{
		SelectedBaseCommit:     strings.ToLower(request.BaseCommit),
		EndpointClassification: classification,
		GraphStorageLocation:   request.Endpoint.GraphStorageLocation,
		Consent:                request.Consent,
	}
	if isFreshV2Graph(request.ExistingGraph, result.SelectedBaseCommit) {
		result.Status = V2VelaGraphFresh
		result.GraphBaseCommit = strings.ToLower(request.ExistingGraph.BaseCommit)
		return result, nil
	}

	result.Status, result.Uncertainty = classifyV2GraphProblem(request.ExistingGraph, result.SelectedBaseCommit)
	result.GraphBaseCommit = strings.ToLower(request.ExistingGraph.BaseCommit)
	if request.Consent != V2VelaConsentGranted {
		result.NeedConsent = request.Consent == V2VelaConsentPending
		if request.Consent == V2VelaConsentDeclined {
			result.Uncertainty += "; re-indexing was declined, so Draft must retain this uncertainty"
		}
		return result, nil
	}

	if isFreshV2Graph(request.ReindexedGraph, result.SelectedBaseCommit) {
		result.Status = V2VelaGraphFresh
		result.GraphBaseCommit = strings.ToLower(request.ReindexedGraph.BaseCommit)
		result.Uncertainty = ""
		return result, nil
	}
	result.Status, result.Uncertainty = classifyV2GraphProblem(request.ReindexedGraph, result.SelectedBaseCommit)
	result.GraphBaseCommit = strings.ToLower(request.ReindexedGraph.BaseCommit)
	result.Uncertainty = "consented re-indexing did not produce a complete graph: " + result.Uncertainty
	return result, nil
}

func classifyV2LocalVelaEndpoint(endpoint string) (string, error) {
	if endpoint == "" {
		return "local_process", nil
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("parse Vela endpoint: %w", err)
	}
	if parsed.Scheme == "unix" && parsed.Host == "" && strings.HasPrefix(parsed.Path, "/") {
		return "unix_socket", nil
	}
	if (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Hostname() != "" {
		if address := net.ParseIP(parsed.Hostname()); address != nil && address.IsLoopback() {
			return "loopback_endpoint", nil
		}
	}
	return "", errors.New("Vela endpoint is not a local process, Unix socket, or loopback endpoint")
}

func isFreshV2Graph(graph V2VelaGraphEvidence, baseCommit string) bool {
	return graph.Available && graph.Complete && strings.EqualFold(graph.BaseCommit, baseCommit)
}

func classifyV2GraphProblem(graph V2VelaGraphEvidence, baseCommit string) (V2VelaGraphStatus, string) {
	if !graph.Available {
		return V2VelaGraphMissing, "no local graph is available"
	}
	if !strings.EqualFold(graph.BaseCommit, baseCommit) {
		return V2VelaGraphStale, "local graph does not identify the selected base commit"
	}
	return V2VelaGraphIncomplete, "local graph reports incomplete repository coverage"
}
