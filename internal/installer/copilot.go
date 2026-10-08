package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func copilotRoot(home string) string { return filepath.Join(home, ".copilot") }

func copilotMCPPath(home string) string { return filepath.Join(copilotRoot(home), "mcp-config.json") }

// Copilot discovers agents globally, while the policy bundle is owned by Rotta.
// Agent stubs reference the same embedded source assets used by every host.
func installCopilot(opts Options, home string) ([]string, error) {
	root := filepath.Join(copilotRoot(home), "rotta-next")
	if !filepath.IsAbs(root) {
		return nil, fmt.Errorf("Copilot source loading unresolved: HOME must be absolute")
	}
	managed := map[string][]byte{}
	core, err := readRenderedAsset("core/rotta-core.md", opts)
	if err != nil {
		return nil, err
	}
	managed[filepath.Join(root, "rotta-core", "SKILL.md")] = bindCopilotAsset(core, root, "rotta-core")
	for _, agent := range rottaAgents {
		role, err := readRenderedAsset(agent.assetPath, opts)
		if err != nil {
			return nil, err
		}
		managed[filepath.Join(root, agent.skillName, "SKILL.md")] = bindCopilotAsset(role, root, agent.skillName)
		stub := fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n\n", agent.key, copilotDescription(agent.key))
		stub += fmt.Sprintf("You are %s. Before acting, use the available file-reading tool to read the entire core at %q, then your role at %q. These are the only authoritative policy files for this Rotta installation. If either file cannot be read completely, stop and report the exact path and reason. Pass this same root to any delegated agent. Do not substitute another host's skills or same-named files. Record the actual loaded paths; a source change requires rebaseline.\n", agent.key, filepath.Join(root, "rotta-core", "SKILL.md"), filepath.Join(root, agent.skillName, "SKILL.md"))
		managed[filepath.Join(copilotRoot(home), "agents", agent.key+".agent.md")] = []byte(stub)
	}
	if hasSelectedMCP(opts) || copilotOwnedMCPConfig(home) {
		path := copilotMCPPath(home)
		if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("Copilot MCP configuration must not be a symlink: %s", path)
		} else if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("inspect Copilot MCP configuration: %w", err)
		}
		config, err := copilotMCPConfig(opts)
		if err != nil {
			return nil, err
		}
		if validateManagedFilesOnly(home, path) == nil {
			managed[path] = config
		}
		// Preserve user-owned configuration; still install independent agents.
	}
	return installManagedFiles(home, managed)
}

func copilotHostCapabilities(opts Options, home string) map[string]HostCapability {
	capabilities := map[string]HostCapability{
		"delegation":     {Name: "delegation", Status: HostCapabilityStatusDegraded, Reason: "Copilot CLI agent tool restrictions and one-time operation guards are not enforced by Rotta.", Remediation: "Verify Copilot's available tools before delegating or running operations."},
		"source_loading": {Name: "source_loading", Status: HostCapabilityStatusPending, Reason: "Copilot agent files reference the installed absolute policy bundle; live loading was not observed.", Remediation: "Restart Copilot CLI, select rotta-orchestrator, and verify its core and role were read from the listed paths."},
	}
	preserved := copilotMCPPreserved(home)
	for _, name := range selectedMCPCapabilities(opts) {
		if preserved {
			capabilities[name] = HostCapability{Name: name, Status: HostCapabilityStatusUnsupported, Reason: "Existing Copilot MCP configuration is not Rotta-owned or was edited; preserved without validation.", Remediation: "Configure the selected server in ~/.copilot/mcp-config.json yourself or reconcile ownership before reinstalling Rotta."}
		} else {
			capabilities[name] = HostCapability{Name: name, Status: HostCapabilityStatusPending, Reason: "Copilot MCP configuration generated; runtime server discovery was not observed.", Remediation: "Restart Copilot CLI and verify the selected MCP server and its tools there."}
		}
	}
	return capabilities
}

func validateManagedFilesOnly(home, path string) error {
	_, err := validateManagedFiles(home, map[string][]byte{path: nil})
	return err
}

func copilotDescription(role string) string {
	if role == "rotta-orchestrator" {
		return "Primary Rotta workflow router for Fast and Strict tasks"
	}
	return "Rotta " + strings.TrimPrefix(role, "rotta-") + " role; use only when delegated by the orchestrator"
}

func bindCopilotAsset(data []byte, root, role string) []byte {
	return bindHostAsset(data, fmt.Sprintf(`## Resolved Copilot CLI policy bundle

The resolved bundle root is %q. Read the core at %q and this role at %q in full using an available file-reading tool before acting. Keep this exact bundle identity across delegations. Never substitute other hosts' copies or a same-named skill. If loading fails or source identity changes, stop and report the reason.
`, root, filepath.Join(root, "rotta-core", "SKILL.md"), filepath.Join(root, role, "SKILL.md")))
}

func copilotMCPConfig(opts Options) ([]byte, error) {
	servers := map[string]any{}
	for _, name := range selectedMCPCapabilities(opts) {
		service := strings.TrimPrefix(name, "mcp:")
		command, args := service, []string{"mcp"}
		switch service {
		case "vela":
			args = []string{"serve", "--mcp"}
		case "context7":
			command, args = "npx", []string{"-y", "@upstash/context7-mcp"}
		}
		servers[service] = map[string]any{"type": "stdio", "command": command, "args": args}
	}
	return json.MarshalIndent(map[string]any{"mcpServers": servers}, "", "  ")
}

func copilotBackupPaths(home string) []string {
	paths := []string{filepath.Join(copilotRoot(home), "rotta-next"), copilotMCPPath(home)}
	for _, agent := range rottaAgents {
		paths = append(paths, filepath.Join(copilotRoot(home), "agents", agent.key+".agent.md"))
	}
	return paths
}

func copilotMCPPreserved(home string) bool {
	if _, err := os.Lstat(copilotMCPPath(home)); os.IsNotExist(err) {
		return false
	}
	return validateManagedFilesOnly(home, copilotMCPPath(home)) != nil
}

func copilotOwnedMCPConfig(home string) bool {
	path := copilotMCPPath(home)
	if _, err := os.Lstat(path); err != nil {
		return false
	}
	return validateManagedFilesOnly(home, path) == nil
}
