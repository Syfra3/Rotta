package workflow

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type QualityGatesReviewState string

const QualityGatesReviewBlocked QualityGatesReviewState = "blocked"

type QualityGatesReview struct {
	State  QualityGatesReviewState
	Result string
}

type Phase4ReviewPlan struct {
	Baseline                 string
	Snapshot                 string
	ConfigurationFingerprint string
	PlanFingerprint          string
	Gates                    []ResolvedQualityGate
}

type ResolvedQualityGate struct {
	Category       string
	Command        string
	MetadataSource string
	DiscoveryRule  string
}

type Phase4ReviewResult struct {
	Readiness                string
	Snapshot                 string
	ConfigurationFingerprint string
	PlanFingerprint          string
}

type PRReadiness struct {
	State   string
	Gates   []PRReadinessGate
	Waivers []DurableWaiver
}

type PRReadinessGate struct {
	Category         string
	Status           string
	UnderlyingStatus string
}

type DurableWaiver struct {
	Gate                     string
	Reason                   string
	Scope                    string
	Timestamp                string
	Snapshot                 string
	ConfigurationFingerprint string
}

type phase4GateOutcome struct {
	Category    string
	Status      string
	Command     string
	Output      string
	ExitResult  string
	Remediation string
	Measurement string
}

var requiredGenericGateCategories = []string{
	"build",
	"tests",
	"changed_file_scope",
	"static_analysis",
	"dependency_checks",
	"security_checks",
}

type ChangedFileScope struct {
	Baseline string
	Snapshot string
	Changed  []string
	Renamed  []ChangedFileRename
	Deleted  []string
}

type ChangedFileRename struct {
	From string
	To   string
}

type ambiguousStaticAnalysisConventionError struct{}

func (ambiguousStaticAnalysisConventionError) Error() string {
	return "resolve review plan: equally preferred static-analysis conventions conflict"
}

// RequestPhase4Review rejects unsupported quality-gates configurations before command execution.
func RequestPhase4Review(repoRoot string, execute func(string) error) (QualityGatesReview, error) {
	configPath := filepath.Join(repoRoot, ".rotta", "quality-gates.yaml")
	config, err := os.ReadFile(configPath)
	if err != nil {
		return QualityGatesReview{}, fmt.Errorf("read quality-gates configuration: %w", err)
	}
	format := qualityGatesFormat(config)
	if format == "rotta.quality-gates/v1" {
		return QualityGatesReview{
			State:  QualityGatesReviewBlocked,
			Result: "quality-gates v1 is unsupported and is not automatically migrated; replace .rotta/quality-gates.yaml with the generated rotta.quality-gates/v2 configuration before requesting Phase 4 review",
		}, nil
	}
	if format == "rotta.quality-gates/v2" {
		plan, err := ResolvePhase4ReviewPlan(repoRoot)
		var ambiguousStaticAnalysis ambiguousStaticAnalysisConventionError
		if errors.As(err, &ambiguousStaticAnalysis) {
			return QualityGatesReview{
				State:  QualityGatesReviewBlocked,
				Result: "static-analysis gate is blocked: equally preferred supported conventions resolve ambiguous conflicting commands; declare exactly one supported convention before requesting Phase 4 review",
			}, nil
		}
		if err == nil && hasResolvedSecurityCheck(plan.Gates) {
			return QualityGatesReview{}, fmt.Errorf("quality-gates review is unavailable for the active configuration")
		}
		return QualityGatesReview{
			State:  QualityGatesReviewBlocked,
			Result: "security-check gate is blocked: declare the security-check command in supported project metadata or configure a supported convention before requesting Phase 4 review",
		}, nil
	}

	return QualityGatesReview{}, fmt.Errorf("quality-gates review is unavailable for the active configuration")
}

