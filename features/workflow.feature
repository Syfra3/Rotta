Feature: OpenCode Rotta workflow
  Rotta provides one safe OpenCode workflow with bounded local evidence.

  @SCN-001 @REQ-001
  Scenario: Retired workflow surfaces are unavailable
    When a user invokes a retired workflow command or presents a retired workflow record
    Then Rotta rejects it without migration or compatibility behavior
    And active workflow surfaces use only the supported naming

  @SCN-002 @REQ-002
  Scenario: Semantic inventory protects mixed assets
    Given a candidate asset serves both retired workflow behavior and unrelated product behavior
    When retired workflow assets are removed
    Then a durable inventory records the candidate, its reachability evidence, and its disposition
    And Rotta retains the unrelated behavior until the retired portion is safely separated

  @SCN-003 @REQ-001 @REQ-002
  Scenario: Retired host integrations are removed without deleting unrelated behavior
    Given retired host assets, their tests, and CI jobs are identified as retired-only
    When removal is verified against the semantic inventory
    Then those retired-only assets are absent from active installation and CI paths
    And unrelated product behavior remains available

  @SCN-004 @REQ-003
  Scenario: Default invocation offers the normal terminal workflow
    When a user runs "rotta" with no arguments
    Then the top-level terminal choices are Install, Status, and Quit
    And choosing Quit exits without changing files

  @SCN-005 @REQ-003
  Scenario: Status reports without mutation
    Given no Rotta assets are installed
    When a user runs "rotta status"
    Then Rotta reports bounded local installation and workflow status
    And it does not change files

  @SCN-006 @REQ-004
  Scenario: Install manages only OpenCode user configuration
    Given the OpenCode user configuration directory is available
    When a user runs "rotta install"
    Then Rotta writes only its managed files under the OpenCode user configuration directory
    And it offers no non-OpenCode installation target

  @SCN-007 @REQ-004
  Scenario: Unmanaged configuration conflicts are preserved
    Given a required managed path contains an unmanaged file
    When a user runs "rotta install"
    Then installation stops with a bounded conflict report and safe next action
    And the unmanaged file is unchanged

  @SCN-008 @REQ-005
  Scenario: Only authorized orchestrator transitions persist
    Given a submission has a recorded status and revision
    When a worker submits a transition without matching orchestrator authorization
    Then Rotta rejects the transition
    And the durable status remains unchanged

  @SCN-009 @REQ-006
  Scenario: Invalid resume fails closed
    Given a resume record is missing, malformed, conflicting, or outside the selected repository
    When a user requests resume
    Then Rotta reports the failed precondition and a safe next action
    And it starts no worker and creates no replacement state

  @SCN-010 @REQ-007
  Scenario: Local graph indexing requires consent
    Given local graph evidence is absent or stale for the selected baseline
    When indexing is needed
    Then Rotta requests explicit human consent before indexing
    And a declined, unavailable, or failed index produces advisory uncertainty rather than graph findings

  @SCN-011 @REQ-007
  Scenario: Graph queries use bounded local evidence
    Given a local graph is available for the selected baseline
    When the workflow requests graph evidence
    Then the local executable returns bounded, redacted advisory evidence
    And graph evidence cannot advance workflow state or satisfy a quality gate

  @SCN-012 @REQ-008
  Scenario: Required Go quality evidence passes only after real adapters pass
    Given an applicable candidate commit is under review
    When quality evidence is collected
    Then test, vet, and coverage outcomes identify that candidate commit
    And Archive is eligible only when every applicable required outcome passes

  @SCN-013 @REQ-008
  Scenario: Failed or blocked Go evidence prevents archive
    Given an applicable required Go adapter fails or is blocked
    When Review evaluates its recorded outcome
    Then a failure returns the work to TDD
    And a blocked result remains in Review

  @SCN-014 @REQ-009
  Scenario: Archive cleanup requires two exact remote-ref observations
    Given publication initially resolves the recorded remote and fully qualified ref to the reviewed commit
    When cleanup consent is granted
    Then Rotta verifies the same remote and ref again before removing the recorded worktree
    And it removes no worktree if either observation differs from the reviewed commit

  @SCN-015 @REQ-009
  Scenario: Publication does not imply acceptance
    Given a recorded remote ref resolves exactly to the reviewed commit
    When publication completes
    Then Rotta records publication evidence
    And it does not merge, create a pull request, accept the change, or delete a ref

  @SCN-016 @REQ-010
  Scenario: Durable reports preserve contract and archive facts
    Given approved contract scenarios and lifecycle evidence exist
    When Rotta records task or archive results
    Then each result traces to the approved requirement and scenario IDs
    And it distinguishes baseline, contract, implementation, reviewed, and published commits
    And contract artifacts and durable archive metadata remain after cleanup
