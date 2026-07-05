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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("cannot create Codex instructions dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(codexInstructions(opts)), 0o644); err != nil {
		return nil, fmt.Errorf("cannot write Codex instructions: %w", err)
	}
	return []string{path}, nil
}

func cleanPreviousCodexInstallation(home string) error {
	path := filepath.Join(home, ".codex", "AGENTS.md")
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("cannot remove stale Codex instructions: %w", err)
	}
	return nil
}

func configureCodexMCPServers(opts Options, home string) ([]string, error) {
	path := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("cannot create Codex config dir: %w", err)
	}
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("cannot read Codex config: %w", err)
	}

	content := replaceCodexManagedMCPBlock(string(data), codexManagedMCPBlock(opts))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
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

func codexInstructions(opts Options) string {
	var b strings.Builder
	b.WriteString("# Rotta Codex Instructions\n\n")
	b.WriteString("Rotta is installed for Codex. Follow the canonical Rotta workflow, keep workspace artifacts as the source of truth, and preserve human approval gates.\n\n")
	if opts.InstallSpec {
		b.WriteString("- Spec mode: draft hard specs and Gherkin scenarios before implementation.\n")
	}
	if opts.InstallImpl {
		b.WriteString("- Implementation mode: use strict Red/Green/Refactor TDD for one approved scenario at a time.\n")
	}
	if opts.InstallReview {
		b.WriteString("- Review mode: judge evidence against traceability, tests, coverage, mutation, and quality gates.\n")
	}
	b.WriteString("\n")
	b.WriteString(integrationInstructions(opts))
	return b.String()
}
