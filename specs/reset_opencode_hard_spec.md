# Hard Spec: Reset and Reinstall Global OpenCode

## Adversarial Pre-Mortem
- Failure mode 1: The target removes only default paths while OpenCode uses custom XDG or `OPENCODE_CONFIG_DIR` paths, leaving credentials, sessions, or configuration behind and making the reset incomplete.
- Failure mode 2: An empty, relative, root, or home-directory-valued environment variable expands into an overly broad deletion target, destroying unrelated user files.
- Failure mode 3: Deletion succeeds but the network, installer endpoint, shell, or installer fails, leaving OpenCode uninstalled; this is an accepted consequence of the required destructive, no-confirmation workflow but must be reported as failure.

## Hidden Assumptions
- The target is executed on Linux in a POSIX-compatible shell with `curl`, `bash`, and the official installer endpoint reachable.
- Default OpenCode global locations follow the XDG defaults: `~/.config/opencode`, `~/.local/share/opencode`, and `~/.cache/opencode`.
- When set, each XDG variable is an absolute directory root; OpenCode's entry within it is the `opencode` child directory.
- When set, `OPENCODE_CONFIG_DIR` designates an OpenCode-only global configuration directory, rather than a shared directory containing unrelated files.
- Project-local `opencode.json` and `.opencode/` are outside the requested global-only scope and must survive the target.
- The official installer command supplied by the requester is authoritative and may install a current OpenCode version rather than a pinned version.

## Alternatives Considered
| Approach | Reason Rejected |
|----------|-----------------|
| Delete only `~/.config/opencode`, `~/.local/share/opencode`, and `~/.cache/opencode` | Fails to remove global OpenCode artifacts when XDG locations or `OPENCODE_CONFIG_DIR` are customized. |
| Require a confirmation prompt or a `FORCE=1` flag | Explicitly rejected: the requester requires unattended execution with no confirmation. |
| Delete all contents of configured XDG roots | Unsafe: XDG roots commonly contain unrelated applications' configuration, data, and cache. |
| Add a standalone reset script | Unnecessary implementation surface; the requested interface is a Makefile target and existing Makefile targets contain their shell logic directly. |

## Summary
Add a Linux-only `make reset-opencode` target that, without prompting, removes all global OpenCode configuration, data, cache, and credentials from the default XDG locations and from explicitly configured XDG and OpenCode configuration locations, while preserving project-local OpenCode files. After successful cleanup, it must run exactly `curl -fsSL https://opencode.ai/install | bash` to install OpenCode again. The target must bound every deletion to an identified OpenCode path and fail rather than risk deleting a broad or unrelated filesystem location.

## Requirements

### REQ-001: Expose the Reset Target Through Existing Makefile Conventions
**Description:** The Makefile must provide a phony target named `reset-opencode` and document it in the existing `help` target.
**Acceptance Criteria:**
- `make reset-opencode` invokes the reset-and-reinstall workflow.
- `reset-opencode` is included in `.PHONY`.
- `make help` lists `reset-opencode` with a concise warning that it removes global OpenCode state and reinstalls OpenCode.
**Edge Cases:**
- A user invokes the target from any working directory through this repository's Makefile.
- A prior OpenCode installation or any OpenCode directory is absent.
**Out of Scope:**
- Adding a CLI subcommand, standalone script, interactive UI, or non-Linux implementation.

### REQ-002: Remove Default Global OpenCode Artifacts
**Description:** The target must remove the default global OpenCode directories that contain configuration, data, cache, and credentials when the relevant XDG variable is unset.
**Acceptance Criteria:**
- With `XDG_CONFIG_HOME` unset, the target removes only `~/.config/opencode` for default global configuration.
- With `XDG_DATA_HOME` unset, the target removes only `~/.local/share/opencode` for data, sessions, logs, and credentials such as `auth.json`.
- With `XDG_CACHE_HOME` unset, the target removes only `~/.cache/opencode` for cache and cached binaries.
- Missing target directories do not prevent the workflow from proceeding to installation.
**Edge Cases:**
- Any subset of the default directories is missing, unreadable, or already removed.
- A path contains spaces or shell-special characters through the user's home directory.
**Out of Scope:**
- Deleting `~/.config`, `~/.local/share`, `~/.cache`, or any non-OpenCode child within those directories.

