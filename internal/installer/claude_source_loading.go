package installer

import (
	"fmt"
	"path/filepath"
)

func claudeBundleRoot(home string) (string, error) {
	root := filepath.Join(home, ".claude", "skills", "rotta-next")
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("Claude Code source loading unresolved: bundle path %q is not absolute; set an absolute HOME", root)
	}
	return root, nil
}

func bindClaudeAsset(data []byte, root, role string) []byte {
	instructions := fmt.Sprintf(`## Resolved Claude Code policy bundle

The resolved bundle root is %q. Before acting, use Claude Code's Read tool to read the absolute file path %q for the core, then %q for this role. Skip a read only when that exact file is already fully loaded in this session; read remaining sections if output is truncated. An automatically loaded agent file supplies this binding but does not replace the core and role reads.
Every instruction to load or read rotta-core or this role refers to these exact files. Do not resolve them using a name-based Skill tool, another host's bundle, or a same-named fallback. Pass this bundle root to children, which must read their own role and core from the same root, and record actual loaded paths. If the parent supplies a conflicting bundle, or a required file is missing, unreadable, denied, or incomplete, report the exact path and reason as source loading blocked/unknown and stop. A source change requires safe-stop and rebaseline.
`, root, filepath.Join(root, "rotta-core", "SKILL.md"), filepath.Join(root, role, "SKILL.md"))
	return bindHostAsset(data, instructions)
}
