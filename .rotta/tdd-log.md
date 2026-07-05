# Rotta TDD Log

## 2026-07-05 — Context7 MCP TUI installation contract (SCN-101–SCN-111)

### RED
- Added traceable failing tests for approved scenarios SCN-101 through SCN-111:
  - `internal/tui/view_test.go`: `TestSCN101_TUIContext7VisibleSelectedByDefault`, `TestSCN111_TUIContext7CanBeDeselectedBeforeInstall`, `TestSCN102_TUIContext7SelectionDoesNotChangeOtherTools`.
  - `internal/installer/context7_test.go`: `TestSCN103_SelectedContext7ConfiguresBothHostsWithStdioCommand`, `TestSCN104_Context7ReportsPerHostConfigurationFailures`, `TestSCN105_Context7MissingCommandFailsWithoutBlamingHostConfig`, `TestSCN106_Context7HealthRequiresMCPInitializationAndToolDiscovery`, `TestSCN107_Context7HealthRejectsFalsePositives`, `TestSCN107_Context7HealthRejectsImmediateServerExit`, `TestSCN107_Context7HealthRejectsTimeout`, `TestSCN108_SkippedContext7LeavesHostConfigAndInstructionsUnchanged`, `TestSCN109_Context7SkipDoesNotAffectAncoraAndVelaSelection`, `TestSCN110_Context7RerunNormalizesDuplicateHostEntriesBeforeHealth`.
- Confirmed RED with `go test ./internal/tui ./internal/installer` failing for missing production API/fields, e.g. `undefined: ConfigureContext7`, `unknown field SetupContext7 in struct literal of type Options`, `undefined: ScreenContext7`, and `model.SetupContext7 undefined`.

### GREEN
- Implemented Context7 selection state and TUI screen as a main optional MCP tool checked by default, independently selectable from Ancora/Vela.
- Implemented Context7 host configuration for OpenCode and Claude Code with stdio command `npx -y @upstash/context7-mcp`, preserving unrelated config and normalizing duplicate Rotta-managed entries.
- Implemented Context7 health check that launches the configured stdio server, sends MCP `initialize`, and requires tool discovery for both `resolve-library-id` and `query-docs`/documented equivalents before reporting success.
- Implemented skipped-state behavior so deselection avoids host config mutation, command checks, health checks, and generated workflow instruction mentions.
- Confirmed GREEN with `go test ./...`.

### REFACTOR
- Ran `gofmt` on changed Go files.
- Kept Context7 workflow instruction changes intentionally absent per contract; host MCP availability is configured without agent prompting rules.
- Confirmed final verification:
  - `make fmt-check` — PASS.
  - `make lint` — PASS (`0 issues.`).
  - `go test ./...` — PASS.
  - `go build -o bin/rotta ./cmd/rotta` — PASS.
  - `make test-ci` — PASS (`146 tests`).
