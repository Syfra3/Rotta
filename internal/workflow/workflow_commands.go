package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const (
	WorkflowCommandFormat         = "rotta.workflow-command/v1"
	workflowCommandMetadataFormat = "rotta.workflow-command-metadata/v1"
	maxWorkflowCommandDiagnostics = 8
	maxCanonicalHandoffIDBytes    = 128
)

var workflowFeatureID = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

const trustedSystemBubblewrapPath = "/usr/bin/bwrap"

var resolveScopedVerificationSandbox = func() (string, error) {
	return trustedSystemBubblewrapPath, nil
}

// WorkflowCommandRequest contains only the explicit, repository-confined
// inputs needed by a narrow deterministic workflow command. It does not carry
// approval, lifecycle-transition, or operational authority.
type WorkflowCommandRequest struct {
	Worktree         string
	Feature          string
	ContractPath     string
	Baseline         string
	Scope            []string
	HandoffID        string
	EvidencePath     string
	EvidenceHash     string
	VerificationPkgs []string
	// RTKPresentation is nil by default so package/library callers never
	// consult host state or execute RTK. The production CLI opts in explicitly.
	RTKPresentation *RTKPresentation
	// Advisory is injected task-scoped context. Workflow commands recover it
	// once and may consume only explicitly named, bounded Vela questions.
	Advisory      *AdvisoryContext
	VelaQuestions []VelaQuestion
}

// WorkflowCommandResult is the compact, versioned result returned to callers.
// The referenced local evidence remains the complete durable record.
type WorkflowCommandResult struct {
	Format          string                 `json:"format"`
	Command         string                 `json:"command"`
	Status          string                 `json:"status"`
	CanonicalInputs WorkflowCommandInputs  `json:"canonical_inputs"`
	EvidencePath    string                 `json:"evidence_path"`
	EvidenceHash    string                 `json:"evidence_hash"`
	Diagnostics     []string               `json:"diagnostics"`
	Remediation     string                 `json:"remediation"`
	Advisory        WorkflowAdvisoryResult `json:"advisory,omitempty"`
	compactCapsule  CompactEvidenceResult
}

type WorkflowAdvisoryResult struct {
	Recovery AdvisoryRecovery     `json:"recovery,omitempty"`
	Vela     []VelaAdvisoryResult `json:"vela,omitempty"`
}

type workflowCommandResultJSON struct {
	Format          string                 `json:"format"`
	Command         string                 `json:"command"`
	Status          string                 `json:"status"`
	CanonicalInputs WorkflowCommandInputs  `json:"canonical_inputs"`
	EvidencePath    string                 `json:"evidence_path"`
	EvidenceHash    string                 `json:"evidence_hash"`
	Diagnostics     []string               `json:"diagnostics"`
	Remediation     string                 `json:"remediation"`
	Advisory        WorkflowAdvisoryResult `json:"advisory,omitempty"`
	Capsule         CompactEvidenceResult  `json:"capsule,omitempty"`
}

// CompactCapsule returns the immutable, validated capsule produced at the
// workflow boundary. The returned value has no public fields or mutators.
func (result WorkflowCommandResult) CompactCapsule() (CompactEvidenceResult, bool) {
	if result.compactCapsule == nil {
		return nil, false
	}
	return result.compactCapsule, true
}

// MarshalJSON cannot accept a caller-supplied capsule: it emits only the
// private value produced by compactWorkflowPresentation, whose marshaler
// revalidates the compact representation.
func (result WorkflowCommandResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(workflowCommandResultJSON{
		Format:          result.Format,
		Command:         result.Command,
		Status:          result.Status,
		CanonicalInputs: result.CanonicalInputs,
		EvidencePath:    result.EvidencePath,
		EvidenceHash:    result.EvidenceHash,
		Diagnostics:     result.Diagnostics,
		Remediation:     result.Remediation,
		Advisory:        result.Advisory,
		Capsule:         result.compactCapsule,
	})
}

