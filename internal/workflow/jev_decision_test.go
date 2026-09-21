package workflow

import (
	"errors"
	"testing"
)

type fakeJevAdapter struct {
	choice JevChoiceResult
	noul   JevNoulResult
	err    error
}

func (f fakeJevAdapter) Choice(JevChoiceQuestion) (JevChoiceResult, error) { return f.choice, f.err }
func (f fakeJevAdapter) Noul(JevNoulQuestion) (JevNoulResult, error)       { return f.noul, f.err }

func TestJevDisabledPreservesRigorousRoutingFallback(t *testing.T) {
	question := NewRoutingQuestion(testJevCorrelation(), "fix typo", "no diff", "fast allowed")
	decision := DecideJevRouting(DefaultJevConfig(), fakeJevAdapter{choice: JevChoiceResult{Option: JevRouteImplementDirect, Probability: 0.99}}, JevRoutingInput{Question: question})
	if decision.Route != JevRoutePlanRigorous || decision.UsedJev {
		t.Fatalf("disabled Jev must preserve rigorous fallback, got route=%s used=%t", decision.Route, decision.UsedJev)
	}
	if decision.Telemetry.FallbackReason != JevFallbackDisabled || decision.Telemetry.Correlation.WorkID != "work-1" {
		t.Fatalf("unexpected telemetry: %+v", decision.Telemetry)
	}
}

func TestHighConfidenceRoutingCannotBypassPolicyTriggers(t *testing.T) {
	config := DefaultJevConfig()
	config.Enabled = true
	question := NewRoutingQuestion(testJevCorrelation(), "change auth", "auth diff", "strict required")
	adapter := fakeJevAdapter{choice: JevChoiceResult{Option: JevRouteImplementDirect, Probability: 0.99}}
	decision := DecideJevRouting(config, adapter, JevRoutingInput{Question: question, StrictTriggered: true})
	if decision.Route != JevRoutePlanRigorous || decision.UsedJev {
		t.Fatalf("policy trigger must force rigorous path, got route=%s used=%t", decision.Route, decision.UsedJev)
	}
	if decision.Telemetry.FallbackReason != JevFallbackPolicyRequiresRigorous {
		t.Fatalf("expected policy fallback, got %s", decision.Telemetry.FallbackReason)
	}
}

func TestRoutingThresholdBoundary(t *testing.T) {
	config := DefaultJevConfig()
	config.Enabled = true
	question := NewRoutingQuestion(testJevCorrelation(), "small docs", "docs only", "fast allowed")

	below := DecideJevRouting(config, fakeJevAdapter{choice: JevChoiceResult{Option: JevRouteImplementDirect, Probability: 0.899}}, JevRoutingInput{Question: question})
	if below.Route != JevRoutePlanRigorous || below.UsedJev || below.Telemetry.FallbackReason != JevFallbackLowConfidence {
		t.Fatalf("p<0.90 must fall back, got %+v", below)
	}

	at := DecideJevRouting(config, fakeJevAdapter{choice: JevChoiceResult{Option: JevRouteImplementDirect, Probability: 0.90}}, JevRoutingInput{Question: question})
	if at.Route != JevRouteImplementDirect || !at.UsedJev || at.Telemetry.FallbackReason != JevFallbackNone {
		t.Fatalf("p>=0.90 should use selected authorized route, got %+v", at)
	}
}

func TestMalformedOrUnavailableRoutingFallsBackWithTelemetry(t *testing.T) {
	config := DefaultJevConfig()
	config.Enabled = true
	question := NewRoutingQuestion(testJevCorrelation(), "task", "diff", "policy")

	unavailable := DecideJevRouting(config, nil, JevRoutingInput{Question: question})
	if unavailable.Telemetry.FallbackReason != JevFallbackUnavailable {
		t.Fatalf("expected unavailable fallback, got %+v", unavailable.Telemetry)
	}

	malformed := DecideJevRouting(config, fakeJevAdapter{choice: JevChoiceResult{Option: "not-an-option", Probability: 0.99}}, JevRoutingInput{Question: question})
	if malformed.Telemetry.FallbackReason != JevFallbackMalformed {
		t.Fatalf("expected malformed fallback for unknown option, got %+v", malformed.Telemetry)
	}

	errorResult := DecideJevRouting(config, fakeJevAdapter{err: errors.New("service down")}, JevRoutingInput{Question: question})
	if errorResult.Telemetry.FallbackReason != JevFallbackMalformed {
		t.Fatalf("expected malformed fallback for adapter error, got %+v", errorResult.Telemetry)
	}
}

