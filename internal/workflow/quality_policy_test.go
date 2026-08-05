package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQualityPolicyLoadsCompleteCanonicalPolicy(t *testing.T) {
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, QualityPolicyPath), qualityPolicyContents(t))
	policy, fingerprint, err := LoadQualityPolicy(repo)
	if err != nil {
		t.Fatalf("LoadQualityPolicy() error = %v", err)
	}
	if policy.Format != "rotta-quality-policy/v1" || len(policy.Dimensions) != 10 || !strings.HasPrefix(fingerprint, "sha256:") {
		t.Fatalf("loaded policy = %#v, fingerprint %q", policy, fingerprint)
	}
}

func TestQualityPolicyRejectsIncompleteOrUnknownDimensions(t *testing.T) {
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, QualityPolicyPath), `{"format":"rotta-quality-policy/v1","id":"default","version":"1","dimensions":[{"id":"unknown","required":true}]}`)
	_, _, err := LoadQualityPolicy(repo)
	if err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("LoadQualityPolicy() error = %v, want invalid policy", err)
	}
}

func qualityPolicyContents(t *testing.T) string {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join("..", "..", QualityPolicyPath))
	if err != nil {
		t.Fatalf("read committed test quality policy: %v", err)
	}
	return string(contents)
}
