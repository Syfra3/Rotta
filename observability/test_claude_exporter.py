import importlib.util
import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('claude_exporter', Path(__file__).with_name('claude_exporter.py'))
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


class ClaudePrivacy(unittest.TestCase):
    def test_adversarial_record_and_dedup(self):
        from datetime import datetime, timezone
        record = {'type': 'assistant', 'timestamp': datetime.now(timezone.utc).isoformat(), 'sessionId': 'private-session',
                  'message': {'id': 'unique-msg', 'model': 'sonnet /home/private', 'usage': {'input_tokens': 9},
                              'content': [{'type': 'tool_use', 'id': 'tool', 'name': 'private_tool', 'input': {'command': 'secret'}}]},
                  'prompt': 'private-prompt', 'user.email': 'private@example.com'}
        rows = module.derive(record, b'fixture-salt'.ljust(32, b'0'))
        self.assertGreaterEqual(len(rows), 4)
        self.assertIn('claude_code.session.count', json.dumps(rows))
        self.assertIn('claude_code.context.size', json.dumps(rows))
        self.assertIn('claude_code.llm_request', json.dumps(rows))
        self.assertNotIn('private', json.dumps(rows))
        self.assertNotIn('secret', json.dumps(rows))
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            (root / 'a.jsonl').write_text(json.dumps(record) + '\n' + json.dumps(record) + '\n')
            calls = []
            def send(signal, body):
                calls.append(body.encode())
            module.run(root, root / 'state', 'http://127.0.0.1:14318', send=send)
            before = len(calls)
            module.run(root, root / 'state', 'http://127.0.0.1:14318', send=send)
            self.assertEqual(len(calls), before)
            self.assertGreaterEqual(len(calls), 3)
            self.assertTrue(all(b'private' not in data and b'secret' not in data for data in calls))
            with self.assertRaises(ValueError): module.run(root, root / 'state', 'https://example.com')

    def test_collector_deny_by_default(self):
        text = Path(__file__).with_name('collector.claude.yaml').read_text()
        for key in ('filter/claude_logs', 'filter/claude_values', 'filter/claude_traces',
                    'filter/claude_trace_values', 'keep_keys(attributes, ["service.name"])',
                    'set(body, "claude_code.metadata")'):
            self.assertIn(key, text)


if __name__ == '__main__': unittest.main()
