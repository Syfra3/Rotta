package installer

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

// AncoraResult describes what Ancora setup did.
type AncoraResult struct {
	BinPath   string
	Installed bool // true if we ran brew/curl install
	Files     []string
}

// SetupAncora detects or installs the Ancora binary, then delegates to
// `ancora setup <agent>` which handles all MCP config, plugin files,
// and permissions automatically.
//
// Installation source: https://github.com/Syfra3/ancora
func SetupAncora(opts Options, home string) (*AncoraResult, error) {
	result := &AncoraResult{}

	binPath, err := detectAncoraBin()
	if err != nil {
		if installErr := installAncora(); installErr != nil {
			return nil, fmt.Errorf(
				"ancora not found and installation failed: %w\n\n"+
					"Install manually:\n"+
					"  brew tap Syfra3/tap && brew install ancora\n"+
					"  # or\n"+
					"  curl -sSL https://raw.githubusercontent.com/Syfra3/ancora/main/scripts/install-ancora.sh | bash\n\n"+
					"Then run: ancora setup claude-code  # or opencode",
				installErr,
			)
		}
		result.Installed = true
		binPath, _ = detectAncoraBin()
		if binPath == "" {
			binPath = "/opt/homebrew/bin/ancora"
		}
	}
	result.BinPath = binPath

	// Delegate all configuration to `ancora setup` — it handles MCP config,
	// plugin files, permissions, and hooks for each target.
	if opts.Target == "claude-code" || opts.Target == "both" {
		if err := runAncoraSetup(binPath, "claude-code"); err != nil {
			return nil, fmt.Errorf("ancora setup claude-code: %w", err)
		}
	}

	if opts.Target == "opencode" || opts.Target == "both" {
		if err := runAncoraSetup(binPath, "opencode"); err != nil {
			return nil, fmt.Errorf("ancora setup opencode: %w", err)
		}
	}

	return result, nil
}

// detectAncoraBin finds the ancora binary via PATH or common install locations.
func detectAncoraBin() (string, error) {
	if path, err := exec.LookPath("ancora"); err == nil {
		return path, nil
	}
	for _, candidate := range []string{
		"/opt/homebrew/bin/ancora",
		"/usr/local/bin/ancora",
		fmt.Sprintf("%s/.local/bin/ancora", os.Getenv("HOME")),
	} {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("ancora binary not found")
}

// installAncora installs the Ancora binary using Homebrew (preferred on macOS)
// or the official install script as fallback.
func installAncora() error {
	brew, err := exec.LookPath("brew")
	if err == nil {
		return installAncoraViaBrew(brew)
	}
	return installAncoraViaScript()
}

// installAncoraViaBrew installs via brew tap + trust + install.
func installAncoraViaBrew(brew string) error {
	tap := exec.Command(brew, "tap", "Syfra3/tap")
	tap.Stdout = os.Stdout
	tap.Stderr = os.Stderr
	if err := tap.Run(); err != nil {
		return fmt.Errorf("brew tap Syfra3/tap: %w", err)
	}

	// Homebrew requires explicit trust for third-party taps.
	trust := exec.Command(brew, "trust", "Syfra3/tap")
	trust.Stdout = os.Stdout
	trust.Stderr = os.Stderr
	if err := trust.Run(); err != nil {
		return fmt.Errorf("brew trust Syfra3/tap: %w", err)
	}

	install := exec.Command(brew, "install", "ancora")
	install.Stdout = os.Stdout
	install.Stderr = os.Stderr
	return install.Run()
}

// installAncoraViaScript installs via the official bash install script.
// Source: https://github.com/Syfra3/ancora/blob/main/scripts/install-ancora.sh
func installAncoraViaScript() error {
	curl, err := exec.LookPath("curl")
	if err != nil {
		return fmt.Errorf("neither brew nor curl is available")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		return fmt.Errorf("bash not found")
	}

	// curl -sSL <url> | bash
	curlCmd := exec.Command(curl, "-sSL",
		"https://raw.githubusercontent.com/Syfra3/ancora/main/scripts/install-ancora.sh")
	bashCmd := exec.Command(bash)

	curlOut, err := curlCmd.Output()
	if err != nil {
		return fmt.Errorf("download install script: %w", err)
	}

	bashCmd.Stdin = bytes.NewReader(curlOut)
	bashCmd.Stdout = os.Stdout
	bashCmd.Stderr = os.Stderr
	return bashCmd.Run()
}

// runAncoraSetup runs `ancora setup <agent>` which configures MCP, plugins,
// and permissions for the given target (claude-code or opencode).
func runAncoraSetup(binPath, agent string) error {
	cmd := exec.Command(binPath, "setup", agent)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin // ancora setup may prompt interactively
	return cmd.Run()
}
