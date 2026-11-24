import importlib.util
import json
import tempfile
import unittest
from datetime import datetime, timezone
from pathlib import Path

spec = importlib.util.spec_from_file_location('exporter', Path(__file__).with_name('claude_exporter.py'))
e = importlib.util.module_from_spec(spec)
spec.loader.exec_module(e)


class Accounting(unittest.TestCase):
    def test_revisions_sessions_tools_and_retry(self):
        stamp = datetime.now(timezone.utc).isoformat()
        def row(session, mid, content=None, usage=None, kind='assistant'):
            return {'type': kind, 'timestamp': stamp, 'sessionId': session,
                    'message': {'id': mid, 'usage': usage, 'content': content or []}}
        use = lambda ident: {'type': 'tool_use', 'id': ident, 'name': 'Edit'}
        records = [row('s1', 'm1', usage=None),
                   row('s1', 'm1', [use('t1'), use('t2')], {'input_tokens': 7}),
                   row('s2', 'm2', [use('t3')], {'input_tokens': 5}),
                   row('s1', 'result', [{'type': 'tool_result', 'tool_use_id': 't1'}], kind='user')]
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            path = root / 'events.jsonl'
            path.write_text(''.join(json.dumps(r) + '\n' for r in records))
            sent = []
            def send(signal, body):
                sent.append((signal, json.loads(body)))
            for _ in range(2):
                e.run(root, root / 'state', 'http://127.0.0.1:14318', send=send)
            sums = {}
            for signal, body in sent:
                if signal != 'metrics': continue
                for metric in body['resourceMetrics'][0]['scopeMetrics'][0]['metrics']:
                    if 'sum' in metric:
                        sums[metric['name']] = metric['sum']['dataPoints'][0]['asDouble']
            self.assertEqual(sums['claude_code.session.count'], 2)
            self.assertEqual(sums['claude_code.tool.count'], 3)
            logs = json.dumps([body for signal, body in sent if signal == 'logs'])
            self.assertIn('Edit', logs)
            self.assertNotIn('code_edit_tool.decision', json.dumps(sent))
            before = len(sent)
            e.run(root, root / 'state', 'http://127.0.0.1:14318', send=send)
            self.assertEqual(len(sent), before)


if __name__ == '__main__': unittest.main()
