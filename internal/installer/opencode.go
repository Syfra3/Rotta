package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// agentEntry defines one OpenCode agent entry for opencode.json.
type agentEntry struct {
	key         string
	description string
	mode        string
	hidden      bool
	tools       map[string]bool
	permission  map[string]string
	prompt      string
	assetPath   string // path inside assets.FS for the SKILL.md content
	skillName   string // directory name under ~/.config/opencode/skills/
}

// rottaAgents defines the complete Rotta Next role surface.
var rottaAgents = []agentEntry{
	{
		key:         "rotta-orchestrator",
		description: "Rotta Next — lightweight Fast/Strict router",
		mode:        "primary",
		hidden:      false,
		tools:       map[string]bool{"bash": false, "delegate": true, "delegation_list": true, "delegation_read": true, "edit": false, "read": true, "write": false},
		permission:  map[string]string{"question": "allow"},
		prompt:      "You are Rotta-Orchestrator. Load rotta-core and rotta-orchestrator from ~/.config/opencode/skills/rotta-next/ before acting. Do not implement code or execute ordinary operations.",
		assetPath:   "agents/rotta-orchestrator.md",
		skillName:   "rotta-orchestrator",
	},
	{
		key:         "rotta-explore",
		description: "Rotta Next — bounded discovery",
		mode:        "subagent",
		hidden:      true,
		tools:       map[string]bool{"bash": false, "edit": false, "read": true, "write": false},
		permission:  map[string]string{"question": "deny"},
		prompt:      "You are the Rotta Explore subagent. Load rotta-core and rotta-explore from ~/.config/opencode/skills/rotta-next/ before acting. Perform bounded read-only discovery only.",
		assetPath:   "agents/rotta-explore.md",
		skillName:   "rotta-explore",
	},
	{
		key:         "rotta-impl",
		description: "Rotta Next — coherent implementation slices",
		mode:        "subagent",
		hidden:      true,
		tools:       map[string]bool{"bash": true, "edit": true, "read": true, "write": true},
		permission:  map[string]string{"question": "deny"},
		prompt:      "You are the Rotta Implementation subagent. Load rotta-core and rotta-impl from ~/.config/opencode/skills/rotta-next/ before acting. Implement only the assigned coherent slice.",
		assetPath:   "agents/rotta-impl.md",
		skillName:   "rotta-impl",
	},
	{
		key:         "rotta-review",
		description: "Rotta Next — independent diff review",
		mode:        "subagent",
		hidden:      true,
		tools:       map[string]bool{"bash": true, "edit": false, "read": true, "write": false},
		permission:  map[string]string{"question": "deny"},
		prompt:      "You are the Rotta Review subagent. Load rotta-core and rotta-review from ~/.config/opencode/skills/rotta-next/ before acting. Inspect the diff and affected code independently.",
		assetPath:   "agents/rotta-review.md",
		skillName:   "rotta-review",
	},
	{
		key:         "rotta-ops",
		description: "Rotta Next — explicit operations",
		mode:        "subagent",
		hidden:      true,
		tools:       map[string]bool{"bash": true, "edit": false, "read": true, "write": false},
		permission:  map[string]string{"question": "deny"},
		prompt:      "You are the Rotta Operations subagent. Load rotta-core and rotta-ops from ~/.config/opencode/skills/rotta-next/ before acting. Execute only explicit bounded operations.",
		assetPath:   "agents/rotta-ops.md",
		skillName:   "rotta-ops",
	},
}

var legacyBobOpenCodeAgentKeys = []string{
	"bob-orchestrator",
	"bob-spec",
	"bob-impl",
	"bob-review",
}

var legacyCleanOpenCodeAgentKeys = []string{
	"clean-orchestrator",
	"clean-spec",
	"clean-impl",
	"clean-review",
}

// installOpenCode writes skill files to ~/.config/opencode/skills/<name>/SKILL.md
// and adds agent entries to ~/.config/opencode/opencode.json under the "agent" key.
func installOpenCode(opts Options, home string) ([]string, error) {
	resolution, err := resolveOpenCodeConfig(opts, home)
	if err != nil {
		return nil, err
	}
	document, err := readResolvedOpenCodeConfig(resolution)
	if err != nil {
		return nil, err
	}
	config := document.config
	if err := applyOpenCodeContextProfile(config); err != nil {
		return nil, err
	}
	agentMap, _ := config["agent"].(map[string]interface{})
	if agentMap == nil {
		agentMap = map[string]interface{}{}
	}
	if err := validateOpenCodeAgentOwnership(home, resolution.Path, agentMap); err != nil {
		return nil, err
	}
	managed, err := openCodeManagedSkills(opts, home)
	if err != nil {
		return nil, err
	}
	if _, err := validateManagedFiles(home, managed); err != nil {
		return nil, err
	}
	original, readErr := readPrivateFile(resolution.Path)
	originalExists := readErr == nil
	if readErr != nil && !os.IsNotExist(readErr) {
		return nil, fmt.Errorf("read existing OpenCode config: %w", readErr)
	}
	for _, agent := range rottaAgents {
		agentMap[agent.key] = openCodeAgentEntry(agent)
	}
	delete(agentMap, "rotta-spec")
	removeLegacyOpenCodeAgents(config, agentMap)
	config["agent"] = agentMap
	if err := writeResolvedOpenCodeConfig(document); err != nil {
		return nil, err
	}
	files, err := installManagedFiles(home, managed)
	if err != nil {
		if restoreErr := restoreOpenCodeConfig(resolution.Path, original, originalExists); restoreErr != nil {
			return nil, fmt.Errorf("install OpenCode role files: %w; restore config: %v", err, restoreErr)
		}
		return nil, err
	}
	if err := recordOpenCodeAgentOwnership(home, resolution.Path, agentMap); err != nil {
		return nil, err
	}
	files = append(files, resolution.Path)

	return files, nil
}

