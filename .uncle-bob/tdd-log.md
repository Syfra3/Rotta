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

## SCN-003 — Backup failure aborts install completely

### RED
- Added `TestSCN003_BackupFailureAbortsInstallCompletely` for `REQ-003 → SCN-003`.
- Focused command: `go test ./internal/installer -run 'TestSCN003' -count=1`.
- Expected failure observed before production code:
  - `expected recovery-safe backup failure message, got create install backup: mkdir .../.clean-workflow/backups: not a directory`

### GREEN
- Changed installer backup failure reporting so install returns a recovery-safe failure message before cleanup or fresh install runs.
- Focused command passed: `go test ./internal/installer -run 'TestSCN003' -count=1`.

### REFACTOR
- Formatted changed Go files with `gofmt`.
- Final focused verification stayed green:
  - `go test ./internal/installer -run 'TestSCN00[1-3]' -count=1`
   - `go test ./...`
   - `make fmt-check`
   - `make lint`

## SCN-004 — TUI lists available backups from recovery

### RED
- Added `TestSCN004_TUIListsAvailableBackupsFromRecovery` for `REQ-006 → SCN-004`.
- Focused command: `go test ./internal/tui -run TestSCN004_TUIListsAvailableBackupsFromRecovery -count=1`.
- Expected failure observed before production code:
  - `expected recovery view to contain "Recovery"` because the welcome screen had no recovery entry point yet.

### GREEN
- Added a recovery screen reachable from the welcome screen with `r`.
- Loaded complete backup manifests from `~/.clean-workflow/backups` and listed valid backups by timestamp, project path, and target.
- Focused command passed: `go test ./internal/tui -run TestSCN004_TUIListsAvailableBackupsFromRecovery -count=1`.

### REFACTOR
- Formatted changed TUI files with `gofmt` and kept invalid/missing manifests excluded from the list.
- Final verification stayed green:
  - `go test ./internal/installer -run 'TestSCN00[1-3]' -count=1`
  - `go test ./internal/tui -run TestSCN004_TUIListsAvailableBackupsFromRecovery -count=1`
  - `go test ./...`
  - `make fmt-check`
  - `make lint`

## SCN-005 — TUI previews backup contents and metadata

### RED
- Added `TestSCN005_TUIPreviewsBackupContentsAndMetadata` for `REQ-006, REQ-009 → SCN-005`.
- Focused command: `go test ./internal/tui -run TestSCN005_TUIPreviewsBackupContentsAndMetadata -count=1`.
- Expected failure observed before production code:
  - `expected backup preview to contain "Backup preview"` because selecting a listed backup did not open a manifest-derived preview yet.

### GREEN
- Added backup preview state from the recovery list and rendered timestamp, project path, target, selected modes, optional integrations, backed-up paths, missing paths, and full-backup-only restore wording from the manifest.
- Focused command passed: `go test ./internal/tui -run TestSCN005_TUIPreviewsBackupContentsAndMetadata -count=1`.

### REFACTOR
- Formatted changed TUI files with `gofmt` and kept preview rendering manifest-derived without adding restore behavior.
- Final focused verification stayed green:
  - `go test ./internal/installer -run 'TestSCN00[1-3]' -count=1`
  - `go test ./internal/tui -run 'TestSCN00(4|5)' -count=1`

## SCN-006 — TUI requires confirmation before full restore

### RED
- Added `TestSCN006_TUIRequiresConfirmationBeforeFullRestore` for `REQ-006, REQ-007 → SCN-006`.
- Focused command: `go test ./internal/tui -run 'TestSCN00[4-6]'`.
- Expected failure observed before production code:
  - `undefined: ScreenRecoveryConfirm` because choosing restore from the preview had no confirmation state yet.

### GREEN
- Added a recovery confirmation screen reachable from the backup preview with `r`.
- The confirmation screen identifies the selected backup and states restore has not started; choosing restore from preview returns no command.
- Focused command passed: `go test ./internal/tui -run 'TestSCN00[4-6]'`.

### REFACTOR
- Formatted changed TUI files with `gofmt` and kept restore execution out of scope for SCN-006.
- Final verification stayed green:
  - `go test ./internal/tui -run 'TestSCN00[4-6]'`
  - `go test ./...`
  - `make fmt-check`
  - `make lint`

## SCN-007 — Restore applies the full backup and removes paths that were absent

### RED
- Added `TestSCN007_RestoreAppliesFullBackupAndRemovesAbsentPaths` for `REQ-007 → SCN-007`.
- Focused command: `go test ./internal/installer -run TestSCN007_RestoreAppliesFullBackupAndRemovesAbsentPaths -count=1`.
- Expected failure observed before production code:
  - `undefined: RestoreBackup`

### GREEN
- Added full-backup restore execution that reads a complete backup manifest, creates a pre-restore safety backup, restores every backed-up file/directory, and removes paths recorded as missing in the selected backup.
- Focused command passed: `go test ./internal/installer -run TestSCN007_RestoreAppliesFullBackupAndRemovesAbsentPaths -count=1`.

### REFACTOR
- Formatted changed Go files with `gofmt` and kept rollback behavior out of scope for later restore-failure scenarios.
- Final verification stayed green:
  - `go test ./internal/installer -run 'TestSCN00[1-3]|TestSCN007' -count=1`
  - `go test ./internal/tui -run 'TestSCN00[4-6]' -count=1`
  - `go test ./...`
  - `make fmt-check`
  - `make lint`