type WorkflowCommandInputs struct {
	Worktree     string   `json:"worktree"`
	Feature      string   `json:"feature"`
	ContractPath string   `json:"contract_path"`
	Baseline     string   `json:"baseline"`
	Scope        []string `json:"scope"`
	HandoffID    string   `json:"handoff_id,omitempty"`
}

type workflowCommandMetadata struct {
	Format          string                `json:"format"`
	Command         string                `json:"command"`
	CanonicalInputs WorkflowCommandInputs `json:"canonical_inputs"`
}

// WorkflowCommandFailure marks a reported failed deterministic command. The
// command has already emitted its compact result and callers must return a
// nonzero exit status without treating the result as authorization.
type WorkflowCommandFailure struct{ result WorkflowCommandResult }

func (failure *WorkflowCommandFailure) Error() string {
	return failure.result.Command + " failed: " + failure.result.Remediation
}

func IsWorkflowCommandFailure(err error) bool {
	_, ok := err.(*WorkflowCommandFailure)
	return ok
}

func (failure *WorkflowCommandFailure) Result() WorkflowCommandResult { return failure.result }

// RunWorkflowPreflight validates an attached, clean worktree and the supplied
// contract/baseline/scope before an agent reconstructs those checks manually.
func RunWorkflowPreflight(workingDirectory string, request WorkflowCommandRequest) (WorkflowCommandResult, error) {
	root, inputs, err := validateWorkflowCommandInputs(workingDirectory, request, false)
	if err != nil {
		return WorkflowCommandResult{}, err
	}
	advisory := resolveWorkflowAdvisory(request)
	if err := verifyWorkflowClean(root); err != nil {
		return persistWorkflowCommandResult(root, "preflight", inputs, "failed", []string{err.Error()}, "restore a clean recorded worktree before continuing")
	}
	if err := validateWorkflowEvidence(root, request, "preflight", inputs); err != nil {
		return persistWorkflowCommandResult(root, "preflight", inputs, "failed", []string{err.Error()}, "provide matching current command evidence or omit the evidence reference")
	}
	return persistWorkflowCommandResultWithAdvisory(root, "preflight", inputs, "passed", nil, "continue with the scoped deterministic command before review", advisory)
}

// RunWorkflowHandoffValidation reuses the canonical handoff recovery checks;
// it neither writes nor advances a handoff record.
func RunWorkflowHandoffValidation(workingDirectory string, request WorkflowCommandRequest) (WorkflowCommandResult, error) {
	root, inputs, err := validateWorkflowCommandInputs(workingDirectory, request, true)
	if err != nil {
		return WorkflowCommandResult{}, err
	}
	advisory := resolveWorkflowAdvisory(request)
	if !canonicalWorkflowHandoffTaskID(request.HandoffID) {
		return WorkflowCommandResult{}, fmt.Errorf("handoff ID must be a lowercase hyphenated task ID of at most %d bytes", maxCanonicalHandoffIDBytes)
	}
	if err := validateWorkflowEvidence(root, request, "handoff-validate", inputs); err != nil {
		return persistWorkflowCommandResult(root, "handoff-validate", inputs, "failed", []string{err.Error()}, "provide matching current command evidence or omit the evidence reference")
	}
	recovery := NewOrchestratorHandoffIndex(root, nil).Recover(request.HandoffID)
	if recovery.Blocked {
		return persistWorkflowCommandResult(root, "handoff-validate", inputs, "failed", []string{recovery.Remediation}, "repair the canonical handoff mirror and workspace state before continuing")
	}
	status := "passed"
	diagnostics := []string{"recovery source: " + recovery.Source}
	remediation := "continue from the validated handoff evidence"
	if recovery.Degraded {
		status = "passed"
		diagnostics = append(diagnostics, recovery.Remediation)
		remediation = "continue only with the validated local mirror; restore Ancora separately"
	}
	return persistWorkflowCommandResultWithAdvisory(root, "handoff-validate", inputs, status, diagnostics, remediation, advisory)
}

