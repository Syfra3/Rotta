package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVelaGuardsNeverScheduleOrInvokeVela(t *testing.T) {
	for name, source := range map[string]string{
		"OpenCode plugin": openCodeVelaFreshnessGuardPlugin(),
		"Claude hook":     claudeVelaFreshnessGuardScript(),
	} {
		t.Run(name, func(t *testing.T) {
			for _, forbidden := range []string{"vela update", "vela build", "spawn(", "schedule_refresh", "scheduleVelaGraphRefresh"} {
				if strings.Contains(strings.ToLower(source), forbidden) {
					t.Fatalf("guard contains forbidden automatic Vela behavior %q:\n%s", forbidden, source)
				}
			}
			if !strings.Contains(source, "no automatic refresh") {
				t.Fatalf("guard does not document inert advisory behavior:\n%s", source)
			}
		})
	}
}

func TestInstallOpenCodeVelaGuardWritesInertPlugin(t *testing.T) {
	home := t.TempDir()
	if _, err := installOpenCodeVelaFreshnessGuard(home); err != nil {
		t.Fatal(err)
	}
	pluginPath := openCodeVelaFreshnessPluginPath(home)
	data, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(data)), "vela update") || strings.Contains(strings.ToLower(string(data)), "vela build") {
		t.Fatalf("installed plugin retained automatic Vela command:\n%s", data)
	}
	config, err := os.ReadFile(filepath.Join(home, ".config", "opencode", "opencode.json"))
	if err != nil || !strings.Contains(string(config), openCodePluginFileURL(pluginPath)) {
		t.Fatalf("installed plugin configuration = %q, %v", config, err)
	}
}

func TestInstallClaudeVelaGuardWritesInertHook(t *testing.T) {
	home := t.TempDir()
	if _, err := installClaudeCodeVelaFreshnessGuard(home); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(claudeCodeVelaFreshnessHookPath(home))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"vela update", "vela build", "schedule_refresh"} {
		if strings.Contains(strings.ToLower(string(data)), forbidden) {
			t.Fatalf("installed hook retained automatic Vela command %q:\n%s", forbidden, data)
		}
	}
}
