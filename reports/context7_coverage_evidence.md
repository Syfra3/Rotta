# Context7 MCP Installer Coverage Evidence

## Gate Source
- `.rotta/quality-gates.yaml`
- Required `changed_line_coverage >= 0.90`
- Required `critical_path_branch_coverage >= 0.95`

## Coverage Command
- `go test ./internal/installer ./internal/tui -coverprofile=/tmp/rotta-context7-cover.out` — PASS.

## Changed-Line Coverage
- Diff basis: current working-tree diff against `HEAD`, including untracked Context7 implementation files.
- Scope: changed production Go lines in the Context7 implementation diff; test files are excluded, and uncovered executable statement lines are derived from the Go coverage profile.
- Covered changed production lines: `288`
- Total instrumented changed production lines: `308`
- Exact value: `288 / 308 = 0.9350649351`
- Gate result: PASS (`0.9350649351 >= 0.90`)

Remaining uncovered changed production statement lines are non-critical error/failure plumbing in:
- `internal/installer/installer.go:115-116`
- `internal/tui/update.go:298-299`
- `internal/installer/context7.go:160-161,178-181,184-187,198-201,216-219,241-242`

## Critical-Path Branch Coverage
- Critical Context7 path branches covered: `32`
- Critical Context7 path branches identified: `32`
- Exact value: `32 / 32 = 1.0000000000`
- Gate result: PASS (`1.0000000000 >= 0.95`)

Covered critical branches:
1. Context7 appears in the TUI optional MCP flow and is selected by default — `TestSCN101_TUIContext7VisibleSelectedByDefault`.
2. Selecting Context7 does not mutate Ancora/Vela choices — `TestSCN102_TUIContext7SelectionDoesNotChangeOtherTools`.
3. Vela selection advances to Context7 before confirmation — `TestSCN101_Context7NavigationBackAndRecoveryFormatting`.
4. Context7 back navigation returns to Vela — `TestSCN101_Context7NavigationBackAndRecoveryFormatting`.
5. Confirmation back navigation returns to Context7 — `TestSCN101_Context7NavigationBackAndRecoveryFormatting`.
6. Context7 can be deselected before install — `TestSCN111_TUIContext7CanBeDeselectedBeforeInstall`.
7. Context7 writes OpenCode stdio MCP config — `TestSCN103_SelectedContext7ConfiguresBothHostsWithStdioCommand`.
8. Context7 writes Claude Code stdio MCP config — `TestSCN103_SelectedContext7ConfiguresBothHostsWithStdioCommand`.
9. OpenCode MCP section is created when missing — `TestSCN103_Context7OpenCodeCreatesMCPSectionWhenMissing`.
10. Duplicate/legacy Context7 OpenCode entries are normalized — `TestSCN110_Context7RerunNormalizesDuplicateHostEntriesBeforeHealth`.
11. Host write failure is recorded per host as partial, not success — `TestSCN104_Context7ReportsPerHostConfigurationFailures` and `TestSCN104_Context7ConfigSummarizesHostWriteFailuresAndEmptyHosts`.
12. Empty host summaries are skipped, not configured — `TestSCN104_Context7ConfigSummarizesHostWriteFailuresAndEmptyHosts`.
13. Deselecting Context7 skips host config and health checks — `TestSCN108_SkippedContext7LeavesHostConfigAndInstructionsUnchanged`.
14. Context7 skip does not affect Ancora/Vela selections — `TestSCN109_Context7SkipDoesNotAffectAncoraAndVelaSelection`.
15. Full install invokes Context7 health after successful host config — `TestSCN106_InstallRunsContext7HealthAndRecordsConfiguredStatus`.
16. Full install records configured status when health is OK and both hosts are configured — `TestSCN106_InstallRunsContext7HealthAndRecordsConfiguredStatus`.
17. Full install returns a partial result and error when health fails — `TestSCN105_InstallReportsContext7HealthFailureWithPartialResult`.
18. Missing `npx` is categorized as command availability failure — `TestSCN105_Context7MissingCommandFailsWithoutBlamingHostConfig`.
19. Health success requires MCP initialize plus tool discovery — `TestSCN106_Context7HealthRequiresMCPInitializationAndToolDiscovery`.
20. MCP initialization error is rejected — `TestSCN107_Context7HealthRejectsFalsePositives`.
21. No tools returned is rejected — `TestSCN107_Context7HealthRejectsFalsePositives`.
22. Only one expected tool is rejected — `TestSCN107_Context7HealthRejectsFalsePositives`.
23. Immediate server exit is rejected — `TestSCN107_Context7HealthRejectsImmediateServerExit`.
24. Health timeout is rejected — `TestSCN107_Context7HealthRejectsTimeout`.
25. Startup/protocol failures are rejected — `TestSCN107_Context7HealthRejectsStartupAndProtocolFailures`.
26. Backup scope includes unique Context7 host config paths — `TestSCN110_Context7BackupScopeIncludesUniqueHostConfigPaths`.
27. Backup manifest restores selected/skipped Context7 choice — `TestSCN108_Context7BackupManifestRestoresSkippedAndSelectedChoice`.
28. Confirmation shows the default-selected Context7 choice as install/configure, not skip — `TestSCN101_TUIConfirmShowsSelectedContext7ByDefault`.
29. Health proof validates the actual JSON-RPC `initialize` method — `TestSCN106_Context7HealthRequiresMCPInitializationAndToolDiscovery`.
30. Health proof validates the actual JSON-RPC `tools/list` method — `TestSCN106_Context7HealthRequiresMCPInitializationAndToolDiscovery`.
31. Install runs Context7 health when either host config succeeds, preserving partial-host safety — `TestSCN106_InstallRunsContext7HealthWhenAnyHostConfigured`.
32. Backup manifest records selected Context7 for restore/re-run safety — `TestSCN110_Context7BackupManifestRecordsSelectedChoice`.

## Approved Scenarios Represented
- `SCN-101`, `SCN-102`, `SCN-103`, `SCN-104`, `SCN-105`, `SCN-106`, `SCN-107`, `SCN-108`, `SCN-109`, `SCN-110`, `SCN-111`.
