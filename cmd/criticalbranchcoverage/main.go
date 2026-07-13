package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type branchOutcome struct {
	ID       string `json:"id"`
	Function string `json:"function"`
	Outcome  string `json:"outcome"`
}

type inventory struct {
	Metric                string   `json:"metric"`
	MinimumBranchCoverage float64  `json:"minimum_branch_coverage"`
	Functions             []string `json:"functions"`
}

type report struct {
	Metric          string           `json:"metric"`
	Coverage        float64          `json:"coverage"`
	CoveredOutcomes int              `json:"covered_outcomes"`
	TotalOutcomes   int              `json:"total_outcomes"`
	MissingOutcomes []branchOutcome  `json:"missing_outcomes"`
	Minimum         float64          `json:"minimum"`
	Passed          bool             `json:"passed"`
	Instrumentation string           `json:"instrumentation"`
	Functions       []functionReport `json:"functions"`
}

type functionReport struct {
	Function        string  `json:"function"`
	Coverage        float64 `json:"coverage"`
	CoveredOutcomes int     `json:"covered_outcomes"`
	TotalOutcomes   int     `json:"total_outcomes"`
}

func main() {
	inventoryPath := flag.String("inventory", ".rotta/critical-path-coverage.json", "critical-path inventory")
	reportPath := flag.String("report", "reports/critical-path-branch-coverage.json", "output report")
	flag.Parse()
	if err := run(*inventoryPath, *reportPath); err != nil {
		fmt.Fprintln(os.Stderr, "critical-path branch coverage FAILED:", err)
		os.Exit(1)
	}
}

func run(inventoryPath, reportPath string) error {
	spec, err := loadInventory(inventoryPath)
	if err != nil {
		return err
	}
	tempRoot, temporaryFS, expected, packages, err := prepareInstrumentation(spec)
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempRoot)
	defer temporaryFS.Close()
	observed, err := runInstrumentedTests(tempRoot, temporaryFS, packages)
	if err != nil {
		return err
	}
	return writeReport(reportPath, spec, expected, observed)
}

func loadInventory(path string) (inventory, error) {
	data, err := readFileAt(path)
	if err != nil {
		return inventory{}, err
	}
	var spec inventory
	if err := json.Unmarshal(data, &spec); err != nil {
		return inventory{}, err
	}
	if spec.MinimumBranchCoverage <= 0 || len(spec.Functions) == 0 {
		return inventory{}, errors.New("inventory must define a positive minimum_branch_coverage and critical functions")
	}
	return spec, nil
}

func prepareInstrumentation(spec inventory) (string, *os.Root, map[string]branchOutcome, map[string]bool, error) {
	root, err := os.Getwd()
	if err != nil {
		return "", nil, nil, nil, err
	}
	tempRoot, err := os.MkdirTemp("", "rotta-critical-branch-coverage-")
	if err != nil {
		return "", nil, nil, nil, err
	}
	if err := copyTree(root, tempRoot); err != nil {
		return "", nil, nil, nil, err
	}
	temporaryFS, err := os.OpenRoot(tempRoot)
	if err != nil {
		return "", nil, nil, nil, err
	}
	expected, packages, err := instrumentFiles(temporaryFS, spec.Functions)
	if err != nil {
		if closeErr := temporaryFS.Close(); closeErr != nil {
			return "", nil, nil, nil, fmt.Errorf("instrument critical source: %w; close temporary root: %v", err, closeErr)
		}
		return "", nil, nil, nil, err
	}
	return tempRoot, temporaryFS, expected, packages, nil
}

func instrumentFiles(temporaryFS *os.Root, functions []string) (map[string]branchOutcome, map[string]bool, error) {
	byFile, err := functionsByFile(functions)
	if err != nil {
		return nil, nil, err
	}
	expected := make(map[string]branchOutcome)
	packages := make(map[string]bool)
	for filename, functions := range byFile {
		source, err := temporaryFS.ReadFile(filename)
		if err != nil {
			return nil, nil, err
		}
		instrumented, outcomes, err := instrumentSource(filename, source, functions)
		if err != nil {
			return nil, nil, fmt.Errorf("instrument %s: %w", filename, err)
		}
		if err := temporaryFS.WriteFile(filename, instrumented, 0o600); err != nil {
			return nil, nil, err
		}
		for _, outcome := range outcomes {
			expected[outcome.ID+"\t"+outcome.Outcome] = outcome
		}
		packages["./"+filepath.ToSlash(filepath.Dir(filename))] = true
	}
	if len(expected) == 0 {
		return nil, nil, errors.New("selected critical functions contain no instrumentable if or switch branches")
	}
	return expected, packages, nil
}

