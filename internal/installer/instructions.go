package installer

import (
	"fmt"
	"strings"

	"github.com/Syfra3/Rotta/assets"
)

func readRenderedAsset(path string, opts Options) ([]byte, error) {
	data, err := assets.FS.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := string(data)
	instructions := integrationInstructions(opts)
	if strings.Contains(text, "{{ROTTA_INTEGRATIONS}}") {
		text = strings.ReplaceAll(text, "{{ROTTA_INTEGRATIONS}}", instructions)
	} else if strings.HasSuffix(path, ".md") {
		text += instructions
	}
	return []byte(text), nil
}

func integrationInstructions(opts Options) string {
	var b strings.Builder
	b.WriteString("\n---\n\n")
	b.WriteString(canonicalWorkflowInstructions())
	b.WriteString("\n")
	b.WriteString("## Installed Integration Choices (Authoritative)\n\n")
	b.WriteString(memoryInstructions(opts.SetupAncora))
	b.WriteString("\n")
	b.WriteString(velaInstructions(opts.SetupVela, opts.SetupAncora))
	b.WriteString("\n")
	b.WriteString(context7Instructions(opts.SetupContext7))
	b.WriteString("\n")
	b.WriteString(explorationEnrichmentInstructions(opts.SetupVela))
	return b.String()
}

func canonicalWorkflowInstructions() string {
	return `## Rotta Canonical Workflow Contract

- Phase 1 — Draft: analyze the request, expose risks and missing information, and prepare only for spec work.
- Phase 2 — Spec + Gherkin: write the hard spec and Gherkin scenarios, then stop at the approval gate. Do NOT advance without explicit human approval.
- Phase 3 — TDD: implement one approved scenario at a time with strict Red/Green/Refactor TDD and traceable tests.
- Every TDD scenario task starts clean: before launching implementation, ` + "`git status --short`" + ` must be empty except for explicitly ignored local artifacts.
- After each TDD scenario task, update the task checklist with completed, remaining, and next work; then checkpoint or clean the task diff before starting another scenario.
- Do not let dirty worktree prompts become the scenario loop. Approved spec/feature contracts are tracked durable artifacts; generated/local artifacts are ignored or removed only when safe; ambiguous changes are escalated.
- Phase 4 — Review: run the metrics-based review workflow. The Judge reviews evidence, not code.
- After Phase 4 passes, provide a testable manual GitHub PR handoff: print the absolute recorded worktree path and git status --short; when reviewed outstanding changes exist, print optional reviewed-path-only git add -- <paths> and commit commands; print the resolved git push <remote> feature/<slug> command and gh pr create --base <base-branch> --head feature/<slug>; offer the GitHub web UI alternative; and disclose active-host command/delegation limits and that credentials remain user-controlled.
- Resolve and print publication commands only when exactly one GitHub-capable push remote is unambiguous. Do not push, create a pull request, merge, or directly modify the base branch.
- Preserve approval gates, phase order, lifecycle artifacts, and command semantics across hosts.
- Command invocation for hosts without slash commands: Use natural-language invocations such as ` + "`Rotta init`, `Rotta new`, `Rotta continue`, `Rotta status`, `Rotta skip`, and `Rotta back`" + `.
- These adapted invocations map to the same canonical Rotta command behavior and state transitions as exact command surfaces.
- When continuing a workflow started from another supported host, read shared workspace state before acting: ` + "`specs/`" + `, ` + "`features/`" + `, and ` + "`.rotta/`" + ` artifacts.
- Always preserve the same phase order, command semantics, and approval gates across hosts.
- Do not treat host-local config as the workflow source of truth.
- Preserve the no AI attribution rule: do not add AI-generated, generated-by, or co-author attribution to commits or generated project artifacts.
- Workspace files are the source of truth. Ancora stores compact pointers/status only when enabled.

## Capability Summary

- Claude Code: host instructions are adapted into Claude Code-consumable skills and settings.
- OpenCode: host instructions are exact OpenCode agent and skill artifacts.
- Codex: host instructions are adapted into a Codex-consumable ` + "`AGENTS.md`" + ` instruction file.
  - Agent capability: adapted; Codex receives role instructions in ` + "`AGENTS.md`" + ` instead of OpenCode-style named sub-agents.
  - Skill capability: adapted; Codex receives workflow sections in ` + "`AGENTS.md`" + ` instead of OpenCode-style skill directories.
  - Command capability: adapted; Codex uses documented natural-language Rotta command invocations instead of OpenCode-style slash commands.
`
}

