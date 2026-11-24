#!/usr/bin/env python3
"""Opt-in Claude transcript metadata exporter; never sends transcript content."""
import argparse
from datetime import datetime, timezone
import hashlib
import fcntl
import json
import math
import os
import re
import secrets
import stat
import sqlite3
import time
import urllib.parse
import urllib.request
from pathlib import Path

MAX_LINE = 1048576
MAX_SCAN_BYTES = 64 * 1024 * 1024
MAX_FILES = 10000
MAX_EVENTS = 100000
MAX_TOOLS = 100000
MAX_SERIES = 4096
MAX_CONTRIBUTIONS = 800000
TOOLS = {'Read', 'Edit', 'Write', 'MultiEdit', 'NotebookEdit', 'Bash', 'Grep', 'Glob', 'Task'}
EDIT = {'Edit', 'Write', 'MultiEdit', 'NotebookEdit'}
# Supported family names from the supplied model-windows.json; unknown is not 200k.
WINDOW = {'opus': 200000, 'sonnet': 200000, 'haiku': 200000}
COMMANDS = frozenset('git gh make pytest python python3 pip uv npm npx node go cargo docker kubectl curl jq ls cat grep rg find sed awk echo mkdir rm cp mv bash sh'.split())
SEPARATOR = re.compile(r'&&|\|\||[;|\n]')
QUOTED = re.compile(r"'[^']*'|\"(?:\\.|[^\"\\])*\"")


def command_metadata(value):
    if not isinstance(value, str):
        return 'other', ()
    # Drop quoted strings and heredoc bodies before classifying. Never return text.
    lines, marker = [], None
    for line in value.splitlines():
        if marker:
            if line.strip() == marker:
                marker = None
            continue
        match = re.search(r'<<-?\s*[\'\"]?(\w+)', line)
        if match:
            marker = match.group(1)
        lines.append(line)
    text = QUOTED.sub('""', '\n'.join(lines))
    found, category = set(), 'other'
    for segment in SEPARATOR.split(text):
        tokens = segment.split()
        while tokens and (re.match(r'^[A-Za-z_][A-Za-z_0-9]*=', tokens[0]) or tokens[0] in ('sudo', 'env', 'command', 'time', 'nohup')):
            tokens.pop(0)
        if not tokens:
            continue
        head = tokens[0].rsplit('/', 1)[-1]
        if category == 'other' and head in COMMANDS:
            category = head
        if head == 'gh':
            found.add('pr_create' if tokens[1:3] == ['pr', 'create'] else 'gh_other')
        if head == 'git':
            args = tokens[1:]
            while args and args[0].startswith('-'):
                option = args.pop(0)
                if option in ('-C', '-c', '--git-dir', '--work-tree', '--namespace') and args:
                    args.pop(0)
            if args:
                if args[0] in ('commit', 'push'):
                    found.add(args[0])
                elif args[0] == 'switch' or (args[0] == 'checkout' and '--' not in args and (any(a in ('-b', '-B') for a in args[1:]) or (len(args) == 2 and re.fullmatch(r'[A-Za-z0-9._/@+-]+', args[1]) and not re.search(r'^[./~]|\.[A-Za-z]\w*$', args[1])))):
                    found.add('branch_switch')
    return category, tuple(x for x in ('commit', 'push', 'pr_create', 'branch_switch', 'gh_other') if x in found)


def safe_count(value):
    return value if type(value) is int and 0 <= value <= 100000000 else 0


def pseudonym(value, salt):
    return hashlib.sha256(salt + value.encode()).hexdigest()[:16]


def model_class(value):
    if not isinstance(value, str):
        return 'unknown'
    return next((x for x in ('opus', 'sonnet', 'haiku') if x in value.lower()), 'unknown')


def attributes(values):
    return [{'key': k, 'value': {'stringValue': str(v).lower() if isinstance(v, bool) else str(v)}} for k, v in values.items()]


def resource():
    return {'attributes': attributes({'service.name': 'claude-code', 'telemetry.source': 'transcript'})}


def timestamp_ns(value):
    if not isinstance(value, str):
        return None
    try:
        dt = datetime.fromisoformat(value.replace('Z', '+00:00'))
        if dt.tzinfo is None:
            return None
        ns = int(dt.timestamp() * 1e9)
        age = time.time_ns() - ns
        return ns if ns > 0 and -300 * 10**9 <= age <= 12 * 3600 * 10**9 else None
    except (ValueError, OverflowError, OSError):
        return None


