# Hard Spec: Safe Installer Backup, Clean Install, and Full Restore

## Adversarial Pre-Mortem
- Failure mode 1: The installer deletes or overwrites existing AI-agent configuration before a complete backup exists, leaving users unable to recover their prior setup.
- Failure mode 2: Restore overwrites the current configuration and then fails midway, replacing a known-bad installation with an even more inconsistent state.
- Failure mode 3: The backup scope misses indirect files written by optional integrations, so a user-visible rollback appears successful while opencode, Claude Code, or project settings remain polluted.

## Hidden Assumptions
- The installer can enumerate every path it may touch before mutating any path.
- The current install flow is allowed to change from additive/idempotent behavior to backup-first cleanup-and-reinstall behavior for normal installs.
- A full-backup restore is acceptable even when a user only wants one file recovered.
- Timestamped local backups under the user's home directory are acceptable as the default recovery location.
- Optional integrations may mutate files not directly written by rotta code; safe rollback therefore requires backing up broader config roots, not only the exact destination files.

## Alternatives Considered
| Approach | Reason Rejected |
|----------|----------------|
| Continue additive install without cleanup | Leaves stale agents, permissions, and instructions in place and does not satisfy the requested clean reinstall guarantee. |
| Back up only files that exist in the install result | The install result is produced after mutation and may miss files modified by optional setup tools or cleanup. |
| Selective per-file restore | Increases UI and safety complexity; the requested recovery behavior is full backup restore only. |
| Store backups inside the target project | Project deletion, git cleanup, or project moves could remove recovery data needed for global agent settings. |

## Summary
Add installer recovery tooling so every normal install first creates a timestamped backup of the user's affected AI-agent and project configuration, then removes previous rotta installation artifacts/settings, then installs rotta fresh. Users must be able to list backups, preview backup metadata and contents, and confirm a full restore from the terminal UI. Backup failure must abort installation before mutation. Restore failure must roll back to the state that existed immediately before the restore attempt.

## Invariants
- No normal install path may mutate opencode, Claude Code, project `.rotta`, project `.vela`, Ancora, or Vela-related configuration before a successful backup is recorded.
- Restore is all-or-nothing at the backup-set level; there is no selective restore workflow.
- Restore must create a pre-restore safety backup of the current configuration before overwriting anything.
- Cleanup must remove only rotta-owned entries/files/directories or integration files in the explicitly defined restore/install scope; unrelated user configuration must be preserved.
- Generated artifacts, code comments, UI copy, docs, commit text, and summaries must use neutral wording and must not mention or identify the external behavioral precedent supplied to the workflow.

## Requirements

### REQ-001: Enumerate Backup Scope Before Mutation
**Description:** The installer must compute a backup scope before any install, cleanup, or restore mutation. The scope must cover all files/directories rotta currently writes or patches, plus broader agent configuration roots where required for safe rollback.
**Acceptance Criteria:**
- The backup scope includes `<project>/.rotta/state-machine.yaml` and `<project>/.rotta/quality-gates.yaml`.
- The backup scope includes `<project>/.vela/graph.db` when Vela setup is enabled or a previous Vela graph may be cleaned/restored.
- The backup scope includes `~/.config/opencode/opencode.json`, `~/.config/opencode/opencode.jsonc` if present, `~/.config/opencode/instructions.md` if present, and `~/.config/opencode/skills/rotta-orchestrator`, `rotta-spec`, `rotta-impl`, and `rotta-review` when opencode is targeted or existing opencode cleanup is relevant.
- The backup scope includes `~/.claude/settings.json`, `~/.claude/skills/rotta`, `~/.claude/mcp/ancora.json` if present, `~/.claude/vela-mcp.json` if present, and `~/.claude/vela-instructions.md` if present when Claude Code is targeted or existing Claude Code cleanup is relevant.
- The backup scope may include the whole `~/.config/opencode` and `~/.claude` configuration roots when needed to preserve safe rollback of project/tool settings touched by external setup commands.
- The scope records missing paths as metadata rather than treating absence as an error.
**Edge Cases:**
- The project path is empty or `~` and resolves to the user's home directory.
- Both opencode and Claude Code are targeted and share optional integrations.
- Optional setup is skipped during the new install but stale optional integration files from a prior install exist and must still be backed up before cleanup.
**Out of Scope:**
- Backing up unrelated system packages or binaries installed by package managers.

