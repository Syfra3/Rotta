//go:build legacy_v2

package installer

import (
	"os"
	"strings"
	"testing"
)

// Kept for unrelated legacy installer tests that shared Vela-guard helpers.
func assertStringListContains(t *testing.T, values []string, want string) {
	t.Helper()
	for _, value := range values {
		if value == want {
			return
		}
	}
	t.Fatalf("expected list to contain %q, got %#v", want, values)
}

func assertFileDoesNotContain(t *testing.T, path, unwanted string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	if strings.Contains(string(data), unwanted) {
		t.Fatalf("expected %s not to contain %q", path, unwanted)
	}
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	return string(data)
}
