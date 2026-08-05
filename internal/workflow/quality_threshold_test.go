package workflow

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestQualityMetricThresholdCannotPassWhenBreached(t *testing.T) {
	policy, dimensions := SeverityTestInputs()
	policy.Dimensions[9].Thresholds = []QualityMetricThreshold{{MetricID: "line_coverage", Comparator: "gte", Target: json.RawMessage("80"), Unit: "percent"}}
	dimensions[9].Metrics = []QualityMetric{{ID: "line_coverage", Value: float64(70), Unit: "percent"}}
	_, err := validateQualityDimensions(dimensions, policy, nil)
	if err == nil || !strings.Contains(err.Error(), "breached metric") {
		t.Fatalf("validateQualityDimensions() error = %v", err)
	}
}
