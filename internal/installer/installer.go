// Package installer handles writing Rotta files to the target tool.
package installer

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Syfra3/Rotta/assets"
)

// Options configures what and where to install.
type Options struct {
	Target          string // "claude-code" | "opencode" | "both"
	ProjectPath     string // project root; config files land here under .rotta/
	InstallSpec     bool
	InstallImpl     bool
	InstallReview   bool
	UseDefaultGates bool
	SetupAncora     bool // whether to install/configure Ancora memory
	SetupVela       bool // whether to install/configure Vela graph intelligence
	SetupContext7   bool // whether to configure Context7 documentation MCP
	CommandStdin    io.Reader
	CommandStdout   io.Writer
	CommandStderr   io.Writer
}

// Result describes what was installed.
type Result struct {
	Target          string
	Files           []string
	Hosts           map[string]HostInstallResult
	BackupDir       string
	Error           string
	AncoraInstalled bool   // true if Ancora binary was installed during this run
	AncoraBin       string // resolved path to the ancora binary
	VelaInstalled   bool   // true if Vela binary was installed during this run
	VelaBin         string // resolved path to the vela binary
	Context7        Context7Result
}

type HostInstallStatus string

const (
	HostInstallStatusInstalled HostInstallStatus = "installed"
	HostInstallStatusFailed    HostInstallStatus = "failed"
)

type HostInstallResult struct {
	Host   string
	Status HostInstallStatus
	Files  []string
}

// Install runs the full installation and returns a summary.
func Install(opts Options) (*Result, error) {
	if !isSupportedInstallTarget(opts.Target) {
		return nil, fmt.Errorf("unsupported host target %q; supported hosts are exactly Claude Code, OpenCode, and Codex", opts.Target)
	}

	result := &Result{Target: opts.Target, Hosts: map[string]HostInstallResult{}}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot resolve home directory: %w", err)
	}

	projectPath := resolveProjectPath(opts.ProjectPath, home)

	backupDir, err := createInstallBackup(opts, home, projectPath)
	if err != nil {
		return nil, fmt.Errorf("backup failure prevented installation: %w", err)
	}
	result.BackupDir = backupDir

	if err := cleanPreviousInstallation(opts, home, projectPath); err != nil {
		return nil, err
	}

	if opts.Target == "all" {
		return installAllHosts(opts, result, home, projectPath)
	}

	if opts.Target == "claude-code" || opts.Target == "both" {
		files, err := installClaudeCode(opts, home)
		if err != nil {
			return nil, err
		}
		result.Files = append(result.Files, files...)
		result.Hosts["claude-code"] = HostInstallResult{Host: "claude-code", Status: HostInstallStatusInstalled, Files: files}
	}

	if opts.Target == "opencode" || opts.Target == "both" {
		files, err := installOpenCode(opts, home)
		if err != nil {
			return nil, err
		}
		result.Files = append(result.Files, files...)
		result.Hosts["opencode"] = HostInstallResult{Host: "opencode", Status: HostInstallStatusInstalled, Files: files}
	}

	if opts.Target == "codex" {
		files, err := installCodex(opts, home)
		if err != nil {
			return nil, err
		}
		result.Files = append(result.Files, files...)
		result.Hosts["codex"] = HostInstallResult{Host: "codex", Status: HostInstallStatusInstalled, Files: files}
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

	if opts.SetupVela {
		vr, err := SetupVela(opts, home, projectPath)
		if err != nil {
			return nil, fmt.Errorf("vela setup: %w", err)
		}
		result.Files = append(result.Files, vr.Files...)
		result.VelaInstalled = vr.Installed
		result.VelaBin = vr.BinPath

		files, err := installVelaFreshnessGuards(opts, home)
		if err != nil {
			return nil, fmt.Errorf("vela freshness guard setup: %w", err)
		}
		result.Files = append(result.Files, files...)
	}

	if opts.SetupContext7 {
		context7Result, err := ConfigureContext7(opts, home)
		if err != nil {
			return nil, fmt.Errorf("context7 setup: %w", err)
		}
		result.Context7 = context7Result
		result.Files = append(result.Files, context7Result.Files...)
		if context7Result.OpenCode.OK || context7Result.ClaudeCode.OK {
			health := CheckContext7Health(Context7ServerConfig())
			result.Context7.Health = health
			result.Context7.HealthRan = true
			if health.OK && context7Result.FullyConfigured {
				result.Context7.Status = Context7StatusConfigured
			} else if !health.OK {
				return result, fmt.Errorf("context7 health: %s", health.Category)
			}
		}
	}

	return result, nil
}

