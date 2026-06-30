# TDD Log

## SCN-001 — Install creates a timestamped backup before any mutation

### RED
- Added `TestSCN001_InstallCreatesTimestampedBackupBeforeMutation` for `REQ-001 → SCN-001`.
- Focused command: `go test ./internal/installer -run TestSCN001_InstallCreatesTimestampedBackupBeforeMutation -count=1`.
- Expected failure observed before production code:
  - `result.BackupDir undefined (type *Result has no field or method BackupDir)`
  - `undefined: backupManifest`

### GREEN
- Added backup creation before installer mutations and exposed the backup directory on `Result`.
- Added a JSON manifest with project path, target, selected modes, optional integrations, backed-up paths, missing paths, status, and schema version.
- Focused command passed: `go test ./internal/installer -run TestSCN001_InstallCreatesTimestampedBackupBeforeMutation -count=1`.

### REFACTOR
- Simplified backup scope handling for project graph metadata and tightened home-relative backup path checks.
- Final verification stayed green:
  - `go test ./internal/installer -run TestSCN001_InstallCreatesTimestampedBackupBeforeMutation -count=1`
  - `go test ./...`
   - `make fmt-check`
   - `make lint`

## SCN-002 — Successful install cleans previous settings before fresh install

### RED
- Added `TestSCN002_SuccessfulInstallCleansPreviousSettingsBeforeFreshInstall` for `REQ-004 → SCN-002`.
- Focused command: `go test ./internal/installer -run 'TestSCN00(1|2)_' -count=1`.
- Expected failure observed before production code:
  - `expected stale clean-spec agent to be replaced`

### GREEN
- Added cleanup after successful backup and before fresh install for selected targets.
- Removed prior clean-workflow-owned opencode agent entries and skill directories before writing selected fresh entries.
- Removed prior Claude Code clean-workflow skill directory and normalized clean-workflow permission entries before applying selected permissions.
- Focused command passed: `go test ./internal/installer -run 'TestSCN00(1|2)_' -count=1`.

### REFACTOR
- Formatted changed Go files with `gofmt` and kept cleanup helpers target-scoped.
- Final verification stayed green:
  - `go test ./internal/installer -run 'TestSCN00(1|2)_' -count=1`
  - `go test ./...`

## SCN-002 — Selected integration cleanup before optional setup

### RED
- Added `TestSCN002_SelectedIntegrationCleanupRunsBeforeOptionalSetup` for `REQ-004 → SCN-002`.
- Focused command: `go test ./internal/installer -run 'TestSCN00[12]'`.
- Expected failure observed before production code:
  - `stale ancora config was not cleaned before setup`
  - `ancora setup: ancora setup claude-code: exit status 17`

### GREEN
- Passed the resolved project path into cleanup before fresh install.
- Removed selected optional integration artifacts before running current optional setup: project graph database, selected target Vela files, and Claude Code Ancora MCP config.
- Focused command passed: `go test ./internal/installer -run 'TestSCN00[12]'`.

### REFACTOR
- Formatted changed Go files with `gofmt`.
- Final verification stayed green:
  - `go test ./internal/installer -run 'TestSCN00[12]'`
  - `go test ./...`
  - `make fmt-check`
  - `make lint`