## SCN-008 — Failed restore rolls back to pre-restore state

### RED
- Added `TestSCN008_FailedRestoreRollsBackToPreRestoreState` for `REQ-008 → SCN-008`.
- Focused command: `go test ./internal/installer -run TestSCN008_FailedRestoreRollsBackToPreRestoreState -count=1`.
- Expected failure observed before production code:
  - `undefined: restoreBackupWithHooks`
  - `undefined: restoreHooks`

### GREEN
- Added restore failure hooks for temp-path tests and rollback-on-failure behavior that restores the pre-restore safety backup after a destination path has changed.
- Restore failures return a failed result with the pre-restore safety backup location and a message identifying the selected backup and successful rollback.
- Focused command passed: `go test ./internal/installer -run TestSCN008_FailedRestoreRollsBackToPreRestoreState -count=1`.

### REFACTOR
- Formatted changed Go files with `gofmt` and extracted backup-content application for restore and rollback reuse.
- Final verification stayed green:
  - `go test ./internal/installer -run 'TestSCN00[1-3]|TestSCN00[78]' -count=1`
  - `go test ./internal/tui -run 'TestSCN00[4-6]' -count=1`
  - `go test ./...`
  - `make fmt-check`
  - `make lint`

## SCN-009 — Restore failure with rollback failure provides manual recovery locations

### RED
- Added `TestSCN009_RestoreFailureWithRollbackFailureProvidesManualRecoveryLocations` for `REQ-008 → SCN-009`.
- Focused command: `go test ./internal/installer -run TestSCN009_RestoreFailureWithRollbackFailureProvidesManualRecoveryLocations -count=1`.
- Expected failure observed before production code:
  - `expected failure to identify pre-restore safety backup ... got restore failed for selected backup ... and rollback to pre-restore state failed: cannot parse backup manifest ...`

### GREEN
- Changed rollback-failure reporting to include both the selected backup location and the pre-restore safety backup location.
- Focused command passed: `go test ./internal/installer -run TestSCN009_RestoreFailureWithRollbackFailureProvidesManualRecoveryLocations -count=1`.

### REFACTOR
- Formatted changed Go files with `gofmt` and kept rollback failure injection limited to temp backup manifests.
- Final focused verification stayed green:
  - `go test ./internal/installer -run 'TestSCN00[1-3]|TestSCN00[7-9]' -count=1`
  - `go test ./internal/tui -run 'TestSCN00[4-6]' -count=1`

## SCN-010 — CLI install path cannot skip backup during normal usage

### RED
- Added `TestSCN010_CLIInstallCannotSkipBackupDuringNormalUsage` for `REQ-005, REQ-010 → SCN-010`.
- Focused command: `go test ./cmd/clean-workflow -run TestSCN010_CLIInstallCannotSkipBackupDuringNormalUsage -count=1`.
- Expected failure observed before production code:
  - `undefined: runCLI`

### GREEN
- Added a non-interactive `install` command that delegates to the existing backup-first installer and prints the backup location.
- Kept `--version` available through the same CLI entry point.
- Did not add any backup-skipping install option, so `--skip-backup` is rejected before installation.
- Focused command passed: `go test ./cmd/clean-workflow -run TestSCN010_CLIInstallCannotSkipBackupDuringNormalUsage -count=1`.

### REFACTOR
- Formatted changed Go files with `gofmt` and kept CLI parsing separate from TUI startup.
- Final verification stayed green:
  - `go test ./internal/installer -run 'TestSCN00[1-3]|TestSCN00[7-9]' -count=1`
  - `go test ./internal/tui -run 'TestSCN00[4-6]' -count=1`
  - `go test ./cmd/clean-workflow -run TestSCN010_CLIInstallCannotSkipBackupDuringNormalUsage -count=1`
  - `go test ./...`
  - `make fmt-check`
  - `make lint`

## SCN-011 — Generated acceptance artifacts and user-facing text avoid external-reference wording

### RED
- Added `TestSCN011_GeneratedArtifactsAndUserFacingTextAvoidExternalReferenceWording` for `REQ-009 → SCN-011`.
- Focused command: `go test ./internal/installer -run TestSCN011_GeneratedArtifactsAndUserFacingTextAvoidExternalReferenceWording -count=1`.
- Expected failure observed before artifact cleanup:
  - `expected neutral wording in ../../.atl/skill-registry.md`

### GREEN
- Removed non-neutral source comments and skill descriptions from the generated skill registry artifact.
- Focused command passed: `go test ./internal/installer -run TestSCN011_GeneratedArtifactsAndUserFacingTextAvoidExternalReferenceWording -count=1`.

### REFACTOR
- Formatted the changed Go test and kept the neutral wording scan limited to repository artifacts while skipping `.git` and `.vela`.
- Final focused verification stayed green:
  - `go test ./internal/installer -run 'TestSCN00[1-9]|TestSCN010|TestSCN011' -count=1`
  - `go test ./internal/tui -run 'TestSCN00[4-6]' -count=1`
  - `go test ./cmd/clean-workflow -run TestSCN010_CLIInstallCannotSkipBackupDuringNormalUsage -count=1`

## Final critical fixes — TUI restore execution and CLI recovery commands

