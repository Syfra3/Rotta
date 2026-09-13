package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// routingFailureHook is test-only fault injection for transaction boundaries.
var routingFailureHook func(stage string) error

type routingFileSnapshot struct {
	data   []byte
	exists bool
}

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
		key:         "rotta-architect",
		description: "Rotta Next — architecture findings",
		mode:        "subagent",
		hidden:      true,
		tools:       map[string]bool{"bash": false, "edit": false, "read": true, "write": false},
		permission:  map[string]string{"question": "deny"},
		prompt:      "You are the Rotta Architect subagent. Load rotta-core and rotta-architect from ~/.config/opencode/skills/rotta-next/ before acting. Produce read-only architecture findings only.",
		assetPath:   "agents/rotta-architect.md",
		skillName:   "rotta-architect",
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
		key:         "rotta-cleaner",
		description: "Rotta Next — behavior-preserving cleanup",
		mode:        "subagent",
		hidden:      true,
		tools:       map[string]bool{"bash": true, "edit": true, "read": true, "write": true},
		permission:  map[string]string{"question": "deny"},
		prompt:      "You are the Rotta Cleaner subagent. Load rotta-core and rotta-cleaner from ~/.config/opencode/skills/rotta-next/ before acting. Preserve behavior and verify focused cleanup only.",
		assetPath:   "agents/rotta-cleaner.md",
		skillName:   "rotta-cleaner",
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
	if opts.skipOpenCodeRouting {
		return nil, nil
	}
	routing, err := opts.ModelRouting.resolved()
	if err != nil {
		return nil, err
	}
	resolution, err := resolveOpenCodeConfig(opts, home)
	if err != nil {
		return nil, err
	}
	if err := validateManagedParents(home, resolution.Path); err != nil {
		return nil, err
	}
	document, err := readResolvedOpenCodeConfig(resolution)
	if err != nil {
		return nil, err
	}
	config := document.config
	agentMap, _ := config["agent"].(map[string]interface{})
	if agentMap == nil {
		agentMap = map[string]interface{}{}
	}
	if err := validateOpenCodeModelOwnership(home, resolution.Path, agentMap, routing); err != nil {
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
	manifestPath := managedArtifactsManifestPath(home)
	manifestOriginal, manifestReadErr := readPrivateFile(manifestPath)
	manifestExists := manifestReadErr == nil
	if manifestReadErr != nil && !os.IsNotExist(manifestReadErr) {
		return nil, fmt.Errorf("read existing routing state: %w", manifestReadErr)
	}
	assetOriginal, err := snapshotRoutingFiles(managed)
	if err != nil {
		return nil, err
	}
	configChanged := false
	for _, agent := range rottaAgents {
		if _, exists := agentMap[agent.key]; !exists {
			agentMap[agent.key] = openCodeAgentEntry(agent)
			configChanged = true
		}
	}
	config["agent"] = agentMap
	if routing == ModelRoutingEnabled {
		configChanged = applyOpenCodeRoutingModels(agentMap) || configChanged
	} else {
		configChanged = removeOpenCodeRoutingModels(agentMap) || configChanged
	}
	if configChanged {
		if err := writeResolvedOpenCodeConfig(document); err != nil {
			return nil, err
		}
	}
	if configChanged {
		if err := failRoutingAt("after-config-write"); err != nil {
			return nil, rollbackOpenCodeRouting(err, resolution.Path, original, originalExists, manifestPath, manifestOriginal, manifestExists, assetOriginal)
		}
	}
	files, err := installManagedFiles(home, managed)
	if err != nil {
		return nil, rollbackOpenCodeRouting(err, resolution.Path, original, originalExists, manifestPath, manifestOriginal, manifestExists, assetOriginal)
	}
	if err := failRoutingAt("after-assets"); err != nil {
		return nil, rollbackOpenCodeRouting(err, resolution.Path, original, originalExists, manifestPath, manifestOriginal, manifestExists, assetOriginal)
	}
	if err := recordOpenCodeAgentOwnership(home, resolution.Path, agentMap, routing); err != nil {
		return nil, rollbackOpenCodeRouting(err, resolution.Path, original, originalExists, manifestPath, manifestOriginal, manifestExists, assetOriginal)
	}
	if err := failRoutingAt("after-managed-state"); err != nil {
		return nil, rollbackOpenCodeRouting(err, resolution.Path, original, originalExists, manifestPath, manifestOriginal, manifestExists, assetOriginal)
	}
	files = append(files, resolution.Path)

	return files, nil
}

func failRoutingAt(stage string) error {
	if routingFailureHook == nil {
		return nil
	}
	return routingFailureHook(stage)
}

