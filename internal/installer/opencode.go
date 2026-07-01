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
	var files []string

	skillsBase := filepath.Join(home, ".config", "opencode", "skills")
	configPath := filepath.Join(home, ".config", "opencode", "opencode.json")

	// Read current opencode.json (or start fresh if it doesn't exist)
	config, err := readOpenCodeConfig(configPath)
	if err != nil {
		return nil, err
	}

	// Ensure top-level "agent" key exists
	agentMap, _ := config["agent"].(map[string]interface{})
	if agentMap == nil {
		agentMap = map[string]interface{}{}
	}
	removeLegacyOpenCodeAgents(config, agentMap)

	for _, a := range rottaAgents {
		if !a.modeFlag(opts) {
			continue
		}

		// Write skill file
		skillDir := filepath.Join(skillsBase, a.skillName)
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			return nil, fmt.Errorf("cannot create skill dir %s: %w", skillDir, err)
		}
		data, err := readRenderedAsset(a.assetPath, opts)
		if err != nil {
			return nil, fmt.Errorf("cannot read embedded %s: %w", a.assetPath, err)
		}
		skillFile := filepath.Join(skillDir, "SKILL.md")
		if err := os.WriteFile(skillFile, data, 0o644); err != nil {
			return nil, fmt.Errorf("cannot write %s: %w", skillFile, err)
		}
		files = append(files, skillFile)

		// Build agent JSON entry
		toolMap := map[string]interface{}{}
		for k, v := range a.tools {
			toolMap[k] = v
		}

		entry := map[string]interface{}{
			"description": a.description,
			"mode":        a.mode,
			"prompt":      a.prompt,
			"tools":       toolMap,
		}
		if a.hidden {
			entry["hidden"] = true
		}

		// Only add if not already present (don't overwrite a user-customised entry)
		if _, exists := agentMap[a.key]; !exists {
			agentMap[a.key] = entry
		}
	}

	config["agent"] = agentMap

	if err := writeOpenCodeConfig(configPath, config); err != nil {
		return nil, err
	}
	files = append(files, configPath)

	return files, nil
}

func cleanPreviousOpenCodeInstallation(home string) error {
	configPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	config, err := readOpenCodeConfig(configPath)
	if err != nil {
		return err
	}
	agentMap, _ := config["agent"].(map[string]interface{})
	if agentMap != nil {
		for _, a := range rottaAgents {
			delete(agentMap, a.key)
		}
		removeLegacyOpenCodeAgents(config, agentMap)
		config["agent"] = agentMap
		if err := writeOpenCodeConfig(configPath, config); err != nil {
			return err
		}
	}

	for _, a := range rottaAgents {
		path := filepath.Join(home, ".config", "opencode", "skills", a.skillName)
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

func removeLegacyOpenCodeAgents(config map[string]interface{}, agentMap map[string]interface{}) {
	for _, key := range append(legacyBobOpenCodeAgentKeys, legacyCleanOpenCodeAgentKeys...) {
		delete(agentMap, key)
	}
	if config["default_agent"] == "bob-orchestrator" || config["default_agent"] == "clean-orchestrator" {
		config["default_agent"] = "rotta-orchestrator"
	}
}

func readOpenCodeConfig(path string) (map[string]interface{}, error) {
	config := map[string]interface{}{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return nil, fmt.Errorf("cannot read opencode.json: %w", err)
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("cannot parse opencode.json: %w", err)
	}
	return config, nil
}

func writeOpenCodeConfig(path string, config map[string]interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("cannot create config dir: %w", err)
	}
	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal opencode.json: %w", err)
	}
	return os.WriteFile(path, out, 0o644)
}
