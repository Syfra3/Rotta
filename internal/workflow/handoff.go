package workflow

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const handoffFormat = "rotta.handoff/v1"
const maxHandoffBytes = 16 * 1024

const (
	deepTriggerStrictClassification = "strict classification"
	deepTriggerUserRequest          = "explicit user request"
	deepTriggerRepositoryPolicy     = "repository policy"
	deepTriggerReviewEvidence       = "concrete review evidence"
)

var handoffTaskID = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var handoffMirrorPath = regexp.MustCompile(`^\.rotta/handoffs/([a-z0-9]+(?:-[a-z0-9]+)*)-([1-9][0-9]*)\.yaml$`)
var sensitiveHandoffData = regexp.MustCompile(`(?i)\b(?:password|secret|credential|token|bearer|authorization|private)\b|api[_-]?key|raw log|-----begin`)

// AncoraHandoffIndex is deliberately a narrow, injected boundary.  It stores
// routing metadata only; callers must still validate the workspace and Git.
type AncoraHandoffIndex interface {
	WriteHandoff(topic string, record HandoffRecord) error
	ReadHandoff(topic string) (HandoffRecord, error)
}

type HandoffEvidence struct {
	Commands   []string
	Result     string
	RecordedAt time.Time
}

// HandoffRecord is compact recovery metadata, never workflow authority.
type HandoffRecord struct {
	Format            string
	HandoffID         string
	Sequence          uint64
	From              string
	To                string
	Status            string
	Priority          string
	BaselineSHA       string
	SnapshotSHA       string
	Scope             []string
	StrictContractRef string
	GherkinRef        string
	DeepReviewTrigger string
	ExpectedEvidence  string
	Evidence          HandoffEvidence
	Disposition       string
}

type HandoffRecovery struct {
	Record      HandoffRecord
	Source      string
	Degraded    bool
	Blocked     bool
	Remediation string
}

// OrchestratorHandoffIndex is the only writer. Roles return evidence to the
// orchestrator; they cannot self-claim accepted, blocked, or completed state.
type OrchestratorHandoffIndex struct {
	repoRoot string
	ancora   AncoraHandoffIndex
}

func NewOrchestratorHandoffIndex(repoRoot string, ancora AncoraHandoffIndex) *OrchestratorHandoffIndex {
	return &OrchestratorHandoffIndex{repoRoot: repoRoot, ancora: ancora}
}

func (index *OrchestratorHandoffIndex) Record(record HandoffRecord) (HandoffRecovery, error) {
	if err := index.validate(record); err != nil {
		return blockedHandoff(err), nil
	}
	taskID, err := handoffTask(record)
	if err != nil {
		return blockedHandoff(err), nil
	}
	previous, err := index.mirrors(taskID)
	if err != nil {
		return blockedHandoff(err), nil
	}
	if err := index.validateHandoffHistory(previous); err != nil {
		return blockedHandoff(err), nil
	}
	if err := validateNextHandoff(previous, record); err != nil {
		return blockedHandoff(err), nil
	}
	if err := index.writeMirror(taskID, record); err != nil {
		return HandoffRecovery{}, err
	}
	result := HandoffRecovery{Record: record, Source: "mirror+ancora"}
	if index.ancora == nil {
		result.Source, result.Degraded, result.Remediation = "mirror", true, "Ancora handoff index is unavailable; restore it before relying on a resumed handoff"
		return result, nil
	}
	if err := index.ancora.WriteHandoff(handoffTopic(taskID), record); err != nil {
		result.Source, result.Degraded, result.Remediation = "mirror", true, "Ancora handoff write failed; retry Ancora before relying on a resumed handoff"
	}
	return result, nil
}

