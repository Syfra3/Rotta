//go:build !linux || !amd64

package workflow

// publishStageNoReplace deliberately has no weaker rename fallback. Platforms
// without the Linux amd64 renameat2 RENAME_NOREPLACE guarantee must fail closed.
func publishStageNoReplace(stage, final string) error {
	return ErrBenchmarkNoReplaceUnsupported
}
