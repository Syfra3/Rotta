package workflow

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type memoryHandoffIndex struct {
	record   HandoffRecord
	readErr  error
	writeErr error
	topic    string
}

func (memory *memoryHandoffIndex) WriteHandoff(topic string, record HandoffRecord) error {
	memory.topic, memory.record = topic, record
	return memory.writeErr
}
func (memory *memoryHandoffIndex) ReadHandoff(topic string) (HandoffRecord, error) {
	memory.topic = topic
	return memory.record, memory.readErr
}

func TestHandoffPersistsAtomicallyAndRecoversMatchingAncoraRecord(t *testing.T) {
	repo, baseline, snapshot := handoffRepository(t)
	memory := &memoryHandoffIndex{}
	index := NewOrchestratorHandoffIndex(repo, memory)
	record := validHandoff(baseline, snapshot)

	result, err := index.Record(record)
	if err != nil || result.Blocked || result.Degraded {
		t.Fatalf("Record() = %#v, %v", result, err)
	}
	path := filepath.Join(repo, ".rotta", "handoffs", "checkout-1.yaml")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != serializeHandoff(record) || memory.topic != "handoff/checkout" {
		t.Fatalf("mirror or topic was not canonical: %q / %q", contents, memory.topic)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mirror permissions = %v, %v", info, err)
	}

	recovered := index.Recover("checkout")
	if recovered.Blocked || recovered.Degraded || recovered.Source != "ancora+mirror" || !sameHandoff(recovered.Record, record) {
		t.Fatalf("Recover() = %#v", recovered)
	}
}

func TestHandoffAncoraFailuresUseOnlyNewestValidMirrorBySequence(t *testing.T) {
	repo, baseline, snapshot := handoffRepository(t)
	memory := &memoryHandoffIndex{writeErr: errors.New("outage")}
	index := NewOrchestratorHandoffIndex(repo, memory)
	first := validHandoff(baseline, snapshot)
	result, err := index.Record(first)
	if err != nil || !result.Degraded || result.Source != "mirror" {
		t.Fatalf("write failure result = %#v, %v", result, err)
	}

	memory.writeErr = nil
	second := first
	second.Sequence, second.HandoffID, second.Status = 2, "checkout/2", "accepted"
	if _, err := index.Record(second); err != nil {
		t.Fatal(err)
	}
	memory.readErr = errors.New("outage")
	recovered := index.Recover("checkout")
	if recovered.Blocked || !recovered.Degraded || recovered.Record.Sequence != 2 || recovered.Source != "mirror" {
		t.Fatalf("read failure recovery = %#v", recovered)
	}
}

func TestHandoffBlocksMismatchConflictTransitionScopeAndSensitiveData(t *testing.T) {
	repo, baseline, snapshot := handoffRepository(t)
	index := NewOrchestratorHandoffIndex(repo, &memoryHandoffIndex{})
	valid := validHandoff(baseline, snapshot)
	if result, err := index.Record(valid); err != nil || result.Blocked {
		t.Fatalf("seed Record() = %#v, %v", result, err)
	}

	tests := []struct {
		name   string
		mutate func(*HandoffRecord)
		want   string
	}{
		{"illegal transition", func(record *HandoffRecord) {
			record.Sequence = 2
			record.HandoffID = "checkout/2"
			record.Status = "ready"
		}, "transition is illegal"},
		{"scope mismatch", func(record *HandoffRecord) {
			record.Sequence = 2
			record.HandoffID = "checkout/2"
			record.Status = "accepted"
			record.Scope = []string{"other/"}
		}, "outside handoff scope"},
		{"sensitive data", func(record *HandoffRecord) {
			record.Sequence = 2
			record.HandoffID = "checkout/2"
			record.Status = "accepted"
			record.Evidence.Result = "secret=not-allowed"
		}, "sensitive"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record := valid
			test.mutate(&record)
			result, err := index.Record(record)
			if err != nil || !result.Blocked || !contains(result.Remediation, test.want) {
				t.Fatalf("Record() = %#v, %v; want %q", result, err, test.want)
			}
		})
	}
}