// Recover validates a matching Ancora/mirror pair. On an Ancora outage only a
// valid mirror selected by sequence (not timestamp) may be used.
func (index *OrchestratorHandoffIndex) Recover(taskID string) HandoffRecovery {
	if !canonicalWorkflowHandoffTaskID(taskID) {
		return blockedHandoff(fmt.Errorf("invalid handoff task ID; use a lowercase hyphenated task ID of at most %d bytes", maxCanonicalHandoffIDBytes))
	}
	mirrors, err := index.mirrors(taskID)
	if err != nil {
		return blockedHandoff(err)
	}
	if len(mirrors) == 0 {
		return blockedHandoff(errors.New("no local handoff mirror exists; restore the local mirror and reconcile it with Ancora before continuing"))
	}
	if index.ancora == nil {
		return index.recoverMirror(taskID, mirrors, "Ancora is unavailable; the Git-validated mirror may continue and Ancora can be restored separately")
	}
	remote, err := index.ancora.ReadHandoff(handoffTopic(taskID))
	if err != nil {
		return index.recoverMirror(taskID, mirrors, "Ancora handoff read failed; the Git-validated mirror may continue and Ancora can be restored separately")
	}
	if err := index.validateStructural(remote); err != nil {
		return blockedHandoff(fmt.Errorf("malformed Ancora handoff: %w; repair the Ancora record and matching mirror", err))
	}
	if remoteTask, err := handoffTask(remote); err != nil || remoteTask != taskID {
		return blockedHandoff(errors.New("Ancora handoff task does not match requested task; repair the task-scoped index"))
	}
	if err := index.validateHandoffHistory(mirrors); err != nil {
		return blockedHandoff(err)
	}
	latest := mirrors[len(mirrors)-1]
	if remote.Sequence != latest.Sequence {
		return blockedHandoff(errors.New("Ancora handoff does not match the latest local sequence; reconcile both records before continuing"))
	}
	if !sameHandoff(latest, remote) {
		return blockedHandoff(errors.New("Ancora and mirror handoffs disagree; reconcile both records before continuing"))
	}
	if err := index.validateCurrentWorkspace(latest); err != nil {
		return blockedHandoff(err)
	}
	return HandoffRecovery{Record: remote, Source: "ancora+mirror"}
}

func (index *OrchestratorHandoffIndex) recoverMirror(taskID string, mirrors []HandoffRecord, reason string) HandoffRecovery {
	if len(mirrors) == 0 {
		return blockedHandoff(errors.New("no valid handoff mirror exists; restore a Git-validated handoff before continuing"))
	}
	if err := index.validateHandoffHistory(mirrors); err != nil {
		return blockedHandoff(err)
	}
	latest := mirrors[len(mirrors)-1]
	if err := index.validateCurrentWorkspace(latest); err != nil {
		return blockedHandoff(err)
	}
	return HandoffRecovery{Record: latest, Source: "mirror", Degraded: true, Remediation: reason}
}

func (index *OrchestratorHandoffIndex) validate(record HandoffRecord) error {
	if err := index.validateStructural(record); err != nil {
		return err
	}
	return index.validateCurrentWorkspace(record)
}

