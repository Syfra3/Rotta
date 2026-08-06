//go:build linux

// Package rtkexec opens the only RTK-related executables Rotta may run.
// It intentionally has no caller-configurable roots: the accepted Homebrew
// layouts and binary locations are a small, exact allow-list.
package rtkexec

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/sys/unix"
)

var homebrewRoots = []string{
	"/opt/homebrew",
	"/usr/local",
	"/home/linuxbrew/.linuxbrew",
}

// runtimeGOOS is a test seam for the explicit descriptor-platform gate. It is
// never configured from the environment or a caller input.
var runtimeGOOS = runtime.GOOS

// trustedExecutionEnvironment is deliberately complete rather than additive:
// exec.Cmd otherwise inherits the caller environment. PATH is needed for a
// trusted script's ordinary system utilities, but is fixed to system roots;
// every shell hook, loader setting, language-tool setting, HOME, and caller
// configuration variable is absent.
func trustedExecutionEnvironment() []string {
	return []string{"PATH=/usr/bin:/bin"}
}

// Executable is a sealed, descriptor-backed snapshot. All inspection and
// execution methods use this one object; its original path is never reopened.
type Executable struct {
	path string
	file *os.File
}

func (executable *Executable) Path() string { return executable.path }
func (executable *Executable) Close() error { return executable.file.Close() }

func (executable *Executable) Version() (string, error) {
	output, err := executable.Run([]string{"--version"}, "", 512)
	if err != nil {
		return "", err
	}
	version := strings.TrimSpace(output)
	if version == "" {
		return "", errors.New("version command returned no version")
	}
	return version, nil
}

func (executable *Executable) Fingerprint() (string, error) {
	info, err := executable.file.Stat()
	if err != nil {
		return "", err
	}
	reader := io.NewSectionReader(executable.file, 0, info.Size())
	sum := sha256.New()
	if _, err := io.Copy(sum, reader); err != nil {
		return "", err
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

func (executable *Executable) Run(args []string, stdin string, maxOutput int) (string, error) {
	if runtimeGOOS != "linux" {
		return "", errors.New("descriptor-backed RTK execution requires Linux")
	}
	if maxOutput <= 0 {
		return "", errors.New("executable output limit must be positive")
	}
	// The sealed memfd is passed as fd 3. /proc/self/fd/3 means a rename,
	// symlink swap, or in-place rewrite of the source pathname cannot affect
	// this version/fingerprint/execution sequence.
	command := exec.Command("/proc/self/fd/3", args...)
	command.ExtraFiles = []*os.File{executable.file}
	// A sealed script still starts its shebang interpreter. Give that
	// interpreter a fixed utility lookup path and a configuration-free working
	// directory, rather than the caller's environment or current directory.
	command.Env = trustedExecutionEnvironment()
	command.Dir = "/"
	command.Stdin = strings.NewReader(stdin)
	output := &limitedOutput{limit: maxOutput}
	command.Stdout, command.Stderr = output, output
	if err := command.Run(); err != nil {
		return "", err
	}
	return output.String(), nil
}

// OpenBrew accepts exactly <trusted-root>/bin/brew, without following any
// symlink in the root-relative path.
func OpenBrew() (*Executable, error) {
	if runtimeGOOS != "linux" {
		return nil, errors.New("trusted Homebrew execution requires Linux descriptor guarantees")
	}
	return openBrewAtRoots(homebrewRoots)
}

func openBrewAtRoots(roots []string) (*Executable, error) {
	for _, root := range roots {
		executable, err := openUnderRoot(root, []string{"bin", "brew"})
		if err == nil {
			return executable, nil
		}
	}
	return nil, errors.New("no exact trusted Homebrew binary is available")
}

// OpenRTK accepts exactly <trusted-root>/Cellar/rtk/<version>/bin/rtk. The
// version is a single non-special path component; no prefix matching or
// symlink canonicalisation is used as a trust decision.
func OpenRTK(recordedPath string) (*Executable, error) {
	if runtimeGOOS != "linux" {
		return nil, errors.New("trusted RTK execution requires Linux descriptor guarantees")
	}
	return openRTKAtRoots(recordedPath, homebrewRoots)
}

// OpenRTKVersion opens the exact direct Cellar location for one version
// reported by the already-trusted brew descriptor. It deliberately does not
// follow Homebrew's mutable opt/ symlink.
func OpenRTKVersion(version string) (*Executable, error) {
	if runtimeGOOS != "linux" {
		return nil, errors.New("trusted RTK execution requires Linux descriptor guarantees")
	}
	if !safeComponent(version) {
		return nil, errors.New("RTK version is not a safe Cellar component")
	}
	for _, root := range homebrewRoots {
		executable, err := openUnderRoot(root, []string{"Cellar", "rtk", version, "bin", "rtk"})
		if err == nil {
			return executable, nil
		}
	}
	return nil, errors.New("no exact trusted Homebrew RTK Cellar executable is available")
}

func openRTKAtRoots(recordedPath string, roots []string) (*Executable, error) {
	for _, root := range roots {
		components, ok := exactRTKComponents(root, recordedPath)
		if !ok {
			continue
		}
		return openUnderRoot(root, components)
	}
	return nil, errors.New("recorded RTK executable is outside the exact trusted Homebrew policy")
}

func exactRTKComponents(root, path string) ([]string, bool) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, false
	}
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || relative == ".." {
		return nil, false
	}
	components := strings.Split(filepath.ToSlash(relative), "/")
	if len(components) != 5 || components[0] != "Cellar" || components[1] != "rtk" || components[3] != "bin" || components[4] != "rtk" || !safeComponent(components[2]) {
		return nil, false
	}
	return components, true
}

