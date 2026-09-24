package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// piInstallFailureHook is test-only fault injection after managed writes.
var piInstallFailureHook func(stage string) error

// piExtensionPath is global-only. Project-local Pi support is intentionally deferred.
func piExtensionPath(home string) string {
	return filepath.Join(home, ".pi", "agent", "extensions", "rotta.ts")
}

func installPi(opts Options, home string) ([]string, error) {
	managed, err := piManagedFiles(opts, home)
	if err != nil {
		return nil, err
	}
	manifestPath := managedArtifactsManifestPath(home)
	manifest, manifestErr := readPrivateFile(manifestPath)
	manifestExists := manifestErr == nil
	if manifestErr != nil && !os.IsNotExist(manifestErr) {
		return nil, fmt.Errorf("snapshot Pi managed-artifact manifest: %w", manifestErr)
	}
	snapshots, err := snapshotRoutingFiles(managed)
	if err != nil {
		return nil, fmt.Errorf("snapshot Pi bundle: %w", err)
	}
	settingsSnapshot, err := snapshotFile(piSettingsPath(home))
	if err != nil {
		return nil, fmt.Errorf("snapshot Pi settings: %w", err)
	}
	modelsSnapshot, err := snapshotFile(piModelsPath(home))
	if err != nil {
		return nil, fmt.Errorf("snapshot Pi models: %w", err)
	}
	files, err := installManagedFiles(home, managed)
	if err != nil {
		return nil, rollbackPiInstallation(err, manifestPath, manifest, manifestExists, snapshots, piSettingsPath(home), settingsSnapshot, piModelsPath(home), modelsSnapshot)
	}
	if piInstallFailureHook != nil {
		if err := piInstallFailureHook("after-assets"); err != nil {
			return nil, rollbackPiInstallation(err, manifestPath, manifest, manifestExists, snapshots, piSettingsPath(home), settingsSnapshot, piModelsPath(home), modelsSnapshot)
		}
	}
	orchestrator, err := resolvePiOrchestratorRouting(opts.PiModelRouting, opts.PiOrchestratorModel, opts.PiOrchestratorEffort)
	if err != nil {
		return nil, rollbackPiInstallation(err, manifestPath, manifest, manifestExists, snapshots, piSettingsPath(home), settingsSnapshot, piModelsPath(home), modelsSnapshot)
	}
	if orchestrator.enabled {
		if err := writePiOrchestratorSettings(home, orchestrator.assignment); err != nil {
			return nil, rollbackPiInstallation(err, manifestPath, manifest, manifestExists, snapshots, piSettingsPath(home), settingsSnapshot, piModelsPath(home), modelsSnapshot)
		}
		routing, err := resolvePiRouting(opts.PiModelRouting, opts.PiModelRoutingModels, opts.PiModelRoutingEfforts)
		if err != nil {
			return nil, rollbackPiInstallation(err, manifestPath, manifest, manifestExists, snapshots, piSettingsPath(home), settingsSnapshot, piModelsPath(home), modelsSnapshot)
		}
		assignments := []PiRoutingAssignment{orchestrator.assignment}
		for _, assignment := range routing.roles {
			assignments = append(assignments, assignment)
		}
		if err := writePiModelContextWindows(home, assignments); err != nil {
			return nil, rollbackPiInstallation(err, manifestPath, manifest, manifestExists, snapshots, piSettingsPath(home), settingsSnapshot, piModelsPath(home), modelsSnapshot)
		}
		files = append(files, piSettingsPath(home), piModelsPath(home))
	}
	return files, nil
}