func runInstrumentedTests(tempRoot string, temporaryFS *os.Root, packages map[string]bool) (map[string]bool, error) {
	outputPath := filepath.Join(tempRoot, "branch-outcomes.tsv")
	for pkg := range packages {
		directory := strings.TrimPrefix(pkg, "./")
		if err := writeRuntime(temporaryFS, filepath.Join(directory, "rotta_branch_coverage_runtime.go"), filepath.Base(directory)); err != nil {
			return nil, err
		}
	}
	for pkg := range packages {
		cmd, err := goTestCommand(pkg)
		if err != nil {
			return nil, err
		}
		cmd.Dir = tempRoot
		cmd.Env = append(os.Environ(), "ROTTA_BRANCH_COVERAGE_OUT="+outputPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("%s: %w", strings.Join(cmd.Args, " "), err)
		}
	}

	observed, err := readObserved(outputPath)
	if err != nil {
		return nil, err
	}
	return observed, nil
}

func writeReport(reportPath string, spec inventory, expected map[string]branchOutcome, observed map[string]bool) error {
	missing, functionResults := summarizeOutcomes(spec, expected, observed)
	covered := len(expected) - len(missing)
	coverage := float64(covered) * 100 / float64(len(expected))
	passed := coverage >= spec.MinimumBranchCoverage && everyFunctionPasses(functionResults, spec.MinimumBranchCoverage)
	result := report{Metric: spec.Metric, Coverage: coverage, CoveredOutcomes: covered, TotalOutcomes: len(expected), MissingOutcomes: missing, Minimum: spec.MinimumBranchCoverage, Passed: passed, Instrumentation: "AST-rewritten temporary source records each selected if true/false outcome and each selected switch case; the repository source is not modified.", Functions: functionResults}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	if err := writeFileAt(reportPath, append(encoded, '\n')); err != nil {
		return err
	}
	fmt.Printf("critical-path branch outcomes: %d/%d = %.2f%% (minimum %.2f%%)\n", covered, len(expected), coverage, spec.MinimumBranchCoverage)
	if !result.Passed {
		return errors.New("branch-outcome coverage is below the configured minimum")
	}
	return nil
}

func summarizeOutcomes(spec inventory, expected map[string]branchOutcome, observed map[string]bool) ([]branchOutcome, []functionReport) {
	missing := make([]branchOutcome, 0)
	functionTotals := make(map[string]int)
	functionMissing := make(map[string]int)
	for _, function := range spec.Functions {
		functionTotals[function] = 0
	}
	for key, outcome := range expected {
		functionTotals[outcome.Function]++
		if !observed[key] {
			missing = append(missing, outcome)
			functionMissing[outcome.Function]++
		}
	}
	sort.Slice(missing, func(i, j int) bool { return missing[i].ID+missing[i].Outcome < missing[j].ID+missing[j].Outcome })
	functionResults := make([]functionReport, 0, len(functionTotals))
	for function, total := range functionTotals {
		missingCount := functionMissing[function]
		functionCoverage := 100.0
		if total > 0 {
			functionCoverage = float64(total-missingCount) * 100 / float64(total)
		}
		functionResults = append(functionResults, functionReport{Function: function, Coverage: functionCoverage, CoveredOutcomes: total - missingCount, TotalOutcomes: total})
	}
	sort.Slice(functionResults, func(i, j int) bool { return functionResults[i].Function < functionResults[j].Function })
	return missing, functionResults
}

func everyFunctionPasses(functions []functionReport, minimum float64) bool {
	for _, function := range functions {
		if function.Coverage < minimum {
			return false
		}
	}
	return true
}

func functionsByFile(functions []string) (map[string]map[string]bool, error) {
	byFile := make(map[string]map[string]bool)
	for _, reference := range functions {
		index := strings.LastIndex(reference, ":")
		if index < 1 || index == len(reference)-1 {
			return nil, fmt.Errorf("invalid function reference %q", reference)
		}
		filename, name := reference[:index], reference[index+1:]
		if byFile[filename] == nil {
			byFile[filename] = make(map[string]bool)
		}
		byFile[filename][name] = true
	}
	return byFile, nil
}

func copyTree(sourceRoot, destinationRoot string) error {
	sourceFS, err := os.OpenRoot(sourceRoot)
	if err != nil {
		return err
	}
	defer sourceFS.Close()
	destinationFS, err := os.OpenRoot(destinationRoot)
	if err != nil {
		return err
	}
	defer destinationFS.Close()
	return fs.WalkDir(sourceFS.FS(), ".", func(relative string, entry fs.DirEntry, walkErr error) error {
		return copyTreeEntry(sourceFS, destinationFS, relative, entry, walkErr)
	})
}

