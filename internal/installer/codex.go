package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	codexMCPStartMarker = "# >>> rotta managed mcp servers"
	codexMCPEndMarker   = "# <<< rotta managed mcp servers"
)

func installCodex(opts Options, home string) ([]string, error) {
	path := filepath.Join(home, ".codex", "AGENTS.md")
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("Codex source loading unresolved: instruction path %q is not absolute; set an absolute HOME", path)
	}
	instructions, err := codexInstructions(opts, path)
	if err != nil {
		return nil, err
	}
	if _, err := installManagedFiles(home, map[string][]byte{path: []byte(instructions)}); err != nil {
		return nil, err
	}
	return []string{path}, nil
}

func cleanPreviousCodexInstallation(_ string) error { return nil }

func configureCodexMCPServers(opts Options, home string) ([]string, error) {
	path := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("cannot create Codex config dir: %w", err)
	}
	data, err := readPrivateFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("cannot read Codex config: %w", err)
	}

	content := replaceCodexManagedMCPBlock(string(data), codexManagedMCPBlock(opts))
	if err := writePrivateFile(path, []byte(content), 0o600); err != nil {
		return nil, fmt.Errorf("cannot write Codex MCP config: %w", err)
	}
	return []string{path}, nil
}

func replaceCodexManagedMCPBlock(content, block string) string {
	start := strings.Index(content, codexMCPStartMarker)
	end := strings.Index(content, codexMCPEndMarker)
	if start >= 0 && end >= start {
		end += len(codexMCPEndMarker)
		content = strings.TrimSpace(content[:start] + content[end:])
	}
	if strings.TrimSpace(content) == "" {
		return block
	}
	return strings.TrimRight(content, "\n") + "\n\n" + block
}

func codexManagedMCPBlock(opts Options) string {
	var b strings.Builder
	b.WriteString(codexMCPStartMarker + "\n")
	if opts.SetupAncora {
		b.WriteString("[mcp_servers.ancora]\n")
		b.WriteString("command = \"ancora\"\n")
		b.WriteString("args = [\"mcp\"]\n\n")
	}
	if opts.SetupVela {
		b.WriteString("[mcp_servers.vela]\n")
		b.WriteString("command = \"vela\"\n")
		b.WriteString("args = [\"mcp\"]\n\n")
	}
	if opts.SetupContext7 {
		b.WriteString("[mcp_servers.context7]\n")
		b.WriteString("command = \"npx\"\n")
		b.WriteString("args = [\"-y\", \"@upstash/context7-mcp\"]\n\n")
	}
	b.WriteString(codexMCPEndMarker + "\n")
	return b.String()
}

func codexInstructions(opts Options, path string) (string, error) {
	core, err := readRenderedAsset("core/rotta-core.md", opts)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("# Rotta Codex Instructions\n\n")
	b.WriteString("Codex adapts Rotta Next roles into this instruction file. Route work through the orchestrator and follow the shared policy below.\n\n")
	fmt.Fprintf(&b, `## Resolved Codex inline policy bundle

The resolved bundle is the single instruction file %q. Its named "Embedded policy: rotta-core" section and "Embedded policy: <role>" sections are the canonical core and roles for this host. Consuming these complete embedded sections satisfies all instructions below to load/read core or role files from the explicit resolved bundle; no separate SKILL.md reads are required or expected. Apply the core and the active role, not every role's duties at once.
Use sections already supplied in the current context only when complete through their "End embedded policy" marker. If a required section is absent or truncated, read that section from this exact AGENTS.md with the host's available file-reading tool. Do not search for SKILL.md files, invoke name-based skill loading, or substitute Claude Code/OpenCode bundles. If the file or section cannot be fully read, report its exact path, section, and reason as source loading blocked/unknown and stop.
Record source identity as this absolute AGENTS.md path plus the core and active-role section names. Pass that same file and section identity to children; a conflicting source requires safe-stop and rebaseline.

`, path)
	b.WriteString("## Embedded policy: rotta-core\n\n")
	b.Write(core)
	b.WriteString("\n\n<!-- End embedded policy: rotta-core -->\n")
	for _, agent := range rottaAgents {
		role, err := readRenderedAsset(agent.assetPath, opts)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "\n\n## Embedded policy: %s\n\n", agent.skillName)
		b.Write(role)
		fmt.Fprintf(&b, "\n\n<!-- End embedded policy: %s -->\n", agent.skillName)
	}
	return b.String(), nil
}
