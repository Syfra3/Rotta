#!/usr/bin/env python3
"""Verify named critical functions against Go coverprofile statement coverage."""

import argparse
import json
import re
import subprocess
import sys


FUNCTION = re.compile(r"^(.+):\d+:\s+(\S+)\s+([\d.]+)%$")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--inventory", required=True)
    parser.add_argument("--profile", required=True)
    args = parser.parse_args()
    inventory = json.load(open(args.inventory, encoding="utf-8"))
    output = subprocess.check_output(["go", "tool", "cover", f"-func={args.profile}"], text=True)
    measured = {}
    for line in output.splitlines():
        match = FUNCTION.match(line)
        if match:
            measured[f"{match.group(1)}:{match.group(2)}"] = float(match.group(3))

    minimum = inventory["minimum_statement_coverage"]
    failures = []
    for function in inventory["functions"]:
        matches = [coverage for name, coverage in measured.items() if name.endswith(function)]
        if len(matches) != 1:
            failures.append(f"{function}: not uniquely present in Go coverprofile")
            continue
        print(f"{function}: {matches[0]:.1f}% statement coverage")
        if matches[0] < minimum:
            failures.append(f"{function}: {matches[0]:.1f}% < {minimum:.1f}%")
    if failures:
        print("critical-path statement coverage FAILED:", *failures, sep="\n  ", file=sys.stderr)
        return 1
    print(f"critical-path statement coverage PASSED: every inventory function >= {minimum:.1f}%")
    return 0


if __name__ == "__main__":
    sys.exit(main())
