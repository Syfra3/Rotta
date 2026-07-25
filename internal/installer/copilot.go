package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func installCopilotCLI(opts Options, home string) ([]string, error) {
	root, err := resolveCopilotGlobalConfigRoot(home)
	if err != nil {
		return nil, err
	}
	agentsDir := filepath.Join(root, "agents")
	if err := os.MkdirAll(agentsDir, 0o750); err != nil {
		return nil, fmt.Errorf("cannot create Copilot agents directory: %w", err)
	}

	var files []string
	for _, agent := range rottaAgents {
		if !agent.modeFlag(opts) {
			continue
		}
		data, err := readRenderedAsset(agent.assetPath, opts)
		if err != nil {
			return nil, fmt.Errorf("cannot read embedded %s: %w", agent.assetPath, err)
		}
		path := filepath.Join(agentsDir, agent.key+".agent.md")
		if err := writePrivateFile(path, copilotAgentMarkdown(agent.key, data), 0o600); err != nil {
			return nil, fmt.Errorf("cannot write %s: %w", path, err)
		}
		files = append(files, path)
	}

	instructionsPath := filepath.Join(root, "instructions", "rotta.instructions.md")
	if err := os.MkdirAll(filepath.Dir(instructionsPath), 0o750); err != nil {
		return nil, fmt.Errorf("cannot create Copilot instructions directory: %w", err)
	}
	instructions := "# Rotta Copilot Instructions\n\n" + integrationInstructions(opts)
	if err := writePrivateFile(instructionsPath, []byte(instructions), 0o600); err != nil {
		return nil, fmt.Errorf("cannot write %s: %w", instructionsPath, err)
	}
	return append(files, instructionsPath), nil
}

func resolveCopilotGlobalConfigRoot(home string) (string, error) {
	root := os.Getenv("COPILOT_HOME")
	if root == "" {
		root = filepath.Join(home, ".copilot")
	}
	resolved, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("cannot resolve Copilot global configuration root: %w", err)
	}
	return resolved, nil
}

func copilotAgentMarkdown(name string, asset []byte) []byte {
	body := strings.TrimPrefix(string(asset), "---\n")
	if end := strings.Index(body, "\n---\n"); end >= 0 {
		body = body[end+len("\n---\n"):]
	}
	return []byte("---\nname: " + name + "\n---\n\n" + body)
}
