package workflow

import (
	"errors"
	"strings"
)

const (
	MaximumDraftQueries      = 5
	MaximumAffectedModules   = 10
	MaximumExplorationRisks  = 10
	MaximumArchitectureRules = 5
)

type VelaQueryRecord struct {
	Purpose string
}

type ExplorationFinding struct {
	Subject    string
	Confidence string
}

type DraftExplorationRequest struct {
	BaseCommit   string
	Preflight    VelaPreflightResult
	Queries      []VelaQueryRecord
	Affected     []ExplorationFinding
	Risks        []ExplorationFinding
	Constraints  []ExplorationFinding
	EvidenceRefs []string
}

// DraftExplorationPacket is intentionally compact: it records useful graph
// findings and uncertainty, not raw graph output or lifecycle authority.
type DraftExplorationPacket struct {
	BaseCommit    string
	GraphStatus   VelaGraphStatus
	QueryPurposes []string
	Affected      []ExplorationFinding
	Risks         []ExplorationFinding
	Constraints   []ExplorationFinding
	EvidenceRefs  []string
	Uncertainty   string
}

func BuildDraftExplorationPacket(request DraftExplorationRequest) (DraftExplorationPacket, error) {
	if !isFullCommitID(request.BaseCommit) || !strings.EqualFold(request.BaseCommit, request.Preflight.SelectedBaseCommit) {
		return DraftExplorationPacket{}, errors.New("Draft exploration requires preflight evidence for the selected base commit")
	}
	if len(request.Queries) > MaximumDraftQueries {
		return DraftExplorationPacket{}, errors.New("Draft exploration exceeds the maximum of five Vela queries")
	}
	packet := DraftExplorationPacket{
		BaseCommit:   strings.ToLower(request.BaseCommit),
		GraphStatus:  request.Preflight.Status,
		Affected:     capFindings(request.Affected, MaximumAffectedModules),
		Risks:        capFindings(request.Risks, MaximumExplorationRisks),
		Constraints:  capFindings(request.Constraints, MaximumArchitectureRules),
		EvidenceRefs: append([]string(nil), request.EvidenceRefs...),
		Uncertainty:  request.Preflight.Uncertainty,
	}
	for _, query := range request.Queries {
		if strings.TrimSpace(query.Purpose) == "" {
			return DraftExplorationPacket{}, errors.New("Draft exploration query purpose is required")
		}
		packet.QueryPurposes = append(packet.QueryPurposes, query.Purpose)
	}
	if len(request.Affected) > len(packet.Affected) || len(request.Risks) > len(packet.Risks) || len(request.Constraints) > len(packet.Constraints) {
		packet.Uncertainty = appendUncertainty(packet.Uncertainty, "exploration scope was capped at the approved packet bounds")
	}
	if packet.GraphStatus != VelaGraphFresh {
		packet.Uncertainty = appendUncertainty(packet.Uncertainty, "graph findings are advisory and may not cover unknown structural areas")
		for index := range packet.Affected {
			packet.Affected[index].Confidence = "uncertain"
		}
	}
	return packet, nil
}

func capFindings(findings []ExplorationFinding, maximum int) []ExplorationFinding {
	if len(findings) > maximum {
		findings = findings[:maximum]
	}
	return append([]ExplorationFinding(nil), findings...)
}

func appendUncertainty(existing, addition string) string {
	if existing == "" {
		return addition
	}
	return existing + "; " + addition
}
