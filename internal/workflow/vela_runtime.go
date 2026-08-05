package workflow

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

const maxVelaEvidenceBytes = 8192

// VelaInvocation is bounded advisory evidence from one local CLI invocation.
type VelaInvocation struct {
	Purpose       string
	BaseCommit    string
	Executable    string
	GraphLocation string
	Succeeded     bool
	Output        string
}

// RunLocalVela invokes only the locally resolved vela executable. Indexing is
// deliberately refused until the caller has recorded explicit human consent.
func RunLocalVela(ctx context.Context, repoRoot, baseCommit, purpose, question string, indexConsent bool) (VelaInvocation, error) {
	if !isFullCommitID(baseCommit) {
		return VelaInvocation{}, errors.New("Vela requires a full immutable base commit")
	}
	if purpose != "preflight" && purpose != "index" && purpose != "query" {
		return VelaInvocation{}, fmt.Errorf("unsupported local Vela purpose %q", purpose)
	}
	if purpose == "query" && strings.TrimSpace(question) == "" {
		return VelaInvocation{}, errors.New("local Vela query requires a structural question")
	}
	if purpose == "index" && !indexConsent {
		return VelaInvocation{}, errors.New("local Vela indexing requires explicit consent")
	}

	executable, err := exec.LookPath("vela")
	if err != nil {
		return VelaInvocation{Purpose: purpose, BaseCommit: strings.ToLower(baseCommit), GraphLocation: filepath.Join(repoRoot, ".vela", "graph.json")}, fmt.Errorf("locate local vela executable: %w", err)
	}
	graph := filepath.Join(repoRoot, ".vela", "graph.json")
	arguments := []string{"status", "--graph", graph, "--json"}
	switch purpose {
	case "index":
		arguments = []string{"build", repoRoot, "--out-dir", filepath.Dir(graph)}
	case "query":
		arguments = []string{"explore", question, "--graph", graph, "--limit", "5"}
	}
	command := exec.CommandContext(ctx, executable, arguments...)
	command.Dir = repoRoot
	output, err := command.CombinedOutput()
	invocation := VelaInvocation{
		Purpose:       purpose,
		BaseCommit:    strings.ToLower(baseCommit),
		Executable:    executable,
		GraphLocation: graph,
		Succeeded:     err == nil,
		Output:        boundedCommandOutput(output),
	}
	if err != nil {
		return invocation, fmt.Errorf("local vela %s: %w", purpose, err)
	}
	return invocation, nil
}

func boundedCommandOutput(output []byte) string {
	result := strings.TrimSpace(string(output))
	if len(result) > maxVelaEvidenceBytes {
		return result[:maxVelaEvidenceBytes]
	}
	return result
}
