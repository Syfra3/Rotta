package installer

// These compatibility guards are deliberately inert. Native question flow is
// prompt-driven by the orchestrator; a local plugin or hook must never mutate
// a graph, schedule a refresh, or invoke Vela while handling advisory evidence.
const openCodeVelaFreshnessGuardPluginSource = `// Rotta Vela advisory guard: no automatic refresh or Vela invocation.
export const RottaVelaFreshnessGuard = async () => ({
  "tool.execute.before": async () => {},
});
`

const claudeVelaFreshnessGuardScriptSource = `#!/usr/bin/env bash
# Rotta Vela advisory guard: no automatic refresh or Vela invocation.
exit 0
`
