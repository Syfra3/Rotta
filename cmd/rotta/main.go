package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Syfra3/Rotta/internal/installer"
	"github.com/Syfra3/Rotta/internal/tui"
	"github.com/Syfra3/Rotta/internal/workflow"
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
		case "v2":
			return runV2Command(args[1:], stdout, stderr)
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

func runV2Command(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("v2 requires new, resume, or transition")
	}
	flags := flag.NewFlagSet("v2 "+args[0], flag.ContinueOnError)
	flags.SetOutput(stderr)
	repo := flags.String("repo", ".", "repository root")
	id := flags.String("id", "", "submission ID")
	draft := flags.String("draft", "", "submission draft")
	base := flags.String("base", "", "full base commit")
	expected := flags.String("expected", "", "expected lifecycle status")
	target := flags.String("target", "", "target lifecycle status")
	ledgerVersion := flags.String("ledger-version", "", "expected ledger version")
	scope := flags.String("scope", "", "comma-separated authorized scenario scope")
	evidence := flags.String("evidence", "", "comma-separated evidence references")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	root, err := filepath.Abs(*repo)
	if err != nil {
		return err
	}
	switch args[0] {
	case "new":
		ledger, err := workflow.InitializeV2NewSubmission(root, workflow.V2NewSubmissionRequest{SubmissionID: *id, Draft: *draft, BaseCommit: *base})
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(stdout, "%s %s v%d\n", ledger.SubmissionID, ledger.Status, ledger.LedgerVersion)
		return err
	case "resume":
		result, err := workflow.ResumeV2Submission(root, *id, nil)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(stdout, "%s %s v%d ancora=%s\n", result.Ledger.SubmissionID, result.Ledger.Status, result.Ledger.LedgerVersion, result.AncoraPointerState)
		return err
	case "transition":
		version, err := strconv.ParseUint(*ledgerVersion, 10, 64)
		if err != nil {
			return fmt.Errorf("parse ledger version: %w", err)
		}
		ledger, err := workflow.PersistV2Transition(root, workflow.V2TransitionRequest{SubmissionID: *id, ExpectedStatus: *expected, TargetStatus: *target, LedgerVersion: version, Authorizer: "orchestrator", AuthorizedScope: splitV2Values(*scope), EvidenceRefs: splitV2Values(*evidence)})
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(stdout, "%s %s v%d\n", ledger.SubmissionID, ledger.Status, ledger.LedgerVersion)
		return err
	default:
		return fmt.Errorf("unknown v2 command %q", args[0])
	}
}

func splitV2Values(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(value, ",")
}

func runInstallCommand(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("install", flag.ContinueOnError)
	flags.SetOutput(stderr)
	target := flags.String("target", "opencode", "install target: opencode")
	projectPath := flags.String("project", "", "project path")
	installSpec := flags.Bool("spec", false, "install spec workflow")
	installImpl := flags.Bool("impl", false, "install implementation workflow")
	installReview := flags.Bool("review", false, "install review workflow")
	setupAncora := flags.Bool("ancora", false, "set up Ancora integration")
	setupVela := flags.Bool("vela", false, "set up Vela integration")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := requireOpenCodeV2Target(*target); err != nil {
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
		CommandStdin:  os.Stdin,
		CommandStdout: stdout,
		CommandStderr: stderr,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Installed rotta for %s\n", result.Target)
	fmt.Fprintf(stdout, "Backup: %s\n", result.BackupDir)
	return nil
}

func runBackupCommand(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("backup", flag.ContinueOnError)
	flags.SetOutput(stderr)
	target := flags.String("target", "opencode", "backup target: opencode")
	projectPath := flags.String("project", "", "project path")
	installSpec := flags.Bool("spec", false, "include spec workflow")
	installImpl := flags.Bool("impl", false, "include implementation workflow")
	installReview := flags.Bool("review", false, "include review workflow")
	setupAncora := flags.Bool("ancora", false, "include Ancora integration")
	setupVela := flags.Bool("vela", false, "include Vela integration")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := requireOpenCodeV2Target(*target); err != nil {
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

func requireOpenCodeV2Target(target string) error {
	if target != "opencode" {
		return fmt.Errorf("unsupported v2 workflow target %q: only opencode is supported", target)
	}
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
