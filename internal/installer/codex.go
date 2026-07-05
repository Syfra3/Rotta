package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
