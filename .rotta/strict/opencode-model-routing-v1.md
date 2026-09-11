# OpenCode-only installer-managed model routing v1 — Strict approval contract

**Contract path:** `.rotta/strict/opencode-model-routing-v1.md`  
**Mode:** Strict  
**Status:** unapproved — this document authorizes neither implementation nor any operational action.  
**Canonical sources (stable this session):** `~/.config/opencode/skills/rotta-next/rotta-core/SKILL.md`; `~/.config/opencode/skills/rotta-next/rotta-orchestrator/SKILL.md`.

## Decision record

The installer-managed OpenCode model-routing capability is **enabled by default**. Its request is tri-state: omitted/unset means enabled by default; explicit `enabled` means enabled; explicit `disabled` means disabled. A plain boolean is not an acceptable representation because it cannot distinguish omission from an explicit choice.

The temporary repository-root `opencode.json` is user-owned and remains byte-for-byte unchanged. It is not migrated, removed, adopted, backed up, validated as managed state, or used as an installer write target. Any file selected through `OPENCODE_CONFIG` is likewise user-owned and remains byte-for-byte unchanged.

The OpenCode role set has exactly seven roles, including the new **architect** and **cleaner** roles. The CLI and keyboard-accessible TUI both expose an enabled/disabled routing selection and require an explicit confirmation before a routing-changing installation proceeds.

## Immutable routing map

When routing is enabled, the adapter shall set only the managed `model` field for the following OpenCode roles:

| Role | Model |
| --- | --- |
| orchestrator | `openai/gpt-5.6-sol` |
| architect | `openai/gpt-5.6-sol` |
| review | `openai/gpt-5.6-sol` |
| impl | `openai/gpt-5.6-terra` |
| ops | `openai/gpt-5.6-luna` |
| explore | `openai/gpt-5.6-luna` |
| cleaner | `openai/gpt-5.6-luna` |

This map is fixed for v1. No alternative model, provider, role-level override, Fast variant, or reasoning-selection UI is in scope.

## Approved implementation boundary, if subsequently approved

1. Add one permanent **OpenCode-only model-routing adapter boundary**. It may be called from installer, CLI, and TUI flows, but it owns only this routing concern; it is not a general provider-plugin abstraction.
2. The adapter may mutate only the resolved **XDG-global OpenCode configuration**: `${XDG_CONFIG_HOME:-~/.config}/opencode/opencode.json` (or the equivalent canonical XDG resolution). It must never select a project configuration as the effective write target and must ignore `OPENCODE_CONFIG` for mutation.
3. The adapter shall leave repository/project files and any `OPENCODE_CONFIG` target byte-identical, including the temporary root `opencode.json`. Normal OpenCode runtime merging is not permission for the installer to write an overlay.
4. Configuration ownership is field-level. For routing, Rotta owns only the `agent.<role>.model` field values it created and records enough per-field provenance to establish that ownership. It does not own, rewrite, hash-replace, or delete an entire agent object merely because it owns that model field.
5. The seven OpenCode role assets, their required OpenCode permissions, and their ownership/managed-artifact records must be present and reconciled as seven distinct assets. Architect and cleaner must have OpenCode-scoped assets; neither may be represented only by a project overlay. Routing enablement must cover all seven roles in the immutable map.
6. Existing user-owned fields, agent definitions, project files, and override files must not be overwritten. If a required operation would collide with a non-owned model field, an unrecognized role record, a malformed/unprovable ownership record, or a diverged managed model value, the installer must refuse before mutation, identify the conflict and remediation, and leave all affected files and records unchanged.
7. Disable is ownership-aware: it removes only a currently matching, recorded Rotta-owned `model` field for each mapped role, then removes that field's ownership record. It preserves agent objects, all non-model fields, all user-owned model fields, all role assets/permissions, and all project/override files. If ownership or current-value matching cannot be proven, disable must refuse rather than delete or overwrite a value.
8. Repeating a completed enabled request, a completed disabled request, or an omitted/default-enabled request must be idempotent: it must not create duplicate assets, backup entries, ownership records, or unrelated formatting/content changes.
9. Each global-config mutation must have transaction semantics: validate and plan before write; create a backup of the exact XDG-global target before its first mutation; write atomically; persist ownership only after the associated write succeeds; and compensate failures by restoring the exact prior target and managed state. A failed compensation is a surfaced failure, never silent continuation.
10. Backup and restore resolution must use the same XDG-global path as mutation. Custom `XDG_CONFIG_HOME` coverage is mandatory; hard-coded `~/.config/opencode` backup paths are prohibited. Backup/restore must also cover failure after a global-config write but before managed-state persistence, and failure after managed-state persistence when rollback is required.
11. CLI and TUI inputs must feed the same tri-state routing request into the adapter. CLI must expose explicit enabled and disabled selection; omission remains unset and therefore defaults to enabled. The TUI must be keyboard accessible, present enabled and disabled as selectable values with a visible current selection, and require a confirmation screen/action that states the pending selection before mutation.

