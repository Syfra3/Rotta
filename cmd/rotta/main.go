package main

import (
	"fmt"
	"io"
	"os"

	"github.com/Syfra3/Rotta/internal/installer"
	"github.com/Syfra3/Rotta/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
)

var version = "dev"

func main() {
	if err := runCLI(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func runCLI(args []string, stdout, stderr io.Writer) error {
	if len(args) > 0 {
		switch args[0] {
		case "--version", "version":
			fmt.Fprintf(stdout, "rotta %s\n", version)
			return nil
		case "install":
			return runInstallCommand(args[1:], stdout, stderr)
		case "status":
			return runStatusCommand(args[1:], stdout)
		default:
			return fmt.Errorf("unsupported command %q; supported commands: install, status, version", args[0])
		}
	}

	p := tea.NewProgram(
		tui.New(),
		tea.WithAltScreen(),
	)
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

func runInstallCommand(args []string, stdout, stderr io.Writer) error {
	if len(args) != 0 {
		return fmt.Errorf("unsupported install option %q; supported commands: install, status, version", args[0])
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	result, err := installer.InstallOpenCode(home)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Installed %d managed OpenCode assets\n", len(result.Files))
	return nil
}

func runStatusCommand(args []string, stdout io.Writer) error {
	if len(args) != 0 {
		return fmt.Errorf("unsupported status option %q; supported commands: install, status, version", args[0])
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	status := installer.OpenCodeStatus(home)
	_, err = fmt.Fprintf(stdout, "OpenCode installation: %s\nWorkflow status: unavailable\n", status.State)
	return err
}