// RunScopedVerification executes only explicit, repository-confined Go package
// tests in a containment boundary. Its sole persistent host write is a full
// local command-evidence record; compiler caches and test temporary files exist
// only in the sandbox's tmpfs.
func RunScopedVerification(workingDirectory string, request WorkflowCommandRequest) (WorkflowCommandResult, error) {
	root, inputs, err := validateWorkflowCommandInputs(workingDirectory, request, false)
	if err != nil {
		return WorkflowCommandResult{}, err
	}
	advisory := resolveWorkflowAdvisory(request)
	if len(request.VerificationPkgs) == 0 || len(request.VerificationPkgs) > maxWorkflowCommandDiagnostics {
		return WorkflowCommandResult{}, fmt.Errorf("scoped verification requires one to %d explicit Go packages", maxWorkflowCommandDiagnostics)
	}
	for _, pkg := range request.VerificationPkgs {
		if !canonicalGoPackage(pkg) {
			return WorkflowCommandResult{}, fmt.Errorf("verification package %q is not a canonical repository-relative Go package", pkg)
		}
	}
	if err := verifyWorkflowClean(root); err != nil {
		return persistWorkflowCommandResult(root, "scoped-verify", inputs, "failed", []string{err.Error()}, "restore a clean recorded worktree before scoped verification")
	}
	if err := validateWorkflowEvidence(root, request, "scoped-verify", inputs); err != nil {
		return persistWorkflowCommandResult(root, "scoped-verify", inputs, "failed", []string{err.Error()}, "provide matching current command evidence or omit the evidence reference")
	}
	command, err := scopedVerificationSandboxCommand(root, request.VerificationPkgs)
	if err != nil {
		return WorkflowCommandResult{}, err
	}
	metadata, err := json.Marshal(workflowCommandMetadata{Format: workflowCommandMetadataFormat, Command: "scoped-verify", CanonicalInputs: inputs})
	if err != nil {
		return WorkflowCommandResult{}, fmt.Errorf("encode scoped verification metadata: %w", err)
	}
	report, err := CaptureLifecycleCommand(context.Background(), LifecycleCommandRequest{
		FeatureWorktree:  root,
		WorkingDirectory: root,
		Command:          command,
		CommandMetadata:  metadata,
		RTKPresentation:  request.RTKPresentation,
	})
	if err != nil {
		return WorkflowCommandResult{}, err
	}
	evidencePath, err := canonicalEvidencePath(root, report.EvidencePath)
	if err != nil {
		return WorkflowCommandResult{}, err
	}
	status, diagnostics, remediation := "passed", []string{}, "continue to independent review; publication remains planning only"
	if !report.Passing {
		status, diagnostics, remediation = "failed", []string{"scoped Go verification failed; inspect durable evidence"}, "correct the in-scope failure and rerun scoped verification"
	}
	result := compactWorkflowPresentation(WorkflowCommandResult{Format: WorkflowCommandFormat, Command: "scoped-verify", Status: status, CanonicalInputs: inputs, EvidencePath: evidencePath, EvidenceHash: report.EvidenceHash, Diagnostics: diagnostics, Remediation: remediation, Advisory: advisory})
	if status == "failed" {
		return result, &WorkflowCommandFailure{result: result}
	}
	return result, nil
}

// RunPublicationPlan validates the same read-only boundary as preflight and
// reports only a plan; it cannot commit, push, create a PR, or publish.
func RunPublicationPlan(workingDirectory string, request WorkflowCommandRequest) (WorkflowCommandResult, error) {
	root, inputs, err := validateWorkflowCommandInputs(workingDirectory, request, false)
	if err != nil {
		return WorkflowCommandResult{}, err
	}
	advisory := resolveWorkflowAdvisory(request)
	if err := verifyWorkflowClean(root); err != nil {
		return persistWorkflowCommandResult(root, "publication-plan", inputs, "failed", []string{err.Error()}, "restore a clean recorded worktree before publication planning")
	}
	if err := validateWorkflowEvidence(root, request, "publication-plan", inputs); err != nil {
		return persistWorkflowCommandResult(root, "publication-plan", inputs, "failed", []string{err.Error()}, "provide matching current command evidence or omit the evidence reference")
	}
	return persistWorkflowCommandResultWithAdvisory(root, "publication-plan", inputs, "passed", []string{"read-only plan: separate approval is required for every publication operation"}, "obtain separate exact authority before any commit, push, or pull-request operation", advisory)
}

