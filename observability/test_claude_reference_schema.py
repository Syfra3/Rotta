"""Reference-shaped metadata fixture: emitted OTLP, not just a static allowlist."""
import json
import tempfile
import unittest
from datetime import datetime, timezone
from pathlib import Path
from unittest.mock import patch

from observability import claude_exporter as exporter


class ReferenceSchema(unittest.TestCase):
    def test_serialized_signals_and_replay(self):
        now = datetime.now(timezone.utc).isoformat()
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            projects = root / 'projects'
            projects.mkdir()
            state = root / 'state'
            records = [
                {'type': 'user', 'timestamp': now, 'sessionId': 'private-session', 'promptId': 'private-prompt',
                 'uuid': 'prompt-1', 'message': {'content': 'PRIVATE PROMPT'}},
                {'type': 'assistant', 'timestamp': now, 'sessionId': 'private-session', 'promptId': 'private-prompt',
                 'gitBranch': 'ticket-private', 'message': {'id': 'm1', 'model': 'claude-sonnet-5-5',
                 'usage': {'input_tokens': 25000, 'output_tokens': 100, 'cache_read_input_tokens': 1000},
                 'content': [{'type': 'tool_use', 'id': 'call-1', 'name': 'Bash',
                              'input': {'command': 'git commit -m "PRIVATE MESSAGE" && git push', 'file_path': '/PRIVATE/PATH'}}]}},
                {'type': 'user', 'timestamp': now, 'sessionId': 'private-session', 'promptId': 'private-prompt',
                 'permissionDecision': {'decision': 'reject', 'source': 'config'},
                 'message': {'content': [{'type': 'tool_result', 'tool_use_id': 'call-1', 'content': 'PRIVATE RESULT'}]}},
                {'type': 'assistant', 'timestamp': now, 'sessionId': 'private-session', 'promptId': 'private-prompt',
                 'message': {'id': 'm2', 'model': 'claude-sonnet-5-5', 'usage': {'input_tokens': 1000, 'output_tokens': 10}, 'content': []}},
                {'type': 'system', 'timestamp': now, 'sessionId': 'private-session', 'uuid': 'retry1',
                 'subtype': 'api_error', 'retryAttempt': 2, 'maxRetries': 4, 'retryInMs': 50},
            ]
            (projects / 'fixture.jsonl').write_text(''.join(json.dumps(r) + '\n' for r in records))
            sent = []
            def send(signal, body):
                sent.append((signal, json.loads(body)))
            exporter.run(projects, state, 'http://127.0.0.1:14318', send)
            first = len(sent)
            exporter.run(projects, state, 'http://127.0.0.1:14318', send)
            self.assertEqual(first, len(sent))
            raw = json.dumps(sent)
            for secret in ('private-session', 'private-prompt', 'PRIVATE', 'ticket-private', 'git push'):
                self.assertNotIn(secret, raw)
            metrics = [m for kind, payload in sent if kind == 'metrics' for group in payload['resourceMetrics'] for scope in group['scopeMetrics'] for m in scope['metrics']]
            names = {m['name'] for m in metrics}
            self.assertTrue({'claude_code.context.size', 'claude_code.context.utilization', 'claude_code.context.drop.count', 'claude_code.git_action.count', 'claude_code.session.count'} <= names)
            self.assertFalse({'claude_code.cost.usage', 'claude_code.token.usage'} & names)
            drops = [m for m in metrics if m['name'] == 'claude_code.context.drop.count']
            self.assertEqual(len(drops), 1)
            self.assertEqual(drops[0]['sum']['dataPoints'][0]['asDouble'], 1)
            logs = [l for kind, payload in sent if kind == 'logs' for group in payload['resourceLogs'] for scope in group['scopeLogs'] for l in scope['logRecords']]
            attrs = [{x['key'].replace('.', '_'): x['value']['stringValue'] for x in l['attributes']} for l in logs]
            result = next(a for a in attrs if a['event_name'] == 'tool_result')
            self.assertEqual(result['success'], 'false')
            self.assertEqual(result['command_class'], 'git')
            self.assertEqual(result['git_action'], 'commit,push')
            self.assertEqual(json.loads(result['tool_parameters']).keys(), {'path_hash'})
            self.assertEqual(next(a for a in attrs if a['event_name'] == 'api_error')['outcome'], 'retry')
            self.assertTrue(all(len(a['prompt_id']) == 16 for a in attrs))

    def test_command_classification_never_exports_arguments(self):
        self.assertEqual(exporter.command_metadata('git commit -m "git push"'), ('git', ('commit',)))
        self.assertEqual(exporter.command_metadata('git checkout -- private.txt'), ('git', ()))
        self.assertEqual(exporter.command_metadata('gh pr create --title "PRIVATE"'), ('gh', ('pr_create',)))
        self.assertEqual(exporter.command_metadata('echo x <<EOF\ngit push\nEOF'), ('echo', ()))
        self.assertEqual(exporter.model_class('unrecognized'), 'unknown')


if __name__ == '__main__':
    unittest.main()