func piManagedFiles(opts Options, home string) (map[string][]byte, error) {
	managed := map[string][]byte{}
	for assetPath, outputPath := range piBundlePaths(home) {
		data, err := readRenderedAsset(assetPath, opts)
		if err != nil {
			return nil, fmt.Errorf("read embedded Pi asset %s: %w", assetPath, err)
		}
		managed[outputPath] = data
	}
	config, err := json.Marshal(struct {
		Version  int `json:"version"`
		Services struct {
			Ancora   piMCPService `json:"ancora"`
			Vela     piMCPService `json:"vela"`
			Context7 piMCPService `json:"context7"`
		} `json:"services"`
	}{
		Version: 1,
		Services: struct {
			Ancora   piMCPService `json:"ancora"`
			Vela     piMCPService `json:"vela"`
			Context7 piMCPService `json:"context7"`
		}{
			Ancora: piMCPService{Enabled: opts.SetupAncora}, Vela: piMCPService{Enabled: opts.SetupVela}, Context7: piMCPService{Enabled: opts.SetupContext7},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("serialize Pi MCP configuration: %w", err)
	}
	managed[piMCPConfigPath(home)] = config
	routing, err := resolvePiRouting(opts.PiModelRouting, opts.PiModelRoutingModels, opts.PiModelRoutingEfforts)
	if err != nil {
		return nil, err
	}
	routingConfig, err := json.Marshal(struct {
		Version int                            `json:"version"`
		Roles   map[string]PiRoutingAssignment `json:"roles,omitempty"`
	}{Version: 1, Roles: routing.roles})
	if err != nil {
		return nil, fmt.Errorf("serialize Pi model routing: %w", err)
	}
	managed[piModelRoutingPath(home)] = routingConfig
	return managed, nil
}

type piMCPService struct {
	Enabled bool `json:"enabled"`
}

func piMCPConfigPath(home string) string {
	return filepath.Join(home, ".pi", "agent", "rotta-next", "mcp.json")
}

func piModelRoutingPath(home string) string {
	return filepath.Join(home, ".pi", "agent", "rotta-next", "model-routing.json")
}

func piSettingsPath(home string) string {
	return filepath.Join(home, ".pi", "agent", "settings.json")
}

func piModelsPath(home string) string {
	return filepath.Join(home, ".pi", "agent", "models.json")
}

var immutablePiRouting = map[string]string{
	"implementation": "openai-codex/gpt-6-sol",
	"reviewer":       "openai-codex/gpt-6-sol",
	"exploration":    "openai-codex/gpt-6-luna",
	"operations":     "openai-codex/gpt-6-luna",
}

var immutablePiRoutingEffort = map[string]string{
	"implementation": "low",
	"reviewer":       "low",
	"exploration":    "medium",
	"operations":     "medium",
}

// PiRoutingAssignment is one delegated-role Pi model and thinking selection.
type PiRoutingAssignment struct {
	Model  string `json:"model"`
	Effort string `json:"effort,omitempty"`
}

type resolvedPiRouting struct {
	enabled bool
	roles   map[string]PiRoutingAssignment
}

// DefaultPiRouting returns a copy of the four Pi delegated-role model assignments.
func DefaultPiRouting() map[string]string {
	models := make(map[string]string, len(immutablePiRouting))
	for role, model := range immutablePiRouting {
		models[role] = model
	}
	return models
}

// DefaultPiRoutingEfforts returns a copy of the four Pi delegated-role effort assignments.
func DefaultPiRoutingEfforts() map[string]string {
	efforts := make(map[string]string, len(immutablePiRoutingEffort))
	for role, effort := range immutablePiRoutingEffort {
		efforts[role] = effort
	}
	return efforts
}

func defaultPiRoutingAssignments() map[string]PiRoutingAssignment {
	roles := make(map[string]PiRoutingAssignment, len(immutablePiRouting))
	for role, model := range immutablePiRouting {
		roles[role] = PiRoutingAssignment{Model: model, Effort: immutablePiRoutingEffort[role]}
	}
	return roles
}

func DefaultPiOrchestratorModel() string  { return "openai-codex/gpt-6-sol" }
func DefaultPiOrchestratorEffort() string { return "low" }

type resolvedPiOrchestratorRouting struct {
	enabled    bool
	assignment PiRoutingAssignment
}

func resolvePiOrchestratorRouting(request ModelRoutingRequest, model, effort string) (resolvedPiOrchestratorRouting, error) {
	resolved, err := request.resolved()
	if err != nil {
		return resolvedPiOrchestratorRouting{}, fmt.Errorf("Pi orchestrator model routing must be enabled, custom, or disabled: %w", err)
	}
	if resolved == ModelRoutingDisabled {
		return resolvedPiOrchestratorRouting{}, nil
	}
	if resolved != ModelRoutingCustom {
		return resolvedPiOrchestratorRouting{enabled: true, assignment: PiRoutingAssignment{Model: DefaultPiOrchestratorModel(), Effort: DefaultPiOrchestratorEffort()}}, nil
	}
	// The CLI's four-role custom routing has no separate orchestrator flag.
	// Keep that existing entry point usable with the default parent selection.
	if model == "" && effort == "" {
		return resolvedPiOrchestratorRouting{enabled: true, assignment: PiRoutingAssignment{Model: DefaultPiOrchestratorModel(), Effort: DefaultPiOrchestratorEffort()}}, nil
	}
	if !IsValidPiModelID(model) {
		return resolvedPiOrchestratorRouting{}, fmt.Errorf("custom Pi model routing must select an orchestrator model")
	}
	if !IsValidPiEffort(effort) {
		return resolvedPiOrchestratorRouting{}, fmt.Errorf("custom Pi model routing must select a valid orchestrator effort")
	}
	return resolvedPiOrchestratorRouting{enabled: true, assignment: PiRoutingAssignment{Model: model, Effort: effort}}, nil
}

// IsValidPiModelID accepts Pi's provider/model-id table form without claiming
// that the locally configured provider can actually use it.
func IsValidPiModelID(model string) bool {
	if strings.TrimSpace(model) != model || strings.Count(model, "/") != 1 || strings.HasPrefix(model, "/") || strings.HasSuffix(model, "/") {
		return false
	}
	for _, r := range model {
		if unicode.IsSpace(r) || r == '\uFEFF' || r == '|' || r == '\\' {
			return false
		}
	}
	return true
}

func IsValidPiEffort(effort string) bool {
	switch effort {
	case "off", "minimal", "low", "medium", "high", "xhigh", "max":
		return true
	default:
		return false
	}
}

func resolvePiRouting(request ModelRoutingRequest, custom map[string]string, efforts map[string]string) (resolvedPiRouting, error) {
	resolved, err := request.resolved()
	if err != nil {
		return resolvedPiRouting{}, fmt.Errorf("Pi model routing must be enabled, custom, or disabled: %w", err)
	}
	if resolved == ModelRoutingDisabled {
		return resolvedPiRouting{}, nil
	}
	if resolved != ModelRoutingCustom {
		return resolvedPiRouting{enabled: true, roles: defaultPiRoutingAssignments()}, nil
	}
	if len(custom) != len(immutablePiRouting) {
		return resolvedPiRouting{}, fmt.Errorf("custom Pi model routing must select all four roles")
	}
	roles := make(map[string]PiRoutingAssignment, len(custom))
	for role := range immutablePiRouting {
		model, ok := custom[role]
		if !ok || !IsValidPiModelID(model) {
			return resolvedPiRouting{}, fmt.Errorf("custom Pi model routing must select a model for %s", role)
		}
		effort := immutablePiRoutingEffort[role]
		if efforts != nil {
			selected, ok := efforts[role]
			if !ok || !IsValidPiEffort(selected) {
				return resolvedPiRouting{}, fmt.Errorf("custom Pi model routing must select a valid effort for %s", role)
			}
			effort = selected
		}
		roles[role] = PiRoutingAssignment{Model: model, Effort: effort}
	}
	for role := range custom {
		if _, ok := immutablePiRouting[role]; !ok {
			return resolvedPiRouting{}, fmt.Errorf("custom Pi model routing contains unknown role %s", role)
		}
	}
	for role := range efforts {
		if _, ok := immutablePiRouting[role]; !ok {
			return resolvedPiRouting{}, fmt.Errorf("custom Pi model routing contains unknown role %s", role)
		}
	}
	return resolvedPiRouting{enabled: true, roles: roles}, nil
}

func snapshotFile(path string) (routingFileSnapshot, error) {
	data, err := readPrivateFile(path)
	if err == nil {
		return routingFileSnapshot{data: data, exists: true}, nil
	}
	if os.IsNotExist(err) {
		return routingFileSnapshot{}, nil
	}
	return routingFileSnapshot{}, err
}

func writePiOrchestratorSettings(home string, assignment PiRoutingAssignment) error {
	settingsPath := piSettingsPath(home)
	settings := map[string]json.RawMessage{}
	data, err := readPrivateFile(settingsPath)
	if err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("cannot parse Pi settings: %w", err)
		}
		if settings == nil {
			return fmt.Errorf("cannot parse Pi settings: expected JSON object")
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("cannot read Pi settings: %w", err)
	}
	parts := strings.SplitN(assignment.Model, "/", 2)
	for key, value := range map[string]string{
		"defaultProvider":      parts[0],
		"defaultModel":         parts[1],
		"defaultThinkingLevel": assignment.Effort,
	} {
		encoded, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("cannot marshal Pi settings value: %w", err)
		}
		settings[key] = encoded
	}
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal Pi settings: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o750); err != nil {
		return err
	}
	return writePrivateFile(settingsPath, out, 0o600)
}