### REQ-002: Create Timestamped Backups for Every Normal Install
**Description:** Every normal install must create a backup under `~/.rotta/backups/{timestamp}/` before cleanup or installation begins.
**Acceptance Criteria:**
- Starting an install through the TUI creates a backup first.
- Starting an install through a non-interactive install command creates a backup first.
- The backup directory contains a manifest with timestamp, project path, target, selected modes, optional integration choices, backed-up paths, missing paths, backup status, and rotta version when available.
- Backup paths preserve enough relative structure to restore files to their original absolute destinations.
- Backups are never overwritten by a later install; timestamp collisions are resolved deterministically by adding a stable suffix or rejecting before mutation.
**Edge Cases:**
- Some scoped files do not exist yet.
- Backup directory creation succeeds but a file copy fails.
- The user's home directory cannot be resolved.
**Out of Scope:**
- Remote backup synchronization.

### REQ-003: Abort Install Completely on Backup Failure
**Description:** If backup creation fails, installation must stop before cleanup or install mutations occur.
**Acceptance Criteria:**
- A failed backup returns an install failure result and displays a recovery-safe error to the user.
- No cleanup runs after a failed backup.
- No rotta files, agent entries, permissions, instructions, or integration configuration are written after a failed backup.
- Partial backup artifacts are either removed or marked unusable in the manifest so they are not offered as restore candidates.
**Edge Cases:**
- Permission denied while reading a scoped file.
- Disk full while writing the backup.
- Manifest write fails after file copies succeed.
**Out of Scope:**
- Continuing install with a warning after backup failure.

### REQ-004: Clean Previous Installation Before Fresh Install
**Description:** After a successful backup and before writing the fresh installation, the installer must remove previous rotta-owned installation artifacts and settings for the selected scope.
**Acceptance Criteria:**
- Previous rotta skill directories are removed before fresh skill files are written.
- Previous opencode rotta agent entries are removed before fresh agent entries are added.
- Previous Claude Code rotta permission entries are removed or normalized before current permissions are applied.
- Previous project `.rotta` generated config files are replaced with the current embedded defaults.
- Stale rotta-managed Vela/Ancora integration files or entries in the selected scope are removed or normalized before current optional setup runs.
- Cleanup preserves unrelated user settings and unrelated agent entries.
**Edge Cases:**
- Prior install contains only a subset of modes.
- Prior install was partially completed or manually edited.
- User has unrelated opencode agents or Claude Code permissions adjacent to rotta entries.
**Out of Scope:**
- Removing user-created files that merely mention rotta but are not installer-owned.

### REQ-005: Provide CLI Recovery Commands Consistent With Existing CLI Style
**Description:** The command-line interface must expose backup, install, and restore operations using names consistent with the existing minimal CLI.
**Acceptance Criteria:**
- The CLI supports a direct install path equivalent to a clean install, such as `rotta install --clean`, while preserving existing `version`/`--version` behavior.
- The CLI supports listing or creating backups through a backup command such as `rotta backup`.
- The CLI supports restoring a full backup through a restore command such as `rotta restore`.
- If the final command names differ, they must remain discoverable from CLI help and must not allow a normal install to skip backup.
**Edge Cases:**
- Unknown commands should fail without launching an install.
- Existing TUI launch behavior with no command must remain available.
**Out of Scope:**
- Designing a full shell completion system.

### REQ-006: Expose Recovery in the TUI
**Description:** The terminal UI must expose a recovery option that lets users list backups, preview details, and confirm full restore.
**Acceptance Criteria:**
- The TUI provides a user-visible path to recovery before starting a new install.
- Users can list available backups from `~/.rotta/backups/`.
- Users can preview a backup's metadata, including timestamp, project path, target, selected modes, optional integrations, backed-up paths, and missing paths.
- Users must explicitly confirm before restore begins.
- The UI communicates that restore is full-backup restore, not selective restore.
**Edge Cases:**
- No backups exist.
- A backup directory exists but has no valid manifest.
- Terminal is too small to show all paths at once.
**Out of Scope:**
- File-by-file selection in the TUI.

