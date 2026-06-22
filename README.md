# bob-workflow

`bob-workflow` is a contract-driven development orchestrator for Uncle Bob style execution:
hard spec → Gherkin → TDD → review.

## What this repository includes

- `bin/uncle-bob` — CLI entrypoint.
- `cmd/` and `internal/` — command and application internals.
- `assets/agents/` — agent contracts that define how the workflow is delegated.

## Compatible coding agents

This project works with the following agents:

- `bob-orchestrator` (`assets/agents/bob-orchestrator.md`)
- `bob-spec` (`assets/agents/bob-spec.md`)
- `bob-impl` (`assets/agents/bob-impl.md`)
- `bob-review` (`assets/agents/bob-review.md`)

## Quick start

1. Initialize Go module dependencies and build as needed with standard Go tooling.
2. Run the CLI entrypoint once the binary is built.
