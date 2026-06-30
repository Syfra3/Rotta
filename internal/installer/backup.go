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
	Ancora bool `json:"ancora"`
	Vela   bool `json:"vela"`
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
			Ancora: opts.SetupAncora,
			Vela:   opts.SetupVela,
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
	root := filepath.Join(home, ".clean-workflow", "backups")
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
		filepath.Join(projectPath, ".clean-workflow", "state-machine.yaml"),
		filepath.Join(projectPath, ".clean-workflow", "quality-gates.yaml"),
	}

	paths = append(paths, filepath.Join(projectPath, ".vela", "graph.db"))

	if opts.Target == "opencode" || opts.Target == "both" {
		paths = append(paths,
			filepath.Join(home, ".config", "opencode", "opencode.json"),
			filepath.Join(home, ".config", "opencode", "opencode.jsonc"),
			filepath.Join(home, ".config", "opencode", "instructions.md"),
			filepath.Join(home, ".config", "opencode", "skills", "clean-orchestrator"),
			filepath.Join(home, ".config", "opencode", "skills", "clean-spec"),
			filepath.Join(home, ".config", "opencode", "skills", "clean-impl"),
			filepath.Join(home, ".config", "opencode", "skills", "clean-review"),
		)
	}

	if opts.Target == "claude-code" || opts.Target == "both" {
		paths = append(paths,
			filepath.Join(home, ".claude", "settings.json"),
			filepath.Join(home, ".claude", "skills", "clean-workflow"),
			filepath.Join(home, ".claude", "mcp", "ancora.json"),
			filepath.Join(home, ".claude", "vela-mcp.json"),
			filepath.Join(home, ".claude", "vela-instructions.md"),
		)
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
