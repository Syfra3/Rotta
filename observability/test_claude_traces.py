import importlib.util
import json
import tempfile
import unittest
from datetime import datetime, timedelta, timezone
from pathlib import Path

spec = importlib.util.spec_from_file_location('trace_exporter', Path(__file__).with_name('claude_exporter.py'))
e = importlib.util.module_from_spec(spec)
spec.loader.exec_module(e)


class Traces(unittest.TestCase):
    def test_interactions_are_per_prompt_with_linked_bounded_observed_intervals(self):
        base = datetime.now(timezone.utc)-timedelta(minutes=1)
        rows = []
        for i in range(2):
            stamp = lambda s: (base+timedelta(seconds=i*10+s)).isoformat()
            rows += [{'type':'user','timestamp':stamp(0),'sessionId':'private-session','uuid':f'p{i}',
                      'message':{'content':'PRIVATE_PROMPT'}},
                     {'type':'assistant','timestamp':stamp(2),'sessionId':'private-session',
                      'message':{'id':f'm{i}','model':'claude-sonnet-4-5','usage':{'input_tokens':100},
                                 'content':[{'type':'tool_use','name':'Read','id':f't{i}','input':{'path':'PRIVATE_PATH'}}]}},
                     {'type':'user','timestamp':stamp(5),'sessionId':'private-session','uuid':f'r{i}',
                      'message':{'content':[{'type':'tool_result','tool_use_id':f't{i}','content':'PRIVATE_RESULT'}]}}]
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            (root/'a.jsonl').write_text(''.join(json.dumps(r)+'\n' for r in rows))
            spans, bodies = [], []
            def send(signal, body):
                bodies.append(body)
                if signal == 'traces':
                    spans.extend(json.loads(body)['resourceSpans'][0]['scopeSpans'][0]['spans'])
            e.run(root, root/'state', 'http://127.0.0.1:14318', send)
            roots = [s for s in spans if s['name'] == 'claude_code.interaction']
            self.assertEqual(len(roots), 2)
            self.assertEqual(len({s['traceId'] for s in roots}), 2)
            for parent in roots:
                children = [s for s in spans if s.get('parentSpanId') == parent['spanId']]
                self.assertEqual(len(children), 2)
                self.assertEqual(int(parent['endTimeUnixNano'])-int(parent['startTimeUnixNano']), 5*10**9)
                for child in children:
                    self.assertEqual(child['traceId'], parent['traceId'])
                    self.assertGreater(int(child['endTimeUnixNano']), int(child['startTimeUnixNano']))
                    self.assertGreaterEqual(int(child['startTimeUnixNano']), int(parent['startTimeUnixNano']))
                    self.assertLessEqual(int(child['endTimeUnixNano']), int(parent['endTimeUnixNano']))
            self.assertNotIn('PRIVATE_', ''.join(bodies))
            self.assertNotIn('private-session', ''.join(bodies))


if __name__ == '__main__':
    unittest.main()
