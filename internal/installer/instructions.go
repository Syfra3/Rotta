package installer

import (
	"fmt"
	"strings"

	"github.com/Syfra3/clean-workflow/assets"
)

func readRenderedAsset(path string, opts Options) ([]byte, error) {
	data, err := assets.FS.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := string(data)
	instructions := integrationInstructions(opts)
	if strings.Contains(text, "{{CLEAN_WORKFLOW_INTEGRATIONS}}") {
		text = strings.ReplaceAll(text, "{{CLEAN_WORKFLOW_INTEGRATIONS}}", instructions)
	} else if strings.HasSuffix(path, ".md") {
		text += instructions
	}
	return []byte(text), nil
}

func integrationInstructions(opts Options) string {
	var b strings.Builder
	b.WriteString("\n---\n\n")
	b.WriteString("## Installed Integration Choices (Authoritative)\n\n")
	b.WriteString(memoryInstructions(opts.SetupAncora))
	b.WriteString("\n")
	b.WriteString(velaInstructions(opts.SetupVela, opts.SetupAncora))
	b.WriteString("\n")
	b.WriteString(explorationEnrichmentInstructions(opts.SetupVela))
	return b.String()
}

func memoryInstructions(enabled bool) string {
	if enabled {
		return `### Ancora Memory Enabled

- Workspace files remain the source of truth; Ancora stores compact state indexes, decisions, and recovery pointers only.
- At session start, recover recent state with ` + "`ancora_context`" + ` and targeted ` + "`ancora_search`" + ` before advancing phases.
- After phase transitions, bug fixes, decisions, or non-obvious discoveries, save a compact pointer/status record with ` + "`ancora_save`" + `.
- Do not store full specs, feature files, TDD logs, or judge reports in Ancora; store paths and concise status only.
`
	}
	return `### Ancora Memory Disabled

- Do not call ` + "`ancora_*`" + ` tools, require Ancora topics, or report that state was saved to Ancora.
- Workspace files are the only state source: ` + "`specs/hard_spec.md`" + `, ` + "`features/*.feature`" + `, ` + "`.clean-workflow/tdd-log.md`" + `, ` + "`reports/judge_report.md`" + `, and files under ` + "`.clean-workflow/`" + `.
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

- Clean Workflow controls phases, gates, delegation, and final decisions. Vela is advisory graph intelligence only; it must never control the whole workflow.
- %s
- For a new codebase or first structural question, graph data may not exist yet. Check for project graph data before relying on Vela.
- If graph data is missing or stale, trigger extraction/indexing first with `+"`vela extract <project>`"+` or the available Vela install/build command, then query the graph.
- Use Vela for structural questions only: dependencies, reverse dependencies, impact, paths, ownership, and architecture explanation.
- Do not send bag-of-words or broad feature descriptions directly to Vela. First identify concrete files, symbols, types, DTOs, services, handlers, or modules.
- If confidence is low, graph data is stale, or graph gaps remain, report the gaps and confidence level to the orchestrator. The orchestrator decides whether to spend more exploration effort.
`, surface)
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
