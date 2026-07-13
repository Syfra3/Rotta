package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstrumentSourceRecordsBothIfOutcomes(t *testing.T) {
	source := []byte("package fixture\nfunc critical(ready bool) { if ready {} }\n")

	instrumented, outcomes, err := instrumentSource("fixture.go", source, map[string]bool{"critical": true})
	if err != nil {
		t.Fatal(err)
	}
	if len(outcomes) != 2 {
		t.Fatalf("expected true and false outcomes, got %#v", outcomes)
	}
	if !strings.Contains(string(instrumented), `rottaBranchOutcome("fixture.go:critical:if:2", ready)`) {
		t.Fatalf("expected source instrumentation, got:\n%s", instrumented)
	}
}

func TestInstrumentSourceRecordsEverySwitchCase(t *testing.T) {
	source := []byte("package fixture\nfunc critical(value int) { switch value { case 1: return; default: return } }\n")

	instrumented, outcomes, err := instrumentSource("fixture.go", source, map[string]bool{"critical": true})
	if err != nil {
		t.Fatal(err)
	}
	if len(outcomes) != 2 {
		t.Fatalf("expected two switch outcomes, got %#v", outcomes)
	}
	if !strings.Contains(string(instrumented), `rottaBranchCase("fixture.go:critical:switch:2:case:0")`) {
		t.Fatalf("expected first switch case instrumentation, got:\n%s", instrumented)
	}
}

func TestRunMeasuresConfiguredCriticalBranchesInTemporarySource(t *testing.T) {
	repositoryRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	previousDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previousDirectory) })
	if err := os.Chdir(repositoryRoot); err != nil {
		t.Fatal(err)
	}

	reportPath := filepath.Join(t.TempDir(), "branch-coverage.json")
	if err := run(".rotta/critical-path-coverage.json", reportPath); err != nil {
		t.Fatalf("run source-instrumented branch coverage: %v", err)
	}
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	var got report
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if !got.Passed || got.Coverage < got.Minimum || got.TotalOutcomes == 0 {
		t.Fatalf("expected a passing non-empty source-instrumented report, got %#v", got)
	}
}

func TestCoverageToolRejectsInvalidInputsAndReportsMissingOutcomes(t *testing.T) {
	temporary := t.TempDir()
	assertInvalidCoverageInputs(t, temporary)
	assertMissingOutcomeReport(t, temporary)
}

func TestCoverageToolCopiesFilesAndReadsObservedOutcomes(t *testing.T) {
	assertTreeCopy(t)
	assertObservedOutcomes(t)
}

func TestCoverageToolReportsFilesystemAndInstrumentationFailures(t *testing.T) {
	temporary := t.TempDir()
	assertCoverageInputFailures(t, temporary)
	assertCoverageFileFailures(t, temporary)
	assertInstrumentationFailures(t, temporary)
}