func safeComponent(value string) bool {
	return value != "" && value != "." && value != ".." && !strings.ContainsAny(value, "/\\\x00")
}

func openUnderRoot(root string, components []string) (*Executable, error) {
	if runtimeGOOS != "linux" {
		return nil, errors.New("trusted executable opening requires Linux descriptor guarantees")
	}
	if len(components) == 0 {
		return nil, errors.New("trusted executable path is empty")
	}
	rootFD, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("open trusted Homebrew root: %w", err)
	}
	defer unix.Close(rootFD)
	currentFD := rootFD
	for index, component := range components {
		if !safeComponent(component) {
			return nil, errors.New("trusted executable path has an unsafe component")
		}
		flags := unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_CLOEXEC
		if index < len(components)-1 {
			flags |= unix.O_DIRECTORY
		}
		nextFD, err := unix.Openat(currentFD, component, flags, 0)
		if currentFD != rootFD {
			_ = unix.Close(currentFD)
		}
		if err != nil {
			return nil, fmt.Errorf("open trusted executable component %q: %w", component, err)
		}
		currentFD = nextFD
	}
	file := os.NewFile(uintptr(currentFD), "trusted-homebrew-executable")
	if file == nil {
		_ = unix.Close(currentFD)
		return nil, errors.New("wrap trusted executable descriptor")
	}
	return sealSnapshot(root, components, file)
}

func sealSnapshot(root string, components []string, source *os.File) (_ *Executable, err error) {
	defer func() {
		if err != nil {
			_ = source.Close()
		}
	}()
	before, err := source.Stat()
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || before.Mode().Perm()&0o111 == 0 {
		return nil, errors.New("trusted executable is not a regular executable")
	}
	sealedFD, err := unix.MemfdCreate("rotta-trusted-executable", unix.MFD_CLOEXEC|unix.MFD_ALLOW_SEALING)
	if err != nil {
		return nil, fmt.Errorf("create sealed executable snapshot: %w", err)
	}
	snapshot := os.NewFile(uintptr(sealedFD), "sealed-trusted-executable")
	if snapshot == nil {
		_ = unix.Close(sealedFD)
		return nil, errors.New("wrap sealed executable snapshot")
	}
	defer func() {
		if err != nil {
			_ = snapshot.Close()
		}
	}()
	if _, err = io.Copy(snapshot, source); err != nil {
		return nil, fmt.Errorf("snapshot trusted executable: %w", err)
	}
	after, err := source.Stat()
	if err != nil {
		return nil, err
	}
	if !sameFileState(before, after) {
		return nil, errors.New("trusted executable changed while being snapshotted")
	}
	if err = snapshot.Chmod(before.Mode().Perm()); err != nil {
		return nil, err
	}
	if _, err = unix.FcntlInt(snapshot.Fd(), unix.F_ADD_SEALS, unix.F_SEAL_WRITE|unix.F_SEAL_GROW|unix.F_SEAL_SHRINK|unix.F_SEAL_SEAL); err != nil {
		return nil, fmt.Errorf("seal trusted executable snapshot: %w", err)
	}
	_ = source.Close()
	return &Executable{path: filepath.Join(append([]string{root}, components...)...), file: snapshot}, nil
}

func sameFileState(before, after os.FileInfo) bool {
	// os.SameFile covers device/inode identity. Size and ModTime catch normal
	// in-place rewrites while copying; the sealed snapshot then closes the last
	// unavoidable mutation window before inspection/execution.
	return os.SameFile(before, after) && before.Size() == after.Size() && before.ModTime().Equal(after.ModTime())
}

type limitedOutput struct {
	limit int
	data  strings.Builder
}

func (output *limitedOutput) Write(value []byte) (int, error) {
	remaining := output.limit - output.data.Len()
	if remaining <= 0 {
		return 0, errors.New("trusted executable output exceeds limit")
	}
	if len(value) > remaining {
		output.data.Write(value[:remaining])
		return remaining, errors.New("trusted executable output exceeds limit")
	}
	return output.data.Write(value)
}

func (output *limitedOutput) String() string { return output.data.String() }
