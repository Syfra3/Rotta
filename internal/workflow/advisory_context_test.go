package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

func TestREQ096_AncoraRecoveryOccursOnceAndFallsBackWithoutAuthority(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root+"/state.txt", "current state")
	sum := sha256.Sum256([]byte("current state"))
	recovery := AncoraAdvisoryRecovery{Decisions: []string{"approved scope is internal/workflow"}, References: []AncoraAdvisoryReference{{Path: "state.txt", SHA256: hex.EncodeToString(sum[:])}}}
	ancora := &fakeAncoraAdvisory{recovery: recovery}
	context := NewWorkflowAdvisoryContext(root, ancora, nil)
	first, second := context.RecoverAncoraOnce(), context.RecoverAncoraOnce()
	if ancora.recoverCalls != 1 || first.Source != "ancora" || second.Source != "ancora" {
		t.Fatalf("recovery = %#v / %#v, calls=%d", first, second, ancora.recoverCalls)
	}

	fallback := NewWorkflowAdvisoryContext(root, &fakeAncoraAdvisory{recoverErr: errors.New("outage")}, nil).RecoverAncoraOnce()
	if !fallback.Degraded || fallback.Source != "workspace_git" || !strings.Contains(fallback.EvidenceGap, "failed") {
		t.Fatalf("failure fallback = %#v", fallback)
	}
	if len(fallback.Context.Decisions) != 0 {
		t.Fatalf("fallback invented advisory authority: %#v", fallback)
	}
}

func TestREQ096_AncoraMissingOrStaleReferencesFallBackToWorkspaceGit(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root+"/state.txt", "current")
	for _, reference := range []AncoraAdvisoryReference{
		{Path: "missing.txt", SHA256: strings.Repeat("0", 64)},
		{Path: "state.txt", SHA256: strings.Repeat("0", 64)},
	} {
		result := NewWorkflowAdvisoryContext(root, &fakeAncoraAdvisory{recovery: AncoraAdvisoryRecovery{References: []AncoraAdvisoryReference{reference}}}, nil).RecoverAncoraOnce()
		if result.Source != "workspace_git" || !result.Degraded || !strings.Contains(result.EvidenceGap, "not compact") {
			t.Fatalf("reference fallback = %#v", result)
		}
	}
}

func TestREQ096_AncoraAcceptsOnlyCompactMaterialSummaries(t *testing.T) {
	ancora := &fakeAncoraAdvisory{}
	context := NewAdvisoryContext(ancora, nil)
	if result := context.SaveMaterialSummary(AncoraMaterialSummary{Kind: AncoraMaterialOutcome, Summary: "focused workflow tests passed"}); !result.Stored || ancora.saveCalls != 1 {
		t.Fatalf("material summary result = %#v, calls=%d", result, ancora.saveCalls)
	}
	if result := context.SaveMaterialSummary(AncoraMaterialSummary{Kind: AncoraMaterialOutcome, Summary: "raw\ntranscript"}); result.Stored || ancora.saveCalls != 1 || !strings.Contains(result.EvidenceGap, "compact") {
		t.Fatalf("raw summary result = %#v, calls=%d", result, ancora.saveCalls)
	}
	ancora.saveErr = errors.New("outage")
	if result := context.SaveMaterialSummary(AncoraMaterialSummary{Kind: AncoraMaterialFix, Summary: "fallback retained"}); result.Stored || !strings.Contains(result.EvidenceGap, "workspace/Git") {
		t.Fatalf("save outage result = %#v", result)
	}
}

func TestREQ096_VelaUsesNamedQuestionsWithinExistingBudgetsAndDistillsEvidence(t *testing.T) {
	vela := &fakeVelaAdvisory{evidence: validVelaEvidence()}
	context := NewAdvisoryContext(nil, vela)
	question := VelaQuestion{Role: VelaExploreRole, Kind: VelaDependency, Subject: "internal/workflow/advisory_context.go", Question: "What depends on AdvisoryContext?"}
	for attempt := 0; attempt < maxVelaExploreCalls; attempt++ {
		result := context.AskVela(question)
		if result.Source != "vela" || result.Evidence.SafeAction == "" || len(result.Evidence.Files) != 1 {
			t.Fatalf("Vela result = %#v", result)
		}
	}
	if result := context.AskVela(question); result.Source != "source" || !strings.Contains(result.EvidenceGap, "budget") || vela.calls != maxVelaExploreCalls {
		t.Fatalf("budget result = %#v, calls=%d", result, vela.calls)
	}
	if calls := context.VelaCalls(VelaExploreRole); calls != maxVelaExploreCalls {
		t.Fatalf("exploration calls = %d", calls)
	}
	if result := NewAdvisoryContext(nil, vela).AskVela(VelaQuestion{Role: VelaReviewRole, Kind: VelaImpact, Question: "What breaks?"}); result.Source != "source" || vela.calls != maxVelaExploreCalls {
		t.Fatalf("unnamed Vela request = %#v, calls=%d", result, vela.calls)
	}
}

