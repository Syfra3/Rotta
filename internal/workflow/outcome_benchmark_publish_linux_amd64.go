//go:build linux && amd64

package workflow

import (
	"syscall"
	"unsafe"
)

const (
	renameat2SyscallNumber = 316 // Linux amd64 __NR_renameat2.
	renameNoReplace        = 1
)

// publishStageNoReplace is a single-kernel-operation no-clobber publication.
// RENAME_NOREPLACE returns EEXIST without following or replacing a final name.
func publishStageNoReplace(stage, final string) error {
	oldPath, err := syscall.BytePtrFromString(stage)
	if err != nil {
		return err
	}
	newPath, err := syscall.BytePtrFromString(final)
	if err != nil {
		return err
	}
	_, _, errno := syscall.Syscall6(renameat2SyscallNumber,
		^uintptr(99), uintptr(unsafe.Pointer(oldPath)),
		^uintptr(99), uintptr(unsafe.Pointer(newPath)),
		renameNoReplace, 0)
	if errno == 0 {
		return nil
	}
	if errno == syscall.ENOSYS || errno == syscall.EINVAL {
		return ErrBenchmarkNoReplaceUnsupported
	}
	return errno
}
