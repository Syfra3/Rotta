package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	files, err := installManagedFiles(home, managed)
	if err != nil {
		return nil, rollbackPiInstallation(err, manifestPath, manifest, manifestExists, snapshots)
	}
	if piInstallFailureHook != nil {
		if err := piInstallFailureHook("after-assets"); err != nil {
			return nil, rollbackPiInstallation(err, manifestPath, manifest, manifestExists, snapshots)
		}
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
	routing, err := resolvePiRouting(opts.PiModelRouting, opts.PiModelRoutingModels)
	if err != nil {
		return nil, err
	}
	routingConfig, err := json.Marshal(struct {
		Version int               `json:"version"`
		Roles   map[string]string `json:"roles,omitempty"`
	}{Version: 1, Roles: routing.models})
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

var immutablePiRouting = map[string]string{
	"implementation": "openai-codex/gpt-5.6-terra",
	"reviewer":       "openai-codex/gpt-5.6-sol",
	"exploration":    "openai-codex/gpt-5.6-luna",
	"operations":     "openai-codex/gpt-5.6-luna",
}

type resolvedPiRouting struct {
	enabled bool
	models  map[string]string
}

// DefaultPiRouting returns a copy of the four Pi delegated-role assignments.
func DefaultPiRouting() map[string]string {
	models := make(map[string]string, len(immutablePiRouting))
	for role, model := range immutablePiRouting {
		models[role] = model
	}
	return models
}

// IsValidPiModelID accepts Pi's provider/model-id table form without claiming
// that the locally configured provider can actually use it.
func IsValidPiModelID(model string) bool {
	return strings.TrimSpace(model) == model && strings.Count(model, "/") == 1 &&
		!strings.ContainsAny(model, "\t\r\n |\\") && !strings.HasPrefix(model, "/") && !strings.HasSuffix(model, "/")
}

func resolvePiRouting(request ModelRoutingRequest, custom map[string]string) (resolvedPiRouting, error) {
	resolved, err := request.resolved()
	if err != nil {
		return resolvedPiRouting{}, fmt.Errorf("Pi model routing must be enabled, custom, or disabled: %w", err)
	}
	if resolved == ModelRoutingDisabled {
		return resolvedPiRouting{}, nil
	}
	if resolved != ModelRoutingCustom {
		return resolvedPiRouting{enabled: true, models: DefaultPiRouting()}, nil
	}
	if len(custom) != len(immutablePiRouting) {
		return resolvedPiRouting{}, fmt.Errorf("custom Pi model routing must select all four roles")
	}
	models := make(map[string]string, len(custom))
	for role := range immutablePiRouting {
		model, ok := custom[role]
		if !ok || !IsValidPiModelID(model) {
			return resolvedPiRouting{}, fmt.Errorf("custom Pi model routing must select a model for %s", role)
		}
		models[role] = model
	}
	for role := range custom {
		if _, ok := immutablePiRouting[role]; !ok {
			return resolvedPiRouting{}, fmt.Errorf("custom Pi model routing contains unknown role %s", role)
		}
	}
	return resolvedPiRouting{enabled: true, models: models}, nil
}

func rollbackPiInstallation(cause error, manifestPath string, manifest []byte, manifestExists bool, snapshots map[string]routingFileSnapshot) error {
	if err := restoreOpenCodeConfig(manifestPath, manifest, manifestExists); err != nil {
		return fmt.Errorf("Pi installation failed: %w; manifest compensation failed: %v", cause, err)
	}
	for path, snapshot := range snapshots {
		if err := restoreOpenCodeConfig(path, snapshot.data, snapshot.exists); err != nil {
			return fmt.Errorf("Pi installation failed: %w; asset compensation failed: %v", cause, err)
		}
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