## Public behavior and interface requirements

### Request shape and defaults

- The installer domain request shall represent routing as `unset`, `enabled`, or `disabled`, not `bool`.
- Omitted CLI and non-selected/default TUI entry produce `unset`; adapter policy resolves `unset` to `enabled` only at the model-routing boundary.
- Explicit CLI disable and explicit TUI disable both produce `disabled`; neither may be collapsed into omission.
- The confirmation payload must include whether the request was omitted/default-enabled, explicitly enabled, or explicitly disabled, so a user can verify the action that will occur.

### CLI and TUI

- The CLI shall offer a named OpenCode model-routing choice accepting only `enabled` or `disabled`; invalid values fail before any write. The exact flag spelling may be finalized during implementation, but its help text must state the enabled default and the scope of disable.
- The TUI shall support keyboard focus/navigation, selection of both values, a visible focus/selection state, Enter/space-equivalent selection as appropriate to the chosen component, and keyboard confirmation/cancellation. Pointer-only interaction is insufficient.
- Cancel, escape, back, invalid input, and a declined confirmation must perform no mutation.

### Ownership, reconciliation, and refusal

- Reconciliation may inspect the XDG-global OpenCode configuration and Rotta's managed-state data only as needed to plan this capability. It must not treat the project-root `opencode.json` or an `OPENCODE_CONFIG` file as installer-owned merely because OpenCode can merge it at runtime.
- A user field other than the exact owned `model` field is outside the adapter's authority. The adapter must preserve it byte/semantic-content-wise according to the configuration serializer's established formatting contract and must never delete an enclosing agent solely to remove routing.
- The managed-state schema must associate ownership with the global target and individual role/model field, not a whole-agent checksum that confers whole-object deletion authority.
- A collision or uncertain state is a refusal, not a merge heuristic. Refusal must happen before a partial write and must identify the global target, role, and field in error output.

## Required focused evidence

Implementation must add focused unit/integration/feature coverage for all of the following, without requiring a repository-wide suite unless separately requested:

1. Omitted request resolves to enabled and produces the immutable seven-role map only in an XDG-global configuration.
2. Explicit CLI enabled and disabled map to distinct tri-state values; CLI disable removes only owned matching model fields.
3. The TUI offers keyboard navigation to enabled/disabled, renders the selected choice, reaches confirmation, and mutates nothing on cancel/decline.
4. Every one of the seven assets has its required OpenCode permission declaration and managed ownership coverage; architect and cleaner are included.
5. Root project `opencode.json` and an `OPENCODE_CONFIG` target are byte-identical before and after enabled, disabled, idempotent, refused, and failed requests.
6. Non-owned models, non-model agent fields, unknown/unmanaged roles, malformed ownership, and diverged owned values are preserved and cause refusal where the requested model operation would otherwise touch them.
7. Enable/disable/idempotent re-runs do not duplicate assets, records, or backups, and do not rewrite unrelated fields.
8. Custom `XDG_CONFIG_HOME` backup and restore addresses the actual custom global target; injected failures at each transaction boundary restore configuration and managed state exactly.

