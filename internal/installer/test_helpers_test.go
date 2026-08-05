package installer

import (
	"os"
	"strings"
	"testing"
)

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return contents
}

func readFileString(t *testing.T, path string) string {
	return string(mustReadFile(t, path))
}

func assertStringListContains(t *testing.T, values []string, want string) {
	t.Helper()
	for _, value := range values {
		if value == want {
			return
		}
	}
	t.Fatalf("values %q do not contain %q", values, want)
}

func assertFileDoesNotContain(t *testing.T, path, unwanted string) {
	t.Helper()
	if strings.Contains(string(mustReadFile(t, path)), unwanted) {
		t.Fatalf("%s contains %q", path, unwanted)
	}
}
