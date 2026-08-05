package workflow

import (
	"errors"
	"strings"
)

const (
	v2MaximumDraftQueries      = 5
	v2MaximumAffectedModules   = 10
	v2MaximumExplorationRisks  = 10
	v2MaximumArchitectureRules = 5
)

type V2VelaQueryRecord struct {
	Purpose string
}

type V2ExplorationFinding struct {
	Subject    string
	Confidence string
}

type V2DraftExplorationRequest struct {
	BaseCommit   string
	Preflight    V2VelaPreflightResult
	Queries      []V2VelaQueryRecord
	Affected     []V2ExplorationFinding
	Risks        []V2ExplorationFinding
	Constraints  []V2ExplorationFinding
	EvidenceRefs []string
}

// V2DraftExplorationPacket is intentionally compact: it records useful graph
// findings and uncertainty, not raw graph output or lifecycle authority.
type V2DraftExplorationPacket struct {
	BaseCommit    string
	GraphStatus   V2VelaGraphStatus
	QueryPurposes []string
	Affected      []V2ExplorationFinding
	Risks         []V2ExplorationFinding
	Constraints   []V2ExplorationFinding
	EvidenceRefs  []string
	Uncertainty   string
}

func BuildV2DraftExplorationPacket(request V2DraftExplorationRequest) (V2DraftExplorationPacket, error) {
	if !isFullCommitID(request.BaseCommit) || !strings.EqualFold(request.BaseCommit, request.Preflight.SelectedBaseCommit) {
		return V2DraftExplorationPacket{}, errors.New("Draft exploration requires preflight evidence for the selected base commit")
	}
	if len(request.Queries) > v2MaximumDraftQueries {
		return V2DraftExplorationPacket{}, errors.New("Draft exploration exceeds the maximum of five Vela queries")
	}
	packet := V2DraftExplorationPacket{
		BaseCommit:   strings.ToLower(request.BaseCommit),
		GraphStatus:  request.Preflight.Status,
		Affected:     capV2Findings(request.Affected, v2MaximumAffectedModules),
		Risks:        capV2Findings(request.Risks, v2MaximumExplorationRisks),
		Constraints:  capV2Findings(request.Constraints, v2MaximumArchitectureRules),
		EvidenceRefs: append([]string(nil), request.EvidenceRefs...),
		Uncertainty:  request.Preflight.Uncertainty,
	}
	for _, query := range request.Queries {
		if strings.TrimSpace(query.Purpose) == "" {
			return V2DraftExplorationPacket{}, errors.New("Draft exploration query purpose is required")
		}
		packet.QueryPurposes = append(packet.QueryPurposes, query.Purpose)
	}
	if len(request.Affected) > len(packet.Affected) || len(request.Risks) > len(packet.Risks) || len(request.Constraints) > len(packet.Constraints) {
		packet.Uncertainty = appendV2Uncertainty(packet.Uncertainty, "exploration scope was capped at the approved packet bounds")
	}
	if packet.GraphStatus != V2VelaGraphFresh {
		packet.Uncertainty = appendV2Uncertainty(packet.Uncertainty, "graph findings are advisory and may not cover unknown structural areas")
		for index := range packet.Affected {
			packet.Affected[index].Confidence = "uncertain"
		}
	}
	return packet, nil
}

func capV2Findings(findings []V2ExplorationFinding, maximum int) []V2ExplorationFinding {
	if len(findings) > maximum {
		findings = findings[:maximum]
	}
	return append([]V2ExplorationFinding(nil), findings...)
}

func appendV2Uncertainty(existing, addition string) string {
	if existing == "" {
		return addition
	}
	return existing + "; " + addition
}