def derive(record, salt, tool_names=None, contexts=None):
    """Return atomic (key, signal, payload) units; native token/cost counters are never reproduced."""
    if not isinstance(record, dict) or record.get('type') not in ('user', 'assistant', 'system'):
        return []
    ns = timestamp_ns(record.get('timestamp'))
    message, session = record.get('message'), record.get('sessionId')
    if ns is None or not isinstance(session, str) or not session or (record.get('type') != 'system' and not isinstance(message, dict)):
        return []
    contexts = contexts if contexts is not None else {}
    tool_names = tool_names if tool_names is not None else {}
    hash_id = lambda value: pseudonym(value, salt)
    sid = hash_id(session)
    source = 'subagent' if record.get('isSidechain') is True or isinstance(record.get('agentId'), str) else 'main'
    branch = record.get('gitBranch')
    branch_id = hash_id(branch)[:8] if isinstance(branch, str) and branch else 'none'
    branch_class = 'default' if branch in ('main', 'master', 'trunk', 'develop') else ('feature' if branch_id != 'none' and branch != 'HEAD' else 'unknown')
    if branch_class == 'unknown':
        branch_id = 'none'
    prompt = record.get('promptId')
    is_prompt = record['type'] == 'user' and isinstance(message, dict) and (
        isinstance(message.get('content'), str) or isinstance(message.get('content'), list)
        and not any(isinstance(x, dict) and x.get('type') == 'tool_result' for x in message['content']))
    thread = hash_id('thread:' + session + ':' + str(record.get('agentId', 'main')))
    clock = contexts.setdefault('clock:' + thread, {'last_ns': ns, 'prompt_id': hash_id('session:' + session)})
    previous_ns = min(clock['last_ns'], ns)
    if isinstance(prompt, str) and prompt:
        clock['prompt_id'] = hash_id(prompt)
    elif is_prompt and isinstance(record.get('uuid'), str):
        clock['prompt_id'] = hash_id(record['uuid'])
    pid = clock['prompt_id']
    clock['last_ns'] = ns
    base = {'data_source': 'transcript', 'session.id': sid, 'user.id': hash_id('member:local'), 'prompt.id': pid, 'query_source': source}
    result = []

    def emit(kind, identity, fields, metrics=(), span=None, span_start=None):
        if not isinstance(identity, str) or not identity:
            return
        key = hash_id(session + ':' + kind + ':' + identity)
        labels = {**base, **fields, 'event.name': kind, 'event.id': key}
        log = {'timeUnixNano': str(ns), 'body': {'stringValue': 'claude_code.' + kind}, 'attributes': attributes(labels)}
        result.append((key, 'logs', log))
        for name, value, dimensions, gauge in metrics:
            if type(value) in (int, float) and math.isfinite(value) and 0 <= value <= 100000000:
                point = {'timeUnixNano': str(ns), 'attributes': attributes({'data_source': 'transcript', 'user': hash_id('member:local'), 'query_source': source, **dimensions}), 'asDouble': float(value)}
                metric = {'name': name, 'unit': 'tokens' if name == 'claude_code.context.size' else ('1' if name == 'claude_code.context.utilization' else ''),
                          ('gauge' if gauge else 'sum'): {'dataPoints': [point]}}
                if not gauge:
                    metric['sum'].update({'aggregationTemporality': 1, 'isMonotonic': True})
                result.append((key, 'metrics', metric))
        if span:
            span_prompt = labels['prompt.id']
            trace = hashlib.sha256(salt + (session + ':' + span_prompt + ':trace').encode()).hexdigest()[:32]
            root_id = hash_id('interaction:' + sid + ':' + span_prompt)
            span_id = root_id if span == 'claude_code.interaction' else hash_id(session + ':' + kind + ':' + identity)
            start = min(span_start, ns) if span_start is not None else ns
            item = {'traceId': trace, 'spanId': span_id, 'name': span, 'kind': 1,
                    'startTimeUnixNano': str(start), 'endTimeUnixNano': str(max(start + 1, ns)),
                    'attributes': attributes(labels)}
            if span != 'claude_code.interaction':
                item['parentSpanId'] = root_id
            result.append((key, 'traces', item))

    emit('session_start', session, {}, [('claude_code.session.count', 1, {}, False)])
    if record['type'] == 'system' and record.get('subtype') == 'api_error':
        emit('api_error', record.get('uuid'), {'outcome': 'retry', 'model': 'unknown',
             'attempt': safe_count(record.get('retryAttempt')), 'max_retries': safe_count(record.get('maxRetries')),
             'retry_in_ms': min(safe_count(record.get('retryInMs')), 900000)})
    if is_prompt:
        emit('user_prompt', record.get('uuid'), {}, span='claude_code.interaction')
    if record['type'] == 'assistant':
        usage, mid = message.get('usage'), message.get('id')
        model = model_class(message.get('model'))
        if record.get('isApiErrorMessage') is True:
            emit('api_error', record.get('uuid'), {'outcome': 'failed', 'model': model})
        if isinstance(usage, dict) and isinstance(mid, str) and message.get('model') != '<synthetic>':
            counts = [safe_count(usage.get(k)) for k in ('input_tokens', 'cache_read_input_tokens', 'cache_creation_input_tokens')]
            tokens = sum(counts)
            dims = {'model': model, 'branch_class': branch_class, 'branch_id': branch_id}
            fields = {**dims, 'context_tokens': tokens, 'input_tokens': counts[0], 'output_tokens': safe_count(usage.get('output_tokens')),
                      'cache_read_tokens': counts[1], 'cache_creation_tokens': counts[2]}
            if 0 < ns - previous_ns <= 900000 * 1000000:
                # Same interval approximation used by the reference exporter;
                # this is transcript timing, not measured provider latency.
                fields['duration_ms'] = (ns - previous_ns) // 1000000
            metrics = [('claude_code.context.size', tokens, {'model': model}, True)]
            if model in WINDOW:
                window = 1000000 if model in ('opus', 'sonnet') and tokens > 200000 else WINDOW[model]
                pct = min(tokens / window, 1)
                fields.update(context_window=window, context_pct=round(pct, 4))
                metrics.append(('claude_code.context.utilization', pct, {'model': model}, True))
            context_key = sid if source == 'main' else None
            previous = contexts.get(context_key) if context_key else None
            if previous is not None and previous >= 20000 and tokens < previous * .5:
                fields.update(context_drop=True, context_previous_tokens=previous)
                metrics.append(('claude_code.context.drop.count', 1, {'model': model}, False))
            if context_key:
                contexts[context_key] = tokens
            emit('api_request', mid, fields, metrics, 'claude_code.llm_request', previous_ns)
        for item in message.get('content', []) if isinstance(message.get('content'), list) else []:
            if isinstance(item, dict) and item.get('type') == 'tool_use' and isinstance(item.get('id'), str):
                name = item.get('name') if item.get('name') in TOOLS else 'other'
                tool_key = hash_id(session + ':tool:' + item['id'])
                if tool_key not in tool_names and len(tool_names) >= MAX_TOOLS:
                    raise RuntimeError('tool state capacity reached; retain checkpoint and select a narrower input')
                info = item.get('input') if isinstance(item.get('input'), dict) else {}
                category, actions = command_metadata(info.get('command')) if name == 'Bash' else ('', ())
                path = next((info.get(k) for k in ('file_path', 'path', 'notebook_path') if isinstance(info.get(k), str)), None)
                tool_names[tool_key] = {'name': name, 'timestamp': ns, 'command_class': category, 'git_actions': actions,
                                        'path_hash': hash_id(path) if path else '', 'prompt_id': pid,
                                        'branch_class': branch_class, 'branch_id': branch_id}
                emit('tool_use', item['id'], {'tool_name': name}, [('claude_code.tool.count', 1, {'tool_name': name}, False)])
    if record['type'] == 'user':
        for item in message.get('content', []) if isinstance(message.get('content'), list) else []:
            if not isinstance(item, dict) or item.get('type') != 'tool_result' or not isinstance(item.get('tool_use_id'), str):
                continue
            call = tool_names.get(hash_id(session + ':tool:' + item['tool_use_id']), {})
            if isinstance(call, str):
                call = {'name': call}
            name = call.get('name', 'other')
            decision = record.get('permissionDecision')
            explicit = decision.get('decision') if isinstance(decision, dict) else None
            decision_source = decision.get('source') if isinstance(decision, dict) else None
            decision_source = decision_source if decision_source in ('config', 'user_temporary', 'user_permanent', 'hook', 'user_reject') else 'unknown'
            success = 'false' if item.get('is_error') is True or explicit == 'reject' else 'true'
            duration = (ns - call.get('timestamp', ns)) // 1000000
            fields = {'tool_name': name, 'success': success, 'branch_class': call.get('branch_class', branch_class),
                      'branch_id': call.get('branch_id', branch_id), 'prompt.id': call.get('prompt_id', pid)}
            if 0 <= duration <= 900000:
                fields['duration_ms'] = duration
            if call.get('path_hash'):
                fields['tool_parameters'] = json.dumps({'path_hash': call['path_hash']})
            if name == 'Bash':
                fields['command_class'] = call.get('command_class', 'other')
            actions = call.get('git_actions', ())
            if actions:
                fields['git_action'] = ','.join(actions)
            emit('tool_result', item['tool_use_id'], fields,
                 [('claude_code.tool.result.count', 1, {'tool_name': name, 'success': success}, False)] +
                 [('claude_code.git_action.count', 1, {'git_action': action, 'success': success, 'branch_class': fields['branch_class'], 'branch_id': fields['branch_id']}, False) for action in actions], 'claude_code.tool', call.get('timestamp', ns))
            if explicit in ('accept', 'reject'):
                emit('tool_decision', item['tool_use_id'], {'tool_name': name, 'decision': explicit, 'source': decision_source},
                     [('claude_code.code_edit_tool.decision', 1, {'tool_name': name, 'decision': explicit, 'source': decision_source}, False)] if name in EDIT else [])
    return result


