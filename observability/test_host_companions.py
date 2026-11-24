"""Offline contract for the provisioned host views, not a live compatibility claim."""
import importlib.util
import json
import pathlib
import unittest

ROOT = pathlib.Path(__file__).parent


class HostCompanions(unittest.TestCase):
    def test_native_query_mappings_and_honest_unavailable(self):
        dashboard = json.loads((ROOT / 'grafana/dashboards/host-companions.json').read_text())
        self.assertEqual(dashboard['uid'], 'ai-cli-host-companions')
        panels = {p['id']: p for p in dashboard['panels']}
        mappings = {
            1: 'ai_cli_estimated_cost_usd_v1_USD_total',
            2: 'ai_cli_tokens_v1_total',
            3: 'ai_cli_tokens_v1_total',
            4: 'ai_cli_tokens_v1_total',
            5: 'ai_cli_pi_model_tokens_total',
            6: 'ai_cli_pi_tool_call_total',
            7: 'ai_cli_pi_agent_runs_total',
            8: 'ai_cli_pi_model_estimated_cost_usd_USD_total',
        }
        collector = (ROOT / 'collector.yaml').read_text()
        for panel_id, name in mappings.items():
            with self.subTest(panel=panel_id):
                expr = panels[panel_id]['targets'][0]['expr']
                self.assertIn(name, expr)
                self.assertIn('source=\\"native\\"'.replace('\\"', '"'), expr)
                self.assertNotIn(' or vector(0)', expr)
                self.assertIn(name.removesuffix('_USD_total').removesuffix('_total'), collector)
        for panel_id in (5, 6, 7, 8):
            self.assertIn('client="pi"', panels[panel_id]['targets'][0]['expr'])
        self.assertIn('unavailable', panels[9]['options']['content'].lower())
        self.assertNotIn('targets', panels[9])

    def test_count_panels_use_observed_range_totals_not_rate_extrapolation(self):
        source = json.loads((ROOT/'host-companions.source.json').read_text())
        spec = importlib.util.spec_from_file_location('host_adapter', ROOT/'sync_reference_dashboards.py')
        adapter = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(adapter)
        actual = json.loads((ROOT/'grafana/dashboards/host-companions.json').read_text())
        self.assertEqual(adapter.host_dashboard(source), actual)
        for panel in actual['panels']:
            for target in panel.get('targets', []):
                if panel['id'] != 3:  # Rate panel intentionally remains rate/second.
                    self.assertNotIn('increase(', target['expr'])
                    self.assertIn('last_over_time(', target['expr'])
        self.assertEqual([(p['id'],p['gridPos'],p['type']) for p in source['panels']],
                         [(p['id'],p['gridPos'],p['type']) for p in actual['panels']])


if __name__ == '__main__':
    unittest.main()
