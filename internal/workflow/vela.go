package workflow

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

type VelaConsent string

const (
	VelaConsentPending  VelaConsent = "pending"
	VelaConsentGranted  VelaConsent = "granted"
	VelaConsentDeclined VelaConsent = "declined"
)

type VelaGraphStatus string

const (
	VelaGraphFresh       VelaGraphStatus = "fresh"
	VelaGraphMissing     VelaGraphStatus = "missing"
	VelaGraphStale       VelaGraphStatus = "stale"
	VelaGraphIncomplete  VelaGraphStatus = "incomplete"
	VelaGraphUnavailable VelaGraphStatus = "unavailable"
)

type VelaEndpoint struct {
	Endpoint             string
	GraphStorageLocation string
}

type VelaGraphEvidence struct {
	Available  bool
	BaseCommit string
	Complete   bool
}

type VelaPreflightRequest struct {
	BaseCommit     string
	Endpoint       VelaEndpoint
	ExistingGraph  VelaGraphEvidence
	Consent        VelaConsent
	ReindexedGraph VelaGraphEvidence
}

// VelaPreflightResult is bounded, local-only advisory evidence. Callers use
// NeedConsent to request a human decision before any extraction is dispatched.
type VelaPreflightResult struct {
	Status                 VelaGraphStatus
	SelectedBaseCommit     string
	GraphBaseCommit        string
	EndpointClassification string
	GraphStorageLocation   string
	NeedConsent            bool
	Consent                VelaConsent
	Uncertainty            string
}

// PreflightVelaGraph performs no extraction or source reads. It only
// classifies supplied local graph evidence and records whether consent is
// required before a separate index worker can be dispatched.
func PreflightVelaGraph(request VelaPreflightRequest) (VelaPreflightResult, error) {
	if !isFullCommitID(request.BaseCommit) {
		return VelaPreflightResult{}, errors.New("Vela preflight requires the selected full immutable base commit")
	}
	classification, err := classifyLocalVelaEndpoint(request.Endpoint.Endpoint)
	if err != nil {
		return VelaPreflightResult{
			Status:             VelaGraphUnavailable,
			SelectedBaseCommit: strings.ToLower(request.BaseCommit),
			Consent:            request.Consent,
			Uncertainty:        "Vela endpoint is non-local and was rejected before graph or source input was read",
		}, nil
	}
	result := VelaPreflightResult{
		SelectedBaseCommit:     strings.ToLower(request.BaseCommit),
		EndpointClassification: classification,
		GraphStorageLocation:   request.Endpoint.GraphStorageLocation,
		Consent:                request.Consent,
	}
	if isFreshGraph(request.ExistingGraph, result.SelectedBaseCommit) {
		result.Status = VelaGraphFresh
		result.GraphBaseCommit = strings.ToLower(request.ExistingGraph.BaseCommit)
		return result, nil
	}

	result.Status, result.Uncertainty = classifyGraphProblem(request.ExistingGraph, result.SelectedBaseCommit)
	result.GraphBaseCommit = strings.ToLower(request.ExistingGraph.BaseCommit)
	if request.Consent != VelaConsentGranted {
		result.NeedConsent = request.Consent == VelaConsentPending
		if request.Consent == VelaConsentDeclined {
			result.Uncertainty += "; re-indexing was declined, so Draft must retain this uncertainty"
		}
		return result, nil
	}

	if isFreshGraph(request.ReindexedGraph, result.SelectedBaseCommit) {
		result.Status = VelaGraphFresh
		result.GraphBaseCommit = strings.ToLower(request.ReindexedGraph.BaseCommit)
		result.Uncertainty = ""
		return result, nil
	}
	result.Status, result.Uncertainty = classifyGraphProblem(request.ReindexedGraph, result.SelectedBaseCommit)
	result.GraphBaseCommit = strings.ToLower(request.ReindexedGraph.BaseCommit)
	result.Uncertainty = "consented re-indexing did not produce a complete graph: " + result.Uncertainty
	return result, nil
}

func classifyLocalVelaEndpoint(endpoint string) (string, error) {
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

func isFreshGraph(graph VelaGraphEvidence, baseCommit string) bool {
	return graph.Available && graph.Complete && strings.EqualFold(graph.BaseCommit, baseCommit)
}

func classifyGraphProblem(graph VelaGraphEvidence, baseCommit string) (VelaGraphStatus, string) {
	if !graph.Available {
		return VelaGraphMissing, "no local graph is available"
	}
	if !strings.EqualFold(graph.BaseCommit, baseCommit) {
		return VelaGraphStale, "local graph does not identify the selected base commit"
	}
	return VelaGraphIncomplete, "local graph reports incomplete repository coverage"
}
