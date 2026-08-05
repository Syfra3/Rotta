package workflow

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestV2QualityMetricThresholdCannotPassWhenBreached(t *testing.T) {
	policy, dimensions := v2SeverityTestInputs()
	policy.Dimensions[9].Thresholds = []V2QualityMetricThreshold{{MetricID: "line_coverage", Comparator: "gte", Target: json.RawMessage("80"), Unit: "percent"}}
	dimensions[9].Metrics = []V2QualityMetric{{ID: "line_coverage", Value: float64(70), Unit: "percent"}}
	_, err := validateV2QualityDimensions(dimensions, policy, nil)
	if err == nil || !strings.Contains(err.Error(), "breached metric") {
		t.Fatalf("validateV2QualityDimensions() error = %v", err)
	}
}
