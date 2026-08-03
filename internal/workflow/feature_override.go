package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// OverrideTarget identifies the eligible item to which a displayed override is
// bound. A gate target must name its persisted non-passing outcome; a process
// target is eligible only when the displayed action says so.
type OverrideTarget struct {
	PersistedGateOutcomePath string
	EligibleProcessRule      bool
}

// DisplayedOverrideActionInput is the complete, human-visible binding for one
// possible feature-local override.
type DisplayedOverrideActionInput struct {
	AuthorizationActionID string
	FeatureID             string
	RuleID                string
	Operation             string
	Baseline              string
	ContractFingerprint   string
	Reason                string
	ExpiresAt             time.Time
	Target                OverrideTarget
}

// DisplayedOverrideAction represents the sole override action shown to the
// human. It intentionally carries no actor identity.
type DisplayedOverrideAction struct {
	input DisplayedOverrideActionInput
}

// FeatureLocalOverride is the persisted, actor-less evidence record for one
// authorized operation.
type FeatureLocalOverride struct {
	Path                  string
	AuthorizationActionID string
	FeatureID             string
	RuleID                string
	Operation             string
	Baseline              string
	ContractFingerprint   string
	Reason                string
	ExpiresAt             time.Time
	Target                string
	TargetReference       string
	UsesRemaining         int
	Status                string
}

func NewDisplayedOverrideAction(input DisplayedOverrideActionInput) *DisplayedOverrideAction {
	return &DisplayedOverrideAction{input: input}
}

// Authorize writes the displayed action as one actor-less, feature-local
// override after the human acknowledges it.
func (action *DisplayedOverrideAction) Authorize(repoRoot, acknowledgement string) (FeatureLocalOverride, error) {
	if !isCompactAcknowledgement(acknowledgement) {
		return FeatureLocalOverride{}, fmt.Errorf("override authorization requires an exact acknowledgement token")
	}

	target, reference, err := action.target(repoRoot)
	if err != nil {
		return FeatureLocalOverride{}, err
	}
	override := FeatureLocalOverride{
		AuthorizationActionID: action.input.AuthorizationActionID,
		FeatureID:             action.input.FeatureID,
		RuleID:                action.input.RuleID,
		Operation:             action.input.Operation,
		Baseline:              action.input.Baseline,
		ContractFingerprint:   action.input.ContractFingerprint,
		Reason:                action.input.Reason,
		ExpiresAt:             action.input.ExpiresAt,
		Target:                target,
		TargetReference:       reference,
		UsesRemaining:         1,
		Status:                "active",
	}
	override.Path = filepath.Join(repoRoot, ".rotta", "current", "overrides", action.input.AuthorizationActionID+".yaml")
	if err := writeFeatureLocalOverride(override); err != nil {
		return FeatureLocalOverride{}, err
	}
	return override, nil
}

// ApplyFeatureLocalOverride consumes a persisted override only for the
// operation it names. The consumed record remains at its evidence path.
func ApplyFeatureLocalOverride(path, operation string) (bool, error) {
	override, err := readFeatureLocalOverride(path)
	if err != nil {
		return false, err
	}
	if override.Operation != operation || override.UsesRemaining == 0 {
		return false, nil
	}
	override.UsesRemaining--
	override.Status = "consumed"
	if err := writeFeatureLocalOverride(override); err != nil {
		return false, err
	}
	return true, nil
}

func (action *DisplayedOverrideAction) target(repoRoot string) (string, string, error) {
	if action.input.Target.PersistedGateOutcomePath != "" {
		contents, err := readRepositoryFile(repoRoot, action.input.Target.PersistedGateOutcomePath)
		if err != nil {
			return "", "", fmt.Errorf("read persisted gate outcome: %w", err)
		}
		outcome := overrideFields(string(contents))
		if outcome["feature_id"] != action.input.FeatureID || outcome["rule_id"] != action.input.RuleID || outcome["baseline"] != action.input.Baseline || outcome["contract_fingerprint"] != action.input.ContractFingerprint || outcome["status"] == "passed" {
			return "", "", fmt.Errorf("persisted gate outcome does not match displayed override")
		}
		return "persisted_gate_outcome", action.input.Target.PersistedGateOutcomePath, nil
	}
	if action.input.Target.EligibleProcessRule {
		return "eligible_process_rule", action.input.RuleID, nil
	}
	return "", "", fmt.Errorf("displayed override has no eligible target")
}

func readFeatureLocalOverride(path string) (FeatureLocalOverride, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return FeatureLocalOverride{}, fmt.Errorf("read feature-local override: %w", err)
	}
	fields := overrideFields(string(contents))
	expiresAt, err := time.Parse(time.RFC3339, fields["expires_at"])
	if err != nil {
		return FeatureLocalOverride{}, fmt.Errorf("parse feature-local override expiry: %w", err)
	}
	var usesRemaining int
	if _, err := fmt.Sscanf(fields["uses_remaining"], "%d", &usesRemaining); err != nil {
		return FeatureLocalOverride{}, fmt.Errorf("parse feature-local override uses: %w", err)
	}
	return FeatureLocalOverride{
		Path:                  path,
		AuthorizationActionID: fields["authorization_action_id"],
		FeatureID:             fields["feature_id"],
		RuleID:                fields["rule_id"],
		Operation:             fields["operation"],
		Baseline:              fields["baseline"],
		ContractFingerprint:   fields["contract_fingerprint"],
		Reason:                fields["reason"],
		ExpiresAt:             expiresAt,
		Target:                fields["target"],
		TargetReference:       fields["target_reference"],
		UsesRemaining:         usesRemaining,
		Status:                fields["status"],
	}, nil
}

func writeFeatureLocalOverride(override FeatureLocalOverride) error {
	if err := os.MkdirAll(filepath.Dir(override.Path), 0o700); err != nil {
		return fmt.Errorf("create feature-local override directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(override.Path), ".override-*")
	if err != nil {
		return fmt.Errorf("create feature-local override: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("secure feature-local override: %w", err)
	}
	if _, err := temporary.WriteString(serializeFeatureLocalOverride(override)); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write feature-local override: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close feature-local override: %w", err)
	}
	if err := os.Rename(temporaryPath, override.Path); err != nil {
		return fmt.Errorf("persist feature-local override: %w", err)
	}
	return nil
}

func serializeFeatureLocalOverride(override FeatureLocalOverride) string {
	return fmt.Sprintf("format: rotta.feature-override/v1\nauthorization_action_id: %s\nfeature_id: %s\nrule_id: %s\noperation: %s\nbaseline: %s\ncontract_fingerprint: %s\nreason: %s\nexpires_at: %s\ntarget: %s\ntarget_reference: %s\nuses_remaining: %d\nstatus: %s\n",
		override.AuthorizationActionID,
		override.FeatureID,
		override.RuleID,
		override.Operation,
		override.Baseline,
		override.ContractFingerprint,
		override.Reason,
		override.ExpiresAt.UTC().Format(time.RFC3339),
		override.Target,
		override.TargetReference,
		override.UsesRemaining,
		override.Status,
	)
}

func overrideFields(contents string) map[string]string {
	fields := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSuffix(contents, "\n"), "\n") {
		name, value, ok := strings.Cut(line, ": ")
		if ok {
			fields[name] = value
		}
	}
	return fields
}
