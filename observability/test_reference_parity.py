"""Guard immutable references and provisioned Claude lab presentation."""
import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parent
EXPECTED = {
    "claude-code.json": ("edcc25fd321c52ebe565a05c54bf071fc3c3abeb9582db5bd6c93c9ee00672e5", "claude-code-telemetry-lab", 37),
    "claude-code-members.json": ("265f3d474013be12e27d8a200ab72b44654e4f00d66c595199d2c289f00c2e21", "claude-code-members-sessions", 16),
    "claude-code-budget.json": ("8eff82483192db2da30ad5cce0765e9031e9f4b90a95c646999bf4aa0f512761", "claude-code-budget", 6),
}

class ReferenceParity(unittest.TestCase):
    def test_reference_bytes_and_panel_families(self):
        directory = ROOT / "reference-dashboards"
        self.assertEqual(set(EXPECTED), {p.name for p in directory.glob("*.json")})
        for name, (digest, uid, count) in EXPECTED.items():
            payload = (directory / name).read_bytes()
            self.assertEqual(hashlib.sha256(payload).hexdigest(), digest, name)
            dashboard = json.loads(payload)
            self.assertEqual(dashboard["uid"], uid)
            self.assertEqual(len(dashboard["panels"]), count)

    def test_provisioned_grid_and_sources(self):
        provider = (ROOT / 'grafana/dashboards/provider.yaml').read_text()
        self.assertIn('path: /etc/grafana/provisioning/dashboards', provider)
        for name in EXPECTED:
            original = json.loads((ROOT / 'reference-dashboards' / name).read_text())
            live = json.loads((ROOT / 'grafana/dashboards' / name).read_text())
            self.assertEqual([(p['id'], p.get('gridPos'), p.get('type')) for p in original['panels']],
                             [(p['id'], p.get('gridPos'), p.get('type')) for p in live['panels']])
            self.assertIn('INCOMPLETE', live['description'])
            self.assertIn('incomplete', live['panels'][-1]['description'])
        sources = (ROOT / 'grafana/datasources/default.yaml').read_text()
        for uid in ('local-prometheus', 'loki', 'tempo'):
            self.assertIn('uid: ' + uid, sources)

    def test_full_presentation_and_queries_except_enumerated_wiring(self):
        def normalize(value):
            if isinstance(value, dict):
                result = {key: ('local-prometheus' if key == 'uid' and item == 'prometheus' else
                                'Not observed' if key == 'noValue' and item in ('0', 0) else normalize(item))
                          for key, item in value.items() if key != 'description'}
                if result.get('id') == 'noValue' and result.get('value') in ('0', 0):
                    result['value'] = 'Not observed'
                if isinstance(result.get('expr'), str):
                    # Normalize the two enumerated native-source compatibility
                    # changes only. Every other expression remains exact.
                    expression = result['expr']
                    expression = expression.replace('claude_code_code_edit_tool_decision_total{data_source="native",',
                                                    'claude_code_code_edit_tool_decision_total{')
                    expression = expression.replace('event_name =~ "api_request|api_cost"', 'event_name = "api_request"')
                    result['expr'] = expression
                return result
            if isinstance(value, list):
                return [normalize(item) for item in value]
            return value.replace('${data_source:raw}', '$data_source') if isinstance(value, str) else value
        legacy_trace = '{ name = "claude_code.interaction" && (span.data_source =~ "$data_source" || (span.data_source = nil && "" =~ "$data_source")) }'
        local_trace = '{ name = "claude_code.interaction" && span.data_source =~ "$data_source" }'
        for name in EXPECTED:
            original = json.loads((ROOT/'reference-dashboards'/name).read_text())
            live = json.loads((ROOT/'grafana/dashboards'/name).read_text())
            source = live['templating']['list'][0]
            self.assertEqual(source['name'], 'data_source')
            self.assertEqual(source['current']['value'], '.*')
            self.assertEqual([(v['text'], v['value']) for v in source['options']],
                             [('transcript','transcript'), ('real-otel','native|^$'), ('all','.*'),
                              ('synthetic','synthetic'), ('transcript-fixture','transcript-fixture')])
            old_source = original['templating']['list'][0]
            allowed = {'description','query','current','options'}
            self.assertEqual({k:v for k,v in source.items() if k not in allowed},
                             {k:v for k,v in old_source.items() if k not in allowed})
            # Datasource selection is the only variable wiring change. Other
            # variables, defaults, links, grids, units and chart options are exact.
            original['templating']['list'][0] = copy.deepcopy(source)
            if name == 'claude-code-budget.json':
                warning = live['panels'][0]
                self.assertEqual(warning['id'], 106)
                self.assertIn('INCOMPLETE', warning['title'])
                self.assertIn('seven-day', warning['title'])
                original['panels'][0]['title'] = warning['title']
            for old_panel, new_panel in zip(original['panels'], live['panels']):
                group = 'model' if name == 'claude-code.json' and old_panel['id'] == 30 else (
                        'user' if name == 'claude-code-members.json' and old_panel['id'] in (53,54) else None)
                if group:
                    wanted = 'C' if old_panel['id'] == 54 else 'A'
                    old_target = next(t for t in old_panel['targets'] if t['refId'] == wanted)
                    new_target = next(t for t in new_panel['targets'] if t['refId'] == wanted)
                    profile = ' | label_format user="local-user"' if group == 'user' else ''
                    expected = 'max by ('+group+') (max_over_time({service_name="claude-code"} | event_name="api_request" | data_source=~"${data_source:raw}" | event_id!="" | context_pct!=""'+profile+' | unwrap context_pct [$__range]))'
                    self.assertEqual(new_target['expr'], expected)
                    self.assertEqual(new_target['datasource'], {'type':'loki','uid':'loki'})
                    self.assertEqual(new_target['queryType'], 'instant')
                    old_target.update(expr=expected, datasource={'type':'loki','uid':'loki'}, queryType='instant')
                    expected_source = {'type':'mixed','uid':'-- Mixed --'} if old_panel['id'] == 54 else {'type':'loki','uid':'loki'}
                    self.assertEqual(new_panel['datasource'], expected_source)
                    old_panel['datasource'] = expected_source
                if name == 'claude-code-members.json':
                    for target in old_panel.get('targets', []):
                        if target.get('query') == legacy_trace:
                            target['query'] = local_trace
            self.assertEqual(normalize(original), normalize(live), name)

    def test_generated_artifacts_match_pinned_reference_adapter(self):
        spec = importlib.util.spec_from_file_location('reference_adapter', ROOT/'sync_reference_dashboards.py')
        adapter = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(adapter)
        for name in EXPECTED:
            reference = json.loads((ROOT/'reference-dashboards'/name).read_text())
            actual = json.loads((ROOT/'grafana/dashboards'/name).read_text())
            self.assertEqual(adapter.local_dashboard(reference), actual)
            for panel in actual['panels']:
                defaults = panel.get('fieldConfig', {}).get('defaults', {})
                self.assertNotEqual(defaults.get('noValue'), '0', (name, panel['id']))
                for target in panel.get('targets', []):
                    expression = target.get('expr', target.get('query', ''))
                    self.assertNotIn('$data_source', expression, 'Grafana would escape the fixed regex choice')

if __name__ == "__main__":
    unittest.main()
