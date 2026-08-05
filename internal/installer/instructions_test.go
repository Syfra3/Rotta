//go:build legacy_v2

package installer

import (
	"os"
	"path/filepath"
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

func TestCorePolicyUsesCoherentSlicesWithoutCleanTreeBoundaries(t *testing.T) {
	data, err := assets.FS.ReadFile("core/rotta-core.md")
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)

	assertContainsAll(t, got, []string{
		"one coherent slice",
		"independent review",
		"Do not require a worktree",
		"2,000 tokens",
	})
	assertNotContains(t, got, "one approved scenario at a time")
}

// REQ-080 -> SCN-516 -> TestSCN516_GeneratedReviewGuidancePreservesGenericGateContract
func TestSCN516_GeneratedReviewGuidancePreservesGenericGateContract(t *testing.T) {
	// Scenario: Generated review guidance preserves the executable generic-gate contract
	home, options := setupCanonicalWorkflowInstall(t)
	if _, err := Install(options); err != nil {
		t.Fatalf("install supported hosts: %v", err)
	}

	for host, path := range map[string]string{
		"claude-code": filepath.Join(home, ".claude", "skills", "rotta", "review-mode", "SKILL.md"),
		"opencode":    filepath.Join(home, ".config", "opencode", "skills", "rotta-review", "SKILL.md"),
		"codex":       filepath.Join(home, ".codex", "AGENTS.md"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s review guidance: %v", host, err)
		}
		got := string(data)

		assertContainsAll(t, got, []string{
			"rotta.quality-gates/v2 generic gate plan",
			".rotta/current/review-evidence.yaml",
			"unresolved required gate command blocks review",
			"Waivers remain visible",
			"final_human_review",
			"matching persisted current review evidence",
		})
		for _, prohibited := range []string{
			"rotta.quality-gates/v1",
			"root `.rotta/tdd-log.md`",
			"go test",
			"go vet",
			"coverage, mutation, and quality gates",
		} {
			assertNotContains(t, got, prohibited)
		}
	}
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

// REQ-005 → SCN-340 → TestSCN340_IllegalLaterPhaseRequestDoesNotExecuteDirectly
func TestSCN340_IllegalLaterPhaseRequestDoesNotExecuteDirectly(t *testing.T) {
	// Scenario: A request for an illegal later phase cannot execute that phase directly
	data, err := assets.FS.ReadFile("agents/rotta-orchestrator.md")
	if err != nil {
		t.Fatalf("read orchestrator asset: %v", err)
	}

	assertContainsAll(t, string(data), []string{
		"A request for a later phase with missing or invalid approval, or while an earlier phase is required, does not execute that phase directly",
		"stops or routes the request to the required earlier phase",
	})
}

// REQ-006 → SCN-342 → TestSCN342_ReviewEvaluatesOnlyConfiguredObjectiveGates
func TestSCN342_ReviewEvaluatesOnlyConfiguredObjectiveGates(t *testing.T) {
	// Scenario: Review evaluates only the configured objective gates
	data, err := assets.FS.ReadFile("skills/review-mode/SKILL.md")
	if err != nil {
		t.Fatalf("read review mode asset: %v", err)
	}

	assertContainsAll(t, string(data), []string{
		"rotta.quality-gates/v2` generic-gate plan",
		"`build`, `tests`, `changed_file_scope`",
		"supported declared project conventions and metadata",
		"configuration and plan fingerprints before evaluation",
		"unresolved, ambiguous, or unavailable required gate command blocks review",
	})
	assertNotContains(t, string(data), "specs/.implementation-complete")
	assertNotContains(t, string(data), ".rotta/review-evidence.yaml")

	qualityGates, err := assets.FS.ReadFile("config/quality-gates.yaml")
	if err != nil {
		t.Fatalf("read quality-gates asset: %v", err)
	}
	assertContainsAll(t, string(qualityGates), []string{
		"format: rotta.quality-gates/v2",
		"gate_order:",
		"discovery:",
		"supported_inputs:",
		"rules:",
		"gates:",
		"enabled: true",
	})

	reviewAgent, err := assets.FS.ReadFile("agents/rotta-review.md")
	if err != nil {
		t.Fatalf("read review agent asset: %v", err)
	}
	assertContainsAll(t, string(reviewAgent), []string{
		"rotta.quality-gates/v2` generic-gate plan",
		"declared supported conventions and metadata",
		"unresolved, ambiguous,",
		"blocks review with remediation",
		".rotta/current/review-evidence.yaml",
	})
	for _, prohibited := range []string{
		"specs/.implementation-complete",
		".rotta/review-evidence.yaml",
		"go test",
		"go-mutesting",
	} {
		assertNotContains(t, string(reviewAgent), prohibited)
	}
}

// REQ-006 → SCN-343 → TestSCN343_InvalidGateConfigurationStopsReviewWithoutDefaults
func TestSCN343_InvalidGateConfigurationStopsReviewWithoutDefaults(t *testing.T) {
	// Scenario: Invalid gate configuration stops review without defaults
	data, err := assets.FS.ReadFile("skills/review-mode/SKILL.md")
	if err != nil {
		t.Fatalf("read review mode asset: %v", err)
	}

	assertContainsAll(t, string(data), []string{
		"unresolved, ambiguous, or unavailable required gate command blocks review",
		"Never guess, substitute, silently pass",
	})
}

// REQ-006 → SCN-344 → TestSCN344_ConfigurationChangesControlSubsequentReviewBehavior
func TestSCN344_ConfigurationChangesControlSubsequentReviewBehavior(t *testing.T) {
	// Scenario: Configuration changes control subsequent review behavior
	data, err := assets.FS.ReadFile("skills/review-mode/SKILL.md")
	if err != nil {
		t.Fatalf("read review mode asset: %v", err)
	}

	assertContainsAll(t, string(data), []string{
		"declared thresholds, severity, and remediation",
		"Evaluate only the persisted plan against its",
	})
}

// REQ-006 → SCN-345 → TestSCN345_EmptyCriticalFunctionListIsNotApplicable
func TestSCN345_EmptyCriticalFunctionListIsNotApplicable(t *testing.T) {
	// Scenario: An explicitly empty critical-function list is not applicable
	data, err := assets.FS.ReadFile("skills/review-mode/SKILL.md")
	if err != nil {
		t.Fatalf("read review mode asset: %v", err)
	}

	assertNotContains(t, string(data), "critical-function")
	assertNotContains(t, string(data), "coverage sub-gate")
}

// REQ-006 → SCN-346 → TestSCN346_ReviewEvidenceIdentifiesConfigurationAndCommandOutcomes
func TestSCN346_ReviewEvidenceIdentifiesConfigurationAndCommandOutcomes(t *testing.T) {
	// Scenario: Review evidence identifies the configuration used and command outcomes
	data, err := assets.FS.ReadFile("skills/review-mode/SKILL.md")
	if err != nil {
		t.Fatalf("read review mode asset: %v", err)
	}

	assertContainsAll(t, string(data), []string{
		"Persist review evidence and the decision to `.rotta/current/review-evidence.yaml`",
		"configuration and plan fingerprints",
		"discovered commands, outputs, measurements, and remediation",
	})
}

// REQ-007 → SCN-347 → TestSCN347_Phase3RequiresValidApprovedCommittedAuthority
func TestSCN347_Phase3RequiresValidApprovedCommittedAuthority(t *testing.T) {
	// Scenario: Phase 3 starts only from valid approved committed authority
	data, err := assets.FS.ReadFile("agents/rotta-orchestrator.md")
	if err != nil {
		t.Fatalf("read orchestrator asset: %v", err)
	}

	assertContainsAll(t, string(data), []string{
		"Before delegating Phase 3, require approved Gherkin",
		"valid matching feature confirmation record",
		"committed baseline",
	})
}

// REQ-007 → SCN-348 → TestSCN348_ScenarioDelegationRequiresCleanRecordedWorktree
func TestSCN348_ScenarioDelegationRequiresCleanRecordedWorktree(t *testing.T) {
	// Scenario: Each scenario delegation requires a clean recorded worktree
	data, err := assets.FS.ReadFile("agents/rotta-orchestrator.md")
	if err != nil {
		t.Fatalf("read orchestrator asset: %v", err)
	}

	assertContainsAll(t, string(data), []string{
		"recorded worktree identity matches the current worktree",
		"tracked or non-ignored changes are present",
		"stop non-destructively and do not delegate that scenario",
	})
}

// REQ-007 → SCN-349 → TestSCN349_IgnoredLocalArtifactsDoNotBlockCleanScenarioBoundary
func TestSCN349_IgnoredLocalArtifactsDoNotBlockCleanScenarioBoundary(t *testing.T) {
	// Scenario: Ignored local artifacts do not block a clean scenario boundary
	data, err := assets.FS.ReadFile("agents/rotta-orchestrator.md")
	if err != nil {
		t.Fatalf("read orchestrator asset: %v", err)
	}

	assertContainsAll(t, string(data), []string{
		"ignored local artifacts alone do not block a clean scenario boundary",
		"tracked and non-ignored paths are clean",
		"may proceed with the approved scenario",
	})
}

// REQ-007 → SCN-350 → TestSCN350_ImplementationTaskReceivesOneApprovedScenarioAndStops
func TestSCN350_ImplementationTaskReceivesOneApprovedScenarioAndStops(t *testing.T) {
	// Scenario: Implementation receives exactly one approved scenario and stops
	data, err := assets.FS.ReadFile("agents/rotta-orchestrator.md")
	if err != nil {
		t.Fatalf("read orchestrator asset: %v", err)
	}

	assertContainsAll(t, string(data), []string{
		"Each rotta-impl task contains exactly one already-approved scenario",
		"After the task reports Red/Green/Refactor traceability and required evidence, it stops",
	})
}

// REQ-007 → SCN-351 → TestSCN351_OrchestratorValidatesScenarioResultBeforeContinuing
func TestSCN351_OrchestratorValidatesScenarioResultBeforeContinuing(t *testing.T) {
	// Scenario: The orchestrator validates a scenario result before continuing
	orchestrator, err := assets.FS.ReadFile("agents/rotta-orchestrator.md")
	if err != nil {
		t.Fatalf("read orchestrator asset: %v", err)
	}

	assertContainsAll(t, string(orchestrator), []string{
		"verify required evidence, approved scope, and boundary cleanliness",
		"Only after successful validation may it accept the scenario result, checkpoint it, and continue to the next approved scenario",
		"persist the scenario checkpoint evidence and accepted completed/remaining/next scenario state in durable current-submission artifacts before continuing",
	})

	review, err := assets.FS.ReadFile("agents/rotta-review.md")
	if err != nil {
		t.Fatalf("read review asset: %v", err)
	}

	assertContainsAll(t, string(review), []string{
		"Derive completed approved scope from durable current-submission state and the",
		"matching feature record; do not accept externally supplied scope",
	})
}

// REQ-007 → SCN-352 → TestSCN352_ScenarioLoopAnomaliesHaltWithoutBypass
func TestSCN352_ScenarioLoopAnomaliesHaltWithoutBypass(t *testing.T) {
	// Scenario: A scenario-loop anomaly halts without bypass
	data, err := assets.FS.ReadFile("agents/rotta-orchestrator.md")
	if err != nil {
		t.Fatalf("read orchestrator asset: %v", err)
	}

	assertContainsAll(t, string(data), []string{
		"checkpoint persistence and state persistence disagree",
		"another process changes the worktree during delegation",
		"contract drift is detected",
		"approval becomes invalid",
		"a required gate fails",
		"halt without bypassing approval, state validation, or clean-boundary checks",
	})
}

// REQ-009 → SCN-357 → TestSCN357_ResumeValidatesDurableWorkspaceAuthority
func TestSCN357_ResumeValidatesDurableWorkspaceAuthority(t *testing.T) {
	// Scenario: Resume validates durable workspace state rather than host or memory state
	data, err := assets.FS.ReadFile("agents/rotta-orchestrator.md")
	if err != nil {
		t.Fatalf("read orchestrator asset: %v", err)
	}

	assertContainsAll(t, string(data), []string{
		"validates the feature record, committed baseline, lifecycle state, recorded worktree, and relevant commit",
		"never reconstruct approval or lifecycle authority from host-local state or memory pointers",
	})
}

// REQ-009 → SCN-358 → TestSCN358_StaleArtifactsAndConcurrentResumesFailClosed
func TestSCN358_StaleArtifactsAndConcurrentResumesFailClosed(t *testing.T) {
	// Scenario: Stale host or memory artifacts cannot override canonical workspace state
	data, err := assets.FS.ReadFile("agents/rotta-orchestrator.md")
	if err != nil {
		t.Fatalf("read orchestrator asset: %v", err)
	}

	assertContainsAll(t, string(data), []string{
		"Stale host assets and memory pointers cannot authorize, transition, or recover workflow state",
		"Conflicting concurrent host resumes fail closed and never merge decisions",
	})
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
