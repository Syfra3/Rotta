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
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		return nil, fmt.Errorf("cannot create ~/.claude/skills: %w", err)
	}

	files, err := copySkillsToDir(opts, skillsDir)
	if err != nil {
		return nil, err
	}

	// Add tool permissions for uncle-bob skills to settings.json
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := addClaudeCodePermissions(settingsPath); err != nil {
		// Non-fatal: user can add permissions manually
		_ = err
	}

	return files, nil
}

// addClaudeCodePermissions injects uncle-bob skill triggers into the
// Claude Code settings.json permissions.allow list.
func addClaudeCodePermissions(settingsPath string) error {
	settings := map[string]interface{}{}

	data, err := os.ReadFile(settingsPath)
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

	newEntries := []string{
		"mcp__uncle_bob__spec_mode",
		"mcp__uncle_bob__implementation_mode",
		"mcp__uncle_bob__review_mode",
	}

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

	return os.WriteFile(settingsPath, out, 0o644)
}
