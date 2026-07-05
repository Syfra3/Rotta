package installer

import (
	"encoding/json"
	"fmt"
	"io"
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
	data, err := os.ReadFile(path)
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
	backupDir, timestamp, err := nextBackupDir(home)
	if err != nil {
		return "", err
	}

	manifest := backupManifest{
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

	if err := os.MkdirAll(filepath.Join(backupDir, "files"), 0o755); err != nil {
		return "", err
	}

	for _, path := range backupScope(opts, home, projectPath) {
		info, statErr := os.Stat(path)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				manifest.MissingPaths = append(manifest.MissingPaths, path)
				continue
			}
			return "", statErr
		}

		dst := backupDestination(backupDir, home, path)
		if info.IsDir() {
			if err := copyDir(path, dst); err != nil {
				return "", err
			}
		} else if err := copyFile(path, dst, info.Mode()); err != nil {
			return "", err
		}
		manifest.BackedUpPaths = append(manifest.BackedUpPaths, path)
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(backupDir, "manifest.json"), data, 0o644); err != nil {
		return "", err
	}

	return backupDir, nil
}

func nextBackupDir(home string) (string, string, error) {
	timestamp := time.Now().UTC().Format("20060102T150405Z")
	root := filepath.Join(home, ".rotta", "backups")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", "", err
	}
	for suffix := 0; suffix < 1000; suffix++ {
		name := timestamp
		if suffix > 0 {
			name = fmt.Sprintf("%s-%03d", timestamp, suffix)
		}
		path := filepath.Join(root, name)
		if err := os.Mkdir(path, 0o755); err == nil {
			return path, name, nil
		} else if !os.IsExist(err) {
			return "", "", err
		}
	}
	return "", "", fmt.Errorf("cannot allocate backup directory")
}

func backupScope(opts Options, home, projectPath string) []string {
	paths := []string{
		filepath.Join(projectPath, ".rotta", "state-machine.yaml"),
		filepath.Join(projectPath, ".rotta", "quality-gates.yaml"),
	}

	paths = append(paths, filepath.Join(projectPath, ".vela", "graph.db"))

	if opts.Target == "opencode" || opts.Target == "both" || opts.Target == "all" {
		paths = append(paths,
			filepath.Join(home, ".config", "opencode", "opencode.json"),
			filepath.Join(home, ".config", "opencode", "opencode.jsonc"),
			filepath.Join(home, ".config", "opencode", "instructions.md"),
			filepath.Join(home, ".config", "opencode", "plugin", "rotta-vela-freshness-guard.js"),
			filepath.Join(home, ".config", "opencode", "skills", "rotta-orchestrator"),
			filepath.Join(home, ".config", "opencode", "skills", "rotta-spec"),
			filepath.Join(home, ".config", "opencode", "skills", "rotta-impl"),
			filepath.Join(home, ".config", "opencode", "skills", "rotta-review"),
			filepath.Join(home, ".config", "opencode", "skills", "clean-orchestrator"),
			filepath.Join(home, ".config", "opencode", "skills", "clean-spec"),
			filepath.Join(home, ".config", "opencode", "skills", "clean-impl"),
			filepath.Join(home, ".config", "opencode", "skills", "clean-review"),
			filepath.Join(home, ".config", "opencode", "skills", "bob-orchestrator"),
			filepath.Join(home, ".config", "opencode", "skills", "bob-spec"),
			filepath.Join(home, ".config", "opencode", "skills", "bob-impl"),
			filepath.Join(home, ".config", "opencode", "skills", "bob-review"),
		)
	}

	if opts.Target == "claude-code" || opts.Target == "both" || opts.Target == "all" {
		paths = append(paths,
			filepath.Join(home, ".claude", "settings.json"),
			filepath.Join(home, ".claude", "hooks", "rotta-vela-freshness-guard.sh"),
			filepath.Join(home, ".claude", "skills", "rotta"),
			filepath.Join(home, ".claude", "skills", "clean-workflow"),
			filepath.Join(home, ".claude", "mcp", "ancora.json"),
			filepath.Join(home, ".claude", "vela-mcp.json"),
			filepath.Join(home, ".claude", "vela-instructions.md"),
		)
	}

	if opts.Target == "codex" || opts.Target == "all" {
		paths = append(paths,
			filepath.Join(home, ".codex", "AGENTS.md"),
			filepath.Join(home, ".codex", "config.toml"),
		)
	}

	if opts.SetupContext7 {
		paths = appendUniquePaths(paths,
			filepath.Join(home, ".config", "opencode", "opencode.json"),
			filepath.Join(home, ".claude", "mcp", "context7.json"),
		)
	}

	return paths
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
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
