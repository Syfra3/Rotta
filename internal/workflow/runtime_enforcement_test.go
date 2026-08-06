package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runtimeBinding(t *testing.T, feature, session, route string) RuntimeBinding {
	t.Helper()
	return RuntimeBinding{Feature: feature, Worktree: t.TempDir(), Baseline: "9a6ff507a243b1cad8bcfd54078e089a6da232d1", ContractFingerprint: "contract-sha256", PolicyFingerprint: "policy-sha256", Session: session, Route: route}
}

func TestRuntimeEnforcementLedgerDurableIdempotentTransitions(t *testing.T) {
	s := NewRuntimeEnforcementStore(t.TempDir())
	b := runtimeBinding(t, "runtime-enforcement", "s1", "delegate")
	if l, err := s.Refresh(b, 1); err != nil {
		t.Fatal(err)
	} else if l.Format != runtimeEnforcementLedgerFormat || l.Version != runtimeEnforcementLedgerVersion || !filepath.IsAbs(l.Worktree) || l.Baseline != b.Baseline || l.ContractFingerprint != b.ContractFingerprint || l.PolicyFingerprint != b.PolicyFingerprint {
		t.Fatalf("durable binding = %#v", l)
	}
	if l, err := s.Reserve(b, "c1"); err != nil || l.Charged != 1 {
		t.Fatalf("reserve = %#v, %v", l, err)
	}
	if l, err := s.Reserve(b, "c1"); err != nil || l.Charged != 1 {
		t.Fatalf("replayed reserve = %#v, %v", l, err)
	}
	if _, err := s.Reserve(b, "c2"); err == nil {
		t.Fatal("second call must be denied before dispatch")
	}
	if _, err := s.Commit(b, "c1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Commit(b, "c1"); err != nil {
		t.Fatal("commit replay must be safe")
	}
	before, err := os.ReadFile(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reserve(b, "c1"); err == nil {
		t.Fatal("terminal ledger must deny reserve replay before idempotency")
	}
	after, err := os.ReadFile(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("terminal reserve replay mutated ledger")
	}
	if _, err := s.Cancel(b, "c1"); err == nil {
		t.Fatal("commit then cancel must be denied")
	}
}

func TestRuntimeEnforcementLedgerRejectsCrossWorkflowAndStaleBindings(t *testing.T) {
	s := NewRuntimeEnforcementStore(t.TempDir())
	b := runtimeBinding(t, "feature-a", "s1", "delegate")
	if _, err := s.Refresh(b, 2); err != nil {
		t.Fatal(err)
	}
	cross := b
	cross.Feature = "feature-b"
	if _, err := s.Reserve(cross, "c1"); err == nil {
		t.Fatal("cross workflow must be denied")
	}
	stale := b
	stale.ContractFingerprint = "other-contract"
	if _, err := s.Reserve(stale, "c1"); err == nil {
		t.Fatal("stale contract must be denied")
	}
	stale = b
	stale.Baseline = "other-baseline"
	if _, err := s.Reserve(stale, "c1"); err == nil {
		t.Fatal("stale baseline must be denied")
	}
	stale = b
	stale.PolicyFingerprint = "other-policy"
	if _, err := s.Reserve(stale, "c1"); err == nil {
		t.Fatal("stale policy must be denied")
	}
	stale = b
	stale.Route = "other"
	if _, err := s.Reserve(stale, "c1"); err == nil {
		t.Fatal("illegal route must be denied")
	}
}

func TestRuntimeEnforcementLedgerRejectsMalformedStateBeforeMutation(t *testing.T) {
	s := NewRuntimeEnforcementStore(t.TempDir())
	b := runtimeBinding(t, "feature-a", "s1", "delegate")
	if _, err := s.Refresh(b, 2); err != nil {
		t.Fatal(err)
	}
	malformed := `{"format":"rotta.runtime-enforcement","version":1,"feature":"feature-a","worktree":"` + b.Worktree + `","baseline":"base","contract_fingerprint":"contract","policy_fingerprint":"policy","session":"s1","route":"delegate","budget":1,"charged":1,"reservations":{},"terminal":false}`
	if err := os.WriteFile(s.Path(), []byte(malformed), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reserve(b, "c1"); err == nil || !strings.Contains(err.Error(), "malformed runtime enforcement ledger") {
		t.Fatalf("reserve malformed ledger = %v", err)
	}
	after, err := os.ReadFile(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("malformed ledger was mutated")
	}
}

func TestRuntimeEnforcementRejectsInvalidWorktreeBeforeLedgerAccessOrReservation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		worktree func(t *testing.T) string
	}{
		{"missing", func(t *testing.T) string { return filepath.Join(t.TempDir(), "missing") }},
		{"deleted", func(t *testing.T) string {
			path := t.TempDir()
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			return path
		}},
		{"dangling symlink", func(t *testing.T) string {
			path := filepath.Join(t.TempDir(), "worktree")
			if err := os.Symlink(filepath.Join(t.TempDir(), "missing"), path); err != nil {
				t.Fatal(err)
			}
			return path
		}},
		{"non-directory", func(t *testing.T) string {
			path := filepath.Join(t.TempDir(), "worktree")
			if err := os.WriteFile(path, []byte("not a directory"), 0o600); err != nil {
				t.Fatal(err)
			}
			return path
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := NewRuntimeEnforcementStore(t.TempDir())
			b := runtimeBinding(t, "feature-a", "s1", "delegate")
			if _, err := s.Refresh(b, 1); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(s.Path(), []byte("not a ledger"), 0o600); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(s.Path())
			if err != nil {
				t.Fatal(err)
			}
			invalid := b
			invalid.Worktree = tc.worktree(t)
			if _, err := s.Reserve(invalid, "c1"); err == nil || !strings.Contains(err.Error(), "canonical runtime enforcement worktree") {
				t.Fatalf("reserve with invalid worktree = %v", err)
			}
			after, err := os.ReadFile(s.Path())
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(before) {
				t.Fatal("invalid worktree mutated ledger")
			}
		})
	}
}

