package installer

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Syfra3/Rotta/internal/workflow"
)

func TestREQ093_RTKSetupRequiresSelectionAndDedicatedConfirmation(t *testing.T) {
	for _, options := range []Options{{}, {SetupRTK: true}} {
		fake := &fakeRTKInstaller{}
		result, err := setupRTK(options, fake)
		if err != nil || result.Status != RTKStatusSkipped || fake.installCalls != 0 {
			t.Fatalf("setupRTK(%#v) = %#v, %v; install calls = %d", options, result, err, fake.installCalls)
		}
	}
}

func TestREQ093_RTKSetupRecordsVerifiedSuccessAndFailureHostLocally(t *testing.T) {
	transaction := t.TempDir()
	fake := &fakeRTKInstaller{path: "/opt/rtk", version: "rtk 1.2.3", hash: "binary-hash"}
	result, err := setupRTK(Options{SetupRTK: true, ConfirmRTK: true}, fake)
	if err != nil || result.Status != RTKStatusSuccess || result.ExecutablePath != fake.path || result.Version != fake.version || fake.installCalls != 1 {
		t.Fatalf("success = %#v, %v; calls=%d", result, err, fake.installCalls)
	}
	if err := writeRTKTransactionEvidence(transaction, result); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(transaction, "rtk.json"))
	if err != nil {
		t.Fatal(err)
	}
	var recorded RTKInstallResult
	if err := json.Unmarshal(data, &recorded); err != nil {
		t.Fatal(err)
	}
	if recorded != result {
		t.Fatalf("recorded result = %#v, want %#v", recorded, result)
	}

	failure, err := setupRTK(Options{SetupRTK: true, ConfirmRTK: true}, &fakeRTKInstaller{installErr: errors.New("installer failed")})
	if err != nil || failure.Status != RTKStatusFailure || failure.FailureReason == "" {
		t.Fatalf("failure = %#v, %v", failure, err)
	}
	if err := writeRTKTransactionEvidence(transaction, failure); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(filepath.Join(transaction, "rtk.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &recorded); err != nil || recorded.Status != RTKStatusFailure {
		t.Fatalf("recorded failure = %#v, %v", recorded, err)
	}

	versionFailure, err := setupRTK(Options{SetupRTK: true, ConfirmRTK: true}, &fakeRTKInstaller{path: "/opt/rtk", versionErr: errors.New("version failed")})
	if err != nil || versionFailure.Status != RTKStatusFailure || versionFailure.FailureReason == "" {
		t.Fatalf("version failure = %#v, %v", versionFailure, err)
	}
}

func TestREQ093_RTKRuntimeStatePreservesSkipFailureAndSuccess(t *testing.T) {
	home, state := t.TempDir(), t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	for _, want := range []RTKInstallResult{
		{Status: RTKStatusSkipped, FailureReason: "not selected"},
		{Status: RTKStatusFailure, FailureReason: "version failed"},
		{Status: RTKStatusSuccess, ExecutablePath: "/opt/homebrew/Cellar/rtk-1/bin/rtk", Version: "rtk 1", ExecutableHash: "hash"},
	} {
		if err := writeRTKRuntimeState(home, want); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(runtimeRTKStatePath(home))
		if err != nil {
			t.Fatal(err)
		}
		var got RTKInstallResult
		if err := json.Unmarshal(data, &got); err != nil || got != want {
			t.Fatalf("runtime state = %#v, %v; want %#v", got, err, want)
		}
	}
}

func TestREQ093_RenderedRoleGuidanceNamesRecordedRTKRuntimeState(t *testing.T) {
	data, err := readRenderedAsset("core/rotta-core.md", Options{SetupRTK: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"$XDG_STATE_HOME/rotta/rtk.json", "rather than PATH or a re-resolved pathname", "exact bounded deterministic output"} {
		if !strings.Contains(string(data), want) {
			t.Fatalf("rendered role guidance missing %q", want)
		}
	}
}

type fakeRTKInstaller struct {
	installCalls           int
	path, version, hash    string
	installErr, versionErr error
}

func (f *fakeRTKInstaller) Install(Options) error { f.installCalls++; return f.installErr }
func (f *fakeRTKInstaller) Resolve() (workflow.RTKExecutable, error) {
	return &fakeInstallerExecutable{path: f.path, version: f.version, hash: f.hash, versionErr: f.versionErr}, nil
}

type fakeInstallerExecutable struct {
	path, version, hash string
	versionErr          error
}

func (f *fakeInstallerExecutable) Path() string                              { return f.path }
func (f *fakeInstallerExecutable) Version() (string, error)                  { return f.version, f.versionErr }
func (f *fakeInstallerExecutable) Fingerprint() (string, error)              { return f.hash, nil }
func (f *fakeInstallerExecutable) Run([]string, string, int) (string, error) { return "", nil }
func (f *fakeInstallerExecutable) Close() error                              { return nil }