func rollbackOpenCodeRouting(cause error, configPath string, config []byte, configExists bool, manifestPath string, manifest []byte, manifestExists bool, assets map[string]routingFileSnapshot) error {
	if err := restoreOpenCodeConfig(configPath, config, configExists); err != nil {
		return fmt.Errorf("routing failed: %w; config compensation failed: %v", cause, err)
	}
	if err := restoreOpenCodeConfig(manifestPath, manifest, manifestExists); err != nil {
		return fmt.Errorf("routing failed: %w; managed-state compensation failed: %v", cause, err)
	}
	for path, snapshot := range assets {
		if err := restoreOpenCodeConfig(path, snapshot.data, snapshot.exists); err != nil {
			return fmt.Errorf("routing failed: %w; asset compensation failed: %v", cause, err)
		}
	}
	return cause
}

func snapshotRoutingFiles(files map[string][]byte) (map[string]routingFileSnapshot, error) {
	snapshots := make(map[string]routingFileSnapshot, len(files))
	for path := range files {
		data, err := readPrivateFile(path)
		if err == nil {
			snapshots[path] = routingFileSnapshot{data: data, exists: true}
			continue
		}
		if os.IsNotExist(err) {
			snapshots[path] = routingFileSnapshot{}
			continue
		}
		return nil, fmt.Errorf("snapshot routing asset %s: %w", path, err)
	}
	return snapshots, nil
}

// immutableOpenCodeRouting maps the only v1 installer-managed model fields.
var immutableOpenCodeRouting = map[string]string{
	"rotta-orchestrator": "openai/gpt-5.6-sol",
	"rotta-architect":    "openai/gpt-5.6-sol",
	"rotta-review":       "openai/gpt-5.6-sol",
	"rotta-impl":         "openai/gpt-5.6-terra",
	"rotta-ops":          "openai/gpt-5.6-luna",
	"rotta-explore":      "openai/gpt-5.6-luna",
	"rotta-cleaner":      "openai/gpt-5.6-luna",
}

// preflightSelectedOpenCodeInstall validates routing ownership and every
// routing-managed target before an installation transaction creates a backup.
// A fully reconciled OpenCode-only request is a true no-op and needs no backup.
func preflightSelectedOpenCodeInstall(opts Options, home string) (bool, error) {
	if !targetsOpenCode(opts.Target) {
		return false, nil
	}
	resolution, err := resolveOpenCodeConfig(opts, home)
	if err != nil {
		return false, fmt.Errorf("effective-config resolution blocked: %w", err)
	}
	document, err := readResolvedOpenCodeConfig(resolution)
	if err != nil {
		return false, fmt.Errorf("schema validation blocked: %w", err)
	}
	if err := validateOpenCodeConfigurationShape(document.config); err != nil {
		return false, fmt.Errorf("schema validation blocked: %w", err)
	}
	routing, err := opts.ModelRouting.resolved()
	if err != nil {
		return false, err
	}
	agents, _ := document.config["agent"].(map[string]interface{})
	if agents == nil {
		agents = map[string]interface{}{}
	}
	if err := validateOpenCodeModelOwnership(home, resolution.Path, agents, routing); err != nil {
		return false, err
	}
	managed, err := openCodeManagedSkills(opts, home)
	if err != nil {
		return false, err
	}
	manifest, err := validateManagedFiles(home, managed)
	if err != nil {
		return false, err
	}
	return !openCodeRoutingConfigurationNeedsChange(document.config, agents, routing) &&
		!managedFilesNeedUpdate(manifest, managed) &&
		!openCodeRoutingOwnershipNeedsChange(manifest, resolution.Path, routing), nil
}

func openCodeRoutingConfigurationNeedsChange(config, agents map[string]interface{}, routing ModelRoutingRequest) bool {
	for _, agent := range rottaAgents {
		if _, exists := agents[agent.key]; !exists {
			return true
		}
	}
	for role, expected := range immutableOpenCodeRouting {
		agent := agents[role].(map[string]interface{})
		_, hasModel := agent["model"]
		if routing == ModelRoutingEnabled && agent["model"] != expected {
			return true
		}
		if routing == ModelRoutingDisabled && hasModel {
			return true
		}
	}
	return false
}

func managedFilesNeedUpdate(manifest managedArtifactsManifest, files map[string][]byte) bool {
	for path, data := range files {
		if manifest.Files[path] != contentDigest(data) {
			return true
		}
		current, err := readPrivateFile(path)
		if err != nil || string(current) != string(data) {
			return true
		}
	}
	return false
}

