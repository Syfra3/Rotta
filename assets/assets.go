// Package assets exposes the embedded Clean Workflow files.
package assets

import "embed"

//go:embed skills/spec-mode/SKILL.md skills/implementation-mode/SKILL.md skills/review-mode/SKILL.md config/state-machine.yaml config/quality-gates.yaml agents/clean-orchestrator.md agents/clean-spec.md agents/clean-impl.md agents/clean-review.md
var FS embed.FS
