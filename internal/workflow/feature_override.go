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
	PolicyFingerprint     string
	EvidenceFingerprint   string
	Scope                 string
	Reason                string
	ExpiresAt             time.Time
	Target                string
	TargetReference       string
	UsesRemaining         int
	Status                string
}

// OverrideEvaluationContext is the current binding against which a persisted
// override is evaluated for one named operation.
type OverrideEvaluationContext struct {
	Operation           string
	Baseline            string
	ContractFingerprint string
	PolicyFingerprint   string
	EvidenceFingerprint string
	Scope               string
	Now                 time.Time
}

// OverrideEvaluation reports whether the named operation received the
// override and, when it did not, the recovery action available to the human.
type OverrideEvaluation struct {
	Applied     bool
	Remediation string
}

// NonWaivableIntegritySafeguard identifies integrity conditions that an
// override can never authorize.
type NonWaivableIntegritySafeguard string

const (
	NonWaivableManifestOrApprovalAuthority NonWaivableIntegritySafeguard = "manifest_or_approval_authority"
	NonWaivableCleanupTarget               NonWaivableIntegritySafeguard = "cleanup_target"
	NonWaivableWorktreeIdentity            NonWaivableIntegritySafeguard = "worktree_identity"
)

// NonWaivableOverrideRequest identifies preserved feature paths while a
// requested override is refused for an integrity safeguard.
type NonWaivableOverrideRequest struct {
	Safeguard      NonWaivableIntegritySafeguard
	PreservedPaths []string
}

// NonWaivableOverrideRefusal gives the failed invariant and a human-directed
// recovery alternative. It never performs cleanup.
type NonWaivableOverrideRefusal struct {
	Refused         bool
	FailedInvariant string
	PreservedPaths  []string
	Recovery        string
}

// RefuseNonWaivableOverride refuses integrity-bypassing overrides without
// changing any preserved path or performing destructive cleanup.
func RefuseNonWaivableOverride(request NonWaivableOverrideRequest) NonWaivableOverrideRefusal {
	refusal := NonWaivableOverrideRefusal{
		Refused:        true,
		PreservedPaths: append([]string(nil), request.PreservedPaths...),
	}
	switch request.Safeguard {
	case NonWaivableManifestOrApprovalAuthority:
		refusal.FailedInvariant = "manifest or approval authority is malformed or inconsistent"
		refusal.Recovery = "repair the manifest or approval authority, then resume from the preserved feature worktree"
	case NonWaivableCleanupTarget:
		refusal.FailedInvariant = "cleanup target is unknown or destructive"
		refusal.Recovery = "handoff the preserved paths and verify target ownership before any cleanup"
	case NonWaivableWorktreeIdentity:
		refusal.FailedInvariant = "recorded worktree identity is missing or incorrect"
		refusal.Recovery = "repair the recorded worktree identity or use a verified terminal archive after ownership is proven"
	default:
		refusal.FailedInvariant = "integrity safeguard is unrecognized"
		refusal.Recovery = "handoff the preserved paths for human verification before retrying"
	}
	return refusal
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

// EvaluateFeatureLocalOverride rejects expired, consumed, malformed, drifted,
// or competing overrides without changing the underlying gate or process
// outcome. Only a fully matching override reaches the existing atomic consume
// path.
func EvaluateFeatureLocalOverride(path string, context OverrideEvaluationContext) (OverrideEvaluation, error) {
	override, err := readFeatureLocalOverride(path)
	if err != nil {
		return rejectedOverrideEvaluation(), nil
	}
	if !override.matchesEvaluationContext(context) {
		return rejectedOverrideEvaluation(), nil
	}
	competing, err := hasCompetingFeatureLocalOverride(path, context.Operation)
	if err != nil {
		return OverrideEvaluation{}, err
	}
	if competing {
		return rejectedOverrideEvaluation(), nil
	}

	applied, err := ApplyFeatureLocalOverride(path, context.Operation)
	if err != nil {
		return OverrideEvaluation{}, err
	}
	return OverrideEvaluation{Applied: applied}, nil
}

func rejectedOverrideEvaluation() OverrideEvaluation {
	return OverrideEvaluation{Remediation: "remediation: repair or re-authorize one current, scoped feature-local override before retrying the operation"}
}

func (override FeatureLocalOverride) matchesEvaluationContext(context OverrideEvaluationContext) bool {
	return override.Operation == context.Operation &&
		override.UsesRemaining == 1 &&
		override.Status == "active" &&
		strings.TrimSpace(override.Reason) != "" &&
		override.ExpiresAt.After(context.Now) &&
		override.Baseline == context.Baseline &&
		override.ContractFingerprint == context.ContractFingerprint &&
		override.PolicyFingerprint == context.PolicyFingerprint &&
		override.EvidenceFingerprint == context.EvidenceFingerprint &&
		override.Scope == context.Scope
}

func hasCompetingFeatureLocalOverride(path, operation string) (bool, error) {
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		return false, fmt.Errorf("read feature-local overrides: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		competingPath := filepath.Join(filepath.Dir(path), entry.Name())
		if competingPath == path {
			continue
		}
		competing, err := readFeatureLocalOverride(competingPath)
		if err != nil {
			return true, nil
		}
		if competing.Operation == operation && competing.Status == "active" && competing.UsesRemaining > 0 {
			return true, nil
		}
	}
	return false, nil
}

func (action *DisplayedOverrideAction) target(repoRoot string) (string, string, error) {
	if action.input.Target.PersistedGateOutcomePath != "" {
		if err := requireQualityGatesInterface(repoRoot); err != nil {
			return "", "", err
		}
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
		PolicyFingerprint:     fields["policy_fingerprint"],
		EvidenceFingerprint:   fields["evidence_fingerprint"],
		Scope:                 fields["scope"],
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
	return fmt.Sprintf("format: rotta.feature-override/v1\nauthorization_action_id: %s\nfeature_id: %s\nrule_id: %s\noperation: %s\nbaseline: %s\ncontract_fingerprint: %s\npolicy_fingerprint: %s\nevidence_fingerprint: %s\nscope: %s\nreason: %s\nexpires_at: %s\ntarget: %s\ntarget_reference: %s\nuses_remaining: %d\nstatus: %s\n",
		override.AuthorizationActionID,
		override.FeatureID,
		override.RuleID,
		override.Operation,
		override.Baseline,
		override.ContractFingerprint,
		override.PolicyFingerprint,
		override.EvidenceFingerprint,
		override.Scope,
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
