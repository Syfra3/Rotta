package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/Syfra3/clean-workflow/internal/installer"
	"github.com/Syfra3/clean-workflow/internal/tui"
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
			fmt.Fprintf(stdout, "clean-workflow %s\n", version)
			return nil
		case "install":
			return runInstallCommand(args[1:], stdout, stderr)
		case "backup":
			return runBackupCommand(args[1:], stdout, stderr)
		case "restore":
			return runRestoreCommand(args[1:], stdout, stderr)
		default:
			return fmt.Errorf("unknown command %q", args[0])
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
	flags := flag.NewFlagSet("install", flag.ContinueOnError)
	flags.SetOutput(stderr)
	target := flags.String("target", "both", "install target: claude-code, opencode, or both")
	projectPath := flags.String("project", "", "project path")
	installSpec := flags.Bool("spec", false, "install spec workflow")
	installImpl := flags.Bool("impl", false, "install implementation workflow")
	installReview := flags.Bool("review", false, "install review workflow")
	setupAncora := flags.Bool("ancora", false, "set up Ancora integration")
	setupVela := flags.Bool("vela", false, "set up Vela integration")
	if err := flags.Parse(args); err != nil {
		return err
	}
	result, err := installer.Install(installer.Options{
		Target:        *target,
		ProjectPath:   *projectPath,
		InstallSpec:   *installSpec,
		InstallImpl:   *installImpl,
		InstallReview: *installReview,
		SetupAncora:   *setupAncora,
		SetupVela:     *setupVela,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Installed clean-workflow for %s\n", result.Target)
	fmt.Fprintf(stdout, "Backup: %s\n", result.BackupDir)
	return nil
}

func runBackupCommand(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("backup", flag.ContinueOnError)
	flags.SetOutput(stderr)
	target := flags.String("target", "both", "backup target: claude-code, opencode, or both")
	projectPath := flags.String("project", "", "project path")
	installSpec := flags.Bool("spec", false, "include spec workflow")
	installImpl := flags.Bool("impl", false, "include implementation workflow")
	installReview := flags.Bool("review", false, "include review workflow")
	setupAncora := flags.Bool("ancora", false, "include Ancora integration")
	setupVela := flags.Bool("vela", false, "include Vela integration")
	if err := flags.Parse(args); err != nil {
		return err
	}
	backupDir, err := installer.Backup(installer.Options{
		Target:        *target,
		ProjectPath:   *projectPath,
		InstallSpec:   *installSpec,
		InstallImpl:   *installImpl,
		InstallReview: *installReview,
		SetupAncora:   *setupAncora,
		SetupVela:     *setupVela,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Backup: %s\n", backupDir)
	return nil
}

func runRestoreCommand(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("restore", flag.ContinueOnError)
	flags.SetOutput(stderr)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return fmt.Errorf("restore requires a backup directory")
	}
	backupDir := flags.Arg(0)
	if _, err := installer.RestoreBackup(backupDir); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Restored backup: %s\n", backupDir)
	return nil
}