func memoryInstructions(enabled bool) string {
	if enabled {
		return `### Ancora Memory Enabled

- Workspace files remain the source of truth; Ancora stores compact state indexes, decisions, and recovery pointers only.
- Workspace files remain the source of truth for specs, Gherkin features, TDD logs, reports, and workflow state.
- At session start, recover recent state with ` + "`ancora_context`" + ` and targeted ` + "`ancora_search`" + ` before advancing phases.
- After phase transitions, bug fixes, decisions, or non-obvious discoveries, save a compact pointer/status record with ` + "`ancora_save`" + `.
- State Index per Cycle (not the full log): save only fields such as ` + "`log_file: .rotta/tdd-log.md`" + `, ` + "`completed_scenarios:`" + `, ` + "`last_scenario:`" + `, ` + "`last_test:`" + `, ` + "`status: green`" + `, and ` + "`files_changed:`" + `.
- Do not store full hard specs, feature files, TDD logs, or review reports in Ancora; store paths and concise status only.

### Ancora Fallback

- Treat a missing or unavailable Ancora tool, when Ancora times out, permission is denied, Ancora cannot recover workflow state, Ancora cannot save workflow state, or any case where Ancora cannot otherwise be used as an Ancora degradation, not a workflow failure.
- Continue from workspace and installed-system OpenSpec workflow artifacts as the durable source of truth and state: applicable ` + "`specs/`" + `, ` + "`features/`" + `, ` + "`.rotta/`" + `, reports, approval markers, and workflow configuration.
- Do not fabricate recovered state, reconstruct authoritative content from Ancora, overwrite reviewed workspace artifacts from memory, and do not block workflow progress while the artifacts are available.
- Explicitly report the active Ancora fallback state, failure category, and a safe retry or recovery action; retry future pointer/state operations only after Ancora is available again.
- While Ancora fallback is active, preserve the canonical phase order, explicit human approval gate, TDD preconditions, quality gates, and workspace/OpenSpec source-of-truth precedence; do not bypass a required human approval or quality gate.
`
	}
	return `### Ancora Memory Disabled

- Do not call ` + "`ancora_*`" + ` tools, require Ancora topics, or report that state was saved to Ancora.
- Workspace files are the only state source: ` + "`specs/hard_spec.md`" + `, ` + "`features/*.feature`" + `, ` + "`.rotta/tdd-log.md`" + `, ` + "`reports/judge_report.md`" + `, and files under ` + "`.rotta/`" + `.
- If a base instruction mentions Ancora, treat it as disabled for this installation and write the equivalent state/index information to the workspace file named by the workflow.
`
}

