package installer

import "testing"

func TestPiDefaultCatalogPresence(t *testing.T) {
	ids := []string{"openai-codex/gpt-6.1-sol", "openai-codex/gpt-6.1-terra", "openai-codex/gpt-6-luna", "openai-codex/gpt-6.2-sol"}
	if !PiDefaultsInCatalog(ids) {
		t.Fatal("complete preset missing")
	}
	if PiDefaultsInCatalog(ids[:2]) {
		t.Fatal("missing luna accepted")
	}
}
