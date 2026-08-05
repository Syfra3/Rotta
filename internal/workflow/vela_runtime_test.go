package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const runtimeTestCommit = "0123456789abcdef0123456789abcdef01234567"

func TestRunLocalVelaExecutesLocalCLIAndGatesIndexing(t *testing.T) {
	repo := t.TempDir()
	bin := t.TempDir()
	logPath := filepath.Join(repo, "vela.log")
	program := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$VELA_LOG\"\nprintf 'local evidence'\n"
	if err := os.WriteFile(filepath.Join(bin, "vela"), []byte(program), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("VELA_LOG", logPath)

	if _, err := RunLocalVela(context.Background(), repo, runtimeTestCommit, "index", "", false); err == nil {
		t.Fatal("RunLocalVela() accepted indexing without consent")
	}
	result, err := RunLocalVela(context.Background(), repo, runtimeTestCommit, "query", "what depends on workflow?", false)
	if err != nil {
		t.Fatalf("RunLocalVela() error = %v", err)
	}
	if !result.Succeeded || result.Executable == "" || result.Output != "local evidence" {
		t.Fatalf("RunLocalVela() = %#v", result)
	}
	log, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(log), "explore what depends on workflow?") || !strings.Contains(string(log), "--graph "+filepath.Join(repo, ".vela", "graph.json")) {
		t.Fatalf("vela invocation = %q", log)
	}
}