### RED
- Added `TestSCN007_TUIConfirmationExecutesFullRestore` for `REQ-006, REQ-007 → SCN-007`.
- Added `TestSCN005_CLIBackupRestoreCommandsAreDiscoverableAndUnknownCommandsFail` for `REQ-005 → SCN-005`.
- Focused commands:
  - `go test ./internal/tui -run TestSCN007_TUIConfirmationExecutesFullRestore -count=1`
  - `go test ./cmd/clean-workflow -run TestSCN005_CLIBackupRestoreCommandsAreDiscoverableAndUnknownCommandsFail -count=1`
- Expected failures observed before production code:
  - `expected confirmation key to start restore command`
  - `could not open a new TTY: open /dev/tty: no such device or address`

### GREEN
- Added explicit `y`/Enter confirmation handling from the TUI recovery confirmation screen that starts `installer.RestoreBackup` for the selected backup directory.
- Added `backup` and `restore` CLI commands and changed unknown commands to fail instead of starting the TUI.
- Focused commands passed:
  - `go test ./internal/tui -run TestSCN007_TUIConfirmationExecutesFullRestore -count=1`
  - `go test ./cmd/clean-workflow -run TestSCN005_CLIBackupRestoreCommandsAreDiscoverableAndUnknownCommandsFail -count=1`

### REFACTOR
- Formatted changed Go files with `gofmt` and kept restore tests temp-home scoped.
- Final verification stayed green:
  - `go test ./internal/tui -run 'TestSCN00(4|5|6|7)_|TestViewConfirm' -count=1`
  - `go test ./cmd/clean-workflow -run 'TestSCN00(5)|TestSCN010' -count=1`
  - `go test ./internal/installer -run 'TestSCN00[1-9]|TestSCN010|TestSCN011' -count=1`
  - `go test ./...`
   - `make fmt-check`
   - `make lint`
   - `git diff --check`

## SCN-018 — Pending generated contracts do not pass the implementation gate

### RED
- Added `TestSCN018_PendingContractRequiresScopedApproval` for `REQ-015 → SCN-018`.
- Focused command: `go test ./internal/workflow -run TestSCN018_PendingContractRequiresScopedApproval -count=1`.
- Expected failure observed before production code:
  - `undefined: EvaluateImplementationGate`
  - `undefined: ContractScope`

### GREEN
- Added a scoped implementation gate seam that checks `specs/approvals/<contract-id>.approved` for the requested scenario or feature-qualified scenario reference.
- The gate ignores the legacy global `specs/.approved` marker for new contract scopes and reports that human approval is still required when no scoped record names the scenario.
- Focused command passed: `go test ./internal/workflow -run TestSCN018_PendingContractRequiresScopedApproval -count=1`.

### REFACTOR
- Formatted the new workflow approval package with `gofmt` and kept the API focused on SCN-018 pending-contract gate behavior.
- Final verification stayed green:
  - `go test ./internal/workflow -run TestSCN018_PendingContractRequiresScopedApproval -count=1`
   - `go test ./...`
   - `make fmt-check`
   - `make lint`

## SCN-018 remediation — Scoped approval positive and edge-case coverage

### RED
- Added coverage-first remediation tests for `REQ-015 → SCN-018`:
  - `TestSCN018_ScopedApprovalAllowsImplementationGate`
  - `TestSCN018_FeatureQualifiedScopedApprovalAllowsImplementationGate`
  - `TestSCN018_MissingScopedApprovalFileFailsClosed`
  - `TestSCN018_UnreadableScopedApprovalFileReturnsError`
  - `TestSCN018_MalformedScopedApprovalFileReturnsError`
- Focused command: `go test ./internal/workflow -run 'TestSCN018_(ScopedApprovalAllowsImplementationGate|FeatureQualifiedScopedApprovalAllowsImplementationGate|MissingScopedApprovalFileFailsClosed|UnreadableScopedApprovalFileReturnsError)' -count=1`.
- Current-code result: PASS. The implementation already supported the positive scoped approval behavior; the failing review gate was the missing test evidence rather than a production assertion failure.

### GREEN
- No production code change was required for the positive scoped approval, feature-qualified approval, missing scoped file, unreadable scoped path, or malformed scoped file behaviors.
- Focused command passed: `go test ./internal/workflow -run 'TestSCN018_(MalformedScopedApprovalFileReturnsError|ScopedApprovalAllowsImplementationGate|FeatureQualifiedScopedApprovalAllowsImplementationGate|MissingScopedApprovalFileFailsClosed|UnreadableScopedApprovalFileReturnsError)' -count=1`.

### REFACTOR
- Extracted `workflowArtifactLifecycleScope` test helper to keep scoped approval tests focused on behavior.
- Added malformed scoped approval coverage for scanner error handling without expanding beyond the SCN-018 approval resolver.
- Focused workflow coverage improved to 91.7% package coverage, with `EvaluateImplementationGate` at 100.0% and `scopedApprovalContains` at 87.5%:
  - `go test ./internal/workflow -run TestSCN018 -coverprofile=/tmp/scn018-workflow.cover -count=1`
  - `go tool cover -func=/tmp/scn018-workflow.cover`

## SCN-012 — Active hard spec and feature files are tracked as the contract source of truth