func writePiModelContextWindows(home string, assignments []PiRoutingAssignment) error {
	modelsPath := piModelsPath(home)
	models, err := readJSONObject(modelsPath, "Pi models")
	if err != nil {
		return err
	}
	providers, err := rawJSONObject(models["providers"], "Pi models providers")
	if err != nil {
		return err
	}
	provider, err := rawJSONObject(providers["openai-codex"], "Pi openai-codex provider")
	if err != nil {
		return err
	}
	overrides, err := rawJSONObject(provider["modelOverrides"], "Pi openai-codex model overrides")
	if err != nil {
		return err
	}
	for _, assignment := range assignments {
		parts := strings.SplitN(assignment.Model, "/", 2)
		if len(parts) != 2 || parts[0] != "openai-codex" {
			continue
		}
		// Only override models with known supported maximum windows.
		if parts[1] != "gpt-6-sol" && parts[1] != "gpt-6-luna" {
			continue
		}
		model, err := rawJSONObject(overrides[parts[1]], "Pi "+parts[1]+" model override")
		if err != nil {
			return err
		}
		model["contextWindow"] = json.RawMessage("1000000")
		overrides[parts[1]] = mustMarshalRawObject(model)
	}
	provider["modelOverrides"] = mustMarshalRawObject(overrides)
	providers["openai-codex"] = mustMarshalRawObject(provider)
	models["providers"] = mustMarshalRawObject(providers)
	out, err := json.MarshalIndent(models, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal Pi models: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(modelsPath), 0o750); err != nil {
		return err
	}
	return writePrivateFile(modelsPath, out, 0o600)
}

