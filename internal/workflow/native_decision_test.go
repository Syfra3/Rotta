package workflow

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestStrictContractQuestionUsesExactChoicesAndCompleteBinding(t *testing.T) {
	binding := testNativeDecisionBinding()
	question, err := StrictContractQuestion(binding)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(question.Choices, "|"), "Approve exact contract|Request changes|Cancel"; got != want {
		t.Fatalf("choices = %q, want %q", got, want)
	}
	for _, value := range []string{binding.ContractPath, binding.FeatureID, binding.Baseline, binding.ContractFingerprint} {
		if !strings.Contains(question.Prompt, value) {
			t.Fatalf("prompt %q does not bind %q", question.Prompt, value)
		}
	}
}

func TestNativeDecisionRejectsEveryStaleBindingAndLegacyAnswer(t *testing.T) {
	question, err := StrictContractQuestion(testNativeDecisionBinding())
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*NativeDecisionBinding){
		func(v *NativeDecisionBinding) { v.QuestionID = "replaced" },
		func(v *NativeDecisionBinding) { v.SessionID = "restarted" },
		func(v *NativeDecisionBinding) { v.FeatureID = "other" },
		func(v *NativeDecisionBinding) { v.ContractFingerprint = "drift" },
		func(v *NativeDecisionBinding) { v.PolicyFingerprint = "drift" },
		func(v *NativeDecisionBinding) { v.Baseline = "drift" },
		func(v *NativeDecisionBinding) { v.Snapshot = "drift" },
		func(v *NativeDecisionBinding) { v.Target = "drift" },
		func(v *NativeDecisionBinding) { v.PendingActions = 2 },
	} {
		current := testNativeDecisionBinding()
		mutate(&current)
		advanced := false
		if err := ApplyNativeDecision(question, NativeAnswerApproveExactContract, current, &MemoryNativeDecisionStore{}, func(NativeDecisionRecord) error { advanced = true; return nil }); err == nil || !strings.Contains(err.Error(), "stale") || advanced {
			t.Fatalf("stale decision = %v, advanced = %v", err, advanced)
		}
	}
	if err := ApplyNativeDecision(question, "approve", testNativeDecisionBinding(), &MemoryNativeDecisionStore{}, nil); err == nil || !strings.Contains(err.Error(), "exact displayed") {
		t.Fatalf("legacy answer error = %v", err)
	}
}

func TestNativeDecisionRejectsForgedQuestionChoices(t *testing.T) {
	question, err := StrictContractQuestion(testNativeDecisionBinding())
	if err != nil {
		t.Fatal(err)
	}
	question.Choices[0] = "approve"
	if err := ApplyNativeDecision(question, "approve", testNativeDecisionBinding(), &MemoryNativeDecisionStore{}, nil); err == nil || !strings.Contains(err.Error(), "choices are not exact") {
		t.Fatalf("forged choices error = %v", err)
	}
}

func TestNativeDecisionRejectsEmptyTargetBeforeDisplayPersistenceOrAdvance(t *testing.T) {
	binding := testNativeDecisionBinding()
	binding.Target = ""
	if _, err := StrictContractQuestion(binding); err == nil {
		t.Fatal("StrictContractQuestion accepted an empty target")
	}
	if _, err := SideEffectingOperationQuestion(binding, "write record", "rotta workflow advance"); err == nil {
		t.Fatal("SideEffectingOperationQuestion accepted an empty target")
	}

	question, err := StrictContractQuestion(testNativeDecisionBinding())
	if err != nil {
		t.Fatal(err)
	}
	question.Binding.Target = ""
	store := &MemoryNativeDecisionStore{}
	advanced := false
	if err := ApplyNativeDecision(question, NativeAnswerApproveExactContract, testNativeDecisionBinding(), store, func(NativeDecisionRecord) error {
		advanced = true
		return nil
	}); err == nil {
		t.Fatal("ApplyNativeDecision accepted an empty target")
	}
	if len(store.records) != 0 || advanced {
		t.Fatalf("empty target mutated state: records=%d advanced=%v", len(store.records), advanced)
	}
}

