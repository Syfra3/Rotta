package workflow

import "fmt"

const (
	JevDecisionSchemaV1 = "rotta.jev-decision/v1"
	JevKitSchemaV1      = "rotta.jev-kit/v1"

	JevDirectRoutingThreshold            = 0.90
	JevCompletionGateThreshold           = 0.90
	JevModelArbitrationReservedThreshold = 0.85
)

type JevPrimitive string

const (
	JevPrimitiveChoice JevPrimitive = "choice"
	JevPrimitiveNoul   JevPrimitive = "noul"
	JevPrimitiveScore  JevPrimitive = "score"
)

type JevDecisionKind string

const (
	JevDecisionRouting        JevDecisionKind = "orchestrator_routing"
	JevDecisionCompletionGate JevDecisionKind = "completion_gate"
)

type JevRouteOption string

const (
	JevRouteImplementDirect JevRouteOption = "implement_direct"
	JevRoutePlanRigorous    JevRouteOption = "plan_rigorous"
	JevRouteExplore         JevRouteOption = "explore"
	JevRouteReview          JevRouteOption = "review"
)

type JevFallbackReason string

const (
	JevFallbackNone                   JevFallbackReason = "none"
	JevFallbackDisabled               JevFallbackReason = "jev_disabled"
	JevFallbackUnavailable            JevFallbackReason = "jev_unavailable"
	JevFallbackMalformed              JevFallbackReason = "malformed_result"
	JevFallbackLowConfidence          JevFallbackReason = "low_confidence"
	JevFallbackPolicyRequiresRigorous JevFallbackReason = "policy_requires_rigorous_path"
	JevFallbackDeterministicFailures  JevFallbackReason = "deterministic_acceptance_failures"
)

type JevHost string

const (
	JevHostNeutral    JevHost = "host-neutral"
	JevHostPi         JevHost = "pi"
	JevHostOpenCode   JevHost = "opencode"
	JevHostClaudeCode JevHost = "claude-code"
)

type JevConfig struct {
	Enabled                    bool
	Adapter                    string
	DirectRoutingThreshold     float64
	CompletionGateThreshold    float64
	AllowLiveTypeSafeByDefault bool
}

func DefaultJevConfig() JevConfig {
	return JevConfig{
		Enabled:                 false,
		Adapter:                 "disabled",
		DirectRoutingThreshold:  JevDirectRoutingThreshold,
		CompletionGateThreshold: JevCompletionGateThreshold,
	}
}

type JevCorrelation struct {
	WorkID   string  `json:"work_id"`
	Revision int     `json:"revision"`
	Host     JevHost `json:"host"`
}

type JevChoiceQuestion struct {
	SchemaVersion string            `json:"schema_version"`
	Kind          JevDecisionKind   `json:"kind"`
	Primitive     JevPrimitive      `json:"primitive"`
	Instructions  string            `json:"instructions"`
	State         map[string]string `json:"state"`
	Options       []JevRouteOption  `json:"options"`
	Correlation   JevCorrelation    `json:"correlation"`
}

type JevNoulQuestion struct {
	SchemaVersion string            `json:"schema_version"`
	Kind          JevDecisionKind   `json:"kind"`
	Primitive     JevPrimitive      `json:"primitive"`
	Instructions  string            `json:"instructions"`
	State         map[string]string `json:"state"`
	Correlation   JevCorrelation    `json:"correlation"`
}

type JevChoiceResult struct {
	Option      JevRouteOption `json:"option"`
	Probability float64        `json:"probability"`
}

type JevNoulResult struct {
	Value       bool    `json:"value"`
	Probability float64 `json:"probability"`
}

type JevAdapter interface {
	Choice(JevChoiceQuestion) (JevChoiceResult, error)
	Noul(JevNoulQuestion) (JevNoulResult, error)
}

type JevTelemetry struct {
	SchemaVersion  string            `json:"schema_version"`
	Kind           JevDecisionKind   `json:"kind"`
	Primitive      JevPrimitive      `json:"primitive"`
	Selected       string            `json:"selected"`
	Probability    float64           `json:"probability"`
	FallbackReason JevFallbackReason `json:"fallback_reason"`
	Correlation    JevCorrelation    `json:"correlation"`
}

type JevRoutingInput struct {
	Question                 JevChoiceQuestion
	StrictTriggered          bool
	OperationConsentRequired bool
	ReviewRequired           bool
}

type JevRoutingDecision struct {
	Route     JevRouteOption
	UsedJev   bool
	Telemetry JevTelemetry
}

type JevCompletionInput struct {
	Question                     JevNoulQuestion
	KnownDeterministicFailureIDs []string
}

type JevCompletionDecision struct {
	AcceptanceReady bool
	UsedJev         bool
	Telemetry       JevTelemetry
}

func NewRoutingQuestion(correlation JevCorrelation, objective, diffSummary, policySummary string) JevChoiceQuestion {
	return JevChoiceQuestion{
		SchemaVersion: JevDecisionSchemaV1,
		Kind:          JevDecisionRouting,
		Primitive:     JevPrimitiveChoice,
		Instructions:  "Choose the safest already-authorized Rotta next state. Do not approve work, bypass Strict triggers, or authorize operations.",
		State: map[string]string{
			"objective":      objective,
			"diff_summary":   diffSummary,
			"policy_summary": policySummary,
		},
		Options:     []JevRouteOption{JevRouteImplementDirect, JevRoutePlanRigorous, JevRouteExplore, JevRouteReview},
		Correlation: correlation,
	}
}

