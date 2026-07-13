package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	velaHomebrewTap = "Syfra3/tap"
	velaFormula     = "vela"
)

// VelaResult describes what Vela setup did.
type VelaResult struct {
	BinPath                    string
	Installed                  bool // true if we ran brew install
	Files                      []string
	NormalizedMCPEntries       []string
	SkippedAmbiguousMCPEntries []string
	MCPAvailability            map[string]map[string]MCPStatusResult
}

// SetupVela detects or installs Vela, initializes the project graph, and sets
// up agent MCP integration when Vela is the standalone graph surface.
func SetupVela(opts Options, home, projectPath string) (*VelaResult, error) {
	if _, err := exec.LookPath("vela"); err != nil {
		result := unavailableVelaMCPResult(opts, home)
		if len(result.MCPAvailability) != 0 {
			return result, nil
		}
	}
	binPath, installed, err := resolveVelaBin(opts)
	if err != nil {
		result := unavailableVelaMCPResult(opts, home)
		if len(result.MCPAvailability) != 0 {
			return result, nil
		}
		return nil, err
	}
	result := &VelaResult{BinPath: binPath, Installed: installed}
	if err := runVelaClusteringInstallIfConfigured(opts, binPath, projectPath); err != nil {
		return nil, err
	}
	return configureVelaHosts(opts, result, home, projectPath)
}

func unavailableVelaMCPResult(opts Options, home string) *VelaResult {
	result := &VelaResult{MCPAvailability: map[string]map[string]MCPStatusResult{}}
	for _, host := range selectedHosts(opts.Target) {
		agent, configDir := velaHostConfig(host, home)
		if agent == "" {
			continue
		}
		path := velaMCPConfigPath(agent, configDir)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		normalized, ambiguous, err := serializeVelaMCPCommand(agent, configDir)
		if err != nil {
			continue
		}
		if normalized {
			result.NormalizedMCPEntries = append(result.NormalizedMCPEntries, path)
		}
		if ambiguous {
			result.SkippedAmbiguousMCPEntries = append(result.SkippedAmbiguousMCPEntries, path)
		}
		result.MCPAvailability[host] = map[string]MCPStatusResult{"vela": {
			Status: MCPStatusDegraded, Reason: "command availability",
			Remediation:     "Install Vela or add the vela command to Rotta's PATH, then rerun Rotta.",
			RuntimeFallback: MCPRuntimeFallback{State: MCPRuntimeFallbackNotObserved},
		}}
	}
	return result
}

func velaHostConfig(host, home string) (agent, configDir string) {
	switch host {
	case "claude-code":
		return "claude", filepath.Join(home, ".claude")
	case "opencode":
		return "opencode", filepath.Join(home, ".config", "opencode")
	}
	return "", ""
}

func resolveVelaBin(opts Options) (string, bool, error) {
	binPath, err := detectVelaBin()
	if err == nil {
		return refreshedVelaBin(opts, binPath)
	}
	if err := installVela(opts); err != nil {
		return "", false, fmt.Errorf("vela not found and installation failed: %w\n\nInstall manually:\n  brew tap Syfra3/tap && brew install vela\n  # or build from source: https://github.com/Syfra3/vela\n\nThen rerun rotta setup or run: vela install --project <project>", err)
	}
	binPath, _ = detectVelaBin()
	if binPath == "" {
		binPath = "/opt/homebrew/bin/vela"
	}
	return binPath, true, nil
}

func refreshedVelaBin(opts Options, binPath string) (string, bool, error) {
	if err := upgradeVela(opts, binPath); err != nil {
		return "", false, fmt.Errorf("refresh homebrew vela: %w", err)
	}
	if upgraded, err := detectVelaBin(); err == nil && upgraded != "" {
		binPath = upgraded
	}
	return binPath, false, nil
}

