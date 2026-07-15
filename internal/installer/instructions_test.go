package installer

import (
	"strings"
	"testing"

	"github.com/Syfra3/Rotta/assets"
)

func TestIntegrationInstructionsWhenAncoraAndVelaDisabled(t *testing.T) {
	got := integrationInstructions(Options{})

	for _, want := range []string{
		"### Ancora Memory Disabled",
		"Do not call `ancora_*` tools",
		"### Vela Graph Intelligence Disabled",
		"Do not call `vela_*` tools",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("integration instructions missing %q:\n%s", want, got)
		}
	}
}

func TestIntegrationInstructionsWhenAncoraEnabledAndVelaDisabled(t *testing.T) {
	got := integrationInstructions(Options{SetupAncora: true})

	assertContainsAll(t, got, []string{
		"### Ancora Memory Enabled",
		"ancora_context",
		"ancora_save",
		"### Vela Graph Intelligence Disabled",
		"Do not call `vela_*` tools",
	})
	assertNotContains(t, got, "### Vela Graph Intelligence Enabled")
}

func TestIntegrationInstructionsWhenAncoraDisabledAndVelaEnabled(t *testing.T) {
	got := integrationInstructions(Options{SetupVela: true})

	assertContainsAll(t, got, []string{
		"### Ancora Memory Disabled",
		"Do not call `ancora_*` tools",
		"### Vela Graph Intelligence Enabled",
		"Vela may be available as standalone `vela_*` MCP tools",
		"non-blocking background graph refresh",
		"cached graph may be used while refresh runs",
		"vela update",
		"vela build",
		"foreground refresh",
		"provenance",
		"confidence",
	})
	assertContainsAll(t, got, velaStructuralQueryEnforcementStrings())
	assertNotContains(t, got, "visible start/end/fallback feedback")
	assertNotContains(t, got, "Ancora remains the primary MCP surface")
}

func TestIntegrationInstructionsWhenAncoraAndVelaEnabled(t *testing.T) {
	got := integrationInstructions(Options{SetupAncora: true, SetupVela: true})

	assertContainsAll(t, got, []string{
		"### Ancora Memory Enabled",
		"### Vela Graph Intelligence Enabled",
		"Ancora remains the primary MCP surface",
		"non-blocking background graph refresh",
		"cached graph may be used while refresh runs",
		"foreground refresh",
		"provenance",
		"confidence",
	})
	assertContainsAll(t, got, velaStructuralQueryEnforcementStrings())
}

func TestReadRenderedAssetAppendsDisabledIntegrationInstructions(t *testing.T) {
	data, err := readRenderedAsset("agents/rotta-orchestrator.md", Options{})
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)

	assertContainsAll(t, got, []string{
		"### Ancora Memory Disabled",
		"Do not call `ancora_*` tools",
		"### Vela Graph Intelligence Disabled",
		"Do not call `vela_*` tools",
		"state_ref: \"specs/hard_spec.md + features/*.feature\"",
	})
	assertNotContains(t, got, "ancora_topic")
	assertNotContains(t, got, "ancora_context")
	assertNotContains(t, got, "ancora_save:")
}

func TestReadRenderedAssetAppendsEnabledIntegrationInstructions(t *testing.T) {
	data, err := readRenderedAsset("agents/rotta-orchestrator.md", Options{SetupAncora: true, SetupVela: true})
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)

	assertContainsAll(t, got, []string{
		"### Ancora Memory Enabled",
		"ancora_context",
		"ancora_save",
		"### Vela Graph Intelligence Enabled",
		"Ancora remains the primary MCP surface",
		"non-blocking background graph refresh",
	})
	assertContainsAll(t, got, velaStructuralQueryEnforcementStrings())
}

func TestCanonicalWorkflowInstructionsEnforceCleanTDDTaskBoundaries(t *testing.T) {
	got := canonicalWorkflowInstructions()

	assertContainsAll(t, got, []string{
		"Every TDD scenario task starts clean",
		"git status --short",
		"update the task checklist with completed, remaining, and next work",
		"checkpoint or clean the task diff before starting another scenario",
		"Approved spec/feature contracts are tracked durable artifacts",
	})
}