func NewCompletionGateQuestion(correlation JevCorrelation, acceptanceCriteria, evidenceSummary string) JevNoulQuestion {
	return JevNoulQuestion{
		SchemaVersion: JevDecisionSchemaV1,
		Kind:          JevDecisionCompletionGate,
		Primitive:     JevPrimitiveNoul,
		Instructions:  "Return true only when current evidence satisfies the acceptance criteria. Known deterministic failures are handled by code and must not be overridden.",
		State: map[string]string{
			"acceptance_criteria": acceptanceCriteria,
			"evidence_summary":    evidenceSummary,
		},
		Correlation: correlation,
	}
}

func DecideJevRouting(config JevConfig, adapter JevAdapter, input JevRoutingInput) JevRoutingDecision {
	threshold := thresholdOrDefault(config.DirectRoutingThreshold, JevDirectRoutingThreshold)
	base := telemetry(input.Question.Correlation, JevDecisionRouting, JevPrimitiveChoice)
	fallback := func(reason JevFallbackReason) JevRoutingDecision {
		base.FallbackReason = reason
		base.Selected = string(JevRoutePlanRigorous)
		return JevRoutingDecision{Route: JevRoutePlanRigorous, Telemetry: base}
	}
	if !config.Enabled {
		return fallback(JevFallbackDisabled)
	}
	if adapter == nil {
		return fallback(JevFallbackUnavailable)
	}
	if input.StrictTriggered || input.OperationConsentRequired || input.ReviewRequired {
		return fallback(JevFallbackPolicyRequiresRigorous)
	}
	result, err := adapter.Choice(input.Question)
	if err != nil || !validProbability(result.Probability) || !routeAllowed(result.Option, input.Question.Options) {
		return fallback(JevFallbackMalformed)
	}
	base.Selected = string(result.Option)
	base.Probability = result.Probability
	if result.Probability < threshold {
		base.FallbackReason = JevFallbackLowConfidence
		return JevRoutingDecision{Route: JevRoutePlanRigorous, Telemetry: base}
	}
	base.FallbackReason = JevFallbackNone
	return JevRoutingDecision{Route: result.Option, UsedJev: true, Telemetry: base}
}

func DecideJevCompletion(config JevConfig, adapter JevAdapter, input JevCompletionInput) JevCompletionDecision {
	threshold := thresholdOrDefault(config.CompletionGateThreshold, JevCompletionGateThreshold)
	base := telemetry(input.Question.Correlation, JevDecisionCompletionGate, JevPrimitiveNoul)
	fallback := func(reason JevFallbackReason) JevCompletionDecision {
		base.FallbackReason = reason
		base.Selected = "false"
		return JevCompletionDecision{AcceptanceReady: false, Telemetry: base}
	}
	if len(input.KnownDeterministicFailureIDs) > 0 {
		return fallback(JevFallbackDeterministicFailures)
	}
	if !config.Enabled {
		return fallback(JevFallbackDisabled)
	}
	if adapter == nil {
		return fallback(JevFallbackUnavailable)
	}
	result, err := adapter.Noul(input.Question)
	if err != nil || !validProbability(result.Probability) {
		return fallback(JevFallbackMalformed)
	}
	base.Selected = fmt.Sprintf("%t", result.Value)
	base.Probability = result.Probability
	if !result.Value || result.Probability < threshold {
		base.FallbackReason = JevFallbackLowConfidence
		return JevCompletionDecision{AcceptanceReady: false, Telemetry: base}
	}
	base.FallbackReason = JevFallbackNone
	return JevCompletionDecision{AcceptanceReady: true, UsedJev: true, Telemetry: base}
}

type JevKitSpec struct {
	SchemaVersion string                     `json:"schema_version"`
	Hosts         []JevHost                  `json:"hosts"`
	Primitives    []JevPrimitive             `json:"primitives"`
	Decisions     []JevDecisionKind          `json:"decisions"`
	Thresholds    map[string]float64         `json:"thresholds"`
	Instructions  map[JevDecisionKind]string `json:"instructions"`
}

func CanonicalJevKitSpec() JevKitSpec {
	return JevKitSpec{
		SchemaVersion: JevKitSchemaV1,
		Hosts:         []JevHost{JevHostNeutral, JevHostPi, JevHostOpenCode, JevHostClaudeCode},
		Primitives:    []JevPrimitive{JevPrimitiveChoice, JevPrimitiveNoul, JevPrimitiveScore},
		Decisions:     []JevDecisionKind{JevDecisionRouting, JevDecisionCompletionGate},
		Thresholds: map[string]float64{
			"direct_routing":             JevDirectRoutingThreshold,
			"completion_gate":            JevCompletionGateThreshold,
			"model_arbitration_reserved": JevModelArbitrationReservedThreshold,
		},
		Instructions: map[JevDecisionKind]string{
			JevDecisionRouting:        "Choice: select one already-authorized next state; confidence is a routing signal, never authorization.",
			JevDecisionCompletionGate: "Noul: judge whether acceptance criteria are satisfied by current evidence; code blocks known deterministic failures.",
		},
	}
}

func telemetry(correlation JevCorrelation, kind JevDecisionKind, primitive JevPrimitive) JevTelemetry {
	return JevTelemetry{SchemaVersion: JevDecisionSchemaV1, Kind: kind, Primitive: primitive, Correlation: correlation}
}

func thresholdOrDefault(value, fallback float64) float64 {
	if value <= 0 || value > 1 {
		return fallback
	}
	return value
}

func validProbability(value float64) bool { return value >= 0 && value <= 1 }

func routeAllowed(route JevRouteOption, options []JevRouteOption) bool {
	for _, option := range options {
		if route == option {
			return true
		}
	}
	return false
}
