package installer

import (
	"path/filepath"
	"strings"
	"testing"
)

// These are generated-policy regression checks, not claims that a model follows
// instructions. Retained decision exercises are documented separately.
func TestWorkflowConvergenceSurvivesHostRendering(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("OPENCODE_CONFIG", "")
	if _, err := installOpenCode(Options{}, home); err != nil {
		t.Fatal(err)
	}
	if _, err := installClaudeCode(Options{}, home); err != nil {
		t.Fatal(err)
	}
	if _, err := installCodex(Options{}, home); err != nil {
		t.Fatal(err)
	}
	for host, path := range map[string]string{
		"opencode": filepath.Join(home, ".config", "opencode", "skills", "rotta-next", "rotta-core", "SKILL.md"),
		"claude":   filepath.Join(home, ".claude", "skills", "rotta-next", "rotta-core", "SKILL.md"),
		"codex":    filepath.Join(home, ".codex", "AGENTS.md"),
	} {
		t.Run(host, func(t *testing.T) {
			policy := readRottaNextInstalledAsset(t, path)
			assertRottaNextContainsAll(t, policy, []string{
				"Fast mode is the default", "one explicit approval",
				"one approval packet, not a second equivalent gate",
				"Decomposing approved work does not require child contracts",
				"smallest in-scope correction", "new material evidence",
				"at most three independent review invocations",
				"one automatic root-cause recovery", "a counter alone is not a user gate",
				"continue independent authorized work", "never reset review history",
				"not a runtime-enforced controller", "No blockers means advance automatically",
			})
			assertRottaNextLacksAll(t, policy, []string{
				"It approves no delegation", "safely stop on resolution",
				"review remediation that is not isolated", "A changed diff after the handoff invalidates stale evidence",
			})
			if strings.Contains(policy, "{{ROTTA_INTEGRATIONS}}") {
				t.Fatal("integration placeholder leaked into installed policy")
			}
		})
	}
}