func resolveWorkflowAdvisory(request WorkflowCommandRequest) WorkflowAdvisoryResult {
	if request.Advisory == nil {
		return WorkflowAdvisoryResult{}
	}
	result := WorkflowAdvisoryResult{Recovery: request.Advisory.RecoverAncoraOnce()}
	for _, question := range request.VelaQuestions {
		result.Vela = append(result.Vela, request.Advisory.AskVela(question))
	}
	return result
}

func validateWorkflowCommandInputs(workingDirectory string, request WorkflowCommandRequest, handoff bool) (string, WorkflowCommandInputs, error) {
	root, err := resolveWorkflowWorktree(workingDirectory, request.Worktree)
	if err != nil {
		return "", WorkflowCommandInputs{}, err
	}
	if len(request.Feature) > 128 || !workflowFeatureID.MatchString(request.Feature) {
		return "", WorkflowCommandInputs{}, fmt.Errorf("feature must be a lowercase hyphenated ID")
	}
	contractPath, err := canonicalExistingWorkflowPath(root, request.ContractPath)
	if err != nil {
		return "", WorkflowCommandInputs{}, fmt.Errorf("contract path: %w", err)
	}
	baseline, err := gitSubmissionOutput(root, "rev-parse", "--verify", request.Baseline+"^{commit}")
	if err != nil {
		return "", WorkflowCommandInputs{}, fmt.Errorf("baseline is missing")
	}
	if err := baselineAncestor(root, baseline); err != nil {
		return "", WorkflowCommandInputs{}, err
	}
	scope, err := canonicalWorkflowScope(root, request.Scope)
	if err != nil {
		return "", WorkflowCommandInputs{}, err
	}
	inputs := WorkflowCommandInputs{Worktree: ".", Feature: request.Feature, ContractPath: contractPath, Baseline: baseline, Scope: scope}
	if handoff {
		inputs.HandoffID = request.HandoffID
	}
	return root, inputs, nil
}

func resolveWorkflowWorktree(workingDirectory, value string) (string, error) {
	if !canonicalWorkflowWorktree(value) {
		return "", fmt.Errorf("worktree path must be a non-empty canonical relative path")
	}
	requested, err := filepath.Abs(filepath.Join(workingDirectory, filepath.FromSlash(value)))
	if err != nil {
		return "", fmt.Errorf("resolve worktree: %w", err)
	}
	root, err := gitSubmissionOutput(requested, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("resolve worktree: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve worktree: %w", err)
	}
	if _, err := gitSubmissionOutput(root, "symbolic-ref", "--quiet", "--short", "HEAD"); err != nil {
		return "", fmt.Errorf("worktree has detached HEAD")
	}
	callerRoot, err := gitSubmissionOutput(workingDirectory, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("resolve invoking worktree: %w", err)
	}
	callerRoot, err = filepath.EvalSymlinks(callerRoot)
	if err != nil || callerRoot != root {
		return "", fmt.Errorf("worktree does not match the invoking worktree")
	}
	return root, nil
}

func canonicalWorkflowPath(path string) bool {
	return path != "" && len(path) <= 512 && filepath.ToSlash(path) == path && !filepath.IsAbs(path) && filepath.Clean(path) == path && path != "." && !strings.HasPrefix(path, "../")
}

func canonicalWorkflowWorktree(path string) bool {
	return path == "." || canonicalWorkflowPath(path)
}

func canonicalExistingWorkflowPath(root, path string) (string, error) {
	if !canonicalWorkflowPath(path) {
		return "", fmt.Errorf("must be relative, canonical, and worktree-confined")
	}
	if err := repositoryFileExists(root, path); err != nil {
		return "", fmt.Errorf("is missing or outside the worktree")
	}
	return path, nil
}