### RED
- Added `TestSCN012_TrackedHardSpecAndFeatureAreAuthoritativeContractSources` for `REQ-011 → REQ-012 → SCN-012`.
- The test creates a temporary git repository, tracks `specs/workflow_artifact_lifecycle.md` and `features/workflow_artifact_lifecycle.feature`, and asserts that repository files are authoritative without requiring full Ancora contract text.
- Focused command: `go test ./internal/workflow -run TestSCN012_TrackedHardSpecAndFeatureAreAuthoritativeContractSources -count=1`.
- Expected failure observed before production code:
  - `undefined: EvaluateContractSourceOfTruth`

### GREEN
- Added `EvaluateContractSourceOfTruth` and `ContractSourceStatus` to classify tracked hard spec and feature files as authoritative repository contract sources.
- Added git-backed path tracking checks using `git ls-files --error-unmatch` against the provided repository root.
- Focused command passed: `go test ./internal/workflow -run TestSCN012_TrackedHardSpecAndFeatureAreAuthoritativeContractSources -count=1`.

### REFACTOR
- Formatted the workflow package with `go fmt ./internal/workflow` and kept the SCN-012 production seam limited to source-of-truth classification.
- Focused command stayed green: `go test ./internal/workflow -run TestSCN012_TrackedHardSpecAndFeatureAreAuthoritativeContractSources -count=1`.
- Final verification stayed green:
  - `go test ./...`
  - `make fmt-check`
  - `make lint`
  - `git diff --check`

## SCN-020 — Retired or superseded process artifacts can be archived without hiding active contracts

### RED
- Added SCN-020 lifecycle coverage for `REQ-016 → SCN-020`:
  - `TestSCN020_RetiredSupersededAndProcessOnlyArtifactsClassifyAsArchiveCandidates` verifies only retired, superseded, or process-only artifacts with retirement reasons are archive candidates and preserve their archive reason.
  - `TestSCN020_ArchiveEligibilityRequiresRetirementReason` verifies retirement state alone is insufficient for archive movement without a reason.
  - `TestSCN020_ArchivePlanMovesOnlyRetiredProcessArtifactsAndKeepsActiveRegressionContracts` verifies a pure archive plan moves only retired/process-only fixtures, records reasons, keeps active regression contracts discoverable, and does not move/delete fixture files.
- Focused commands:
  - `go test ./internal/workflow -run TestSCN020_RetiredSupersededAndProcessOnlyArtifactsClassifyAsArchiveCandidates`
  - `go test ./internal/workflow -run TestSCN020_ArchiveEligibilityRequiresRetirementReason`
  - `go test ./internal/workflow -run TestSCN020_ArchivePlanMovesOnlyRetiredProcessArtifactsAndKeepsActiveRegressionContracts`
- Expected failures observed before production code:
  - `unknown field Retired in struct literal of type WorkflowArtifactLifecycleInput`
  - `undefined: WorkflowArtifactRetired`
  - `classification.ArchiveReason undefined`
  - `expected retired artifact without a reason to stay out of archive moves`
  - `undefined: PlanWorkflowArtifactArchive`
  - `plan.ArchiveMoves undefined`

### GREEN
- Added retired, superseded, and process-only lifecycle classifications with required trimmed retirement reasons before an artifact becomes an archive candidate.
- Added `WorkflowArtifactArchiveMove` and `PlanWorkflowArtifactArchive` to build a pure archive move plan with `archive/<source>` destinations and reason text, while active approved feature files remain in `KeptActivePaths` and out of archive moves.
- Focused SCN-020 command passed: `go test ./internal/workflow -run 'TestSCN020_(ArchivePlanMovesOnlyRetiredProcessArtifactsAndKeepsActiveRegressionContracts|RetiredSupersededAndProcessOnlyArtifactsClassifyAsArchiveCandidates|ArchiveEligibilityRequiresRetirementReason)'`.

### REFACTOR
- Formatted changed workflow files with `gofmt` and kept SCN-020 as pure planning/classification logic; no real artifacts are moved or deleted.
- Full test suite stayed green: `go test ./...`.

## SCN-015 — Tests reference stable scenario IDs from feature files

### RED
- Added SCN-015 traceability coverage for `REQ-013 → REQ-019 → SCN-015`:
  - `TestSCN015_FeatureScenarioParserKeepsStableIDsWhenScenarioOrderChanges` verifies feature parsing preserves `@SCN-015`, requirement tags, and feature identity even when scenario order changes.
  - `TestSCN015_TestTraceValidatorRequiresScenarioIDAndFeatureIdentity` verifies test traces must include stable scenario IDs and feature identity when local scenario IDs can collide across feature files, including metadata and subtest naming conventions.
- Focused command: `go test ./internal/workflow -run TestSCN015 -count=1`.
- Expected failure observed before production code:
  - `undefined: ParseFeatureScenarioTags`
  - `undefined: FeatureScenario`
  - `undefined: ValidateTestScenarioTrace`
  - `undefined: TestScenarioTrace`

### GREEN
- Added pure feature scenario tag parsing and test trace validation seams in `internal/workflow/scenario_trace.go`.
- The parser extracts feature path, stable `SCN-*` tags, requirement tags, and scenario names from repository feature text without relying on scenario order.
- The validator accepts scenario IDs in test names, metadata, subtest names, or equivalent trace fields, and requires feature-qualified identity such as `features/workflow_artifact_lifecycle.feature#SCN-015` when another feature can contain the same local scenario ID.
- Focused command passed: `go test ./internal/workflow -run TestSCN015 -count=1`.

