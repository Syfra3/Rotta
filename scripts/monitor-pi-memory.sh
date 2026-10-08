#!/usr/bin/env bash
# Local, metadata-only Pi memory monitor. No heap snapshots or command lines.
set -euo pipefail
if [[ $# -lt 1 || $# -gt 2 || ! $1 =~ ^[1-9][0-9]*$ ]]; then
  echo 'usage: bash scripts/monitor-pi-memory.sh PID [INTERVAL_SECONDS]' >&2
  exit 2
fi
pid=$1
interval=${2:-5}
if [[ ! $interval =~ ^[1-9][0-9]*$ ]]; then
  echo 'interval must be a positive integer' >&2
  exit 2
fi
if [[ ! -r /proc/$pid/status || ! -r /proc/$pid/stat ]]; then
  echo 'PID is not readable or no longer exists' >&2
  exit 1
fi
# Linux starttime (field 22) prevents silently following a reused PID.
start_time() { awk '{sub(/^.*\\) /, ""); print $20}' "/proc/$pid/stat" 2>/dev/null; }
initial_start=$(start_time)
[[ -n $initial_start ]] || exit 1
# Output stays in a private temp directory; never print process arguments or environment.
dir=$(mktemp -d "${TMPDIR:-/tmp}/rotta-pi-memory.XXXXXXXX")
chmod 700 "$dir"
file=$dir/samples.csv
printf 'timestamp_utc,pid,rss_kb,virtual_kb,threads\n' > "$file"
chmod 600 "$file"
echo "Writing metadata-only samples to $file" >&2
while [[ -r /proc/$pid/status && $(start_time) == "$initial_start" ]]; do
  # /proc can disappear during a sample; stop rather than emit partial rows.
  row=$(awk '/^VmRSS:/ {rss=$2} /^VmSize:/ {vsz=$2} /^Threads:/ {threads=$2} END {if (rss != "" && vsz != "" && threads != "") print rss "," vsz "," threads}' "/proc/$pid/status") || break
  [[ -n $row ]] || break
  printf '%s,%s,%s\n' "$(date -u +%FT%TZ)" "$pid" "$row" >> "$file"
  sleep "$interval"
done
echo "Stopped; samples: $file" >&2