func velaInstructions(enabled, ancoraEnabled bool) string {
	if !enabled {
		return `### Vela Graph Intelligence Disabled

- Do not call ` + "`vela_*`" + ` tools or require graph data.
- Use normal codebase exploration for structure, dependency, and impact questions.
`
	}

	surface := "Vela may be available as standalone `vela_*` MCP tools."
	if ancoraEnabled {
		surface = "Ancora remains the primary MCP surface; Vela graph tools may be exposed as `vela_*` tools through Ancora forwarding."
	}
	return fmt.Sprintf(`### Vela Graph Intelligence Enabled

- Rotta controls phases, gates, delegation, and final decisions. Vela is advisory graph intelligence only; it must never control the whole workflow.
- %s
- Rotta install persists a host-level Vela freshness guard (OpenCode plugin and Claude Code hooks) that schedules non-blocking background graph refresh before Vela graph query tools run; cached graph may be used while refresh runs.
- If Vela is intentionally skipped for an answer, do not call graph tools just because they are available.
- For structural dependency, reverse-dependency, impact, path, ownership, or architecture questions, run `+"`vela_status`"+` first and cache the result before any graph query. If graph data is stale, missing, or unavailable, follow the foreground refresh freshness/update/build path (`+"`vela update <workspace>`"+`, `+"`vela build <workspace>`"+`, `+"`vela extract <project>`"+`, or the available install command) before graph proof.
- Use Vela for structural questions only: dependencies, reverse dependencies, impact, paths, ownership, and architecture explanation.
- For ranking or hotspot structural questions ("highest impact", "most depended-on", "most dependencies", "central module", "biggest blast radius", "cross-package hotspot"), use compact `+"`vela_rank`"+` or `+"`vela_hotspots`"+` first when available. Do not manually rank candidates by repeatedly dumping full edges.
- Default compact ranking budget: limit 10 candidates, 3 examples per candidate, 5 examples for `+"`vela_module_summary`"+`, and at most 5 graph calls total for one ranking/hotspot question unless the user explicitly approves more.
- After compact ranking, call `+"`vela_module_summary`"+` or `+"`vela_explain`"+` only for top candidates that need verification, with low limits/bounded examples. Full edge dumps require an explicit user request.
- If compact tools are unavailable, use a bounded fallback: one status/lookup, one scoped explore or exact specialized query, summarize the limitation, and stop at the same 5-call graph budget instead of expanding into repeated edge dumps.
- Prefer exact file, module, controller, use-case, service, DTO, route handler, endpoint, or API-client subjects over broad prose. Do not send bag-of-words or broad feature descriptions directly to graph tools.
- Use `+"`vela_lookup`"+` to resolve concrete subjects before specialized graph calls such as `+"`vela_dependencies`"+`, `+"`vela_reverse_dependencies`"+`, `+"`vela_impact`"+`, `+"`vela_path`"+`, or `+"`vela_explain`"+`.
- `+"`vela_explore`"+` is routing/discovery only, not final proof when ambiguous. Follow it with lookup and specialized graph queries whenever possible.
- If symbol-level `+"`vela_dependencies`"+` or `+"`vela_reverse_dependencies`"+` returns `+"`(none)`"+` or an empty result and a containing file node exists, retry at file level before treating Vela as insufficient.
- Launch an exploration subagent for structural questions only after the exact Vela workflow fails or text/app caller verification is required. Before launching that subagent, state the specific Vela insufficiency or gap, such as empty symbol and file results, ambiguous/truncated graph data, stale/missing graph after refresh, or required source verification.
- Final answers must report Vela confidence and gaps when graph results are ambiguous, empty, stale, missing, truncated, or when optional ranking metrics are unavailable. Mention file-level fallback, graph-call budget use, and subagent justification when used.
- Vela is advisory only: do not let Vela control Rotta phase decisions, approvals, or review outcomes.

### Vela Degradation Fallback

- Treat missing graph tools or an unavailable Vela, when Vela times out, permission is denied, or stale, unusable, or failed graph data as a visible Vela-degraded state.
- Do not invoke a replacement graph MCP. For the affected structural question, perform no more than five focused source/code exploration actions against concrete files, symbols, callers, or configuration.
- Report source-derived evidence, state that Vela graph proof was unavailable, and identify any remaining gap; do not claim graph proof that source exploration cannot establish.
- Vela degradation does not change the canonical phase order, approval gates, or quality gates.
`, surface)
}

func context7Instructions(enabled bool) string {
	if !enabled {
		return ""
	}
	return `### Context7 Degradation Fallback

- Treat missing or unavailable Context7 tools, when Context7 times out, permission is denied, or there is a command, initialization, or documentation-query failure as a visible Context7-degraded state.
- The applicable workflow action continues without a documentation lookup and does not present unverified library or API details as fact.
- Identify assumptions and verification needs using only available project or user-provided evidence; ask for or defer verification when documentation is needed for a safe claim.
- Context7 degradation does not change phase order, approval, TDD, review, quality-gate, or source-of-truth requirements.
`
}

func explorationEnrichmentInstructions(velaEnabled bool) string {
	if !velaEnabled {
		return `### Exploration Output

- Return concise findings and file references to the orchestrator.
`
	}
	return `### Exploration Enrichment For Vela

- Treat targeted exploration as structured graph-enrichment input, not prose only.
- Return facts in this shape when exploring code structure:
  - subject: exact symbol, file, module, DTO, handler, or service
  - predicate: relationship such as depends_on, calls, implements, owns, emits, reads, writes, maps_to, or validates
  - object: exact related symbol, file, module, endpoint, event, or data shape
  - provenance: file path plus line range or command/tool used
  - confidence: high, medium, or low
  - source: ast, static_search, test_evidence, runtime_output, docs, or human_input
- If a fact cannot be proven, label confidence low and explain the missing evidence.
`
}