### REFACTOR
- Formatted the new workflow parser and trace tests with `gofmt` and kept behavior pure with no live external service calls.
- Focused command stayed green: `go test ./internal/workflow -run TestSCN015 -count=1`.

## SCN-016 — Ancora records pointer-only workflow state

### RED
- Added `TestSCN016_AncoraWorkflowStateSerializesPointersWithoutFullContractText` for `REQ-014 → SCN-016`.
- The test serializes fake workflow memory state and asserts artifact paths, phase, approval status, risk level, requirement IDs, scenario IDs, observation IDs, and checksums are present while full Markdown/Gherkin bodies are absent.
- Focused command: `go test ./internal/workflow -run TestSCN016_AncoraWorkflowStateSerializesPointersWithoutFullContractText -count=1`.
- Expected failure observed before production code:
  - `undefined: SerializeAncoraWorkflowState`
  - `undefined: AncoraWorkflowState`

### GREEN
- Added `AncoraWorkflowState` and `SerializeAncoraWorkflowState` to serialize pointer-only state with paths, IDs, phase/status, risk, observation IDs, and optional checksums.
- Did not add any live Ancora calls or full contract body fields.
- Focused command passed: `go test ./internal/workflow -run TestSCN016_AncoraWorkflowStateSerializesPointersWithoutFullContractText -count=1`.

### REFACTOR
- Formatted the workflow package changes with `gofmt` and kept SCN-016 behavior limited to pure state serialization.
- Focused command stayed green: `go test ./internal/workflow -run TestSCN016_AncoraWorkflowStateSerializesPointersWithoutFullContractText -count=1`.
- Final verification stayed green:
  - `go test ./...`
  - `make fmt-check`
  - `make lint`
  - `git diff --check`

## SCN-023 — QA planning enumerates approved scenarios from repository feature files

### RED
- Added `TestSCN023_QAPlanningEnumeratesApprovedRepositoryScenarios` for `REQ-019 → SCN-023`.
- The test creates temporary repository feature fixtures, records scoped approval only for `features/workflow_artifact_lifecycle.feature#SCN-023`, and asserts QA/TDD planning returns only that implementation-ready item with both feature path and scenario ID.
- Focused command: `go test ./internal/workflow -run TestSCN023_QAPlanningEnumeratesApprovedRepositoryScenarios -count=1`.
- Expected failure observed before production code:
  - `undefined: PlanImplementationReadyScenarios`

### GREEN
- Added pure repository feature planning via `PlanImplementationReadyScenarios`, which walks active `features/**/*.feature` files, parses tagged scenarios from repository content, and includes only scenarios present in the matching scoped approval marker.
- Pending unapproved scenarios in active or pending feature files are excluded from implementation-ready planning.
- Focused command passed: `go test ./internal/workflow -run TestSCN023_QAPlanningEnumeratesApprovedRepositoryScenarios -count=1`.

### REFACTOR
- Formatted changed workflow files with `gofmt` and kept planning pure with temporary fixtures only; no live external services are called.
- Final verification stayed green:
  - `go test ./internal/workflow -run TestSCN023 -count=1`
  - `go test ./...`
  - `make fmt-check`
  - `make lint`
  - `git diff --check`

### REMEDIATION — changed-line coverage gate
- Fresh Bob review failed `.clean-workflow/quality-gates.yaml` changed-line coverage for `PlanImplementationReadyScenarios` at 71.9% against the 90% threshold.
- Added focused SCN-023 planner tests for:
  - feature-qualified scoped approval and duplicate-SCN exclusion across repository feature identities;
  - excluding scenarios without a stable SCN identity from implementation-ready planning;
  - feature parse error propagation with repository feature path context;
  - missing feature directory behavior as no implementation-ready scenarios;
  - scoped approval read error propagation during planning.
- Remediation coverage evidence after tests: `PlanImplementationReadyScenarios` 93.8%, package statements 91.2% via `go test ./internal/workflow -coverprofile=/tmp/opencode/scn023-workflow.cover -count=1` and `go tool cover -func=/tmp/opencode/scn023-workflow.cover`.

## SCN-017 — Repository content wins when an Ancora pointer is stale

### RED
- Added SCN-017 pointer-validation coverage for `REQ-014 → SCN-017`:
  - `TestSCN017_ChangedRepositoryArtifactReportsStalePointerWithoutOverwritingContent` verifies checksum and missing-path stale pointer reports while repository content remains unchanged despite older fake Ancora text.
  - `TestSCN017_RenamedRepositoryArtifactRepairsPointerMetadataWithoutRestoringMemoryText` verifies a renamed reviewed repository artifact can repair pointer metadata from tracked repo files without restoring stale memory text.
- Focused command: `go test ./internal/workflow -run TestSCN017 -count=1`.
- Expected failure observed before production code:
  - `undefined: ValidateAncoraWorkflowPointers`
  - `undefined: PointerIssueChecksumMismatch`
  - `undefined: PointerIssueMissing`
  - `undefined: WorkflowPointerValidationReport`

### GREEN
- Added pure pointer validation for `AncoraWorkflowState` that reports missing and checksum-mismatched pointers, keeps repository content authoritative, ignores fake memory text, and repairs pointer metadata only from tracked repository artifacts when an unambiguous reviewed contract path is discoverable.
- Focused command passed: `go test ./internal/workflow -run TestSCN017 -count=1`.

