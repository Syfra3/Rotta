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
)

const nativeDecisionFormat = "rotta.native-decision/v1"

const nativeDecisionStateDirectory = ".rotta/current/native-decisions"

const (
	NativeAnswerApproveExactContract = "Approve exact contract"
	NativeAnswerRequestChanges       = "Request changes"
	NativeAnswerCancel               = "Cancel"
)

// NativeDecisionBinding is the complete identity that has to survive from a
// native Question display until its selected answer advances any workflow
// state. It deliberately contains no conversational approval text.
type NativeDecisionBinding struct {
	QuestionID          string
	SessionID           string
	FeatureID           string
	ContractPath        string
	ContractFingerprint string
	PolicyFingerprint   string
	Baseline            string
	Snapshot            string
	Target              string
	PendingActions      int
}

// NativeQuestion is a presentation-neutral representation of an OpenCode
// native Question. The host owns display and answer collection; workflow owns
// the exact choices and binding validation.
type NativeQuestion struct {
	ID        string
	Prompt    string
	Choices   []string
	Binding   NativeDecisionBinding
	Operation bool
}

type NativeDecisionRecord struct {
	Format   string
	Question NativeQuestion
	Answer   string
}

// NativeDecisionStore persists the selected native Question answer before an
// advancing callback runs. A failed persistence attempt must leave state still.
type NativeDecisionStore interface {
	PersistNativeDecision(NativeDecisionRecord) error
}

// StrictContractQuestion presents exactly the one approved Strict decision.
func StrictContractQuestion(binding NativeDecisionBinding) (NativeQuestion, error) {
	if err := validateNativeBinding(binding, true); err != nil {
		return NativeQuestion{}, err
	}
	return NativeQuestion{
		ID:      binding.QuestionID,
		Prompt:  fmt.Sprintf("Strict contract decision: %s; feature: %s; baseline: %s; contract fingerprint: %s", binding.ContractPath, binding.FeatureID, binding.Baseline, binding.ContractFingerprint),
		Choices: []string{NativeAnswerApproveExactContract, NativeAnswerRequestChanges, NativeAnswerCancel},
		Binding: binding,
	}, nil
}

// SideEffectingOperationQuestion keeps each exact operation separate from the
// contract question and from installer TUI setup confirmation.
func SideEffectingOperationQuestion(binding NativeDecisionBinding, operation, command string) (NativeQuestion, error) {
	if err := validateNativeBinding(binding, false); err != nil {
		return NativeQuestion{}, err
	}
	if operation == "" || command == "" {
		return NativeQuestion{}, errors.New("native operation question requires an exact operation and intended command/effect")
	}
	return NativeQuestion{
		ID:        binding.QuestionID,
		Prompt:    fmt.Sprintf("Operation: %s; target: %s; path: %s; intended command/effect: %s", operation, binding.Target, binding.ContractPath, command),
		Choices:   []string{"Approve exact operation", NativeAnswerCancel},
		Binding:   binding,
		Operation: true,
	}, nil
}

// ApplyNativeDecision rejects stale, dismissed, custom, and legacy text
// answers. It persists the bound selection before invoking advance.
func ApplyNativeDecision(question NativeQuestion, answer string, current NativeDecisionBinding, store NativeDecisionStore, advance func(NativeDecisionRecord) error) error {
	if err := validateNativeBinding(question.Binding, question.Operation == false); err != nil {
		return err
	}
	if err := validateNativeQuestion(question); err != nil {
		return err
	}
	if !sameNativeBinding(question.Binding, current) {
		return errors.New("native Question binding is stale; ask a new Question")
	}
	if !nativeQuestionChoice(question, answer) {
		return errors.New("native Question answer is not one of the exact displayed choices")
	}
	record := NativeDecisionRecord{Format: nativeDecisionFormat, Question: question, Answer: answer}
	if store == nil {
		return errors.New("native Question decision persistence is required before advancing")
	}
	if err := store.PersistNativeDecision(record); err != nil {
		return fmt.Errorf("persist native Question decision before advancing: %w", err)
	}
	if advance != nil {
		return advance(record)
	}
	return nil
}

func validateNativeBinding(binding NativeDecisionBinding, strict bool) error {
	if binding.QuestionID == "" || binding.SessionID == "" || binding.FeatureID == "" || binding.ContractFingerprint == "" || binding.PolicyFingerprint == "" || binding.Baseline == "" || binding.Snapshot == "" || binding.Target == "" || binding.PendingActions != 1 {
		return errors.New("native Question requires one complete binding with exactly one pending action")
	}
	if !canonicalNativeDecisionID(binding.QuestionID) {
		return errors.New("native Question ID must not be a path")
	}
	if strict && binding.ContractPath == "" {
		return errors.New("Strict contract Question requires the contract path")
	}
	return nil
}

func canonicalNativeDecisionID(value string) bool {
	return len(value) <= 512 && filepath.Base(value) == value && value != "." && !strings.ContainsAny(value, `/\\`)
}

