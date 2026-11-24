"""Explicit-file, offline Pi JSONL metadata reader. Never discovers HOME or sessions.

The owned output JSONL is the idempotent sink: replay after a crash scans its bounded
IDs before writing. No stdout emission is used as a delivery guarantee.
"""
import hashlib
import json
import os
from pathlib import Path

MAX_LINE = 1_048_576
MAX_ENTRIES = 10000
VERSION = 1


def _digest(value):
    return hashlib.sha256(value).hexdigest()


def _load_lines(path):
    if not path.exists():
        return []
    rows = []
    with path.open('rb') as source:
        for _ in range(MAX_ENTRIES + 1):
            line = source.readline(MAX_LINE + 2)
            if not line:
                return rows
            if len(line) > MAX_LINE or not line.endswith(b'\n'):
                raise ValueError('bounded sink/checkpoint limit exceeded')
            rows.append(json.loads(line))
    raise ValueError('bounded sink/checkpoint limit exceeded')


def read_history(session, checkpoint, sink):
    """Process an explicitly supplied synthetic session path; caller owns isolation.

    Returns newly persisted metadata records. Rejects rewrites and unknown versions
    rather than guessing whether old data has already been delivered.
    """
    session, checkpoint, sink = map(Path, (session, checkpoint, sink))
    if session.resolve() in (checkpoint.resolve(), sink.resolve()) or checkpoint.resolve() == sink.resolve():
        raise ValueError('distinct paths required')
    if checkpoint.exists():
        with checkpoint.open('rb') as source:
            raw = source.read(MAX_LINE + 1)
            if len(raw) > MAX_LINE or source.read(1):
                raise ValueError('bounded checkpoint limit exceeded')
        state = json.loads(raw)
    else:
        state = {'version': VERSION, 'offset': 0, 'prefix': _digest(b''), 'header': None}
    if set(state) != {'version', 'offset', 'prefix', 'header'} or state['version'] != VERSION:
        raise ValueError('unknown checkpoint format')
    existing = _load_lines(sink)
    if any(set(row) != {'key', 'kind', 'status'} for row in existing):
        raise ValueError('unknown sink format')
    seen = {row['key'] for row in existing}
    if len(seen) != len(existing):
        raise ValueError('duplicate sink keys')
    offset = state['offset']
    if type(offset) is not int or offset < 0:
        raise ValueError('invalid checkpoint offset')
    header = state['header']
    new = []
    prefix = hashlib.sha256()
    with session.open('rb') as source:
        remaining = offset
        while remaining:
            block = source.read(min(65536, remaining))
            if not block:
                raise ValueError('session rewritten or truncated; explicit reset required')
            prefix.update(block)
            remaining -= len(block)
        if prefix.hexdigest() != state['prefix']:
            raise ValueError('session rewritten or truncated; explicit reset required')
        cursor = offset
        count = 0
        while True:
            line = source.readline(MAX_LINE + 2)
            if not line:
                break
            if len(line) > MAX_LINE:
                raise ValueError('bounded history limit exceeded')
            if not line.endswith(b'\n'):
                break
            count += 1
            if len(existing) + count > MAX_ENTRIES:
                raise ValueError('bounded history limit exceeded')
            prefix.update(line)
            entry = json.loads(line)
            if not isinstance(entry, dict):
                raise ValueError('invalid entry')
            if cursor == 0:
                if entry.get('type') != 'session' or entry.get('version') != 3 or not isinstance(entry.get('id'), str) or not entry['id']:
                    raise ValueError('unsupported Pi session header')
                header = _digest(line)
                record = {'key': _digest(('session:' + entry['id']).encode()), 'kind': 'session', 'status': 'derived'}
                if record['key'] not in seen:
                    seen.add(record['key'])
                    new.append(record)
            elif entry.get('type') not in {'message', 'model_change', 'thinking_level_change', 'usage', 'compaction', 'context_edit', 'branch_summary', 'custom', 'custom_message', 'label', 'session_info'}:
                raise ValueError('unsupported Pi entry type')
            elif not isinstance(entry.get('id'), str) or not isinstance(entry.get('parentId'), (str, type(None))):
                raise ValueError('invalid Pi entry')
            elif entry['type'] == 'message' and isinstance(entry.get('message'), dict) and entry['message'].get('role') == 'toolResult':
                status = 'error' if entry['message'].get('isError') is True else 'ok' if entry['message'].get('isError') is False else 'unknown'
                record = {'key': _digest((header + ':' + entry['id']).encode()), 'kind': 'tool_result', 'status': status}
                if record['key'] not in seen:
                    seen.add(record['key'])
                    new.append(record)
            cursor += len(line)
    end = cursor
    if state['header'] is not None and header != state['header']:
        raise ValueError('session identity changed')
    # Commit sink first. A crash before checkpoint update replays safely via sink keys.
    if new:
        with sink.open('ab') as output:
            os.chmod(sink, 0o600)
            for row in new:
                output.write((json.dumps(row, separators=(',', ':')) + '\n').encode())
            output.flush()
            os.fsync(output.fileno())
    next_state = {'version': VERSION, 'offset': end, 'prefix': prefix.hexdigest(), 'header': header}
    temporary = checkpoint.with_name(checkpoint.name + '.tmp')
    with temporary.open('w') as output:
        os.chmod(temporary, 0o600)
        json.dump(next_state, output, separators=(',', ':'))
        output.flush()
        os.fsync(output.fileno())
    os.replace(temporary, checkpoint)
    return new