### REFACTOR
- Formatted workflow changes with `gofmt` and kept the repair path metadata-only; no live Ancora calls and no repository content overwrite path were introduced.
- Focused command stayed green: `go test ./internal/workflow -run TestSCN017 -count=1`.

## SCN-014 — Implemented feature files remain active regression contracts

### RED
- Added SCN-014 lifecycle coverage for `REQ-012 → REQ-016 → SCN-014`:
  - `TestSCN014_ImplementedFeatureFileClassifiesAsActiveRegressionContract` verifies an approved implemented feature remains classified as an active regression contract, not an archive candidate.
  - `TestSCN014_ArchivePreparationKeepsImplementedActiveFeatureUnderFeatures` verifies completion/archive preparation keeps the implemented active feature under `features/` using temp fixtures only.
- Focused command: `go test ./internal/workflow -run TestSCN014 -count=1`.
- Expected failure observed before production code:
  - `undefined: ClassifyWorkflowArtifactLifecycle`
  - `undefined: WorkflowArtifactLifecycleInput`
  - `undefined: WorkflowArtifactActiveRegressionContract`
  - `undefined: PrepareCompletedChangeArchive`

### GREEN
- Added lifecycle classification and completed-change archive preparation seams that keep approved implemented feature files active under `features/`.
- Archive preparation reads the implementation completion marker and scoped approval marker, discovers the scenario from repository feature files, and records active features to keep without moving or deleting them.
- Focused command passed: `go test ./internal/workflow -run TestSCN014 -count=1`.

### REFACTOR
- Formatted changed workflow files with `gofmt` and kept archive preparation as a pure planning operation over repository/temp fixtures.
- Added focused SCN-014 coverage for archive preparation edge paths:
  - missing completion marker plans no archive action;
  - unapproved completed features are not treated as approved active regression contracts;
  - completion marker read errors and feature parse errors are surfaced instead of guessing archive behavior.
- Coverage evidence: `go test ./internal/workflow -coverprofile=/tmp/opencode/scn014-workflow.cover -covermode=count && go tool cover -func=/tmp/opencode/scn014-workflow.cover` reported package coverage at 91.1%, with `ClassifyWorkflowArtifactLifecycle`, `PrepareCompletedChangeArchive`, and `completedScenarioIDs` at 100.0%.

## SCN-013 — Namespaced workflow-policy artifacts do not overwrite an existing active contract

### RED
- Added `TestSCN013_NamespacedWorkflowPolicyArtifactsDoNotOverwriteExistingActiveContract` for `REQ-011 → REQ-020 → SCN-013`.
- The test creates existing active installer recovery contract files at `specs/hard_spec.md` and `features/installer_recovery.feature`, generates the workflow artifact lifecycle contract, and asserts the legacy files remain byte-for-byte unchanged while namespaced outputs are written.
- Focused command: `go test ./internal/workflow -run TestSCN013_NamespacedWorkflowPolicyArtifactsDoNotOverwriteExistingActiveContract -count=1`.
- Expected failure observed before production code:
  - `undefined: GenerateNamespacedWorkflowPolicyArtifacts`
  - `undefined: WorkflowPolicyArtifactRequest`

### GREEN
- Added `GenerateNamespacedWorkflowPolicyArtifacts`, `WorkflowPolicyArtifactRequest`, and `WorkflowPolicyArtifacts` to write workflow-policy contracts to `specs/<contract_id>.md` and `features/<contract_id>.feature`.
- Added a fail-fast guard for generated paths that would collide with supplied legacy active contract paths.
- Focused command passed: `go test ./internal/workflow -run TestSCN013_NamespacedWorkflowPolicyArtifactsDoNotOverwriteExistingActiveContract -count=1`.

### REFACTOR
- Formatted the workflow package with `gofmt` and kept generation behavior scoped to namespaced artifact path creation plus legacy overwrite protection.
- Full test suite stayed green: `go test ./...`.

## SCN-019 — Untracked active contracts are tracked instead of deleted to clean the tree

### RED
- Added `TestSCN019_UntrackedActiveContractsRequireTrackingInsteadOfDeletion` for `REQ-015 → REQ-020 → SCN-019`.
- The test creates a temporary git repository with approved workflow artifact lifecycle spec/feature files left untracked, then asks for clean-tree contract actions and asserts the active contracts are marked for tracking while their contents remain present.
- Focused command: `go test ./internal/workflow -run TestSCN019_UntrackedActiveContractsRequireTrackingInsteadOfDeletion -count=1`.
- Expected failure observed before production code:
  - `undefined: PlanCleanTreeContractActions`
  - `undefined: ContractCleanupTrack`
  - `undefined: ContractCleanupAction`
  - `undefined: ContractCleanupActionKind`

### GREEN
- Added `PlanCleanTreeContractActions`, `ContractCleanupAction`, and `ContractCleanupActionKind` with the `track` action for approved active contract files that are present but not tracked by git.
- Reused the scoped implementation gate so cleanup planning only treats the contract as active when human approval is recorded for the requested scope.
- Focused command passed: `go test ./internal/workflow -run TestSCN019_UntrackedActiveContractsRequireTrackingInsteadOfDeletion -count=1`.

### REFACTOR
- Formatted the workflow package with `gofmt` and kept SCN-019 behavior limited to planning tracking actions; no deletion path was introduced.
- Focused package verification stayed green: `go test ./internal/workflow -count=1`.

