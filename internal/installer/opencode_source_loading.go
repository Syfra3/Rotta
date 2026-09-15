package installer

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Use the same root for emitted loaders and managed files. Never let a relative
// XDG path turn into a workspace-dependent policy source at host runtime.
func openCodeBundleRoot(home string) (string, error) {
	root := filepath.Join(openCodeConfigDir(home), "skills", "rotta-next")
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("OpenCode source loading unresolved: bundle path %q is not absolute; set an absolute HOME/XDG_CONFIG_HOME", root)
	}
	return root, nil
}

func openCodeBundleInstructions(root, role string) string {
	corePath := filepath.Join(root, "rotta-core", "SKILL.md")
	rolePath := filepath.Join(root, role, "SKILL.md")
	return fmt.Sprintf(`## Resolved OpenCode policy bundle

The resolved bundle root is %q. Before acting, use the read tool with absolute filePath %q to load the core, then %q to load this role (skip a read only if that exact file is already fully loaded in this session). Read remaining sections if output is truncated.
For this session, every instruction to load rotta-core or this role means these exact files, including "Load the rotta-core skill" in role instructions. Do not use the name-based skill tool to resolve these files, search alternate skill directories, or substitute a same-named skill from another bundle.
Keep this resolved bundle in delegated task context; each child must read its own role and the core from this same root. Record actual loaded paths, not merely intended paths. If a parent supplies a different bundle, or either file is missing, unreadable, denied, or cannot be fully loaded with this host's tools, report the exact path and reason as source loading blocked/unknown and stop; never fall back to name-based loading. A source change requires safe-stop and rebaseline.
`, root, corePath, rolePath)
}

func openCodeAgentPrompt(agent agentEntry, root string) string {
	// Retain the baseline text as an exact migration fingerprint, but remove its
	// ambiguous loading sentence from every newly generated prompt.
	legacyLoader := fmt.Sprintf("Load rotta-core and %s from ~/.config/opencode/skills/rotta-next/ before acting. ", agent.skillName)
	prompt := strings.Replace(agent.prompt, legacyLoader, "", 1)
	if agent.key == "rotta-orchestrator" {
		prompt += " Use file edits only for workflow records and approval packets under core policy."
	}
	return prompt + "\n\n" + openCodeBundleInstructions(root, agent.skillName)
}

func bindOpenCodeAsset(data []byte, root, role string) []byte {
	return bindHostAsset(data, openCodeBundleInstructions(root, role))
}

// Existing non-model config is user-owned. The sole migration exception is an
// exact baseline loader with an intact, recorded role artifact in this bundle.
func canUpgradeOpenCodePrompt(agent agentEntry, entry map[string]interface{}, root string, manifest managedArtifactsManifest) bool {
	if entry["prompt"] != agent.prompt {
		return false
	}
	path := filepath.Join(root, agent.skillName, "SKILL.md")
	data, err := readPrivateFile(path)
	return err == nil && manifest.Files[path] == contentDigest(data) && !strings.Contains(string(data), "## Resolved OpenCode policy bundle")
}

func openCodeSourceLoadingCapability(home string, config map[string]interface{}) HostCapability {
	capability := HostCapability{Name: "source_loading", Status: HostCapabilityStatusExact,
		Reason:      "Generated OpenCode prompts select core and role by absolute read paths. Runtime reads and effective host overlays have not been verified.",
		Remediation: "Restart OpenCode after installation; verify actual loaded paths and read/external_directory permissions. Never substitute name-based skill loading."}
	root, err := openCodeBundleRoot(home)
	if err != nil {
		capability.Status, capability.Reason = HostCapabilityStatusDegraded, err.Error()
		return capability
	}
	agents, _ := config["agent"].(map[string]interface{})
	var unresolved []string
	for _, agent := range rottaAgents {
		entry, _ := agents[agent.key].(map[string]interface{})
		if entry["prompt"] != openCodeAgentPrompt(agent, root) {
			unresolved = append(unresolved, "agent."+agent.key+".prompt (custom or unrecognized loader preserved)")
		}
		if tools, ok := entry["tools"].(map[string]interface{}); ok && tools["read"] == false {
			unresolved = append(unresolved, "agent."+agent.key+".tools.read=false (absolute loader unavailable)")
		}
	}
	if len(unresolved) != 0 {
		capability.Status = HostCapabilityStatusDegraded
		capability.Reason = "OpenCode source loading unknown at " + openCodeConfigPath(home) + ": " + strings.Join(unresolved, "; ")
		capability.Remediation = "Preserved user-owned configuration; reconcile the listed fields with absolute core/role read paths under " + root + ". " + capability.Remediation
	}
	return capability
}
