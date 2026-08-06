package installer

import (
	"strings"
	"testing"

	"github.com/Syfra3/Rotta/assets"
)

func TestRottaOpsREQ005BEnforcesConsentGatedGitRootVelaIndexing(t *testing.T) {
	data, err := assets.FS.ReadFile("agents/rotta-ops.md")
	if err != nil {
		t.Fatal(err)
	}
	policy := string(data)

	for _, want := range []string{
		"fresh explicit user consent in the current ops capsule",
		"names exactly one target project",
		"Refuse empty, ambiguous, inferred, multiple, substituted, or symlinked targets",
		"current working directory",
		"canonical physical path",
		"git rev-parse --show-toplevel",
		"Git worktree/root",
		"filesystem root, home directory, non-Git directory",
		"`.vela` entry",
		"symlink, non-directory, unreadable, ambiguous, or resolves outside the canonical Git root",
		"Never create, delete, repair, migrate, register, initialize, or otherwise prepare `.vela`",
		"direct argv with no shell wrapper",
		"`['vela', 'update', '.']`",
		"Set cwd to the canonical Git root",
		"run exactly `vela update .`",
		"`vela serve --mcp` is host MCP service configuration, not an indexing command",
		"Do not run `vela build`",
		"Run the command once only: no automatic retry and no fallback command",
		"may mutate only the named canonical root's `.vela/`",
		"Report the named target and canonical Git root",
		"pre-existing `.vela` state, exact cwd and command, exit result",
		"whether graph state was generated or changed",
		"ignored for approval and unreviewed advisory state, never verified source truth",
		"visible evidence gap",
		"failure, stale/incomplete output, or unavailable graph",
		"source exploration as the fallback evidence",
		"do not block Fast work solely because graph evidence is unavailable or stale",
		"Do not index from an installer, TUI, CLI flow, background task, setup flow, or any automatic path",
		"Do not modify, remove, or disable usable MCP configuration on any Vela failure",
	} {
		if !strings.Contains(policy, want) {
			t.Fatalf("rotta-ops REQ-005B policy missing %q", want)
		}
	}
}
