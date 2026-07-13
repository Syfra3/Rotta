#!/usr/bin/env python3
"""Measure Go statement coverage for executable lines changed from a baseline."""

import argparse
import collections
import re
import subprocess
import sys


HUNK = re.compile(r"^\+\+\+ b/(.+)$|^@@ .*\+(\d+)(?:,(\d+))? @@")
BLOCK = re.compile(r"^(.+):(\d+)\.\d+,(\d+)\.\d+ (\d+) (\d+)$")


def changed_lines(baseline):
    output = subprocess.check_output(
        ["git", "diff", "--unified=0", f"{baseline}..HEAD", "--", "*.go"], text=True
    )
    changed = collections.defaultdict(set)
    current_file = None
    for line in output.splitlines():
        match = HUNK.match(line)
        if not match:
            continue
        if match.group(1):
            current_file = match.group(1)
            continue
        if current_file and int(match.group(3) or 1):
            start, length = int(match.group(2)), int(match.group(3) or 1)
            changed[current_file].update(range(start, start + length))
    return changed


def covered_changed_lines(profile, changed):
    covered, uncovered = set(), set()
    for entry in open(profile, encoding="utf-8").read().splitlines()[1:]:
        match = BLOCK.match(entry)
        if not match:
            raise ValueError(f"unrecognised coverprofile entry: {entry}")
        path, start, end, _, count = match.groups()
        for changed_path, lines in changed.items():
            if path.endswith("/" + changed_path):
                overlap = lines.intersection(range(int(start), int(end) + 1))
                target = covered if int(count) else uncovered
                target.update((changed_path, line) for line in overlap)
    return covered, uncovered - covered


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--baseline", required=True)
    parser.add_argument("--profile", required=True)
    parser.add_argument("--minimum", type=float, default=90.0)
    args = parser.parse_args()

    covered, uncovered = covered_changed_lines(args.profile, changed_lines(args.baseline))
    total = len(covered | uncovered)
    percent = 100 * len(covered) / total if total else 100.0
    print(f"changed executable lines: {len(covered)}/{total} = {percent:.2f}%")
    if uncovered:
        print("uncovered changed executable lines:")
        for path, lines in group_by_path(changed_lines(args.baseline), uncovered).items():
            print(f"  {path}: {format_lines(lines)}")
    return 0 if percent >= args.minimum else 1


def group_by_path(changed, lines):
    return {
        path: sorted(line for uncovered_path, line in lines if uncovered_path == path)
        for path in changed
        if any(uncovered_path == path for uncovered_path, _ in lines)
    }


def format_lines(lines):
    ranges = []
    for line in lines:
        if not ranges or line > ranges[-1][-1] + 1:
            ranges.append([line, line])
        else:
            ranges[-1][-1] = line
    return ", ".join(str(start) if start == end else f"{start}-{end}" for start, end in ranges)


if __name__ == "__main__":
    sys.exit(main())
