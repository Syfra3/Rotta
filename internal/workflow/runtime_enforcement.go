package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

const (
	runtimeEnforcementLedgerFormat  = "rotta.runtime-enforcement"
	runtimeEnforcementLedgerVersion = 1
)

// RuntimeEnforcementLedger is separate from current-submission documents.
// Its immutable binding makes a recovered ledger unusable by another
// worktree, baseline, contract, or policy.
type RuntimeEnforcementLedger struct {
	Format              string                        `json:"format"`
	Version             int                           `json:"version"`
	Feature             string                        `json:"feature"`
	Worktree            string                        `json:"worktree"`
	Baseline            string                        `json:"baseline"`
	ContractFingerprint string                        `json:"contract_fingerprint"`
	PolicyFingerprint   string                        `json:"policy_fingerprint"`
	Session             string                        `json:"session"`
	Route               string                        `json:"route"`
	Budget              int                           `json:"budget"`
	Charged             int                           `json:"charged"`
	Reservations        map[string]RuntimeReservation `json:"reservations"`
	HandedOff           bool                          `json:"handed_off"`
	Terminal            bool                          `json:"terminal"`
	Remediated          bool                          `json:"remediated"`
}

type RuntimeReservation struct {
	CallID string `json:"call_id"`
	State  string `json:"state"` // reserved, committed, cancelled
}

type RuntimeBinding struct {
	Feature             string
	Worktree            string
	Baseline            string
	ContractFingerprint string
	PolicyFingerprint   string
	Session             string
	Route               string
}

type RuntimeEnforcementStore struct{ Root string }

func NewRuntimeEnforcementStore(repoRoot string) RuntimeEnforcementStore {
	return RuntimeEnforcementStore{Root: repoRoot}
}
func (s RuntimeEnforcementStore) Path() string {
	return filepath.Join(s.Root, ".rotta", "current", "runtime-enforcement.json")
}

