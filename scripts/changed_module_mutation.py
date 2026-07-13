#!/usr/bin/env python3
"""Run bounded critical changed-module mutation testing in a detached worktree."""

import argparse
import json
import pathlib
import shutil
import subprocess
import tempfile
import time

TARGETS = ["./internal/installer", "./internal/tui", "./internal/workflow"]
CRITICAL_FUNCTIONS = [
    "integrationInstructions",
    "memoryInstructions",
    "velaInstructions",
    "context7Instructions",
    "mcpStatusResult",
    "statusForCapability",
    "writeMCPStatuses",
    "writeHostMCPStatuses",
]


def command(cwd, *args, **kwargs):
    return subprocess.run(args, cwd=cwd, text=True, capture_output=True, check=False, **kwargs)


def apply_checkout_diff(root, worktree):
    patch = command(root, "git", "diff", "--binary", "HEAD")
    if patch.returncode:
        raise RuntimeError(patch.stderr)
    if patch.stdout:
        applied = subprocess.run(["git", "apply", "--whitespace=nowarn"], cwd=worktree, text=True, input=patch.stdout, capture_output=True, check=False)
        if applied.returncode:
            raise RuntimeError(applied.stderr)
    untracked = command(root, "git", "ls-files", "--others", "--exclude-standard")
    if untracked.returncode:
        raise RuntimeError(untracked.stderr)
    for relative in filter(None, untracked.stdout.splitlines()):
        source = root / relative
        destination = worktree / relative
        if source.is_file():
            destination.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(source, destination)


def mutation_counts(output):
    killed, survivors = [], []
    for line in output.splitlines():
        if line.startswith("PASS "):
            killed.append(line)
        elif line.startswith("FAIL "):
            survivors.append(line)
    return killed, survivors


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--timeout", type=int, default=900)
    parser.add_argument("--report", default="reports/changed_module_mutation.json")
    args = parser.parse_args()
    root = pathlib.Path(command(".", "git", "rev-parse", "--show-toplevel").stdout.strip()).resolve()
    report_path = root / args.report
    report_path.parent.mkdir(parents=True, exist_ok=True)
    worktree = pathlib.Path(tempfile.mkdtemp(prefix="rotta-changed-module-mutation-"))
    started = time.monotonic()
    result = None
    timed_out = False
    try:
        added = command(root, "git", "worktree", "add", "--detach", str(worktree), "HEAD")
        if added.returncode:
            raise RuntimeError(added.stderr)
        apply_checkout_diff(root, worktree)
        for name in ("changed_module_mutation.py", "mutation_exec.sh"):
            shutil.copy2(root / "scripts" / name, worktree / "scripts" / name)
        (worktree / "scripts" / "mutation_exec.sh").chmod(0o755)
        matcher = "^(" + "|".join(CRITICAL_FUNCTIONS) + ")$"
        result = subprocess.run(
            ["go-mutesting", "--match", matcher, "--exec", "./scripts/mutation_exec.sh", "--exec-timeout", "20", *TARGETS],
            cwd=worktree, text=True, capture_output=True, check=False, timeout=args.timeout,
        )
    except subprocess.TimeoutExpired as error:
        timed_out = True
        result = error
    finally:
        command(root, "git", "worktree", "remove", "--force", str(worktree))
        shutil.rmtree(worktree, ignore_errors=True)

    output = "" if result is None else (result.stdout or "") + (result.stderr or "")
    killed, survivors = mutation_counts(output)
    tested = len(killed) + len(survivors)
    score = 100 * len(killed) / tested if tested else 0.0
    report = {
        "scope": "changed modules; all mutations generated for the declared critical changed-function catalog",
        "targets": TARGETS,
        "critical_functions": CRITICAL_FUNCTIONS,
        "timeout_seconds": args.timeout,
        "elapsed_seconds": round(time.monotonic() - started, 2),
        "timed_out": timed_out,
        "killed": len(killed),
        "survivors": survivors,
        "surviving_critical_mutations": len(survivors),
        "mutation_score": round(score, 2),
        "output": output,
    }
    report_path.write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({key: report[key] for key in ("mutation_score", "killed", "survivors", "surviving_critical_mutations", "timed_out")}, indent=2))
    return 0 if not timed_out and tested and score >= 80 and not survivors else 1


if __name__ == "__main__":
    raise SystemExit(main())