// ResolvePhase4ReviewPlan uses only explicitly declared v2 convention metadata.
func ResolvePhase4ReviewPlan(repoRoot string) (Phase4ReviewPlan, error) {
	configPath := filepath.Join(repoRoot, ".rotta", "quality-gates.yaml")
	config, err := os.ReadFile(configPath)
	if err != nil {
		return Phase4ReviewPlan{}, fmt.Errorf("read quality-gates configuration: %w", err)
	}
	if qualityGatesFormat(config) != "rotta.quality-gates/v2" {
		return Phase4ReviewPlan{}, fmt.Errorf("resolve review plan: unsupported quality-gates configuration")
	}
	if !supportsDeclaredConventionDiscovery(config) {
		return Phase4ReviewPlan{}, fmt.Errorf("resolve review plan: declared convention discovery is not supported by the active configuration")
	}

	metadataPath := filepath.Join(repoRoot, ".rotta", "current", "review-snapshot.yaml")
	metadata, err := os.ReadFile(metadataPath)
	if err != nil {
		return Phase4ReviewPlan{}, fmt.Errorf("read recorded review snapshot: %w", err)
	}
	baseline, snapshot, gates, err := declaredConventionGates(metadata, metadataPath)
	if err != nil {
		return Phase4ReviewPlan{}, err
	}

	plan := Phase4ReviewPlan{
		Baseline:                 baseline,
		Snapshot:                 snapshot,
		ConfigurationFingerprint: fingerprint(config),
		Gates:                    gates,
	}
	plan.PlanFingerprint = fingerprint([]byte(planFingerprintInput(plan)))
	if err := persistPhase4ReviewPlan(filepath.Join(repoRoot, ".rotta", "current", "review-plan.yaml"), plan); err != nil {
		return Phase4ReviewPlan{}, err
	}
	return plan, nil
}

// ResolveChangedFileScope compares only the recorded baseline and review snapshot.
func ResolveChangedFileScope(repoRoot string, _ []string) (ChangedFileScope, error) {
	metadataPath := filepath.Join(repoRoot, ".rotta", "current", "review-snapshot.yaml")
	metadata, err := os.ReadFile(metadataPath)
	if err != nil {
		return ChangedFileScope{}, fmt.Errorf("read recorded review snapshot: %w", err)
	}
	baseline, snapshot := recordedReviewSnapshotIdentities(metadata)
	if baseline == "" || snapshot == "" {
		return ChangedFileScope{}, fmt.Errorf("resolve changed-file scope: recorded review snapshot has no comparison identities")
	}

	command := exec.Command("git", "diff", "--name-status", "-M", baseline, snapshot) // #nosec G204 -- Git binary and command structure are fixed; identities come from recorded snapshot metadata.
	command.Dir = repoRoot
	output, err := command.Output()
	if err != nil {
		return ChangedFileScope{}, fmt.Errorf("measure changed-file scope: %w", err)
	}

	scope := ChangedFileScope{Baseline: baseline, Snapshot: snapshot}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}
		switch {
		case fields[0] == "D":
			scope.Deleted = append(scope.Deleted, fields[1])
		case strings.HasPrefix(fields[0], "R") && len(fields) == 3:
			scope.Renamed = append(scope.Renamed, ChangedFileRename{From: fields[1], To: fields[2]})
		default:
			scope.Changed = append(scope.Changed, fields[len(fields)-1])
		}
	}
	if err := persistChangedFileScope(filepath.Join(repoRoot, ".rotta", "current", "changed-file-scope.yaml"), scope); err != nil {
		return ChangedFileScope{}, err
	}
	return scope, nil
}