func TestVelaInstructionsEnforceExactSubjectStructuralQueryWorkflow(t *testing.T) {
	got := integrationInstructions(Options{SetupVela: true})

	assertContainsAll(t, got, velaStructuralQueryEnforcementStrings())
}

func TestVelaBinCandidatesIncludesLinuxbrew(t *testing.T) {
	got := strings.Join(velaBinCandidates(), "\n")
	assertContainsAll(t, got, []string{
		"/opt/homebrew/bin/vela",
		"/home/linuxbrew/.linuxbrew/bin/vela",
		"/usr/local/bin/vela",
	})
}

func TestVelaResultAddFileDeduplicatesGraphDB(t *testing.T) {
	result := &VelaResult{}
	result.addFiles("/project/.vela/graph.db", "/home/.claude/vela-mcp.json")
	result.addFiles("/project/.vela/graph.db", "/home/.config/opencode/opencode.json")

	if countOccurrences(result.Files, "/project/.vela/graph.db") != 1 {
		t.Fatalf("expected graph db once, got %#v", result.Files)
	}
}

func assertContainsAll(t *testing.T, got string, wants []string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q:\n%s", want, got)
		}
	}
}

func assertNotContains(t *testing.T, got, unwanted string) {
	t.Helper()
	if strings.Contains(got, unwanted) {
		t.Fatalf("unexpected %q:\n%s", unwanted, got)
	}
}

func velaStructuralQueryEnforcementStrings() []string {
	return []string{
		"For structural dependency, reverse-dependency, impact, path, ownership, or architecture questions, run `vela_status` first",
		"For ranking or hotspot structural questions",
		"use compact `vela_rank` or `vela_hotspots` first when available",
		"Do not manually rank candidates by repeatedly dumping full edges",
		"at most 5 graph calls total for one ranking/hotspot question",
		"call `vela_module_summary` or `vela_explain` only for top candidates",
		"Full edge dumps require an explicit user request",
		"If compact tools are unavailable, use a bounded fallback",
		"Use `vela_lookup` to resolve concrete subjects before specialized graph calls",
		"Prefer exact file, module, controller, use-case, service, DTO, route handler, endpoint, or API-client subjects",
		"If symbol-level `vela_dependencies` or `vela_reverse_dependencies` returns `(none)` or an empty result and a containing file node exists, retry at file level",
		"`vela_explore` is routing/discovery only, not final proof when ambiguous",
		"Launch an exploration subagent for structural questions only after the exact Vela workflow fails",
		"Before launching that subagent, state the specific Vela insufficiency or gap",
		"Final answers must report Vela confidence and gaps",
		"graph-call budget use",
		"Vela is advisory only",
	}
}

func TestVelaInstructionsEnforceCompactRankingBudget(t *testing.T) {
	got := integrationInstructions(Options{SetupVela: true})

	assertContainsAll(t, got, []string{
		"use compact `vela_rank` or `vela_hotspots` first when available",
		"limit 10 candidates",
		"3 examples per candidate",
		"5 examples for `vela_module_summary`",
		"at most 5 graph calls total",
		"bounded fallback",
	})
}

func TestRottaOrchestratorAssetEnforcesCompactRankingWorkflow(t *testing.T) {
	data, err := assets.FS.ReadFile("agents/rotta-orchestrator.md")
	if err != nil {
		t.Fatalf("read orchestrator asset: %v", err)
	}
	got := string(data)

	assertContainsAll(t, got, []string{
		"Vela compact ranking enforcement",
		"use compact `vela_rank` or `vela_hotspots` first when available",
		"at most 5 graph calls total",
		"Do not manually rank candidates by repeatedly dumping full edges",
		"Vela is advisory graph intelligence only",
	})
}

