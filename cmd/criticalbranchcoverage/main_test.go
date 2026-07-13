package main

import (
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