func assertInvalidCoverageInputs(t *testing.T, temporary string) {
	t.Helper()
	path := filepath.Join(temporary, "invalid.json")
	if err := os.WriteFile(path, []byte(`{"minimum_branch_coverage":0}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadInventory(path); err == nil {
		t.Fatal("expected invalid inventory")
	}
	if _, err := functionsByFile([]string{"not-a-reference"}); err == nil {
		t.Fatal("expected malformed function reference")
	}
	if _, _, err := instrumentSource("broken.go", []byte("package broken\nfunc"), map[string]bool{}); err == nil {
		t.Fatal("expected unparsable source")
	}
	if _, err := goTestCommand("./unsupported"); err == nil {
		t.Fatal("expected unsupported package")
	}
}

func assertMissingOutcomeReport(t *testing.T, temporary string) {
	t.Helper()
	expected := map[string]branchOutcome{"first\ttrue": {ID: "first", Function: "fixture.go:critical", Outcome: "true"}, "first\tfalse": {ID: "first", Function: "fixture.go:critical", Outcome: "false"}}
	path := filepath.Join(temporary, "report.json")
	if err := writeReport(path, inventory{Metric: "fixture", MinimumBranchCoverage: 100, Functions: []string{"fixture.go:critical"}}, expected, map[string]bool{"first\ttrue": true}); err == nil {
		t.Fatal("expected incomplete outcomes")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got report
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Passed || len(got.MissingOutcomes) != 1 || got.MissingOutcomes[0].Outcome != "false" {
		t.Fatalf("unexpected report %#v", got)
	}
}

func assertTreeCopy(t *testing.T) {
	t.Helper()
	source, destination := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(source, "nested"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "nested", "fixture.txt"), []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := copyTree(source, destination); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(destination, "nested", "fixture.txt"))
	if err != nil || string(data) != "fixture" {
		t.Fatalf("copy=%q err=%v", data, err)
	}
}

func assertObservedOutcomes(t *testing.T) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "observed.tsv")
	if err := os.WriteFile(path, []byte("first\ttrue\nfirst\ttrue\nsecond\ttaken\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readObserved(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !got["first\ttrue"] || !got["second\ttaken"] {
		t.Fatalf("outcomes=%#v", got)
	}
}

func assertCoverageInputFailures(t *testing.T, temporary string) {
	t.Helper()
	assertMissingCoverageInputs(t, temporary)
	assertInvalidCoverageInventories(t, temporary)
}

func assertMissingCoverageInputs(t *testing.T, temporary string) {
	t.Helper()
	if err := run(filepath.Join(temporary, "missing-inventory.json"), filepath.Join(temporary, "report.json")); err == nil {
		t.Fatal("expected missing inventory to fail the coverage run")
	}
	if _, err := readObserved(filepath.Join(temporary, "missing-observations.tsv")); err != nil {
		t.Fatalf("expected missing observations to mean no observed branches, got %v", err)
	}
	if _, err := readFileAt(filepath.Join(temporary, "missing.txt")); err == nil {
		t.Fatal("expected missing file read to fail")
	}
	if _, err := readObserved(temporary); err == nil {
		t.Fatal("expected observation directory read to fail")
	}
}

func assertInvalidCoverageInventories(t *testing.T, temporary string) {
	t.Helper()
	malformedInventory := filepath.Join(temporary, "malformed.json")
	if err := os.WriteFile(malformedInventory, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadInventory(malformedInventory); err == nil {
		t.Fatal("expected malformed inventory to fail")
	}
	missingFunctionInventory := filepath.Join(temporary, "missing-function.json")
	if err := os.WriteFile(missingFunctionInventory, []byte(`{"minimum_branch_coverage":95,"functions":["missing.go:critical"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run(missingFunctionInventory, filepath.Join(temporary, "missing-function-report.json")); err == nil {
		t.Fatal("expected unavailable configured source to fail the coverage run")
	}
	unsupportedPackageInventory := filepath.Join(temporary, "unsupported-package.json")
	if err := os.WriteFile(unsupportedPackageInventory, []byte(`{"minimum_branch_coverage":95,"functions":["main.go:run"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run(unsupportedPackageInventory, filepath.Join(temporary, "unsupported-package-report.json")); err == nil {
		t.Fatal("expected configured function outside a supported package to fail")
	}
}

func assertCoverageFileFailures(t *testing.T, temporary string) {
	t.Helper()
	blockedDirectory := filepath.Join(temporary, "blocked")
	if err := os.WriteFile(blockedDirectory, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readFileAt(filepath.Join(blockedDirectory, "child")); err == nil {
		t.Fatal("expected read through a regular-file parent to fail")
	}
	if err := writeFileAt(filepath.Join(blockedDirectory, "report.json"), []byte("report")); err == nil {
		t.Fatal("expected report write below a regular file to fail")
	}
	if err := writeReport(filepath.Join(blockedDirectory, "report.json"), inventory{Metric: "fixture", MinimumBranchCoverage: 100}, map[string]branchOutcome{"id\ttrue": {ID: "id", Function: "fixture:fn", Outcome: "true"}}, map[string]bool{"id\ttrue": true}); err == nil {
		t.Fatal("expected report creation below a regular file to fail")
	}
	if err := copyTree(filepath.Join(temporary, "missing-source"), t.TempDir()); err == nil {
		t.Fatal("expected missing source tree to fail copying")
	}
	copySource := t.TempDir()
	if err := os.WriteFile(filepath.Join(copySource, "fixture"), []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	copyDestination := filepath.Join(temporary, "copy-destination")
	if err := os.WriteFile(copyDestination, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := copyTree(copySource, copyDestination); err == nil {
		t.Fatal("expected regular-file copy destination to fail")
	}
}

func assertInstrumentationFailures(t *testing.T, temporary string) {
	t.Helper()
	root, err := os.OpenRoot(temporary)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	assertInstrumentSourceFailures(t, temporary, root)
	assertObservedOutputFailure(t)
}

func assertInstrumentSourceFailures(t *testing.T, temporary string, root *os.Root) {
	t.Helper()
	if _, _, err := instrumentFiles(root, []string{"missing.go:critical"}); err == nil {
		t.Fatal("expected missing critical source to fail instrumentation")
	}
	if err := os.WriteFile(filepath.Join(temporary, "empty.go"), []byte("package fixture\nfunc critical() {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := instrumentFiles(root, []string{"empty.go:critical"}); err == nil {
		t.Fatal("expected source without selected branches to fail instrumentation")
	}
	if err := os.WriteFile(filepath.Join(temporary, "broken.go"), []byte("package fixture\nfunc critical("), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := instrumentFiles(root, []string{"broken.go:critical"}); err == nil {
		t.Fatal("expected invalid selected source to fail instrumentation")
	}
	if err := copyRegularFile(root, root, "missing-copy-source"); err == nil {
		t.Fatal("expected a missing copied source file to fail")
	}
}

func assertObservedOutputFailure(t *testing.T) {
	t.Helper()
	outputDirectory := t.TempDir()
	if err := os.Mkdir(filepath.Join(outputDirectory, "branch-outcomes.tsv"), 0o750); err != nil {
		t.Fatal(err)
	}
	outputRoot, err := os.OpenRoot(outputDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer outputRoot.Close()
	if _, err := runInstrumentedTests(outputDirectory, outputRoot, map[string]bool{}); err == nil {
		t.Fatal("expected observation directory to fail result collection")
	}
}
