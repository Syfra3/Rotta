Feature: Vela v2 workflow and local graph intelligence
  The project needs one auditable workflow that uses local structural evidence
  without allowing graph tools or specialist workers to authorize delivery.

  @SCN-701 @REQ-101 @REQ-104
  Scenario: Explicit NEW ignores legacy artifacts and initializes Draft from no prior v2 state
    Given the human explicitly requests NEW v2 submission "submission-A"
    And no v2 contract or submission ledger exists for "submission-A"
    And a legacy approval or lifecycle artifact is present and names the same feature
    When authorized NEW initialization is requested
    Then the legacy artifact has no effect on phase selection or approved scope
    And "submission-A" is initialized only in Draft with an initial Draft contract/submission ledger
    And it has no approved scope and does not enter a later phase

  @SCN-702 @REQ-108 @REQ-109 @REQ-117
  Scenario: Fresh local graph evidence produces a bounded Draft exploration packet
    Given the orchestrator selected base commit "base-A"
    And the local Vela graph is complete and identifies "base-A" as its graph commit
    When Draft requests index preflight and exploration
    Then the preflight result is "fresh" for "base-A"
    And one bounded exploration packet identifies affected modules, impact risks, architectural constraints, confidence, and evidence pointers
    And Draft made no more than 5 Vela queries in total
    And the packet lists no more than 10 affected modules or symbols, 10 risks, and 5 architectural constraints
    And the packet contains no raw graph dump or raw tool log
    And the packet records "base-A" and its graph status
    And no lifecycle transition is made by Vela

  @SCN-703 @REQ-108 @REQ-109
  Scenario: Missing graph is extracted only after human consent
    Given the orchestrator selected base commit "base-A"
    And no local Vela graph is available for that repository
    When Draft requests index preflight
    Then the human is asked whether to extract or index the graph
    And no extraction starts before the human responds
    When the human consents and the resulting graph identifies "base-A" with no incompleteness
    Then preflight records a fresh graph for "base-A"
    And Draft receives an exploration packet tied to "base-A"

  @SCN-704 @REQ-108 @REQ-109 @REQ-117
  Scenario: Stale graph falls back visibly when the human declines re-indexing
    Given the orchestrator selected base commit "base-B"
    And the local Vela graph identifies "base-A" as its graph commit
    When Draft requests index preflight
    Then preflight reports the graph as stale for "base-B"
    And the human is asked whether to re-index it
    When the human declines re-indexing
    Then Draft continues with an exploration packet that names the stale graph and unknown structural areas
    And the packet does not present stale findings as fresh confidence

  @SCN-705 @REQ-108 @REQ-109
  Scenario: Incomplete graph remains uncertain after consented indexing
    Given the orchestrator selected base commit "base-A"
    And graph preflight reports missing coverage for part of the repository
    When the human consents to re-indexing
    And the resulting extraction still reports omitted files
    Then the preflight result records the incompleteness for "base-A"
    And the Draft exploration packet identifies affected findings as uncertain where coverage is missing
    And the graph is not classified as fresh

  @SCN-706 @REQ-109 @REQ-102
  Scenario: Contract uses the Draft packet without repeating broad exploration
    Given Draft produced a bounded exploration packet for selected base commit "base-A"
    When the Spec plus Gherkin worker prepares the Contract
    Then the hard spec and scenarios can trace relevant constraints to the packet
    And the Contract does not request another broad repository exploration
    And the packet itself does not add an unapproved behavioral requirement

  @SCN-707 @REQ-105 @REQ-104
  Scenario: Explicit approval of an unchanged Contract authorizes TDD
    Given Contract contains an unchanged hard spec and approved Gherkin scenarios
    And its fingerprint is "contract-F"
    When the human explicitly approves fingerprint "contract-F"
    And an authorized operations task creates and verifies contract commit "contract-C"
    Then the ledger records "contract-F", "contract-C", and the approved scenario IDs atomically
    And the orchestrator may authorize the TDD phase

  @SCN-708 @REQ-105 @REQ-102
  Scenario: Changed Contract requires renewed human approval
    Given a human approved Contract fingerprint "contract-F"
    And the hard spec or Gherkin contract changes to fingerprint "contract-G"
    When the orchestrator considers entering TDD
    Then approval of "contract-F" is rejected as stale
    And the submission remains in Contract until the human approves "contract-G"

  @SCN-709 @REQ-103 @REQ-115
  Scenario: Specialist worker cannot advance lifecycle state
    Given a submission is in Contract at ledger version 8
    And a review worker returns evidence claiming that the submission should enter Archive
    When the lifecycle persistence service receives the worker-originated transition request
    Then it rejects the request as unauthorized
    And the durable status remains Contract at ledger version 8
    And the rejection reports the expected authorizer and safe next action

  @SCN-710 @REQ-103 @REQ-104 @REQ-115
  Scenario: Delayed transition evidence cannot overwrite a newer lifecycle state
    Given a TDD worker was authorized against ledger version 12
    And the orchestrator already persisted a later valid transition at ledger version 13
    When the delayed worker result requests its original transition
    Then persistence rejects the stale ledger version
    And it preserves the recorded state and both evidence references for audit

  @SCN-711 @REQ-106 @REQ-107 @REQ-115
  Scenario: Resume uses the recorded persistent worktree only
    Given a TDD submission is explicitly requested as RESUME/RECOVERY
    And valid matching durable contract and ledger state identify its one persistent worktree, repository identity, branch, and expected commit "contract-C"
    When the workflow resumes
    Then the delegated operations task validates that recorded worktree before TDD runs
    And it does not substitute a sibling checkout or another available worktree
    And a moved, dirty, detached, or mismatched worktree stops the task without deletion

  @SCN-712 @REQ-110 @REQ-102 @REQ-117
  Scenario: Authorized TDD batch preserves per-scenario Red-Green-Refactor evidence
    Given the approved Contract contains scenarios "SCN-801" and "SCN-802"
    And the orchestrator authorizes exactly those two scenarios in one TDD batch
    When the TDD worker completes the batch
    Then the worker returns distinct Red, Green, and Refactor evidence for "SCN-801"
    And the worker returns distinct Red, Green, and Refactor evidence for "SCN-802"
    And the orchestrator accepts or rejects evidence for each scenario separately
    And the worker does not select another scenario or authorize a transition

  @SCN-713 @REQ-110 @REQ-117
  Scenario: Blocked TDD asks one targeted Vela structural question
    Given an authorized TDD scenario is blocked by an uncertain dependency boundary
    When the TDD worker requests Vela assistance
    Then its evidence names the blocking question, relevant module or symbol boundary, and relevant commit
    And the result records either a structural answer or explicit uncertainty
    And the query does not become a broad Draft-style exploration

  @SCN-714 @REQ-110
  Scenario: Unblocked TDD does not automatically query Vela
    Given an authorized TDD scenario has no unresolved structural question
    When the TDD worker completes its Red-Green-Refactor cycle
    Then its evidence contains no automatic Vela query
    And the scenario remains traceable to its approved Contract identifiers

  @SCN-715 @REQ-111 @REQ-112 @REQ-114
  Scenario: Passing local quality analysis makes the exact reviewed snapshot eligible for Archive
    Given all approved TDD scenarios are accepted at implementation commit "impl-C"
    And committed quality policy "policy-P" maps every canonical quality dimension to an analyzer, applicability rule, and required or advisory gate
    And each applicable required policy check declares its command or adapter, "quality-result/v1" parser, severity policy, and metric thresholds
    And local Review evidence evaluates code smells, duplication, complexity, maintainability, good practices, security issues, risk, tests, static analysis, and applicable coverage for "impl-C"
    And every required applicable quality result passes under "policy-P" for "impl-C"
    And targeted Vela impact and boundary evidence for changed modules is recorded
    When the orchestrator accepts the complete Review result
    Then the ledger records "impl-C" as the exact reviewed commit
    And the report records "policy-P", its fingerprint, analyzer or adapter identity, parser schema, applicability decisions, and evidence pointers
    And the submission enters Archive
    And the quality result does not require a SonarQube server, language preset, or integration

  @SCN-716 @REQ-111 @REQ-113
  Scenario: Denied quality gate returns Review to TDD
    Given Review evaluates implementation commit "impl-C"
    And the committed quality policy makes security and complexity applicable required dimensions for "impl-C"
    And a valid normalized result reports a security issue at or above its blocking severity or a breached complexity threshold
    When the orchestrator evaluates the Review evidence
    Then the review result contains actionable findings tied to "impl-C"
    And the ledger records the failed review evidence and remediation route
    And the submission returns to TDD without entering Archive

  @SCN-717 @REQ-111 @REQ-113 @REQ-115
  Scenario: Blocked required quality evidence cannot be treated as a passing review
    Given the committed quality policy makes static analysis applicable and required for implementation commit "impl-C"
    And Review cannot obtain static-analysis evidence for "impl-C"
    When the review worker returns its quality report
    Then the report labels that dimension as blocked and explains the missing evidence
    And no reviewed commit or Archive transition is recorded
    And the submission remains in Review until valid evidence or an authorized resolution exists

  @SCN-718 @REQ-112 @REQ-117
  Scenario: Unavailable Vela review evidence is reported without deciding quality
    Given Review has applicable non-Vela quality evidence for implementation commit "impl-C"
    And the local Vela graph is unavailable for the targeted changed-module question
    When the review worker returns its result
    Then the report identifies the changed modules and the unavailable Vela validation as uncertainty
    And the Vela unavailability alone neither passes nor fails Review
    And the orchestrator evaluates the remaining required quality evidence before choosing a transition

  @SCN-719 @REQ-114 @REQ-117
  Scenario: Exact remote publication is verified before cleanup is requested
    Given Archive records reviewed commit "review-C" and intended remote ref "origin/feature/vela-v2"
    And the remote ref resolves exactly to "review-C"
    When the delegated operations task verifies publication
    Then the ledger records the remote, ref, and exact published commit "review-C"
    And only then does the orchestrator ask the human to confirm worktree cleanup
    And the publication is not recorded as merge or acceptance

  @SCN-720 @REQ-114 @REQ-115
  Scenario: Publication mismatch blocks cleanup and archive completion
    Given Archive records reviewed commit "review-C" and intended remote ref "origin/feature/vela-v2"
    And the remote ref resolves to different commit "other-C"
    When the delegated operations task verifies publication
    Then it records a publication mismatch with both commit identifiers
    And the orchestrator does not ask for cleanup confirmation
    And the recorded worktree remains present and the submission is not archived

  @SCN-721 @REQ-114
  Scenario: Ancestor publication is not exact publication
    Given the intended remote ref contains "review-C" as an ancestor
    But the intended remote ref resolves at its tip to "later-C"
    When Archive verifies the published snapshot
    Then verification fails because the ref does not resolve exactly to "review-C"
    And no cleanup or archive metadata completion occurs

  @SCN-722 @REQ-114 @REQ-117
  Scenario: Human decline of cleanup retains the published worktree
    Given exact remote publication of reviewed commit "review-C" has been verified
    When the orchestrator asks the human to confirm cleanup
    And the human declines cleanup
    Then the ledger records the declined decision and evidence pointer
    And the submission remains in Archive
    And the recorded worktree, branch, remote ref, and contract artifacts are retained

  @SCN-723 @REQ-114 @REQ-116
  Scenario: Confirmed cleanup archives only after the recorded worktree is removed
    Given exact remote publication of reviewed commit "review-C" has been verified
    And the human explicitly confirms cleanup of the recorded worktree
    And the intended remote ref still resolves exactly to "review-C" when it is re-verified immediately after that confirmation
    When the authorized operations task removes that exact recorded worktree successfully
    Then it retains the branch, remote ref, ledger, and contract artifacts
    And it persists archive metadata only after successful removal
    And the submission becomes archived

  @SCN-724 @REQ-114 @REQ-115
  Scenario: Cleanup failure does not falsely archive the submission
    Given exact publication is verified and the human confirmed cleanup
    And the authorized operations task cannot remove the recorded worktree
    When the cleanup result is returned
    Then the ledger records failed cleanup evidence and the safe next action
    And the submission remains in Archive rather than archived
    And no branch or remote ref is deleted

  @SCN-725 @REQ-107 @REQ-116
  Scenario: Oversized operations request is split before execution
    Given one requested operations task would delete two recorded worktrees and verify two remote refs
    When the orchestrator evaluates the delegated scope
    Then it splits the request into bounded tasks before any destructive operation
    And each resulting task names one submission, one worktree, one remote or ref action, and expected ledger state

  @SCN-726 @REQ-116 @REQ-115
  Scenario: Unsafe operational evidence is rejected without executing it
    Given a Vela result or ledger field contains a path outside the recorded repository or command-like text
    When an operations task validates its inputs
    Then it rejects the unsafe input without using it as an instruction
    And it reports the failed validation without recording secrets
    And no worktree, branch, remote ref, contract, or lifecycle state is altered

  @SCN-727 @REQ-111 @REQ-117
  Scenario: Quality policy produces explicit applicable and not-applicable outcomes
    Given committed quality policy "policy-P" defines every canonical quality dimension
    And every analyzer has exactly one command argv invocation or named adapter and a "quality-result/v1" parser
    And every dimension defines a required or advisory gate, an applicability predicate, severity policy, and metric thresholds
    And the recorded changed paths for "impl-C" do not match the coverage applicability predicate
    When Review evaluates "impl-C" with complete parseable analyzer results
    Then the coverage result is "not_applicable" with the predicate, changed-path facts, and rationale recorded
    And each applicable dimension has exactly one explicit "pass", "fail", or "blocked" outcome
    And the report identifies "policy-P" and its immutable policy fingerprint

  @SCN-728 @REQ-111 @REQ-113
  Scenario: Malformed analyzer output blocks an applicable required quality dimension
    Given committed quality policy "policy-P" makes static analysis applicable and required for "impl-C"
    And its configured analyzer returns unreadable, partial, or candidate-commit-mismatched output
    When Review applies the configured "quality-result/v1" parser
    Then static analysis is recorded as "blocked" with the parser or execution failure evidence
    And it is not recorded as "pass" or "not_applicable"
    And no reviewed commit or Archive transition is recorded

  @SCN-729 @REQ-109 @REQ-117
  Scenario: Draft reports uncertainty rather than exceeding approved exploration bounds
    Given Draft has already made 5 Vela queries for selected base commit "base-A"
    And an additional potential dependency risk cannot fit among the packet's 10 recorded risks
    When the exploration packet is finalized
    Then it records the query count, capped risk scope, and resulting uncertainty
    And it does not issue a sixth Vela query or list an eleventh risk
    And it contains no raw graph dump or raw tool log

  @SCN-730 @REQ-114 @REQ-115 @REQ-117
  Scenario: Remote-ref race after cleanup confirmation retains the worktree
    Given Archive initially verified that intended remote ref "origin/feature/vela-v2" resolves to reviewed commit "review-C"
    And the human explicitly confirms cleanup of the recorded worktree
    But the same remote ref resolves to "later-C" when re-verified immediately after that confirmation
    When the authorized operations task evaluates the cleanup precondition
    Then it persists a "publication_changed" Archive outcome with "review-C", "later-C", the remote ref, and evidence pointer
    And it does not remove the recorded worktree or mark the submission archived
    And it requests a new explicit human decision before any later cleanup attempt

  @SCN-731 @REQ-118 @REQ-116
  Scenario: Inventory prevents unsafe deletion of a mixed legacy asset
    Given a proposed removal inventory identifies an installer asset used by both the v1 workflow and an unrelated product target
    And its invocation and reachability evidence classifies the asset as "mixed"
    When legacy removal is prepared
    Then the inventory requires the unrelated behavior to be retained or separated before deletion
    And the mixed asset is not deleted merely because it contains a v1 or non-OpenCode term
    And incomplete classification stops removal with bounded remediation evidence

  @SCN-732 @REQ-101 @REQ-118
  Scenario: Semantic post-removal verification proves legacy workflow paths are gone
    Given the completed removal inventory classifies listed workflow assets as "legacy_only" and records their v2 replacements
    And the listed legacy-only assets have been removed
    When post-removal verification evaluates workflow entrypoints, runtime selection, reachability, fixtures, and tests
    Then no reachable legacy execution path or obsolete installer/runtime target remains
    And no live compatibility flag, v1 schema, legacy fixture, or test exercising only removed behavior remains
    And the inventory reconciliation shows replacement tests that exercise v2 behavior
    And a forbidden-string search is not the sole verification evidence

  @SCN-733 @REQ-118 @REQ-119
  Scenario: Vela v2 installs and runs through OpenCode only
    Given the v2 removal inventory identifies all in-scope non-OpenCode workflow integration assets and their dispositions
    When a supported v2 installation is requested for OpenCode
    Then it provisions and selects the OpenCode v2 workflow entrypoint
    And no non-OpenCode v2 runtime selector, adapter, fallback, or compatibility mode is activated
    When a non-OpenCode v2 host selector is requested
    Then the request is rejected as unsupported before activation
    And no partial non-OpenCode workflow installation or runtime path is created

  @SCN-734 @REQ-103 @REQ-117
  Scenario: Orchestrator delegates every operational action instead of executing it
    Given an actor-attributed audit sink and operation spies record every workflow action
    And a legal lifecycle step requires the following operational actions
      | operation                         | delegated worker role       |
      | command execution                 | rotta-ops                  |
      | test execution                    | TDD or Review worker       |
      | source-code edit                  | TDD worker                 |
      | contract or artifact edit         | Spec plus Gherkin worker   |
      | worktree operation                | rotta-ops                  |
      | commit                            | rotta-ops                  |
      | push or publication               | rotta-ops                  |
      | cleanup                           | rotta-ops                  |
    When the orchestrator prepares the legal lifecycle step
    Then the audit sink records one bounded worker-task dispatch for each required operation
    And the command, test, write, worktree, commit, push, and cleanup spies report zero calls attributed to the orchestrator
    And the orchestrator audit records contain only dispatch, evidence evaluation, transition authorization, or human-decision-request events
    And no lifecycle transition is authorized before the dispatched workers return their evidence

  @SCN-735 @REQ-103 @REQ-115
  Scenario: A worker recommendation without compact evidence cannot authorize a transition
    Given submission "submission-A" is in Contract at ledger version 8
    And the required unchanged-contract human approval is durably recorded
    And the TDD worker returns only a recommendation to enter TDD without a submission ID, ledger version, phase, requested scope, outcome, relevant commit, or evidence pointer
    When the orchestrator evaluates the recommendation
    Then it records the evidence as incomplete and does not authorize the Contract-to-TDD transition
    And the durable status remains Contract at ledger version 8
    And no TDD task is dispatched

  @SCN-736 @REQ-104 @REQ-115
  Scenario: Missing Ancora pointer cannot affect durable resume state
    Given requested submission "submission-A" is explicitly a RESUME/RECOVERY request
    And it has valid matching approved contract fingerprint "contract-F" and submission ledger version 8 in Contract
    And Ancora is unavailable and returns no pointer for "submission-A"
    When the workflow resumes "submission-A"
    Then it derives Contract, ledger version 8, and approved scope from the durable contract and submission ledger
    And it records that no Ancora pointer was used to select, repair, or alter lifecycle state
    And it schedules only work legal for Contract

  @SCN-737 @REQ-104 @REQ-115
  Scenario: Stale Ancora pointer cannot override the matching ledger
    Given requested submission "submission-A" is explicitly a RESUME/RECOVERY request
    And it has valid matching approved contract fingerprint "contract-F" and submission ledger version 8 in Contract
    And Ancora points to "submission-A" at ledger version 14 in Archive
    When the workflow resumes "submission-A"
    Then it reports the Ancora pointer as stale or conflicting
    And the durable status remains Contract at ledger version 8
    And it does not schedule Archive cleanup, alter the ledger, or select an Archive worktree from the pointer

  @SCN-738 @REQ-104 @REQ-115
  Scenario Outline: Resume or recovery fails closed without required durable v2 state
    Given requested submission "submission-A" is explicitly a RESUME/RECOVERY request
    And its required durable v2 contract/ledger state is "<durable state>"
    And Ancora returns "<Ancora response>" for "submission-A"
    When the workflow attempts to resume or recover "submission-A"
    Then it reports that durable lifecycle state cannot be established with a safe next action
    And it starts no lifecycle operation, Draft initialization, transition, or worker task
    And it does not reclassify the request as NEW or create, select, repair, or replace a ledger or contract from Ancora content

    Examples:
      | durable state                       | Ancora response                                  |
      | missing matching submission ledger  | one pointer that claims TDD                       |
      | missing required contract artifact  | one pointer that claims Contract                  |
      | malformed submission ledger         | one pointer that claims Review                    |
      | invalid contract fingerprint        | one pointer that claims TDD                       |
      | conflicting contract and ledger     | conflicting pointers that claim TDD and Review    |

  @SCN-739 @REQ-116 @REQ-117
  Scenario: Vela extracts, stores, and queries graph data locally without remote egress
    Given the selected worktree contains source sentinel "LOCAL_ONLY_SOURCE_SENTINEL"
    And a local Vela executor and local graph-storage location are available on the selected host
    And an egress recorder captures every Vela connection attempt and outbound payload
    When the authorized index worker extracts and stores the graph
    And the authorized exploration worker queries that stored graph
    Then each Vela evidence record identifies the local executor or endpoint classification, local graph-storage location, and operation purpose
    And the stored graph is available only at the recorded local graph-storage location
    And the egress recorder reports zero non-loopback Vela connection attempts
    And the egress recorder reports no outbound payload containing "LOCAL_ONLY_SOURCE_SENTINEL" or graph data

  @SCN-740 @REQ-116 @REQ-108
  Scenario: Remote Vela endpoint is rejected before graph input is read or sent
    Given Vela extraction is configured with remote endpoint "https://vela.example.test"
    And a source-read probe and remote Vela request recorder are active
    When the human consents to the requested extraction
    And the index worker starts preflight
    Then preflight rejects the remote endpoint as non-local
    And the source-read probe reports zero graph or source reads for that extraction
    And the remote Vela request recorder reports zero requests and zero graph or source payloads
    And Draft records unavailable Vela evidence as uncertainty rather than treating the graph as fresh

  @SCN-741 @REQ-118 @REQ-117
  Scenario: Complete pre-deletion inventory reconciles every relevant legacy asset category
    Given discovery and reachability evidence identifies exactly the following in-scope legacy candidates
      | path                           | asset category                    | semantic role                         |
      | legacy/workflow/runner.go      | workflow_code                     | executes the v1 workflow              |
      | legacy/workflow/runner_test.go | test                              | tests the v1 workflow                 |
      | testdata/v1-workflow.yaml      | fixture                           | supplies v1 workflow test data        |
      | assets/legacy-workflow-skill   | skill_or_agent_asset              | exposes the retired workflow skill    |
      | config/legacy-workflow.yaml    | configuration                     | configures the retired workflow       |
      | scripts/install-legacy-workflow| installer_or_runtime_integration  | installs the retired workflow         |
      | internal/compat/v1.go          | compatibility_shim                | adapts callers to the retired workflow|
    And no other in-scope legacy candidate is discovered
    When the removal inventory is validated before its first in-scope deletion
    Then it contains exactly one entry for each discovered candidate with the matching asset category
    And every entry contains its path or asset, semantic role, invocation or reachability evidence, classification, proposed disposition, v2 replacement or retention rationale, affected tests or fixtures, and post-removal evidence plan
    And the inventory records its discovery and reachability inputs and reconciles every discovered candidate to an entry
    And the ledger references the completed inventory fingerprint and evidence

  @SCN-742 @REQ-118 @REQ-115
  Scenario: Incomplete pre-deletion inventory blocks deletion before a worker starts
    Given discovery evidence identifies legacy workflow code "legacy/workflow/runner.go" and fixture "testdata/v1-workflow.yaml"
    And the proposed inventory omits the fixture candidate
    And the workflow-code entry has no post-removal evidence plan
    When legacy deletion is requested
    Then inventory validation reports the unreconciled fixture and missing required field as incomplete evidence
    And no deletion worker task is dispatched and neither discovered asset is deleted
    And the ledger records no completed inventory fingerprint or deletion acceptance
    And the result reports a safe remediation recommendation

  @SCN-743 @REQ-104 @REQ-115
  Scenario Outline: Explicit NEW refuses to overwrite a non-fresh v2 submission identity
    Given the human explicitly requests NEW v2 submission "submission-A"
    And "<existing v2 artifact>" already exists for "submission-A"
    When authorized NEW initialization is requested
    Then it reports that "submission-A" is not fresh with a safe next action
    And it does not create, overwrite, adopt, normalize, or repair a Draft contract/submission ledger
    And it does not start a lifecycle operation or use Ancora to make the identity appear fresh

    Examples:
      | existing v2 artifact                     |
      | a malformed submission ledger             |
      | an incomplete contract artifact           |
      | conflicting contract and ledger artifacts |