func restoreOpenCodeConfig(path string, data []byte, existed bool) error {
	if !existed {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return writePrivateFile(path, data, 0o600)
}

func validateOpenCodeAgentOwnership(home, configPath string, agentMap map[string]interface{}) error {
	manifestPath := filepath.Join(home, ".config", "rotta", "managed-artifacts.json")
	manifest, err := readManagedArtifactsManifest(manifestPath)
	if err != nil {
		return err
	}
	for _, agent := range rottaAgents {
		current, exists := agentMap[agent.key]
		if !exists {
			continue
		}
		want, managed := manifest.Files[openCodeAgentOwnershipKey(configPath, agent.key)]
		if !managed {
			return fmt.Errorf("refusing to overwrite unmanaged OpenCode agent: %s", agent.key)
		}
		data, err := json.Marshal(current)
		if err != nil {
			return fmt.Errorf("serialize OpenCode agent %s: %w", agent.key, err)
		}
		if contentDigest(data) != want {
			return fmt.Errorf("refusing to overwrite modified managed OpenCode agent: %s", agent.key)
		}
	}
	return nil
}

func recordOpenCodeAgentOwnership(home, configPath string, agentMap map[string]interface{}) error {
	manifestPath := filepath.Join(home, ".config", "rotta", "managed-artifacts.json")
	manifest, err := readManagedArtifactsManifest(manifestPath)
	if err != nil {
		return err
	}
	for _, agent := range rottaAgents {
		data, err := json.Marshal(agentMap[agent.key])
		if err != nil {
			return fmt.Errorf("serialize OpenCode agent %s: %w", agent.key, err)
		}
		manifest.Files[openCodeAgentOwnershipKey(configPath, agent.key)] = contentDigest(data)
	}
	return writeManagedArtifactsManifest(home, manifest)
}

func openCodeAgentOwnershipKey(configPath, agentKey string) string {
	return configPath + "#agent:" + agentKey
}

func applyOpenCodeContextProfile(config map[string]interface{}) error {
	compaction, err := openCodeConfigurationObject(config, "compaction")
	if err != nil {
		return err
	}
	compaction["auto"] = true
	compaction["prune"] = true
	compaction["buffer"] = 10000

	toolOutput, err := openCodeConfigurationObject(config, "tool_output")
	if err != nil {
		return err
	}
	toolOutput["max_lines"] = 120
	toolOutput["max_bytes"] = 12288
	return nil
}

func openCodeConfigurationObject(config map[string]interface{}, key string) (map[string]interface{}, error) {
	if value, exists := config[key]; exists {
		object, ok := value.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("cannot apply OpenCode context profile: %s is not an object", key)
		}
		return object, nil
	}
	object := map[string]interface{}{}
	config[key] = object
	return object, nil
}

func openCodeManagedSkills(opts Options, home string) (map[string][]byte, error) {
	skillsBase := filepath.Join(home, ".config", "opencode", "skills")
	managed := map[string][]byte{}
	for _, agent := range rottaAgents {
		data, err := readRenderedAsset(agent.assetPath, opts)
		if err != nil {
			return nil, fmt.Errorf("cannot read embedded %s: %w", agent.assetPath, err)
		}
		managed[filepath.Join(skillsBase, "rotta-next", agent.skillName, "SKILL.md")] = data
	}
	core, err := readRenderedAsset("core/rotta-core.md", opts)
	if err != nil {
		return nil, err
	}
	managed[filepath.Join(skillsBase, "rotta-next", "rotta-core", "SKILL.md")] = core
	return managed, nil
}

func openCodeAgentEntry(agent agentEntry) map[string]interface{} {
	tools := map[string]interface{}{}
	for key, value := range agent.tools {
		tools[key] = value
	}
	entry := map[string]interface{}{
		"description": agent.description,
		"mode":        agent.mode,
		"prompt":      agent.prompt,
		"tools":       tools,
	}
	if agent.hidden {
		entry["hidden"] = true
	}
	if len(agent.permission) != 0 {
		permission := map[string]string{}
		for key, value := range agent.permission {
			permission[key] = value
		}
		entry["permission"] = permission
	}
	return entry
}

func cleanPreviousOpenCodeInstallation(_ Options, _ string) error { return nil }

func removeLegacyOpenCodeAgents(config map[string]interface{}, agentMap map[string]interface{}) bool {
	changed := false
	for _, key := range append(legacyBobOpenCodeAgentKeys, legacyCleanOpenCodeAgentKeys...) {
		if _, exists := agentMap[key]; exists {
			delete(agentMap, key)
			changed = true
		}
	}
	if config["default_agent"] == "bob-orchestrator" || config["default_agent"] == "clean-orchestrator" {
		config["default_agent"] = "rotta-orchestrator"
		changed = true
	}
	return changed
}

func readOpenCodeConfig(path string) (map[string]interface{}, error) {
	config := map[string]interface{}{}
	data, err := readPrivateFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return nil, fmt.Errorf("cannot read opencode.json: %w", err)
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("cannot parse %s: %w", path, err)
	}
	return config, nil
}

func writeOpenCodeConfig(path string, config map[string]interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("cannot create config dir: %w", err)
	}
	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal opencode.json: %w", err)
	}
	return writePrivateFile(path, out, 0o600)
}