### COVERAGE REMEDIATION
- Added focused SCN-019 tests for clean-tree planning branches that were under-covered in review:
  - `TestSCN019_CleanTreePlanningRequiresScopedApproval` proves cleanup planning refuses unapproved active contracts and leaves files intact.
  - `TestSCN019_CleanTreePlanningSurfacesApprovalReadErrors` proves approval-store read failures stop planning without cleanup actions.
  - `TestSCN019_TrackedActiveContractsRequireNoCleanTreeAction` proves already tracked active contracts produce no unnecessary tracking action.
  - `TestSCN019_CleanTreePlanningReportsGitMetadataErrors` proves git metadata errors are surfaced instead of guessing a cleanup action.
- Focused remediation command passed: `go test ./internal/workflow -run 'TestSCN019_' -count=1`.
- Coverage evidence: `go test ./internal/workflow -coverprofile=/tmp/opencode/scn019-workflow.cover -covermode=count && go tool cover -func=/tmp/opencode/scn019-workflow.cover` reported `PlanCleanTreeContractActions` at `100.0%` statement coverage.
- Final verification stayed green:
  - `go test ./...`
  - `make fmt-check`
  - `make lint`
  - `git diff --check`

## SCN-021 — Local graph and cache artifacts are excluded unless intentionally promoted

### RED
- Added SCN-021 lifecycle coverage for `REQ-017 → SCN-021`:
  - `TestSCN021_LocalGraphAndCacheArtifactsClassifyOutsideReviewSet` verifies `.vela` and cache paths classify as local generated cache artifacts, stay out of the review set by default, and require an explicit project-artifact decision before intentional tracking can proceed.
  - `TestSCN021_ReviewSetPreparationExcludesVelaCacheAndKeepsContracts` verifies review-set preparation includes active `specs/` and `features/` contract artifacts while excluding `.vela/` and `.cache/` generated fixtures without deleting them.
- Focused commands:
  - `go test ./internal/workflow -run TestSCN021 -count=1`
  - `go test ./internal/workflow -run TestSCN021_LocalGraphAndCacheArtifactsClassifyOutsideReviewSet -count=1`
- Expected failures observed before production code:
  - `undefined: WorkflowArtifactLocalGeneratedCache`
  - `classification.ReviewCandidate undefined`
  - `classification.RequiresProjectArtifactDecision undefined`
  - `undefined: PrepareWorkflowArtifactReviewSet`
  - `unknown field IntentionallyTrackedGeneratedArtifact in struct literal of type WorkflowArtifactLifecycleInput`

### GREEN
- Added local generated graph/cache classification for `.vela`, `.cache`, `cache`, and `caches` path segments.
- Added pure review-set preparation that includes review candidates and excludes generated cache artifacts by default.
- Added explicit intentional-promotion fields so generated artifacts only enter the review set when an intentional tracking request also has a project-artifact decision.
- Focused command passed: `go test ./internal/workflow -run TestSCN021 -count=1`.

### REFACTOR
- Formatted changed workflow files with `gofmt` and kept SCN-021 as pure classification/planning logic over temp fixtures; no real `.vela` or cache files are deleted or modified.
- Final verification stayed green:
  - `go test ./...`
  - `make fmt-check`
  - `make lint`
  - `git diff --check`

## SCN-022 — Backup outputs and sensitive config captures are rejected as workflow artifacts

### RED
- Added SCN-022 lifecycle coverage for `REQ-018 → SCN-022`:
  - `TestSCN022_SensitiveBackupAndMachineStateArtifactsAreRejected` verifies backup outputs, restore snapshots, user config captures, token-bearing files, and private machine-state files are rejected as workflow artifacts and require delete/ignore/sanitized replacement handling.
  - `TestSCN022_ReviewSetRejectsSensitiveFixturesAndKeepsSanitizedExamples` verifies review-set preparation excludes sensitive active spec/config captures while keeping active features and sanitized authored examples.
- Focused command: `go test ./internal/workflow -run TestSCN022 -count=1`.
- Expected failure observed before production code:
  - `unknown field Content in struct literal of type WorkflowArtifactLifecycleInput`
  - `undefined: WorkflowArtifactRejectedSensitive`
  - `classification.RequiresSanitizedReplacement undefined`

### GREEN
- Added sensitive artifact classification for backup/restore paths, captured user config paths, token/secret-bearing paths or contents, and private machine-state markers.
- Sensitive artifacts fail closed outside the review set and require deletion, ignore, or replacement with a sanitized authored example; examples containing redacted placeholders remain review candidates.
- Focused command passed: `go test ./internal/workflow -run TestSCN022 -count=1`.

### REFACTOR
- Formatted changed workflow files with `gofmt` and kept SCN-022 as pure classification/review planning over synthetic test content only; no real secret files or private machine-state files are read, moved, or deleted.
- Focused command stayed green: `go test ./internal/workflow -run TestSCN022 -count=1`.
- Coverage evidence: `go test ./internal/workflow -coverprofile=/tmp/opencode/scn022-workflow.cover -covermode=count -count=1` and `go tool cover -func=/tmp/opencode/scn022-workflow.cover` reported package coverage at 92.5%, with SCN-022 changed functions at 100.0% except unchanged pre-existing functions outside this scenario.
- Final verification stayed green:
  - `go test ./...`
  - `make fmt-check`
  - `make lint`
  - `git diff --check`