## Material Gherkin scenarios

```gherkin
Feature: Installer-managed OpenCode model routing

  Background:
    Given the OpenCode global configuration target is resolved from XDG_CONFIG_HOME
    And the repository-root opencode.json is user-owned
    And OPENCODE_CONFIG, when set, names a user-owned configuration file

  Scenario: Omitted installer routing request defaults to enabled
    Given no OpenCode model-routing option was supplied
    When the installer is confirmed
    Then only the XDG-global OpenCode configuration may be mutated
    And the seven role model fields equal the immutable routing map
    And the repository-root opencode.json and OPENCODE_CONFIG file are byte-identical
    And field-level ownership is recorded for exactly those model fields

  Scenario: CLI explicitly disables routing
    Given all seven routing model fields match recorded Rotta field ownership
    When the user selects OpenCode model routing "disabled" through the CLI and confirms
    Then only those seven owned model fields are removed
    And each role's remaining agent fields, asset, permission, and object remain present
    And the corresponding model-field ownership records are removed
    And project and OPENCODE_CONFIG files are byte-identical

  Scenario: TUI selection is keyboard accessible and confirmation-gated
    Given the installer TUI is on the OpenCode model-routing selection
    When the user navigates by keyboard to "disabled"
    And confirms that selection by keyboard on the confirmation screen
    Then the request submitted to the adapter is explicitly disabled
    And no mutation occurs before confirmation
    But when the user cancels or declines confirmation
    Then no configuration, asset, backup, or ownership record is changed

  Scenario: User-owned configuration is preserved and conflicts refuse safely
    Given a mapped role has a non-owned model field or an ownership record that cannot prove a matching Rotta value
    When enable or disable would touch that role's model field
    Then the installer refuses before mutation with the target, role, and field identified
    And the XDG-global configuration and managed state are unchanged
    And the repository-root opencode.json and OPENCODE_CONFIG file are byte-identical

  Scenario: Disable honors field-level ownership only
    Given a mapped agent has a Rotta-owned matching model field and user-owned non-model fields
    And another mapped agent has a user-owned or diverged model field
    When a disabled routing request is confirmed
    Then the matching owned model field is removed without deleting its agent
    And the user-owned non-model fields are unchanged
    And the operation refuses rather than deleting or changing the user-owned or diverged model field

  Scenario: Custom-XDG transaction failure restores exact state
    Given XDG_CONFIG_HOME points to a custom directory containing the global OpenCode configuration
    And a backup of that exact target is created before the first routing mutation
    When an injected failure occurs after a routing write or managed-state update
    Then restoration uses the custom-XDG backup path
    And the global configuration and managed state equal their pre-request contents
    And no partial routing ownership or stale backup record remains
```

## Explicit non-goals

- No host or provider beyond OpenCode and the three immutable `openai/gpt-5.6-*` model identifiers above.
- No Fast variants, model picker, reasoning-effort selection, or arbitrary model/provider configuration.
- No migration, deletion, adoption, or removal of the project-overlay/root `opencode.json`.
- No mutation of a project file or `OPENCODE_CONFIG` file, even if OpenCode runtime layering would apply it.
- No runtime OpenCode restart, model invocation, provider use, or validation by invoking a model.
- No wholesale host-installer refactor, provider-plugin framework, or unrelated installer/configuration redesign.
- No implementation, tests, installer execution, OpenCode operation, configuration update, dependency installation, commit, or other operational action under this unapproved contract.

## Approval gate

Implementation may begin only after the user explicitly approves the final SHA-256 digest reported with this exact contract. Approval authorizes only the bounded capability and focused evidence above; any change to the immutable map, write boundary, ownership/refusal rules, protected files, or non-goals requires a contract amendment and fresh explicit approval.

**Current state:** no approval and no implementation occurred while rendering this contract.