// EvaluatePhase4Review persists successful required-gate evidence for the
// committed review snapshot, then makes that snapshot eligible for final review.
func EvaluatePhase4Review(repoRoot string, execute func(string) (string, error)) (Phase4ReviewResult, error) {
	if execute == nil {
		return Phase4ReviewResult{}, fmt.Errorf("evaluate Phase 4 review: command executor is required")
	}
	plan, err := ResolvePhase4ReviewPlan(repoRoot)
	if err != nil {
		return Phase4ReviewResult{}, err
	}
	if !hasRequiredGenericGateOrder(plan.Gates) {
		return Phase4ReviewResult{}, fmt.Errorf("evaluate Phase 4 review: resolved plan must contain the six ordered generic gates")
	}
	statePath := filepath.Join(repoRoot, ".rotta", "current", "state.yaml")
	state, err := os.ReadFile(statePath)
	if err != nil {
		return Phase4ReviewResult{}, fmt.Errorf("read current submission state: %w", err)
	}
	if stateValue(state, "baseline_commit") != plan.Baseline {
		return Phase4ReviewResult{}, fmt.Errorf("evaluate Phase 4 review: recorded baseline does not match review plan")
	}
	if err := validateCurrentTDDEvidence(repoRoot, state); err != nil {
		return Phase4ReviewResult{}, err
	}
	if err := validateCommittedReviewSnapshot(repoRoot, plan.Snapshot); err != nil {
		return Phase4ReviewResult{}, err
	}

	scope, err := ResolveChangedFileScope(repoRoot, nil)
	if err != nil {
		return Phase4ReviewResult{}, err
	}
	outcomes := make([]phase4GateOutcome, 0, len(plan.Gates))
	for _, gate := range plan.Gates {
		output, err := execute(gate.Command)
		if err != nil {
			outcomes = append(outcomes, phase4GateOutcome{
				Category:    gate.Category,
				Status:      "failed",
				Command:     gate.Command,
				Output:      output,
				ExitResult:  err.Error(),
				Remediation: fmt.Sprintf("fix the %s gate failure and rerun %q before requesting Phase 4 review", gate.Category, gate.Command),
			})
			result := Phase4ReviewResult{
				Readiness:                "not_ready",
				Snapshot:                 plan.Snapshot,
				ConfigurationFingerprint: plan.ConfigurationFingerprint,
				PlanFingerprint:          plan.PlanFingerprint,
			}
			if err := persistPhase4ReviewEvidence(filepath.Join(repoRoot, ".rotta", "current", "review-evidence.yaml"), plan, outcomes, result.Readiness); err != nil {
				return Phase4ReviewResult{}, err
			}
			updatedState := setStateValue(state, "overall_readiness", result.Readiness)
			updatedState = setStateValue(updatedState, "review_evidence", ".rotta/current/review-evidence.yaml")
			updatedState = setStateValue(updatedState, "configuration_fingerprint", plan.ConfigurationFingerprint)
			updatedState = setStateValue(updatedState, "plan_fingerprint", plan.PlanFingerprint)
			if err := os.WriteFile(statePath, updatedState, 0o600); err != nil {
				return Phase4ReviewResult{}, fmt.Errorf("persist not-ready review state: %w", err)
			}
			return result, nil
		}
		measurement := "command passed"
		if gate.Category == "changed_file_scope" {
			measurement = changedFileScopeMeasurement(scope)
		}
		outcomes = append(outcomes, phase4GateOutcome{Category: gate.Category, Status: "passed", Command: gate.Command, Output: output, Measurement: measurement})
	}

	result := Phase4ReviewResult{
		Readiness:                "ready",
		Snapshot:                 plan.Snapshot,
		ConfigurationFingerprint: plan.ConfigurationFingerprint,
		PlanFingerprint:          plan.PlanFingerprint,
	}
	if err := persistPhase4ReviewEvidence(filepath.Join(repoRoot, ".rotta", "current", "review-evidence.yaml"), plan, outcomes, result.Readiness); err != nil {
		return Phase4ReviewResult{}, err
	}
	updatedState := setStateValue(state, "phase", "final_human_review")
	updatedState = setStateValue(updatedState, "reviewed_commit", plan.Snapshot)
	updatedState = setStateValue(updatedState, "overall_readiness", result.Readiness)
	updatedState = setStateValue(updatedState, "review_evidence", ".rotta/current/review-evidence.yaml")
	updatedState = setStateValue(updatedState, "configuration_fingerprint", plan.ConfigurationFingerprint)
	updatedState = setStateValue(updatedState, "plan_fingerprint", plan.PlanFingerprint)
	if err := os.WriteFile(statePath, updatedState, 0o600); err != nil {
		return Phase4ReviewResult{}, fmt.Errorf("persist final human review state: %w", err)
	}
	return result, nil
}