func canonicalWorkflowScope(root string, scope []string) ([]string, error) {
	if len(scope) == 0 || len(scope) > maxWorkflowCommandDiagnostics {
		return nil, fmt.Errorf("scope requires one to %d repository-relative paths", maxWorkflowCommandDiagnostics)
	}
	seen := make(map[string]bool, len(scope))
	canonical := make([]string, 0, len(scope))
	for _, path := range scope {
		if !canonicalWorkflowPath(path) || seen[path] {
			return nil, fmt.Errorf("scope path %q is missing, ambiguous, or not worktree-confined", path)
		}
		if err := repositoryFileExists(root, path); err != nil {
			return nil, fmt.Errorf("scope path %q is missing or outside the worktree", path)
		}
		seen[path] = true
		canonical = append(canonical, path)
	}
	return canonical, nil
}

func baselineAncestor(root, baseline string) error {
	if _, err := gitSubmissionOutput(root, "merge-base", "--is-ancestor", baseline, "HEAD"); err != nil {
		return fmt.Errorf("baseline is not an ancestor of HEAD")
	}
	return nil
}

func verifyWorkflowClean(root string) error {
	status, err := gitSubmissionOutput(root, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return fmt.Errorf("check worktree cleanliness: %w", err)
	}
	for _, line := range strings.Split(status, "\n") {
		if line == "" || isWorkflowCommandEvidenceStatus(line) {
			continue
		}
		return fmt.Errorf("worktree has non-evidence changes")
	}
	return nil
}

func isWorkflowCommandEvidenceStatus(line string) bool {
	path := strings.TrimSpace(line)
	if len(line) > 3 {
		path = strings.TrimSpace(line[3:])
	}
	return strings.HasPrefix(path, ".rotta/current/evidence/command-") && strings.HasSuffix(path, ".json")
}

func canonicalGoPackage(pkg string) bool {
	return strings.HasPrefix(pkg, "./") && pkg != "./" && !strings.Contains(pkg, "\\") && !strings.Contains(pkg, "//") && !strings.Contains(pkg, "..")
}

func canonicalWorkflowHandoffTaskID(value string) bool {
	return len(value) <= maxCanonicalHandoffIDBytes && handoffTaskID.MatchString(value)
}

// scopedVerificationSandboxCommand returns the only supported execution model
// for REQ-092 scoped Go verification. Bubblewrap exposes only the read-only
// system toolchain paths and worktree, removes network access, and supplies
// tmpfs-backed paths for Go's transient cache and test files. A missing
// Linux/Bubblewrap capability refuses verification before any repository test
// code runs.
func scopedVerificationSandboxCommand(root string, packages []string) ([]string, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("scoped verification requires Linux Bubblewrap containment; go test was not started")
	}
	bwrap, err := resolveScopedVerificationSandbox()
	if err != nil {
		return nil, fmt.Errorf("scoped verification requires Bubblewrap containment; go test was not started: %w", err)
	}
	bwrap, err = canonicalTrustedSystemBubblewrap(bwrap)
	if err != nil {
		return nil, fmt.Errorf("scoped verification requires trusted Bubblewrap containment; go test was not started: %w", err)
	}
	goCommand, err := exec.LookPath("go")
	if err != nil {
		return nil, fmt.Errorf("scoped verification requires the Go command; go test was not started: %w", err)
	}
	if !strings.HasPrefix(filepath.ToSlash(goCommand), "/usr/") {
		return nil, fmt.Errorf("scoped verification requires a Go toolchain beneath /usr for Bubblewrap containment; go test was not started")
	}
	for _, path := range []string{"/usr", "/lib", "/lib64", "/bin"} {
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("scoped verification requires system path %q for Bubblewrap containment; go test was not started", path)
		}
	}
	args := []string{
		bwrap,
		"--unshare-all", "--die-with-parent",
		"--ro-bind", "/usr", "/usr",
		"--ro-bind", "/lib", "/lib",
		"--ro-bind", "/lib64", "/lib64",
		"--ro-bind", "/bin", "/bin",
		"--dev", "/dev", "--proc", "/proc", "--tmpfs", "/tmp",
		"--ro-bind", root, root,
		"--clearenv",
		"--setenv", "HOME", "/tmp/home",
		"--setenv", "TMPDIR", "/tmp",
		"--setenv", "GOCACHE", "/tmp/go-build",
		"--setenv", "GOMODCACHE", "/tmp/go-mod",
		"--setenv", "GOPATH", "/tmp/go",
		"--setenv", "PATH", "/usr/bin:/bin",
		"--chdir", root,
		"--", goCommand, "test",
	}
	return append(args, packages...), nil
}

