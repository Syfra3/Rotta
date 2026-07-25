package installer

import (
	"encoding/json"
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
	instructions := "# Rotta Copilot Instructions\n\n" + copilotAdaptationInstructions() + integrationInstructions(opts)
	if err := writePrivateFile(instructionsPath, []byte(instructions), 0o600); err != nil {
		return nil, fmt.Errorf("cannot write %s: %w", instructionsPath, err)
	}
	files = append(files, instructionsPath)
	if !hasSelectedMCP(opts) || os.Getenv("COPILOT_MCP_CONFIG") == "" {
		return files, nil
	}
	mcpPath, err := configureCopilotMCPFixture(opts)
	if err != nil {
		return nil, err
	}
	return append(files, mcpPath), nil
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

func copilotAdaptationInstructions() string {
	return `## Copilot CLI Adaptation

- Copilot integration is global-only; it does not create repository .github Copilot files, .mcp.json, AGENTS.md, or CLAUDE.md files.
- Select ` + "`rotta-orchestrator`" + ` through ` + "`/agent rotta-orchestrator`" + ` or ` + "`copilot --agent rotta-orchestrator`" + ` before requesting phase work.
- This routes phase work through the Rotta-Orchestrator decision point before phase execution; direct phase roles do not bypass it.
- Copilot role-agent and command support is adapted: custom agents select role guidance. It is not host-native hidden subagent delegation, automatic delegation, or direct phase bypass.

`
}

type copilotMCPServer struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

func configureCopilotMCPFixture(opts Options) (string, error) {
	path, err := resolveCopilotMCPConfigPath()
	if err != nil {
		return "", err
	}
	config := map[string]interface{}{}
	if data, err := readPrivateFile(path); err != nil {
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("cannot read Copilot MCP configuration: %w", err)
		}
	} else if err := json.Unmarshal(data, &config); err != nil {
		return "", fmt.Errorf("cannot parse Copilot MCP configuration: %w", err)
	}
	mcpServers, _ := config["mcpServers"].(map[string]interface{})
	if mcpServers == nil {
		mcpServers = map[string]interface{}{}
	}
	for name, server := range selectedCopilotMCPFixture(opts) {
		mcpServers[name] = server
	}
	config["mcpServers"] = mcpServers
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("cannot marshal Copilot MCP configuration: %w", err)
	}
	if err := writePrivateFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("cannot write Copilot MCP configuration: %w", err)
	}
	return path, nil
}

func resolveCopilotMCPConfigPath() (string, error) {
	path := os.Getenv("COPILOT_MCP_CONFIG")
	if path == "" {
		return "", fmt.Errorf("cannot resolve active Copilot MCP configuration path; set COPILOT_MCP_CONFIG to the active global MCP fixture path")
	}
	resolved, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("cannot resolve Copilot MCP configuration path: %w", err)
	}
	return resolved, nil
}

func selectedCopilotMCPFixture(opts Options) map[string]copilotMCPServer {
	fixture := map[string]copilotMCPServer{}
	if opts.SetupAncora {
		fixture["ancora"] = copilotMCPServer{Command: "ancora", Args: []string{"mcp"}}
	}
	if opts.SetupVela {
		fixture["vela"] = copilotMCPServer{Command: "vela", Args: []string{"mcp"}}
	}
	if opts.SetupContext7 {
		fixture["context7"] = copilotMCPServer{Command: "npx", Args: []string{"-y", "@upstash/context7-mcp"}}
	}
	return fixture
}

func recordCopilotMCPHealthEvidence(result *Result, opts Options) {
	if !includesCopilot(opts.Target) || !hasSelectedMCP(opts) {
		return
	}
	evidence := opts.CopilotMCPHealthEvidence
	result.CopilotMCPHealthEvidence = evidence

	host := result.Hosts["copilot-cli"]
	if host.Status != HostInstallStatusInstalled || !evidence.ConfigurationAccepted || evidence.VersionOutput == "" {
		return
	}
	allHealthy := true
	for _, name := range selectedCopilotMCPNames(opts) {
		if !evidence.provesHealthyServer(name) {
			allHealthy = false
			continue
		}
		host.Capabilities["mcp:"+name] = exactMCPCapability("mcp:" + name)
	}
	if allHealthy {
		host.Capabilities["mcp"] = exactCapability("mcp")
	}
	result.Hosts["copilot-cli"] = host
}

func (evidence CopilotMCPHealthEvidence) provesHealthyServer(name string) bool {
	proof, ok := evidence.InteractiveMCPShowOutputs[name]
	return ok && proof.Healthy &&
		strings.Contains(evidence.MCPListOutput, name) &&
		strings.Contains(evidence.InteractiveMCPListOutput, name) &&
		strings.Contains(proof.Output, name)
}

func selectedCopilotMCPNames(opts Options) []string {
	var names []string
	if opts.SetupAncora {
		names = append(names, "ancora")
	}
	if opts.SetupVela {
		names = append(names, "vela")
	}
	if opts.SetupContext7 {
		names = append(names, "context7")
	}
	return names
}