// DerivePRReadiness applies persisted durable waivers without changing review evidence.
func DerivePRReadiness(repoRoot, snapshot, configurationFingerprint string) (PRReadiness, error) {
	evidence, err := os.ReadFile(filepath.Join(repoRoot, ".rotta", "current", "review-evidence.yaml"))
	if err != nil {
		return PRReadiness{}, fmt.Errorf("read review evidence: %w", err)
	}
	if quotedStateValue(evidence, "snapshot") != snapshot || quotedStateValue(evidence, "configuration_fingerprint") != configurationFingerprint {
		return PRReadiness{}, fmt.Errorf("derive PR readiness: review evidence does not match requested snapshot and configuration")
	}
	waiverData, err := os.ReadFile(filepath.Join(repoRoot, ".rotta", "current", "waivers.yaml"))
	if err != nil {
		return PRReadiness{}, fmt.Errorf("read durable waivers: %w", err)
	}

	readiness := PRReadiness{Gates: persistedReviewGates(evidence), Waivers: persistedDurableWaivers(waiverData)}
	for index := range readiness.Gates {
		readiness.Gates[index].UnderlyingStatus = readiness.Gates[index].Status
		for _, waiver := range readiness.Waivers {
			if waiver.Gate == readiness.Gates[index].Category && waiver.Snapshot == snapshot && waiver.ConfigurationFingerprint == configurationFingerprint {
				readiness.Gates[index].Status = "waived"
			}
		}
	}

	allSatisfied, hasWaiver := true, false
	for _, gate := range readiness.Gates {
		if gate.Status != "passed" && gate.Status != "waived" {
			allSatisfied = false
		}
		if gate.Status == "waived" {
			hasWaiver = true
		}
	}
	if allSatisfied && hasWaiver {
		readiness.State = "ready_with_waivers"
	} else if allSatisfied {
		readiness.State = "ready"
	} else {
		readiness.State = "not_ready"
	}
	return readiness, nil
}

func quotedStateValue(data []byte, key string) string {
	return strings.Trim(stateValue(data, key), "\"")
}

func persistedReviewGates(evidence []byte) []PRReadinessGate {
	var gates []PRReadinessGate
	for _, line := range strings.Split(string(evidence), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "- category: "):
			gates = append(gates, PRReadinessGate{Category: strings.Trim(strings.TrimPrefix(trimmed, "- category: "), "\"")})
		case strings.HasPrefix(trimmed, "status: ") && len(gates) > 0:
			gates[len(gates)-1].Status = strings.Trim(strings.TrimPrefix(trimmed, "status: "), "\"")
		}
	}
	return gates
}

func persistedDurableWaivers(data []byte) []DurableWaiver {
	var waivers []DurableWaiver
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- gate: ") {
			waivers = append(waivers, DurableWaiver{Gate: strings.Trim(strings.TrimPrefix(trimmed, "- gate: "), "\"")})
			continue
		}
		if len(waivers) == 0 {
			continue
		}
		waiver := &waivers[len(waivers)-1]
		switch {
		case strings.HasPrefix(trimmed, "reason: "):
			waiver.Reason = strings.Trim(strings.TrimPrefix(trimmed, "reason: "), "\"")
		case strings.HasPrefix(trimmed, "scope: "):
			waiver.Scope = strings.Trim(strings.TrimPrefix(trimmed, "scope: "), "\"")
		case strings.HasPrefix(trimmed, "timestamp: "):
			waiver.Timestamp = strings.Trim(strings.TrimPrefix(trimmed, "timestamp: "), "\"")
		case strings.HasPrefix(trimmed, "snapshot: "):
			waiver.Snapshot = strings.Trim(strings.TrimPrefix(trimmed, "snapshot: "), "\"")
		case strings.HasPrefix(trimmed, "configuration_fingerprint: "):
			waiver.ConfigurationFingerprint = strings.Trim(strings.TrimPrefix(trimmed, "configuration_fingerprint: "), "\"")
		}
	}
	return waivers
}

func hasRequiredGenericGateOrder(gates []ResolvedQualityGate) bool {
	if len(gates) != len(requiredGenericGateCategories) {
		return false
	}
	for index, category := range requiredGenericGateCategories {
		if gates[index].Category != category || gates[index].Command == "" {
			return false
		}
	}
	return true
}

func validateCurrentTDDEvidence(repoRoot string, state []byte) error {
	evidence, err := os.ReadFile(filepath.Join(repoRoot, ".rotta", "current", "tdd-log.md"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			scenarioIDs := stateList(state, "completed_scenarios")
			if len(scenarioIDs) > 0 {
				return fmt.Errorf("evaluate Phase 4 review: current TDD evidence is missing %s", scenarioIDs[0])
			}
		}
		return fmt.Errorf("read current TDD evidence: %w", err)
	}
	for _, scenarioID := range stateList(state, "completed_scenarios") {
		if !strings.Contains(string(evidence), scenarioID) {
			return fmt.Errorf("evaluate Phase 4 review: current TDD evidence is missing %s", scenarioID)
		}
	}
	return nil
}

