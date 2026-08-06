//go:build !linux

package rtkexec

import "errors"

// Executable is unavailable outside Linux because Rotta has no equivalent
// descriptor-sealing primitive there.
type Executable struct{}

func (*Executable) Path() string { return "" }
func (*Executable) Close() error { return nil }
func (*Executable) Version() (string, error) {
	return "", errors.New("trusted executable execution requires Linux")
}
func (*Executable) Fingerprint() (string, error) {
	return "", errors.New("trusted executable execution requires Linux")
}
func (*Executable) Run([]string, string, int) (string, error) {
	return "", errors.New("trusted executable execution requires Linux")
}

func OpenBrew() (*Executable, error) {
	return nil, errors.New("trusted Homebrew execution requires Linux descriptor guarantees")
}
func OpenRTK(string) (*Executable, error) {
	return nil, errors.New("trusted RTK execution requires Linux descriptor guarantees")
}
func OpenRTKVersion(string) (*Executable, error) {
	return nil, errors.New("trusted RTK execution requires Linux descriptor guarantees")
}
