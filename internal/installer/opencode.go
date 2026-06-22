package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Syfra3/uncle-bob-workflow/assets"
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

// bobAgents defines all four Bob workflow agents in dependency order.
// The orchestrator is always installed; sub-agents depend on mode selection.
var bobAgents = []agentEntry{
	{
		key:         "bob-orchestrator",
		description: "Uncle Bob Workflow — Senior Architect Orchestrator",
		mode:        "primary",
		hidden:      false,
		tools:       map[string]bool{"bash": true, "delegate": true, "delegation_list": true, "delegation_read": true, "edit": true, "read": true, "write": true},
		prompt:      "You are the Uncle Bob workflow orchestrator (Senior Architect). Do NOT be a sub-agent executor. Read your full instructions at ~/.config/opencode/skills/bob-orchestrator/SKILL.md and follow them exactly.",
		assetPath:   "agents/bob-orchestrator.md",
		skillName:   "bob-orchestrator",
		modeFlag:    func(_ Options) bool { return true }, // always install
	},
	{
		key:         "bob-spec",
		description: "Uncle Bob — Spec Partner + Gherkin Author",
		mode:        "subagent",
		hidden:      true,
		tools:       map[string]bool{"bash": false, "edit": true, "read": true, "write": true},
		prompt:      "You are the Bob Spec sub-agent (Spec Partner + Gherkin Author). Do NOT delegate to other agents. Read your full instructions at ~/.config/opencode/skills/bob-spec/SKILL.md and follow them exactly.",
		assetPath:   "agents/bob-spec.md",
		skillName:   "bob-spec",
		modeFlag:    func(o Options) bool { return o.InstallSpec },
	},
	{
		key:         "bob-impl",
		description: "Uncle Bob — TDD Craftsman",
		mode:        "subagent",
		hidden:      true,
		tools:       map[string]bool{"bash": true, "edit": true, "read": true, "write": true},
		prompt:      "You are the Bob Implementation sub-agent (TDD Craftsman). Do NOT delegate to other agents. Read your full instructions at ~/.config/opencode/skills/bob-impl/SKILL.md and follow them exactly.",
		assetPath:   "agents/bob-impl.md",
		skillName:   "bob-impl",
		modeFlag:    func(o Options) bool { return o.InstallImpl },
	},
	{
		key:         "bob-review",
		description: "Uncle Bob — Judge (Metrics-based Quality Auditor)",
		mode:        "subagent",
		hidden:      true,
		tools:       map[string]bool{"bash": true, "edit": false, "read": true, "write": true},
		prompt:      "You are the Bob Review sub-agent (Judge). Do NOT delegate to other agents. You review evidence, not code. Read your full instructions at ~/.config/opencode/skills/bob-review/SKILL.md and follow them exactly.",
		assetPath:   "agents/bob-review.md",
		skillName:   "bob-review",
		modeFlag:    func(o Options) bool { return o.InstallReview },
	},
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

	for _, a := range bobAgents {
		if !a.modeFlag(opts) {
			continue
		}

		// Write skill file
		skillDir := filepath.Join(skillsBase, a.skillName)
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			return nil, fmt.Errorf("cannot create skill dir %s: %w", skillDir, err)
		}
		data, err := assets.FS.ReadFile(a.assetPath)
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
