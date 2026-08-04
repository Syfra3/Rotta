package workflow

import (
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

func qualityGatesFormat(config []byte) string {
	for _, line := range strings.Split(string(config), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if ok && key == "format" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