// validateStructural verifies a record against its own immutable Git snapshot.
// Historical records are not required to describe the checkout's current HEAD.
func (index *OrchestratorHandoffIndex) validateStructural(record HandoffRecord) error {
	if record.Format != handoffFormat || record.Sequence == 0 || record.HandoffID == "" || record.From == "" || record.To == "" || record.BaselineSHA == "" || record.SnapshotSHA == "" || len(record.Scope) == 0 || record.Evidence.Result == "" || record.Evidence.RecordedAt.IsZero() || record.Disposition == "" {
		return errors.New("handoff record is missing a required v1 field")
	}
	if _, err := handoffTask(record); err != nil {
		return err
	}
	if !oneOf(record.Status, "ready", "accepted", "blocked", "completed", "superseded") || !oneOf(record.Priority, "low", "normal", "high") {
		return errors.New("handoff status or priority is invalid")
	}
	if !legalHandoffRoles(record.From, record.To) {
		return errors.New("handoff role transition is illegal")
	}
	if (record.DeepReviewTrigger == "") != (record.ExpectedEvidence == "") {
		return errors.New("deep-review trigger and expected evidence must be recorded together")
	}
	if record.DeepReviewTrigger != "" && !legalDeepReviewTrigger(record.DeepReviewTrigger) {
		return errors.New("deep-review trigger must be Strict classification, explicit user request, repository policy, or concrete review evidence")
	}
	if qualityRole(record.From) || qualityRole(record.To) {
		if record.DeepReviewTrigger == "" || record.ExpectedEvidence == "" {
			return errors.New("deep-review handoff is missing its trigger or expected evidence")
		}
	}
	if !compactHandoffText(record) {
		return errors.New("handoff contains multiline or oversized field data")
	}
	for _, path := range record.Scope {
		if !canonicalHandoffPath(path) {
			return fmt.Errorf("handoff scope %q is not repository-confined", path)
		}
	}
	for _, path := range []string{record.StrictContractRef, record.GherkinRef} {
		if path != "" && (!canonicalHandoffPath(path) || repositoryFileExists(index.repoRoot, path) != nil) {
			return fmt.Errorf("handoff reference %q is missing or unsafe", path)
		}
	}
	if hasSensitiveHandoffData(record) {
		return errors.New("handoff contains sensitive, private, raw, or oversized data")
	}
	if !approvalBaselineIsCommitted(index.repoRoot, record.BaselineSHA) {
		return errors.New("handoff baseline is missing")
	}
	snapshot, err := gitSubmissionOutput(index.repoRoot, "rev-parse", record.SnapshotSHA+"^{commit}")
	if err != nil {
		return errors.New("handoff snapshot is missing")
	}
	if !approvalBaselineIsAncestorOf(index.repoRoot, record.BaselineSHA, snapshot) {
		return errors.New("handoff baseline is not an ancestor of the snapshot")
	}
	changed, err := gitSubmissionOutput(index.repoRoot, "diff", "--name-only", record.BaselineSHA+"..."+snapshot)
	if err != nil {
		return fmt.Errorf("validate handoff diff: %w", err)
	}
	for _, path := range strings.Fields(changed) {
		if !inHandoffScope(path, record.Scope) {
			return fmt.Errorf("current diff path %q is outside handoff scope", path)
		}
	}
	return nil
}

// validateCurrentWorkspace applies only to the candidate selected for recovery.
func (index *OrchestratorHandoffIndex) validateCurrentWorkspace(record HandoffRecord) error {
	head, err := gitSubmissionOutput(index.repoRoot, "rev-parse", "HEAD")
	snapshot, snapshotErr := gitSubmissionOutput(index.repoRoot, "rev-parse", record.SnapshotSHA+"^{commit}")
	if err != nil || snapshotErr != nil || head != snapshot {
		return errors.New("handoff snapshot does not match current HEAD")
	}
	staged, err := gitSubmissionOutput(index.repoRoot, "diff", "--name-only", "--cached", snapshot, "--")
	if err != nil {
		return fmt.Errorf("validate handoff index: %w", err)
	}
	if staged != "" {
		return errors.New("handoff workspace has staged index drift after the snapshot; stash or discard changes and restore the index to the snapshot before continuing")
	}
	unstaged, err := gitSubmissionOutput(index.repoRoot, "diff", "--name-only", "--")
	if err != nil {
		return fmt.Errorf("validate handoff worktree: %w", err)
	}
	if unstaged != "" {
		return errors.New("handoff workspace has unstaged worktree drift after the snapshot; stash or discard changes and restore the worktree to the snapshot before continuing")
	}
	untracked, err := gitSubmissionOutput(index.repoRoot, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return fmt.Errorf("validate handoff untracked files: %w", err)
	}
	for _, path := range strings.Fields(untracked) {
		if !isHandoffMirrorPath(path) && !isWorkflowCommandEvidencePath(path) {
			return fmt.Errorf("handoff workspace has untracked path %q after the snapshot; add, stash, or remove it before continuing", path)
		}
	}
	return nil
}

