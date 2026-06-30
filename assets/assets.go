// Package assets exposes the embedded Rotta files.
package assets

import "embed"

//go:embed skills/spec-mode/SKILL.md skills/implementation-mode/SKILL.md skills/review-mode/SKILL.md config/state-machine.yaml config/quality-gates.yaml agents/rotta-orchestrator.md agents/rotta-spec.md agents/rotta-impl.md agents/rotta-review.md
var FS embed.FS
