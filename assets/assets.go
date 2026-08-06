// Package assets exposes the embedded Rotta files.
package assets

import "embed"

//go:embed core/rotta-core.md agents/rotta-orchestrator.md agents/rotta-explore.md agents/rotta-impl.md agents/rotta-review.md agents/rotta-ops.md
var FS embed.FS