### REQ-007: Restore Full Backup Atomically From User Perspective
**Description:** Restoring a backup must restore all backed-up files and remove paths recorded as absent at backup time, after first protecting the current configuration.
**Acceptance Criteria:**
- Restore creates a pre-restore safety backup of the current in-scope configuration before overwriting or deleting anything.
- Restore copies every backed-up file/directory to its original destination.
- Restore removes rotta-scoped destination paths that were recorded as missing in the selected backup, when those paths exist at restore time.
- Restore reports success only after all destination changes complete.
- Restore does not offer selective path restore.
**Edge Cases:**
- Destination parent directories no longer exist.
- Current files are read-only.
- The selected backup was made for a different project path.
**Out of Scope:**
- Merging restored JSON with current JSON.

### REQ-008: Roll Back Failed Restore to Pre-Restore State
**Description:** If restore fails after mutation begins, the system must roll back to the pre-restore state captured immediately before the restore attempt.
**Acceptance Criteria:**
- A failed restore attempts to restore the pre-restore safety backup automatically.
- The user receives a failure message that identifies the selected backup and whether rollback to pre-restore state succeeded.
- A restore is not reported successful if rollback was required.
- If rollback also fails, the error identifies the location of both the selected backup and the pre-restore safety backup for manual recovery.
**Edge Cases:**
- Failure occurs during deletion of a path that should be absent.
- Failure occurs during copy of a nested directory.
- Rollback cannot recreate a directory due to permissions.
**Out of Scope:**
- Guaranteeing rollback after external processes mutate files concurrently during restore.

### REQ-009: Manifest and Preview Must Be Stable and Neutral
**Description:** Backup metadata and generated user-facing text must be stable enough for tests and must avoid external-reference wording.
**Acceptance Criteria:**
- The manifest schema includes a version field so future backup formats can be handled explicitly.
- Backup previews are derived from manifest data, not by scanning arbitrary directories at display time.
- Generated artifacts, UI strings, docs, comments, and summaries do not mention or identify the external behavioral precedent supplied to the workflow.
- Acceptance criteria and scenario titles use neutral recovery terminology only.
**Edge Cases:**
- Manifest version is unknown.
- Manifest has extra fields from a newer version.
- Backup path contains spaces or non-ASCII characters.
**Out of Scope:**
- Encrypting backup metadata.

### REQ-010: Preserve Existing Installer Behavior Except Where Safety Requires Change
**Description:** Existing target selection, project path selection, mode selection, Ancora setup, Vela setup, and success/error reporting must continue to work after backup-first installation is introduced.
**Acceptance Criteria:**
- Existing TUI screens remain reachable unless intentionally superseded by the recovery entry point.
- Existing install options still determine which current rotta files are installed after cleanup.
- The install success summary includes the backup location or a clear way to find it.
- The install error path distinguishes backup failure, cleanup failure, install failure, and restore failure.
**Edge Cases:**
- User cancels from the recovery option back to install.
- User cancels from install confirmation.
- Install succeeds after cleaning a partial previous installation.
**Out of Scope:**
- Changing rotta's core spec/implementation/review workflow semantics.

## Non-Goals
- Do not implement selective restore.
- Do not back up package-manager installed binaries or attempt to uninstall package-manager dependencies.
- Do not migrate unrelated opencode or Claude Code settings.
- Do not introduce cloud synchronization, encryption, or compression unless existing project conventions already require it.
- Do not implement production code as part of this spec artifact.

## Open Questions
- None.

## Trade-offs
- Backing up broader config roots increases disk usage but reduces rollback risk for files modified by optional setup tools outside direct rotta writes.
- Full restore is safer and easier to reason about than selective restore, but users cannot recover one file through the supported UI.
- Cleanup before fresh install removes stale rotta-owned configuration, but implementation must carefully preserve unrelated user customization.

## Risk Level
high — Justification: The feature intentionally mutates user-level AI-agent configuration and project settings; incorrect ordering or incomplete backup scope can cause data loss or broken agent installations.