func TestRuntimeEnforcementCanonicalizesResolvableWorktreeSymlink(t *testing.T) {
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "worktree")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	s := NewRuntimeEnforcementStore(t.TempDir())
	b := runtimeBinding(t, "feature-a", "s1", "delegate")
	b.Worktree = link
	l, err := s.Refresh(b, 1)
	if err != nil {
		t.Fatal(err)
	}
	if l.Worktree != target {
		t.Fatalf("canonical worktree = %q, want %q", l.Worktree, target)
	}
}

func TestRuntimeEnforcementRejectsImpossibleStatesBeforeReservation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		ledger string
	}{
		{"terminal without committed call", `"budget":0,"charged":0,"reservations":{},"terminal":true`},
		{"terminal active reservation", `"budget":2,"charged":2,"reservations":{"c1":{"call_id":"c1","state":"committed"},"c2":{"call_id":"c2","state":"reserved"}},"terminal":true`},
		{"multiple committed calls", `"budget":2,"charged":2,"reservations":{"c1":{"call_id":"c1","state":"committed"},"c2":{"call_id":"c2","state":"committed"}},"terminal":true`},
		{"committed nonterminal", `"budget":1,"charged":1,"reservations":{"c1":{"call_id":"c1","state":"committed"}},"terminal":false`},
		{"charged beyond budget", `"budget":0,"charged":1,"reservations":{"c1":{"call_id":"c1","state":"reserved"}},"terminal":false`},
		{"inconsistent charge", `"budget":1,"charged":0,"reservations":{"c1":{"call_id":"c1","state":"reserved"}},"terminal":false`},
		{"invalid reservation", `"budget":1,"charged":1,"reservations":{"c1":{"call_id":"other","state":"reserved"}},"terminal":false`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := NewRuntimeEnforcementStore(t.TempDir())
			b := runtimeBinding(t, "feature-a", "s1", "delegate")
			malformed := `{"format":"rotta.runtime-enforcement","version":1,"feature":"feature-a","worktree":"` + b.Worktree + `","baseline":"` + b.Baseline + `","contract_fingerprint":"` + b.ContractFingerprint + `","policy_fingerprint":"` + b.PolicyFingerprint + `","session":"s1","route":"delegate",` + tc.ledger + `}`
			if err := os.MkdirAll(filepath.Dir(s.Path()), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(s.Path(), []byte(malformed), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Reserve(b, "new-call"); err == nil || !strings.Contains(err.Error(), "malformed runtime enforcement ledger") {
				t.Fatalf("reserve impossible ledger = %v", err)
			}
			after, err := os.ReadFile(s.Path())
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != malformed {
				t.Fatal("impossible ledger was mutated or reservation authorized")
			}
		})
	}
}

func TestRuntimeEnforcementRemediationIsPreHandoffAndNonTerminalOnly(t *testing.T) {
	t.Run("post handoff", func(t *testing.T) {
		s := NewRuntimeEnforcementStore(t.TempDir())
		from := runtimeBinding(t, "feature-a", "s1", "delegate")
		to := from
		to.Session = "s2"
		if _, err := s.Refresh(from, 2); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Handoff(from, to); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Remediation(to); err == nil {
			t.Fatal("post-handoff remediation must be denied")
		}
	})
	t.Run("terminal", func(t *testing.T) {
		s := NewRuntimeEnforcementStore(t.TempDir())
		b := runtimeBinding(t, "feature-a", "s1", "delegate")
		if _, err := s.Refresh(b, 2); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Reserve(b, "c1"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Commit(b, "c1"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Remediation(b); err == nil {
			t.Fatal("terminal remediation must be denied")
		}
	})
}

func TestRuntimeEnforcementRemediationRecoveryIsIdempotent(t *testing.T) {
	s := NewRuntimeEnforcementStore(t.TempDir())
	b := runtimeBinding(t, "feature-a", "s1", "delegate")
	if _, err := s.Refresh(b, 2); err != nil {
		t.Fatal(err)
	}
	if l, err := s.Remediation(b); err != nil || !l.Remediated {
		t.Fatalf("remediation = %#v, %v", l, err)
	}
	// A replay after a crash observes the durable first transition and cannot
	// perform another remediation.
	if _, err := s.Remediation(b); err == nil || !strings.Contains(err.Error(), "second remediation") {
		t.Fatalf("recovery replay = %v", err)
	}
}
