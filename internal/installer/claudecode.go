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

	// Add tool permissions for rotta skills to settings.json
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := addClaudeCodePermissions(settingsPath, opts); err != nil {
		// Non-fatal: user can add permissions manually
		_ = err
	}

	return files, nil
}

func cleanPreviousClaudeCodeInstallation(home string) error {
	if err := os.RemoveAll(filepath.Join(home, ".claude", "skills", "rotta")); err != nil {
		return fmt.Errorf("cannot remove stale Claude Code skills: %w", err)
	}
	if err := os.RemoveAll(filepath.Join(home, ".claude", "skills", "clean-workflow")); err != nil {
		return fmt.Errorf("cannot remove legacy Claude Code skills: %w", err)
	}
	if err := cleanClaudeCodeVelaFreshnessGuard(home); err != nil {
		return err
	}
	return cleanClaudeCodePermissions(filepath.Join(home, ".claude", "settings.json"))
}

// addClaudeCodePermissions injects rotta skill triggers into the
// Claude Code settings.json permissions.allow list.
func addClaudeCodePermissions(settingsPath string, opts Options) error {
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

	return os.WriteFile(settingsPath, out, 0o644)
}

func cleanClaudeCodePermissions(settingsPath string) error {
	settings := map[string]interface{}{}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("cannot read settings.json: %w", err)
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("cannot parse settings.json: %w", err)
	}
	permissions, _ := settings["permissions"].(map[string]interface{})
	if permissions == nil {
		return nil
	}
	allow, _ := permissions["allow"].([]interface{})
	if allow == nil {
		return nil
	}

	owned := map[string]bool{}
	for _, entry := range selectedClaudeCodePermissions(Options{InstallSpec: true, InstallImpl: true, InstallReview: true}) {
		owned[entry] = true
	}
	for _, entry := range legacyCleanClaudeCodePermissions() {
		owned[entry] = true
	}
	kept := allow[:0]
	for _, entry := range allow {
		value, _ := entry.(string)
		if !owned[value] {
			kept = append(kept, entry)
		}
	}
	permissions["allow"] = kept
	settings["permissions"] = permissions
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal settings.json: %w", err)
	}
	return os.WriteFile(settingsPath, out, 0o644)
}

func selectedClaudeCodePermissions(opts Options) []string {
	var entries []string
	if opts.InstallSpec {
		entries = append(entries, "mcp__rotta__spec_mode")
	}
	if opts.InstallImpl {
		entries = append(entries, "mcp__rotta__implementation_mode")
	}
	if opts.InstallReview {
		entries = append(entries, "mcp__rotta__review_mode")
	}
	return entries
}

func legacyCleanClaudeCodePermissions() []string {
	return []string{
		"mcp__clean_workflow__spec_mode",
		"mcp__clean_workflow__implementation_mode",
		"mcp__clean_workflow__review_mode",
	}
}
