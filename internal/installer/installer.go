// Package installer handles writing Uncle Bob workflow files to the target tool.
package installer

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Syfra3/uncle-bob-workflow/assets"
)

// Options configures what and where to install.
type Options struct {
	Target          string // "claude-code" | "opencode" | "both"
	ProjectPath     string // project root; config files land here under .uncle-bob/
	InstallSpec     bool
	InstallImpl     bool
	InstallReview   bool
	UseDefaultGates bool
	SetupAncora     bool // whether to install/configure Ancora memory
}

// Result describes what was installed.
type Result struct {
	Target          string
	Files           []string
	AncoraInstalled bool   // true if Ancora binary was installed during this run
	AncoraBin       string // resolved path to the ancora binary
}

// Install runs the full installation and returns a summary.
func Install(opts Options) (*Result, error) {
	result := &Result{Target: opts.Target}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot resolve home directory: %w", err)
	}

	projectPath := resolveProjectPath(opts.ProjectPath, home)

	if opts.Target == "claude-code" || opts.Target == "both" {
		files, err := installClaudeCode(opts, home)
		if err != nil {
			return nil, err
		}
		result.Files = append(result.Files, files...)
	}

	if opts.Target == "opencode" || opts.Target == "both" {
		files, err := installOpenCode(opts, home)
		if err != nil {
			return nil, err
		}
		result.Files = append(result.Files, files...)
	}

	files, err := installConfig(projectPath)
	if err != nil {
		return nil, err
	}
	result.Files = append(result.Files, files...)

	if opts.SetupAncora {
		ar, err := SetupAncora(opts, home)
		if err != nil {
			return nil, fmt.Errorf("ancora setup: %w", err)
		}
		result.Files = append(result.Files, ar.Files...)
		result.AncoraInstalled = ar.Installed
		result.AncoraBin = ar.BinPath
	}

	return result, nil
}

func resolveProjectPath(path, home string) string {
	if path == "" || path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}

// installConfig writes state-machine.yaml and quality-gates.yaml to <project>/.uncle-bob/
func installConfig(projectPath string) ([]string, error) {
	dir := filepath.Join(projectPath, ".uncle-bob")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("cannot create .uncle-bob dir: %w", err)
	}

	configs := map[string]string{
		"config/state-machine.yaml": filepath.Join(dir, "state-machine.yaml"),
		"config/quality-gates.yaml": filepath.Join(dir, "quality-gates.yaml"),
	}

	var files []string
	for src, dst := range configs {
		data, err := assets.FS.ReadFile(src)
		if err != nil {
			return nil, fmt.Errorf("cannot read embedded %s: %w", src, err)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return nil, fmt.Errorf("cannot write %s: %w", dst, err)
		}
		files = append(files, dst)
	}
	return files, nil
}

// copySkillsToDir copies selected SKILL.md files into skillsDir/uncle-bob/<mode>/
func copySkillsToDir(opts Options, skillsDir string) ([]string, error) {
	type modeEntry struct {
		enabled bool
		src     string // path inside assets.FS
		name    string // subdirectory name
	}
	modes := []modeEntry{
		{opts.InstallSpec, "skills/spec-mode", "spec-mode"},
		{opts.InstallImpl, "skills/implementation-mode", "implementation-mode"},
		{opts.InstallReview, "skills/review-mode", "review-mode"},
	}

	var files []string
	for _, m := range modes {
		if !m.enabled {
			continue
		}
		dst := filepath.Join(skillsDir, "uncle-bob", m.name)
		if err := os.MkdirAll(dst, 0o755); err != nil {
			return nil, fmt.Errorf("cannot create dir %s: %w", dst, err)
		}
		err := fs.WalkDir(assets.FS, m.src, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil || d.IsDir() {
				return walkErr
			}
			data, err := assets.FS.ReadFile(path)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(m.src, path)
			out := filepath.Join(dst, rel)
			if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
				return err
			}
			return os.WriteFile(out, data, 0o644)
		})
		if err != nil {
			return nil, fmt.Errorf("cannot copy %s: %w", m.src, err)
		}
		files = append(files, filepath.Join(dst, "SKILL.md"))
	}
	return files, nil
}