func copyTreeEntry(sourceFS, destinationFS *os.Root, relative string, entry fs.DirEntry, walkErr error) error {
	if walkErr != nil {
		return walkErr
	}
	if relative == "." {
		return nil
	}
	if entry.IsDir() && (relative == ".git" || relative == ".vela" || relative == "bin") {
		return filepath.SkipDir
	}
	if entry.IsDir() {
		return destinationFS.MkdirAll(relative, 0o750)
	}
	if !entry.Type().IsRegular() {
		return nil
	}
	return copyRegularFile(sourceFS, destinationFS, relative)
}

func copyRegularFile(sourceFS, destinationFS *os.Root, relative string) error {
	if err := destinationFS.MkdirAll(filepath.Dir(relative), 0o750); err != nil {
		return err
	}
	input, err := sourceFS.Open(relative)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := destinationFS.OpenFile(relative, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func writeRuntime(root *os.Root, path, packageName string) error {
	runtime := fmt.Sprintf(`package %s

import (
	"fmt"
	"os"
	"sync"
)

var rottaBranchCoverageMu sync.Mutex

func rottaBranchOutcome(id string, outcome bool) bool {
	rottaRecordBranchOutcome(id, fmt.Sprintf("%%t", outcome))
	return outcome
}

func rottaBranchCase(id string) { rottaRecordBranchOutcome(id, "taken") }

func rottaRecordBranchOutcome(id, outcome string) {
	path := os.Getenv("ROTTA_BRANCH_COVERAGE_OUT")
	if path == "" { return }
	rottaBranchCoverageMu.Lock()
	defer rottaBranchCoverageMu.Unlock()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err == nil {
		_, _ = fmt.Fprintf(file, "%%s\t%%s\n", id, outcome)
		_ = file.Close()
	} else {
		fmt.Fprintln(os.Stderr, "branch coverage write failed:", err)
	}
}
`, packageName)
	return root.WriteFile(path, []byte(runtime), 0o600)
}

func readObserved(path string) (map[string]bool, error) {
	data, err := readFileAt(path)
	if os.IsNotExist(err) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	observed := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line != "" {
			observed[line] = true
		}
	}
	return observed, nil
}

func readFileAt(path string) ([]byte, error) {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.ReadFile(filepath.Base(path))
}

func writeFileAt(path string, data []byte) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return err
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer root.Close()
	return root.WriteFile(filepath.Base(path), data, 0o600)
}

func goTestCommand(pkg string) (*exec.Cmd, error) {
	switch pkg {
	case "./internal/installer":
		return exec.Command("go", "test", "./internal/installer", "-count=1"), nil
	case "./internal/tui":
		return exec.Command("go", "test", "./internal/tui", "-count=1"), nil
	default:
		return nil, fmt.Errorf("unsupported critical-path package %q", pkg)
	}
}

func instrumentSource(filename string, source []byte, functions map[string]bool) ([]byte, []branchOutcome, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, source, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}

	var outcomes []branchOutcome
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Body == nil || !functions[function.Name.Name] {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			if ifStatement, ok := node.(*ast.IfStmt); ok {
				id := fmt.Sprintf("%s:%s:if:%d", filename, function.Name.Name, fset.Position(ifStatement.If).Line)
				ifStatement.Cond = &ast.CallExpr{
					Fun:  ast.NewIdent("rottaBranchOutcome"),
					Args: []ast.Expr{&ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", id)}, ifStatement.Cond},
				}
				outcomes = append(outcomes,
					branchOutcome{ID: id, Function: filename + ":" + function.Name.Name, Outcome: "true"},
					branchOutcome{ID: id, Function: filename + ":" + function.Name.Name, Outcome: "false"},
				)
			}
			if switchStatement, ok := node.(*ast.SwitchStmt); ok {
				for index, statement := range switchStatement.Body.List {
					caseClause := statement.(*ast.CaseClause)
					id := fmt.Sprintf("%s:%s:switch:%d:case:%d", filename, function.Name.Name, fset.Position(switchStatement.Switch).Line, index)
					probe := &ast.ExprStmt{X: &ast.CallExpr{Fun: ast.NewIdent("rottaBranchCase"), Args: []ast.Expr{&ast.BasicLit{Kind: token.STRING, Value: fmt.Sprintf("%q", id)}}}}
					caseClause.Body = append([]ast.Stmt{probe}, caseClause.Body...)
					outcomes = append(outcomes, branchOutcome{ID: id, Function: filename + ":" + function.Name.Name, Outcome: "taken"})
				}
			}
			return true
		})
	}

	var formatted bytes.Buffer
	if err := format.Node(&formatted, fset, file); err != nil {
		return nil, nil, err
	}
	return formatted.Bytes(), outcomes, nil
}