func readJSONObject(path, label string) (map[string]json.RawMessage, error) {
	data, err := readPrivateFile(path)
	if os.IsNotExist(err) {
		return map[string]json.RawMessage{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", label, err)
	}
	return rawJSONObject(data, label)
}

func rawJSONObject(data json.RawMessage, label string) (map[string]json.RawMessage, error) {
	if len(data) == 0 {
		return map[string]json.RawMessage{}, nil
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, fmt.Errorf("cannot parse %s: %w", label, err)
	}
	if object == nil {
		return nil, fmt.Errorf("cannot parse %s: expected JSON object", label)
	}
	return object, nil
}

func mustMarshalRawObject(object map[string]json.RawMessage) json.RawMessage {
	data, err := json.Marshal(object)
	if err != nil {
		panic(err) // Values were already validated by json.Unmarshal or created above.
	}
	return data
}

func rollbackPiInstallation(cause error, manifestPath string, manifest []byte, manifestExists bool, snapshots map[string]routingFileSnapshot, settingsPath string, settingsSnapshot routingFileSnapshot, modelsPath string, modelsSnapshot routingFileSnapshot) error {
	if err := restoreOpenCodeConfig(manifestPath, manifest, manifestExists); err != nil {
		return fmt.Errorf("Pi installation failed: %w; manifest compensation failed: %v", cause, err)
	}
	for path, snapshot := range snapshots {
		if err := restoreOpenCodeConfig(path, snapshot.data, snapshot.exists); err != nil {
			return fmt.Errorf("Pi installation failed: %w; asset compensation failed: %v", cause, err)
		}
	}
	if err := restoreOpenCodeConfig(settingsPath, settingsSnapshot.data, settingsSnapshot.exists); err != nil {
		return fmt.Errorf("Pi installation failed: %w; settings compensation failed: %v", cause, err)
	}
	if err := restoreOpenCodeConfig(modelsPath, modelsSnapshot.data, modelsSnapshot.exists); err != nil {
		return fmt.Errorf("Pi installation failed: %w; models compensation failed: %v", cause, err)
	}
	return cause
}

func piBundlePaths(home string) map[string]string {
	root := filepath.Join(home, ".pi", "agent", "rotta-next")
	paths := map[string]string{"pi/rotta-extension.ts": piExtensionPath(home), "pi/rotta-child-guard.ts": filepath.Join(home, ".pi", "agent", "extensions", "rotta-child-guard.ts"), "pi/rotta-mcp-bridge.ts": filepath.Join(root, "rotta-mcp-bridge.ts"), "core/rotta-core.md": filepath.Join(root, "rotta-core", "SKILL.md")}
	for _, agent := range rottaAgents {
		paths[agent.assetPath] = filepath.Join(root, agent.skillName, "SKILL.md")
	}
	return paths
}
