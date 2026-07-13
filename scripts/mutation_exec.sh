#!/usr/bin/env bash
# Execute one mutation only inside the isolated worktree created by the harness.
set -eu

original="${MUTATE_ORIGINAL:?MUTATE_ORIGINAL is required}"
changed="${MUTATE_CHANGED:?MUTATE_CHANGED is required}"
package="${MUTATE_PACKAGE:?MUTATE_PACKAGE is required}"

cleanup() {
  if [ -f "$original.tmp" ]; then
    mv "$original.tmp" "$original"
  fi
}
trap cleanup EXIT HUP INT TERM

mv "$original" "$original.tmp"
cp "$changed" "$original"
if go test -count=1 -timeout "${MUTATE_TIMEOUT:-20}s" "$package" >/dev/null 2>&1; then
  diff -u "$original.tmp" "$original" || true
  exit 1 # Survivor: the mutated behavior passed every package test.
fi
exit 0 # Killed: tests or compilation rejected the mutation.
