package tui

import (
	"strings"
	"testing"
)

func TestViewConfirmRendersAncoraVelaCombinations(t *testing.T) {
	tests := []struct {
		name        string
		ancora      bool
		vela        bool
		want        []string
		notExpected []string
	}{
		{
			name:   "ancora and vela enabled",
			ancora: true,
			vela:   true,
			want: []string{
				"Ancora memory:",
				"Vela graph:",
				"yes (install + configure)",
				"~/.claude/mcp/ancora.json",
				"<project>/.vela/graph.db  (initialized, not extracted)",
			},
			notExpected: []string{"~/.claude/vela-mcp.json"},
		},
		{
			name:   "ancora enabled and vela disabled",
			ancora: true,
			vela:   false,
			want: []string{
				"Ancora memory:",
				"yes (install + configure)",
				"Vela graph:",
				"skip",
				"~/.claude/mcp/ancora.json",
			},
			notExpected: []string{"<project>/.vela/graph.db", "~/.claude/vela-mcp.json"},
		},
		{
			name:   "ancora disabled and vela enabled",
			ancora: false,
			vela:   true,
			want: []string{
				"Ancora memory:",
				"skip",
				"Vela graph:",
				"yes (install + configure)",
				"<project>/.vela/graph.db  (initialized, not extracted)",
				"~/.claude/vela-mcp.json",
			},
			notExpected: []string{"~/.claude/mcp/ancora.json"},
		},
		{
			name:   "ancora and vela disabled",
			ancora: false,
			vela:   false,
			want: []string{
				"Ancora memory:",
				"Vela graph:",
				"skip",
			},
			notExpected: []string{"~/.claude/mcp/ancora.json", "<project>/.vela/graph.db", "~/.claude/vela-mcp.json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := New()
			model.Screen = ScreenConfirm
			model.Target = TargetClaudeCode
			model.ProjectPath = "/tmp/project"
			model.SetupAncora = tt.ancora
			model.SetupVela = tt.vela

			got := model.viewConfirm()
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Fatalf("missing %q:\n%s", want, got)
				}
			}
			for _, unwanted := range tt.notExpected {
				if strings.Contains(got, unwanted) {
					t.Fatalf("unexpected %q:\n%s", unwanted, got)
				}
			}
		})
	}
}