### REQ-003: Remove OpenCode Entries From Custom Global Locations
**Description:** When custom locations are set, the target must remove only their OpenCode-specific paths, never the full custom XDG root or unrelated content.
**Acceptance Criteria:**
- When `XDG_CONFIG_HOME` is set, the target removes only `${XDG_CONFIG_HOME}/opencode` instead of the default config location.
- When `XDG_DATA_HOME` is set, the target removes only `${XDG_DATA_HOME}/opencode` instead of the default data location.
- When `XDG_CACHE_HOME` is set, the target removes only `${XDG_CACHE_HOME}/opencode` instead of the default cache location.
- When `OPENCODE_CONFIG_DIR` is set, the target removes that configured OpenCode configuration directory in addition to the XDG-derived configuration directory.
- Before deletion, the target rejects an unsafe configured path that is empty, relative, `/`, the user's home directory, or otherwise resolves to a path that is not a distinct OpenCode-only target; it must fail without performing that unsafe deletion.
- A custom path with spaces or shell-special characters is handled as one filesystem path.
**Edge Cases:**
- One, several, or all custom variables are set.
- A custom OpenCode location equals a default or another configured location; it is removed at most once without error.
- A custom OpenCode location does not exist.
- `OPENCODE_CONFIG_DIR` is set while all XDG variables are unset, or vice versa.
**Out of Scope:**
- Removing any sibling or parent directory of an OpenCode-specific path.
- Searching arbitrary filesystems for OpenCode artifacts not addressed by the default or explicitly configured locations.
- Removing OpenCode files placed by users in unrelated custom locations without one of the specified environment variables.

### REQ-004: Preserve Project-Local OpenCode Configuration
**Description:** The reset is limited to global user state and must leave repository/project OpenCode configuration untouched.
**Acceptance Criteria:**
- The target does not delete or modify a project-local `opencode.json`.
- The target does not delete or modify any project-local `.opencode/` directory or its contents.
- The target does not traverse the repository or other project directories to locate OpenCode files.
**Edge Cases:**
- The target is invoked from a project that contains both `opencode.json` and `.opencode/`.
- A project-local OpenCode path has the same name as a global path component.
**Out of Scope:**
- Resetting per-project agents, commands, skills, plugins, session state, or project configuration.

### REQ-005: Reinstall With the Required Official Linux Installer
**Description:** After the required cleanup completes, the target must invoke the exact official installer command specified by the requester.
**Acceptance Criteria:**
- The install command is exactly `curl -fsSL https://opencode.ai/install | bash`.
- The command runs only after all required cleanup operations complete successfully or are no-ops because their targets are absent.
- The target requires no confirmation prompt, confirmation variable, or interactive approval before deletion or installation.
- If cleanup validation/removal fails, the target exits non-zero and does not start installation.
- If the installer command fails, the target exits non-zero and reports the failure through Make/shell status.
**Edge Cases:**
- `curl` is missing, the endpoint is unavailable, or the installer exits non-zero after cleanup.
- The installer is rerun after a partial previous reset.
- The target runs non-interactively, including CI or a script.
**Out of Scope:**
- Pinning an OpenCode version, verifying the downloaded installer beyond the specified command's behavior, rollback after failed installation, or restoring deleted configuration/credentials.

## Open Questions
- None. The target name, deletion boundaries, no-confirmation behavior, project-local exclusions, and installer command are explicitly defined.

## Trade-offs
- The no-confirmation requirement supports automation but makes global OpenCode state deletion immediate and irreversible.
- Refusing unsafe custom paths prevents broad deletion but can require a user to correct an invalid environment variable before the reset can proceed.
- Using the official unpinned installer ensures the current official installation path but sacrifices version reproducibility and cannot restore deleted state if installation fails.
- Preserving project-local files avoids repository mutation but means project-specific OpenCode configuration is not reset.

## Risk Level
high — Justification: The target irreversibly deletes global configuration, sessions, credentials, data, and cache without confirmation, and then depends on a network-delivered installer. Strict path validation limits accidental deletion but cannot eliminate the operational consequence of a failed reinstall after cleanup.