func (index *OrchestratorHandoffIndex) mirrors(taskID string) ([]HandoffRecord, error) {
	directory := filepath.Join(index.repoRoot, ".rotta", "handoffs")
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read handoff mirrors: %w", err)
	}
	var records []HandoffRecord
	seen := map[uint64]bool{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		contents, err := readRepositoryFile(index.repoRoot, filepath.ToSlash(filepath.Join(".rotta", "handoffs", entry.Name())))
		if err != nil {
			return nil, fmt.Errorf("read handoff mirror: %w", err)
		}
		record, err := parseHandoff(string(contents))
		if err != nil {
			return nil, fmt.Errorf("malformed handoff mirror: %w", err)
		}
		recordTask, err := handoffTask(record)
		if err != nil || recordTask != taskID {
			continue
		}
		if seen[record.Sequence] {
			return nil, errors.New("conflicting handoff mirrors claim the same task and sequence; reconcile them before continuing")
		}
		seen[record.Sequence] = true
		records = append(records, record)
	}
	return records, nil
}

func (index *OrchestratorHandoffIndex) writeMirror(taskID string, record HandoffRecord) error {
	directory := filepath.Join(index.repoRoot, ".rotta", "handoffs")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create handoff mirror directory: %w", err)
	}
	path := filepath.Join(directory, fmt.Sprintf("%s-%d.yaml", taskID, record.Sequence))
	if _, err := os.Lstat(path); err == nil {
		return errors.New("handoff mirror already exists; orchestrator cannot overwrite a record")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect handoff mirror: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".handoff-*")
	if err != nil {
		return fmt.Errorf("create handoff mirror: %w", err)
	}
	defer os.Remove(temporary.Name())
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.WriteString(serializeHandoff(record)); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	// Link fails when a concurrent orchestrator has already created the final
	// name, unlike Rename which can replace a valid immutable mirror on Unix.
	if err := os.Link(temporary.Name(), path); err != nil {
		return fmt.Errorf("persist handoff mirror: %w", err)
	}
	if err := os.Remove(temporary.Name()); err != nil {
		return fmt.Errorf("finalize handoff mirror: %w", err)
	}
	return nil
}

func validateNextHandoff(previous []HandoffRecord, next HandoffRecord) error {
	if err := validateHandoffSequence(previous); err != nil {
		return err
	}
	if len(previous) == 0 {
		if next.Sequence != 1 || next.Status != "ready" {
			return errors.New("initial handoff must be sequence 1 in ready status")
		}
		return validateHandoffRoute([]HandoffRecord{next})
	}
	last := previous[len(previous)-1]
	if next.Sequence != last.Sequence+1 {
		return errors.New("handoff sequence is not monotonic; create exactly the next sequence")
	}
	if !legalHandoffStatus(last.Status, next.Status) {
		return errors.New("handoff status transition is illegal")
	}
	return validateHandoffRoute(append(append([]HandoffRecord{}, previous...), next))
}

func (index *OrchestratorHandoffIndex) validateHandoffHistory(records []HandoffRecord) error {
	for _, record := range records {
		if err := index.validateStructural(record); err != nil {
			return fmt.Errorf("malformed handoff history: %w; repair the complete task history before continuing", err)
		}
	}
	if err := validateHandoffSequence(records); err != nil {
		return fmt.Errorf("invalid handoff history: %w; restore a contiguous legal task history beginning at sequence 1 before continuing", err)
	}
	return nil
}

func validateHandoffSequence(records []HandoffRecord) error {
	if len(records) == 0 {
		return nil
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Sequence < records[j].Sequence })
	if records[0].Sequence != 1 || records[0].Status != "ready" {
		return errors.New("handoff history must begin at sequence 1 in ready status")
	}
	for i := 1; i < len(records); i++ {
		if records[i].Sequence != records[i-1].Sequence+1 {
			return errors.New("handoff history sequence is not contiguous")
		}
		if !legalHandoffStatus(records[i-1].Status, records[i].Status) {
			return errors.New("handoff history status transition is illegal")
		}
	}
	return validateHandoffRoute(records)
}

