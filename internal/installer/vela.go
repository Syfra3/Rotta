package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// VelaResult describes what Vela setup did.
type VelaResult struct {
	BinPath   string
	Installed bool // true if we ran brew install
	Files     []string
}

// SetupVela detects or installs Vela, initializes the project graph, and sets
// up agent MCP integration when Vela is the standalone graph surface.
func SetupVela(opts Options, home, projectPath string) (*VelaResult, error) {
	result := &VelaResult{}

	binPath, err := detectVelaBin()
	if err != nil {
		if installErr := installVela(opts); installErr != nil {
			return nil, fmt.Errorf(
				"vela not found and installation failed: %w\n\n"+
					"Install manually:\n"+
					"  brew tap Syfra3/tap && brew install vela\n"+
					"  # or build from source: https://github.com/Syfra3/vela\n\n"+
					"Then rerun rotta setup or run: vela install --project <project>",
				installErr,
			)
		}
		result.Installed = true
		binPath, _ = detectVelaBin()
		if binPath == "" {
			binPath = "/opt/homebrew/bin/vela"
		}
	}
	result.BinPath = binPath

	if opts.SetupAncora {
		if err := runVelaInstall(opts, binPath, projectPath, "", ""); err != nil {
			return nil, fmt.Errorf("initialize vela project graph: %w", err)
		}
		result.addFile(filepath.Join(projectPath, ".vela", "graph.db"))
		return result, nil
	}

	if opts.Target == "claude-code" || opts.Target == "both" {
		claudeDir := filepath.Join(home, ".claude")
		if err := runVelaInstall(opts, binPath, projectPath, "claude", claudeDir); err != nil {
			return nil, fmt.Errorf("vela install claude: %w", err)
		}
		result.addFiles(
			filepath.Join(projectPath, ".vela", "graph.db"),
			filepath.Join(claudeDir, "vela-mcp.json"),
			filepath.Join(claudeDir, "vela-instructions.md"),
		)
	}

	if opts.Target == "opencode" || opts.Target == "both" {
		opencodeDir := filepath.Join(home, ".config", "opencode")
		if err := runVelaInstall(opts, binPath, projectPath, "opencode", opencodeDir); err != nil {
			return nil, fmt.Errorf("vela install opencode: %w", err)
		}
		result.addFiles(
			filepath.Join(projectPath, ".vela", "graph.db"),
			filepath.Join(opencodeDir, "opencode.json"),
			filepath.Join(opencodeDir, "instructions.md"),
		)
	}

	return result, nil
}

// detectVelaBin finds the vela binary via PATH or common install locations.
func detectVelaBin() (string, error) {
	if path, err := exec.LookPath("vela"); err == nil {
		return path, nil
	}
	for _, candidate := range velaBinCandidates() {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("vela binary not found")
}

func velaBinCandidates() []string {
	return []string{
		"/opt/homebrew/bin/vela",
		"/home/linuxbrew/.linuxbrew/bin/vela",
		"/usr/local/bin/vela",
		fmt.Sprintf("%s/.local/bin/vela", os.Getenv("HOME")),
	}
}

func (r *VelaResult) addFiles(paths ...string) {
	for _, path := range paths {
		r.addFile(path)
	}
}

func (r *VelaResult) addFile(path string) {
	for _, existing := range r.Files {
		if existing == path {
			return
		}
	}
	r.Files = append(r.Files, path)
}

// installVela installs Vela through the Syfra Homebrew tap. There is no known
// official curl installer, so we do not run one implicitly.
func installVela(opts Options) error {
	brew, err := exec.LookPath("brew")
	if err != nil {
		return fmt.Errorf("brew not found")
	}
	if err := runCommand(opts, brew, "tap", "Syfra3/tap"); err != nil {
		return fmt.Errorf("brew tap Syfra3/tap: %w", err)
	}
	if err := runCommand(opts, brew, "trust", "Syfra3/tap"); err != nil {
		return fmt.Errorf("brew trust Syfra3/tap: %w", err)
	}
	return runCommand(opts, brew, "install", "vela")
}

func runVelaInstall(opts Options, binPath, projectPath, agent, configDir string) error {
	args := []string{"install", "--project", projectPath}
	if agent != "" {
		args = append(args, "--agent", agent)
		switch agent {
		case "claude":
			args = append(args, "--claude-dir", configDir)
		case "opencode":
			args = append(args, "--opencode-dir", configDir)
		}
	}
	return runCommand(opts, binPath, args...)
}

func runCommand(opts Options, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	configureCommandIO(cmd, opts)
	return cmd.Run()
}