func isSupportedInstallTarget(target string) bool {
	switch target {
	case "", "claude-code", "opencode", "codex", "both", "all":
		return true
	default:
		return false
	}
}

func installAllHosts(opts Options, result *Result, home, projectPath string) (*Result, error) {
	var installErr error
	for _, host := range []string{"claude-code", "opencode", "codex"} {
		files, err := cleanAndInstallHost(opts, host, home)
		if err != nil {
			result.Hosts[host] = HostInstallResult{Host: host, Status: HostInstallStatusFailed}
			installErr = fmt.Errorf("%s host installation: %w", host, err)
			continue
		}
		result.Files = append(result.Files, files...)
		result.Hosts[host] = HostInstallResult{Host: host, Status: HostInstallStatusInstalled, Files: files}
	}

	files, err := installConfig(projectPath)
	if err != nil {
		return result, err
	}
	result.Files = append(result.Files, files...)

	if installErr != nil {
		result.Error = installErr.Error()
		return result, installErr
	}
	return result, nil
}

func cleanAndInstallHost(opts Options, host, home string) ([]string, error) {
	hostOpts := opts
	hostOpts.Target = host
	switch host {
	case "claude-code":
		if err := cleanPreviousClaudeCodeInstallation(home); err != nil {
			return nil, err
		}
		return installClaudeCode(hostOpts, home)
	case "opencode":
		if err := cleanPreviousOpenCodeInstallation(home); err != nil {
			return nil, err
		}
		return installOpenCode(hostOpts, home)
	case "codex":
		if err := cleanPreviousCodexInstallation(home); err != nil {
			return nil, err
		}
		return installCodex(hostOpts, home)
	default:
		return nil, fmt.Errorf("unsupported host target %q", host)
	}
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

// installConfig writes state-machine.yaml and quality-gates.yaml to <project>/.rotta/
func installConfig(projectPath string) ([]string, error) {
	dir := filepath.Join(projectPath, ".rotta")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("cannot create .rotta dir: %w", err)
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

func cleanPreviousInstallation(opts Options, home, projectPath string) error {
	if opts.Target == "all" {
		return cleanSelectedIntegrationArtifacts(opts, home, projectPath)
	}
	if opts.Target == "opencode" || opts.Target == "both" {
		if err := cleanPreviousOpenCodeInstallation(home); err != nil {
			return err
		}
	}
	if opts.Target == "claude-code" || opts.Target == "both" {
		if err := cleanPreviousClaudeCodeInstallation(home); err != nil {
			return err
		}
	}
	if opts.Target == "codex" {
		if err := cleanPreviousCodexInstallation(home); err != nil {
			return err
		}
	}
	if err := cleanSelectedIntegrationArtifacts(opts, home, projectPath); err != nil {
		return err
	}
	return nil
}

func cleanSelectedIntegrationArtifacts(opts Options, home, projectPath string) error {
	if opts.SetupVela {
		paths := []string{filepath.Join(projectPath, ".vela", "graph.db")}
		if opts.Target == "claude-code" || opts.Target == "both" {
			if err := cleanClaudeCodeVelaFreshnessGuard(home); err != nil {
				return err
			}
			paths = append(paths,
				filepath.Join(home, ".claude", "vela-mcp.json"),
				filepath.Join(home, ".claude", "vela-instructions.md"),
			)
		}
		if opts.Target == "opencode" || opts.Target == "both" {
			if err := cleanOpenCodeVelaFreshnessGuard(home); err != nil {
				return err
			}
			paths = append(paths, filepath.Join(home, ".config", "opencode", "instructions.md"))
		}
		for _, path := range paths {
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("cannot remove stale integration artifact %s: %w", path, err)
			}
		}
	}

	if opts.SetupAncora && (opts.Target == "claude-code" || opts.Target == "both") {
		path := filepath.Join(home, ".claude", "mcp", "ancora.json")
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("cannot remove stale integration artifact %s: %w", path, err)
		}
	}

	return nil
}

// copySkillsToDir copies selected SKILL.md files into skillsDir/rotta/<mode>/
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
		dst := filepath.Join(skillsDir, "rotta", m.name)
		if err := os.MkdirAll(dst, 0o755); err != nil {
			return nil, fmt.Errorf("cannot create dir %s: %w", dst, err)
		}
		err := fs.WalkDir(assets.FS, m.src, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil || d.IsDir() {
				return walkErr
			}
			data, err := readRenderedAsset(path, opts)
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
