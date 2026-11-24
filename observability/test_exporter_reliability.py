"""Synthetic replay/transport/capacity regressions, independent of real history."""
from contextlib import closing
import importlib.util
import json
import sqlite3
import subprocess
import sys
import tempfile
import unittest
from datetime import datetime, timedelta, timezone
from pathlib import Path
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('reliable_exporter', Path(__file__).with_name('claude_exporter.py'))
e = importlib.util.module_from_spec(spec)
spec.loader.exec_module(e)
ENDPOINT = 'http://127.0.0.1:14318'


def record(session, mid, seconds=0, content=None, usage=None, kind='assistant'):
    return {'type': kind, 'timestamp': (datetime.now(timezone.utc) - timedelta(seconds=60-seconds)).isoformat(),
            'sessionId': session, 'message': {'id': mid, 'usage': usage, 'content': content or []}}


def metrics(sent, name):
    return [m for signal, body in sent if signal == 'metrics'
            for m in json.loads(body)['resourceMetrics'][0]['scopeMetrics'][0]['metrics'] if m['name'] == name]


class Reliability(unittest.TestCase):
    def test_real_timestamps_revisions_and_session_start_do_not_replay(self):
        rows = [record('s', 'a', usage={'input_tokens': 2}),
                record('s', 'a', 1, usage={'input_tokens': 8}),
                record('s', 'b', 2, content=[{'type': 'tool_use', 'id': 't', 'name': 'Edit'}]),
                record('s', 'r', 3, kind='user', content=[{'type': 'tool_result', 'tool_use_id': 't'}])]
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            (root/'a.jsonl').write_text(''.join(json.dumps(r)+'\n' for r in rows))
            sent = []
            send = lambda signal, body: sent.append((signal, body))
            e.run(root, root/'state', ENDPOINT, send=send)
            before = len(sent)
            e.run(root, root/'state', ENDPOINT, send=send)
            self.assertEqual(len(sent), before, 'unchanged history must not oscillate through old revisions')
            self.assertEqual(len(metrics(sent, 'claude_code.context.size')), 1)
            self.assertEqual(metrics(sent, 'claude_code.context.size')[0]['gauge']['dataPoints'][0]['asDouble'], 8)
            self.assertEqual(metrics(sent, 'claude_code.session.count')[-1]['sum']['dataPoints'][0]['asDouble'], 1)
            self.assertIn('Edit', ''.join(body for _, body in sent))

    def test_checkpointed_transport_retry_preserves_exact_cumulative_body(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            rows = [record('s1', 'a'), record('s2', 'b', 1)]
            (root/'a.jsonl').write_text(''.join(json.dumps(r)+'\n' for r in rows))
            accepted, attempts = [], []
            failed = False
            def send(signal, body):
                nonlocal failed
                attempts.append((signal, body))
                if signal == 'logs' and not failed:
                    failed = True
                    raise OSError('synthetic transport failure')
                accepted.append((signal, body))
            with self.assertRaises(OSError):
                e.run(root, root/'state', ENDPOINT, send=send)
            with closing(sqlite3.connect(root/'state'/'claude.sqlite')) as db:
                self.assertTrue(all('logs' not in body and 'traces' not in body
                                    for (body,) in db.execute('SELECT body FROM pending')))
                self.assertEqual(db.execute('SELECT count(*) FROM metadata_receipts').fetchone()[0], 1)
            e.run(root, root/'state', ENDPOINT, send=send)
            self.assertEqual(len(metrics(accepted, 'claude_code.session.count')), 2)
            points = [m['sum']['dataPoints'][0] for m in metrics(accepted, 'claude_code.session.count')]
            self.assertEqual([p['asDouble'] for p in points], [1, 2])
            self.assertEqual(points[0]['startTimeUnixNano'], points[1]['startTimeUnixNano'])
            before = len(accepted)
            e.run(root, root/'state', ENDPOINT, send=send)
            self.assertEqual(len(accepted), before)

    def test_capacity_never_evicts_counted_ids(self):
        with tempfile.TemporaryDirectory() as temp, patch.object(e, 'MAX_EVENTS', 2, create=True):
            root = Path(temp)
            path = root/'a.jsonl'
            path.write_text(json.dumps(record('s1', 'a'))+'\n'+json.dumps(record('s2', 'b', 1))+'\n')
            sent = []
            send = lambda signal, body: sent.append((signal, body))
            e.run(root, root/'state', ENDPOINT, send=send)
            path.write_text(path.read_text()+json.dumps(record('s3', 'c', 2))+'\n')
            with self.assertRaisesRegex(RuntimeError, 'capacity'):
                e.run(root, root/'state', ENDPOINT, send=send)
            with closing(sqlite3.connect(root/'state'/'claude.sqlite')) as db:
                self.assertEqual(db.execute('SELECT count(*) FROM seen').fetchone()[0], 2)
                self.assertEqual(db.execute('SELECT total FROM series').fetchone()[0], 2)
            path.write_text(json.dumps(record('s1', 'a'))+'\n')
            before = len(sent)
            e.run(root, root/'state', ENDPOINT, send=send)
            # A timestamp revision may update metadata, but never increases its counter.
            self.assertTrue(all(m['sum']['dataPoints'][0]['asDouble'] <= 2
                                for m in metrics(sent[before:], 'claude_code.session.count')))

    def test_tool_capacity_is_explicit_not_unbounded_or_silent_eviction(self):
        with tempfile.TemporaryDirectory() as temp, patch.object(e, 'MAX_TOOLS', 1, create=True):
            root = Path(temp)
            row = record('s', 'a', content=[{'type':'tool_use','id':str(i),'name':'Edit'} for i in range(2)])
            (root/'a.jsonl').write_text(json.dumps(row)+'\n')
            with self.assertRaisesRegex(RuntimeError, 'capacity'):
                e.run(root, root/'state', ENDPOINT, send=lambda *_: None)
            with closing(sqlite3.connect(root/'state'/'claude.sqlite')) as db:
                self.assertLessEqual(db.execute('SELECT count(*) FROM tools').fetchone()[0], 1)

    def test_oversized_line_suffix_is_not_parsed_as_an_independent_record(self):
        with tempfile.TemporaryDirectory() as temp, patch.object(e, 'MAX_LINE', 512):
            root = Path(temp)
            injected = json.dumps(record('injected', 'x'))+'\n'
            (root/'a.jsonl').write_text('x'*514+injected)
            sent = []
            e.run(root, root/'state', ENDPOINT, send=lambda *args: sent.append(args))
            self.assertEqual(sent, [])

    def test_context_is_chronological_across_files_and_revisions(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            # Deliberately create later events first, in a differently named file.
            later = record('s', 'b', 2, usage={'input_tokens': 26000})
            earlier = record('s', 'a', 0, usage={'input_tokens': 25000})
            revised = record('s', 'a', 1, usage={'input_tokens': 1000})
            (root/'z.jsonl').write_text(json.dumps(later)+'\n')
            (root/'a.jsonl').write_text(json.dumps(earlier)+'\n'+json.dumps(revised)+'\n')
            sent = []
            e.run(root, root/'state', ENDPOINT, send=lambda *args: sent.append(args))
            # Only the final revision of message a is a request: 1000 -> 26000
            # is growth, not a compaction. Partial updates must not create a drop.
            self.assertEqual(metrics(sent, 'claude_code.context.drop.count'), [])
            before = len(sent)
            e.run(root, root/'state', ENDPOINT, send=lambda *args: sent.append(args))
            self.assertEqual(len(sent), before)

    def test_revised_tool_content_preserves_prior_calls_without_counting_twice(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            first = record('s', 'a', content=[{'type':'tool_use','id':'t1','name':'Edit'}])
            second = record('s', 'a', 1, content=[{'type':'tool_use','id':'t2','name':'Edit'}])
            (root/'a.jsonl').write_text(json.dumps(first)+'\n'+json.dumps(second)+'\n')
            sent = []
            e.run(root, root/'state', ENDPOINT, send=lambda *args: sent.append(args))
            self.assertEqual(metrics(sent, 'claude_code.tool.count')[-1]['sum']['dataPoints'][0]['asDouble'], 2)

    def test_later_revision_can_introduce_new_counter_without_recounting_session(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            path = root/'a.jsonl'
            a = record('s', 'a', 0, usage={'input_tokens':26000})
            b = record('s', 'b', 1, usage={'input_tokens':25000})
            path.write_text(json.dumps(a)+'\n'+json.dumps(b)+'\n')
            sent = []
            send = lambda *args: sent.append(args)
            e.run(root, root/'state', ENDPOINT, send)
            self.assertEqual(metrics(sent, 'claude_code.context.drop.count'), [])
            with path.open('a') as stream:
                stream.write(json.dumps(record('s', 'b', 2, usage={'input_tokens':1000}))+'\n')
            e.run(root, root/'state', ENDPOINT, send)
            self.assertEqual(metrics(sent, 'claude_code.context.drop.count')[-1]['sum']['dataPoints'][0]['asDouble'], 1)
            self.assertEqual(metrics(sent, 'claude_code.session.count')[-1]['sum']['dataPoints'][0]['asDouble'], 1)
            before = len(sent)
            e.run(root, root/'state', ENDPOINT, send)
            self.assertEqual(len(sent), before)

    def test_partial_otlp_rejection_keeps_outbox_unacknowledged(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            (root/'a.jsonl').write_text(json.dumps(record('s', 'a'))+'\n')
            class Response:
                def __enter__(self): return self
                def __exit__(self, *_): pass
                def read(self, *_): return b'{"partialSuccess":{"rejectedDataPoints":1}}'
            class Opener:
                def open(self, *_args, **_kwargs): return Response()
            with patch.object(e.urllib.request, 'build_opener', return_value=Opener()):
                with self.assertRaisesRegex(RuntimeError, 'partially rejected'):
                    e.run(root, root/'state', ENDPOINT)
            with closing(sqlite3.connect(root/'state'/'claude.sqlite')) as db:
                self.assertEqual(db.execute('SELECT sent FROM pending').fetchone()[0], 0)
            sent = []
            e.run(root, root/'state', ENDPOINT, send=lambda *args: sent.append(args))
            self.assertEqual(metrics(sent, 'claude_code.session.count')[-1]['sum']['dataPoints'][0]['asDouble'], 1)

    def test_legacy_pending_metadata_is_scrubbed_without_losing_accounting(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            row = record('s', 'a')
            (root/'a.jsonl').write_text(json.dumps(row)+'\n')
            sent = []
            e.run(root, root/'state', ENDPOINT, send=lambda *args: sent.append(args))
            dbpath = root/'state'/'claude.sqlite'
            with closing(sqlite3.connect(dbpath)) as db:
                key = db.execute('SELECT key FROM seen LIMIT 1').fetchone()[0]
                db.execute('INSERT INTO pending VALUES (?,?,0)',
                           (key, json.dumps([('metrics', {'resourceMetrics': []}),
                                            ('logs', {'canary': 'PRIVATE_METADATA_CANARY'}),
                                            ('traces', {'canary': 'PRIVATE_METADATA_CANARY'})])))
                db.execute('PRAGMA journal_mode=WAL')
                db.commit()
            row['timestamp'] = (datetime.now(timezone.utc)-timedelta(days=3)).isoformat()
            (root/'a.jsonl').write_text(json.dumps(row)+'\n')
            self.assertIn(b'PRIVATE_METADATA_CANARY', dbpath.read_bytes())
            before = len(sent)
            e.run(root, root/'state', ENDPOINT, send=lambda *args: sent.append(args))
            self.assertEqual(len(sent), before+1)  # recover only the staged metrics
            self.assertFalse(any(b'PRIVATE_METADATA_CANARY' in p.read_bytes()
                                 for p in (root/'state').glob('claude.sqlite*')))
            with closing(sqlite3.connect(dbpath)) as db:
                self.assertEqual(db.execute('SELECT count(*) FROM seen').fetchone()[0], 1)
                self.assertEqual(db.execute('SELECT total FROM series').fetchone()[0], 1)
                self.assertEqual(db.execute('SELECT count(*) FROM pending').fetchone()[0], 0)
            e.run(root, root/'state', ENDPOINT, send=lambda *args: sent.append(args))
            self.assertEqual(len(sent), before+1)

    def test_redirects_cannot_forward_payloads_off_loopback(self):
        request = e.urllib.request.Request(ENDPOINT+'/v1/logs', data=b'{}')
        with self.assertRaisesRegex(ValueError, 'redirects'):
            e.NoRedirect().redirect_request(request, None, 302, 'Found', {}, 'https://example.invalid/collect')

    def test_empty_cli_path_is_not_implicit_current_directory(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            source = Path(e.__file__).resolve()
            result = subprocess.run([sys.executable, str(source), '--projects-dir', '',
                                     '--state-dir', str(root/'state')], cwd=root,
                                    capture_output=True, text=True)
            self.assertEqual(result.returncode, 2)
            self.assertFalse((root/'state').exists())
            self.assertIn('nonempty', result.stderr)

    def test_state_refuses_unrelated_files_before_chmod_or_writes(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            (root/'a.jsonl').write_text(json.dumps(record('s','a'))+'\n')
            original = root.stat().st_mode
            with self.assertRaisesRegex(ValueError, 'dedicated'):
                e.run(root, root, ENDPOINT, send=lambda *_: None)
            self.assertEqual(root.stat().st_mode, original)
            self.assertFalse((root/'identity.salt').exists())


if __name__ == '__main__':
    unittest.main()