### REVIEW REMEDIATION
- Added adversarial SCN-022 regression coverage for redacted example content under sensitive backup, capture, token-bearing, and private machine-state paths.
- Confirmed RED failure exposed the review finding: sanitized example allowance let `backups/example/...`, `captures/example/...`, `docs/examples/api-token...`, and `machine-state/example/...` stay review candidates despite sensitive path markers.
- Fixed classification order so sensitive path and path-marker checks fail closed before sanitized authored examples can be allowed.
- Focused remediation command passed: `go test ./internal/workflow -run TestSCN022 -count=1`.
- Focused remediation coverage command passed: `go test ./internal/workflow -run TestSCN022 -coverprofile=/tmp/opencode/scn022-remediation.cover -covermode=count -count=1`; changed sensitive classifier helpers reported 100.0% coverage.
- Final remediation verification stayed green:
  - `go test ./...`
  - `make fmt-check`
  - `make lint`
  - `git diff --check`

## SCN-024 — Workflow cleanup explains artifact lifecycle actions explicitly

### RED
- Added `TestSCN024_WorkflowCleanupGuidanceLabelsArtifactLifecycleActions` for `REQ-020 → SCN-024`.
- The test uses synthetic lifecycle inputs for active contracts, pending contracts, archive candidates, local caches, and sensitive outputs, then asserts each cleanup guidance item has an explicit action label and actionable reason.
- Focused command: `go test ./internal/workflow -run TestSCN024_WorkflowCleanupGuidanceLabelsArtifactLifecycleActions -count=1`.
- Expected failure observed before production code:
  - `undefined: PrepareWorkflowArtifactCleanupGuidance`
  - `undefined: WorkflowArtifactCleanupTrack`
  - `undefined: WorkflowArtifactCleanupKeepPending`
  - `undefined: WorkflowArtifactCleanupArchive`
  - `undefined: WorkflowArtifactCleanupIgnore`
  - `undefined: WorkflowArtifactCleanupDelete`

### GREEN
- Added pure cleanup guidance planning with the explicit labels `track`, `keep pending`, `archive`, `ignore`, and `delete`.
- Active approved behavior contracts are labeled `track` rather than `delete`, pending contracts are labeled `keep pending` until human approval, archive candidates retain their archive reason, generated caches are ignored, and sensitive outputs are labeled for deletion guidance without moving or deleting files.
- Focused command passed: `go test ./internal/workflow -run TestSCN024_WorkflowCleanupGuidanceLabelsArtifactLifecycleActions -count=1`.

### REFACTOR
- Formatted changed workflow files with `gofmt` and kept SCN-024 as guidance/reporting only over synthetic fixtures; no real artifacts are deleted, moved, or read from private locations.
- Final verification stayed green:
  - `go test ./...`
  - `make fmt-check`
  - `make lint`
  - `git diff --check`

### REMEDIATION — Pending contract/archive precedence
- Fresh review found a pending workflow contract path could be labeled `archive` when the same input also carried archive-candidate metadata.
- RED: added `TestSCN024_WorkflowCleanupGuidanceKeepsPendingContractBeforeArchiveCandidate` with unapproved `specs/pending_contract.md` plus `ProcessOnly` and `RetirementReason`; focused command failed because the action was `archive` instead of `keep pending`.
- GREEN: moved unapproved workflow contract handling before archive-candidate handling in `PrepareWorkflowArtifactCleanupGuidance`, while keeping sensitive rejection and generated-cache guidance ahead of contract/archive guidance.
- Focused command passed: `go test ./internal/workflow -run 'TestSCN024_WorkflowCleanupGuidance(LabelsArtifactLifecycleActions|KeepsPendingContractBeforeArchiveCandidate)' -count=1`.

## Final mutation-gate remediation — workflow artifact lifecycle

### RED
- Final full review found only the active mutation gate failing: temp-copy changed-module mutation score was 77.42% (72/93 killed), below `.clean-workflow/quality-gates.yaml` threshold `mutation_score >= 0.80`.
- Survivor summary inspected from `/tmp/clean-workflow-mutation-review-full-summary.json` across `internal/workflow/ancora_state.go`, `artifact_source.go`, and `scenario_trace.go`.
- Added contract-focused tests for SCN-012, SCN-013, SCN-014, SCN-017, SCN-020, SCN-022, SCN-023, and SCN-024 to cover boundary behavior the surviving mutants obscured: partial tracked contract authority, either legacy collision, active-feature classification constraints, no-repair/no-checksum pointer validation, scenario-bearing rename repair, approved spec archive handling, example path secret rejection, non-feature planning skips, and cleanup reason precedence.

### GREEN
- No production code changes were required; survivors exposed missing behavioral assertions rather than incorrect current behavior.
- Focused workflow verification passed after gofmt: `go test ./internal/workflow -count=1`.

### REFACTOR
- Kept tests behavior-oriented and scenario-traceable, using temp repositories/files only.
- Targeted prior-survivor mutation rerun killed 18/21 previously surviving mutants. Combined with the unchanged 72 previously killed mutants, the comparable projected changed-module score is 90/93 = 96.77%, above the active 80% gate. Report: `/tmp/clean-workflow-mutation-remediation/targeted-survivor-rerun-summary.json`.
- Final gate evidence for this remediation is recorded in the session result: full suite, formatting, lint, diff whitespace, and mutation sweep rerun with temp outputs under `/tmp`.