func openCodeRoutingOwnershipNeedsChange(manifest managedArtifactsManifest, configPath string, routing ModelRoutingRequest) bool {
	for role, model := range immutableOpenCodeRouting {
		key := openCodeModelOwnershipKey(configPath, role)
		_, exists := manifest.Files[key]
		if routing == ModelRoutingEnabled && manifest.Files[key] != contentDigest([]byte(model)) {
			return true
		}
		if routing == ModelRoutingDisabled && exists {
			return true
		}
	}
	return false
}

func applyOpenCodeRoutingModels(agentMap map[string]interface{}) bool {
	changed := false
	for role, model := range immutableOpenCodeRouting {
		agent, ok := agentMap[role].(map[string]interface{})
		if !ok {
			continue
		}
		if agent["model"] != model {
			agent["model"] = model
			changed = true
		}
	}
	return changed
}

func removeOpenCodeRoutingModels(agentMap map[string]interface{}) bool {
	changed := false
	for role := range immutableOpenCodeRouting {
		if agent, ok := agentMap[role].(map[string]interface{}); ok {
			if _, exists := agent["model"]; exists {
				delete(agent, "model")
				changed = true
			}
		}
	}
	return changed
}

func restoreOpenCodeConfig(path string, data []byte, existed bool) error {
	if !existed {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return writePrivateFile(path, data, 0o600)
}

func validateOpenCodeModelOwnership(home, configPath string, agentMap map[string]interface{}, routing ModelRoutingRequest) error {
	manifestPath := managedArtifactsManifestPath(home)
	manifest, err := readManagedArtifactsManifest(manifestPath)
	if err != nil {
		return err
	}
	for key := range manifest.Files {
		prefix := configPath + "#agent:"
		if !strings.HasPrefix(key, prefix) || !strings.HasSuffix(key, ".model") {
			continue
		}
		role := strings.TrimSuffix(strings.TrimPrefix(key, prefix), ".model")
		if _, known := immutableOpenCodeRouting[role]; !known {
			return routingConflict(configPath, role, "unrecognized ownership record")
		}
	}
	for role, expected := range immutableOpenCodeRouting {
		raw, exists := agentMap[role]
		if !exists {
			continue
		}
		agent, ok := raw.(map[string]interface{})
		if !ok {
			return routingConflict(configPath, role, "agent is not an object")
		}
		current, hasModel := agent["model"]
		model, isString := current.(string)
		owned := manifest.Files[openCodeModelOwnershipKey(configPath, role)] == contentDigest([]byte(expected))
		switch routing {
		case ModelRoutingEnabled:
			if !hasModel {
				continue
			}
			if !isString || model != expected || !owned {
				return routingConflict(configPath, role, "non-owned or diverged model")
			}
		case ModelRoutingDisabled:
			if !hasModel && !owned {
				continue
			}
			if !hasModel || !isString || model != expected || !owned {
				return routingConflict(configPath, role, "ownership cannot prove the matching model")
			}
		}
	}
	return nil
}

func routingConflict(configPath, role, reason string) error {
	return fmt.Errorf("refusing OpenCode model routing at %s agent.%s.model: %s; remediation: preserve the user value or restore the recorded Rotta model", configPath, role, reason)
}

func recordOpenCodeAgentOwnership(home, configPath string, agentMap map[string]interface{}, routing ModelRoutingRequest) error {
	manifestPath := managedArtifactsManifestPath(home)
	manifest, err := readManagedArtifactsManifest(manifestPath)
	if err != nil {
		return err
	}
	changed := false
	for _, agent := range rottaAgents {
		if model, ok := immutableOpenCodeRouting[agent.key]; ok && routing == ModelRoutingEnabled {
			key := openCodeModelOwnershipKey(configPath, agent.key)
			if manifest.Files[key] != contentDigest([]byte(model)) {
				manifest.Files[key] = contentDigest([]byte(model))
				changed = true
			}
		} else {
			key := openCodeModelOwnershipKey(configPath, agent.key)
			if _, exists := manifest.Files[key]; exists {
				delete(manifest.Files, key)
				changed = true
			}
		}
	}
	if !changed {
		return nil
	}
	return writeManagedArtifactsManifest(home, manifest)
}

func openCodeModelOwnershipKey(configPath, agentKey string) string {
	return configPath + "#agent:" + agentKey + ".model"
}

func openCodeAgentOwnershipKey(configPath, agentKey string) string {
	return configPath + "#agent:" + agentKey
}

func openCodeManagedSkills(opts Options, home string) (map[string][]byte, error) {
	skillsBase := filepath.Join(openCodeConfigHome(home), "opencode", "skills")
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
