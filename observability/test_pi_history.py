"""Hermetic history fixtures; no Pi session discovery or HOME access."""
import json
import tempfile
import unittest
from pathlib import Path
from pi_history import read_history


class HistoryTest(unittest.TestCase):
    def test_incremental_recovery_privacy_and_format(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            session, checkpoint, sink = (root / name for name in ('fake.jsonl', 'state.json', 'out.jsonl'))
            secret = 'PROMPT_SECRET_ARGUMENT_RESULT_PATH_/private/example'
            header = {'type': 'session', 'version': 3, 'id': 'uuid', 'cwd': secret, 'timestamp': '2024-12-03T14:00:00Z'}
            def entry(ident, error=False):
                return {'type': 'message', 'id': ident, 'parentId': None, 'timestamp': '2024-12-03T14:00:01Z', 'message': {'role': 'toolResult', 'toolCallId': secret, 'toolName': secret, 'content': [{'type': 'text', 'text': secret}], 'isError': error}}
            def append(obj, complete=True):
                with session.open('ab') as file:
                    file.write(json.dumps(obj).encode() + (b'\n' if complete else b''))
            append(header)
            append({'type': 'message', 'id': 'user1', 'parentId': None, 'message': {'role': 'user', 'content': secret}})
            append(entry('tool1'))
            self.assertEqual([row['kind'] for row in read_history(session, checkpoint, sink)], ['session', 'tool_result'])
            self.assertEqual(read_history(session, checkpoint, sink), [])
            fragment = json.dumps(entry('tool2', True)).encode()
            with session.open('ab') as file:
                file.write(fragment[:20])
            self.assertEqual(read_history(session, checkpoint, sink), [])
            with session.open('ab') as file:
                file.write(fragment[20:] + b'\n')
            self.assertEqual([row['status'] for row in read_history(session, checkpoint, sink)], ['error'])
            # Simulate restart with stale checkpoint after sink fsync.
            state = json.loads(checkpoint.read_text())
            checkpoint.unlink()
            self.assertEqual(read_history(session, checkpoint, sink), [])
            self.assertEqual(len(sink.read_text().splitlines()), 3)
            self.assertEqual(json.loads(checkpoint.read_text()), state)
            for path in (sink, checkpoint):
                self.assertNotIn(secret, path.read_text())
                self.assertNotIn('tool1', path.read_text())
                self.assertEqual(path.stat().st_mode & 0o777, 0o600)
            with session.open('ab') as file:
                file.write(b'{"type":"new_format","id":"x","parentId":null}\n')
            before = checkpoint.read_bytes()
            with self.assertRaisesRegex(ValueError, 'unsupported Pi entry type'):
                read_history(session, checkpoint, sink)
            self.assertEqual(checkpoint.read_bytes(), before)
            self.assertNotIn(secret, sink.read_text() + checkpoint.read_text())
            session.write_text(json.dumps({**header, 'version': 4}) + '\n')
            with self.assertRaises(ValueError):
                read_history(session, checkpoint, sink)

    def test_session_identity_deduplicates_across_paths_and_restart(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            sink = root / 'out.jsonl'
            identity = 'private-session-id'
            for index, name in enumerate(('first', 'resume', 'fork')):
                session = root / (name + '.jsonl')
                checkpoint = root / (name + '.state')
                session.write_text(json.dumps({'type': 'session', 'version': 3,
                                               'id': identity if index < 2 else 'fork-id'}) + '\n')
                records = read_history(session, checkpoint, sink)
                self.assertEqual(len(records), 0 if index == 1 else 1)
                self.assertEqual(read_history(session, checkpoint, sink), [])
                checkpoint.unlink()  # replay after stale/missing checkpoint
                self.assertEqual(read_history(session, checkpoint, sink), [])
            rows = [json.loads(line) for line in sink.read_text().splitlines()]
            self.assertEqual(len(rows), 2)
            self.assertEqual({row['status'] for row in rows}, {'derived'})
            self.assertNotIn(identity, sink.read_text())
            self.assertNotIn('fork-id', sink.read_text())

    def test_partial_and_malformed_header_fail_closed(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            session, checkpoint, sink = (root / name for name in ('fake.jsonl', 'state.json', 'out.jsonl'))
            session.write_bytes(b'{"type":"session","version":3,"id":"x"')
            self.assertEqual(read_history(session, checkpoint, sink), [])
            self.assertFalse(sink.exists())
            self.assertEqual(json.loads(checkpoint.read_text())['offset'], 0)
            for bad in (b'{invalid}\n', b'{"type":"session","version":3,"id":""}\n'):
                session.write_bytes(bad)
                with self.assertRaises(ValueError):
                    read_history(session, checkpoint, sink)
                self.assertFalse(sink.exists())
                self.assertEqual(json.loads(checkpoint.read_text())['offset'], 0)

    def test_oversize_unterminated_line_is_rejected_without_checkpoint_advance(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            session, checkpoint, sink = (root / name for name in ('fake.jsonl', 'state.json', 'out.jsonl'))
            session.write_bytes(b'{"type":"session","version":3,"id":"x"}\n')
            read_history(session, checkpoint, sink)
            before = checkpoint.read_bytes()
            with session.open('ab') as output:
                output.write(b'P' * (1_048_576 + 1))
            with self.assertRaisesRegex(ValueError, 'bounded history limit'):
                read_history(session, checkpoint, sink)
            self.assertEqual(checkpoint.read_bytes(), before)
            self.assertEqual([row['kind'] for row in map(json.loads, sink.read_text().splitlines())], ['session'])
            # A bounded incomplete line must wait; completion then advances once.
            session.write_bytes(b'{"type":"session","version":3,"id":"x"}\n')
            with session.open('ab') as output:
                output.write(b'{"type":"message","id":"later","parentId":null')
            self.assertEqual(read_history(session, checkpoint, sink), [])
            self.assertEqual(checkpoint.read_bytes(), before)
            with session.open('ab') as output:
                output.write(b'}\n')
            self.assertEqual(read_history(session, checkpoint, sink), [])
            self.assertGreater(json.loads(checkpoint.read_text())['offset'], json.loads(before)['offset'])
            self.assertEqual(len(sink.read_text().splitlines()), 1)


if __name__ == '__main__':
    unittest.main()
