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
	if opts.SetupContext7 {
		b.WriteString("\nContext7 is enabled for current library and API documentation; a lookup failure is advisory and must be reported as uncertainty.")
	}
	return b.String()
}

func memoryInstructions(enabled bool) string {
	if enabled {
		return "Ancora is enabled. Recover concise relevant context at task start and save only compact decisions, discoveries, fix summaries, and end-of-session summaries. A failure is a bounded warning; workspace and Git state remain authoritative."
	}
	return "Ancora is disabled. Do not call `ancora_*`; recover from the current workspace and Git state."
}

func velaInstructions(enabled bool) string {
	if !enabled {
		return "Vela is disabled. Use focused source exploration for structural questions."
	}

	return "Vela is enabled as advisory local graph evidence. Use it only for a named structural question, never index automatically, and fall back to source when it is absent, stale, or fails."
}
