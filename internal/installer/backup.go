package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const backupManifestVersion = 1

type backupManifest struct {
	Version              int                  `json:"version"`
	Timestamp            string               `json:"timestamp"`
	ProjectPath          string               `json:"project_path"`
	Target               string               `json:"target"`
	SelectedModes        selectedModes        `json:"selected_modes"`
	OptionalIntegrations optionalIntegrations `json:"optional_integrations"`
	BackedUpPaths        []string             `json:"backed_up_paths"`
	MissingPaths         []string             `json:"missing_paths"`
	Status               string               `json:"status"`
}

type selectedModes struct {
	Spec           bool `json:"spec"`
	Implementation bool `json:"implementation"`
	Review         bool `json:"review"`
}

type optionalIntegrations struct {
	Ancora   bool `json:"ancora"`
	Vela     bool `json:"vela"`
	Context7 bool `json:"context7"`
}

type RestoreResult struct {
	PreRestoreBackupDir string
}

func Backup(opts Options) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot resolve home directory: %w", err)
	}
	projectPath := resolveProjectPath(opts.ProjectPath, home)
	return createInstallBackup(opts, home, projectPath)
}

type restoreHooks struct {
	afterRestorePath func(string) error
}

func RestoreBackup(backupDir string) (*RestoreResult, error) {
	return restoreBackupWithHooks(backupDir, restoreHooks{})
}

func restoreBackupWithHooks(backupDir string, hooks restoreHooks) (*RestoreResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot resolve home directory: %w", err)
	}

	manifest, err := loadBackupManifest(filepath.Join(backupDir, "manifest.json"))
	if err != nil {
		return nil, err
	}
	if manifest.Status != "complete" {
		return nil, fmt.Errorf("cannot restore incomplete backup")
	}

	opts := optionsFromManifest(manifest)
	preRestoreBackupDir, err := createInstallBackup(opts, home, manifest.ProjectPath)
	if err != nil {
		return nil, fmt.Errorf("pre-restore safety backup: %w", err)
	}
	result := &RestoreResult{PreRestoreBackupDir: preRestoreBackupDir}

	if err := applyBackupContents(backupDir, home, manifest, hooks); err != nil {
		rollbackErr := restorePreRestoreBackup(preRestoreBackupDir, home)
		if rollbackErr != nil {
			return result, fmt.Errorf("restore failed for selected backup %s and rollback to pre-restore safety backup %s failed: %w", backupDir, preRestoreBackupDir, rollbackErr)
		}
		return result, fmt.Errorf("restore failed for selected backup %s: %w; rollback to pre-restore state succeeded", backupDir, err)
	}

	return result, nil
}

func applyBackupContents(backupDir, home string, manifest backupManifest, hooks restoreHooks) error {
	for _, path := range manifest.BackedUpPaths {
		if err := restoreBackedUpPath(backupDir, home, path); err != nil {
			return err
		}
		if hooks.afterRestorePath != nil {
			if err := hooks.afterRestorePath(path); err != nil {
				return fmt.Errorf("restore failed after changing %s: %w", path, err)
			}
		}
	}
	for _, path := range manifest.MissingPaths {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("cannot remove path absent in backup %s: %w", path, err)
		}
	}
	return nil
}

func restorePreRestoreBackup(preRestoreBackupDir, home string) error {
	manifest, err := loadBackupManifest(filepath.Join(preRestoreBackupDir, "manifest.json"))
	if err != nil {
		return err
	}
	return applyBackupContents(preRestoreBackupDir, home, manifest, restoreHooks{})
}

func loadBackupManifest(path string) (backupManifest, error) {
	data, err := readPrivateFile(path)
	if err != nil {
		return backupManifest{}, fmt.Errorf("cannot read backup manifest: %w", err)
	}
	var manifest backupManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return backupManifest{}, fmt.Errorf("cannot parse backup manifest: %w", err)
	}
	return manifest, nil
}

func optionsFromManifest(manifest backupManifest) Options {
	return Options{
		Target:        manifest.Target,
		ProjectPath:   manifest.ProjectPath,
		InstallSpec:   manifest.SelectedModes.Spec,
		InstallImpl:   manifest.SelectedModes.Implementation,
		InstallReview: manifest.SelectedModes.Review,
		SetupAncora:   manifest.OptionalIntegrations.Ancora,
		SetupVela:     manifest.OptionalIntegrations.Vela,
		SetupContext7: manifest.OptionalIntegrations.Context7,
	}
}