func sameNativeBinding(left, right NativeDecisionBinding) bool {
	return left.QuestionID == right.QuestionID && left.SessionID == right.SessionID && left.FeatureID == right.FeatureID && left.ContractPath == right.ContractPath && left.ContractFingerprint == right.ContractFingerprint && left.PolicyFingerprint == right.PolicyFingerprint && left.Baseline == right.Baseline && left.Snapshot == right.Snapshot && left.Target == right.Target && right.PendingActions == 1
}

func nativeQuestionChoice(question NativeQuestion, answer string) bool {
	for _, choice := range question.Choices {
		if answer == choice {
			return true
		}
	}
	return false
}

func validateNativeQuestion(question NativeQuestion) error {
	if question.ID != question.Binding.QuestionID || question.Prompt == "" {
		return errors.New("native Question identity does not match its binding")
	}
	if question.Operation {
		if len(question.Choices) != 2 || question.Choices[0] != "Approve exact operation" || question.Choices[1] != NativeAnswerCancel {
			return errors.New("native operation Question choices are not exact")
		}
		return nil
	}
	if len(question.Choices) != 3 || question.Choices[0] != NativeAnswerApproveExactContract || question.Choices[1] != NativeAnswerRequestChanges || question.Choices[2] != NativeAnswerCancel {
		return errors.New("Strict contract Question choices are not exact")
	}
	return nil
}

// MemoryNativeDecisionStore is a concurrency-safe test and host adapter. It
// refuses to overwrite an answer for the same native Question.
type MemoryNativeDecisionStore struct {
	mu      sync.Mutex
	records map[string]NativeDecisionRecord
}

// FileNativeDecisionStore durably records decisions beneath the canonical
// repository state directory. It accepts no caller-selected decision path.
type FileNativeDecisionStore struct {
	repositoryRoot string
}

// NewFileNativeDecisionStore binds the store to one canonical repository root.
func NewFileNativeDecisionStore(repositoryRoot string) (FileNativeDecisionStore, error) {
	if repositoryRoot == "" {
		return FileNativeDecisionStore{}, errors.New("native Question repository root is required")
	}
	absolute, err := filepath.Abs(repositoryRoot)
	if err != nil {
		return FileNativeDecisionStore{}, fmt.Errorf("resolve native Question repository root: %w", err)
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return FileNativeDecisionStore{}, fmt.Errorf("resolve native Question repository root: %w", err)
	}
	info, err := os.Lstat(canonical)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return FileNativeDecisionStore{}, errors.New("native Question repository root must be a canonical directory")
	}
	return FileNativeDecisionStore{repositoryRoot: canonical}, nil
}

func (store FileNativeDecisionStore) PersistNativeDecision(record NativeDecisionRecord) error {
	if store.repositoryRoot == "" {
		return errors.New("native Question decision store requires a canonical repository root")
	}
	if err := validateNativeDecisionRecord(record); err != nil {
		return err
	}
	stateRoot, err := canonicalNativeDecisionStateRoot(store.repositoryRoot)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode native Question decision: %w", err)
	}
	temporary, err := os.CreateTemp(stateRoot, ".native-decision-*")
	if err != nil {
		return fmt.Errorf("create native Question decision: %w", err)
	}
	defer os.Remove(temporary.Name())
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(payload); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	digest := sha256.Sum256([]byte(record.Question.ID))
	decisionPath := filepath.Join(stateRoot, fmt.Sprintf("%x.json", digest))
	if err := os.Link(temporary.Name(), decisionPath); err != nil {
		return fmt.Errorf("persist native Question decision: %w", err)
	}
	return nil
}

func validateNativeDecisionRecord(record NativeDecisionRecord) error {
	if record.Format != nativeDecisionFormat {
		return errors.New("native Question decision record format is invalid")
	}
	if err := validateNativeBinding(record.Question.Binding, !record.Question.Operation); err != nil {
		return err
	}
	if err := validateNativeQuestion(record.Question); err != nil {
		return err
	}
	if !nativeQuestionChoice(record.Question, record.Answer) {
		return errors.New("native Question decision answer is not one of the exact displayed choices")
	}
	return nil
}

func canonicalNativeDecisionStateRoot(repositoryRoot string) (string, error) {
	canonicalRoot, err := filepath.EvalSymlinks(repositoryRoot)
	if err != nil || canonicalRoot != repositoryRoot {
		return "", errors.New("native Question repository root is no longer canonical")
	}
	rootInfo, err := os.Lstat(canonicalRoot)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("native Question repository root is not a directory")
	}
	current := canonicalRoot
	for _, component := range []string{".rotta", "current", "native-decisions"} {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			if err := os.Mkdir(current, 0o700); err != nil {
				return "", fmt.Errorf("create native Question decision directory: %w", err)
			}
			continue
		}
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("native Question decision state path must be a canonical directory")
		}
	}
	return current, nil
}

func (store *MemoryNativeDecisionStore) PersistNativeDecision(record NativeDecisionRecord) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.records == nil {
		store.records = map[string]NativeDecisionRecord{}
	}
	if _, exists := store.records[record.Question.ID]; exists {
		return errors.New("native Question decision already persisted")
	}
	store.records[record.Question.ID] = record
	return nil
}
