Feature: Workflow artifact lifecycle
  Clean-workflow contributors need approved workflow contracts to remain reviewable, traceable, and safe to consume during TDD and QA without turning Ancora or local generated output into the source of truth.

  Background:
    Given the repository contains workflow artifact directories named "specs" and "features"
    And implementation agents require a clean working tree and an approval marker before implementation begins

  @REQ-011 @REQ-012 @SCN-012
  Scenario: Active hard spec and feature files are tracked as the contract source of truth
    Given a workflow change has an approved hard spec and an approved Gherkin feature file
    When the workflow prepares the change for implementation
    Then the hard spec remains as a tracked file under "specs"
    And the Gherkin feature remains as a tracked file under "features"
    And the workflow treats those repository files as the authoritative contract content
    And the workflow does not require full contract text from Ancora to recover the approved behavior

  @REQ-011 @REQ-020 @SCN-013
  Scenario: Namespaced workflow-policy artifacts do not overwrite an existing active contract
    Given "specs/hard_spec.md" describes an existing active contract
    And "features/installer_recovery.feature" describes an existing active contract
    When a new workflow artifact lifecycle contract is generated
    Then the new hard spec is written to "specs/workflow_artifact_lifecycle.md"
    And the new feature is written to "features/workflow_artifact_lifecycle.feature"
    And the existing active hard spec and installer recovery feature are left unchanged

  @REQ-012 @REQ-016 @SCN-014
  Scenario: Implemented feature files remain active regression contracts
    Given an approved feature file has been implemented and verified
    When the change process is completed
    Then the feature file remains active under "features" while its behavior must keep working
    And archive cleanup does not move the feature file only because implementation is complete
    And future regression work can still discover the scenario from the active feature file

  @REQ-013 @REQ-019 @SCN-015
  Scenario: Tests reference stable scenario IDs from feature files
    Given an approved feature scenario is tagged with "@SCN-012" and a requirement tag
    When a test is written to verify that scenario
    Then the test references "SCN-012" in a traceable test name, metadata field, subtest name, or equivalent location
    And the trace includes the feature identity when another feature file can contain the same local scenario ID
    And later scenario reordering does not change the approved scenario ID

  @REQ-014 @SCN-016
  Scenario: Ancora records pointer-only workflow state
    Given a hard spec exists at "specs/workflow_artifact_lifecycle.md"
    And a feature exists at "features/workflow_artifact_lifecycle.feature"
    When the workflow saves state to Ancora
    Then the Ancora observation records the artifact paths, phase, approval status, risk level, requirement IDs, and scenario IDs
    And the observation may record hashes or checksums for drift detection
    And the observation does not become the only source of the full hard spec or Gherkin text

  @REQ-014 @SCN-017
  Scenario: Repository content wins when an Ancora pointer is stale
    Given Ancora points to an active workflow artifact path
    And the repository file at that path has been renamed or changed through review
    When the workflow resumes from Ancora state
    Then it verifies the pointer against repository files
    And it repairs or reports the stale pointer
    And it does not replace reviewed repository content with older full text from Ancora

  @REQ-015 @SCN-018
  Scenario: Pending generated contracts do not pass the implementation gate
    Given a new hard spec and feature contract were generated for human review
    And no human approval has been recorded for that new contract
    When implementation is requested for the new contract
    Then the workflow reports that human approval is still required
    And it does not create or rely on "specs/.approved" for the pending contract
    And implementation does not begin for the pending scenarios

  @REQ-015 @REQ-020 @SCN-019
  Scenario: Untracked active contracts are tracked instead of deleted to clean the tree
    Given generated spec and feature files describe an approved active behavior contract
    And those files are untracked in the working tree
    When an implementation agent requires a clean tree
    Then the workflow identifies the files as active contract artifacts
    And the workflow requires tracking, committing, or otherwise explicitly approving those contract files
    And the workflow does not delete the approved contract as the normal cleanup path

  @REQ-016 @SCN-020
  Scenario: Retired or superseded process artifacts can be archived without hiding active contracts
    Given a completed workflow produced process artifacts that are no longer active behavior contracts
    And active feature files still describe behavior that must keep working
    When the workflow archives completed or retired artifacts
    Then only retired, superseded, or process-only artifacts move to archive
    And active feature files remain discoverable as regression contracts
    And the archive record states why each moved artifact is no longer active

  @REQ-017 @SCN-021
  Scenario: Local graph and cache artifacts are excluded unless intentionally promoted
    Given local generated graph or cache files exist under ".vela" or a similar cache directory
    When the workflow prepares artifact changes for review
    Then those local generated files are ignored or removed from the review set
    And they are not committed as spec or feature artifacts by default
    And any intentionally tracked generated artifact has an explicit project-artifact decision

  @REQ-018 @SCN-022
  Scenario: Backup outputs and sensitive config captures are rejected as workflow artifacts
    Given a backup output, restore snapshot, user config capture, token-bearing file, or private machine-state file exists in the repository checkout
    When the workflow classifies artifacts for tracking or archive
    Then it rejects that file as a workflow contract artifact
    And it requires the content to be deleted, ignored, or replaced with a sanitized authored example
    And no active spec or feature embeds the sensitive captured values

  @REQ-019 @SCN-023
  Scenario: QA planning enumerates approved scenarios from repository feature files
    Given one or more approved feature files are active under "features"
    When QA planning or strict TDD planning asks for the behavior backlog
    Then the workflow enumerates active scenarios from the repository feature files
    And each planned test or QA item can reference the feature file and scenario ID
    And pending unapproved scenarios are not treated as implementation-ready

  @REQ-020 @SCN-024
  Scenario: Workflow cleanup explains artifact lifecycle actions explicitly
    Given the working tree contains active contracts, pending contracts, archive candidates, local caches, and sensitive outputs
    When the workflow prepares cleanup guidance
    Then it labels each artifact as track, keep pending, archive, ignore, or delete
    And active behavior contracts are never labeled for deletion solely to satisfy a clean-tree requirement
    And pending contracts remain pending until a human approves them
