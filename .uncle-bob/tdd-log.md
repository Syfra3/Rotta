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
