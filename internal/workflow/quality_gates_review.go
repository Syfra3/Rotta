package workflow

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