func configureVelaHosts(opts Options, result *VelaResult, home, projectPath string) (*VelaResult, error) {
	if opts.SetupAncora {
		return configureVelaWithAncora(opts, result, home, projectPath)
	}
	if includesClaude(opts.Target) {
		if err := installVelaForHost(opts, result, projectPath, "claude", filepath.Join(home, ".claude")); err != nil {
			return nil, err
		}
	}
	if includesOpenCode(opts.Target) {
		if err := installVelaForHost(opts, result, projectPath, "opencode", filepath.Join(home, ".config", "opencode")); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func configureVelaWithAncora(opts Options, result *VelaResult, home, projectPath string) (*VelaResult, error) {
	if opts.Target == "all" {
		if err := installVelaForHost(opts, result, projectPath, "claude", filepath.Join(home, ".claude")); err != nil {
			return nil, err
		}
	}
	if includesOpenCode(opts.Target) {
		return result, installVelaForHost(opts, result, projectPath, "opencode", filepath.Join(home, ".config", "opencode"))
	}
	if err := runVelaInstall(opts, result.BinPath, projectPath, "", ""); err != nil {
		return nil, fmt.Errorf("initialize vela project graph: %w", err)
	}
	result.addFile(filepath.Join(projectPath, ".vela", "graph.db"))
	return result, nil
}

func includesClaude(target string) bool {
	return target == "claude-code" || target == "both" || target == "all"
}
func includesOpenCode(target string) bool {
	return target == "opencode" || target == "both" || target == "all"
}

func installVelaForHost(opts Options, result *VelaResult, projectPath, agent, configDir string) error {
	if err := runVelaInstall(opts, result.BinPath, projectPath, agent, configDir); err != nil {
		return fmt.Errorf("vela install %s: %w", agent, err)
	}
	normalized, ambiguous, err := serializeVelaMCPCommand(agent, configDir)
	if err != nil {
		return fmt.Errorf("serialize Vela MCP command for %s: %w", agent, err)
	}
	if normalized {
		result.NormalizedMCPEntries = append(result.NormalizedMCPEntries, velaMCPConfigPath(agent, configDir))
	}
	if ambiguous {
		result.SkippedAmbiguousMCPEntries = append(result.SkippedAmbiguousMCPEntries, velaMCPConfigPath(agent, configDir))
	}
	result.addFile(filepath.Join(projectPath, ".vela", "graph.db"))
	if agent == "claude" {
		result.addFiles(filepath.Join(configDir, "vela-mcp.json"), filepath.Join(configDir, "vela-instructions.md"))
		return nil
	}
	result.addFiles(filepath.Join(configDir, "opencode.json"), filepath.Join(configDir, "instructions.md"))
	return nil
}

func serializeVelaMCPCommand(agent, configDir string) (bool, bool, error) {
	switch agent {
	case "claude":
		return normalizeProvenManagedMCPCommand(velaMCPConfigPath(agent, configDir), "", "vela")
	case "opencode":
		return normalizeProvenManagedMCPCommand(velaMCPConfigPath(agent, configDir), "vela", "vela")
	default:
		return false, false, fmt.Errorf("unsupported Vela setup target %q", agent)
	}
}

func velaMCPConfigPath(agent, configDir string) string {
	if agent == "claude" {
		return filepath.Join(configDir, "vela-mcp.json")
	}
	return filepath.Join(configDir, "opencode.json")
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
	if err := prepareVelaHomebrewFormula(opts, brew); err != nil {
		return err
	}
	return runCommand(opts, brew, "install", velaFormula)
}

func upgradeVela(opts Options, binPath string) error {
	brew, err := exec.LookPath("brew")
	if err != nil {
		return nil
	}
	if filepath.Dir(brew) != filepath.Dir(binPath) {
		return nil
	}
	if err := prepareVelaHomebrewFormula(opts, brew); err != nil {
		return err
	}
	return runCommand(opts, brew, "upgrade", velaFormula)
}

func prepareVelaHomebrewFormula(opts Options, brew string) error {
	if err := runCommand(opts, brew, "tap", velaHomebrewTap); err != nil {
		return fmt.Errorf("brew tap %s: %w", velaHomebrewTap, err)
	}
	if err := runCommand(opts, brew, "trust", velaHomebrewTap); err != nil {
		return fmt.Errorf("brew trust %s: %w", velaHomebrewTap, err)
	}
	if err := runCommand(opts, brew, "update"); err != nil {
		return fmt.Errorf("brew update: %w", err)
	}
	return nil
}

func runVelaClusteringInstallIfConfigured(opts Options, binPath, projectPath string) error {
	if _, err := os.Stat(filepath.Join(projectPath, "requirements-clustering.txt")); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect vela clustering requirements: %w", err)
	}
	return runCommand(opts, binPath, "install", "--project", projectPath, "--clustering", "--repair-venv")
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
	var cmd *exec.Cmd
	switch filepath.Base(name) {
	case "brew":
		cmd = exec.Command("brew")
	case "vela":
		cmd = exec.Command("vela")
	default:
		return fmt.Errorf("unsupported executable %q", name)
	}
	cmd.Args = append(cmd.Args, args...)
	configureCommandIO(cmd, opts)
	return cmd.Run()
}