func TestREQ096_VelaFailureFallsBackAndDoesNotRetryOrOperate(t *testing.T) {
	vela := &fakeVelaAdvisory{err: errors.New("unavailable")}
	context := NewAdvisoryContext(nil, vela)
	question := VelaQuestion{Role: VelaReviewRole, Kind: VelaArchitecturalFlow, Subject: "internal/workflow/handoff.go", Question: "Which route reaches recovery?"}
	result := context.AskVela(question)
	if result.Source != "source" || !strings.Contains(result.EvidenceGap, "source evidence") || vela.calls != 1 || context.VelaCalls(VelaReviewRole) != 1 {
		t.Fatalf("failure result = %#v, calls=%d", result, vela.calls)
	}
	if retry := context.AskVela(question); retry.Source != "source" || vela.calls != 1 {
		t.Fatalf("automatic retry occurred: %#v, calls=%d", retry, vela.calls)
	}
	// The injected Vela seam exposes only AnswerStructuralQuestion; this context
	// has no install/setup/index/re-index operation path to invoke.
}

func TestREQ096_RejectsRawOrAmbiguousVelaEvidence(t *testing.T) {
	vela := &fakeVelaAdvisory{evidence: VelaEvidence{Subject: "internal/workflow", Confidence: "high", SafeAction: "inspect source", Files: []string{"internal/workflow/advisory_context.go"}, Gaps: []string{"raw\ngraph output"}}}
	result := NewAdvisoryContext(nil, vela).AskVela(VelaQuestion{Role: VelaReviewRole, Kind: VelaOwnership, Subject: "internal/workflow", Question: "Who owns handoff persistence?"})
	if result.Source != "source" || !strings.Contains(result.EvidenceGap, "not compact") {
		t.Fatalf("raw evidence result = %#v", result)
	}
}

func TestREQ096_VelaRejectsStaleConflictingOrOutOfModuleFileEvidence(t *testing.T) {
	question := VelaQuestion{Role: VelaReviewRole, Kind: VelaOwnership, Subject: "internal/workflow", Question: "Who owns advisory recovery?"}
	for _, evidence := range []VelaEvidence{
		{Subject: "internal/other", Files: []string{"internal/workflow/advisory_context.go"}, Confidence: "medium", SafeAction: "inspect source"},
		{Subject: "internal/workflow", Files: []string{"internal/installer/instructions.go"}, Confidence: "medium", SafeAction: "inspect source"},
	} {
		result := NewAdvisoryContext(nil, &fakeVelaAdvisory{evidence: evidence}).AskVela(question)
		if result.Source != "source" || !strings.Contains(result.EvidenceGap, "stale, ambiguous") {
			t.Fatalf("invalid Vela evidence = %#v", result)
		}
	}
}

func TestREQ096_StartResumeAndWorkflowShareOneBoundedAdvisoryContext(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, root+"/spec.md", "# contract\n")
	mustWrite(t, root+"/feature.feature", "Feature: fixture\n")
	sum := sha256.Sum256([]byte("# contract\n"))
	ancora := &fakeAncoraAdvisory{recovery: AncoraAdvisoryRecovery{References: []AncoraAdvisoryReference{{Path: "spec.md", SHA256: hex.EncodeToString(sum[:])}}}}
	context := NewWorkflowAdvisoryContext(root, ancora, &fakeVelaAdvisory{evidence: VelaEvidence{Subject: "internal/demo", Files: []string{"internal/demo/demo.go"}, Confidence: "medium", SafeAction: "inspect source"}})
	if _, err := InitializeCurrentSubmission(root, CurrentSubmissionRequest{ID: "fixture", SpecPath: "spec.md", FeaturePaths: []string{"feature.feature"}, ScenarioIDs: []string{"SCN-001"}, Advisory: context}); err != nil {
		t.Fatal(err)
	}
	if _, err := ResumeCurrentSubmission(root, nil, context); err != nil {
		t.Fatal(err)
	}
	if recovery := context.RecoverAncoraOnce(); recovery.Source != "ancora" || ancora.recoverCalls != 1 {
		t.Fatalf("start/resume recovery = %#v, calls=%d", recovery, ancora.recoverCalls)
	}
	for range []int{0, 1, 2} {
		context.AskVela(VelaQuestion{Role: VelaExploreRole, Kind: VelaDependency, Subject: "internal/demo", Question: "What depends on demo?"})
	}
	if calls := context.VelaCalls(VelaExploreRole); calls != maxVelaExploreCalls {
		t.Fatalf("workflow Vela budget = %d", calls)
	}
}

type fakeAncoraAdvisory struct {
	recovery     AncoraAdvisoryRecovery
	recoverErr   error
	saveErr      error
	recoverCalls int
	saveCalls    int
}

func (fake *fakeAncoraAdvisory) RecoverRelevant() (AncoraAdvisoryRecovery, error) {
	fake.recoverCalls++
	return fake.recovery, fake.recoverErr
}
func (fake *fakeAncoraAdvisory) SaveMaterialSummary(AncoraMaterialSummary) error {
	fake.saveCalls++
	return fake.saveErr
}

type fakeVelaAdvisory struct {
	evidence VelaEvidence
	err      error
	calls    int
}

func (fake *fakeVelaAdvisory) AnswerStructuralQuestion(VelaQuestion) (VelaEvidence, error) {
	fake.calls++
	return fake.evidence, fake.err
}

func validVelaEvidence() VelaEvidence {
	return VelaEvidence{Subject: "internal/workflow/advisory_context.go", Symbols: []string{"AdvisoryContext"}, Files: []string{"internal/workflow/advisory_context.go"}, Confidence: "medium", Gaps: []string{"graph freshness not independently verified"}, SafeAction: "inspect current source before changing recovery"}
}
