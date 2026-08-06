package installer

import (
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
	}
	return []byte(text), nil
}

func integrationInstructions(opts Options) string {
	var b strings.Builder
	b.WriteString("## Installed Integration Choices\n\n")
	b.WriteString(memoryInstructions(opts.SetupAncora))
	b.WriteString("\n")
	b.WriteString(velaInstructions(opts.SetupVela))
	if opts.SetupRTK {
		b.WriteString("\nRTK is optional presentation infrastructure. Use it only through the recorded host-local verified executable after runtime path, version, and executable-fingerprint revalidation; otherwise return the unfiltered underlying bounded result. RTK output never replaces durable evidence.")
		b.WriteString(" Runtime reads the installer-written `$XDG_STATE_HOME/rotta/rtk.json` (or `~/.local/state/rotta/rtk.json`), accepts only the recorded canonical Homebrew Cellar executable, and validates and executes one opened file object rather than PATH or a re-resolved pathname. Use RTK only for the documented compact Git, Go/test, diff, log, and error views; unsupported or failed filtering must preserve the exact bounded deterministic output and its evidence path/hash.")
	}
	if opts.SetupContext7 {
		b.WriteString("\nContext7 is enabled for current library and API documentation; a lookup failure is advisory and must be reported as uncertainty.")
	}
	return b.String()
}

func memoryInstructions(enabled bool) string {
	if enabled {
		return "Ancora is enabled. Recover concise relevant context once at task start or resume, never once per role, and save only compact material decisions, discoveries, fixes, or outcomes. On any advisory failure, report the evidence gap and continue from workspace/Git; Ancora cannot authorize or block work. Handoff records use injected Ancora only as a non-authoritative index with an atomic matching `.rotta/handoffs/` mirror; on failure report degraded recovery and validate the newest matching mirror by sequence, never timestamp. Workspace and Git state remain authoritative."
	}
	return "Ancora is disabled. Do not call `ancora_*`; recover from the current workspace and Git state."
}

func velaInstructions(enabled bool) string {
	if !enabled {
		return "Vela is disabled. Use focused source exploration for structural questions."
	}

	return "Vela is enabled as advisory local graph evidence. Use it only for a named dependency, impact, ownership, architectural-flow, or unfamiliar-module question within the existing two-call exploration or one-call review budget. Distill only symbols/files, confidence, gaps, and a safe action into a capsule; fall back to source when it is absent, stale, ambiguous, or fails. Never automatically install, set up, index, re-index, or retry Vela."
}