func legalHandoffStatus(from, to string) bool {
	return (from == "ready" && oneOf(to, "accepted", "blocked", "superseded")) || (from == "accepted" && oneOf(to, "completed", "blocked", "superseded")) || (from == "completed" && to == "ready") || (from == "blocked" && to == "ready")
}
func legalHandoffRoles(from, to string) bool {
	if from == "rotta-orchestrator" {
		return oneOf(to, "rotta-impl", "rotta-cleaner", "rotta-architect")
	}
	return from == "rotta-impl" && oneOf(to, "rotta-cleaner", "rotta-review") ||
		from == "rotta-cleaner" && oneOf(to, "rotta-architect", "rotta-review") ||
		from == "rotta-architect" && oneOf(to, "rotta-impl", "rotta-review") ||
		from == "rotta-review" && to == "rotta-orchestrator"
}
func qualityRole(role string) bool { return oneOf(role, "rotta-cleaner", "rotta-architect") }

func legalDeepReviewTrigger(trigger string) bool {
	return oneOf(trigger,
		deepTriggerStrictClassification,
		deepTriggerUserRequest,
		deepTriggerRepositoryPolicy,
		deepTriggerReviewEvidence,
	)
}

// validateHandoffRoute permits only the orchestrator-mediated Fast and bounded
// deep routes. Consecutive equal role pairs are lifecycle updates of one
// handoff, not a second delegation.
func validateHandoffRoute(records []HandoffRecord) error {
	var route []HandoffRecord
	for _, record := range records {
		if len(route) == 0 || route[len(route)-1].From != record.From || route[len(route)-1].To != record.To {
			route = append(route, record)
		}
	}
	for i, record := range route {
		if !legalHandoffRoles(record.From, record.To) {
			return errors.New("handoff route contains an illegal role transition")
		}
		if i > 0 && route[i-1].To != record.From {
			return errors.New("handoff route is discontinuous")
		}
	}
	if !legalHandoffRoutePrefix(route) {
		return errors.New("handoff route must use one bounded Fast or deep path with exactly one fresh final review")
	}
	return nil
}

