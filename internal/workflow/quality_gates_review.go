package workflow

import (
	"crypto/sha256"
	"fmt"
	"os"
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

// RequestPhase4Review rejects unsupported quality-gates configurations before command execution.
func RequestPhase4Review(repoRoot string, execute func(string) error) (QualityGatesReview, error) {
	configPath := filepath.Join(repoRoot, ".rotta", "quality-gates.yaml")
	config, err := os.ReadFile(configPath)
	if err != nil {
		return QualityGatesReview{}, fmt.Errorf("read quality-gates configuration: %w", err)
	}
	if qualityGatesFormat(config) == "rotta.quality-gates/v1" {
		return QualityGatesReview{
			State:  QualityGatesReviewBlocked,
			Result: "quality-gates v1 is unsupported and is not automatically migrated; replace .rotta/quality-gates.yaml with the generated rotta.quality-gates/v2 configuration before requesting Phase 4 review",
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

func declaredConventionGates(metadata []byte, metadataPath string) (string, string, []ResolvedQualityGate, error) {
	var baseline, snapshot, category string
	var gates []ResolvedQualityGate
	for _, line := range strings.Split(string(metadata), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "baseline: "):
			baseline = strings.TrimSpace(strings.TrimPrefix(line, "baseline: "))
		case strings.HasPrefix(line, "snapshot: "):
			snapshot = strings.TrimSpace(strings.TrimPrefix(line, "snapshot: "))
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
