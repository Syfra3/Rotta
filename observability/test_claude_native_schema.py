import importlib.util
import json
import unittest
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parent

def load(name, filename):
    spec = importlib.util.spec_from_file_location(name, ROOT/filename)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class NativeSchema(unittest.TestCase):
    def test_native_model_identity_and_pi_filter_are_separate_from_derived_families(self):
        claude = (ROOT/'collector.claude.yaml').read_text()
        canonical = (ROOT/'collector.yaml').read_text()
        rule = next(line.strip() for line in canonical.splitlines() if 'not IsMatch(attributes["model"], "^[A-Za-z0-9]' in line)
        self.assertIn(rule, claude)
        native = claude.split('  transform/claude_native:', 1)[1].split('  filter/claude_metrics:', 1)[0]
        self.assertIn('not IsMatch(attributes["model"], "^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$")', native)
        # Hash the raw identity before trimming; normalize only the exported label.
        hash_pos = native.index('set(attributes["counter.id"], SHA256(')
        trim_pos = native.index('replace_pattern(attributes["model"]')
        validate_pos = native.index('not IsMatch(attributes["model"],')
        self.assertLess(hash_pos, trim_pos)
        self.assertLess(trim_pos, validate_pos)
        self.assertIn('where IsString(attributes["model"])', native[trim_pos:validate_pos])
        self.assertIn('delete_key(attributes, "model") where metric.name == "claude_code.code_edit_tool.decision"', native)
        self.assertNotIn('set(attributes["model"], "sonnet")', native)
        self.assertIn('set(attributes["model"], "sonnet")', claude.split('  filter/claude_metrics:', 1)[1])

    def test_documented_native_cost_fixture_preserves_typed_values_for_receiver_test(self):
        probe = load('native_probe', 'verify_lab.py')
        logs = probe.native_api_costs()['resourceLogs'][0]['scopeLogs'][0]['logRecords']
        self.assertEqual([next(a['value']['doubleValue'] for a in row['attributes'] if a['key']=='cost_usd')
                          for row in logs], [.125, .25])
        self.assertTrue(all(any(a['key']=='event.sequence' and 'intValue' in a['value']
                                for a in row['attributes']) for row in logs))

    def test_each_parallel_event_has_a_distinct_stable_pseudonymous_identity(self):
        exporter = load('id_exporter', 'claude_exporter.py')
        record = {'type':'assistant','timestamp':datetime.now(timezone.utc).isoformat(),
                  'sessionId':'private-session','message':{'id':'private-msg','usage':{'input_tokens':10},
                    'model':'claude-sonnet-4-5','content':[{'type':'tool_use','id':str(i),'name':'Edit'} for i in range(2)]}}
        def identities():
            rows = exporter.derive(record, b'fixture-salt'.ljust(32,b'0'))
            return [next(a['value']['stringValue'] for a in item['attributes'] if a['key']=='event.id')
                    for _, signal, item in rows if signal=='logs']
        ids = identities()
        self.assertEqual(len(ids), 4)  # session, API request, two tool calls
        self.assertEqual(len(set(ids)), 4)
        self.assertEqual(identities(), ids)
        self.assertTrue(all(len(value)==16 and all(c in '0123456789abcdef' for c in value) for value in ids))

    def test_reference_edit_counter_clauses_use_native_authority(self):
        total = 0
        for filename in ('claude-code.json','claude-code-members.json','claude-code-budget.json'):
            dashboard = json.loads((ROOT/'grafana/dashboards'/filename).read_text())
            for panel in dashboard['panels']:
                for target in panel.get('targets',[]):
                    expression = target.get('expr','')
                    count = expression.count('claude_code_code_edit_tool_decision_total{')
                    total += count
                    self.assertEqual(count, expression.count('claude_code_code_edit_tool_decision_total{data_source="native",'))
        self.assertGreater(total, 0)


if __name__ == '__main__':
    unittest.main()