// canonicalTrustedSystemBubblewrap permits only the explicitly supported
// system Bubblewrap path. It resolves symlinks before comparing the result so
// a trusted-looking path cannot redirect execution outside the system path.
func canonicalTrustedSystemBubblewrap(path string) (string, error) {
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("resolve trusted Bubblewrap path: %w", err)
	}
	canonical = filepath.Clean(canonical)
	if path != trustedSystemBubblewrapPath || canonical != trustedSystemBubblewrapPath {
		return "", fmt.Errorf("Bubblewrap path %q resolves to non-trusted system location %q", path, canonical)
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return "", fmt.Errorf("stat trusted Bubblewrap path: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return "", fmt.Errorf("trusted Bubblewrap path is not a regular executable")
	}
	return canonical, nil
}

func validateWorkflowEvidence(root string, request WorkflowCommandRequest, command string, inputs WorkflowCommandInputs) error {
	if request.EvidencePath == "" && request.EvidenceHash == "" {
		return nil
	}
	if request.EvidencePath == "" || request.EvidenceHash == "" {
		return fmt.Errorf("evidence path and evidence hash must be supplied together")
	}
	evidence, err := loadWorkflowEvidence(root, request.EvidencePath, request.EvidenceHash)
	if err != nil {
		return err
	}
	var metadata workflowCommandMetadata
	if err := json.Unmarshal(evidence.CommandMetadata, &metadata); err != nil || metadata.Format != workflowCommandMetadataFormat || metadata.Command != command || !sameWorkflowInputs(metadata.CanonicalInputs, inputs) {
		return fmt.Errorf("evidence is stale or belongs to another command, baseline, or scope")
	}
	return nil
}

// loadWorkflowEvidence resolves a caller reference only beneath root, then
// confirms that the durable record and its complete output still match the
// supplied content hash. Callers must validate their own bound payload after
// this common integrity check.
func loadWorkflowEvidence(root, path, hash string) (lifecycleCommandEvidence, error) {
	canonicalPath, err := canonicalExistingWorkflowPath(root, path)
	if err != nil {
		return lifecycleCommandEvidence{}, fmt.Errorf("evidence path: %w", err)
	}
	contents, err := readRepositoryFile(root, canonicalPath)
	if err != nil {
		return lifecycleCommandEvidence{}, fmt.Errorf("read evidence: %w", err)
	}
	var evidence lifecycleCommandEvidence
	if err := json.Unmarshal(contents, &evidence); err != nil || evidence.ContentHash != hash {
		return lifecycleCommandEvidence{}, fmt.Errorf("evidence hash does not match durable evidence")
	}
	if commandOutputHash(evidence.Stdout, evidence.Stderr) != evidence.ContentHash {
		return lifecycleCommandEvidence{}, fmt.Errorf("evidence content hash does not match persisted stdout/stderr")
	}
	return evidence, nil
}

func sameWorkflowInputs(left, right WorkflowCommandInputs) bool {
	return left.Worktree == right.Worktree && left.Feature == right.Feature && left.ContractPath == right.ContractPath && left.Baseline == right.Baseline && left.HandoffID == right.HandoffID && strings.Join(left.Scope, "\x00") == strings.Join(right.Scope, "\x00")
}

func persistWorkflowCommandResult(root, command string, inputs WorkflowCommandInputs, status string, diagnostics []string, remediation string) (WorkflowCommandResult, error) {
	return persistWorkflowCommandResultWithAdvisory(root, command, inputs, status, diagnostics, remediation, WorkflowAdvisoryResult{})
}

