package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
		case "backup":
			return runBackupCommand(args[1:], stdout, stderr)
		case "restore":
			return runRestoreCommand(args[1:], stdout, stderr)
		case "workflow":
			return runWorkflowCommand(args[1:], stdout, stderr)
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

func runWorkflowCommand(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("workflow requires one of: preflight, handoff-validate, scoped-verify, publication-plan, benchmark")
	}
	command := args[0]
	if command == "benchmark" {
		return runOutcomeBenchmarkCommand(args[1:], stdout, stderr)
	}
	if command != "preflight" && command != "handoff-validate" && command != "scoped-verify" && command != "publication-plan" {
		return fmt.Errorf("unknown workflow command %q", command)
	}
	flags := flag.NewFlagSet("workflow "+command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	worktree := flags.String("worktree", "", "canonical relative worktree path")
	feature := flags.String("feature", "", "lowercase hyphenated feature ID")
	contract := flags.String("contract", "", "canonical relative contract path")
	baseline := flags.String("baseline", "", "baseline commit")
	handoff := flags.String("handoff", "", "handoff task ID")
	evidencePath := flags.String("evidence", "", "canonical relative durable evidence path")
	evidenceHash := flags.String("evidence-hash", "", "durable evidence output hash")
	var scope, packages stringListFlag
	flags.Var(&scope, "scope", "canonical relative scope path (repeatable)")
	flags.Var(&packages, "package", "canonical Go package for scoped verification (repeatable)")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("workflow %s accepts flags only", command)
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve current directory: %w", err)
	}
	// One context is shared by every participating workflow role for this CLI
	// task. Nil dependencies deliberately select bounded workspace/Git and
	// source fallbacks; the CLI never discovers, configures, or invokes an
	// external advisory integration on its own.
	advisory := workflow.NewWorkflowAdvisoryContext(workingDirectory, nil, nil)
	request := workflow.WorkflowCommandRequest{
		Worktree: *worktree, Feature: *feature, ContractPath: *contract, Baseline: *baseline,
		Scope: []string(scope), HandoffID: *handoff, EvidencePath: *evidencePath, EvidenceHash: *evidenceHash,
		VerificationPkgs: []string(packages), Advisory: advisory,
	}
	if command == "scoped-verify" {
		// CLI execution is the explicit production opt-in. Library and package
		// tests stay inert unless they inject a fake or disabled presenter.
		request.RTKPresentation = workflow.DefaultRTKPresentation()
	}
	var result workflow.WorkflowCommandResult
	switch command {
	case "preflight":
		result, err = workflow.RunWorkflowPreflight(workingDirectory, request)
	case "handoff-validate":
		result, err = workflow.RunWorkflowHandoffValidation(workingDirectory, request)
	case "scoped-verify":
		result, err = workflow.RunScopedVerification(workingDirectory, request)
	case "publication-plan":
		result, err = workflow.RunPublicationPlan(workingDirectory, request)
	}
	if result.Format != "" {
		if renderErr := writeWorkflowCommandResult(stdout, result); renderErr != nil {
			return renderErr
		}
	}
	return err
}

func runOutcomeBenchmarkCommand(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("workflow benchmark", flag.ContinueOnError)
	flags.SetOutput(stderr)
	worktree := flags.String("worktree", ".", "canonical relative worktree path")
	inputPath := flags.String("input", "", "canonical relative REQ-091 benchmark input JSON path")
	recordsDirectory := flags.String("records-dir", ".rotta/benchmarks/req-091", "canonical relative immutable record directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("workflow benchmark accepts flags only")
	}
	if *inputPath == "" {
		return fmt.Errorf("workflow benchmark requires --input")
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve current directory: %w", err)
	}
	worktreePath, err := confinedCLIWorktree(workingDirectory, *worktree)
	if err != nil {
		return fmt.Errorf("workflow benchmark worktree: %w", err)
	}
	inputFile, err := confinedCLIPath(worktreePath, *inputPath)
	if err != nil {
		return fmt.Errorf("workflow benchmark input: %w", err)
	}
	if filepath.IsAbs(*recordsDirectory) {
		return fmt.Errorf("workflow benchmark records directory must be relative")
	}
	contents, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("read workflow benchmark input: %w", err)
	}
	input, err := workflow.UnmarshalOutcomeBenchmarkInput(contents)
	if err != nil {
		return err
	}
	result, paths, err := workflow.PersistAndCompareOutcomeBenchmark(worktreePath, *recordsDirectory, input)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(struct {
		workflow.OutcomeBenchmarkResult
		Records []string `json:"records"`
	}{OutcomeBenchmarkResult: result, Records: paths})
	if err != nil {
		return fmt.Errorf("encode workflow benchmark result: %w", err)
	}
	_, err = fmt.Fprintln(stdout, string(encoded))
	return err
}

func confinedCLIPath(root, value string) (string, error) {
	if strings.TrimSpace(value) == "" || filepath.IsAbs(value) || filepath.Clean(value) != value {
		return "", fmt.Errorf("path must be non-empty, relative, and canonical")
	}
	base, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	base, err = filepath.EvalSymlinks(base)
	if err != nil {
		return "", fmt.Errorf("resolve canonical root: %w", err)
	}
	target, err := filepath.Abs(filepath.Join(base, value))
	if err != nil {
		return "", err
	}
	target, err = filepath.EvalSymlinks(target)
	if err != nil {
		return "", fmt.Errorf("resolve canonical path: %w", err)
	}
	relative, err := filepath.Rel(base, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes worktree")
	}
	return target, nil
}

func confinedCLIWorktree(root, value string) (string, error) {
	path, err := confinedCLIPath(root, value)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("worktree is not a directory")
	}
	return path, nil
}

type stringListFlag []string

func (values *stringListFlag) String() string { return strings.Join(*values, ",") }
func (values *stringListFlag) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func writeWorkflowCommandResult(stdout io.Writer, result workflow.WorkflowCommandResult) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode workflow command result: %w", err)
	}
	if _, err := fmt.Fprintln(stdout, string(encoded)); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "%s: %s; evidence: %s#%s; remediation: %s\n", result.Command, result.Status, result.EvidencePath, result.EvidenceHash, result.Remediation)
	return err
}

func runInstallCommand(args []string, stdout, stderr io.Writer) error {
	for _, arg := range args {
		if arg == "--vela" || arg == "-vela" || len(arg) > len("--vela=") && arg[:len("--vela=")] == "--vela=" {
			return fmt.Errorf("rotta install --vela is TUI-only: use the dedicated OpenCode Vela selection and confirmation")
		}
	}
	flags := flag.NewFlagSet("install", flag.ContinueOnError)
	flags.SetOutput(stderr)
	target := flags.String("target", "both", "install target: claude-code, opencode, or both")
	projectPath := flags.String("project", "", "project path")
	setupAncora := flags.Bool("ancora", false, "set up Ancora integration")
	if err := flags.Parse(args); err != nil {
		return err
	}
	result, err := installer.Install(installer.Options{
		Target:        *target,
		ProjectPath:   *projectPath,
		SetupAncora:   *setupAncora,
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
	target := flags.String("target", "both", "backup target: claude-code, opencode, or both")
	projectPath := flags.String("project", "", "project path")
	setupAncora := flags.Bool("ancora", false, "include Ancora integration")
	setupVela := flags.Bool("vela", false, "include Vela integration")
	if err := flags.Parse(args); err != nil {
		return err
	}
	backupDir, err := installer.Backup(installer.Options{
		Target:      *target,
		ProjectPath: *projectPath,
		SetupAncora: *setupAncora,
		SetupVela:   *setupVela,
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