// legalHandoffRoutePrefix accepts an in-progress route, but only if it can
// still end in one of the approved paths. An initial review may return to the
// orchestrator for one quality escalation; that review then becomes stale and
// the escalation must end in one fresh terminal review. No candidate permits a
// second escalation or a second final review.
func legalHandoffRoutePrefix(route []HandoffRecord) bool {
	approved := [][][2]string{
		{{"rotta-orchestrator", "rotta-impl"}, {"rotta-impl", "rotta-review"}, {"rotta-review", "rotta-orchestrator"}},
		{{"rotta-orchestrator", "rotta-impl"}, {"rotta-impl", "rotta-cleaner"}, {"rotta-cleaner", "rotta-review"}, {"rotta-review", "rotta-orchestrator"}},
		{{"rotta-orchestrator", "rotta-impl"}, {"rotta-impl", "rotta-cleaner"}, {"rotta-cleaner", "rotta-architect"}, {"rotta-architect", "rotta-review"}, {"rotta-review", "rotta-orchestrator"}},
		{{"rotta-orchestrator", "rotta-impl"}, {"rotta-impl", "rotta-cleaner"}, {"rotta-cleaner", "rotta-architect"}, {"rotta-architect", "rotta-impl"}, {"rotta-impl", "rotta-review"}, {"rotta-review", "rotta-orchestrator"}},
		{{"rotta-orchestrator", "rotta-impl"}, {"rotta-impl", "rotta-review"}, {"rotta-review", "rotta-orchestrator"}, {"rotta-orchestrator", "rotta-cleaner"}, {"rotta-cleaner", "rotta-review"}, {"rotta-review", "rotta-orchestrator"}},
		{{"rotta-orchestrator", "rotta-impl"}, {"rotta-impl", "rotta-review"}, {"rotta-review", "rotta-orchestrator"}, {"rotta-orchestrator", "rotta-cleaner"}, {"rotta-cleaner", "rotta-architect"}, {"rotta-architect", "rotta-review"}, {"rotta-review", "rotta-orchestrator"}},
		{{"rotta-orchestrator", "rotta-impl"}, {"rotta-impl", "rotta-review"}, {"rotta-review", "rotta-orchestrator"}, {"rotta-orchestrator", "rotta-cleaner"}, {"rotta-cleaner", "rotta-architect"}, {"rotta-architect", "rotta-impl"}, {"rotta-impl", "rotta-review"}, {"rotta-review", "rotta-orchestrator"}},
		{{"rotta-orchestrator", "rotta-impl"}, {"rotta-impl", "rotta-review"}, {"rotta-review", "rotta-orchestrator"}, {"rotta-orchestrator", "rotta-architect"}, {"rotta-architect", "rotta-review"}, {"rotta-review", "rotta-orchestrator"}},
		{{"rotta-orchestrator", "rotta-impl"}, {"rotta-impl", "rotta-review"}, {"rotta-review", "rotta-orchestrator"}, {"rotta-orchestrator", "rotta-architect"}, {"rotta-architect", "rotta-impl"}, {"rotta-impl", "rotta-review"}, {"rotta-review", "rotta-orchestrator"}},
	}
	for _, candidate := range approved {
		if len(route) > len(candidate) {
			continue
		}
		matches := true
		for i, record := range route {
			if record.From != candidate[i][0] || record.To != candidate[i][1] {
				matches = false
				break
			}
		}
		if matches {
			return true
		}
	}
	return false
}
func oneOf(value string, choices ...string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}
func handoffTopic(taskID string) string { return "handoff/" + taskID }
func handoffTask(record HandoffRecord) (string, error) {
	task, sequence, ok := strings.Cut(record.HandoffID, "/")
	if !ok || len(record.HandoffID) > maxCanonicalHandoffIDBytes || !canonicalWorkflowHandoffTaskID(task) {
		return "", fmt.Errorf("handoff ID must be <lowercase-task-id>/<sequence> and at most %d bytes", maxCanonicalHandoffIDBytes)
	}
	n, err := strconv.ParseUint(sequence, 10, 64)
	if err != nil || n != record.Sequence {
		return "", errors.New("handoff ID sequence does not match record sequence")
	}
	return task, nil
}
func canonicalHandoffPath(path string) bool {
	path = strings.TrimSuffix(path, "/")
	return path != "" && filepath.ToSlash(path) == path && !filepath.IsAbs(path) && filepath.Clean(path) == path && path != "." && !strings.HasPrefix(path, "../")
}
func inHandoffScope(path string, scope []string) bool {
	for _, item := range scope {
		if path == strings.TrimSuffix(item, "/") || strings.HasPrefix(path, strings.TrimSuffix(item, "/")+"/") {
			return true
		}
	}
	return false
}
func isHandoffMirrorPath(path string) bool {
	match := handoffMirrorPath.FindStringSubmatch(path)
	if match == nil {
		return false
	}
	_, err := strconv.ParseUint(match[2], 10, 64)
	return err == nil
}
func isWorkflowCommandEvidencePath(path string) bool {
	return strings.HasPrefix(path, ".rotta/current/evidence/command-") && strings.HasSuffix(path, ".json")
}
func hasSensitiveHandoffData(record HandoffRecord) bool {
	data := serializeHandoff(record)
	if len(data) > maxHandoffBytes || len(record.Evidence.Commands) > 16 || len(record.Scope) > 32 {
		return true
	}
	return sensitiveHandoffData.MatchString(data)
}
func compactHandoffText(record HandoffRecord) bool {
	fields := []string{record.Format, record.HandoffID, record.From, record.To, record.Status, record.Priority, record.BaselineSHA, record.SnapshotSHA, record.StrictContractRef, record.GherkinRef, record.DeepReviewTrigger, record.ExpectedEvidence, record.Evidence.Result, record.Disposition}
	fields = append(fields, record.Scope...)
	fields = append(fields, record.Evidence.Commands...)
	for _, field := range fields {
		if len(field) > 512 || strings.ContainsAny(field, "\r\n") {
			return false
		}
	}
	return true
}
func blockedHandoff(err error) HandoffRecovery {
	return HandoffRecovery{Blocked: true, Remediation: "remediation: " + err.Error()}
}
func sameHandoff(left, right HandoffRecord) bool {
	return serializeHandoff(left) == serializeHandoff(right)
}