def payload(signal, items):
    prefix, scope, key = {'metrics': ('resourceMetrics', 'scopeMetrics', 'metrics'), 'logs': ('resourceLogs', 'scopeLogs', 'logRecords'), 'traces': ('resourceSpans', 'scopeSpans', 'spans')}[signal]
    return {prefix: [{'resource': resource(), scope: [{key: items}]}]}


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise ValueError('collector redirects are not permitted')


def run(projects, state, endpoint, send=None):
    parsed = urllib.parse.urlsplit(endpoint)
    if parsed.scheme != 'http' or parsed.hostname != '127.0.0.1' or parsed.port != 14318 or parsed.username or parsed.password or parsed.path not in ('', '/') or parsed.query or parsed.fragment:
        raise ValueError('only local collector http://127.0.0.1:14318 is accepted')
    if any(p.is_symlink() for root in (state, projects) for p in (root, *root.parents)):
        raise ValueError('symlink root')
    if not projects.is_dir():
        raise ValueError('explicit projects directory must exist')
    if state.exists():
        allowed = {'writer.lock', 'identity.salt', 'claude.sqlite', 'claude.sqlite-journal', 'claude.sqlite-wal', 'claude.sqlite-shm'}
        if not state.is_dir() or state.stat().st_uid != os.getuid() or any(p.name not in allowed for p in state.iterdir()):
            raise ValueError('state must be a dedicated owned exporter directory')
        for path in state.iterdir():
            info = path.lstat()
            if not stat.S_ISREG(info.st_mode) or info.st_uid != os.getuid() or info.st_nlink != 1:
                raise ValueError('state files must be owned regular files without links')
    state.mkdir(parents=True, exist_ok=True, mode=0o700)
    os.chmod(state, 0o700)
    fd = os.open(state / 'writer.lock', os.O_WRONLY | os.O_CREAT | os.O_NOFOLLOW, 0o600)
    try:
        try:
            fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError as exc:
            raise RuntimeError('another exporter owns this state directory') from exc
        _run_locked(projects, state, endpoint.rstrip('/'), send)
    finally:
        os.close(fd)


