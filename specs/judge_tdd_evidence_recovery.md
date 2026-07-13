# Hard Spec: Judge TDD Evidence Recovery

## Adversarial Pre-Mortem
- Failure mode 1: A recovery log is written as though the original SCN-223 through SCN-225 cycles occurred, converting missing history into fabricated evidence and repeating the Judge failure.
- Failure mode 2: New behavior-equivalent scenarios are implemented with a passing baseline rather than an isolated RED → GREEN → REFACTOR cycle, so they cannot independently establish the required evidence.
- Failure mode 3: The recovery scenarios are completed but the approval and implementation markers still describe incompatible scenario scopes, leaving the submission ineligible before objective gates run.

## Hidden Assumptions
- The current behavior represented by SCN-223 through SCN-225 can be independently exercised from a clean baseline without relying on historical command output.
- A human with authority over the existing approval state can decide whether the original approved IDs remain in scope or are formally superseded by the recovery contract.
- The TDD Craftsman can preserve a reproducible record of each new scenario's actual failing test, passing test, and behavior-preserving refactor run.
- The implementation and approval markers have a normal lifecycle process capable of recording an exact submitted scope without inventing completion evidence.

## Alternatives Considered
| Approach | Reason Rejected |
|----------|----------------|
| Fill the missing RED or REFACTOR rows for SCN-223 through SCN-225 from inference or current results | The Judge explicitly rejects this as invented historical evidence. |
| Submit the existing SCN-223 through SCN-225 GREEN results again | A pre-existing GREEN result cannot demonstrate strict TDD. |
| Add only process documentation while leaving marker scopes unchanged | Documentation cannot satisfy the hard precondition that every approved scenario has truthful TDD coverage. |
| Reimplement equivalent observable behavior under new scenarios and record a new strict TDD cycle | Retained: it creates fresh, reproducible evidence without misrepresenting history, provided the old approval scope is reconciled by an authorized lifecycle decision. |

## Summary
This recovery contract defines four new, pending-approval scenarios: three independently observable equivalents of the behavior covered by SCN-223 through SCN-225, and one lifecycle scenario for reconciling the submitted approval and implementation scopes. It does not amend historical logs, tests, production behavior, existing features, or markers. After explicit approval and resolution of the approval-scope question, each behavior scenario must be reimplemented or independently re-exercised through a recorded strict RED → GREEN → REFACTOR cycle. The lifecycle work must truthfully reconcile the markers before review; no marker may imply completion, approval, or TDD evidence that does not exist.

## Requirements

### REQ-028: Recover portable managed-command serialization through fresh TDD
**Description:** A new recovery scenario must establish that Rotta-managed MCP executable commands are serialized as canonical bare commands rather than resolved absolute or versioned executable locations.
**Acceptance Criteria:**
- A new approved recovery scenario covers a supported managed MCP server whose executable resolves to a versioned or absolute location.
- The observable serialized command is the canonical bare command.
- The serialized executable command does not expose the resolved absolute or versioned location.
- Its implementation records an actual, reproducible RED → GREEN → REFACTOR cycle under the new scenario ID.
**Edge Cases:**
- A Homebrew Cellar path includes a version segment.
- A manually installed executable resolves under a user-local absolute path.
- Non-executable arguments may legitimately contain a slash and must not be classified as an executable path.
**Out of Scope:**
- Reconstructing or relabeling the historical SCN-223 TDD cycle.

### REQ-029: Recover managed stale-command normalization through fresh TDD
**Description:** A new recovery scenario must establish that reinstall normalizes a proven Rotta-managed stale executable command and is idempotent on a subsequent reinstall.
**Acceptance Criteria:**
- A new approved recovery scenario begins with a proven Rotta-managed entry using a stale versioned executable command.
- Reinstall changes only that managed executable command to its canonical bare command.
- The recovery reports that the managed entry was normalized.
- A subsequent reinstall makes no further command-field change.
- Its implementation records an actual, reproducible RED → GREEN → REFACTOR cycle under the new scenario ID.
**Edge Cases:**
- The stale path is from a different installed version.
- The managed entry contains additional valid arguments.
- Reinstall runs more than once after normalization.
**Out of Scope:**
- Claiming that SCN-224 previously ran RED or REFACTOR.

### REQ-030: Recover preservation of non-command absolute references through fresh TDD
**Description:** A new recovery scenario must establish that executable normalization does not rewrite unrelated absolute references in the same host configuration.
**Acceptance Criteria:**
- A new approved recovery scenario contains both a proven managed MCP executable path and a separate absolute generated hook-script reference.
- Reinstall normalizes the managed MCP executable command to its bare name.
- Reinstall retains the non-command absolute hook-script reference unchanged.
- Its implementation records an actual, reproducible RED → GREEN → REFACTOR cycle under the new scenario ID.
**Edge Cases:**
- The hook reference uses a `file://` URL or an absolute filesystem path.
- The hook reference includes a slash while the executable command is the only field eligible for normalization.
- An unproven managed entry remains outside the normalization scope.
**Out of Scope:**
- Retrofitting a RED phase into SCN-225 after its test was already green.

### REQ-031: Reconcile review-submission lifecycle markers truthfully
**Description:** Before a recovery submission reaches review, lifecycle markers must identify one exact submitted scenario scope and must not attest to scenarios lacking required evidence.
**Acceptance Criteria:**
- The authorized lifecycle decision explicitly records whether SCN-223 through SCN-225 remain approved for the submission or are superseded by SCN-231 through SCN-233.
- The approval marker's submitted scope has TDD-log coverage for every listed scenario, including any retained legacy scenario IDs.
- The implementation-complete marker lists exactly the scenarios completed for the submitted scope and no scenario lacking completion evidence.
- If legacy scenarios remain approved, the recovery does not treat new scenario evidence as historical evidence for those legacy IDs.
- Marker changes occur only through the normal lifecycle process after the relevant approval and implementation conditions are met.
**Edge Cases:**
- SCN-002 and SCN-218 through SCN-222 remain in the existing approval marker but lack TDD-log entries.
- A human approves recovery scenarios but does not authorize supersession of legacy approved scenarios.
- A partial recovery is complete while another scenario remains pending.
**Out of Scope:**
- Editing `specs/.approved`, `specs/.implementation-complete`, or `.rotta/tdd-log.md` in this spec-authoring task.

## Open Questions
- Does the authorized lifecycle owner require SCN-223 through SCN-225 to remain in `specs/.approved` and receive fresh evidence under their original IDs, or may they be formally superseded by SCN-231 through SCN-233 for the recovery submission?
- How will the authorized lifecycle owner reconcile the other currently approved but uncovered IDs (SCN-002 and SCN-218 through SCN-222): supply their truthful TDD coverage, or formally narrow/supersede the approved scope?
- What marker format or authorized workflow record demonstrates supersession while preserving the original IDs' historical traceability?

## Trade-offs
- New scenario IDs preserve historical truth but require an explicit approval-scope decision instead of silently treating replacement evidence as evidence for legacy IDs.
- Exact marker reconciliation adds workflow work, but it prevents another precondition stop before quality gates can run.
- Recording fresh strict TDD cycles costs more than documenting current green behavior, but it is the only reproducible evidence the Judge can accept.

## Risk Level
critical — Justification: A false recovery record would compromise the audit trail, while an unreconciled marker scope will block review before any current quality evidence is evaluated.