func persistWorkflowCommandResultWithAdvisory(root, command string, inputs WorkflowCommandInputs, status string, diagnostics []string, remediation string, advisory WorkflowAdvisoryResult) (WorkflowCommandResult, error) {
	diagnostics = boundedWorkflowDiagnostics(diagnostics)
	result := WorkflowCommandResult{Format: WorkflowCommandFormat, Command: command, Status: status, CanonicalInputs: inputs, Diagnostics: diagnostics, Remediation: truncateSummaryValue(remediation, 512), Advisory: advisory}
	payload, err := json.Marshal(result)
	if err != nil {
		return WorkflowCommandResult{}, fmt.Errorf("encode command result: %w", err)
	}
	metadata, err := json.Marshal(workflowCommandMetadata{Format: workflowCommandMetadataFormat, Command: command, CanonicalInputs: inputs})
	if err != nil {
		return WorkflowCommandResult{}, fmt.Errorf("encode command metadata: %w", err)
	}
	hash := commandOutputHash(string(payload), "")
	evidence := lifecycleCommandEvidence{Command: []string{"rotta", "workflow", command}, WorkingDirectory: root, StartedAt: time.Now().UTC(), FinishedAt: time.Now().UTC(), Stdout: string(payload), ExitStatus: map[string]int{"passed": 0, "failed": 1}[status], ContentHash: hash, CommandMetadata: metadata}
	evidencePath, err := writeLifecycleCommandEvidence(root, evidence)
	if err != nil {
		return WorkflowCommandResult{}, fmt.Errorf("persist workflow command evidence: %w", err)
	}
	canonicalPath, err := canonicalEvidencePath(root, evidencePath)
	if err != nil {
		return WorkflowCommandResult{}, err
	}
	result.EvidencePath, result.EvidenceHash = canonicalPath, evidence.ContentHash
	result = compactWorkflowPresentation(result)
	if status == "failed" {
		return result, &WorkflowCommandFailure{result: result}
	}
	return result, nil
}

// compactWorkflowPresentation is the production workflow/agent handoff
// boundary. It returns a bounded reference-only capsule or makes its omission
// explicit through a fixed bounded diagnostic; durable evidence remains the
// authority in both cases.
func compactWorkflowPresentation(result WorkflowCommandResult) WorkflowCommandResult {
	canonical := result
	canonical.compactCapsule = nil
	canonical.Diagnostics = compactCapsuleDiagnostics(result.Diagnostics)
	status := OutcomeFailed
	if result.Status == "passed" {
		status = OutcomePassed
	}
	capsule, err := NewCompactEvidenceResult(CompactEvidenceInput{
		CanonicalOutcome: canonical,
		Evidence:         DurableEvidenceReference{Check: result.Command, Path: result.EvidencePath, Hash: result.EvidenceHash, Status: status},
		ChangedPaths:     append([]string(nil), result.CanonicalInputs.Scope...),
		Scope:            append([]string(nil), result.CanonicalInputs.Scope...),
		Risk:             compactEvidenceRisk,
		Remediation:      result.Remediation,
	})
	if err != nil {
		result.Diagnostics = boundedWorkflowDiagnostics(result.Diagnostics)
		const omission = "compact capsule omitted: rejected at compact boundary"
		if len(result.Diagnostics) == maxWorkflowCommandDiagnostics {
			result.Diagnostics[len(result.Diagnostics)-1] = omission
		} else {
			result.Diagnostics = append(result.Diagnostics, omission)
		}
		return result
	}
	result.compactCapsule = capsule
	return result
}

func canonicalEvidencePath(root, path string) (string, error) {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return "", fmt.Errorf("canonicalize durable evidence path: %w", err)
	}
	relative = filepath.ToSlash(relative)
	if !canonicalWorkflowPath(relative) {
		return "", fmt.Errorf("durable evidence path is outside the worktree")
	}
	return relative, nil
}

func boundedWorkflowDiagnostics(values []string) []string {
	if len(values) > maxWorkflowCommandDiagnostics {
		values = values[:maxWorkflowCommandDiagnostics]
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, truncateSummaryValue(value, 256))
	}
	return result
}
