package installer

import (
	"fmt"
	"os"
	"path/filepath"
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
	return managed, nil
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
	paths := map[string]string{"pi/rotta-extension.ts": piExtensionPath(home), "pi/rotta-child-guard.ts": filepath.Join(home, ".pi", "agent", "extensions", "rotta-child-guard.ts"), "core/rotta-core.md": filepath.Join(root, "rotta-core", "SKILL.md")}
	for _, agent := range rottaAgents {
		paths[agent.assetPath] = filepath.Join(root, agent.skillName, "SKILL.md")
	}
	return paths
}