func canonicalRuntimeBinding(b RuntimeBinding) (RuntimeBinding, error) {
	if b.Feature == "" || b.Worktree == "" || b.Baseline == "" || b.ContractFingerprint == "" || b.PolicyFingerprint == "" || b.Session == "" || b.Route == "" {
		return RuntimeBinding{}, errors.New("runtime enforcement binding requires feature, worktree, baseline, contract fingerprint, policy fingerprint, session, and route")
	}
	path, err := filepath.Abs(b.Worktree)
	if err != nil {
		return RuntimeBinding{}, fmt.Errorf("canonical runtime enforcement worktree: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return RuntimeBinding{}, fmt.Errorf("canonical runtime enforcement worktree: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return RuntimeBinding{}, fmt.Errorf("canonical runtime enforcement worktree: %w", err)
	}
	if !info.IsDir() {
		return RuntimeBinding{}, errors.New("canonical runtime enforcement worktree must be a directory")
	}
	b.Worktree = filepath.Clean(resolved)
	return b, nil
}

// Refresh establishes an immutable identity once. Replays are safe.
func (s RuntimeEnforcementStore) Refresh(binding RuntimeBinding, budget int) (RuntimeEnforcementLedger, error) {
	binding, err := canonicalRuntimeBinding(binding)
	if err != nil || budget < 0 {
		if err != nil {
			return RuntimeEnforcementLedger{}, err
		}
		return RuntimeEnforcementLedger{}, errors.New("runtime enforcement refresh requires non-negative budget")
	}
	return s.change(func(l *RuntimeEnforcementLedger, exists bool) error {
		if !exists {
			*l = RuntimeEnforcementLedger{Format: runtimeEnforcementLedgerFormat, Version: runtimeEnforcementLedgerVersion, Feature: binding.Feature, Worktree: binding.Worktree, Baseline: binding.Baseline, ContractFingerprint: binding.ContractFingerprint, PolicyFingerprint: binding.PolicyFingerprint, Session: binding.Session, Route: binding.Route, Budget: budget, Reservations: map[string]RuntimeReservation{}}
			return nil
		}
		if !l.sameBinding(binding) || l.Budget != budget {
			return errors.New("runtime enforcement stale or cross-workflow binding")
		}
		return nil
	})
}

func (s RuntimeEnforcementStore) Handoff(from, to RuntimeBinding) (RuntimeEnforcementLedger, error) {
	from, err := canonicalRuntimeBinding(from)
	if err != nil {
		return RuntimeEnforcementLedger{}, err
	}
	to, err = canonicalRuntimeBinding(to)
	if err != nil {
		return RuntimeEnforcementLedger{}, err
	}
	return s.change(func(l *RuntimeEnforcementLedger, exists bool) error {
		if !exists || l.Terminal || !l.sameImmutableBinding(to) {
			return errors.New("runtime enforcement stale or cross-workflow handoff")
		}
		if l.Session == to.Session && l.Route == to.Route {
			return nil
		}
		if !l.sameBinding(from) {
			return errors.New("runtime enforcement stale or cross-workflow handoff")
		}
		l.Session, l.Route, l.HandedOff = to.Session, to.Route, true
		return nil
	})
}

func (s RuntimeEnforcementStore) Reserve(binding RuntimeBinding, callID string) (RuntimeEnforcementLedger, error) {
	return s.changeBinding(binding, func(l *RuntimeEnforcementLedger) error {
		if callID == "" {
			return errors.New("runtime enforcement call ID is required")
		}
		if l.Terminal {
			return errors.New("runtime enforcement terminal ledger denies dispatch")
		}
		if r, ok := l.Reservations[callID]; ok {
			if r.State == "cancelled" {
				return errors.New("runtime enforcement duplicate invalid call")
			}
			return nil
		}
		if l.Charged >= l.Budget {
			return errors.New("runtime enforcement budget exhausted before dispatch")
		}
		l.Reservations[callID] = RuntimeReservation{CallID: callID, State: "reserved"}
		l.Charged++
		return nil
	})
}
func (s RuntimeEnforcementStore) Commit(binding RuntimeBinding, callID string) (RuntimeEnforcementLedger, error) {
	return s.terminal(binding, callID, "committed")
}
func (s RuntimeEnforcementStore) Cancel(binding RuntimeBinding, callID string) (RuntimeEnforcementLedger, error) {
	return s.terminal(binding, callID, "cancelled")
}
func (s RuntimeEnforcementStore) terminal(binding RuntimeBinding, callID, state string) (RuntimeEnforcementLedger, error) {
	return s.changeBinding(binding, func(l *RuntimeEnforcementLedger) error {
		r, ok := l.Reservations[callID]
		if !ok {
			return errors.New("runtime enforcement call was not reserved")
		}
		if r.State == state {
			return nil
		}
		if r.State != "reserved" {
			return errors.New("runtime enforcement duplicate invalid call")
		}
		r.State = state
		l.Reservations[callID] = r
		if state == "cancelled" {
			l.Charged--
		} else {
			l.Terminal = true
		}
		return nil
	})
}
func (s RuntimeEnforcementStore) Remediation(binding RuntimeBinding) (RuntimeEnforcementLedger, error) {
	return s.changeBinding(binding, func(l *RuntimeEnforcementLedger) error {
		if l.HandedOff || l.Terminal {
			return errors.New("runtime enforcement remediation is only legal before handoff and terminal state")
		}
		if l.Remediated {
			return errors.New("runtime enforcement second remediation denied")
		}
		l.Remediated = true
		return nil
	})
}

func (l RuntimeEnforcementLedger) sameImmutableBinding(b RuntimeBinding) bool {
	return l.Feature == b.Feature && l.Worktree == b.Worktree && l.Baseline == b.Baseline && l.ContractFingerprint == b.ContractFingerprint && l.PolicyFingerprint == b.PolicyFingerprint
}
func (l RuntimeEnforcementLedger) sameBinding(b RuntimeBinding) bool {
	return l.sameImmutableBinding(b) && l.Session == b.Session && l.Route == b.Route
}
func (s RuntimeEnforcementStore) changeBinding(binding RuntimeBinding, apply func(*RuntimeEnforcementLedger) error) (RuntimeEnforcementLedger, error) {
	binding, err := canonicalRuntimeBinding(binding)
	if err != nil {
		return RuntimeEnforcementLedger{}, err
	}
	return s.change(func(l *RuntimeEnforcementLedger, exists bool) error {
		if !exists || !l.sameBinding(binding) {
			return errors.New("runtime enforcement stale or illegal route binding")
		}
		return apply(l)
	})
}

func (l RuntimeEnforcementLedger) validate() error {
	if l.Format != runtimeEnforcementLedgerFormat || l.Version != runtimeEnforcementLedgerVersion {
		return errors.New("runtime enforcement ledger has unsupported format or version")
	}
	b, err := canonicalRuntimeBinding(RuntimeBinding{Feature: l.Feature, Worktree: l.Worktree, Baseline: l.Baseline, ContractFingerprint: l.ContractFingerprint, PolicyFingerprint: l.PolicyFingerprint, Session: l.Session, Route: l.Route})
	if err != nil || b.Worktree != l.Worktree {
		return errors.New("runtime enforcement ledger has invalid binding")
	}
	if l.Budget < 0 || l.Charged < 0 || l.Charged > l.Budget || l.Reservations == nil {
		return errors.New("runtime enforcement ledger has invalid budget state")
	}
	charged := 0
	committed := 0
	reserved := 0
	for id, r := range l.Reservations {
		if id == "" || r.CallID != id || (r.State != "reserved" && r.State != "committed" && r.State != "cancelled") {
			return errors.New("runtime enforcement ledger has invalid reservation")
		}
		if r.State != "cancelled" {
			charged++
		}
		if r.State == "committed" {
			committed++
		}
		if r.State == "reserved" {
			reserved++
		}
	}
	if charged != l.Charged || (l.Terminal && (committed != 1 || reserved != 0)) || (!l.Terminal && committed != 0) {
		return errors.New("runtime enforcement ledger has inconsistent terminal or charge state")
	}
	return nil
}

func (s RuntimeEnforcementStore) change(apply func(*RuntimeEnforcementLedger, bool) error) (RuntimeEnforcementLedger, error) {
	if err := os.MkdirAll(filepath.Dir(s.Path()), 0o700); err != nil {
		return RuntimeEnforcementLedger{}, fmt.Errorf("create runtime enforcement directory: %w", err)
	}
	lock, err := os.OpenFile(s.Path()+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return RuntimeEnforcementLedger{}, fmt.Errorf("open runtime enforcement lock: %w", err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return RuntimeEnforcementLedger{}, fmt.Errorf("lock runtime enforcement ledger: %w", err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	l := RuntimeEnforcementLedger{}
	data, err := os.ReadFile(s.Path())
	exists := err == nil
	if err != nil && !os.IsNotExist(err) {
		return l, fmt.Errorf("read runtime enforcement ledger: %w", err)
	}
	if exists {
		if json.Unmarshal(data, &l) != nil {
			return l, errors.New("runtime enforcement ledger is corrupt")
		}
		if err := l.validate(); err != nil {
			return l, fmt.Errorf("malformed runtime enforcement ledger: %w", err)
		}
	}
	if err := apply(&l, exists); err != nil {
		return l, err
	}
	data, err = json.MarshalIndent(l, "", "  ")
	if err != nil {
		return l, fmt.Errorf("encode runtime enforcement ledger: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.Path()), ".runtime-enforcement-*")
	if err != nil {
		return l, fmt.Errorf("create runtime enforcement ledger temp file: %w", err)
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return l, fmt.Errorf("write runtime enforcement ledger: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return l, fmt.Errorf("set runtime enforcement ledger permissions: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return l, fmt.Errorf("sync runtime enforcement ledger: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return l, errors.New("atomically write runtime enforcement ledger")
	}
	if err := os.Rename(name, s.Path()); err != nil {
		return l, fmt.Errorf("replace runtime enforcement ledger: %w", err)
	}
	dir, err := os.Open(filepath.Dir(s.Path()))
	if err != nil {
		return l, fmt.Errorf("open runtime enforcement ledger directory: %w", err)
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil {
		return l, fmt.Errorf("sync runtime enforcement ledger directory: %w", err)
	}
	return l, nil
}
