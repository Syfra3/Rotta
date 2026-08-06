package installer

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeEnforcementCompatibilityProbeFailsClosedAndRetainsInstalledVersion(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "opencode")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nprintf '1.18.14\\n'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	got := ProbeOpenCodeRuntimeEnforcement(context.Background(), bin)
	if got.OpenCodeVersion != "1.18.14" || got.Supported || !strings.Contains(got.Reason, "delegate/task tool ID") {
		t.Fatalf("compatibility = %#v", got)
	}
}

func TestRuntimeEnforcementCompatibilityFixtureRecordsTheUnprovenDelegateBoundary(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "runtime-enforcement-opencode-1.18.14.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Hook   string `json:"hook"`
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.Hook != "tool.execute.before" || fixture.Status != "unsupported" || !strings.Contains(fixture.Reason, "delegate/task") {
		t.Fatalf("fixture = %#v", fixture)
	}
}

func TestInstallOpenCodeEvaluatesUnsupportedRuntimeGateWithoutMutatingUserPlugins(t *testing.T) {
	home := t.TempDir()
	config := filepath.Join(home, ".config", "opencode", "opencode.json")
	before := []byte(`{"plugin":["file:///user-plugin.js"],"theme":"keep"}`)
	if err := os.MkdirAll(filepath.Dir(config), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, before, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	compatibility, err := InstallOpenCodeRuntimeEnforcement(context.Background(), Options{Target: "opencode"}, home)
	if err != nil || compatibility.Supported || !strings.Contains(compatibility.Reason, "unsupported OpenCode runtime enforcement host") {
		t.Fatalf("compatibility = %#v, %v", compatibility, err)
	}
	if _, err := installOpenCode(Options{Target: "opencode"}, home); err != nil {
		t.Fatalf("install OpenCode with unsupported runtime enforcement host: %v", err)
	}
	after, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	var installed map[string]interface{}
	if err := json.Unmarshal(after, &installed); err != nil {
		t.Fatal(err)
	}
	plugins, ok := installed["plugin"].([]interface{})
	if !ok || len(plugins) != 1 || plugins[0] != "file:///user-plugin.js" || installed["theme"] != "keep" {
		t.Fatalf("user configuration changed: %s", after)
	}
	if strings.Contains(string(after), "runtime-enforcement") {
		t.Fatalf("unsupported host enabled a runtime enforcement plugin: %s", after)
	}
}