func TestRottaOrchestratorAssetEnforcesCleanTDDTaskBoundaries(t *testing.T) {
	data, err := assets.FS.ReadFile("agents/rotta-orchestrator.md")
	if err != nil {
		t.Fatalf("read orchestrator asset: %v", err)
	}
	got := string(data)

	assertContainsAll(t, got, []string{
		"TDD task boundary rule",
		"every scenario task MUST start from a clean",
		"update the task checklist with",
		"Do not launch the next `rotta-impl` call",
		"until the worktree is clean again",
		"`rotta-impl` reports changed files; it does",
		"not decide how to persist or discard them",
	})
}

// REQ-003 → SCN-330 → TestSCN330_OnlyOrchestratorPersistsLifecycleDecisions
func TestSCN330_OnlyOrchestratorPersistsLifecycleDecisions(t *testing.T) {
	// Scenario: Only the orchestrator persists lifecycle decisions
	data, err := assets.FS.ReadFile("agents/rotta-orchestrator.md")
	if err != nil {
		t.Fatalf("read orchestrator asset: %v", err)
	}

	assertContainsAll(t, string(data), []string{
		"Exclusive Lifecycle Authority",
		"Only the Rotta-Orchestrator may persist lifecycle decisions",
		"approval, phase transition, scenario acceptance, checkpoint, or lifecycle archive",
		"Phase-role output alone is never lifecycle authority",
	})
}

// REQ-003 → SCN-331 → TestSCN331_SpecWorkProducesOnlyAssignedContractArtifacts
func TestSCN331_SpecWorkProducesOnlyAssignedContractArtifacts(t *testing.T) {
	// Scenario: Spec work produces only its contract artifacts
	data, err := assets.FS.ReadFile("agents/rotta-spec.md")
	if err != nil {
		t.Fatalf("read spec asset: %v", err)
	}

	got := string(data)
	assertContainsAll(t, got, []string{
		"MAY ONLY write the assigned hard spec and Gherkin contract artifacts",
		"MUST NOT create an approval record, baseline, current state, lifecycle state, or commit",
	})
	for _, forbidden := range []string{
		"Maintain the workflow state index",
		"Save a STATE INDEX",
		"Update the state index",
		"ancora_save",
	} {
		assertNotContains(t, got, forbidden)
	}
}

// REQ-003 → SCN-332 → TestSCN332_ImplementationWorkStopsAfterAssignedScenario
func TestSCN332_ImplementationWorkStopsAfterAssignedScenario(t *testing.T) {
	// Scenario: Implementation work stops after its assigned scenario
	data, err := assets.FS.ReadFile("agents/rotta-impl.md")
	if err != nil {
		t.Fatalf("read implementation asset: %v", err)
	}

	assertContainsAll(t, string(data), []string{
		"reports its evidence and changed paths",
		"does not choose another scenario, transition lifecycle state, approve, commit, clean, or mark completion",
	})
	for _, forbidden := range []string{
		"SCN-NNN COMPLETE",
		"completed_scenarios:",
		"ancora_save (upsert same topic_key):",
	} {
		assertNotContains(t, string(data), forbidden)
	}
}

// REQ-003 → SCN-333 → TestSCN333_ReviewWorkReturnsEvidenceWithoutAdvancingLifecycleState
func TestSCN333_ReviewWorkReturnsEvidenceWithoutAdvancingLifecycleState(t *testing.T) {
	// Scenario: Review work returns evidence without advancing lifecycle state
	data, err := assets.FS.ReadFile("agents/rotta-review.md")
	if err != nil {
		t.Fatalf("read review asset: %v", err)
	}

	got := string(data)
	assertContainsAll(t, got, []string{
		"returns pass, fail, or escalation evidence",
		"does not change approval, current-submission, lifecycle state, checkpoints, commits, or completion",
	})
	for _, forbidden := range []string{
		"Saves verdict",
		"next: feature_complete",
		"Save the State Index",
		"reports/judge_report.md",
		"ancora_save:",
	} {
		assertNotContains(t, got, forbidden)
	}
}