def _run_locked(projects, state, endpoint, send):
    salt_path = state / 'identity.salt'
    for filename in ('identity.salt', 'claude.sqlite', 'claude.sqlite-journal', 'claude.sqlite-wal', 'claude.sqlite-shm'):
        if (state / filename).is_symlink():
            raise ValueError('symlink state')
    if not salt_path.exists():
        fd = os.open(salt_path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
        with os.fdopen(fd, 'wb') as stream:
            stream.write(secrets.token_bytes(32))
    if not stat.S_ISREG(salt_path.stat().st_mode):
        raise ValueError('invalid salt')
    os.chmod(salt_path, 0o600)
    salt = salt_path.read_bytes()
    if len(salt) != 32:
        raise ValueError('invalid salt')
    db_fd = os.open(state / 'claude.sqlite', os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW, 0o600)
    os.close(db_fd)
    os.chmod(state / 'claude.sqlite', 0o600)
    db = sqlite3.connect(state / 'claude.sqlite')
    try:
        db.execute('CREATE TABLE IF NOT EXISTS seen (key TEXT PRIMARY KEY, fingerprint TEXT NOT NULL DEFAULT "")')
        if 'fingerprint' not in {row[1] for row in db.execute('PRAGMA table_info(seen)')}:
            db.execute('ALTER TABLE seen ADD COLUMN fingerprint TEXT NOT NULL DEFAULT ""')
        db.execute('CREATE TABLE IF NOT EXISTS series (name TEXT PRIMARY KEY, total REAL, start_ns INTEGER)')
        db.execute('PRAGMA secure_delete=ON')
        db.execute('CREATE TABLE IF NOT EXISTS pending (key TEXT PRIMARY KEY, body TEXT NOT NULL, sent INTEGER NOT NULL DEFAULT 0)')
        db.execute('CREATE TABLE IF NOT EXISTS metadata_receipts (key TEXT PRIMARY KEY)')
        # Old versions persisted full OTLP log/trace bodies. Keep cumulative metric
        # snapshots and accounting, but remove metadata before any network I/O.
        legacy = False
        for key, body in db.execute('SELECT key, body FROM pending').fetchall():
            signals = json.loads(body)
            safe = [(signal, request) for signal, request in signals if signal == 'metrics']
            if len(safe) != len(signals):
                legacy = True
                db.execute('UPDATE pending SET body=?, sent=0 WHERE key=?', (json.dumps(safe), key))
                db.execute('INSERT OR IGNORE INTO metadata_receipts VALUES (?)', (key,))
        db.execute('CREATE TABLE IF NOT EXISTS tools (id TEXT PRIMARY KEY, name TEXT NOT NULL)')
        db.execute('CREATE TABLE IF NOT EXISTS contexts (id TEXT PRIMARY KEY, tokens INTEGER NOT NULL)')
        if not db.execute("SELECT 1 FROM sqlite_master WHERE type='table' AND name='counted_metrics'").fetchone():
            db.execute('CREATE TABLE counted_metrics (event_key TEXT, series TEXT, PRIMARY KEY(event_key, series))')
            # Legacy checkpoints lack per-metric contributions. Never recount
            # already accepted legacy events when upgrading their state.
            db.execute("INSERT INTO counted_metrics SELECT key, '*' FROM seen")
        db.commit()
        if legacy:
            # Secure-delete covers freed cells; VACUUM rebuilds free pages and
            # truncates the DB. Checkpoint/truncate any old WAL frames as well.
            db.execute('PRAGMA wal_checkpoint(TRUNCATE)')
            db.execute('VACUUM')
            db.execute('PRAGMA wal_checkpoint(TRUNCATE)')
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())

        def http_send(signal, body):
            request = urllib.request.Request(endpoint + '/v1/' + signal, body.encode(), {'Content-Type': 'application/json'})
            with opener.open(request, timeout=3) as response:
                result = json.loads(response.read(65537) or b'{}')
                partial = result.get('partialSuccess', result.get('partial_success', {}))
                if any(str(partial.get(k, 0)) != '0' for k in ('rejectedDataPoints', 'rejectedLogRecords', 'rejectedSpans', 'rejected_data_points', 'rejected_log_records', 'rejected_spans')):
                    raise RuntimeError('collector partially rejected telemetry; checkpoint retained')
        sender = send or http_send

        def flush():
            # A stable outbox survives failure between signals. Cumulative metrics can
            # safely be retried even when a response is lost after backend acceptance.
            for key, body, sent in db.execute('SELECT key, body, sent FROM pending ORDER BY rowid').fetchall():
                for index, (signal, request) in enumerate(json.loads(body)):
                    if not (sent & (1 << index)):
                        sender(signal, json.dumps(request))
                        sent |= 1 << index
                        db.execute('UPDATE pending SET sent=? WHERE key=?', (sent, key))
                        db.commit()
                db.execute('DELETE FROM pending WHERE key=?', (key,))
                db.commit()
        flush()
        # Never evict an accepted ID to make room: replay would count it twice.
        # Capacity is explicit backpressure, not a best-effort accounting reset.
        seen_count = db.execute('SELECT count(*) FROM seen').fetchone()[0]
        tool_names = {k: json.loads(v) if v.startswith('{') else v for k, v in db.execute('SELECT id, name FROM tools')}
        # Recompute context across the bounded input on each scan: persisted final
        # context would compare the first old message to the last old message on replay.
        contexts = {}
        series_count = db.execute('SELECT count(*) FROM series').fetchone()[0]
        contribution_count = db.execute('SELECT count(*) FROM counted_metrics').fetchone()[0]
        if seen_count > MAX_EVENTS or len(tool_names) > MAX_TOOLS or series_count > MAX_SERIES or contribution_count > MAX_CONTRIBUTIONS:
            raise RuntimeError('checkpoint capacity exceeded; retain it for recovery')
        events, scanned, records = {}, 0, {}
        for file_index, path in enumerate(projects.rglob('*.jsonl')):
            if file_index >= MAX_FILES:
                raise RuntimeError('input file capacity reached; select a narrower input')
            if any(p.is_symlink() for p in (path, *path.parents)) or not path.is_file():
                continue
            fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
            with os.fdopen(fd, 'rb') as stream:
                while line := stream.readline(MAX_LINE + 2):
                    scanned += len(line)
                    if scanned > MAX_SCAN_BYTES:
                        raise RuntimeError('input byte capacity reached; select a narrower input')
                    if len(line) > MAX_LINE:
                        while not line.endswith(b'\n'):
                            line = stream.readline(MAX_LINE + 2)
                            scanned += len(line)
                            if scanned > MAX_SCAN_BYTES:
                                raise RuntimeError('input byte capacity reached; select a narrower input')
                            if not line:
                                break
                        continue
                    if not line.endswith(b'\n'):
                        continue
                    try:
                        record = json.loads(line)
                    except (UnicodeError, ValueError):
                        continue
                    if not isinstance(record, dict):
                        continue
                    stamp = timestamp_ns(record.get('timestamp'))
                    if stamp is None:
                        continue
                    message = record.get('message')
                    identity = message.get('id') if isinstance(message, dict) else None
                    identity = identity or record.get('uuid')
                    # Records without a stable message identity are still bounded by
                    # input bytes and can only produce independently keyed events.
                    key = (str(record.get('sessionId')), record.get('isSidechain') is True,
                           str(record.get('agentId', '')), str(record.get('type')),
                           str(identity) if identity else str((file_index, scanned)))
                    old = records.get(key)
                    if old is None and len(records) >= MAX_EVENTS:
                        raise RuntimeError('input record capacity reached; select a narrower input')
                    if old and record.get('type') == 'assistant':
                        older, newer = (old[1], record) if old[0] <= stamp else (record, old[1])
                        merged = {**older, **newer}
                        prior = older.get('message') if isinstance(older.get('message'), dict) else {}
                        final = newer.get('message') if isinstance(newer.get('message'), dict) else {}
                        merged_message = {**prior, **final}
                        if not isinstance(final.get('usage'), dict):
                            merged_message['usage'] = prior.get('usage')
                        calls = {}
                        for version in (prior, final):
                            content = version.get('content')
                            for item in content if isinstance(content, list) else []:
                                if isinstance(item, dict) and item.get('type') == 'tool_use' and isinstance(item.get('id'), str):
                                    calls[item['id']] = item
                        # Only tool blocks are needed for metadata; text never enters
                        # the sanitized event or persistent state.
                        merged_message['content'] = list(calls.values())
                        merged['message'] = merged_message
                        records[key] = (max(old[0], stamp), merged)
                    elif old is None or stamp >= old[0]:
                        records[key] = (stamp, record)
        # Derive context/tool relationships only after coalescing raw revisions and
        # ordering ALL files. Filesystem order must not invent context drops.
        for _, record in sorted(records.values(), key=lambda item: item[0]):
            rows = derive(record, salt, tool_names, contexts)
            grouped = {}
            for key, signal, item in rows:
                grouped.setdefault(key, []).append((signal, item))
            for key, selected in grouped.items():
                if key not in events and len(events) >= MAX_EVENTS:
                    raise RuntimeError('input event capacity reached; select a narrower input')
                event = next(item for signal, item in selected if signal == 'logs')
                stamp = int(event['timeUnixNano'])
                kind = event['body']['stringValue']
                old = events.get(key)
                if old is None or (kind != 'claude_code.session_start' and stamp >= old[0]) or (kind == 'claude_code.session_start' and stamp < old[0]):
                    events[key] = (stamp, selected)
        # Close each derived interaction at its latest observed child completion.
        # No wall-clock synthetic duration and no cross-prompt/session trace joins.
        trace_ends = {}
        for _, selected in events.values():
            for signal, item in selected:
                if signal == 'traces':
                    trace_ends[item['traceId']] = max(trace_ends.get(item['traceId'], 0), int(item['endTimeUnixNano']))
        for _, selected in events.values():
            for signal, item in selected:
                if signal == 'traces' and item['name'] == 'claude_code.interaction':
                    item['endTimeUnixNano'] = str(trace_ends[item['traceId']])
        for tid, name in tool_names.items():
            db.execute('INSERT OR REPLACE INTO tools VALUES (?,?)', (tid, json.dumps(name) if isinstance(name, dict) else name))
        db.commit()
        for key, (stamp, selected) in sorted(events.items(), key=lambda entry: (entry[1][0], entry[0])):
            fingerprint = hashlib.sha256(json.dumps(selected, sort_keys=True).encode()).hexdigest()
            previous = db.execute('SELECT fingerprint FROM seen WHERE key=?', (key,)).fetchone()
            if previous and previous[0] == fingerprint and not db.execute('SELECT 1 FROM metadata_receipts WHERE key=?', (key,)).fetchone():
                continue
            if not previous and seen_count >= MAX_EVENTS:
                raise RuntimeError('event checkpoint capacity reached; retain checkpoint and select a narrower input')
            signals = []
            with db:
                for signal in ('metrics', 'logs', 'traces'):
                    items = [item for s, item in selected if s == signal]
                    if signal == 'metrics':
                        for item in items:
                            if 'sum' not in item:
                                continue
                            point = item['sum']['dataPoints'][0]
                            name = item['name'] + ':' + json.dumps(point['attributes'], sort_keys=True)
                            old = db.execute('SELECT total, start_ns FROM series WHERE name=?', (name,)).fetchone()
                            if old is None and series_count >= MAX_SERIES:
                                raise RuntimeError('metric series capacity reached; checkpoint retained')
                            counted = db.execute("SELECT 1 FROM counted_metrics WHERE event_key=? AND series IN (?, '*')", (key, name)).fetchone()
                            total = (old[0] if old else 0) + (0 if counted else point['asDouble'])
                            if not counted:
                                if contribution_count >= MAX_CONTRIBUTIONS:
                                    raise RuntimeError('metric contribution capacity reached; checkpoint retained')
                                db.execute('INSERT INTO counted_metrics VALUES (?,?)', (key, name))
                                contribution_count += 1
                            start = old[1] if old else time.time_ns() - 1
                            db.execute('INSERT OR REPLACE INTO series VALUES (?,?,?)', (name, total, start))
                            series_count += old is None
                            point.update(asDouble=total, startTimeUnixNano=str(start), timeUnixNano=str(max(time.time_ns(), start + 1)))
                            item['sum']['aggregationTemporality'] = 2
                    if items:
                        signals.append((signal, payload(signal, items)))
                if not previous or previous[0] != fingerprint:
                    db.execute('INSERT OR REPLACE INTO pending(key,body,sent) VALUES (?,?,0)',
                               (key, json.dumps([entry for entry in signals if entry[0] == 'metrics'])))
                    db.execute('INSERT OR REPLACE INTO seen VALUES (?,?)', (key, fingerprint))
                    db.execute('INSERT OR IGNORE INTO metadata_receipts VALUES (?)', (key,))
            seen_count += previous is None
            flush()
            # Only a receipt survives: reconstruct metadata from the bounded source
            # on a later invocation if transport fails. Never stage bodies in SQLite.
            if db.execute('SELECT 1 FROM metadata_receipts WHERE key=?', (key,)).fetchone():
                if time.time_ns() - stamp < 24 * 60 * 60 * 1_000_000_000:
                    for signal, request in signals:
                        if signal != 'metrics':
                            sender(signal, json.dumps(request))
                db.execute('DELETE FROM metadata_receipts WHERE key=?', (key,))
                db.commit()
    finally:
        db.close()


def explicit_path(value):
    if not value.strip():
        raise argparse.ArgumentTypeError('an explicit nonempty directory path is required')
    return Path(value)


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--projects-dir', type=explicit_path, required=True)
    parser.add_argument('--state-dir', type=explicit_path, required=True)
    parser.add_argument('--endpoint', default='http://127.0.0.1:14318')
    args = parser.parse_args()
    run(args.projects_dir, args.state_dir, args.endpoint)
