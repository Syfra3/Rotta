package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestV2QualityPolicyLoadsCompleteCanonicalPolicy(t *testing.T) {
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, v2QualityPolicyPath), v2TestQualityPolicyContents(t))
	policy, fingerprint, err := LoadV2QualityPolicy(repo)
	if err != nil {
		t.Fatalf("LoadV2QualityPolicy() error = %v", err)
	}
	if policy.Format != "rotta-quality-policy/v1" || len(policy.Dimensions) != 10 || !strings.HasPrefix(fingerprint, "sha256:") {
		t.Fatalf("loaded policy = %#v, fingerprint %q", policy, fingerprint)
	}
}

func TestV2QualityPolicyRejectsIncompleteOrUnknownDimensions(t *testing.T) {
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, v2QualityPolicyPath), `{"format":"rotta-quality-policy/v1","id":"default","version":"1","dimensions":[{"id":"unknown","required":true}]}`)
	_, _, err := LoadV2QualityPolicy(repo)
	if err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("LoadV2QualityPolicy() error = %v, want invalid policy", err)
	}
}

func v2TestQualityPolicyContents(t *testing.T) string {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join("..", "..", v2QualityPolicyPath))
	if err != nil {
		t.Fatalf("read committed test quality policy: %v", err)
	}
	return string(contents)
}