func restoreBackedUpPath(backupDir, home, path string) error {
	src := backupDestination(backupDir, home, path)
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("cannot read backed-up path %s: %w", path, err)
	}
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("cannot prepare restore destination %s: %w", path, err)
	}
	if info.IsDir() {
		if err := copyDir(src, path); err != nil {
			return fmt.Errorf("cannot restore directory %s: %w", path, err)
		}
		return nil
	}
	if err := copyFile(src, path, info.Mode()); err != nil {
		return fmt.Errorf("cannot restore file %s: %w", path, err)
	}
	return nil
}

func createInstallBackup(opts Options, home, projectPath string) (string, error) {
	return createBackup(opts, home, projectPath, backupScope(opts, home, projectPath))
}

func createAgentBackup(opts Options, host, home string) (string, error) {
	agentOpts := opts
	agentOpts.Target = host
	return createBackup(agentOpts, home, resolveProjectPath(opts.ProjectPath, home), targetBackupPaths(host, home))
}

func createAgentBackups(opts Options, home, transactionBackupDir string) (map[string]string, error) {
	backups := make(map[string]string, len(selectedHosts(opts.Target)))
	for _, host := range selectedHosts(opts.Target) {
		backupDir, err := createAgentBackupAt(opts, host, home, filepath.Join(transactionBackupDir, "agents", host))
		if err != nil {
			return nil, fmt.Errorf("backup %s configuration: %w", host, err)
		}
		backups[host] = backupDir
	}
	return backups, nil
}

func createAgentBackupAt(opts Options, host, home, backupDir string) (string, error) {
	agentOpts := opts
	agentOpts.Target = host
	if err := os.MkdirAll(backupDir, 0o750); err != nil {
		return "", err
	}
	manifest := newBackupManifest(agentOpts, time.Now().UTC().Format("20060102T150405Z"), resolveProjectPath(opts.ProjectPath, home))
	if err := os.MkdirAll(filepath.Join(backupDir, "files"), 0o750); err != nil {
		return "", err
	}
	if err := backupInstallPaths(&manifest, backupDir, home, targetBackupPaths(host, home)); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", err
	}
	if err := writePrivateFile(filepath.Join(backupDir, "manifest.json"), data, 0o600); err != nil {
		return "", err
	}
	return backupDir, nil
}

func createBackup(opts Options, home, projectPath string, paths []string) (string, error) {
	backupDir, timestamp, err := nextBackupDir(home)
	if err != nil {
		return "", err
	}
	manifest := newBackupManifest(opts, timestamp, projectPath)
	if err := os.MkdirAll(filepath.Join(backupDir, "files"), 0o750); err != nil {
		return "", err
	}
	if err := backupInstallPaths(&manifest, backupDir, home, paths); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", err
	}
	if err := writePrivateFile(filepath.Join(backupDir, "manifest.json"), data, 0o600); err != nil {
		return "", err
	}
	return backupDir, nil
}

func newBackupManifest(opts Options, timestamp, projectPath string) backupManifest {
	return backupManifest{
		Version:     backupManifestVersion,
		Timestamp:   timestamp,
		ProjectPath: projectPath,
		Target:      opts.Target,
		SelectedModes: selectedModes{
			Spec:           opts.InstallSpec,
			Implementation: opts.InstallImpl,
			Review:         opts.InstallReview,
		},
		OptionalIntegrations: optionalIntegrations{
			Ancora:   opts.SetupAncora,
			Vela:     opts.SetupVela,
			Context7: opts.SetupContext7,
		},
		Status: "complete",
	}
}

func backupInstallPaths(manifest *backupManifest, backupDir, home string, paths []string) error {
	for _, path := range paths {
		info, statErr := os.Stat(path)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				manifest.MissingPaths = append(manifest.MissingPaths, path)
				continue
			}
			return statErr
		}
		dst := backupDestination(backupDir, home, path)
		if info.IsDir() {
			if err := copyDir(path, dst); err != nil {
				return err
			}
		} else if err := copyFile(path, dst, info.Mode()); err != nil {
			return err
		}
		manifest.BackedUpPaths = append(manifest.BackedUpPaths, path)
	}
	return nil
}