func validateCommittedReviewSnapshot(repoRoot, snapshot string) error {
	command := exec.Command("git", "rev-parse", "HEAD") // #nosec G204 -- Git binary and arguments are fixed.
	command.Dir = repoRoot
	head, err := command.Output()
	if err != nil || strings.TrimSpace(string(head)) != snapshot {
		return fmt.Errorf("evaluate Phase 4 review: recorded snapshot is not the current committed snapshot")
	}
	command = exec.Command("git", "cat-file", "-e", snapshot+"^{commit}") // #nosec G204 -- Git binary and command structure are fixed; snapshot came from recorded metadata.
	command.Dir = repoRoot
	if err := command.Run(); err != nil {
		return fmt.Errorf("evaluate Phase 4 review: recorded snapshot is not committed")
	}
	return nil
}

func changedFileScopeMeasurement(scope ChangedFileScope) string {
	paths := append([]string(nil), scope.Changed...)
	for _, rename := range scope.Renamed {
		paths = append(paths, rename.From+" -> "+rename.To)
	}
	paths = append(paths, scope.Deleted...)
	return strings.Join(paths, ",")
}

func persistPhase4ReviewEvidence(path string, plan Phase4ReviewPlan, outcomes []phase4GateOutcome, readiness string) error {
	var contents strings.Builder
	fmt.Fprintf(&contents, "format: rotta.review-evidence/v1\nbaseline: %q\nsnapshot: %q\nconfiguration_fingerprint: %q\nplan_fingerprint: %q\nevaluated_at: %q\noverall_readiness: %s\ngates:\n", plan.Baseline, plan.Snapshot, plan.ConfigurationFingerprint, plan.PlanFingerprint, time.Now().UTC().Format(time.RFC3339), readiness)
	for _, outcome := range outcomes {
		fmt.Fprintf(&contents, "  - category: %s\n    status: %s\n    command: %q\n    output: %q\n", outcome.Category, outcome.Status, outcome.Command, outcome.Output)
		if outcome.Status == "failed" {
			fmt.Fprintf(&contents, "    exit_result: %q\n    remediation: %q\n", outcome.ExitResult, outcome.Remediation)
			continue
		}
		fmt.Fprintf(&contents, "    exit_status: 0\n    measurement: %q\n", outcome.Measurement)
	}
	if err := os.WriteFile(path, []byte(contents.String()), 0o600); err != nil {
		return fmt.Errorf("persist review evidence: %w", err)
	}
	return nil
}