func TestCompletionGateBlocksKnownDeterministicFailures(t *testing.T) {
	config := DefaultJevConfig()
	config.Enabled = true
	question := NewCompletionGateQuestion(testJevCorrelation(), "AC1 passes", "focused tests green but AC2 failed")
	adapter := fakeJevAdapter{noul: JevNoulResult{Value: true, Probability: 0.99}}
	decision := DecideJevCompletion(config, adapter, JevCompletionInput{Question: question, KnownDeterministicFailureIDs: []string{"AC2"}})
	if decision.AcceptanceReady || decision.UsedJev {
		t.Fatalf("known deterministic failures must block acceptance readiness, got %+v", decision)
	}
	if decision.Telemetry.FallbackReason != JevFallbackDeterministicFailures {
		t.Fatalf("expected deterministic failure telemetry, got %+v", decision.Telemetry)
	}
}

func TestCompletionGateThresholdAndDisabledBehavior(t *testing.T) {
	question := NewCompletionGateQuestion(testJevCorrelation(), "all ACs", "evidence")
	disabled := DecideJevCompletion(DefaultJevConfig(), fakeJevAdapter{noul: JevNoulResult{Value: true, Probability: 0.99}}, JevCompletionInput{Question: question})
	if disabled.AcceptanceReady || disabled.UsedJev || disabled.Telemetry.FallbackReason != JevFallbackDisabled {
		t.Fatalf("disabled completion must not mark ready, got %+v", disabled)
	}

	config := DefaultJevConfig()
	config.Enabled = true
	low := DecideJevCompletion(config, fakeJevAdapter{noul: JevNoulResult{Value: true, Probability: 0.89}}, JevCompletionInput{Question: question})
	if low.AcceptanceReady || low.UsedJev || low.Telemetry.FallbackReason != JevFallbackLowConfidence {
		t.Fatalf("low confidence true must not mark ready, got %+v", low)
	}

	high := DecideJevCompletion(config, fakeJevAdapter{noul: JevNoulResult{Value: true, Probability: 0.90}}, JevCompletionInput{Question: question})
	if !high.AcceptanceReady || !high.UsedJev || high.Telemetry.FallbackReason != JevFallbackNone {
		t.Fatalf("high confidence true should mark ready, got %+v", high)
	}
}

func TestCanonicalJevKitSpecIsHostNeutralAndReservesLaterThresholds(t *testing.T) {
	spec := CanonicalJevKitSpec()
	if spec.SchemaVersion != JevKitSchemaV1 {
		t.Fatalf("unexpected schema %q", spec.SchemaVersion)
	}
	for _, host := range []JevHost{JevHostPi, JevHostOpenCode, JevHostClaudeCode} {
		if !containsHost(spec.Hosts, host) {
			t.Fatalf("canonical kit missing host %s", host)
		}
	}
	if spec.Thresholds["direct_routing"] != 0.90 || spec.Thresholds["model_arbitration_reserved"] != 0.85 {
		t.Fatalf("unexpected thresholds: %+v", spec.Thresholds)
	}
	if len(spec.Instructions) != 2 || spec.Instructions[JevDecisionRouting] == "" || spec.Instructions[JevDecisionCompletionGate] == "" {
		t.Fatalf("canonical instructions must own decision semantics, got %+v", spec.Instructions)
	}
}

func TestDefaultJevConfigRequiresExplicitEnablementAndNoLiveCredentials(t *testing.T) {
	config := DefaultJevConfig()
	if config.Enabled || config.AllowLiveTypeSafeByDefault {
		t.Fatalf("Jev and live TypeSafe calls must be disabled by default: %+v", config)
	}
	if config.DirectRoutingThreshold != JevDirectRoutingThreshold || config.CompletionGateThreshold != JevCompletionGateThreshold {
		t.Fatalf("unexpected default thresholds: %+v", config)
	}
}

func testJevCorrelation() JevCorrelation {
	return JevCorrelation{WorkID: "work-1", Revision: 7, Host: JevHostNeutral}
}

func containsHost(hosts []JevHost, target JevHost) bool {
	for _, host := range hosts {
		if host == target {
			return true
		}
	}
	return false
}