func TestHandoffRecoveryBlocksAncoraMismatchAndDuplicateMirrorSequence(t *testing.T) {
	repo, baseline, snapshot := handoffRepository(t)
	memory := &memoryHandoffIndex{}
	index := NewOrchestratorHandoffIndex(repo, memory)
	record := validHandoff(baseline, snapshot)
	if _, err := index.Record(record); err != nil {
		t.Fatal(err)
	}
	memory.record.Disposition = "different"
	if result := index.Recover("checkout"); !result.Blocked || !contains(result.Remediation, "disagree") {
		t.Fatalf("mismatch recovery = %#v", result)
	}

	memory.readErr = errors.New("outage")
	duplicate := serializeHandoff(record)
	if err := os.WriteFile(filepath.Join(repo, ".rotta", "handoffs", "duplicate.yaml"), []byte(duplicate), 0o600); err != nil {
		t.Fatal(err)
	}
	if result := index.Recover("checkout"); !result.Blocked || !contains(result.Remediation, "conflicting") {
		t.Fatalf("conflict recovery = %#v", result)
	}
}

func TestHandoffBlocksGitDriftAndMalformedMirror(t *testing.T) {
	repo, baseline, snapshot := handoffRepository(t)
	for _, test := range []struct {
		name   string
		mutate func(*HandoffRecord)
		want   string
	}{
		{"missing baseline", func(record *HandoffRecord) { record.BaselineSHA = "deadbeef" }, "baseline is missing"},
		{"stale snapshot", func(record *HandoffRecord) { record.SnapshotSHA = baseline }, "snapshot does not match"},
	} {
		t.Run(test.name, func(t *testing.T) {
			record := validHandoff(baseline, snapshot)
			test.mutate(&record)
			result, err := NewOrchestratorHandoffIndex(repo, nil).Record(record)
			if err != nil || !result.Blocked || !contains(result.Remediation, test.want) {
				t.Fatalf("Record() = %#v, %v; want %q", result, err, test.want)
			}
		})
	}

	if err := os.MkdirAll(filepath.Join(repo, ".rotta", "handoffs"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".rotta", "handoffs", "bad.yaml"), []byte("not: a handoff\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result := NewOrchestratorHandoffIndex(repo, &memoryHandoffIndex{readErr: errors.New("outage")}).Recover("checkout")
	if !result.Blocked || !contains(result.Remediation, "malformed handoff mirror") {
		t.Fatalf("Recover() = %#v", result)
	}
}

func TestHandoffRecoveryBlocksStagedAndUnstagedWorkspaceDrift(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(t *testing.T, repo string)
		want   string
	}{
		{"staged", func(t *testing.T, repo string) {
			t.Helper()
			path := filepath.Join(repo, "internal", "checkout", "checkout.go")
			if err := os.WriteFile(path, []byte("package checkout\n// staged drift\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := gitSubmissionOutput(repo, "add", path); err != nil {
				t.Fatal(err)
			}
		}, "staged index drift"},
		{"unstaged", func(t *testing.T, repo string) {
			t.Helper()
			path := filepath.Join(repo, "internal", "checkout", "checkout.go")
			if err := os.WriteFile(path, []byte("package checkout\n// unstaged drift\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, "unstaged worktree drift"},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo, baseline, snapshot := handoffRepository(t)
			memory := &memoryHandoffIndex{}
			index := NewOrchestratorHandoffIndex(repo, memory)
			if result, err := index.Record(validHandoff(baseline, snapshot)); err != nil || result.Blocked {
				t.Fatalf("seed Record() = %#v, %v", result, err)
			}
			test.mutate(t, repo)
			if result := index.Recover("checkout"); !result.Blocked || !contains(result.Remediation, test.want) || !contains(result.Remediation, "stash or discard") {
				t.Fatalf("Recover() = %#v", result)
			}
		})
	}
}

func TestHandoffRecoveryBlocksUntrackedDriftButAllowsLocalMirrors(t *testing.T) {
	for _, test := range []struct {
		name    string
		mutate  func(t *testing.T, repo string)
		blocked bool
	}{
		{"untracked source", func(t *testing.T, repo string) {
			t.Helper()
			if err := os.WriteFile(filepath.Join(repo, "internal", "checkout", "new.go"), []byte("package checkout\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}, true},
		{"local handoff mirror", func(t *testing.T, repo string) { t.Helper() }, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo, baseline, snapshot := handoffRepository(t)
			memory := &memoryHandoffIndex{}
			index := NewOrchestratorHandoffIndex(repo, memory)
			if result, err := index.Record(validHandoff(baseline, snapshot)); err != nil || result.Blocked {
				t.Fatalf("seed Record() = %#v, %v", result, err)
			}
			test.mutate(t, repo)
			result := index.Recover("checkout")
			if result.Blocked != test.blocked {
				t.Fatalf("Recover() = %#v; blocked = %v, want %v", result, result.Blocked, test.blocked)
			}
			if test.blocked && (!contains(result.Remediation, "untracked path") || !contains(result.Remediation, "add, stash, or remove")) {
				t.Fatalf("untracked recovery remediation = %q", result.Remediation)
			}
		})
	}
}

func TestHandoffRecoveryBlocksReadableAncoraWithoutLocalMirror(t *testing.T) {
	repo, _, _ := handoffRepository(t)
	result := NewOrchestratorHandoffIndex(repo, &memoryHandoffIndex{}).Recover("checkout")
	if !result.Blocked || !contains(result.Remediation, "no local handoff mirror exists") || !contains(result.Remediation, "restore") || !contains(result.Remediation, "reconcile") {
		t.Fatalf("Recover() = %#v", result)
	}
}

func TestHandoffRecoveryAcceptsLegalHistoryWithEarlierSnapshot(t *testing.T) {
	repo, baseline, firstSnapshot := handoffRepository(t)
	memory := &memoryHandoffIndex{}
	index := NewOrchestratorHandoffIndex(repo, memory)
	first := validHandoff(baseline, firstSnapshot)
	if result, err := index.Record(first); err != nil || result.Blocked {
		t.Fatalf("first Record() = %#v, %v", result, err)
	}
	path := filepath.Join(repo, "internal", "checkout", "checkout.go")
	if err := os.WriteFile(path, []byte("package checkout\n// second snapshot\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := gitSubmissionOutput(repo, "add", path); err != nil {
		t.Fatal(err)
	}
	if _, err := gitSubmissionOutput(repo, "commit", "-m", "second snapshot"); err != nil {
		t.Fatal(err)
	}
	secondSnapshot, err := gitSubmissionOutput(repo, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	second := first
	second.Sequence, second.HandoffID, second.Status, second.SnapshotSHA = 2, "checkout/2", "accepted", secondSnapshot
	if result, err := index.Record(second); err != nil || result.Blocked {
		t.Fatalf("second Record() = %#v, %v", result, err)
	}
	if result := index.Recover("checkout"); result.Blocked || result.Record.Sequence != 2 || result.Record.SnapshotSHA != secondSnapshot {
		t.Fatalf("Recover() = %#v", result)
	}
}

func TestHandoffRecoveryBlocksIncompleteAndIllegalHistory(t *testing.T) {
	for _, test := range []struct {
		name   string
		second func(HandoffRecord) HandoffRecord
		want   string
	}{
		{"sequence gap", func(record HandoffRecord) HandoffRecord {
			record.Sequence, record.HandoffID, record.Status = 3, "checkout/3", "accepted"
			return record
		}, "sequence is not contiguous"},
		{"illegal transition", func(record HandoffRecord) HandoffRecord {
			record.Sequence, record.HandoffID, record.Status = 2, "checkout/2", "ready"
			return record
		}, "status transition is illegal"},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo, baseline, snapshot := handoffRepository(t)
			index := NewOrchestratorHandoffIndex(repo, &memoryHandoffIndex{readErr: errors.New("outage")})
			first := validHandoff(baseline, snapshot)
			if result, err := index.Record(first); err != nil || result.Blocked {
				t.Fatalf("seed Record() = %#v, %v", result, err)
			}
			second := test.second(first)
			path := filepath.Join(repo, ".rotta", "handoffs", "checkout-"+strconv.FormatUint(second.Sequence, 10)+".yaml")
			if err := os.WriteFile(path, []byte(serializeHandoff(second)), 0o600); err != nil {
				t.Fatal(err)
			}
			if result := index.Recover("checkout"); !result.Blocked || !contains(result.Remediation, test.want) || !contains(result.Remediation, "contiguous legal task history") {
				t.Fatalf("Recover() = %#v", result)
			}
		})
	}
}

func TestHandoffRecoveryRequiresHistoryBeginningAtSequenceOne(t *testing.T) {
	repo, baseline, snapshot := handoffRepository(t)
	index := NewOrchestratorHandoffIndex(repo, &memoryHandoffIndex{readErr: errors.New("outage")})
	record := validHandoff(baseline, snapshot)
	record.Sequence, record.HandoffID, record.Status = 2, "checkout/2", "accepted"
	if err := os.MkdirAll(filepath.Join(repo, ".rotta", "handoffs"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".rotta", "handoffs", "checkout-2.yaml"), []byte(serializeHandoff(record)), 0o600); err != nil {
		t.Fatal(err)
	}
	if result := index.Recover("checkout"); !result.Blocked || !contains(result.Remediation, "begin at sequence 1") || !contains(result.Remediation, "contiguous legal task history") {
		t.Fatalf("Recover() = %#v", result)
	}
}

func TestHandoffBlocksCredentialBearingPayloadsBeforePersistence(t *testing.T) {
	for _, payload := range []string{"token=not-allowed", "Bearer not-allowed", "Authorization: Basic not-allowed"} {
		t.Run(payload, func(t *testing.T) {
			repo, baseline, snapshot := handoffRepository(t)
			index := NewOrchestratorHandoffIndex(repo, nil)
			first := validHandoff(baseline, snapshot)
			if result, err := index.Record(first); err != nil || result.Blocked {
				t.Fatalf("seed Record() = %#v, %v", result, err)
			}
			second := first
			second.Sequence, second.HandoffID, second.Status, second.Evidence.Result = 2, "checkout/2", "accepted", payload
			if result, err := index.Record(second); err != nil || !result.Blocked || !contains(result.Remediation, "sensitive") {
				t.Fatalf("Record() = %#v, %v", result, err)
			}
			if _, err := os.Stat(filepath.Join(repo, ".rotta", "handoffs", "checkout-2.yaml")); !os.IsNotExist(err) {
				t.Fatalf("credential-bearing record was persisted: %v", err)
			}
		})
	}
}

func TestHandoffRecordBlocksInvalidExistingHistoryBeforePersistence(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*HandoffRecord)
		want   string
	}{
		{"structurally malformed", func(record *HandoffRecord) { record.Status = "not-a-status" }, "status or priority is invalid"},
		{"Git-invalid", func(record *HandoffRecord) { record.BaselineSHA = "deadbeef" }, "baseline is missing"},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo, baseline, snapshot := handoffRepository(t)
			memory := &memoryHandoffIndex{}
			index := NewOrchestratorHandoffIndex(repo, memory)
			first := validHandoff(baseline, snapshot)
			if result, err := index.Record(first); err != nil || result.Blocked {
				t.Fatalf("seed Record() = %#v, %v", result, err)
			}
			forged := first
			test.mutate(&forged)
			if err := os.WriteFile(filepath.Join(repo, ".rotta", "handoffs", "checkout-1.yaml"), []byte(serializeHandoff(forged)), 0o600); err != nil {
				t.Fatal(err)
			}
			next := first
			next.Sequence, next.HandoffID, next.Status = 2, "checkout/2", "accepted"
			result, err := index.Record(next)
			if err != nil || !result.Blocked || !contains(result.Remediation, test.want) {
				t.Fatalf("Record() = %#v, %v; want %q", result, err, test.want)
			}
			if _, err := os.Stat(filepath.Join(repo, ".rotta", "handoffs", "checkout-2.yaml")); !os.IsNotExist(err) {
				t.Fatalf("successor mirror was persisted: %v", err)
			}
			if memory.record.Sequence != 1 {
				t.Fatalf("successor was persisted to Ancora: %#v", memory.record)
			}
		})
	}
}

func TestHandoffMirrorPathRequiresCanonicalTaskSequenceFilename(t *testing.T) {
	for _, test := range []struct {
		path string
		want bool
	}{
		{".rotta/handoffs/checkout-1.yaml", true},
		{".rotta/handoffs/release-2026-2.yaml", true},
		{".rotta/handoffs/checkout-01.yaml", false},
		{".rotta/handoffs/checkout-0.yaml", false},
		{".rotta/handoffs/checkout-18446744073709551616.yaml", false},
		{".rotta/handoffs/nested/checkout-2.yaml", false},
		{".rotta/handoffs/../handoffs/checkout-2.yaml", false},
		{".rotta/handoffs/checkout-2.yaml/extra", false},
		{".rotta/handoffs/Checkout-2.yaml", false},
	} {
		t.Run(test.path, func(t *testing.T) {
			if got := isHandoffMirrorPath(test.path); got != test.want {
				t.Fatalf("isHandoffMirrorPath(%q) = %v, want %v", test.path, got, test.want)
			}
		})
	}
}

func TestHandoffRecoveryBlocksNestedUntrackedMirrorPath(t *testing.T) {
	repo, baseline, snapshot := handoffRepository(t)
	index := NewOrchestratorHandoffIndex(repo, &memoryHandoffIndex{})
	if result, err := index.Record(validHandoff(baseline, snapshot)); err != nil || result.Blocked {
		t.Fatalf("seed Record() = %#v, %v", result, err)
	}
	nested := filepath.Join(repo, ".rotta", "handoffs", "nested", "checkout-2.yaml")
	if err := os.MkdirAll(filepath.Dir(nested), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nested, []byte("not a canonical mirror\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if result := index.Recover("checkout"); !result.Blocked || !contains(result.Remediation, "untracked path") {
		t.Fatalf("Recover() = %#v", result)
	}
}

func TestHandoffRouteAllowsOnlyBoundedOrchestratorMediatedPaths(t *testing.T) {
	for _, test := range []struct {
		name  string
		route [][2]string
		want  string
	}{
		{"Fast", [][2]string{{"rotta-orchestrator", "rotta-impl"}, {"rotta-impl", "rotta-review"}, {"rotta-review", "rotta-orchestrator"}}, ""},
		{"deep with optional architect", [][2]string{{"rotta-orchestrator", "rotta-impl"}, {"rotta-impl", "rotta-cleaner"}, {"rotta-cleaner", "rotta-review"}, {"rotta-review", "rotta-orchestrator"}}, ""},
		{"deep with architect", [][2]string{{"rotta-orchestrator", "rotta-impl"}, {"rotta-impl", "rotta-cleaner"}, {"rotta-cleaner", "rotta-architect"}, {"rotta-architect", "rotta-review"}, {"rotta-review", "rotta-orchestrator"}}, ""},
		{"isolated architecture remediation", [][2]string{{"rotta-orchestrator", "rotta-impl"}, {"rotta-impl", "rotta-cleaner"}, {"rotta-cleaner", "rotta-architect"}, {"rotta-architect", "rotta-impl"}, {"rotta-impl", "rotta-review"}}, ""},
		{"reviewer escalation through cleaner", [][2]string{{"rotta-orchestrator", "rotta-impl"}, {"rotta-impl", "rotta-review"}, {"rotta-review", "rotta-orchestrator"}, {"rotta-orchestrator", "rotta-cleaner"}, {"rotta-cleaner", "rotta-review"}, {"rotta-review", "rotta-orchestrator"}}, ""},
		{"reviewer escalation through architect", [][2]string{{"rotta-orchestrator", "rotta-impl"}, {"rotta-impl", "rotta-review"}, {"rotta-review", "rotta-orchestrator"}, {"rotta-orchestrator", "rotta-architect"}, {"rotta-architect", "rotta-review"}, {"rotta-review", "rotta-orchestrator"}}, ""},
		{"direct review to cleaner", [][2]string{{"rotta-orchestrator", "rotta-impl"}, {"rotta-impl", "rotta-review"}, {"rotta-review", "rotta-cleaner"}}, "illegal role transition"},
		{"direct review to architect", [][2]string{{"rotta-orchestrator", "rotta-impl"}, {"rotta-impl", "rotta-review"}, {"rotta-review", "rotta-architect"}}, "illegal role transition"},
		{"quality role self-scheduling", [][2]string{{"rotta-orchestrator", "rotta-impl"}, {"rotta-impl", "rotta-cleaner"}, {"rotta-cleaner", "rotta-cleaner"}}, "illegal role transition"},
		{"repeated reviewer escalation", [][2]string{{"rotta-orchestrator", "rotta-impl"}, {"rotta-impl", "rotta-review"}, {"rotta-review", "rotta-orchestrator"}, {"rotta-orchestrator", "rotta-cleaner"}, {"rotta-cleaner", "rotta-review"}, {"rotta-review", "rotta-orchestrator"}, {"rotta-orchestrator", "rotta-architect"}}, "exactly one fresh final review"},
		{"recursive cleaner after remediation", [][2]string{{"rotta-orchestrator", "rotta-impl"}, {"rotta-impl", "rotta-cleaner"}, {"rotta-cleaner", "rotta-architect"}, {"rotta-architect", "rotta-impl"}, {"rotta-impl", "rotta-cleaner"}}, "exactly one fresh final review"},
	} {
		t.Run(test.name, func(t *testing.T) {
			records := make([]HandoffRecord, len(test.route))
			for i, edge := range test.route {
				records[i] = HandoffRecord{From: edge[0], To: edge[1]}
			}
			err := validateHandoffRoute(records)
			if test.want == "" && err != nil {
				t.Fatalf("validateHandoffRoute() = %v", err)
			}
			if test.want != "" && (err == nil || !contains(err.Error(), test.want)) {
				t.Fatalf("validateHandoffRoute() = %v; want %q", err, test.want)
			}
		})
	}
}

func TestHandoffSequenceRepresentsCompletedDeepRoute(t *testing.T) {
	edges := [][2]string{
		{"rotta-orchestrator", "rotta-impl"},
		{"rotta-impl", "rotta-cleaner"},
		{"rotta-cleaner", "rotta-architect"},
		{"rotta-architect", "rotta-review"},
		{"rotta-review", "rotta-orchestrator"},
	}
	var records []HandoffRecord
	for _, edge := range edges {
		for _, status := range []string{"ready", "accepted", "completed"} {
			records = append(records, HandoffRecord{Sequence: uint64(len(records) + 1), From: edge[0], To: edge[1], Status: status})
		}
	}
	if err := validateHandoffSequence(records); err != nil {
		t.Fatalf("validateHandoffSequence() = %v", err)
	}
}

func TestHandoffDeepRoleRequiresTriggerAndExpectedEvidence(t *testing.T) {
	repo, baseline, snapshot := handoffRepository(t)
	index := NewOrchestratorHandoffIndex(repo, nil)
	record := validHandoff(baseline, snapshot)
	record.From, record.To = "rotta-impl", "rotta-cleaner"
	if err := index.validateStructural(record); err == nil || !contains(err.Error(), "missing its trigger or expected evidence") {
		t.Fatalf("validateStructural() = %v", err)
	}
	record.DeepReviewTrigger = deepTriggerReviewEvidence
	record.ExpectedEvidence = "targeted changed-code verification"
	if err := index.validateStructural(record); err != nil {
		t.Fatalf("validateStructural() with deep evidence = %v", err)
	}
	parsed, err := parseHandoff(serializeHandoff(record))
	if err != nil || parsed.DeepReviewTrigger != record.DeepReviewTrigger || parsed.ExpectedEvidence != record.ExpectedEvidence {
		t.Fatalf("deep handoff round-trip = %#v, %v", parsed, err)
	}
}

func TestHandoffDeepTriggerUsesSemanticAllowlist(t *testing.T) {
	repo, baseline, snapshot := handoffRepository(t)
	index := NewOrchestratorHandoffIndex(repo, nil)
	for _, trigger := range []string{
		deepTriggerStrictClassification,
		deepTriggerUserRequest,
		deepTriggerRepositoryPolicy,
		deepTriggerReviewEvidence,
	} {
		t.Run(trigger, func(t *testing.T) {
			record := validHandoff(baseline, snapshot)
			record.From, record.To = "rotta-impl", "rotta-cleaner"
			record.DeepReviewTrigger, record.ExpectedEvidence = trigger, "targeted changed-code verification"
			if err := index.validateStructural(record); err != nil {
				t.Fatalf("validateStructural() = %v", err)
			}
		})
	}
	for _, trigger := range []string{"high risk", "because deep review", "concrete evidence"} {
		t.Run("reject "+trigger, func(t *testing.T) {
			record := validHandoff(baseline, snapshot)
			record.From, record.To = "rotta-impl", "rotta-cleaner"
			record.DeepReviewTrigger, record.ExpectedEvidence = trigger, "targeted changed-code verification"
			if err := index.validateStructural(record); err == nil || !contains(err.Error(), "must be Strict classification") {
				t.Fatalf("validateStructural() = %v", err)
			}
		})
	}
}

func TestHandoffRouteRequiresFreshFinalReviewAfterQualityWork(t *testing.T) {
	for _, test := range []struct {
		name  string
		route [][2]string
		want  string
	}{
		{"cleaner route has one final review", [][2]string{{"rotta-orchestrator", "rotta-impl"}, {"rotta-impl", "rotta-cleaner"}, {"rotta-cleaner", "rotta-review"}, {"rotta-review", "rotta-orchestrator"}}, ""},
		{"architect remediation has one fresh final review", [][2]string{{"rotta-orchestrator", "rotta-impl"}, {"rotta-impl", "rotta-cleaner"}, {"rotta-cleaner", "rotta-architect"}, {"rotta-architect", "rotta-impl"}, {"rotta-impl", "rotta-review"}, {"rotta-review", "rotta-orchestrator"}}, ""},
		{"reviewer escalation replaces stale review with fresh final review", [][2]string{{"rotta-orchestrator", "rotta-impl"}, {"rotta-impl", "rotta-review"}, {"rotta-review", "rotta-orchestrator"}, {"rotta-orchestrator", "rotta-cleaner"}, {"rotta-cleaner", "rotta-architect"}, {"rotta-architect", "rotta-impl"}, {"rotta-impl", "rotta-review"}, {"rotta-review", "rotta-orchestrator"}}, ""},
		{"duplicate final review after escalation", [][2]string{{"rotta-orchestrator", "rotta-impl"}, {"rotta-impl", "rotta-review"}, {"rotta-review", "rotta-orchestrator"}, {"rotta-orchestrator", "rotta-cleaner"}, {"rotta-cleaner", "rotta-review"}, {"rotta-review", "rotta-orchestrator"}, {"rotta-orchestrator", "rotta-impl"}, {"rotta-impl", "rotta-review"}}, "exactly one fresh final review"},
	} {
		t.Run(test.name, func(t *testing.T) {
			records := make([]HandoffRecord, len(test.route))
			for i, edge := range test.route {
				records[i] = HandoffRecord{From: edge[0], To: edge[1]}
			}
			err := validateHandoffRoute(records)
			if test.want == "" && err != nil {
				t.Fatalf("validateHandoffRoute() = %v", err)
			}
			if test.want != "" && (err == nil || !contains(err.Error(), test.want)) {
				t.Fatalf("validateHandoffRoute() = %v; want %q", err, test.want)
			}
		})
	}
}

func validHandoff(baseline, snapshot string) HandoffRecord {
	return HandoffRecord{Format: handoffFormat, HandoffID: "checkout/1", Sequence: 1, From: "rotta-orchestrator", To: "rotta-impl", Status: "ready", Priority: "normal", BaselineSHA: baseline, SnapshotSHA: snapshot, Scope: []string{"internal/checkout/"}, Evidence: HandoffEvidence{Commands: []string{"go test ./internal/checkout"}, Result: "passed", RecordedAt: time.Date(2026, time.August, 5, 0, 0, 0, 0, time.UTC)}, Disposition: "ready_for_implementation"}
}

func handoffRepository(t *testing.T) (string, string, string) {
	t.Helper()
	repo := t.TempDir()
	for _, args := range [][]string{{"init"}, {"config", "user.email", "test@example.com"}, {"config", "user.name", "Test"}} {
		if _, err := gitSubmissionOutput(repo, args...); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(repo, "internal", "checkout", "checkout.go")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package checkout\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := gitSubmissionOutput(repo, "add", "."); err != nil {
		t.Fatal(err)
	}
	if _, err := gitSubmissionOutput(repo, "commit", "-m", "baseline"); err != nil {
		t.Fatal(err)
	}
	baseline, err := gitSubmissionOutput(repo, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package checkout\n// changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := gitSubmissionOutput(repo, "add", "."); err != nil {
		t.Fatal(err)
	}
	if _, err := gitSubmissionOutput(repo, "commit", "-m", "snapshot"); err != nil {
		t.Fatal(err)
	}
	snapshot, err := gitSubmissionOutput(repo, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	return repo, baseline, snapshot
}

func contains(value, want string) bool { return strings.Contains(value, want) }
