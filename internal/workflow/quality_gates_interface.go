package workflow

import (
	"fmt"
	"strings"
)

const (
	qualityGatesPolicyV2Format = "rotta.quality-gates/v2"
	currentReviewEvidencePath  = ".rotta/current/review-evidence.yaml"
)

// requireQualityGatesInterface verifies only that the separately owned v2
// interface artifacts are present. Gate semantics remain outside this package.
func requireQualityGatesInterface(repoRoot string) error {
	policy, err := readRepositoryFile(repoRoot, featureWorkflowPolicyPath)
	if err != nil || qualityGatesPolicyFormat(policy) != qualityGatesPolicyV2Format {
		return fmt.Errorf("quality-gates interface remediation: provide %s at %s", qualityGatesPolicyV2Format, featureWorkflowPolicyPath)
	}
	if _, err := readRepositoryFile(repoRoot, currentReviewEvidencePath); err != nil {
		return fmt.Errorf("quality-gates interface remediation: provide current review evidence at %s", currentReviewEvidencePath)
	}
	return nil
}

func qualityGatesPolicyFormat(contents []byte) string {
	for _, line := range strings.Split(string(contents), "\n") {
		if format, ok := strings.CutPrefix(line, "format: "); ok {
			return format
		}
	}
	return ""
}