func TestNativeDecisionPersistsBeforeSingleConcurrentAdvance(t *testing.T) {
	question, err := StrictContractQuestion(testNativeDecisionBinding())
	if err != nil {
		t.Fatal(err)
	}
	store := &MemoryNativeDecisionStore{}
	var advances int
	var mu sync.Mutex
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- ApplyNativeDecision(question, NativeAnswerApproveExactContract, testNativeDecisionBinding(), store, func(NativeDecisionRecord) error { mu.Lock(); advances++; mu.Unlock(); return nil })
		}()
	}
	wg.Wait()
	close(errs)
	failures := 0
	for err := range errs {
		if err != nil {
			failures++
		}
	}
	if advances != 1 || failures != 1 {
		t.Fatalf("advances/failures = %d/%d, want 1/1", advances, failures)
	}
	if err := ApplyNativeDecision(question, NativeAnswerApproveExactContract, testNativeDecisionBinding(), failingDecisionStore{}, func(NativeDecisionRecord) error { t.Fatal("advanced after persistence failure"); return nil }); !errors.Is(err, errDecisionPersistence) {
		t.Fatalf("persistence failure = %v", err)
	}
}

func TestFileNativeDecisionStorePersistsOneBoundRecord(t *testing.T) {
	question, err := StrictContractQuestion(testNativeDecisionBinding())
	if err != nil {
		t.Fatal(err)
	}
	repositoryRoot := t.TempDir()
	store, err := NewFileNativeDecisionStore(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyNativeDecision(question, NativeAnswerApproveExactContract, testNativeDecisionBinding(), store, nil); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(question.ID))
	path := filepath.Join(repositoryRoot, ".rotta", "current", "native-decisions", fmt.Sprintf("%x.json", digest))
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var record NativeDecisionRecord
	if err := json.Unmarshal(contents, &record); err != nil || record.Format != nativeDecisionFormat || record.Answer != NativeAnswerApproveExactContract || record.Question.Binding != testNativeDecisionBinding() {
		t.Fatalf("persisted record = %#v, %v", record, err)
	}
	if err := ApplyNativeDecision(question, NativeAnswerApproveExactContract, testNativeDecisionBinding(), store, nil); err == nil {
		t.Fatal("existing decision record was overwritten")
	}
}

func TestFileNativeDecisionStoreRejectsSymlinkedStateAndTraversalQuestionID(t *testing.T) {
	repositoryRoot := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(repositoryRoot, ".rotta")); err != nil {
		t.Fatal(err)
	}
	store, err := NewFileNativeDecisionStore(repositoryRoot)
	if err != nil {
		t.Fatal(err)
	}
	question, err := StrictContractQuestion(testNativeDecisionBinding())
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyNativeDecision(question, NativeAnswerApproveExactContract, testNativeDecisionBinding(), store, nil); err == nil || !strings.Contains(err.Error(), "state path") {
		t.Fatalf("symlinked state write = %v, want canonical-state rejection", err)
	}
	if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
		t.Fatalf("symlinked state wrote outside repository: %v, %#v", err, entries)
	}

	cleanRoot := t.TempDir()
	store, err = NewFileNativeDecisionStore(cleanRoot)
	if err != nil {
		t.Fatal(err)
	}
	question.Binding.QuestionID = "../../outside"
	question.ID = question.Binding.QuestionID
	if err := ApplyNativeDecision(question, NativeAnswerApproveExactContract, question.Binding, store, nil); err == nil || !strings.Contains(err.Error(), "must not be a path") {
		t.Fatalf("traversal-shaped question ID = %v, want path rejection", err)
	}
	if _, err := os.Stat(filepath.Join(cleanRoot, "outside")); !os.IsNotExist(err) {
		t.Fatalf("traversal-shaped question ID escaped state root: %v", err)
	}
}

type failingDecisionStore struct{}

var errDecisionPersistence = errors.New("durable store unavailable")

func (failingDecisionStore) PersistNativeDecision(NativeDecisionRecord) error {
	return errDecisionPersistence
}

func testNativeDecisionBinding() NativeDecisionBinding {
	return NativeDecisionBinding{QuestionID: "question-1", SessionID: "session-1", FeatureID: "token-workflow", ContractPath: ".rotta/strict/token-workflow.md", ContractFingerprint: "contract-1", PolicyFingerprint: "policy-1", Baseline: "baseline-1", Snapshot: "snapshot-1", Target: "implementation", PendingActions: 1}
}
