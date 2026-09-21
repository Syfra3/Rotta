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
	if len(args) == 0 || args[0] != "benchmark" {
		return fmt.Errorf("workflow requires benchmark subcommand")
	}
	flags := flag.NewFlagSet("workflow benchmark", flag.ContinueOnError)
	flags.SetOutput(stderr)
	input := flags.String("input", "", "directory containing exactly three canonical outcome JSON records")
	worktree := flags.String("worktree", "", "active worktree root")
	id := flags.String("id", "", "versioned benchmark identifier")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 || *input == "" || *worktree == "" || *id == "" {
		return fmt.Errorf("workflow benchmark requires --input, --worktree, and --id")
	}
	records, paths, err := readBenchmarkInput(*input)
	if err != nil {
		return err
	}
	result, err := workflow.RunRetainedBenchmark(workflow.BenchmarkRequest{Worktree: *worktree, BenchmarkID: *id, Records: records, CanonicalInputPaths: paths})
	if err != nil {
		return err
	}
	return json.NewEncoder(stdout).Encode(result)
}

func readBenchmarkInput(input string) ([]workflow.OutcomeRecord, []string, error) {
	root, err := filepath.EvalSymlinks(input)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve benchmark input: %w", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, nil, err
	}
	var records []workflow.OutcomeRecord
	var paths []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path, err := filepath.EvalSymlinks(filepath.Join(root, entry.Name()))
		if err != nil {
			return nil, nil, err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil || relative == ".." || len(relative) > 2 && relative[:3] == ".."+string(filepath.Separator) {
			return nil, nil, fmt.Errorf("benchmark input escapes input directory")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, err
		}
		var record workflow.OutcomeRecord
		if err := json.Unmarshal(data, &record); err != nil {
			return nil, nil, fmt.Errorf("parse %s: %w", path, err)
		}
		records = append(records, record)
		paths = append(paths, path)
	}
	if len(records) != 3 {
		return nil, nil, fmt.Errorf("benchmark input requires exactly three JSON records")
	}
	return records, paths, nil
}

func runInstallCommand(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("install", flag.ContinueOnError)
	flags.SetOutput(stderr)
	target := flags.String("target", "both", "install target: claude-code, opencode, codex, pi, both, or all")
	projectPath := flags.String("project", "", "project path")
	setupAncora := flags.Bool("ancora", false, "set up Ancora integration")
	setupVela := flags.Bool("vela", false, "set up Vela integration")
	setupTypeSafe := flags.Bool("typesafe", false, "auto-enable TypeSafe/Jev for Pi sessions")
	typeSafeKey := flags.String("typesafe-api-key", "", "TypeSafe/Jev API key to store for pi-typesafe (prefer the interactive TUI or /typesafe login)")
	routing := flags.String("model-routing", "", "OpenCode model routing: enabled, custom, or disabled")
	piRouting := flags.String("pi-model-routing", "", "Pi model routing: enabled, custom, or disabled")
	var piModels repeatedFlag
	flags.Var(&piModels, "pi-model", "Pi custom role=model assignment (repeat for all four roles)")
	confirmRouting := flags.Bool("confirm-model-routing", false, "confirm OpenCode model-routing changes")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if !validRoutingFlag(*routing) || !validRoutingFlag(*piRouting) {
		return fmt.Errorf("model routing must be enabled, custom, or disabled")
	}
	if (*target == "opencode" || *target == "both" || *target == "all") && !*confirmRouting {
		return fmt.Errorf("OpenCode model routing requires --confirm-model-routing")
	}
	piCustom, err := parseRoleModels(piModels)
	if err != nil {
		return err
	}
	result, err := installer.Install(installer.Options{
		Target:               *target,
		ProjectPath:          *projectPath,
		SetupAncora:          *setupAncora,
		SetupVela:            *setupVela,
		SetupTypeSafe:        *setupTypeSafe,
		TypeSafeAPIKey:       *typeSafeKey,
		ModelRouting:         installer.ModelRoutingRequest(*routing),
		PiModelRouting:       installer.ModelRoutingRequest(*piRouting),
		PiModelRoutingModels: piCustom,
		CommandStdin:         os.Stdin,
		CommandStdout:        stdout,
		CommandStderr:        stderr,
	})
	if result != nil {
		for _, warning := range result.Warnings {
			fmt.Fprintf(stderr, "Warning: %s\n", warning)
		}
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Installed rotta for %s\n", result.Target)
	fmt.Fprintf(stdout, "Backup: %s\n", result.BackupDir)
	if capability, ok := result.Hosts["opencode"].Capabilities["source_loading"]; ok {
		fmt.Fprintf(stdout, "OpenCode policy loading: %s\n%s\n%s\n", capability.Status, capability.Reason, capability.Remediation)
	}
	fmt.Fprintln(stdout, "Restart the coding agent to load the installed policy bundle.")
	return nil
}

type repeatedFlag []string

func (f *repeatedFlag) String() string         { return strings.Join(*f, ",") }
func (f *repeatedFlag) Set(value string) error { *f = append(*f, value); return nil }
func validRoutingFlag(value string) bool {
	return value == "" || value == string(installer.ModelRoutingEnabled) || value == string(installer.ModelRoutingCustom) || value == string(installer.ModelRoutingDisabled)
}
func parseRoleModels(values []string) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	models := make(map[string]string, len(values))
	for _, value := range values {
		role, model, ok := strings.Cut(value, "=")
		if !ok || role == "" || model == "" {
			return nil, fmt.Errorf("--pi-model must use role=model")
		}
		if _, exists := models[role]; exists {
			return nil, fmt.Errorf("--pi-model repeats role %s", role)
		}
		models[role] = model
	}
	return models, nil
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
