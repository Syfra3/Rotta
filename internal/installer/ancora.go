package installer

import (
	"bytes"
	"fmt"
	"io"
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
		if installErr := installAncora(opts); installErr != nil {
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
	if opts.Target == "claude-code" || opts.Target == "both" || opts.Target == "all" {
		if err := runAncoraSetup(opts, binPath, "claude-code"); err != nil {
			return nil, fmt.Errorf("ancora setup claude-code: %w", err)
		}
	}

	if opts.Target == "opencode" || opts.Target == "both" || opts.Target == "all" {
		if err := runAncoraSetup(opts, binPath, "opencode"); err != nil {
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
		if exists, err := fileExistsWithinParent(candidate); err == nil && exists {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("ancora binary not found")
}

// installAncora installs the Ancora binary using Homebrew (preferred on macOS)
// or the official install script as fallback.
func installAncora(opts Options) error {
	brew, err := exec.LookPath("brew")
	if err == nil {
		return installAncoraViaBrew(opts, brew)
	}
	return installAncoraViaScript(opts)
}

// installAncoraViaBrew installs via brew tap + trust + install.
func installAncoraViaBrew(opts Options, brew string) error {
	tap := exec.Command(brew, "tap", "Syfra3/tap")
	configureCommandIO(tap, opts)
	if err := tap.Run(); err != nil {
		return fmt.Errorf("brew tap Syfra3/tap: %w", err)
	}

	// Homebrew requires explicit trust for third-party taps.
	trust := exec.Command(brew, "trust", "Syfra3/tap")
	configureCommandIO(trust, opts)
	if err := trust.Run(); err != nil {
		return fmt.Errorf("brew trust Syfra3/tap: %w", err)
	}

	install := exec.Command(brew, "install", "ancora")
	configureCommandIO(install, opts)
	return install.Run()
}

// installAncoraViaScript installs via the official bash install script.
// Source: https://github.com/Syfra3/ancora/blob/main/scripts/install-ancora.sh
func installAncoraViaScript(opts Options) error {
	_, err := exec.LookPath("curl")
	if err != nil {
		return fmt.Errorf("neither brew nor curl is available")
	}
	_, err = exec.LookPath("bash")
	if err != nil {
		return fmt.Errorf("bash not found")
	}

	// curl -sSL <url> | bash
	curlCmd := exec.Command("curl", "-sSL",
		"https://raw.githubusercontent.com/Syfra3/ancora/main/scripts/install-ancora.sh")
	bashCmd := exec.Command("bash")

	curlOut, err := curlCmd.Output()
	if err != nil {
		return fmt.Errorf("download install script: %w", err)
	}

	bashCmd.Stdin = bytes.NewReader(curlOut)
	bashCmd.Stdout = writerOrDefault(opts.CommandStdout, os.Stdout)
	bashCmd.Stderr = writerOrDefault(opts.CommandStderr, os.Stderr)
	return bashCmd.Run()
}

// runAncoraSetup runs `ancora setup <agent>` which configures MCP, plugins,
// and permissions for the given target (claude-code or opencode).
func runAncoraSetup(opts Options, _ string, agent string) error {
	if agent != "claude-code" && agent != "opencode" {
		return fmt.Errorf("unsupported Ancora setup target %q", agent)
	}
	cmd := exec.Command("ancora")
	cmd.Args = []string{"ancora", "setup", agent}
	configureCommandIO(cmd, opts)
	return cmd.Run()
}

func configureCommandIO(cmd *exec.Cmd, opts Options) {
	cmd.Stdout = writerOrDefault(opts.CommandStdout, os.Stdout)
	cmd.Stderr = writerOrDefault(opts.CommandStderr, os.Stderr)
	cmd.Stdin = readerOrDefault(opts.CommandStdin, os.Stdin)
}

func readerOrDefault(r io.Reader, fallback io.Reader) io.Reader {
	if r != nil {
		return r
	}
	return fallback
}

func writerOrDefault(w io.Writer, fallback io.Writer) io.Writer {
	if w != nil {
		return w
	}
	return fallback
}