func nextBackupDir(home string) (string, string, error) {
	timestamp := time.Now().UTC().Format("20060102T150405Z")
	root := filepath.Join(home, ".rotta", "backups")
	if err := os.MkdirAll(root, 0o750); err != nil {
		return "", "", err
	}
	for suffix := 0; suffix < 1000; suffix++ {
		name := timestamp
		if suffix > 0 {
			name = fmt.Sprintf("%s-%03d", timestamp, suffix)
		}
		path := filepath.Join(root, name)
		if err := os.Mkdir(path, 0o750); err == nil {
			return path, name, nil
		} else if !os.IsExist(err) {
			return "", "", err
		}
	}
	return "", "", fmt.Errorf("cannot allocate backup directory")
}

func backupScope(opts Options, home, projectPath string) []string {
	paths := append([]string{
		filepath.Join(projectPath, ".rotta", "state-machine.yaml"),
		filepath.Join(projectPath, ".rotta", "quality-gates.yaml"),
	}, filepath.Join(projectPath, ".vela", "graph.db"))
	paths = append(paths, targetBackupPaths(opts.Target, home)...)
	if opts.SetupContext7 {
		paths = appendUniquePaths(paths, filepath.Join(home, ".config", "opencode", "opencode.json"), filepath.Join(home, ".claude", "mcp", "context7.json"))
	}
	return paths
}

func targetBackupPaths(target, home string) []string {
	var paths []string
	if target == "opencode" || target == "both" || target == "all" {
		paths = append(paths, openCodeBackupPaths(home)...)
	}
	if target == "claude-code" || target == "both" || target == "all" {
		paths = append(paths, claudeCodeBackupPaths(home)...)
	}
	if target == "codex" || target == "all" {
		paths = append(paths, filepath.Join(home, ".codex", "AGENTS.md"), filepath.Join(home, ".codex", "config.toml"))
	}
	return paths
}

func openCodeBackupPaths(home string) []string {
	root := filepath.Join(home, ".config", "opencode")
	paths := []string{filepath.Join(root, "opencode.json"), filepath.Join(root, "opencode.jsonc"), filepath.Join(root, "instructions.md"), filepath.Join(root, "plugin", "rotta-vela-freshness-guard.js")}
	for _, skill := range append(append([]string{}, []string{"rotta-orchestrator", "rotta-spec", "rotta-impl", "rotta-review"}...), append(legacyCleanOpenCodeAgentKeys, legacyBobOpenCodeAgentKeys...)...) {
		paths = append(paths, filepath.Join(root, "skills", skill))
	}
	return paths
}
func claudeCodeBackupPaths(home string) []string {
	root := filepath.Join(home, ".claude")
	return []string{filepath.Join(root, "settings.json"), filepath.Join(root, "hooks", "rotta-vela-freshness-guard.sh"), filepath.Join(root, "skills", "rotta"), filepath.Join(root, "skills", "clean-workflow"), filepath.Join(root, "mcp", "ancora.json"), filepath.Join(root, "vela-mcp.json"), filepath.Join(root, "vela-instructions.md")}
}

func appendUniquePaths(paths []string, candidates ...string) []string {
	existing := map[string]bool{}
	for _, path := range paths {
		existing[path] = true
	}
	for _, path := range candidates {
		if !existing[path] {
			paths = append(paths, path)
			existing[path] = true
		}
	}
	return paths
}

func backupDestination(backupDir, home, src string) string {
	if rel, err := filepath.Rel(home, src); err == nil && rel != ".." && !strings.HasPrefix(rel, "../") {
		return filepath.Join(backupDir, "files", "home", rel)
	}
	return filepath.Join(backupDir, "files", "absolute", strings.TrimPrefix(filepath.Clean(src), string(filepath.Separator)))
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		out := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(out, info.Mode())
		}
		return copyFile(path, out, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return err
	}
	data, err := readPrivateFile(src)
	if err != nil {
		return err
	}
	return writePrivateFile(dst, data, mode)
}