func stateValue(state []byte, key string) string {
	for _, line := range strings.Split(string(state), "\n") {
		if value, found := strings.CutPrefix(line, key+": "); found {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func stateList(state []byte, key string) []string {
	value := strings.Trim(stateValue(state, key), "[]")
	if value == "" {
		return nil
	}
	return strings.Split(value, ", ")
}

func setStateValue(state []byte, key, value string) []byte {
	lines := strings.Split(strings.TrimSuffix(string(state), "\n"), "\n")
	for index, line := range lines {
		if strings.HasPrefix(line, key+": ") {
			lines[index] = key + ": " + value
			return []byte(strings.Join(lines, "\n") + "\n")
		}
	}
	return []byte(strings.Join(append(lines, key+": "+value), "\n") + "\n")
}

func qualityGatesFormat(config []byte) string {
	for _, line := range strings.Split(string(config), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if ok && key == "format" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func supportsDeclaredConventionDiscovery(config []byte) bool {
	declaredMetadata := false
	declaredRule := false
	for _, line := range strings.Split(string(config), "\n") {
		switch strings.TrimSpace(line) {
		case "- declared_project_metadata":
			declaredMetadata = true
		case "- declared_convention_only":
			declaredRule = true
		}
	}
	return declaredMetadata && declaredRule
}

func hasResolvedSecurityCheck(gates []ResolvedQualityGate) bool {
	for _, gate := range gates {
		if gate.Category == "security_checks" && gate.Command != "" {
			return true
		}
	}
	return false
}

func declaredConventionGates(metadata []byte, metadataPath string) (string, string, []ResolvedQualityGate, error) {
	baseline, snapshot := recordedReviewSnapshotIdentities(metadata)
	var category string
	var staticAnalysisCommand string
	var gates []ResolvedQualityGate
	for _, line := range strings.Split(string(metadata), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") && strings.HasSuffix(trimmed, ":") && trimmed != "conventions:":
			category = strings.TrimSuffix(trimmed, ":")
		case strings.HasPrefix(line, "    command: "):
			command := strings.TrimSpace(strings.TrimPrefix(line, "    command: "))
			if category == "" || command == "" {
				return "", "", nil, fmt.Errorf("resolve review plan: recorded convention is incomplete")
			}
			if category == "static_analysis" {
				if staticAnalysisCommand != "" && staticAnalysisCommand != command {
					return "", "", nil, ambiguousStaticAnalysisConventionError{}
				}
				staticAnalysisCommand = command
			}
			gates = append(gates, ResolvedQualityGate{
				Category:       category,
				Command:        command,
				MetadataSource: metadataPath,
				DiscoveryRule:  "declared_convention_only",
			})
		}
	}
	if baseline == "" || snapshot == "" || len(gates) == 0 {
		return "", "", nil, fmt.Errorf("resolve review plan: recorded review snapshot has no declared conventions")
	}
	return baseline, snapshot, gates, nil
}

func recordedReviewSnapshotIdentities(metadata []byte) (string, string) {
	var baseline, snapshot string
	for _, line := range strings.Split(string(metadata), "\n") {
		switch {
		case strings.HasPrefix(line, "baseline: "):
			baseline = strings.TrimSpace(strings.TrimPrefix(line, "baseline: "))
		case strings.HasPrefix(line, "snapshot: "):
			snapshot = strings.TrimSpace(strings.TrimPrefix(line, "snapshot: "))
		}
	}
	return baseline, snapshot
}

func fingerprint(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum)
}

func planFingerprintInput(plan Phase4ReviewPlan) string {
	var input strings.Builder
	fmt.Fprintf(&input, "baseline=%q\nsnapshot=%q\nconfiguration=%q\n", plan.Baseline, plan.Snapshot, plan.ConfigurationFingerprint)
	for _, gate := range plan.Gates {
		fmt.Fprintf(&input, "category=%q\ncommand=%q\nmetadata_source=%q\ndiscovery_rule=%q\n", gate.Category, gate.Command, gate.MetadataSource, gate.DiscoveryRule)
	}
	return input.String()
}

func persistPhase4ReviewPlan(path string, plan Phase4ReviewPlan) error {
	var contents strings.Builder
	fmt.Fprintf(&contents, "format: rotta.phase4-review-plan/v1\nbaseline: %q\nsnapshot: %q\nconfiguration_fingerprint: %q\nplan_fingerprint: %q\ngates:\n", plan.Baseline, plan.Snapshot, plan.ConfigurationFingerprint, plan.PlanFingerprint)
	for _, gate := range plan.Gates {
		fmt.Fprintf(&contents, "  - category: %q\n    command: %q\n    metadata_source: %q\n    discovery_rule: %q\n", gate.Category, gate.Command, gate.MetadataSource, gate.DiscoveryRule)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create review plan directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(contents.String()), 0o600); err != nil {
		return fmt.Errorf("persist review plan: %w", err)
	}
	return nil
}

func persistChangedFileScope(path string, scope ChangedFileScope) error {
	var contents strings.Builder
	fmt.Fprintf(&contents, "format: rotta.changed-file-scope/v1\nbaseline: %q\nsnapshot: %q\nchanged:\n", scope.Baseline, scope.Snapshot)
	for _, path := range scope.Changed {
		fmt.Fprintf(&contents, "  - %q\n", path)
	}
	contents.WriteString("renamed:\n")
	for _, rename := range scope.Renamed {
		fmt.Fprintf(&contents, "  - from: %q\n    to: %q\n", rename.From, rename.To)
	}
	contents.WriteString("deleted:\n")
	for _, path := range scope.Deleted {
		fmt.Fprintf(&contents, "  - %q\n", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create changed-file scope directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(contents.String()), 0o600); err != nil {
		return fmt.Errorf("persist changed-file scope: %w", err)
	}
	return nil
}
