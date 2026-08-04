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
	prompt      string
	assetPath   string // path inside assets.FS for the SKILL.md content
	skillName   string // directory name under ~/.config/opencode/skills/
	modeFlag    func(opts Options) bool
}

// rottaAgents defines all four Rotta agents in dependency order.
// The orchestrator is always installed; sub-agents depend on mode selection.
var rottaAgents = []agentEntry{
	{
		key:         "rotta-orchestrator",
		description: "Rotta-Orchestrator — Senior Architect Orchestrator",
		mode:        "primary",
		hidden:      false,
		tools:       map[string]bool{"bash": true, "delegate": true, "delegation_list": true, "delegation_read": true, "edit": true, "read": true, "write": true},
		prompt:      "You are Rotta-Orchestrator, the Rotta orchestrator (Senior Architect). Do NOT be a sub-agent executor. Read your full instructions at ~/.config/opencode/skills/rotta-orchestrator/SKILL.md and follow them exactly.",
		assetPath:   "agents/rotta-orchestrator.md",
		skillName:   "rotta-orchestrator",
		modeFlag:    func(_ Options) bool { return true }, // always install
	},
	{
		key:         "rotta-spec",
		description: "Rotta — Spec Partner + Gherkin Author",
		mode:        "subagent",
		hidden:      true,
		tools:       map[string]bool{"bash": false, "edit": true, "read": true, "write": true},
		prompt:      "You are the Rotta Spec sub-agent (Spec Partner + Gherkin Author). Do NOT delegate to other agents. Read your full instructions at ~/.config/opencode/skills/rotta-spec/SKILL.md and follow them exactly.",
		assetPath:   "agents/rotta-spec.md",
		skillName:   "rotta-spec",
		modeFlag:    func(o Options) bool { return o.InstallSpec },
	},
	{
		key:         "rotta-impl",
		description: "Rotta — TDD Craftsman",
		mode:        "subagent",
		hidden:      true,
		tools:       map[string]bool{"bash": true, "edit": true, "read": true, "write": true},
		prompt:      "You are the Rotta Implementation sub-agent (TDD Craftsman). Do NOT delegate to other agents. Read your full instructions at ~/.config/opencode/skills/rotta-impl/SKILL.md and follow them exactly.",
		assetPath:   "agents/rotta-impl.md",
		skillName:   "rotta-impl",
		modeFlag:    func(o Options) bool { return o.InstallImpl },
	},
	{
		key:         "rotta-review",
		description: "Rotta — Judge (Metrics-based Quality Auditor)",
		mode:        "subagent",
		hidden:      true,
		tools:       map[string]bool{"bash": true, "edit": false, "read": true, "write": true},
		prompt:      "You are the Rotta Review sub-agent (Judge). Do NOT delegate to other agents. You review evidence, not code. Read your full instructions at ~/.config/opencode/skills/rotta-review/SKILL.md and follow them exactly.",
		assetPath:   "agents/rotta-review.md",
		skillName:   "rotta-review",
		modeFlag:    func(o Options) bool { return o.InstallReview },
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
	skillsBase := filepath.Join(home, ".config", "opencode", "skills")
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
	removeLegacyOpenCodeAgents(config, agentMap)

	files, err := installOpenCodeAgents(opts, skillsBase, agentMap)
	if err != nil {
		return nil, err
	}
	config["agent"] = agentMap
	if err := writeResolvedOpenCodeConfig(document); err != nil {
		return nil, err
	}
	files = append(files, resolution.Path)

	return files, nil
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

func installOpenCodeAgents(opts Options, skillsBase string, agentMap map[string]interface{}) ([]string, error) {
	var files []string
	for _, agent := range rottaAgents {
		if !agent.modeFlag(opts) {
			continue
		}
		skillFile, err := writeOpenCodeSkill(opts, skillsBase, agent)
		if err != nil {
			return nil, err
		}
		files = append(files, skillFile)
		if _, exists := agentMap[agent.key]; !exists {
			agentMap[agent.key] = openCodeAgentEntry(agent)
		}
	}
	return files, nil
}

func writeOpenCodeSkill(opts Options, skillsBase string, agent agentEntry) (string, error) {
	skillDir := filepath.Join(skillsBase, agent.skillName)
	if err := os.MkdirAll(skillDir, 0o750); err != nil {
		return "", fmt.Errorf("cannot create skill dir %s: %w", skillDir, err)
	}
	data, err := readRenderedAsset(agent.assetPath, opts)
	if err != nil {
		return "", fmt.Errorf("cannot read embedded %s: %w", agent.assetPath, err)
	}
	skillFile := filepath.Join(skillDir, "SKILL.md")
	if err := writePrivateFile(skillFile, data, 0o600); err != nil {
		return "", fmt.Errorf("cannot write %s: %w", skillFile, err)
	}
	return skillFile, nil
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
	return entry
}

func cleanPreviousOpenCodeInstallation(opts Options, home string) error {
	resolution, err := resolveOpenCodeConfig(opts, home)
	if err != nil {
		return err
	}
	document, err := readResolvedOpenCodeConfig(resolution)
	if err != nil {
		return err
	}
	config := document.config
	agentMap, _ := config["agent"].(map[string]interface{})
	if agentMap != nil {
		changed := false
		for _, agent := range rottaAgents {
			if _, exists := agentMap[agent.key]; exists {
				delete(agentMap, agent.key)
				changed = true
			}
		}
		if removeLegacyOpenCodeAgents(config, agentMap) {
			changed = true
		}
		if changed {
			config["agent"] = agentMap
			if err := writeResolvedOpenCodeConfig(document); err != nil {
				return err
			}
		}
	}

	for _, agent := range rottaAgents {
		path := filepath.Join(home, ".config", "opencode", "skills", agent.skillName)
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("cannot remove stale opencode skill %s: %w", path, err)
		}
	}
	for _, skillName := range append(legacyBobOpenCodeAgentKeys, legacyCleanOpenCodeAgentKeys...) {
		path := filepath.Join(home, ".config", "opencode", "skills", skillName)
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("cannot remove legacy opencode skill %s: %w", path, err)
		}
	}
	if err := cleanOpenCodeVelaFreshnessGuard(home); err != nil {
		return err
	}
	return nil
}

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