func serializeHandoff(record HandoffRecord) string {
	var b strings.Builder
	fmt.Fprintf(&b, "format: %s\nhandoff_id: %s\nsequence: %d\nfrom: %s\nto: %s\nstatus: %s\npriority: %s\nbaseline_sha: %s\nsnapshot_sha: %s\nscope:\n", record.Format, record.HandoffID, record.Sequence, record.From, record.To, record.Status, record.Priority, record.BaselineSHA, record.SnapshotSHA)
	for _, path := range record.Scope {
		fmt.Fprintf(&b, "  - %s\n", path)
	}
	if record.StrictContractRef != "" {
		fmt.Fprintf(&b, "strict_contract_ref: %s\n", record.StrictContractRef)
	}
	if record.GherkinRef != "" {
		fmt.Fprintf(&b, "gherkin_ref: %s\n", record.GherkinRef)
	}
	if record.DeepReviewTrigger != "" {
		fmt.Fprintf(&b, "deep_review_trigger: %s\n", record.DeepReviewTrigger)
		fmt.Fprintf(&b, "expected_evidence: %s\n", record.ExpectedEvidence)
	}
	b.WriteString("evidence:\n  commands:\n")
	for _, command := range record.Evidence.Commands {
		fmt.Fprintf(&b, "    - %s\n", command)
	}
	fmt.Fprintf(&b, "  result: %s\n  recorded_at: %s\ndisposition: %s\n", record.Evidence.Result, record.Evidence.RecordedAt.UTC().Format(time.RFC3339), record.Disposition)
	return b.String()
}

func parseHandoff(contents string) (HandoffRecord, error) {
	if len(contents) > maxHandoffBytes {
		return HandoffRecord{}, errors.New("handoff payload is oversized")
	}
	var record HandoffRecord
	section := ""
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSuffix(contents, "\n"), "\n") {
		if strings.HasPrefix(line, "  - ") && section == "scope" {
			record.Scope = append(record.Scope, strings.TrimPrefix(line, "  - "))
			continue
		}
		if strings.HasPrefix(line, "    - ") && section == "commands" {
			record.Evidence.Commands = append(record.Evidence.Commands, strings.TrimPrefix(line, "    - "))
			continue
		}
		if line == "scope:" {
			section = "scope"
			continue
		}
		if line == "evidence:" {
			section = "evidence"
			continue
		}
		if line == "  commands:" && section == "evidence" {
			section = "commands"
			continue
		}
		name, value, ok := strings.Cut(line, ": ")
		if !ok || seen[name] {
			return HandoffRecord{}, errors.New("handoff YAML is not a compact canonical record")
		}
		seen[name] = true
		section = ""
		switch name {
		case "format":
			record.Format = value
		case "handoff_id":
			record.HandoffID = value
		case "sequence":
			n, err := strconv.ParseUint(value, 10, 64)
			if err != nil {
				return HandoffRecord{}, err
			}
			record.Sequence = n
		case "from":
			record.From = value
		case "to":
			record.To = value
		case "status":
			record.Status = value
		case "priority":
			record.Priority = value
		case "baseline_sha":
			record.BaselineSHA = value
		case "snapshot_sha":
			record.SnapshotSHA = value
		case "strict_contract_ref":
			record.StrictContractRef = value
		case "gherkin_ref":
			record.GherkinRef = value
		case "deep_review_trigger":
			record.DeepReviewTrigger = value
		case "expected_evidence":
			record.ExpectedEvidence = value
		case "disposition":
			record.Disposition = value
		case "  result":
			record.Evidence.Result = value
		case "  recorded_at":
			parsed, err := time.Parse(time.RFC3339, value)
			if err != nil {
				return HandoffRecord{}, err
			}
			record.Evidence.RecordedAt = parsed
		default:
			return HandoffRecord{}, fmt.Errorf("unsupported handoff field %q", strings.TrimSpace(name))
		}
	}
	return record, nil
}
