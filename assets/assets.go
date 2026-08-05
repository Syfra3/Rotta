// Package assets exposes the embedded Rotta files.
package assets

import "embed"

//go:embed config/quality-gates.yaml agents/rotta-orchestrator.md agents/rotta-spec.md agents/rotta-impl.md agents/rotta-review.md
var FS embed.FS
