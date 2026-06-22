// Package assets exposes the embedded Uncle Bob workflow files.
package assets

import "embed"

//go:embed skills/spec-mode/SKILL.md skills/implementation-mode/SKILL.md skills/review-mode/SKILL.md config/state-machine.yaml config/quality-gates.yaml agents/bob-orchestrator.md agents/bob-spec.md agents/bob-impl.md agents/bob-review.md
var FS embed.FS