// REQ-003 → SCN-334 → TestSCN334_LateOrDirectPhaseAgentOutputCannotAdvanceWorkflow
func TestSCN334_LateOrDirectPhaseAgentOutputCannotAdvanceWorkflow(t *testing.T) {
	// Scenario: Late or direct phase-agent output cannot advance the workflow
	data, err := assets.FS.ReadFile("agents/rotta-orchestrator.md")
	if err != nil {
		t.Fatalf("read orchestrator asset: %v", err)
	}

	assertContainsAll(t, string(data), []string{
		"Direct, retried, or late phase-agent output never independently advances lifecycle state",
		"Before accepting any phase-agent result, validate it against approved scope and required evidence",
	})
}

// REQ-004 → SCN-335 → TestSCN335_ObjectiveReviewSuccessEntersFinalHumanReview
func TestSCN335_ObjectiveReviewSuccessEntersFinalHumanReview(t *testing.T) {
	// Scenario: Objective review success enters final human review rather than completion
	data, err := assets.FS.ReadFile("agents/rotta-orchestrator.md")
	if err != nil {
		t.Fatalf("read orchestrator asset: %v", err)
	}

	assertContainsAll(t, string(data), []string{
		"records that committed implementation snapshot as reviewed_commit",
		"transitions the feature durably to final_human_review",
		"does not mark the feature complete",
	})
}

// REQ-004 → SCN-336 → TestSCN336_ExplicitHumanApprovalCompletesReviewedSnapshot
func TestSCN336_ExplicitHumanApprovalCompletesReviewedSnapshot(t *testing.T) {
	// Scenario: Explicit human approval completes the reviewed snapshot
	data, err := assets.FS.ReadFile("agents/rotta-orchestrator.md")
	if err != nil {
		t.Fatalf("read orchestrator asset: %v", err)
	}

	assertContainsAll(t, string(data), []string{
		"Only explicit human approval",
		"current approved implementation snapshot matches reviewed_commit",
		"transitions the feature to complete",
		"does not record reviewer identity",
	})
}

// REQ-004 → SCN-337 → TestSCN337_ChangedOrInvalidatedReviewedSnapshotCannotComplete
func TestSCN337_ChangedOrInvalidatedReviewedSnapshotCannotComplete(t *testing.T) {
	// Scenario: A changed or invalidated reviewed snapshot cannot complete
	data, err := assets.FS.ReadFile("agents/rotta-orchestrator.md")
	if err != nil {
		t.Fatalf("read orchestrator asset: %v", err)
	}

	assertContainsAll(t, string(data), []string{
		"later code change, manual commit, amendment, rebase, dirty code change, or subsequent review failure",
		"does not complete from the stale reviewed commit",
		"returns the feature to review before completion can be possible",
	})
}

// REQ-004 → SCN-338 → TestSCN338_ReviewEligibilityPersistenceFailurePreventsFinalApproval
func TestSCN338_ReviewEligibilityPersistenceFailurePreventsFinalApproval(t *testing.T) {
	// Scenario: Failure to persist review eligibility does not create final-review authority
	data, err := assets.FS.ReadFile("agents/rotta-orchestrator.md")
	if err != nil {
		t.Fatalf("read orchestrator asset: %v", err)
	}

	assertContainsAll(t, string(data), []string{
		"If recording reviewed_commit or the final_human_review transition fails",
		"the feature is not eligible for final approval",
		"report the persistence failure",
	})
}

// REQ-005 → SCN-339 → TestSCN339_UserInvocableClaudePhaseRequestsRouteThroughOrchestrator
func TestSCN339_UserInvocableClaudePhaseRequestsRouteThroughOrchestrator(t *testing.T) {
	// Scenario: User-invocable Claude phase requests route through the orchestrator
	for _, assetPath := range []string{
		"skills/spec-mode/SKILL.md",
		"skills/implementation-mode/SKILL.md",
		"skills/review-mode/SKILL.md",
	} {
		data, err := assets.FS.ReadFile(assetPath)
		if err != nil {
			t.Fatalf("read %s: %v", assetPath, err)
		}

		assertContainsAll(t, string(data), []string{
			"MUST route the request through the Rotta-Orchestrator",
			"evaluates workspace authority and legal phase order before phase work starts",
		})
	}
}

func countOccurrences(items []string, want string) int {
	count := 0
	for _, item := range items {
		if item == want {
			count++
		}
	}
	return count
}
