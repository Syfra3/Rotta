package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// installClaudeCode copies skills and patches Claude Code settings.
func installClaudeCode(opts Options, home string) ([]string, error) {
	skillsDir := filepath.Join(home, ".claude", "skills")
	agentsDir := filepath.Join(home, ".claude", "agents")
	managed := map[string][]byte{}
	core, err := readRenderedAsset("core/rotta-core.md", opts)
	if err != nil {
		return nil, fmt.Errorf("cannot read embedded core policy: %w", err)
	}
	managed[filepath.Join(skillsDir, "rotta-next", "rotta-core", "SKILL.md")] = core
	for _, agent := range rottaAgents {
		data, err := readRenderedAsset(agent.assetPath, opts)
		if err != nil {
			return nil, fmt.Errorf("cannot read embedded %s: %w", agent.assetPath, err)
		}
		managed[filepath.Join(skillsDir, "rotta-next", agent.skillName, "SKILL.md")] = data
		managed[filepath.Join(agentsDir, agent.key+".md")] = data
	}
	return installManagedFiles(home, managed)
}

func installClaudeCodeAgents(opts Options, agentsDir string) ([]string, error) {
	managed := map[string][]byte{}
	for _, agent := range rottaAgents {
		data, err := readRenderedAsset(agent.assetPath, opts)
		if err != nil {
			return nil, fmt.Errorf("cannot read embedded %s: %w", agent.assetPath, err)
		}
		managed[filepath.Join(agentsDir, agent.key+".md")] = data
	}
	home := filepath.Clean(filepath.Join(agentsDir, "..", ".."))
	return installManagedFiles(home, managed)
}

func cleanPreviousClaudeCodeInstallation(home string) error {
	return nil
}

// addClaudeCodePermissions injects rotta skill triggers into the
// Claude Code settings.json permissions.allow list.
func addClaudeCodePermissions(settingsPath string, opts Options) error {
	settings := map[string]interface{}{}

	data, err := readPrivateFile(settingsPath)
	if err == nil {
		if jsonErr := json.Unmarshal(data, &settings); jsonErr != nil {
			return fmt.Errorf("cannot parse settings.json: %w", jsonErr)
		}
	}

	permissions, _ := settings["permissions"].(map[string]interface{})
	if permissions == nil {
		permissions = map[string]interface{}{}
	}

	allow, _ := permissions["allow"].([]interface{})

	newEntries := selectedClaudeCodePermissions(opts)

	existing := make(map[string]bool)
	for _, a := range allow {
		if s, ok := a.(string); ok {
			existing[s] = true
		}
	}

	for _, entry := range newEntries {
		if !existing[entry] {
			allow = append(allow, entry)
		}
	}

	permissions["allow"] = allow
	settings["permissions"] = permissions

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal settings.json: %w", err)
	}

	return writePrivateFile(settingsPath, out, 0o600)
}

func cleanClaudeCodePermissions(settingsPath string) error {
	settings, err := readClaudeCodeSettings(settingsPath)
	if err != nil {
		return err
	}
	if settings == nil {
		return nil
	}
	permissions, _ := settings["permissions"].(map[string]interface{})
	if permissions == nil {
		return nil
	}
	allow, _ := permissions["allow"].([]interface{})
	if allow == nil {
		return nil
	}

	permissions["allow"] = removeOwnedClaudeCodePermissions(allow)
	settings["permissions"] = permissions
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal settings.json: %w", err)
	}
	return writePrivateFile(settingsPath, out, 0o600)
}

func readClaudeCodeSettings(settingsPath string) (map[string]interface{}, error) {
	settings := map[string]interface{}{}
	data, err := readPrivateFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read settings.json: %w", err)
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("cannot parse settings.json: %w", err)
	}
	return settings, nil
}

func removeOwnedClaudeCodePermissions(allow []interface{}) []interface{} {
	owned := allOwnedClaudeCodePermissions()
	kept := allow[:0]
	for _, entry := range allow {
		value, _ := entry.(string)
		if !owned[value] {
			kept = append(kept, entry)
		}
	}
	return kept
}

func allOwnedClaudeCodePermissions() map[string]bool {
	owned := map[string]bool{}
	for _, entry := range []string{"mcp__rotta__spec_mode", "mcp__rotta__implementation_mode", "mcp__rotta__review_mode"} {
		owned[entry] = true
	}
	for _, entry := range legacyCleanClaudeCodePermissions() {
		owned[entry] = true
	}
	return owned
}

func selectedClaudeCodePermissions(opts Options) []string {
	return nil
}

func legacyCleanClaudeCodePermissions() []string {
	return []string{
		"mcp__clean_workflow__spec_mode",
		"mcp__clean_workflow__implementation_mode",
		"mcp__clean_workflow__review_mode",
	}
}
